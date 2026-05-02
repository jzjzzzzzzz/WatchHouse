#!/usr/bin/env python3
"""Run control-store tests against one pinned, disposable local PostgreSQL."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time
import urllib.parse

ROOT = Path(__file__).resolve().parents[1]
SPEC = json.loads((ROOT / "lab/postgres/images.json").read_text())["postgres"]
LABEL = "org.watchhouse.integration=control-postgres"


def run(argv, **kwargs):
    return subprocess.run(argv, check=True, capture_output=True, text=True,
                          timeout=kwargs.pop("timeout", 120), **kwargs)


def main():
    go = os.environ.get("GO", "go")
    source_commit = run(["git", "rev-parse", "HEAD"], cwd=ROOT).stdout.strip()
    source_dirty = bool(run(["git", "status", "--porcelain"], cwd=ROOT).stdout.strip())
    run(["docker", "pull", SPEC["reference"]], timeout=300)
    image = json.loads(run(["docker", "image", "inspect", SPEC["reference"]]).stdout)[0]
    name = "watchhouse-postgres-" + secrets.token_hex(6)
    password = secrets.token_urlsafe(32)
    container = None
    started = time.monotonic()
    local = ROOT / "lab/local"
    local.mkdir(mode=0o700, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="postgres-", dir=local) as directory:
        secret_path = Path(directory) / "password"
        secret_path.write_text(password)
        secret_path.chmod(0o600)
        try:
            args = ["docker", "run", "-d", "--name", name, "--label", LABEL,
                    "--memory", "1g", "--cpus", "2", "--pids-limit", "256",
                    "--publish", "127.0.0.1::5432",
                    "--tmpfs", "/var/lib/postgresql:rw,nosuid,nodev,size=512m",
                    "--tmpfs", "/var/run/postgresql:rw,nosuid,nodev,size=16m",
                    "--mount", "type=bind,source=" + str(secret_path) + ",target=/run/secrets/postgres_password,readonly",
                    "--env", "POSTGRES_PASSWORD_FILE=/run/secrets/postgres_password",
                    "--env", "POSTGRES_USER=watchhouse", "--env", "POSTGRES_DB=watchhouse",
                    SPEC["reference"]]
            container = run(args).stdout.strip()
            deadline = time.monotonic() + 120
            while time.monotonic() < deadline:
                probe = subprocess.run(["docker", "exec", container, "pg_isready", "-U", "watchhouse", "-d", "watchhouse"],
                                       capture_output=True, text=True, timeout=10)
                if probe.returncode == 0:
                    break
                details = json.loads(run(["docker", "inspect", container]).stdout)[0]
                if not details["State"]["Running"]:
                    raise RuntimeError("PostgreSQL integration container stopped")
                time.sleep(1)
            else:
                raise TimeoutError("PostgreSQL integration readiness timed out")
            details = json.loads(run(["docker", "inspect", container]).stdout)[0]
            binding = details["NetworkSettings"]["Ports"]["5432/tcp"]
            if len(binding) != 1 or binding[0]["HostIp"] != "127.0.0.1":
                raise RuntimeError("PostgreSQL was not exclusively loopback-published")
            port = int(binding[0]["HostPort"])
            dsn = "postgres://watchhouse:" + urllib.parse.quote(password, safe="") + "@127.0.0.1:" + str(port) + "/watchhouse?sslmode=disable"
            environment = dict(os.environ, WATCHHOUSE_TEST_POSTGRES_DSN=dsn)
            tested = run([go, "test", "-count=1", "-run", "TestPostgresIntegration", "./internal/controlstore"],
                         cwd=ROOT, env=environment, timeout=180)
            version = run(["docker", "exec", container, "psql", "-U", "watchhouse", "-d", "watchhouse", "-Atqc", "SHOW server_version"]).stdout.strip()
            report = {"type": "postgres_integration", "passed": True,
                      "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                      "elapsed_seconds": round(time.monotonic() - started, 2),
                      "source_commit": source_commit, "worktree_dirty_at_start": source_dirty,
                      "runner_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
                      "image_reference": SPEC["reference"], "image_id": image["Id"],
                      "platform": image.get("Architecture"), "postgres_version": version,
                      "network_exposure": "dynamic 127.0.0.1-only port",
                      "database_storage": "disposable bounded tmpfs",
                      "tests": ["idempotent migration", "idempotent repeat", "atomic conflict rollback", "eight-way concurrent repeat"],
                      "test_output": tested.stdout.strip(),
                      "scope": "local disposable database; loopback connection is not production database TLS"}
            output = ROOT / "lab/local/postgres.result.json"
            output.write_text(json.dumps(report, indent=2) + "\n")
            output.chmod(0o600)
            print(json.dumps(report))
        finally:
            if container:
                inspected = subprocess.run(["docker", "inspect", container], capture_output=True, text=True, timeout=30)
                if inspected.returncode == 0:
                    details = json.loads(inspected.stdout)[0]
                    if details["Name"] != "/" + name or details["Config"]["Labels"].get("org.watchhouse.integration") != "control-postgres":
                        raise RuntimeError("refusing to remove container with mismatched identity")
                    run(["docker", "rm", "-f", container], timeout=60)


if __name__ == "__main__":
    main()
