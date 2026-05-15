import tempfile
from pathlib import Path
import subprocess
import unittest

from scripts import integration_pki


class IntegrationPKITests(unittest.TestCase):
    def test_distinct_agent_and_human_uri_sans(self):
        with tempfile.TemporaryDirectory() as directory:
            files = integration_pki.generate(directory, host="host-1", viewer="viewer-1", unknown="unknown-1")
            for name, expected in (("agent_cert", "URI:spiffe://watchhouse/host/host-1"),
                                   ("viewer_cert", "URI:spiffe://watchhouse/user/viewer-1"),
                                   ("unknown_cert", "URI:spiffe://watchhouse/user/unknown-1")):
                text = subprocess.check_output(["openssl", "x509", "-in", str(files[name]), "-noout", "-text"], text=True)
                self.assertIn(expected, text)
            for name in ("agent_key", "viewer_key", "unknown_key", "server_key"):
                self.assertEqual(Path(files[name]).stat().st_mode & 0o777, 0o600)


if __name__ == "__main__":
    unittest.main()
