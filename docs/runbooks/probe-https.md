# HTTPS network-path probe

Use a machine outside the service host when claiming external reachability:

```sh
watchhouse probe-https \
  --url https://service.example/health \
  --expect-status 200 \
  --timeout 15s \
  --max-body 65536 | jq .
```

The output identifies DNS answers, the pinned connected address, DNS/TCP/TLS/
total timings, TLS identity, HTTP status and body digest. It does not log the
body. Redirects are forbidden.

For a private lab, provide its CA and explicitly relax target-address policy:

```sh
watchhouse probe-https --url https://127.0.0.1:8443/health \
  --ca /absolute/path/lab-ca.pem --allow-private
```

The reproducible local acceptance command is:

```sh
python3 scripts/nginx-probe-integration.py
```

It verifies a non-root read-only Nginx TLS edge and bridge-only backend, injects
a backend outage, observes the gateway timeout, restarts the backend, verifies
recovery, and removes only its own uniquely labeled Docker resources.
