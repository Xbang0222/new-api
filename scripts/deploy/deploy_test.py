"""Exercise the production deploy and rollback payloads against a simulated Docker host."""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

# A tiny Docker: containers live in a JSON file so the payload's renames,
# stops and starts are observable and the health check can depend on them.
DOCKER = r"""#!/usr/bin/env python3
import json, os, sys
args = sys.argv[1:]
state_path = os.environ['TEST_STATE']
state = json.load(open(state_path))
with open(os.environ['TEST_CALLS'], 'a') as log:
    log.write(json.dumps(args) + '\n')
def save():
    json.dump(state, open(state_path, 'w'))
def container(name):
    if name not in state:
        sys.stderr.write('No such container: ' + name + '\n')
        sys.exit(1)
    return state[name]
cmd = args[0]
if cmd == 'inspect':
    template = args[args.index('-f') + 1] if '-f' in args else ''
    c = container(args[-1])
    if 'Config.Image' in template:
        print(c['image'])
    elif 'Config.Env' in template:
        print('SQL_DSN=test:fixture@tcp(localhost:3306)/fixture')
        print('SHUTDOWN_TIMEOUT_SECONDS=120')
    elif 'State.Running' in template:
        print('true' if c['running'] else 'false')
    elif 'State.ExitCode' in template:
        print(c.get('exit', 0))
    else:
        print('{}')
elif cmd == 'stop':
    c = container(args[-1])
    if not os.environ.get('TEST_STOP_IGNORED'):
        c['running'] = False
        c['exit'] = int(os.environ.get('TEST_EXIT_CODE', '0'))
    save()
elif cmd == 'start':
    container(args[-1])['running'] = True
    save()
elif cmd == 'rm':
    state.pop(args[-1], None)
    save()
elif cmd == 'rename':
    if args[2] in state:
        sys.exit(1)
    state[args[2]] = state.pop(args[1])
    save()
elif cmd == 'update':
    container(args[-1])['restart'] = args[args.index('--restart') + 1]
    save()
elif cmd == 'run':
    name = args[args.index('--name') + 1]
    if name in state:
        sys.exit(125)
    env_file = args[args.index('--env-file') + 1]
    open(os.environ['TEST_ENV_COPY'], 'w').write(open(env_file).read())
    if os.environ.get('TEST_RUN_FAILS'):
        sys.exit(125)
    state[name] = {'image': args[-1], 'running': True, 'restart': args[args.index('--restart') + 1]}
    save()
"""

# Healthy when the serving container runs one of TEST_HEALTHY images.
CURL = r"""#!/usr/bin/env python3
import json, os, sys
c = json.load(open(os.environ['TEST_STATE'])).get('new-api')
healthy = os.environ.get('TEST_HEALTHY', 'v-old,v-next,v-prev').split(',')
if c and c['running'] and c['image'].split(':')[-1] in healthy:
    print('{"success":true}')
else:
    sys.exit(7)
"""

BACKUPS = {
    "ok": 'echo "$(date) ok 6s: stored new_api-x.sql.gz 1 bytes"',
    "fail": "echo rejected >&2; exit 1",
    "skipping": 'echo "$(date) previous run still active, skipping"',
    # Reads its stdin, like an ssh or docker exec -i without redirection would.
    "reads-stdin": 'cat >/dev/null; echo "$(date) ok 6s: stored new_api-x.sql.gz 1 bytes"',
}


def payload(name, base):
    source = Path(__file__).with_name("deploy.sh").read_text()
    body = re.search(
        r"read -r -d '' " + name + r" <<'REMOTE' \|\| true\n(.*?)\nREMOTE",
        source,
        re.S,
    ).group(1)
    body = body.replace("BASE=/opt/new-api", "BASE=" + str(base / "data"))
    # Keep the health deadline short; the logic under test is the same.
    return body.replace("HEALTH_SECONDS=30", "HEALTH_SECONDS=2")


def image(tag):
    return {"image": "xbang0222/new-api:" + tag, "running": True, "restart": "unless-stopped"}


