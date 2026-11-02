"""Exercise the production deployment payload with isolated command boundaries."""

import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest


class ProductionShutdownTest(unittest.TestCase):
    def deploy(self, timeout=None, exit_code=0):
        with tempfile.TemporaryDirectory() as directory:
            base = Path(directory)
            commands = base / "commands"
            commands.mkdir()
            docker = commands / "docker"
            docker.write_text("""#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
args = sys.argv[1:]
with open(os.environ['TEST_CALLS'], 'a') as log:
    log.write(json.dumps(args) + '\\n')
if args[0] == 'inspect':
    template = args[args.index('-f') + 1] if '-f' in args else ''
    if 'Config.Image' in template:
        print('xbang0222/new-api:v-old')
    elif 'Config.Env' in template:
        print('SQL_DSN=test:fixture@tcp(localhost:3306)/fixture')
        if 'TEST_DRAIN' in os.environ:
            print('SHUTDOWN_TIMEOUT_SECONDS=' + os.environ['TEST_DRAIN'])
    elif 'State.ExitCode' in template:
        print(os.environ['TEST_EXIT_CODE'])
    elif 'State.Running' in template:
        print('true')
    else:
        print('{}')
elif args[0] == 'exec':
    sys.stdout.buffer.write(bytes(range(256)) * 128)
""")
            docker.chmod(0o755)
            for name, body in {
                "curl": "printf '%s' '{\"success\":true}'",
                "stat": "printf '32768\\n'",
            }.items():
                command = commands / name
                command.write_text("#!/bin/sh\n" + body + "\n")
                command.chmod(0o755)
            source = Path(__file__).with_name("deploy.sh").read_text()
            payload = re.search(
                r"read -r -d '' PROD_SCRIPT <<'REMOTE' \|\| true\n(.*?)\nREMOTE",
                source,
                re.S,
            ).group(1)
            payload = payload.replace("BASE=/opt/new-api", "BASE=" + str(base / "data"))
            env = {**os.environ, "PATH": str(commands) + os.pathsep + os.environ["PATH"],
                   "TEST_CALLS": str(base / "calls"), "TEST_EXIT_CODE": str(exit_code)}
            if timeout is not None:
                env["TEST_DRAIN"] = timeout
            result = subprocess.run(["bash", "-s", "--", "v-next"], input=payload,
                                    text=True, capture_output=True, env=env, timeout=10)
            calls = [json.loads(line) for line in (base / "calls").read_text().splitlines()]
            return result, calls

    def test_default_and_custom_drain_finish_before_container_deadline(self):
        for timeout, expected in [(None, "180"), ("300", "360")]:
            with self.subTest(timeout=timeout):
                result, calls = self.deploy(timeout)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertIn(["stop", "-t", expected, "new-api"], calls)
                self.assertTrue(any(call[0] == "run" for call in calls))

    def test_forced_exit_restores_current_container_and_aborts_upgrade(self):
        result, calls = self.deploy(exit_code=137)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(["start", "new-api"], calls)
        self.assertFalse(any(call[0] in {"rename", "run"} for call in calls))
        self.assertIn("aborting", result.stderr)

    def test_invalid_drain_aborts_before_stopping_production(self):
        for timeout in ["-1", "0", "invalid", "99999999999999999999"]:
            with self.subTest(timeout=timeout):
                result, calls = self.deploy(timeout)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(any(call[0] in {"stop", "rename", "run"} for call in calls))


if __name__ == "__main__":
    unittest.main()
