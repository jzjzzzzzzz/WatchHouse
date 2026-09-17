#!/usr/bin/env python3
"""Exercise the Linux Docker-port collector against a real published fixture."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import time

ROOT = Path(__file__).resolve().parents[1]
CLI = json.loads((ROOT / "lab/docker/images.json").read_text())["docker_cli"]
NGINX = json.loads((ROOT / "lab/nginx/images.json").read_text())["nginx"]
LABEL = "org.watchhouse.integration=docker-ports"


def run(argv, **kwargs):
    return subprocess.run(argv, check=True, capture_output=True, text=True,
                          timeout=kwargs.pop("timeout", 120), **kwargs)


def main():
    go = os.environ.get("GO", "go")
    source = run(["git", "rev-parse", "HEAD"], cwd=ROOT).stdout.strip()
    dirty = bool(run(["git", "status", "--porcelain"], cwd=ROOT).stdout.strip())
    binary = ROOT / "bin/watchhouse-linux-arm64"
    environment = dict(os.environ, CGO_ENABLED="0", GOOS="linux", GOARCH="arm64")
    run([go, "build", "-trimpath", "-o", str(binary), "./cmd/watchhouse"], cwd=ROOT, env=environment, timeout=180)
    name = "watchhouse-port-fixture-" + secrets.token_hex(5)
    container = None
    started = time.monotonic()
    try:
        container = run(["docker", "run", "-d", "--name", name, "--label", LABEL,
                         "--publish", "127.0.0.1::80", "--read-only", "--cap-drop=ALL",
                         "--security-opt", "no-new-privileges", "--tmpfs", "/var/cache/nginx:size=16m",
                         "--tmpfs", "/var/run:size=1m", NGINX["reference"]], timeout=180).stdout.strip()
        details = json.loads(run(["docker", "inspect", container]).stdout)[0]
        binding = details["NetworkSettings"]["Ports"]["80/tcp"]
        if len(binding) != 1 or binding[0]["HostIp"] != "127.0.0.1":
            raise RuntimeError("fixture was not loopback-only")
        command = ["docker", "run", "--rm", "--network=none", "--read-only", "--cap-drop=ALL",
                   "--security-opt", "no-new-privileges", "--pids-limit", "128", "--memory", "128m",
                   "--mount", "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock",
                   "--mount", "type=bind,source=" + str(binary) + ",target=/watchhouse,readonly",
                   "--env", "DOCKER_HOST=unix:///var/run/docker.sock", CLI["reference"],
                   "/watchhouse", "docker-ports"]
        observed = json.loads(run(command, timeout=120).stdout)
        matches = [item for item in observed["bindings"] if item["container_id"] == container and
                   item["container_port"] == 80 and item["host_ip"] == "127.0.0.1" and
                   item["host_port"] == int(binding[0]["HostPort"])]
        if len(matches) != 1:
            raise RuntimeError("collector did not return the exact fixture binding")
        report = {"schema_version": 1, "type": "docker_ports_integration", "passed": True,
                  "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                  "source_commit": source, "worktree_dirty_at_start": dirty,
                  "runner_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
                  "binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
                  "docker_cli_image": CLI["reference"], "fixture_image": NGINX["reference"],
                  "fixture": {"container_id": container, "name": name, "host_ip": "127.0.0.1",
                              "host_port": int(binding[0]["HostPort"]), "container_port": 80},
                  "observed_container_count": observed["container_count"],
                  "observed_binding_count": observed["published_binding_count"],
                  "elapsed_seconds": round(time.monotonic() - started, 2),
                  "scope": "Docker Desktop Linux VM; daemon socket is root-equivalent; not public reachability"}
        output = ROOT / "lab/local/docker-ports.result.json"
        output.write_text(json.dumps(report, indent=2) + "\n"); output.chmod(0o600)
        print(json.dumps(report))
    finally:
        if container:
            inspected = subprocess.run(["docker", "inspect", container], capture_output=True, text=True, timeout=30)
            if inspected.returncode == 0:
                details = json.loads(inspected.stdout)[0]
                if details["Name"] != "/" + name or details["Config"]["Labels"].get("org.watchhouse.integration") != "docker-ports":
                    raise RuntimeError("refusing to remove mismatched container")
                run(["docker", "rm", "-f", container], timeout=60)


if __name__ == "__main__":
    main()
