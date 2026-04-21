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
