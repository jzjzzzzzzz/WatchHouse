import importlib.util
from pathlib import Path
import unittest

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
