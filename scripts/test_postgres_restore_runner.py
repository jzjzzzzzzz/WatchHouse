import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, str(Path(__file__).parent))
spec = importlib.util.spec_from_file_location("postgres_restore_runner", Path(__file__).with_name("postgres-restore-test.py"))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


class RestoreRunnerTests(unittest.TestCase):
    def test_inspect_requires_running_labeled_container(self):
        cases = [
            ({"Config": {"Labels": {}}, "State": {"Running": True}}, "lacks"),
            ({"Config": {"Labels": {"org.watchhouse.integration": "control-postgres"}}, "State": {"Running": False}}, "not running"),
        ]
        for details, message in cases:
            process = subprocess.CompletedProcess([], 0, json.dumps([details]), "")
            with mock.patch.object(runner.subprocess, "run", return_value=process):
                with self.assertRaisesRegex(RuntimeError, message):
                    runner.inspect_container("pg-test")

    def test_main_reconciles_restore_and_writes_manifest(self):
        counts = {table: index for index, table in enumerate(runner.postgres_restore.TABLES)}
        with tempfile.TemporaryDirectory() as directory, \
             mock.patch.object(runner, "inspect_container"), \
             mock.patch.object(runner.postgres_restore, "table_counts", side_effect=[counts, counts]), \
             mock.patch.object(runner.postgres_restore, "dump_database", return_value={"bytes": 5, "sha256": "a" * 64}), \
             mock.patch.object(runner.postgres_restore, "restore_database"):
            output = Path(directory) / "evidence"
            code = runner.main(["--container", "watchhouse-postgres-test", "--output", str(output)])
            self.assertEqual(code, 0)
            report = json.loads((output / "postgres-restore-result.json").read_text())
            self.assertTrue(report["passed"])
            self.assertEqual(report["source_counts"], counts)
            self.assertEqual((output / "postgres-restore-result.json").stat().st_mode & 0o777, 0o600)


if __name__ == "__main__":
    unittest.main()
