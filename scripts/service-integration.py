#!/usr/bin/env python3
"""Install only the fixed collector in our localhost lab; verify OS boundaries."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys
import time

import lab_vm

ROOT = lab_vm.ROOT
RUNNER_SHA256 = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
PHASE = "startup"
STARTED = time.monotonic()


def remote(state, command, check=True):
    return subprocess.run([*lab_vm.ssh_args(state), command], capture_output=True, text=True, check=check, timeout=60)


def upload(state, path, destination):
    args = ["scp", "-F", "/dev/null", "-i", str(lab_vm.VM / "operator"), "-P", str(state["port"]),
            "-o", f"ConnectTimeout={lab_vm.SSH_CONNECT_TIMEOUT}",
            "-o", "IdentitiesOnly=yes", "-o", "IdentityAgent=none", "-o", "StrictHostKeyChecking=yes",
            "-o", "UserKnownHostsFile=" + str(lab_vm.VM / "known_hosts"), str(path),
            "watchhouse-lab@127.0.0.1:/home/watchhouse-lab/" + destination]
    subprocess.run(args, check=True, capture_output=True, text=True, timeout=180)


def sandbox(state, program, check=True):
    # These fixed properties mirror the service boundary; this is a lab-only
    # acceptance harness, not an arbitrary production action interface.
    properties = ["User=watchhouse", "Group=watchhouse", "SupplementaryGroups=systemd-journal",
                  "NoNewPrivileges=yes", "CapabilityBoundingSet=", "AmbientCapabilities=",
                  "ProtectSystem=strict", "ProtectHome=yes", "ReadWritePaths=/var/lib/watchhouse",
                  "PrivateTmp=yes", "RestrictAddressFamilies=AF_UNIX", "RestrictSUIDSGID=yes"]
    argv = ["sudo", "-n", "systemd-run", "--quiet", "--wait", "--pipe", "--collect"]
    for value in properties:
        argv += ["--property", value]
    argv += program
    return remote(state, shlex.join(argv), check=check)


def main():
    global PHASE
    parser = argparse.ArgumentParser()
    parser.add_argument("--go", default=os.environ.get("GO", "go"))
    args = parser.parse_args()
    PHASE = "guest_readiness"
    state, details = lab_vm.inspect()
    if not details["State"]["Running"] or not lab_vm.probe().get("ssh_ready"):
        raise RuntimeError("verified, ready lab VM required")
    source_commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    source_dirty = bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT, text=True).strip())
    binary = ROOT / "bin/watchhouse-linux-arm64"
    subprocess.run([args.go, "build", "-trimpath", "-o", str(binary), "./cmd/watchhouse"], cwd=ROOT,
                   env=dict(os.environ, GOOS="linux", GOARCH="arm64", CGO_ENABLED="0"), check=True, timeout=180)
    PHASE = "artifact_upload"
    upload(state, binary, "watchhouse-service-binary")
    for name in ("watchhouse-collect.service", "watchhouse-collect.timer"):
        upload(state, ROOT / "deploy/systemd" / name, name)
    PHASE = "account_and_install"
    account = remote(state, "getent passwd watchhouse", check=False)
    if account.returncode != 0:
        remote(state, "sudo -n useradd --system --user-group --home-dir /var/lib/watchhouse --shell /usr/sbin/nologin watchhouse")
        account = remote(state, "getent passwd watchhouse")
    fields = account.stdout.strip().split(":")
    if len(fields) != 7 or int(fields[2]) == 0 or fields[6] != "/usr/sbin/nologin":
        raise RuntimeError("existing account is not the expected dedicated system user")
    uid = int(fields[2])
    existing = remote(state, "test -e /var/lib/watchhouse", check=False)
    if existing.returncode == 0:
        owner = remote(state, "stat -c '%u %a' /var/lib/watchhouse").stdout.strip()
        if owner != f"{uid} 700":
            raise RuntimeError("existing state ownership differs; refusing silent repair")
    remote(state, "set -eu; sudo -n install -d -m 0755 /usr/local/libexec /etc/watchhouse; "
                  "sudo -n install -o root -g root -m 0755 /home/watchhouse-lab/watchhouse-service-binary /usr/local/libexec/watchhouse; "
                  "sudo -n install -d -o watchhouse -g watchhouse -m 0700 /var/lib/watchhouse")
    PHASE = "unit_install_and_start"
    remote(state, "set -eu; printf 'WATCHHOUSE_HOST=vm-reader\n' | sudo -n tee /etc/watchhouse/agent.env >/dev/null; "
                  "sudo -n chmod 0600 /etc/watchhouse/agent.env; "
                  "sudo -n install -o root -g root -m 0644 /home/watchhouse-lab/watchhouse-collect.service /etc/systemd/system/; "
                  "sudo -n install -o root -g root -m 0644 /home/watchhouse-lab/watchhouse-collect.timer /etc/systemd/system/; "
                  "sudo -n systemd-analyze verify /etc/systemd/system/watchhouse-collect.service /etc/systemd/system/watchhouse-collect.timer; "
                  "sudo -n systemctl daemon-reload; sudo -n systemctl start watchhouse-collect.service")
    PHASE = "runtime_boundary_checks"
    unit = remote(state, "systemctl show watchhouse-collect.service -p Result -p ExecMainStatus -p User -p Group -p NoNewPrivileges -p ProtectSystem -p ProtectHome").stdout
    if "Result=success" not in unit or "ExecMainStatus=0" not in unit:
        raise RuntimeError("sandboxed collector failed; inspect its journald, do not weaken settings")
    profile = json.loads(sandbox(state, ["/usr/local/libexec/watchhouse", "self"]).stdout)
    if profile["uid"] != uid or not profile["linux_security"]["no_new_privileges"] or profile["linux_security"]["effective_capabilities"] != "0000000000000000":
        raise RuntimeError("collector privilege boundary not effective")
    groups = remote(state, "id -Gn watchhouse").stdout.strip().split()
    if "sudo" in groups:
        raise RuntimeError("dedicated collector unexpectedly belongs to sudo")
    stats = json.loads(remote(state, "sudo -n -u watchhouse /usr/local/libexec/watchhouse spool status --state /var/lib/watchhouse").stdout)
    audit = json.loads(remote(state, "sudo -n -u watchhouse /usr/local/libexec/watchhouse spool check --state /var/lib/watchhouse").stdout)
    if stats["stats"]["pending_records"] < 1 or not audit["result"]["valid"]:
        raise RuntimeError("sandboxed collector did not persist valid real telemetry")
    # Demonstrate ProtectHome against a public-readable fixture; private home
    # permissions alone would not be adequate evidence for this mount boundary.
    remote(state, "set -eu; sudo -n install -d -m 0755 /home/watchhouse-boundary-public; "
                  "printf 'public fixture\n' | sudo -n tee /home/watchhouse-boundary-public/visible >/dev/null; "
                  "sudo -n chmod 0644 /home/watchhouse-boundary-public/visible")
    remote(state, "sudo -n -u watchhouse test -r /home/watchhouse-boundary-public/visible")
    hidden = sandbox(state, ["/usr/bin/test", "-r", "/home/watchhouse-boundary-public/visible"], check=False)
    if hidden.returncode == 0:
        raise RuntimeError("ProtectHome did not hide baseline-readable fixture")
    remote(state, "sudo -n systemctl start watchhouse-collect.timer")
    timer = remote(state, "systemctl is-active watchhouse-collect.timer").stdout.strip()
    if timer != "active":
        raise RuntimeError("collection timer did not activate")
    report = {"type": "service_integration", "passed": True,
              "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "source_commit_at_build": source_commit, "worktree_dirty_at_build": source_dirty,
              "runner_sha256": RUNNER_SHA256, "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
              "service_uid": uid, "service_groups": groups, "profile": profile,
              "unit_properties": unit.strip().splitlines(), "queue": stats["stats"], "audit": audit["result"],
              "protect_home_differential_passed": True, "timer": timer,
              "scope": "fixed read-only collector in isolated Ubuntu VM; no production privileged executor"}
    (lab_vm.VM / "service.result.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        failure = {"type": "service_integration_failure", "passed": False,
                   "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                   "phase": PHASE, "error_type": type(error).__name__,
                   "elapsed_seconds": round(time.monotonic() - STARTED, 2),
                   "runner_sha256": RUNNER_SHA256}
        if lab_vm.VM.is_dir():
            (lab_vm.VM / "service.last-failure.json").write_text(json.dumps(failure, indent=2) + "\n")
        print(json.dumps(failure), file=sys.stderr)
        raise
