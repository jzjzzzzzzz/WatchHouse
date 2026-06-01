# Nginx reverse-proxy lab

`images.json` pins the Docker Official Image by its multi-platform OCI index.
`scripts/nginx-probe-integration.py` creates a uniquely named, labeled Docker
network and two non-root containers. Only the TLS edge is published, and only to
a dynamic `127.0.0.1` port. The backend remains bridge-only.

The lab verifies DNS/address policy, TCP, TLS certificate validation, HTTP
status and body digest, then stops the backend to observe a real proxy failure
and restarts it to prove recovery. This is local loopback evidence, not public
Internet or firewall/NAT evidence.
