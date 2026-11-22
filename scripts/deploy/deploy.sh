#!/usr/bin/env bash
# Deploy a published xbang0222/new-api image to the netcup server.
#
#   scripts/deploy/deploy.sh login            open the shared SSH connection (1Password fingerprint once, 8h)
#   scripts/deploy/deploy.sh status           show running versions and recent deploys
#   scripts/deploy/deploy.sh staging <tag>    isolated test instance: own SQLite, no Redis, 127.0.0.1:3041
#   scripts/deploy/deploy.sh tunnel           open staging at http://localhost:3041 (Ctrl-C to close)
#   scripts/deploy/deploy.sh prod <tag>       stop the old container at once, start the new one, health-check
#                                             within 30s, roll back automatically on failure
#   scripts/deploy/deploy.sh rollback         switch back to the previous container kept by the last deploy
#   scripts/deploy/deploy.sh release [--staging] [notes]
#                                             publish GitHub main (tag + Docker image), wait for the image, deploy it
#                                             to production; with --staging deploy it to staging only
set -euo pipefail

HOST=${DEPLOY_HOST:-netcup}
CONTROL_PATH=${DEPLOY_CONTROL_PATH:-$HOME/.ssh/cm-netcup}
SSH_OPTS=(-o ControlMaster=auto -o "ControlPath=$CONTROL_PATH" -o ControlPersist=8h)
REPO=${DEPLOY_REPO:-Xbang0222/new-api}

remote() { ssh "${SSH_OPTS[@]}" "$HOST" "$@"; }

usage() { sed -n '2,13p' "$0" | sed 's/^# \{0,1\}//'; exit 1; }

# Print the id of the newest run of a workflow (optionally for one branch or tag), or nothing.
latest_run() {
  gh run list --repo "$REPO" --workflow "$1" ${2:+--branch "$2"} --limit 1 --json databaseId --jq '.[0].databaseId // empty'
}

require_tag() {
  [[ ${1:-} =~ ^v[0-9A-Za-z.+-]+$ ]] || { echo "error: tag must look like v1.0.0-rc.25-ruoli-0.4" >&2; exit 1; }
}

read -r -d '' STAGING_SCRIPT <<'REMOTE' || true
set -euo pipefail
TAG=$1
IMAGE=xbang0222/new-api:$TAG
NAME=new-api-staging
PORT=3041
BASE=/opt/new-api-staging

mkdir -p "$BASE/data" "$BASE/logs"
if [ ! -f "$BASE/staging.env" ]; then
  umask 077
  printf 'SESSION_SECRET=%s\nCRYPTO_SECRET=%s\nTZ=Asia/Shanghai\nMEMORY_CACHE_ENABLED=true\nERROR_LOG_ENABLED=true\n' \
    "$(openssl rand -hex 32)" "$(openssl rand -hex 32)" > "$BASE/staging.env"
fi

docker pull -q "$IMAGE"
docker rm -f "$NAME" >/dev/null 2>&1 || true
docker run -d --name "$NAME" --restart unless-stopped \
  -p "127.0.0.1:$PORT:3000" --env-file "$BASE/staging.env" \
  -v "$BASE/data:/data" -v "$BASE/logs:/app/logs" \
  --log-driver json-file --log-opt max-size=50m --log-opt max-file=3 \
  "$IMAGE" --log-dir /app/logs >/dev/null

for _ in $(seq 60); do
  if curl -fsS "http://127.0.0.1:$PORT/api/status" 2>/dev/null | grep -q '"success":true'; then
    echo "staging is up: $IMAGE on 127.0.0.1:$PORT"
    exit 0
  fi
  sleep 2
done
echo "staging failed health check; last logs:" >&2
docker logs --tail 40 "$NAME" >&2
exit 1
REMOTE

