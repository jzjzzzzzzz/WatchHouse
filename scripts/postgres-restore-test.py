#!/usr/bin/env python3
"""Prove a Watchhouse logical backup restores into an isolated database."""
import argparse
import datetime
import json
from pathlib import Path
import subprocess
import tempfile

import postgres_restore

EXPECTED_LABEL = "org.watchhouse.integration=control-postgres"


def inspect_container(container):
    details = json.loads(subprocess.run(
        ["docker", "inspect", container], check=True, capture_output=True, text=True, timeout=30,
    ).stdout)
    if len(details) != 1 or details[0]["Config"]["Labels"].get("org.watchhouse.integration") != "control-postgres":
        raise RuntimeError("container lacks the Watchhouse PostgreSQL integration label")
    if not details[0]["State"]["Running"]:
        raise RuntimeError("PostgreSQL container is not running")


def main(argv=None):
    parser = argparse.ArgumentParser()
    parser.add_argument("--container", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)
    if not postgres_restore.IDENTIFIER.fullmatch(args.container.replace("-", "_")):
        parser.error("container name must be a bounded identifier")
    inspect_container(args.container)
    args.output.mkdir(mode=0o700, parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="restore-", dir=args.output) as directory:
        archive = Path(directory) / "watchhouse.dump"
        source = postgres_restore.table_counts(args.container, "watchhouse")
        artifact = postgres_restore.dump_database(args.container, "watchhouse", archive)
        postgres_restore.restore_database(args.container, "watchhouse_restore", archive)
        restored = postgres_restore.table_counts(args.container, "watchhouse_restore")
        differences = postgres_restore.compare_counts(source, restored)
        report = {
            "schema_version": 1,
            "type": "postgres_restore_test",
            "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "source_database": "watchhouse",
            "restore_database": "watchhouse_restore",
            "archive": artifact,
            "source_counts": source,
            "restored_counts": restored,
            "differences": differences,
            "passed": not differences,
        }
        postgres_restore.write_manifest(args.output / "postgres-restore-result.json", report)
        print(json.dumps(report, sort_keys=True))
        return 0 if not differences else 1


if __name__ == "__main__":
    raise SystemExit(main())
