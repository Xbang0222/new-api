#!/usr/bin/env bash
# Run the same checks as .github/workflows/ci.yml on this machine.
#
#   scripts/ci-local.sh [--backend|--web] [ref]   check a commit (default: HEAD)
#   scripts/ci-local.sh --install-hook            run it automatically before every git push
#
# The commit is checked out into a temporary folder that is deleted afterwards, so
# uncommitted files never affect the result. Go and Bun download caches are reused.
set -uo pipefail

ROOT=$(git rev-parse --show-toplevel)

if [ "${1:-}" = "--install-hook" ]; then
  hook="$(git rev-parse --git-common-dir)/hooks/pre-push"
  cat > "$hook" <<'HOOK'
#!/usr/bin/env bash
# Installed by scripts/ci-local.sh. Skip once with: git push --no-verify
zero=0000000000000000000000000000000000000000
while read -r local_ref local_sha _ _; do
  case $local_ref in refs/heads/*) ;; *) continue ;; esac
  [ "$local_sha" = "$zero" ] && continue
  "$(git rev-parse --show-toplevel)/scripts/ci-local.sh" "$local_sha" || exit 1
done
HOOK
  chmod +x "$hook"
  echo "installed $hook"
  exit 0
fi

PART=all
case ${1:-} in
  --backend) PART=backend; shift ;;
  --web) PART=web; shift ;;
esac
REF=${1:-HEAD}
SHA=$(git rev-parse --verify "$REF^{commit}") || exit 1

WORK=$(mktemp -d "${TMPDIR:-/tmp}/new-api-ci.XXXXXX")
cleanup() {
  git -C "$ROOT" worktree remove --force "$WORK/src" >/dev/null 2>&1
  rm -rf "$WORK"
}
trap cleanup EXIT

git -C "$ROOT" worktree add --detach --quiet "$WORK/src" "$SHA" || exit 1
cd "$WORK/src" || exit 1
export GOWORK=off

RESULTS=()
FAILED=0
step() {
  local name=$1; shift
  local start=$SECONDS
  echo "==> $name"
  if "$@" > "$WORK/step.log" 2>&1; then
    RESULTS+=("PASS  $name ($((SECONDS - start))s)")
  else
    RESULTS+=("FAIL  $name ($((SECONDS - start))s)")
    FAILED=1
    tail -n 40 "$WORK/step.log"
  fi
}

echo "checking $(git log -1 --format='%h %s' "$SHA")"

if [ "$PART" != web ]; then
  mkdir -p web/dist && touch web/dist/index.html
  step "go vet (root)" go vet ./...
  step "go vet (relaykit)" bash -c 'cd relaykit && go vet ./...'
  step "go vet (tokenkit)" bash -c 'cd tokenkit && go vet ./...'
  step "go build (root)" go build ./...
  step "go build (relaykit)" bash -c 'cd relaykit && go build ./...'
  step "go build (tokenkit)" bash -c 'cd tokenkit && go build ./...'
  step "go test (make test)" make test
fi

if [ "$PART" != backend ]; then
  step "bun install" bash -c 'cd web && bun install --frozen-lockfile'
  step "web typecheck" bash -c 'cd web && bun run typecheck'
  step "web test" bash -c 'cd web && bun run test'
fi

echo
printf '%s\n' "${RESULTS[@]}"
if [ "$FAILED" -ne 0 ]; then
  echo "CI FAILED"
  exit 1
fi
echo "CI PASSED"
