"""Ephemeral private CA fixtures for local integration tests only."""
from pathlib import Path
import subprocess


def run(argv):
    return subprocess.run(argv, check=True, capture_output=True, text=True, timeout=30)


def generate(directory, host="process-host", viewer="alice", unknown="eve", server_name="control.test"):
    directory = Path(directory)
    ca_key, ca = directory / "ca-key.pem", directory / "ca.pem"
    server_key, server_csr, server_cert = directory / "server-key.pem", directory / "server.csr", directory / "server.pem"
    identities = {
        "agent": (host, "host"),
        "viewer": (viewer, "user"),
        "unknown": (unknown, "user"),
    }
    run(["openssl", "genpkey", "-algorithm", "EC", "-pkeyopt", "ec_paramgen_curve:P-256", "-out", str(ca_key)])
    run(["openssl", "req", "-x509", "-new", "-key", str(ca_key), "-sha256", "-days", "1",
         "-subj", "/CN=Watchhouse process integration CA", "-out", str(ca)])
    entries = [("control", server_key, server_csr, server_cert,
                "subjectAltName=DNS:" + server_name + "\nextendedKeyUsage=serverAuth\n")]
    result = {"ca": ca, "server_key": server_key, "server_cert": server_cert}
    for name, (identity, kind) in identities.items():
        key, csr, certificate = directory / (name + "-key.pem"), directory / (name + ".csr"), directory / (name + ".pem")
        extension = "subjectAltName=URI:spiffe://watchhouse/" + kind + "/" + identity + "\nextendedKeyUsage=clientAuth\n"
        entries.append((name, key, csr, certificate, extension))
        result[name + "_key"], result[name + "_cert"] = key, certificate
    for name, key, csr, certificate, extension in entries:
        run(["openssl", "genpkey", "-algorithm", "EC", "-pkeyopt", "ec_paramgen_curve:P-256", "-out", str(key)])
        run(["openssl", "req", "-new", "-key", str(key), "-subj", "/CN=" + name, "-out", str(csr)])
        ext = directory / (name + ".ext")
        ext.write_text(extension)
        run(["openssl", "x509", "-req", "-in", str(csr), "-CA", str(ca), "-CAkey", str(ca_key),
             "-CAcreateserial", "-days", "1", "-sha256", "-extfile", str(ext), "-out", str(certificate)])
    for path in (ca_key, server_key, result["agent_key"], result["viewer_key"], result["unknown_key"]):
        path.chmod(0o600)
    return result
