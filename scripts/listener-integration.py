#!/usr/bin/env python3
"""Verify TCP inode/PID/unit attribution in the dedicated localhost VM."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time

import lab_vm

ROOT = lab_vm.ROOT
PORT = 18081
UNIT = "watchhouse-listener-fixture.service"
RUNNER_SHA256 = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()
PHASE = "startup"
STARTED = time.monotonic()


def remote(state, command, check=True):
    return subprocess.run([*lab_vm.ssh_args(state), command], capture_output=True, text=True,
                          check=check, timeout=90)


def snapshot(state, privileged):
    prefix = "sudo -n " if privileged else ""
    result = remote(state, prefix + "/home/watchhouse-lab/watchhouse-listeners listeners")
    return json.loads(result.stdout)


def fixture(snapshot_value):
    matches = [item for item in snapshot_value["listeners"]
               if item["family"] == "ipv4" and item["local_address"] == "127.0.0.1"
               and item["local_port"] == PORT]
    if len(matches) != 1:
        raise RuntimeError("controlled listener was absent or ambiguous")
    return matches[0]


def evaluate(state, policy_name):
    result = remote(state, "sudo -n /home/watchhouse-lab/watchhouse-listeners listener-check --policy /home/watchhouse-lab/" + policy_name)
    return json.loads(result.stdout)


def main():
    global PHASE
    parser = __import__("argparse").ArgumentParser()
    parser.add_argument("--go", default=os.environ.get("GO", "go"))
    args = parser.parse_args()
    PHASE = "guest_readiness"
    state, details = lab_vm.inspect()
    ready = lab_vm.probe()
    if not details["State"]["Running"] or not ready.get("ssh_ready"):
        raise RuntimeError("verified, ready lab VM required")
    source_commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    source_dirty = bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT, text=True).strip())
    binary = ROOT / "bin/watchhouse-linux-arm64"
    subprocess.run([args.go, "build", "-trimpath", "-o", str(binary), "./cmd/watchhouse"], cwd=ROOT,
                   env=dict(os.environ, GOOS="linux", GOARCH="arm64", CGO_ENABLED="0"), check=True, timeout=180)
    PHASE = "artifact_upload"
    lab_vm.upload(state, binary, "watchhouse-listeners")
    with tempfile.TemporaryDirectory(prefix="watchhouse-listener-policy-") as work:
        allowed = {"schema_version": 1, "policy_id": "lab-allowed", "listeners": [
            {"resource_id": "fixture", "family": "ipv4", "local_address": "127.0.0.1",
             "local_port": PORT, "allowed_units": [UNIT]}]}
        drift = {"schema_version": 1, "policy_id": "lab-drift", "listeners": [
            {"resource_id": "missing-fixture", "family": "ipv4", "local_address": "127.0.0.1",
             "local_port": PORT + 1, "allowed_units": []}]}
        for name, value in (("listener-allowed.json", allowed), ("listener-drift.json", drift)):
            path = Path(work) / name
            path.write_text(json.dumps(value) + "\n")
            lab_vm.upload(state, path, name)
    remote(state, "chmod 0755 /home/watchhouse-lab/watchhouse-listeners")
    PHASE = "fixture_start"
    remote(state, "sudo -n systemctl stop " + UNIT, check=False)
    remote(state, "sudo -n systemd-run --quiet --unit=" + UNIT
                  + " --property=Type=simple --property=User=root --property=NoNewPrivileges=yes"
                  + " /usr/bin/python3 -m http.server " + str(PORT) + " --bind 127.0.0.1")
    try:
        deadline = time.monotonic() + 60
        while time.monotonic() < deadline:
            active = remote(state, "systemctl is-active " + UNIT, check=False)
            if active.returncode == 0:
                break
            time.sleep(1)
        else:
            raise RuntimeError("controlled listener unit did not become active")
        PHASE = "unprivileged_snapshot"
        ordinary = snapshot(state, False)
        ordinary_fixture = fixture(ordinary)
        if ordinary_fixture["ownership"] == "attributed":
            raise RuntimeError("unprivileged scan unexpectedly attributed root fixture")
        if ordinary_fixture["owners"]:
            raise RuntimeError("unprivileged scan invented a process owner")
        if ordinary["quality"]["permission_denied"] < 1:
            raise RuntimeError("unprivileged attribution gap was not reported")
        PHASE = "privileged_read_only_snapshot"
        privileged = snapshot(state, True)
        privileged_fixture = fixture(privileged)
        owners = privileged_fixture["owners"]
        if privileged_fixture["ownership"] != "attributed" or len(owners) != 1:
            raise RuntimeError("privileged bounded reader did not attribute controlled listener")
        owner = owners[0]
        if owner["effective_uid"] != 0 or owner.get("systemd_unit") != UNIT or owner["start_time_ticks"] <= 0:
            raise RuntimeError("listener owner identity or unit attribution was incorrect")
        if ordinary["network_namespace"] != privileged["network_namespace"]:
            raise RuntimeError("snapshots did not observe the same network namespace")
        PHASE = "policy_evaluation"
        allowed_result = evaluate(state, "listener-allowed.json")
        if any(item.get("resource_id") == "fixture" for item in allowed_result["findings"]):
            raise RuntimeError("declared fixture produced a drift finding")
        drift_result = evaluate(state, "listener-drift.json")
        fixture_findings = [item for item in drift_result["findings"]
                            if item["conclusion"] == "unexpected_listener"
                            and item.get("actual", {}).get("local_port") == PORT]
        missing_findings = [item for item in drift_result["findings"]
                            if item["conclusion"] == "missing_listener"
                            and item.get("resource_id") == "missing-fixture"]
        if len(fixture_findings) != 1 or len(missing_findings) != 1:
            raise RuntimeError("controlled declaration drift was not explained")
        report = {
            "type": "listener_integration", "passed": True,
            "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "source_commit_at_build": source_commit, "worktree_dirty_at_build": source_dirty,
            "runner_sha256": RUNNER_SHA256,
            "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
            "network_namespace": privileged["network_namespace"], "boot_id": privileged["boot_id"],
            "fixture": {"family": privileged_fixture["family"],
                        "local_address": privileged_fixture["local_address"],
                        "local_port": privileged_fixture["local_port"],
                        "kernel_uid": privileged_fixture["kernel_uid"],
                        "ownership": privileged_fixture["ownership"], "owners": owners},
            "unprivileged_conclusion": ordinary_fixture["ownership"],
            "unprivileged_quality": ordinary["quality"],
            "privileged_quality": privileged["quality"],
            "policy_evaluation": {
                "allowed_fixture_findings": sum(1 for item in allowed_result["findings"]
                                                if item.get("resource_id") == "fixture"),
                "unexpected_fixture_finding_id": fixture_findings[0]["finding_id"],
                "missing_fixture_finding_id": missing_findings[0]["finding_id"]},
            "scope": "current guest network namespace only; no firewall, NAT, container publication, or reachability claim"
        }
        (lab_vm.VM / "listener.result.json").write_text(json.dumps(report, indent=2) + "\n")
        print(json.dumps(report))
    finally:
        remote(state, "sudo -n systemctl stop " + UNIT, check=False)
        remote(state, "sudo -n systemctl reset-failed " + UNIT, check=False)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        failure = {"type": "listener_integration_failure", "passed": False,
                   "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                   "phase": PHASE, "error_type": type(error).__name__,
                   "elapsed_seconds": round(time.monotonic() - STARTED, 2),
                   "runner_sha256": RUNNER_SHA256}
        if lab_vm.VM.is_dir():
            (lab_vm.VM / "listener.last-failure.json").write_text(json.dumps(failure, indent=2) + "\n")
        print(json.dumps(failure), file=sys.stderr)
        raise
