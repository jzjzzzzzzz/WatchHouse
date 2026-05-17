# Control event receiver

`watchhouse-control` exposes `POST /v1/events/batch` for agents and bounded
`GET /v1/events` for separately authorized humans. Every connection requires a
verified client certificate. Ingest derives host identity from the single
Watchhouse URI SAN, strictly decodes a maximum 8 MiB / 500-item batch, and
commits the whole batch to PostgreSQL before returning exact receipts.

```sh
watchhouse-control \
  --listen 0.0.0.0:8443 \
  --database-url-file /run/credentials/watchhouse-control.service/database-url \
  --roles-file /run/credentials/watchhouse-control.service/roles.json \
  --client-ca /run/credentials/watchhouse-control.service/agent-ca.pem \
  --tls-cert /run/credentials/watchhouse-control.service/server-cert.pem \
  --tls-key /run/credentials/watchhouse-control.service/server-key.pem \
  --server-name control.example
```

The database URL is read from an absolute regular file with no group write and
no access for other users; this accepts systemd's root-managed mode-0440
credential. It is never a CLI value. PostgreSQL connections have connect,
statement, lock, idle-transaction, and pool bounds. The embedded migration is protected by a transaction-scoped
advisory lock and refuses a database schema newer than the binary.

HTTP has header/read/write/idle limits and TLS 1.3 minimum. Application mTLS
must remain end to end: a reverse proxy may use TCP passthrough, but must not
replace the certificate identity with a caller-controlled header. The unit has
been exercised in the Ubuntu guest with systemd credentials and a dynamic user;
firewall, reverse-proxy and public-VPS deployment are not yet shipped.

Duplicate normalized content succeeds idempotently. A repeated identity with
changed content returns conflict and rolls back the complete batch. Database or
migration failure returns no receipts. The current database stores normalized
SSH events only; it is not a raw log archive or general blob endpoint.

Human queries use a distinct `spiffe://watchhouse/user/<user_id>` certificate
and a startup-loaded, non-symlink, non-writable role map. Agents cannot query,
humans cannot ingest, and unmapped humans are denied. `watchhouse query-events`
supports a bounded `--limit` and exclusive `--before` server sequence; neither
the human certificate nor query parameter can impersonate an agent host.
