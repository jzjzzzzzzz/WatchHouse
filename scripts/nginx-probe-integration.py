#!/usr/bin/env python3
"""Exercise a pinned non-root Nginx TLS proxy and external probe."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time

ROOT = Path(__file__).resolve().parents[1]
IMAGE = json.loads((ROOT / "lab/nginx/images.json").read_text())["nginx"]["reference"]
LABEL = "org.watchhouse.integration=nginx-probe"


def run(argv, **kwargs):
    return subprocess.run(argv, check=True, capture_output=True, text=True,
                          timeout=kwargs.pop("timeout", 120), **kwargs)


def main():
    go = os.environ.get("GO", "go")
    source_commit = run(["git", "rev-parse", "HEAD"], cwd=ROOT).stdout.strip()
    source_dirty = bool(run(["git", "status", "--porcelain"], cwd=ROOT).stdout.strip())
    token = secrets.token_hex(6)
    network = "watchhouse-probe-" + token
    backend = "watchhouse-backend-" + token
    edge = "watchhouse-edge-" + token
    created = []
    started = time.monotonic()
    local = ROOT / "lab/local"
    local.mkdir(mode=0o700, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="nginx-probe-", dir=local) as directory:
        directory = Path(directory)
        backend_conf = directory / "backend.conf"
        edge_conf = directory / "edge.conf"
        key = directory / "edge.key"
        certificate = directory / "edge.crt"
        backend_conf.write_text("""pid /tmp/nginx.pid;
events { worker_connections 128; }
http { access_log /dev/stdout; error_log /dev/stderr notice;
  server { listen 8080; location = /health { add_header X-Watchhouse-Backend backend-1 always; return 200 'watchhouse-backend-ok\\n'; } }
}
""")
        edge_conf.write_text("""pid /tmp/nginx.pid;
