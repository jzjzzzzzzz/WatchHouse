# Bounded event delivery

`watchhouse deliver` peeks one SQLite batch, sends it to the fixed v1 HTTPS
endpoint, validates an exact receipt set, and only then deletes the corresponding
local `(sequence,event_id)` pairs. It is a bounded oneshot intended for the
non-overlapping systemd timer, not a streaming daemon.

```sh
watchhouse deliver \
  --state /var/lib/watchhouse \
  --endpoint https://control.example:8443 \
  --ca /run/credentials/watchhouse-deliver.service/agent-ca.pem \
  --cert /run/credentials/watchhouse-deliver.service/agent-cert.pem \
  --key /run/credentials/watchhouse-deliver.service/agent-key.pem \
  --server-name control.example
```

The client certificate must contain exactly one URI SAN
`spiffe://watchhouse/host/<host_id>`. The server name and private CA are always
verified; redirects, environment proxy discovery, insecure TLS, extra receipt
identities, and partial receipt sets are rejected. Certificate identity—not a
request field—authorizes every event host ID.

TLS material uses absolute, non-symlink paths. Private keys must have no group or
other permission bits and all credential files are size bounded. The shipped
unit uses systemd `LoadCredential`; neither keys nor database URLs belong in
unit command lines, environment files, Git, or logs.

On timeout, TLS failure, non-200 status, invalid response, or local Ack failure,
unacknowledged items remain. A response can be lost after PostgreSQL commit, so
retries are expected. The server's `(authenticated_host,event_id)` uniqueness
makes identical retries idempotent; changed content is a conflict.

The command reports only counts and authenticated host identity. Inspect
`spool status` and `spool check` after a failed run. Do not manually Ack records
because an operator believes the server probably received them.
