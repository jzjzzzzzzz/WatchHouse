# Nginx reverse-proxy and network-path acceptance — 2026-10-05 local time

A clean-worktree run used the digest-pinned Docker Official Image for Nginx
1.30.5 to start two containers on a uniquely named user-defined bridge. Both
ran as UID/GID 101:101 with all capabilities dropped, `no-new-privileges`, a
read-only root filesystem, bounded memory/PIDs/CPU, and only explicit tmpfs
write locations.

The backend served HTTP health on the bridge and had no host port binding. The
edge alone exposed TLS on a dynamic `127.0.0.1` port, terminated a generated
one-run certificate, and reverse-proxied `/health` to the backend.

The actual `watchhouse probe-https` binary independently resolved the target,
pinned the selected address for the TCP connection, verified TLS 1.3 and the
certificate, recorded the cipher and certificate SHA-256, required HTTP 200,
and hashed the bounded 22-byte backend body. The harness then stopped the
backend. The same TLS edge returned HTTP 504 after its bounded upstream timeout;
the probe returned nonzero while still emitting structured failure evidence.
After the backend restarted, a third probe recovered HTTP 200 with the original
body digest and certificate identity.

See `nginx-probe.json` for timings, identities, image digest, healthy/outage/
recovered observations, and isolation claims. Actual execution was 2026-10-05
23:11:12 EDT.

This is strong evidence for local container isolation, Nginx TLS termination,
reverse proxy behavior, and failure diagnosis. The edge was loopback-only. It
does **not** prove public Internet reachability, VPS firewall/security-group
policy, NAT behavior, or continuous monitoring from a second machine.