events { worker_connections 128; }
http { access_log /dev/stdout; error_log /dev/stderr notice;
  server { listen 8443 ssl; ssl_certificate /etc/nginx/tls/edge.crt; ssl_certificate_key /etc/nginx/tls/edge.key;
    ssl_protocols TLSv1.2 TLSv1.3; location = /health { proxy_connect_timeout 2s; proxy_read_timeout 2s; proxy_pass http://backend:8080; proxy_set_header Host $host; proxy_set_header X-Forwarded-Proto https; } }
}
""")
        backend_conf.chmod(0o644)
        edge_conf.chmod(0o644)
        run(["openssl", "req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:P-256", "-nodes",
             "-days", "1", "-subj", "/CN=127.0.0.1", "-addext", "subjectAltName=IP:127.0.0.1",
             "-keyout", str(key), "-out", str(certificate)])
        # The temporary parent is 0700. The read-only bind must be readable by
        # the container's remapped non-root UID; the directory is deleted after
        # the run and the key is never published or copied into the image.
        key.chmod(0o444)
        certificate.chmod(0o644)
        agent = ROOT / "bin/watchhouse"
        run([go, "build", "-trimpath", "-o", str(agent), "./cmd/watchhouse"], cwd=ROOT, timeout=180)
        try:
            run(["docker", "pull", IMAGE], timeout=300)
            image = json.loads(run(["docker", "image", "inspect", IMAGE]).stdout)[0]
            run(["docker", "network", "create", "--label", LABEL, network])
            created.append(("network", network))
            common = ["--read-only", "--user", "101:101", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
                      "--pids-limit", "64", "--memory", "128m", "--cpus", "0.5", "--tmpfs", "/tmp:rw,nosuid,nodev,size=8m",
                      "--tmpfs", "/var/cache/nginx:rw,nosuid,nodev,size=16m,uid=101,gid=101,mode=0700",
                      "--network", network, "--label", LABEL]
            run(["docker", "run", "-d", "--name", backend, "--network-alias", "backend", *common,
                 "--mount", "type=bind,source=" + str(backend_conf) + ",target=/etc/nginx/nginx.conf,readonly", IMAGE])
            created.append(("container", backend))
            backend_state = json.loads(run(["docker", "inspect", backend]).stdout)[0]
            if not backend_state["State"]["Running"]:
                logs = subprocess.run(["docker", "logs", backend], capture_output=True, text=True, timeout=20)
                raise RuntimeError("backend exited: " + (logs.stderr + logs.stdout)[-4096:])
            time.sleep(1)
            run(["docker", "run", "-d", "--name", edge, *common, "--publish", "127.0.0.1::8443",
                 "--mount", "type=bind,source=" + str(edge_conf) + ",target=/etc/nginx/nginx.conf,readonly",
                 "--mount", "type=bind,source=" + str(certificate) + ",target=/etc/nginx/tls/edge.crt,readonly",
                 "--mount", "type=bind,source=" + str(key) + ",target=/etc/nginx/tls/edge.key,readonly", IMAGE])
            created.append(("container", edge))
            details = json.loads(run(["docker", "inspect", edge]).stdout)[0]
            if not details["State"]["Running"]:
                logs = subprocess.run(["docker", "logs", edge], capture_output=True, text=True, timeout=20)
                raise RuntimeError("TLS edge exited: " + (logs.stderr + logs.stdout)[-4096:])
            bindings = details["NetworkSettings"]["Ports"].get("8443/tcp")
            if len(bindings) != 1 or bindings[0]["HostIp"] != "127.0.0.1":
                raise RuntimeError("TLS edge was not exclusively loopback published")
            port = int(bindings[0]["HostPort"])
            url = f"https://127.0.0.1:{port}/health"
            deadline = time.monotonic() + 30
            success = None
            while time.monotonic() < deadline:
                probe = subprocess.run([str(agent), "probe-https", "--url", url, "--ca", str(certificate), "--allow-private"],
                                       capture_output=True, text=True, timeout=20)
                if probe.returncode == 0:
                    success = json.loads(probe.stdout)
                    break
                time.sleep(0.5)
            if success is None:
                raise RuntimeError("healthy reverse proxy did not become probeable")
            expected_body = hashlib.sha256(b"watchhouse-backend-ok\n").hexdigest()
            if success["body_sha256"] != expected_body or success["http_status"] != 200:
                raise RuntimeError("probe did not verify backend response content")
            backend_details = json.loads(run(["docker", "inspect", backend]).stdout)[0]
            backend_ports = backend_details["NetworkSettings"]["Ports"] or {}
            if any(bindings for bindings in backend_ports.values()):
                raise RuntimeError("backend unexpectedly published a host port")
            run(["docker", "stop", "--time", "2", backend])
            failed = subprocess.run([str(agent), "probe-https", "--url", url, "--ca", str(certificate), "--allow-private", "--expect-status", "200"],
                                    capture_output=True, text=True, timeout=20)
            if failed.returncode != 1 or not failed.stdout:
                raise RuntimeError("backend outage probe mismatch: rc=" + str(failed.returncode) + " stderr=" + failed.stderr[-2048:] + " stdout=" + failed.stdout[-2048:])
            failure = json.loads(failed.stdout)
            if failure["http_status"] not in (502, 504) or failure["expected"]:
                raise RuntimeError("backend outage evidence mismatch: " + json.dumps(failure))
            run(["docker", "start", backend])
            deadline = time.monotonic() + 30
            recovered = None
            while time.monotonic() < deadline:
                probe = subprocess.run([str(agent), "probe-https", "--url", url, "--ca", str(certificate), "--allow-private"],
                                       capture_output=True, text=True, timeout=20)
                if probe.returncode == 0:
                    recovered = json.loads(probe.stdout)
                    break
                time.sleep(0.5)
            if recovered is None or recovered["body_sha256"] != expected_body:
                raise RuntimeError("reverse proxy did not recover after backend restart")
            report = {"type": "nginx_probe_integration", "passed": True,
                      "actual_execution_time_utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                      "elapsed_seconds": round(time.monotonic() - started, 2), "source_commit": source_commit,
                      "worktree_dirty_at_start": source_dirty, "runner_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
                      "image_reference": IMAGE, "image_id": image["Id"], "platform": image["Architecture"],
                      "edge_publication": "dynamic 127.0.0.1-only port", "backend_publication": "none; user-defined bridge only",
                      "container_user": "101:101", "capabilities": "all dropped", "no_new_privileges": True, "read_only_rootfs": True,
                      "healthy": success, "outage": failure, "recovered": recovered,
                      "scope": "local Docker bridge and loopback probe; not public Internet, host firewall, or NAT evidence"}
            output = local / "nginx-probe.result.json"
            output.write_text(json.dumps(report, indent=2) + "\n")
            output.chmod(0o600)
            print(json.dumps(report))
        finally:
            for kind, name in reversed(created):
                if kind == "container":
                    inspected = subprocess.run(["docker", "inspect", name], capture_output=True, text=True, timeout=20)
                    if inspected.returncode == 0:
                        details = json.loads(inspected.stdout)[0]
                        if details["Config"]["Labels"].get("org.watchhouse.integration") != "nginx-probe":
                            raise RuntimeError("refusing cleanup of mismatched container")
                        run(["docker", "rm", "-f", name], timeout=30)
                else:
                    inspected = subprocess.run(["docker", "network", "inspect", name], capture_output=True, text=True, timeout=20)
                    if inspected.returncode == 0:
                        details = json.loads(inspected.stdout)[0]
                        if details["Labels"].get("org.watchhouse.integration") != "nginx-probe":
                            raise RuntimeError("refusing cleanup of mismatched network")
                        run(["docker", "network", "rm", name], timeout=30)


if __name__ == "__main__":
    main()
