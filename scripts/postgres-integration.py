#!/usr/bin/env python3
"""Run control-store tests against one pinned, disposable local PostgreSQL."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.parse

from integration_pki import generate

ROOT = Path(__file__).resolve().parents[1]
SPEC = json.loads((ROOT / "lab/postgres/images.json").read_text())["postgres"]
LABEL = "org.watchhouse.integration=control-postgres"


def run(argv, **kwargs):
    return subprocess.run(argv, check=True, capture_output=True, text=True,
                          timeout=kwargs.pop("timeout", 120), **kwargs)



def reserve_loopback_port():
    listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    listener.bind(("127.0.0.1", 0))
    port = listener.getsockname()[1]
    listener.close()
    return port


def main():
    go = os.environ.get("GO", "go")
    source_commit = run(["git", "rev-parse", "HEAD"], cwd=ROOT).stdout.strip()
    source_dirty = bool(run(["git", "status", "--porcelain"], cwd=ROOT).stdout.strip())
    run(["docker", "pull", SPEC["reference"]], timeout=300)
    image = json.loads(run(["docker", "image", "inspect", SPEC["reference"]]).stdout)[0]
    name = "watchhouse-postgres-" + secrets.token_hex(6)
    password = secrets.token_urlsafe(32)
    container = None
    control = None
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
            try:
                tested = run([go, "test", "-count=1", "-run", "Test(PostgresIntegration|EndToEndMutualTLSDeliveryPostgresAndReceiptRecovery)", "./internal/controlstore"],
                             cwd=ROOT, env=environment, timeout=180)
            except subprocess.CalledProcessError as error:
                failure = {"type": "postgres_integration_failure", "passed": False,
                           "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                           "source_commit": source_commit, "phase": "go_integration_tests",
                           "return_code": error.returncode,
                           "stdout": (error.stdout or "")[-16384:], "stderr": (error.stderr or "")[-16384:]}
                output = ROOT / "lab/local/postgres.last-failure.json"
                output.write_text(json.dumps(failure, indent=2) + "\n")
                output.chmod(0o600)
                print(json.dumps(failure))
                raise RuntimeError("PostgreSQL Go integration tests failed; private bounded diagnostics preserved") from error
            pki = generate(directory)
            database_url = Path(directory) / "database-url"
            database_url.write_text(dsn + "\n")
            database_url.chmod(0o600)
            roles = Path(directory) / "roles.json"
            roles.write_text(json.dumps({"schema_version": 1, "principals": {"alice": "viewer"}}) + "\n")
            roles.chmod(0o644)
            agent_binary = ROOT / "bin/watchhouse"
            control_binary = ROOT / "bin/watchhouse-control"
            run([go, "build", "-trimpath", "-o", str(agent_binary), "./cmd/watchhouse"], cwd=ROOT, timeout=180)
            run([go, "build", "-trimpath", "-o", str(control_binary), "./cmd/watchhouse-control"], cwd=ROOT, timeout=180)
            control_port = reserve_loopback_port()
            control_log_path = Path(directory) / "control.stderr"
            control_log = control_log_path.open("w")
            control = subprocess.Popen([str(control_binary), "--listen", "127.0.0.1:" + str(control_port),
                                        "--database-url-file", str(database_url), "--client-ca", str(pki["ca"]),
                                        "--roles-file", str(roles),
                                        "--tls-cert", str(pki["server_cert"]), "--tls-key", str(pki["server_key"]),
                                        "--server-name", "control.test"], cwd=ROOT, stdout=subprocess.DEVNULL,
                                       stderr=control_log, text=True)
            deadline = time.monotonic() + 30
            while time.monotonic() < deadline:
                if control.poll() is not None:
                    control_log.flush()
                    raise RuntimeError("control process exited before readiness: " + control_log_path.read_text()[-4096:])
                try:
                    probe = socket.create_connection(("127.0.0.1", control_port), timeout=1)
                    probe.close()
                    break
                except OSError:
                    time.sleep(0.2)
            else:
                raise TimeoutError("control process TCP readiness timed out")
            endpoint = "https://127.0.0.1:" + str(control_port)
            fixture = ROOT / "tests/fixtures/ssh-sequence.journal.jsonl"
            state = Path(directory) / "agent-state"
            run([str(agent_binary), "spool", "ingest", "--state", str(state), "--host", "process-host", "--input", str(fixture)])
            before = json.loads(run([str(agent_binary), "spool", "status", "--state", str(state)]).stdout)
            pending_before = before["stats"]["pending_records"]
            if pending_before < 1:
                raise RuntimeError("process integration fixture produced no queued events")
            delivered = json.loads(run([str(agent_binary), "deliver", "--state", str(state), "--endpoint", endpoint,
                                        "--ca", str(pki["ca"]), "--cert", str(pki["agent_cert"]),
                                        "--key", str(pki["agent_key"]), "--server-name", "control.test"]).stdout)
            after = json.loads(run([str(agent_binary), "spool", "status", "--state", str(state)]).stdout)
            if delivered["authenticated_host"] != "process-host" or delivered["result"]["acknowledged"] != pending_before or after["stats"]["pending_records"] != 0:
                raise RuntimeError("process-level exact delivery did not drain the spool")
            remote_count = int(run(["docker", "exec", container, "psql", "-U", "watchhouse", "-d", "watchhouse", "-Atqc",
                                    "SELECT count(*) FROM control_events WHERE host_id='process-host'"]).stdout.strip())
            if remote_count != pending_before:
                raise RuntimeError("process-level remote event count did not match receipts")
            finding_row = run(["docker", "exec", container, "psql", "-U", "watchhouse", "-d", "watchhouse", "-AtF", "|", "-c",
                               "SELECT count(*),COALESCE(jsonb_array_length(min(evidence_event_ids::text)::jsonb),0) FROM control_findings WHERE host_id='process-host'"]).stdout.strip()
            finding_count, finding_evidence = (int(value) for value in finding_row.split("|"))
            if finding_count != 1 or finding_evidence != 6:
                raise RuntimeError("process-level detection finding was not persisted with six evidence events")
            findings = json.loads(run([str(agent_binary), "query-findings", "--host", "process-host", "--endpoint", endpoint,
                                       "--ca", str(pki["ca"]), "--cert", str(pki["viewer_cert"]),
                                       "--key", str(pki["viewer_key"]), "--server-name", "control.test", "--limit", "3"]).stdout)
            finding_records = findings["page"]["records"]
            if findings["authenticated_user"] != "alice" or len(finding_records) != 1 or len(finding_records[0]["evidence_event_ids"]) != 6:
                raise RuntimeError("viewer finding query did not return the persisted evidence chain")
            queried = json.loads(run([str(agent_binary), "query-events", "--host", "process-host", "--endpoint", endpoint,
                                      "--ca", str(pki["ca"]), "--cert", str(pki["viewer_cert"]),
                                      "--key", str(pki["viewer_key"]), "--server-name", "control.test", "--limit", "3"]).stdout)
            records = queried["page"]["records"]
            if queried["authenticated_user"] != "alice" or len(records) != 3:
                raise RuntimeError("viewer query identity or page bound failed")
            older = json.loads(run([str(agent_binary), "query-events", "--host", "process-host", "--endpoint", endpoint,
                                    "--ca", str(pki["ca"]), "--cert", str(pki["viewer_cert"]),
                                    "--key", str(pki["viewer_key"]), "--server-name", "control.test", "--limit", "3",
                                    "--before", str(records[-1]["ingest_sequence"])]).stdout)
            if not older["page"]["records"] or older["page"]["records"][0]["ingest_sequence"] >= records[-1]["ingest_sequence"]:
                raise RuntimeError("viewer pagination did not advance exclusively")
            unknown_query = subprocess.run([str(agent_binary), "query-events", "--host", "process-host", "--endpoint", endpoint,
                                            "--ca", str(pki["ca"]), "--cert", str(pki["unknown_cert"]),
                                            "--key", str(pki["unknown_key"]), "--server-name", "control.test"],
                                           capture_output=True, text=True, timeout=45)
            if unknown_query.returncode == 0:
                raise RuntimeError("unmapped human certificate queried events")
            audit_summary = run(["docker", "exec", container, "psql", "-U", "watchhouse", "-d", "watchhouse", "-AtF", "|", "-c",
                                 "SELECT count(*),count(*) FILTER (WHERE decision='allowed'),count(*) FILTER (WHERE decision='denied') FROM control_query_audit WHERE host_id='process-host'"]).stdout.strip()
            audit_total, audit_allowed, audit_denied = (int(value) for value in audit_summary.split("|"))
            if (audit_total, audit_allowed, audit_denied) != (4, 3, 1):
                raise RuntimeError("query authorization audit did not capture three allows and one denial")
            wrong_state = Path(directory) / "wrong-state"
            run([str(agent_binary), "spool", "ingest", "--state", str(wrong_state), "--host", "claimed-host", "--input", str(fixture)])
            wrong = subprocess.run([str(agent_binary), "deliver", "--state", str(wrong_state), "--endpoint", endpoint,
                                    "--ca", str(pki["ca"]), "--cert", str(pki["agent_cert"]),
                                    "--key", str(pki["agent_key"]), "--server-name", "control.test"],
                                   capture_output=True, text=True, timeout=45)
            wrong_status = json.loads(run([str(agent_binary), "spool", "status", "--state", str(wrong_state)]).stdout)
            wrong_remote = int(run(["docker", "exec", container, "psql", "-U", "watchhouse", "-d", "watchhouse", "-Atqc",
                                    "SELECT count(*) FROM control_events WHERE host_id='claimed-host'"]).stdout.strip())
            if wrong.returncode == 0 or wrong_status["stats"]["pending_records"] != pending_before or wrong_remote != 0:
                raise RuntimeError("certificate/payload host mismatch was not rejected without local loss")
            control.terminate()
            control_exit = control.wait(timeout=15)
            control = None
            control_log.close()
            if control_exit != 0:
                raise RuntimeError("control process did not shut down cleanly")
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
                      "tests": ["idempotent migration", "idempotent repeat", "atomic conflict rollback", "eight-way concurrent repeat",
                                "TLS 1.3 client identity", "SQLite-to-PostgreSQL exact receipts", "lost-receipt retry without remote duplicate",
                                "persistent SSH finding with evidence", "human viewer pagination", "unmapped human authorization rejection"],
                      "process_test": {"queued": pending_before, "acknowledged": delivered["result"]["acknowledged"],
                                       "remote_rows": remote_count, "wrong_host_rejected": True,
                                       "persisted_findings": finding_count, "finding_evidence_events": finding_evidence,
                                       "viewer_finding_records": len(finding_records),
                                       "viewer_page_records": len(records), "viewer_second_page_records": len(older["page"]["records"]),
                                       "unmapped_user_rejected": True,
                                       "query_audit_rows": audit_total, "query_audit_allowed": audit_allowed, "query_audit_denied": audit_denied,
                                       "wrong_host_pending": wrong_status["stats"]["pending_records"],
                                       "control_exit_code": control_exit,
                                       "agent_binary_sha256": hashlib.sha256(agent_binary.read_bytes()).hexdigest(),
                                       "control_binary_sha256": hashlib.sha256(control_binary.read_bytes()).hexdigest()},
                      "test_output": tested.stdout.strip(),
                      "scope": "local disposable database; loopback connection is not production database TLS"}
            output = ROOT / "lab/local/postgres.result.json"
            output.write_text(json.dumps(report, indent=2) + "\n")
            output.chmod(0o600)
            print(json.dumps(report))
        finally:
            if control is not None:
                control.terminate()
                try:
                    control.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    control.kill()
                    control.wait(timeout=10)
            if container:
                inspected = subprocess.run(["docker", "inspect", container], capture_output=True, text=True, timeout=30)
                if inspected.returncode == 0:
                    details = json.loads(inspected.stdout)[0]
                    if details["Name"] != "/" + name or details["Config"]["Labels"].get("org.watchhouse.integration") != "control-postgres":
                        raise RuntimeError("refusing to remove container with mismatched identity")
                    run(["docker", "rm", "-f", container], timeout=60)


if __name__ == "__main__":
    main()
