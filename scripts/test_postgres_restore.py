from pathlib import Path
import importlib.util
import subprocess
import tempfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location("postgres_restore", Path(__file__).with_name("postgres_restore.py"))
postgres_restore = importlib.util.module_from_spec(spec)
spec.loader.exec_module(postgres_restore)


class PostgresRestoreTests(unittest.TestCase):
    def test_exec_uses_fixed_docker_prefix_and_binary_capture(self):
        run = mock.Mock()
        run.return_value = subprocess.CompletedProcess([], 0, b"ok", b"")
        with mock.patch.object(postgres_restore.subprocess, "run", run):
            result = postgres_restore.docker_exec("pg-test", ["psql", "-X"])
        self.assertEqual(result.stdout, b"ok")
        run.assert_called_once_with(
            ["docker", "exec", "-i", "pg-test", "psql", "-X"],
            input=None, check=True, capture_output=True, timeout=120,
        )

    def test_exec_rejects_empty_tokens(self):
        for container, argv in (("", ["psql"]), ("pg", []), ("pg", [""])):
            with self.assertRaises(ValueError):
                postgres_restore.docker_exec(container, argv)

    def test_table_counts_are_allowlisted_and_numeric(self):
        execute = mock.Mock()
        execute.side_effect = [subprocess.CompletedProcess([], 0, str(i).encode(), b"")
                               for i in range(len(postgres_restore.TABLES))]
        with mock.patch.object(postgres_restore, "docker_exec", execute):
            counts = postgres_restore.table_counts("pg", "restored")
        self.assertEqual(list(counts), list(postgres_restore.TABLES))
        self.assertEqual(counts[postgres_restore.TABLES[-1]], len(postgres_restore.TABLES) - 1)
        for call, table in zip(execute.call_args_list, postgres_restore.TABLES):
            self.assertEqual(call.args[1][-1], "SELECT count(*) FROM " + table)

    def test_dump_requires_custom_archive_and_writes_private_file(self):
        execute = mock.Mock()
        execute.return_value = subprocess.CompletedProcess([], 0, b"PGDMPpayload", b"")
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "backup.dump"
            with mock.patch.object(postgres_restore, "docker_exec", execute):
                manifest = postgres_restore.dump_database("pg", "watchhouse", destination)
            self.assertEqual(destination.read_bytes(), b"PGDMPpayload")
            self.assertEqual(destination.stat().st_mode & 0o777, 0o600)
            self.assertEqual(manifest["bytes"], 12)
            self.assertEqual(len(manifest["sha256"]), 64)

    def test_dump_rejects_non_archive_output(self):
        execute = mock.Mock()
        execute.return_value = subprocess.CompletedProcess([], 0, b"error page", b"")
        with tempfile.TemporaryDirectory() as directory:
            with mock.patch.object(postgres_restore, "docker_exec", execute):
                with self.assertRaises(RuntimeError):
                    postgres_restore.dump_database("pg", "watchhouse", Path(directory) / "backup")


if __name__ == "__main__":
    unittest.main()
