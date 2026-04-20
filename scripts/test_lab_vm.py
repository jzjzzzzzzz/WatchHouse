import importlib.util
from pathlib import Path
import unittest
from unittest import mock
import subprocess
import json
from types import SimpleNamespace

spec = importlib.util.spec_from_file_location("lab_vm", Path(__file__).with_name("lab_vm.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class BootstrapTests(unittest.TestCase):
    def test_ssh_bootstrap_is_key_only(self):
        config = module.cloud_config("synthetic-public", "synthetic-private", "synthetic-host-public")
        self.assertFalse(config["ssh_pwauth"])
        self.assertTrue(config["disable_root"])
        self.assertEqual(config["ssh_keys"]["ed25519_public"], "synthetic-host-public")
        self.assertEqual(config["users"][0]["ssh_authorized_keys"], ["synthetic-public"])
        self.assertTrue(config["users"][0]["lock_passwd"])

    def test_client_does_not_use_personal_agent_or_disable_host_validation(self):
        args = module.ssh_args({"port": 12345})
        self.assertIn("StrictHostKeyChecking=yes", args)
        self.assertIn("IdentityAgent=none", args)
        self.assertIn("IdentitiesOnly=yes", args)
        self.assertIn("ForwardAgent=no", args)
        self.assertEqual(args[args.index("-F") + 1], "/dev/null")
        self.assertEqual(args[-1], "watchhouse-lab@127.0.0.1")

    def test_running_guest_cannot_be_restarted(self):
        with mock.patch.object(module, "inspect", return_value=({"container": "synthetic"}, {"State": {"Running": True, "Restarting": False}})):
            with mock.patch.object(module, "run") as execute:
                with self.assertRaises(ValueError):
                    module.restart()
                execute.assert_not_called()

    def test_probe_timeout_revalidates_state_without_restart(self):
        metadata = ({"container": "synthetic", "port": 12345}, {"State": {"Running": True}})
        with mock.patch.object(module, "inspect", return_value=metadata) as inspect:
            with mock.patch.object(module.subprocess, "run", side_effect=subprocess.TimeoutExpired("ssh", 15)):
                result = module.probe()
        self.assertTrue(result["running"])
        self.assertFalse(result["ssh_ready"])
        self.assertEqual(inspect.call_count, 2)

    def test_tools_label_checked_before_mounting_private_state(self):
        info = [{"Config": {"Labels": None}, "Id": "synthetic-untrusted"}]
        with mock.patch.object(module, "run", return_value=SimpleNamespace(stdout=json.dumps(info))) as execute:
            with self.assertRaises(ValueError):
                module.tools(["cp", "source", "destination"])
        self.assertEqual(execute.call_count, 1)
        self.assertEqual(execute.call_args.args[0][:3], ["docker", "image", "inspect"])

    def test_personal_identity_path_cannot_be_selected(self):
        with self.assertRaises(ValueError):
            module.ssh_args({"port": 12345}, "../../.ssh/id_ed25519")
