#!/usr/bin/env python3
"""Run real control and delivery systemd units in the dedicated Ubuntu guest."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import sys
import tempfile
import time
import urllib.parse

import integration_pki
import lab_vm

ROOT = lab_vm.ROOT
POSTGRES = json.loads((ROOT / "lab/postgres/images.json").read_text())["postgres"]["reference"]
LABEL = "org.watchhouse.integration=systemd-control"
RUNNER_SHA256 = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
PHASE = "startup"
STARTED = time.monotonic()


def remote(state, command, check=True, timeout=120):
    return subprocess.run([*lab_vm.ssh_args(state), command], capture_output=True, text=True,
                          check=check, timeout=timeout)


def wait_remote(state, command, timeout=60):
    deadline = time.monotonic() + timeout
    last = None
    while time.monotonic() < deadline:
        last = remote(state, command, check=False)
        if last.returncode == 0:
            return
        time.sleep(1)
    raise RuntimeError("guest readiness command failed: " + ((last.stderr if last else "")[-1024:]))


def main():
    global PHASE
    go = os.environ.get("GO", "go")
    PHASE = "guest_readiness"
    state, details = lab_vm.inspect()
    if not details["State"]["Running"] or not lab_vm.probe().get("ssh_ready"):
        raise RuntimeError("verified, ready lab VM required")
    source_commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    source_dirty = bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT, text=True).strip())
    local = ROOT / "lab/local"
    local.mkdir(mode=0o700, exist_ok=True)
    container = None
    container_name = "watchhouse-systemd-pg-" + secrets.token_hex(6)
    password = secrets.token_urlsafe(32)
    state_name = "watchhouse-e2e-" + secrets.token_hex(6)
    state_path = "/var/lib/" + state_name
    probe_state_name = "watchhouse-probe-e2e-" + secrets.token_hex(6)
    probe_state_path = "/var/lib/" + probe_state_name
    with tempfile.TemporaryDirectory(prefix="systemd-control-", dir=local) as directory:
        directory = Path(directory)
        password_file = directory / "postgres-password"
        password_file.write_text(password)
        password_file.chmod(0o600)
        try:
            PHASE = "postgres_start"
            container = subprocess.check_output([
                "docker", "run", "-d", "--name", container_name, "--label", LABEL,
                "--memory", "1g", "--cpus", "2", "--pids-limit", "256",
                "--tmpfs", "/var/lib/postgresql:rw,nosuid,nodev,size=512m",
                "--tmpfs", "/var/run/postgresql:rw,nosuid,nodev,size=16m",
                "--mount", "type=bind,source=" + str(password_file) + ",target=/run/secrets/postgres_password,readonly",
                "--env", "POSTGRES_PASSWORD_FILE=/run/secrets/postgres_password",
                "--env", "POSTGRES_USER=watchhouse", "--env", "POSTGRES_DB=watchhouse", POSTGRES], text=True).strip()
            deadline = time.monotonic() + 120
            while time.monotonic() < deadline:
                ready = subprocess.run(["docker", "exec", container, "pg_isready", "-U", "watchhouse", "-d", "watchhouse"], capture_output=True, timeout=10)
                if ready.returncode == 0:
                    break
                time.sleep(1)
            else:
                raise TimeoutError("PostgreSQL readiness timed out")
            pg_details = json.loads(subprocess.check_output(["docker", "inspect", container], text=True))[0]
            pg_ip = pg_details["NetworkSettings"]["Networks"]["bridge"]["IPAddress"]
            if not pg_ip:
                raise RuntimeError("PostgreSQL integration container has no bridge address")
            PHASE = "build_and_upload"
            agent = ROOT / "bin/watchhouse-linux-arm64"
            control = ROOT / "bin/watchhouse-control-linux-arm64"
            environment = dict(os.environ, GOOS="linux", GOARCH="arm64", CGO_ENABLED="0")
            subprocess.run([go, "build", "-trimpath", "-o", str(agent), "./cmd/watchhouse"], cwd=ROOT, env=environment, check=True, timeout=180)
            subprocess.run([go, "build", "-trimpath", "-o", str(control), "./cmd/watchhouse-control"], cwd=ROOT, env=environment, check=True, timeout=180)
            pki = integration_pki.generate(directory, host="vm-agent", viewer="vm-viewer", unknown="vm-unknown")
            roles = directory / "roles.json"
            roles.write_text(json.dumps({"schema_version": 1, "principals": {"vm-viewer": "viewer"}}) + "\n")
            roles.chmod(0o644)
            dsn = "postgres://watchhouse:" + urllib.parse.quote(password, safe="") + "@" + pg_ip + ":5432/watchhouse?sslmode=disable"
            database_url = directory / "database-url"
            database_url.write_text(dsn + "\n")
            database_url.chmod(0o600)
            e2e_unit = directory / "watchhouse-deliver-e2e.service"
            text = (ROOT / "deploy/systemd/watchhouse-deliver.service").read_text()
            text = text.replace("/var/lib/watchhouse", state_path).replace("StateDirectory=watchhouse\n", "StateDirectory=" + state_name + "\n")
            e2e_unit.write_text(text)
            probe_unit = directory / "watchhouse-probe-e2e.service"
            text = (ROOT / "deploy/systemd/watchhouse-probe.service").read_text()
            text = text.replace("/var/lib/watchhouse-probe", probe_state_path).replace("StateDirectory=watchhouse-probe\n", "StateDirectory=" + probe_state_name + "\n")
            text = text.replace("LoadCredential=control-ca.pem:/etc/watchhouse/probe/control-ca.pem\n",
                                "LoadCredential=control-ca.pem:/etc/watchhouse/probe/control-ca.pem\nLoadCredential=target-ca.pem:/etc/watchhouse/probe/target-ca.pem\n")
            text = text.replace("--timeout 15s --max-body 65536", "--timeout 15s --max-body 65536 --target-ca ${CREDENTIALS_DIRECTORY}/target-ca.pem --allow-private")
            probe_unit.write_text(text)
            probe_deliver_unit = directory / "watchhouse-probe-deliver-e2e.service"
            text = (ROOT / "deploy/systemd/watchhouse-probe-deliver.service").read_text()
            text = text.replace("/var/lib/watchhouse-probe", probe_state_path).replace("StateDirectory=watchhouse-probe\n", "StateDirectory=" + probe_state_name + "\n")
            probe_deliver_unit.write_text(text)
            uploads = {
                agent: "watchhouse-agent", control: "watchhouse-control",
                ROOT / "deploy/systemd/watchhouse-control.service": "watchhouse-control.service",
                ROOT / "deploy/systemd/watchhouse-deliver.service": "watchhouse-deliver.service",
                ROOT / "deploy/systemd/watchhouse-deliver.timer": "watchhouse-deliver.timer",
                e2e_unit: "watchhouse-deliver-e2e.service",
                probe_unit: "watchhouse-probe-e2e.service", probe_deliver_unit: "watchhouse-probe-deliver-e2e.service",
                ROOT / "deploy/systemd/watchhouse-probe.service": "watchhouse-probe.service",
                ROOT / "deploy/systemd/watchhouse-probe.timer": "watchhouse-probe.timer",
                ROOT / "deploy/systemd/watchhouse-probe-deliver.service": "watchhouse-probe-deliver.service",
                ROOT / "deploy/systemd/watchhouse-probe-deliver.timer": "watchhouse-probe-deliver.timer",
                ROOT / "tests/fixtures/ssh-sequence.journal.jsonl": "ssh-sequence.jsonl",
                pki["ca"]: "client-ca.pem", pki["server_cert"]: "server-cert.pem",
                pki["server_key"]: "server-key.pem", pki["agent_cert"]: "agent-cert.pem",
                pki["agent_key"]: "agent-key.pem", pki["viewer_cert"]: "viewer-cert.pem",
                pki["viewer_key"]: "viewer-key.pem", pki["probe_cert"]: "probe-cert.pem",
                pki["probe_key"]: "probe-key.pem", roles: "roles.json", database_url: "database-url"}
            for path, destination in uploads.items():
                lab_vm.upload(state, path, destination)
            PHASE = "unit_install"
            install = f"""set -eu