class DeployPayloadTest(unittest.TestCase):
    def run_payload(self, name, containers, args=(), backup="ok", **env_overrides):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            commands = base / "commands"
            commands.mkdir()
            (base / "data").mkdir()
            (base / "state").write_text(json.dumps(containers))
            stubs = {
                "docker": DOCKER,
                "curl": CURL,
                "flock": "#!/bin/sh\nexit 0\n",
                # Skip the 30s backup retry wait; keep short health-check waits real.
                "sleep": '#!/bin/sh\n[ "$1" = 30 ] && exit 0\nexec /bin/sleep "$@"\n',
                "backup": "#!/bin/sh\n" + BACKUPS[backup] + "\n",
            }
            for stub, body in stubs.items():
                path = commands / stub
                path.write_text(body)
                path.chmod(0o755)
            env = {
                **os.environ,
                "PATH": str(commands) + os.pathsep + os.environ["PATH"],
                "TEST_STATE": str(base / "state"),
                "TEST_CALLS": str(base / "calls"),
                "TEST_ENV_COPY": str(base / "env-copy"),
                "OFFSITE_BACKUP": str(commands / "backup"),
                "OFFSITE_LOCK": str(base / "lock"),
                **{"TEST_" + key.upper(): str(value) for key, value in env_overrides.items()},
            }
            # Feed the payload on stdin exactly as `ssh ... bash -s` does.
            result = subprocess.run(["bash", "-s", "--", *args], input=payload(name, base),
                                    text=True, capture_output=True, env=env, timeout=60)
            calls_file = base / "calls"
            calls = [json.loads(line) for line in calls_file.read_text().splitlines()] if calls_file.exists() else []
            env_copy = (base / "env-copy").read_text() if (base / "env-copy").exists() else ""
            final = json.loads((base / "state").read_text())
            return result, calls, env_copy, final

    def deploy(self, **kwargs):
        return self.run_payload("PROD_SCRIPT", {"new-api": image("v-old")}, ["v-next"], **kwargs)

    def assert_serving(self, final, tag):
        self.assertEqual(final["new-api"]["image"], "xbang0222/new-api:" + tag)
        self.assertTrue(final["new-api"]["running"])
        self.assertEqual(final["new-api"]["restart"], "unless-stopped")

    def test_switches_with_bounded_waits_and_short_app_drain(self):
        result, calls, env_copy, final = self.deploy()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(["stop", "-t", "20", "new-api"], calls)
        self.assert_serving(final, "v-next")
        self.assertFalse(final["new-api-prev"]["running"])
        self.assertEqual(final["new-api-prev"]["restart"], "no")
        self.assertIn("SHUTDOWN_TIMEOUT_SECONDS=15", env_copy.splitlines())
        self.assertNotIn("SHUTDOWN_TIMEOUT_SECONDS=120", env_copy)
        self.assertIn("SQL_DSN=", env_copy)
        self.assertFalse(any(call[0] == "exec" for call in calls), "no local MySQL dump")

    def test_backup_reading_stdin_does_not_swallow_the_deployment(self):
        result, _, _, final = self.deploy(backup="reads-stdin")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_serving(final, "v-next")

    def test_old_container_killed_at_deadline_still_upgrades(self):
        result, _, _, final = self.deploy(exit_code=137)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_serving(final, "v-next")

    def test_old_container_still_running_aborts(self):
        result, calls, _, final = self.deploy(stop_ignored=1)
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(call[0] in {"rename", "run"} for call in calls))
        self.assert_serving(final, "v-old")

    def test_failed_or_skipped_backup_aborts_before_stopping_production(self):
        for backup in ["fail", "skipping"]:
            with self.subTest(backup=backup):
                result, calls, _, final = self.deploy(backup=backup)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("aborting before any change", result.stderr)
                self.assertFalse(any(call[0] in {"stop", "rename", "run"} for call in calls))
                self.assert_serving(final, "v-old")

    def test_unhealthy_new_version_rolls_back_to_serving_old_version(self):
        result, _, _, final = self.deploy(healthy="v-old")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("rolled back after", result.stderr)
        self.assert_serving(final, "v-old")
        self.assertNotIn("new-api-prev", final)

    def test_new_container_that_cannot_start_rolls_back(self):
        result, _, _, final = self.deploy(run_fails=1)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("rolled back after", result.stderr)
        self.assert_serving(final, "v-old")

    def rollback(self, containers, **kwargs):
        return self.run_payload("ROLLBACK_SCRIPT", containers, **kwargs)

    def after_deploy(self):
        prev = image("v-old")
        prev.update(running=False, restart="no")
        return {"new-api": image("v-next"), "new-api-prev": prev}

    def test_manual_rollback_restores_previous_container(self):
        result, calls, _, final = self.rollback(self.after_deploy())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(["stop", "-t", "20", "new-api"], calls)
        self.assert_serving(final, "v-old")
        self.assertEqual(final["new-api-rolled-back"]["image"], "xbang0222/new-api:v-next")
        self.assertEqual(final["new-api-rolled-back"]["restart"], "no")

    def test_manual_rollback_after_interrupted_deploy_starts_previous(self):
        containers = self.after_deploy()
        del containers["new-api"]
        result, _, _, final = self.rollback(containers)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assert_serving(final, "v-old")

    def test_failed_manual_rollback_restores_what_was_serving(self):
        result, _, _, final = self.rollback(self.after_deploy(), healthy="v-next")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("rollback abandoned", result.stderr)
        self.assert_serving(final, "v-next")
        self.assertEqual(final["new-api-prev"]["image"], "xbang0222/new-api:v-old")

    def test_manual_rollback_without_previous_container_changes_nothing(self):
        result, calls, _, final = self.rollback({"new-api": image("v-next")})
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(call[0] in {"stop", "rename", "start"} for call in calls))
        self.assert_serving(final, "v-next")


if __name__ == "__main__":
    unittest.main()