read -r -d '' PROD_SCRIPT <<'REMOTE' || true
set -euo pipefail
TAG=$1
IMAGE=xbang0222/new-api:$TAG
NAME=new-api
PREV=new-api-prev
PORT=3031
BASE=/opt/new-api
# The owner wants the old version replaced at once, not drained: requests still
# running when it stops are cut off. Worst case without service: STOP_SECONDS,
# HEALTH_SECONDS for the new container, then a few seconds to restart the old one.
# The database is not backed up here; the hourly off-site backup covers it.
STOP_SECONDS=3
HEALTH_SECONDS=30
# Written into the new container so it stops its HTTP server and saves the
# dashboard cache before the next deploy's STOP_SECONDS deadline.
APP_SHUTDOWN_SECONDS=1

healthy() {
  local deadline=$((SECONDS + HEALTH_SECONDS))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if curl -fsS --max-time 2 "http://127.0.0.1:$PORT/api/status" 2>/dev/null | grep -q '"success":true'; then
      return 0
    fi
    [ "$(docker inspect -f '{{.State.Running}}' "$NAME" 2>/dev/null)" = true ] || return 1
    sleep 1
  done
  return 1
}

CURRENT=$(docker inspect -f '{{.Config.Image}}' "$NAME")
if [ "$CURRENT" = "$IMAGE" ]; then
  echo "production already runs $IMAGE"
  exit 0
fi

echo "==> pulling $IMAGE"
docker pull -q "$IMAGE"

TS=$(date +%Y%m%d-%H%M%S)
DIR=$BASE/backups/deploy-$TS
umask 077
mkdir -p "$DIR"

echo "==> saving current container config ($CURRENT)"
docker inspect "$NAME" > "$DIR/container.inspect.json"
docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$NAME" | grep -v -e '^PATH=' -e '^$' > "$DIR/app.env"
echo "$CURRENT" > "$DIR/previous-image"
grep -v '^SHUTDOWN_TIMEOUT_SECONDS=' "$DIR/app.env" > "$DIR/app.env.next" || true
echo "SHUTDOWN_TIMEOUT_SECONDS=$APP_SHUTDOWN_SECONDS" >> "$DIR/app.env.next"

echo "==> switching $CURRENT -> $IMAGE"
mkdir -p "$BASE/logs-$TAG"
switch_started=$SECONDS
# SIGTERM, then SIGKILL after STOP_SECONDS. A killed container exits 137; the
# upgrade continues whatever the exit code, since only a restart brings it back.
docker stop -t "$STOP_SECONDS" "$NAME" >/dev/null
if [ "$(docker inspect -f '{{.State.Running}}' "$NAME")" != false ]; then
  echo "current container is still running; aborting upgrade" >&2
  exit 1
fi
echo "    old container stopped after $((SECONDS - switch_started))s (exit $(docker inspect -f '{{.State.ExitCode}}' "$NAME"))"
docker rm -f "$PREV" >/dev/null 2>&1 || true
docker rename "$NAME" "$PREV"

rollback() {
  echo "!!> $1, rolling back to $CURRENT" >&2
  docker logs --tail 80 "$NAME" > "$DIR/failed-start.log" 2>&1 || true
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  docker rename "$PREV" "$NAME"
  docker start "$NAME" >/dev/null
  if healthy; then
    echo "!!> rolled back after $((SECONDS - switch_started))s; production runs $CURRENT again. Failed logs: $DIR/failed-start.log" >&2
  else
    echo "!!> ROLLBACK ALSO FAILED. Check 'docker logs $NAME' now." >&2
  fi
  printf '%s\t%s\t->\t%s\tFAILED-ROLLED-BACK\t%s\n' "$TS" "$CURRENT" "$IMAGE" "$DIR" >> "$BASE/deploy-history.log"
  exit 1
}