sudo -n install -o root -g root -m 0755 /home/watchhouse-lab/watchhouse-agent /usr/local/libexec/watchhouse
sudo -n install -o root -g root -m 0755 /home/watchhouse-lab/watchhouse-control /usr/local/libexec/watchhouse-control
sudo -n install -d -o root -g root -m 0755 /etc/watchhouse/control /etc/watchhouse/pki
sudo -n install -d -o root -g root -m 0755 /etc/watchhouse/probe
id -u watchhouse-probe >/dev/null 2>&1 || sudo -n useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin watchhouse-probe
sudo -n install -o root -g root -m 0600 /home/watchhouse-lab/database-url /etc/watchhouse/control/database-url
sudo -n install -o root -g root -m 0644 /home/watchhouse-lab/roles.json /etc/watchhouse/control/roles.json
sudo -n install -o root -g root -m 0644 /home/watchhouse-lab/client-ca.pem /etc/watchhouse/control/client-ca.pem
sudo -n install -o root -g root -m 0644 /home/watchhouse-lab/server-cert.pem /etc/watchhouse/control/server-cert.pem
sudo -n install -o root -g root -m 0600 /home/watchhouse-lab/server-key.pem /etc/watchhouse/control/server-key.pem
sudo -n install -o root -g root -m 0644 /home/watchhouse-lab/client-ca.pem /etc/watchhouse/pki/agent-ca.pem
sudo -n install -o root -g root -m 0644 /home/watchhouse-lab/agent-cert.pem /etc/watchhouse/pki/agent-cert.pem
sudo -n install -o root -g root -m 0600 /home/watchhouse-lab/agent-key.pem /etc/watchhouse/pki/agent-key.pem
sudo -n install -o root -g root -m 0644 /home/watchhouse-lab/client-ca.pem /etc/watchhouse/probe/control-ca.pem
sudo -n install -o root -g root -m 0644 /home/watchhouse-lab/client-ca.pem /etc/watchhouse/probe/target-ca.pem
sudo -n install -o root -g root -m 0644 /home/watchhouse-lab/probe-cert.pem /etc/watchhouse/probe/probe-cert.pem
sudo -n install -o root -g root -m 0600 /home/watchhouse-lab/probe-key.pem /etc/watchhouse/probe/probe-key.pem
printf 'WATCHHOUSE_LISTEN=127.0.0.1:18443\nWATCHHOUSE_SERVER_NAME=control.test\n' | sudo -n tee /etc/watchhouse/control.env >/dev/null
printf 'WATCHHOUSE_ENDPOINT=https://127.0.0.1:18443\nWATCHHOUSE_SERVER_NAME=control.test\n' | sudo -n tee /etc/watchhouse/delivery.env >/dev/null
printf 'WATCHHOUSE_PROBE_ID=outside-1\nWATCHHOUSE_TARGET_URL=https://127.0.0.1:19443/\nWATCHHOUSE_EXPECT_STATUS=200\nWATCHHOUSE_CONTROL_ENDPOINT=https://127.0.0.1:18443\nWATCHHOUSE_CONTROL_SERVER_NAME=control.test\n' | sudo -n tee /etc/watchhouse/probe.env >/dev/null
sudo -n chmod 0600 /etc/watchhouse/control.env /etc/watchhouse/delivery.env /etc/watchhouse/probe.env
for unit in watchhouse-control.service watchhouse-deliver.service watchhouse-deliver.timer watchhouse-deliver-e2e.service watchhouse-probe.service watchhouse-probe.timer watchhouse-probe-deliver.service watchhouse-probe-deliver.timer watchhouse-probe-e2e.service watchhouse-probe-deliver-e2e.service; do sudo -n install -o root -g root -m 0644 /home/watchhouse-lab/$unit /etc/systemd/system/$unit; done
sudo -n install -d -o watchhouse -g watchhouse -m 0700 {state_path}
sudo -n install -o watchhouse -g watchhouse -m 0600 /home/watchhouse-lab/ssh-sequence.jsonl {state_path}/fixture.jsonl
sudo -n install -o watchhouse -g watchhouse -m 0400 /etc/watchhouse/pki/agent-ca.pem {state_path}/agent-ca.pem
sudo -n install -o watchhouse -g watchhouse -m 0400 /etc/watchhouse/pki/agent-cert.pem {state_path}/agent-cert.pem
sudo -n install -o watchhouse -g watchhouse -m 0400 /etc/watchhouse/pki/agent-key.pem {state_path}/agent-key.pem
sudo -n systemd-analyze verify /etc/systemd/system/watchhouse-control.service /etc/systemd/system/watchhouse-deliver.service /etc/systemd/system/watchhouse-deliver.timer /etc/systemd/system/watchhouse-deliver-e2e.service /etc/systemd/system/watchhouse-probe.service /etc/systemd/system/watchhouse-probe.timer /etc/systemd/system/watchhouse-probe-deliver.service /etc/systemd/system/watchhouse-probe-deliver.timer /etc/systemd/system/watchhouse-probe-e2e.service /etc/systemd/system/watchhouse-probe-deliver-e2e.service
sudo -n systemctl daemon-reload
"""
            remote(state, install, timeout=180)
            PHASE = "seed_and_start"
            remote(state, f"sudo -n -u watchhouse /usr/local/libexec/watchhouse spool ingest --state {state_path} --host vm-agent --input {state_path}/fixture.jsonl")
            before = json.loads(remote(state, f"sudo -n -u watchhouse /usr/local/libexec/watchhouse spool status --state {state_path}").stdout)
            pending = before["stats"]["pending_records"]
            if pending < 1:
                raise RuntimeError("systemd delivery fixture produced no queue")
            remote(state, "sudo -n systemctl restart watchhouse-control.service", timeout=180)
            wait_remote(state, "systemctl is-active --quiet watchhouse-control.service && timeout 3 bash -c '</dev/tcp/127.0.0.1/18443'", timeout=90)
            control_properties = remote(state, "systemctl show watchhouse-control.service -p ActiveState -p SubState -p DynamicUser -p User -p NoNewPrivileges -p ProtectSystem -p ProtectHome").stdout.strip().splitlines()
            PHASE = "delivery_unit"
            remote(state, "sudo -n systemctl start watchhouse-deliver-e2e.service", timeout=90)
            delivery_properties = remote(state, "systemctl show watchhouse-deliver-e2e.service -p Result -p ExecMainStatus -p NoNewPrivileges -p ProtectSystem -p ProtectHome").stdout.strip().splitlines()
            after = json.loads(remote(state, f"sudo -n -u watchhouse /usr/local/libexec/watchhouse spool status --state {state_path}").stdout)
            if after["stats"]["pending_records"] != 0 or "Result=success" not in delivery_properties or "ExecMainStatus=0" not in delivery_properties:
                raise RuntimeError("sandboxed systemd delivery did not commit exact receipts")
            remote_count = int(subprocess.check_output(["docker", "exec", container, "psql", "-U", "watchhouse", "-d", "watchhouse", "-Atqc",
                                                        "SELECT count(*) FROM control_events WHERE host_id='vm-agent'"], text=True).strip())
            if remote_count != pending:
                raise RuntimeError("systemd control database count differs from drained queue")
            PHASE = "probe_unit"
            remote(state, "sudo -n systemd-run --unit watchhouse-probe-target-e2e --property=User=root --property=ProtectSystem=strict --property=NoNewPrivileges=yes /usr/bin/openssl s_server -quiet -www -accept 127.0.0.1:19443 -cert /home/watchhouse-lab/server-cert.pem -key /home/watchhouse-lab/server-key.pem")
            wait_remote(state, "timeout 3 bash -c '</dev/tcp/127.0.0.1/19443'", timeout=30)
            remote(state, "sudo -n systemctl start watchhouse-probe-e2e.service", timeout=90)
            probe_properties = remote(state, "systemctl show watchhouse-probe-e2e.service -p Result -p ExecMainStatus -p NoNewPrivileges -p ProtectSystem -p ProtectHome -p User").stdout.strip().splitlines()
            probe_stats = json.loads(remote(state, f"sudo -n -u watchhouse-probe /usr/local/libexec/watchhouse spool status --state {probe_state_path}").stdout)
            probe_rows = int(subprocess.check_output(["docker", "exec", container, "psql", "-U", "watchhouse", "-d", "watchhouse", "-Atqc",
                                                      "SELECT count(*) FROM control_probe_observations WHERE probe_id='outside-1'"], text=True).strip())
            if probe_stats["stats"]["probe_pending_records"] != 0 or probe_rows != 1 or "Result=success" not in probe_properties or "ExecMainStatus=0" not in probe_properties:
                raise RuntimeError("sandboxed probe did not queue, deliver, and persist one observation")
            PHASE = "listener_report"
            listener = json.loads(remote(state, f"sudo -n -u watchhouse /usr/local/libexec/watchhouse report-listeners --host vm-agent --state {state_path} --endpoint https://127.0.0.1:18443 --ca {state_path}/agent-ca.pem --cert {state_path}/agent-cert.pem --key {state_path}/agent-key.pem --server-name control.test").stdout)
            listener_rows = int(subprocess.check_output(["docker", "exec", container, "psql", "-U", "watchhouse", "-d", "watchhouse", "-Atqc",
                                                         "SELECT count(*) FROM control_listener_snapshots WHERE host_id='vm-agent'"], text=True).strip())
            listener_after = json.loads(remote(state, f"sudo -n -u watchhouse /usr/local/libexec/watchhouse spool status --state {state_path}").stdout)
            if listener["result"]["acknowledged"] != 1 or listener_rows != 1 or listener_after["stats"]["listener_pending_records"] != 0:
                raise RuntimeError("listener snapshot was not durably queued, acknowledged, and persisted")
            PHASE = "human_query"
            query = remote(state, "/usr/local/libexec/watchhouse query-events --host vm-agent --endpoint https://127.0.0.1:18443 --ca /home/watchhouse-lab/client-ca.pem --cert /home/watchhouse-lab/viewer-cert.pem --key /home/watchhouse-lab/viewer-key.pem --server-name control.test --limit 3")
            page = json.loads(query.stdout)
            if page["authenticated_user"] != "vm-viewer" or len(page["page"]["records"]) != min(3, pending):
                raise RuntimeError("human systemd control query failed")
            report = {"type": "systemd_control_integration", "passed": True,
                      "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                      "source_commit": source_commit, "worktree_dirty_at_build": source_dirty,
                      "runner_sha256": RUNNER_SHA256,
                      "agent_binary_sha256": hashlib.sha256(agent.read_bytes()).hexdigest(),
                      "control_binary_sha256": hashlib.sha256(control.read_bytes()).hexdigest(),
                      "postgres_image": POSTGRES, "postgres_network_exposure": "unpublished Docker bridge address",
                      "queued_before": pending, "pending_after": after["stats"]["pending_records"],
                      "remote_rows": remote_count, "viewer_records": len(page["page"]["records"]),
                      "listener_snapshot_rows": listener_rows, "listener_pending_after": listener_after["stats"]["listener_pending_records"],
                      "listener_snapshot_id": listener["snapshot_id"], "integration_state_directory": state_path,
                      "probe_observation_rows": probe_rows, "probe_pending_after": probe_stats["stats"]["probe_pending_records"],
                      "control_properties": control_properties, "delivery_properties": delivery_properties, "probe_properties": probe_properties,
                      "scope": "real Ubuntu systemd guest and disposable bridge PostgreSQL; database link lacks TLS; not public VPS"}
            (lab_vm.VM / "systemd-control.result.json").write_text(json.dumps(report, indent=2) + "\n")
            print(json.dumps(report))
        finally:
            try:
                current_state, current_details = lab_vm.inspect()
                if current_details["State"]["Running"]:
                    remote(current_state, "sudo -n systemctl stop watchhouse-probe-target-e2e.service watchhouse-probe-e2e.service watchhouse-probe-deliver-e2e.service watchhouse-deliver-e2e.service watchhouse-control.service", check=False, timeout=60)
                    remote(current_state, f"case {state_path} in /var/lib/watchhouse-e2e-*) sudo -n rm -rf -- {state_path};; *) exit 1;; esac", check=False, timeout=60)
                    remote(current_state, f"case {probe_state_path} in /var/lib/watchhouse-probe-e2e-*) sudo -n rm -rf -- {probe_state_path};; *) exit 1;; esac", check=False, timeout=60)
            except Exception:
                pass
            if container:
                inspected = subprocess.run(["docker", "inspect", container], capture_output=True, text=True, timeout=30)
                if inspected.returncode == 0:
                    current = json.loads(inspected.stdout)[0]
                    if current["Name"] != "/" + container_name or current["Config"]["Labels"].get("org.watchhouse.integration") != "systemd-control":
                        raise RuntimeError("refusing cleanup of mismatched PostgreSQL container")
                    subprocess.run(["docker", "rm", "-f", container], check=True, capture_output=True, timeout=60)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        failure = {"type": "systemd_control_integration_failure", "passed": False,
                   "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                   "phase": PHASE, "error_type": type(error).__name__,
                   "elapsed_seconds": round(time.monotonic() - STARTED, 2), "runner_sha256": RUNNER_SHA256}
        if lab_vm.VM.is_dir():
            (lab_vm.VM / "systemd-control.last-failure.json").write_text(json.dumps(failure, indent=2) + "\n")
        print(json.dumps(failure), file=sys.stderr)
        raise
