import hashlib
import json
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]


class DockerPortsIntegrationTests(unittest.TestCase):
    def test_images_are_digest_pinned(self):
        for specification in (ROOT / "lab/docker/images.json", ROOT / "lab/nginx/images.json"):
            data = json.loads(specification.read_text())
            for value in data.values():
                if isinstance(value, dict) and "reference" in value:
                    self.assertRegex(value["reference"], r"@sha256:[0-9a-f]{64}$")

    def test_runner_uses_no_shell_and_preserves_safety_controls(self):
        body = (ROOT / "scripts/docker-ports-integration.py").read_text()
        self.assertNotIn("shell=True", body)
        for token in ("--network=none", "--read-only", "--cap-drop=ALL", "no-new-privileges",
                      "--pids-limit", "--memory", "org.watchhouse.integration"):
            self.assertIn(token, body)
        self.assertRegex(body, r'if details\["Name"\] != "/" \+ name')

    def test_committed_result_binds_runner_hash(self):
        result = json.loads((ROOT / "evidence/test-runs/2026-10-06-docker/result.json").read_text())
        actual = hashlib.sha256((ROOT / "scripts/docker-ports-integration.py").read_bytes()).hexdigest()
        self.assertEqual(result["runner_sha256"], actual)
        self.assertTrue(result["passed"])
        self.assertEqual(result["fixture"]["host_ip"], "127.0.0.1")
        self.assertTrue(re.fullmatch(r"[0-9a-f]{64}", result["fixture"]["container_id"]))


if __name__ == "__main__":
    unittest.main()