docker run -d --name "$NAME" --restart unless-stopped --network 1panel-network \
  -p "127.0.0.1:$PORT:3000" --env-file "$DIR/app.env.next" \
  -v "$BASE/data:/data" -v "$BASE/logs-$TAG:/app/logs" \
  --log-driver json-file --log-opt max-size=100m --log-opt max-file=5 \
  "$IMAGE" >/dev/null || rollback "new container did not start"

if healthy; then
  docker update --restart no "$PREV" >/dev/null
  printf '%s\t%s\t->\t%s\t%s\n' "$TS" "$CURRENT" "$IMAGE" "$DIR" >> "$BASE/deploy-history.log"
  echo "==> production is up on $IMAGE after $((SECONDS - switch_started))s (previous container kept stopped as $PREV)"
  echo "    undo: scripts/deploy/deploy.sh rollback"
  exit 0
fi
rollback "health check failed within ${HEALTH_SECONDS}s"
REMOTE

read -r -d '' ROLLBACK_SCRIPT <<'REMOTE' || true
set -euo pipefail
NAME=new-api
PREV=new-api-prev
FAILED=new-api-rolled-back
PORT=3031
BASE=/opt/new-api
STOP_SECONDS=3
HEALTH_SECONDS=30

healthy() {
  local deadline=$((SECONDS + HEALTH_SECONDS))
  while [ "$SECONDS" -lt "$deadline" ]; do
    if curl -fsS --max-time 2 "http://127.0.0.1:$PORT/api/status" 2>/dev/null | grep -q '"success":true'; then
      return 0
    fi
    sleep 1
  done
  return 1
}

docker inspect "$PREV" >/dev/null 2>&1 || { echo "no $PREV container to roll back to" >&2; exit 1; }
TARGET=$(docker inspect -f '{{.Config.Image}}' "$PREV")
started=$SECONDS
# A deploy interrupted after renaming the old container leaves no $NAME at all.
if docker inspect "$NAME" >/dev/null 2>&1; then
  CURRENT=$(docker inspect -f '{{.Config.Image}}' "$NAME")
  echo "==> rolling back $CURRENT -> $TARGET"
  docker stop -t "$STOP_SECONDS" "$NAME" >/dev/null
  docker rm -f "$FAILED" >/dev/null 2>&1 || true
  docker rename "$NAME" "$FAILED"
  docker update --restart no "$FAILED" >/dev/null
else
  CURRENT=none
  echo "==> no $NAME container; starting $TARGET"
fi
docker rename "$PREV" "$NAME"
docker update --restart unless-stopped "$NAME" >/dev/null
docker start "$NAME" >/dev/null
if healthy; then
  printf '%s\t%s\t->\t%s\tMANUAL-ROLLBACK\n' "$(date +%Y%m%d-%H%M%S)" "$CURRENT" "$TARGET" >> "$BASE/deploy-history.log"
  echo "==> production runs $TARGET again after $((SECONDS - started))s"
  [ "$CURRENT" = none ] || echo "    $CURRENT kept stopped as $FAILED"
  exit 0
fi
echo "!!> $TARGET did not pass the health check within ${HEALTH_SECONDS}s" >&2
if [ "$CURRENT" != none ]; then
  # Put back what was running before this rollback rather than leave it broken.
  echo "!!> restoring $CURRENT" >&2
  docker stop -t 5 "$NAME" >/dev/null 2>&1 || true
  docker rename "$NAME" "$PREV"
  docker update --restart no "$PREV" >/dev/null
  docker rename "$FAILED" "$NAME"
  docker update --restart unless-stopped "$NAME" >/dev/null
  docker start "$NAME" >/dev/null
  healthy && echo "!!> $CURRENT is serving again; rollback abandoned" >&2 && exit 1
fi
echo "!!> PRODUCTION IS NOT HEALTHY. Check 'docker logs $NAME' now." >&2
exit 1
REMOTE

