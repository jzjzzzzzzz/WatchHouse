# Control event receiver

`watchhouse-control` exposes only `POST /v1/events/batch` and requires a verified
agent certificate during the TLS handshake. It derives host identity from the
single Watchhouse URI SAN, strictly decodes a maximum 8 MiB / 500-item batch,
and commits the whole batch to PostgreSQL before returning exact receipts.

```sh
watchhouse-control \
  --listen 0.0.0.0:8443 \
  --database-url-file /run/credentials/watchhouse-control.service/database-url \
  --client-ca /run/credentials/watchhouse-control.service/agent-ca.pem \
  --tls-cert /run/credentials/watchhouse-control.service/server-cert.pem \
  --tls-key /run/credentials/watchhouse-control.service/server-key.pem \
  --server-name control.example
```

The database URL is read from an absolute mode-0600 regular file, never a CLI
value. PostgreSQL connections have connect, statement, lock, idle-transaction,
and pool bounds. The embedded migration is protected by a transaction-scoped
advisory lock and refuses a database schema newer than the binary.

HTTP has header/read/write/idle limits and TLS 1.3 minimum. Application mTLS
must remain end to end: a reverse proxy may use TCP passthrough, but must not
replace the certificate identity with a caller-controlled header. Firewall and
proxy deployment are not yet shipped, and the current process integration is
loopback only.

Duplicate normalized content succeeds idempotently. A repeated identity with
changed content returns conflict and rolls back the complete batch. Database or
migration failure returns no receipts. The current database stores normalized
SSH events only; it is not a raw log archive or general blob endpoint.
