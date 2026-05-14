import configparser
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1]


class ServiceUnitTests(unittest.TestCase):
    def test_initial_collector_is_nonroot_and_sandboxed(self):
        config = configparser.ConfigParser(interpolation=None, strict=False)
        config.read(ROOT / "deploy/systemd/watchhouse-collect.service")
        service = config["Service"]
        self.assertEqual(service["User"], "watchhouse")
        self.assertEqual(service["SupplementaryGroups"], "systemd-journal")
        self.assertEqual(service["NoNewPrivileges"], "yes")
        self.assertEqual(service["CapabilityBoundingSet"], "")
        self.assertEqual(service["AmbientCapabilities"], "")
        self.assertEqual(service["ProtectSystem"], "strict")
        self.assertEqual(service["ProtectHome"], "yes")
        self.assertEqual(service["ReadWritePaths"], "/var/lib/watchhouse")
        self.assertEqual(service["StateDirectoryMode"], "0700")
        self.assertEqual(service["RestrictAddressFamilies"], "AF_UNIX")
        self.assertNotIn("sudo", service["ExecStart"])
        self.assertNotIn("/bin/sh", service["ExecStart"])

    def test_timer_waits_for_finished_oneshot(self):
        config = configparser.ConfigParser(interpolation=None)
        config.read(ROOT / "deploy/systemd/watchhouse-collect.timer")
        self.assertEqual(config["Timer"]["OnUnitInactiveSec"], "10s")
        self.assertEqual(config["Timer"]["Unit"], "watchhouse-collect.service")

    def test_delivery_adds_only_outbound_network_and_credential_files(self):
        config = configparser.ConfigParser(interpolation=None, strict=False)
        config.read(ROOT / "deploy/systemd/watchhouse-deliver.service")
        service = config["Service"]
        self.assertEqual(service["User"], "watchhouse")
        self.assertEqual(service["NoNewPrivileges"], "yes")
        self.assertEqual(service["CapabilityBoundingSet"], "")
        self.assertEqual(service["ProtectSystem"], "strict")
        self.assertEqual(service["ReadWritePaths"], "/var/lib/watchhouse")
        self.assertEqual(service["RestrictAddressFamilies"], "AF_UNIX AF_INET AF_INET6")
        self.assertNotIn("systemd-journal", service.get("SupplementaryGroups", ""))
        self.assertNotIn("sudo", service["ExecStart"])
        self.assertNotIn("/bin/sh", service["ExecStart"])
        unit_text = (ROOT / "deploy/systemd/watchhouse-deliver.service").read_text()
        self.assertEqual(unit_text.count("\nLoadCredential="), 3)
        self.assertIn("${CREDENTIALS_DIRECTORY}/agent-key.pem", service["ExecStart"])

    def test_delivery_timer_is_nonoverlapping(self):
        config = configparser.ConfigParser(interpolation=None)
        config.read(ROOT / "deploy/systemd/watchhouse-deliver.timer")
        self.assertEqual(config["Timer"]["OnUnitInactiveSec"], "15s")
        self.assertEqual(config["Timer"]["Unit"], "watchhouse-deliver.service")

    def test_control_uses_dynamic_user_credentials_and_no_writable_host_path(self):
        config = configparser.ConfigParser(interpolation=None, strict=False)
        path = ROOT / "deploy/systemd/watchhouse-control.service"
        config.read(path)
        service = config["Service"]
        self.assertEqual(service["DynamicUser"], "yes")
        self.assertEqual(service["NoNewPrivileges"], "yes")
        self.assertEqual(service["CapabilityBoundingSet"], "")
        self.assertEqual(service["ProtectSystem"], "strict")
        self.assertEqual(service["ProtectHome"], "yes")
        self.assertNotIn("ReadWritePaths", service)
        self.assertEqual(service["RestrictAddressFamilies"], "AF_UNIX AF_INET AF_INET6")
        self.assertEqual(path.read_text().count("\nLoadCredential="), 5)
        self.assertIn("${CREDENTIALS_DIRECTORY}/database-url", service["ExecStart"])
        self.assertIn("${CREDENTIALS_DIRECTORY}/roles.json", service["ExecStart"])
        self.assertNotIn("sudo", service["ExecStart"])
        self.assertNotIn("/bin/sh", service["ExecStart"])
