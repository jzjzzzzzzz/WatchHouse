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

    def test_restore_recreates_fixed_database_and_streams_archive(self):
        execute = mock.Mock(return_value=subprocess.CompletedProcess([], 0, b"", b""))
        with tempfile.TemporaryDirectory() as directory:
            archive = Path(directory) / "backup.dump"
            archive.write_bytes(b"PGDMPpayload")
            with mock.patch.object(postgres_restore, "docker_exec", execute):
                postgres_restore.restore_database("pg", "watchhouse_restore", archive)
        self.assertEqual(execute.call_count, 3)
        self.assertEqual(execute.call_args_list[0].args[1], ["dropdb", "-U", "watchhouse", "--if-exists", "watchhouse_restore"])
        self.assertEqual(execute.call_args_list[1].args[1], ["createdb", "-U", "watchhouse", "--template=template0", "watchhouse_restore"])
        self.assertEqual(execute.call_args_list[2].kwargs["input_text"], b"PGDMPpayload")

    def test_restore_rejects_identifier_and_archive_confusion(self):
        with tempfile.TemporaryDirectory() as directory:
            archive = Path(directory) / "backup.dump"
            archive.write_bytes(b"not-a-dump")
            with self.assertRaises(ValueError):
                postgres_restore.restore_database("pg", "watchhouse;DROP DATABASE x", archive)
            with self.assertRaises(ValueError):
                postgres_restore.restore_database("pg", "watchhouse_restore", archive)

    def test_compare_counts_returns_exact_differences(self):
        source = {table: 1 for table in postgres_restore.TABLES}
        restored = dict(source)
        restored[postgres_restore.TABLES[0]] = 0
        self.assertEqual(postgres_restore.compare_counts(source, restored), {
            postgres_restore.TABLES[0]: {"source": 1, "restored": 0}
        })
        del restored[postgres_restore.TABLES[-1]]
        with self.assertRaises(ValueError):
            postgres_restore.compare_counts(source, restored)


if __name__ == "__main__":
    unittest.main()