case ${1:-} in
  login)
    ssh "${SSH_OPTS[@]}" -o ControlMaster=yes -N -f "$HOST"
    echo "connected to $HOST for 8 hours"
    ;;
  status)
    remote 'docker ps -a --filter name=new-api --format "{{.Names}}\t{{.Image}}\t{{.Status}}"; echo; tail -n 5 /opt/new-api/deploy-history.log 2>/dev/null || echo "(no deploys recorded yet)"'
    ;;
  staging)
    require_tag "${2:-}"
    remote "bash -s -- $2" <<< "$STAGING_SCRIPT"
    ;;
  tunnel)
    echo "staging: http://localhost:3041  (Ctrl-C to close)"
    ssh "${SSH_OPTS[@]}" -N -L 3041:127.0.0.1:3041 "$HOST"
    ;;
  prod)
    require_tag "${2:-}"
    remote "bash -s -- $2" <<< "$PROD_SCRIPT"
    ;;
  rollback)
    remote "bash -s" <<< "$ROLLBACK_SCRIPT"
    ;;
  release)
    shift
    target=prod
    if [ "${1:-}" = --staging ]; then target=staging; shift; fi
    notes=${1:-}
    # Checked first so a missing SSH connection fails now, not after a 20-minute image build.
    ssh -O check -o "ControlPath=$CONTROL_PATH" "$HOST" 2>/dev/null \
      || { echo "error: no SSH connection; run 'scripts/deploy/deploy.sh login' in a terminal first" >&2; exit 1; }
    head=$(gh api "repos/$REPO/commits/main" --jq .sha)
    [ "$(git rev-parse HEAD)" = "$head" ] \
      || { echo "error: this checkout is not at GitHub main ($head); merge and push first" >&2; exit 1; }

    before=$(latest_run ruoli-publish.yml)
    gh workflow run ruoli-publish.yml --repo "$REPO" --ref main ${notes:+-f notes="$notes"}
    echo "==> waiting for Ruoli Publish to start"
    for _ in $(seq 30); do
      run=$(latest_run ruoli-publish.yml)
      [ -n "$run" ] && [ "$run" != "$before" ] && break
      sleep 2
    done
    [ -n "$run" ] && [ "$run" != "$before" ] || { echo "error: Ruoli Publish did not start" >&2; exit 1; }
    gh run watch "$run" --repo "$REPO" --exit-status --interval 10 >/dev/null \
      || { echo "error: Ruoli Publish failed: gh run view $run --repo $REPO --log-failed" >&2; exit 1; }

    # Ruoli Publish creates an annotated tag; its peeled "^{}" line carries the commit it points at.
    tag=$(git ls-remote --tags "https://github.com/$REPO.git" \
      | awk -v h="$head" '$1 == h && $2 ~ /-ruoli-0\.[0-9]+\^\{\}$/ { sub("refs/tags/", "", $2); sub(/\^\{\}$/, "", $2); print $2 }' \
      | sort -V | tail -1)
    require_tag "$tag"
    echo "==> published $tag; waiting for the Docker image (usually 10-20 minutes)"
    for _ in $(seq 30); do
      build=$(latest_run docker-build.yml "$tag")
      [ -n "$build" ] && break
      sleep 2
    done
    [ -n "$build" ] || { echo "error: Docker build for $tag did not start" >&2; exit 1; }
    gh run watch "$build" --repo "$REPO" --exit-status --interval 30 >/dev/null \
      || { echo "error: Docker build failed: gh run view $build --repo $REPO --log-failed" >&2; exit 1; }

    if [ "$target" = staging ]; then
      echo "==> deploying $tag to staging"
      remote "bash -s -- $tag" <<< "$STAGING_SCRIPT"
      echo
      echo "next: scripts/deploy/deploy.sh tunnel   then check http://localhost:3041"
      echo "      scripts/deploy/deploy.sh prod $tag   only after the owner approves"
    else
      echo "==> deploying $tag to production"
      remote "bash -s -- $tag" <<< "$PROD_SCRIPT"
    fi
    ;;
  *)
    usage
    ;;
esac
