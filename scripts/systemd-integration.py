#!/usr/bin/env python3
"""Exercise native read-only collectors against our verified local Ubuntu VM."""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time

import lab_vm

ROOT = lab_vm.ROOT
STATE = "/home/watchhouse-lab/watchhouse-state"
PROGRAM = "/home/watchhouse-lab/watchhouse"
RUNNER_SHA256 = hashlib.sha256(Path(__file__).read_bytes()).hexdigest()


def remote(state, command, check=True):
    return subprocess.run([*lab_vm.ssh_args(state), command], check=check, capture_output=True, text=True, timeout=45)


def wait_ready(timeout=240, previous_boot=None):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        state, details = lab_vm.inspect()
        if not details["State"]["Running"]:
            raise RuntimeError("VM container stopped; inspect logs, do not restart blindly")
        result = lab_vm.probe()
        if result.get("ssh_ready"):
            boot = remote(state, "cat /proc/sys/kernel/random/boot_id").stdout.strip()
            if previous_boot is None or boot != previous_boot:
                return state, boot
        time.sleep(5)
    raise TimeoutError("SSH/guest readiness timed out; running container preserved")


def decode(result):
    return json.loads(result.stdout)


def pending_events(state):
    result = remote(state, PROGRAM + " spool peek --state " + STATE + " --limit 500")
    items = [json.loads(line) for line in result.stdout.splitlines()]
    if len(items) == 500:
        raise RuntimeError("test evidence snapshot reached batch ceiling; do not infer complete coverage")
    return {item["event_id"]: item["event"] for item in items}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--go", default=os.environ.get("GO", "go"))
    parser.add_argument("--reboot", action="store_true", help="reboot only the isolated guest and verify persistent resume")
    args = parser.parse_args()
    state, initial_boot = wait_ready()
    build_commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    build_dirty = bool(subprocess.check_output(["git", "status", "--porcelain"], cwd=ROOT, text=True).strip())
    binary = ROOT / "bin/watchhouse-linux-arm64"
    environment = dict(os.environ, GOOS="linux", GOARCH="arm64", CGO_ENABLED="0")
    subprocess.run([args.go, "build", "-trimpath", "-o", str(binary), "./cmd/watchhouse"], cwd=ROOT, env=environment, check=True, timeout=180)
    scp = ["scp", "-F", "/dev/null", "-i", str(lab_vm.VM / "operator"), "-P", str(state["port"]),
           "-o", "IdentitiesOnly=yes", "-o", "IdentityAgent=none", "-o", "StrictHostKeyChecking=yes",
           "-o", "UserKnownHostsFile=" + str(lab_vm.VM / "known_hosts"), str(binary),
           "watchhouse-lab@127.0.0.1:" + PROGRAM]
    subprocess.run(scp, check=True, capture_output=True, text=True, timeout=120)
    versions = remote(state, "uname -sr; systemd --version | head -1; ssh -V 2>&1; id -u; id -Gn").stdout.strip().splitlines()
    first = decode(remote(state, PROGRAM + " collect --host vm-staging --state " + STATE + " --limit 1000"))
    before_ids = set(pending_events(state))
    rejected = 0
    for _ in range(5):
        attempt = subprocess.run([*lab_vm.ssh_args(state, "rejected"), "true"], capture_output=True, text=True, timeout=15)
        if attempt.returncode != 255 or "Permission denied" not in attempt.stderr:
            raise RuntimeError("rejected-key scenario failed for a reason other than authentication")
        rejected += 1
    second = decode(remote(state, PROGRAM + " collect --host vm-staging --state " + STATE + " --limit 1000"))
    pending = pending_events(state)
    current_failures = {identity for identity, event in pending.items() if identity not in before_ids
                        and event["authentication"]["outcome"] == "failed"
                        and event["authentication"]["method"] == "publickey"
                        and event["authentication"]["user"] == "watchhouse-lab"}
    if len(current_failures) != 5:
        raise RuntimeError("this test run did not persist exactly five rejected-key events")
    status = decode(remote(state, PROGRAM + " spool status --state " + STATE))
    audit = decode(remote(state, PROGRAM + " spool check --state " + STATE))
    if not first["stats"]["complete"] or not second["stats"]["complete"] or not audit["result"]["valid"]:
        raise RuntimeError("native collection or audit incomplete")
    raw = remote(state, "journalctl --no-pager --output=json --lines=2000 _COMM=sshd + _COMM=sshd-session").stdout
    raw_path = lab_vm.VM / "journal.private.jsonl"
    raw_path.write_text(raw)
    raw_path.chmod(0o600)
    local_binary = ROOT / "bin/watchhouse"
    subprocess.run([args.go, "build", "-trimpath", "-o", str(local_binary), "./cmd/watchhouse"], cwd=ROOT, check=True, timeout=180)
    replay = subprocess.run([str(local_binary), "replay", "--host", "vm-staging", "--input", str(raw_path)], check=True, capture_output=True, text=True, timeout=30)
    replay_path = lab_vm.VM / "replay.private.jsonl"
    replay_path.write_text(replay.stdout)
    replay_path.chmod(0o600)
    summary = json.loads(replay.stderr)
    findings = [item["finding"] for item in map(json.loads, replay.stdout.splitlines()) if item["type"] == "finding"]
    if not any(set(finding["evidence_event_ids"][:-1]) == current_failures
               and finding["evidence_event_ids"][-1] in pending
               and finding["evidence_event_ids"][-1] not in before_ids for finding in findings):
        raise RuntimeError("current rejected-key evidence was not linked to a new successful authentication")
    small = remote(state, PROGRAM + " collect --host vm-small --state /home/watchhouse-lab/small-state --max-records 1 --limit 1000", check=False)
    if small.returncode != 1 or "capacity reached" not in small.stderr:
        raise RuntimeError("real journal capacity scenario did not stop explicitly")
    small_status = decode(remote(state, PROGRAM + " spool status --state /home/watchhouse-lab/small-state --max-records 1"))
    if small_status["stats"]["pending_records"] != 1 or small_status["stats"]["blocked_attempts"] < 1:
        raise RuntimeError("capacity violated or blockage not observed")
    reboot = None
    if args.reboot:
        before = decode(remote(state, PROGRAM + " spool status --state " + STATE))["stats"]["pending_records"]
        remote(state, "sudo -n reboot", check=False)
        state, new_boot = wait_ready(timeout=240, previous_boot=initial_boot)
        after = decode(remote(state, PROGRAM + " spool status --state " + STATE))["stats"]["pending_records"]
        if before != after:
            raise RuntimeError("reboot lost or duplicated pending records")
        continued = decode(remote(state, PROGRAM + " collect --host vm-staging --state " + STATE + " --limit 1000"))
        if not continued["stats"]["complete"]:
            raise RuntimeError("persistent journal cursor failed after guest reboot")
        reboot = dict(boot_changed=initial_boot != new_boot, pending_before=before, pending_after=after, continued=continued["stats"])
    report = {"type": "systemd_integration", "passed": True,
              "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "source_commit": build_commit, "worktree_dirty_at_build": build_dirty,
              "report_end_commit": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip(),
              "runner_sha256": RUNNER_SHA256,
              "guest_versions": versions, "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
              "first_collect": first["stats"], "second_collect": second["stats"], "rejected_key_attempts": rejected,
              "current_failure_evidence_ids": sorted(current_failures),
              "pending_records": status["stats"]["pending_records"], "audit": audit["result"],
              "replay": summary["stats"], "backpressure": small_status["stats"], "reboot": reboot,
              "scope": "isolated real Linux/systemd VM; not public VPS or production accuracy"}
    (lab_vm.VM / "integration.result.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report))


if __name__ == "__main__":
    main()
