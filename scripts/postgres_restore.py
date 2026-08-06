#!/usr/bin/env python3
"""Helpers for a bounded PostgreSQL logical backup/restore acceptance test."""
import hashlib
import json
from pathlib import Path
import re
import subprocess

TABLES = (
    "control_events",
    "control_findings",
    "control_listener_snapshots",
    "control_probe_observations",
    "control_query_audit",
    "control_schema_migrations",
)
IDENTIFIER = re.compile(r"^[a-z][a-z0-9_]{0,62}$")


def docker_exec(container, argv, *, input_text=None, timeout=120):
    if not container or not argv or any(not isinstance(item, str) or not item for item in argv):
        raise ValueError("container and argv must be non-empty strings")
    return subprocess.run(
        ["docker", "exec", "-i", container, *argv],
        input=input_text,
        check=True,
        capture_output=True,
        timeout=timeout,
    )


def table_counts(container, database):
    counts = {}
    for table in TABLES:
        sql = "SELECT count(*) FROM " + table
        result = docker_exec(container, ["psql", "-X", "-U", "watchhouse", "-d", database, "-Atqc", sql])
        value = result.stdout.decode("ascii").strip()
        if not value.isdigit():
            raise RuntimeError(f"non-numeric count for {table}")
        counts[table] = int(value)
    return counts


def dump_database(container, database, destination):
    destination = Path(destination)
    process = docker_exec(container, ["pg_dump", "-U", "watchhouse", "-d", database,
                                      "--format=custom", "--compress=6", "--no-owner", "--no-privileges"], timeout=300)
    if not process.stdout.startswith(b"PGDMP"):
        raise RuntimeError("pg_dump did not produce a custom-format archive")
    destination.write_bytes(process.stdout)
    destination.chmod(0o600)
    return {"bytes": len(process.stdout), "sha256": hashlib.sha256(process.stdout).hexdigest()}


def restore_database(container, database, archive):
    if not IDENTIFIER.fullmatch(database):
        raise ValueError("restore database name is not a safe identifier")
    archive = Path(archive)
    body = archive.read_bytes()
    if len(body) < 5 or not body.startswith(b"PGDMP"):
        raise ValueError("restore input is not a PostgreSQL custom archive")
    docker_exec(container, ["dropdb", "-U", "watchhouse", "--if-exists", database])
    docker_exec(container, ["createdb", "-U", "watchhouse", "--template=template0", database])
    docker_exec(container, ["pg_restore", "-U", "watchhouse", "-d", database,
                            "--exit-on-error", "--no-owner", "--no-privileges"],
                input_text=body, timeout=300)


def compare_counts(source, restored):
    missing = set(TABLES) - set(source) | (set(TABLES) - set(restored))
    if missing:
        raise ValueError("table count maps are incomplete: " + ",".join(sorted(missing)))
    differences = {table: {"source": source[table], "restored": restored[table]}
                   for table in TABLES if source[table] != restored[table]}
    return differences


def write_manifest(path, payload):
    encoded = json.dumps(payload, sort_keys=True, indent=2) + "\n"
    path = Path(path)
    path.write_text(encoded)
    path.chmod(0o600)
