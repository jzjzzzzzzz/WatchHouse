#!/usr/bin/env python3
"""Lifecycle of one private, localhost-only QEMU integration guest."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import uuid

ROOT = Path(__file__).resolve().parents[1]
LOCAL = ROOT / "lab/local"
VM = LOCAL / "systemd-vm"
IMAGE = "watchhouse-vm-tools:24.04"
LABEL = "org.watchhouse.instance"


def run(argv, **kwargs):
    return subprocess.run(argv, check=True, capture_output=True, text=True, timeout=kwargs.pop("timeout", 60), **kwargs)


def tools(argv):
    return run(["docker", "run", "--rm", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
                "--user", f"{os.getuid()}:{os.getgid()}", "--network", "none", "--tmpfs", "/tmp:rw,noexec,nosuid,size=16m",
                "--mount", f"type=bind,source={LOCAL / 'cache'},target=/cache,readonly",
                "--mount", f"type=bind,source={VM},target=/state", IMAGE, *argv])


def cloud_config(operator, host_private, host_public):
    return {"users": [{"name": "watchhouse-lab", "groups": ["adm", "sudo"], "shell": "/bin/bash",
                       "sudo": ["ALL=(ALL) NOPASSWD:ALL"], "lock_passwd": True, "ssh_authorized_keys": [operator]}],
            "disable_root": True, "ssh_pwauth": False, "package_update": False, "package_upgrade": False,
            "ssh_deletekeys": True, "ssh_keys": {"ed25519_private": host_private, "ed25519_public": host_public},
            "write_files": [
                {"path": "/etc/ssh/sshd_config.d/80-watchhouse-lab.conf", "permissions": "0644",
                 "content": "LogLevel VERBOSE\nPasswordAuthentication no\nPermitRootLogin no\n"},
                {"path": "/etc/systemd/journald.conf.d/80-watchhouse-lab.conf", "permissions": "0644",
                 "content": "[Journal]\nStorage=persistent\nRuntimeMaxUse=16M\nSystemMaxUse=64M\n"}],
            "runcmd": [["systemctl", "restart", "systemd-journald.service"], ["systemctl", "restart", "ssh.service"]]}


def prepare():
    # VM roots are never reinitialized over existing state or credentials.
    VM.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
    VM.mkdir(mode=0o700)
    image = LOCAL / "cache/noble-arm64.img"
    if not image.is_file():
        raise ValueError("run scripts/vm_image.py first")
    image_spec = json.loads((ROOT / "lab/vm/images.json").read_text())["guest"]
    from vm_image import digest_file
    if image.is_symlink() or digest_file(image) != image_spec["sha256"]:
        raise ValueError("guest image checksum changed; refusing prepare")
    instance = "watchhouse-" + uuid.uuid4().hex
    for name in ("operator", "host", "rejected"):
        run(["ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "watchhouse-lab-" + name, "-f", str(VM / name)])
    config = cloud_config((VM / "operator.pub").read_text().strip(), (VM / "host").read_text(), (VM / "host.pub").read_text().strip())
    (VM / "user-data").write_text("#cloud-config\n" + json.dumps(config, indent=2) + "\n")
    (VM / "meta-data").write_text(json.dumps({"instance-id": instance, "local-hostname": "watchhouse-lab"}) + "\n")
    for name in ("user-data", "meta-data"):
        (VM / name).chmod(0o600)
    tools(["cloud-localds", "/state/seed.img", "/state/user-data", "/state/meta-data"])
    tools(["qemu-img", "create", "-f", "qcow2", "-F", "qcow2", "-b", "/cache/noble-arm64.img", "/state/disk.qcow2", "8G"])
    tools(["cp", "/usr/share/AAVMF/AAVMF_VARS.fd", "/state/vars.fd"])
    for name in ("seed.img", "disk.qcow2", "vars.fd"):
        (VM / name).chmod(0o600)
    (VM / "configuration.json").write_text(json.dumps({"instance": instance, "image_sha256": image_spec["sha256"]}) + "\n")
    (VM / "configuration.json").chmod(0o600)
    return {"prepared": True, "instance": instance}


def inspect():
    state = json.loads((VM / "runtime.json").read_text())
    if not re.fullmatch(r"[0-9a-f]{64}", state["container"]):
        raise ValueError("invalid saved container identity")
    details = json.loads(run(["docker", "inspect", state["container"]]).stdout)[0]
    configured = json.loads((VM / "configuration.json").read_text())
    if details["Config"]["Labels"].get(LABEL) != configured["instance"]:
        raise ValueError("container instance label does not match; refusing operation")
    return state, details


def start():
    if (VM / "runtime.json").exists():
        raise ValueError("runtime already exists; inspect its actual status instead of restarting")
    configuration = json.loads((VM / "configuration.json").read_text())
    tools_info = json.loads(run(["docker", "image", "inspect", IMAGE]).stdout)[0]
    if tools_info["Config"]["Labels"].get("org.watchhouse.component") != "lab-vm-tools":
        raise ValueError("unrecognized VM tools image")
    args = ["docker", "run", "-d", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges",
            "--memory", "2g", "--cpus", "3", "--pids-limit", "128", "--user", f"{os.getuid()}:{os.getgid()}",
            "--label", LABEL + "=" + configuration["instance"], "--publish", "127.0.0.1::2222",
            "--tmpfs", "/tmp:rw,noexec,nosuid,size=32m",
            "--mount", f"type=bind,source={LOCAL / 'cache'},target=/cache,readonly",
            "--mount", f"type=bind,source={VM},target=/state", tools_info["Id"],
            "qemu-system-aarch64", "-machine", "virt", "-cpu", "max", "-accel", "tcg", "-smp", "2", "-m", "1024",
            "-nographic", "-monitor", "none",
            "-drive", "if=pflash,format=raw,readonly=on,file=/usr/share/AAVMF/AAVMF_CODE.fd",
            "-drive", "if=pflash,format=raw,file=/state/vars.fd",
            "-drive", "if=virtio,format=qcow2,file=/state/disk.qcow2",
            "-drive", "if=virtio,format=raw,readonly=on,file=/state/seed.img",
            "-netdev", "user,id=net,hostfwd=tcp:0.0.0.0:2222-:22", "-device", "virtio-net-pci,netdev=net,romfile="]
    container = run(args).stdout.strip()
    details = json.loads(run(["docker", "inspect", container]).stdout)[0]
    bindings = details["NetworkSettings"]["Ports"]["2222/tcp"]
    if len(bindings) != 1 or bindings[0]["HostIp"] != "127.0.0.1":
        raise ValueError("SSH publication is not exclusively localhost")
    port = int(bindings[0]["HostPort"])
    (VM / "known_hosts").write_text(f"[127.0.0.1]:{port} " + (VM / "host.pub").read_text().strip() + "\n")
    (VM / "known_hosts").chmod(0o600)
    state = {"container": container, "port": port, "tools_image": tools_info["Id"]}
    (VM / "runtime.json").write_text(json.dumps(state) + "\n")
    (VM / "runtime.json").chmod(0o600)
    return state


def restart():
    state, details = inspect()
    if details["State"]["Running"] or details["State"]["Restarting"]:
        raise ValueError("refusing to restart a running guest; inspect/probe it or stop explicitly")
    # Preserve previous runtime evidence; remove only our verified, terminal
    # container, never the overlay, credentials, or unrelated workloads.
    run(["docker", "rm", state["container"]])
    previous = VM / ("runtime.previous." + state["container"][:12] + ".json")
    (VM / "runtime.json").rename(previous)
    return start()


def ssh_args(state, identity="operator"):
    return ["ssh", "-F", "/dev/null", "-i", str(VM / identity), "-p", str(state["port"]),
            "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "IdentityAgent=none",
            "-o", "ForwardAgent=no", "-o", "ClearAllForwardings=yes", "-o", "ConnectTimeout=5",
            "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=" + str(VM / "known_hosts"),
            "watchhouse-lab@127.0.0.1"]


def probe():
    state, details = inspect()
    if not details["State"]["Running"]:
        return {"running": False, "exit_code": details["State"]["ExitCode"]}
    command = [*ssh_args(state), "systemctl is-active ssh.service && test -f /var/lib/cloud/instance/boot-finished && uname -sr"]
    try:
        answer = subprocess.run(command, capture_output=True, text=True, timeout=15)
    except subprocess.TimeoutExpired:
        _, current = inspect()
        return {"running": current["State"]["Running"], "ssh_ready": False,
                "diagnostic": "SSH readiness probe timed out; no automatic restart"}
    return {"running": True, "ssh_ready": answer.returncode == 0, "probe": answer.stdout.strip(), "diagnostic": answer.stderr.strip()}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("action", choices=("prepare", "start", "status", "probe", "stop", "restart"))
    args = parser.parse_args()
    if args.action == "prepare":
        result = prepare()
    elif args.action == "start":
        result = start()
    elif args.action == "probe":
        result = probe()
    elif args.action == "restart":
        result = restart()
    else:
        state, details = inspect()
        if args.action == "stop":
            run(["docker", "stop", "--time", "20", state["container"]])
            state, details = inspect()
        result = {"container": state["container"], "state": details["State"], "port": state["port"]}
    print(json.dumps(result))


if __name__ == "__main__":
    main()
