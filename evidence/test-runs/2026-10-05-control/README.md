# PostgreSQL control-store acceptance — 2026-10-05 local time

A clean-worktree run built the agent and control binaries, generated an ephemeral private CA, and started the Docker Official Image for PostgreSQL 18.6,
pinned by multi-platform OCI index digest, on a dynamic loopback-only port with
data on a bounded disposable tmpfs. A random password was passed through a
mode-0600 mounted file and was not included in the report or command arguments.

The real database and mutual-TLS run proved:

- embedded schema migration succeeds and a second migration is idempotent;
- retrying the same authenticated host/event content keeps one row;
- a two-item batch that inserts one new event and then encounters a conflicting
  existing identity rolls the whole transaction back;
- eight concurrent identical batch commits converge on one stored event;
- the resulting count is two distinct events, not ten delivery attempts;
- two SQLite spool records cross real TLS 1.3 client authentication and are deleted only after exact receipts;
- a deliberately corrupted response after remote commit leaves the third event pending, and retry acknowledges it without increasing the PostgreSQL count;
- the actual agent CLI delivered seven fixture events to the actual control process, received seven receipts, drained the SQLite queue, and left seven PostgreSQL rows;
- the control process derived one durable SSH finding with six event IDs, and the authenticated viewer retrieved that evidence through the finding API;
- reusing that certificate for a spool claiming `claimed-host` failed, left all seven local records pending, and inserted zero rows for the claimed identity;
- a viewer certificate mapped to `alice` returned two exclusive three-record pages, while a valid but unmapped `eve` certificate was denied;
- PostgreSQL recorded three allowed decisions and the unmapped-user denial in an append-only audit table;
- a host-bound listener snapshot crossed TLS 1.3, received its exact snapshot ID, and left exactly one PostgreSQL row;
- a distinct `outside-1` probe certificate observed a real loopback TLS fixture, submitted the bounded result with an exact receipt, and left exactly one matching PostgreSQL row;
- SIGTERM stopped the control process with exit code 0.
- `pg_dump` produced a 22,051-byte custom archive; `pg_restore` loaded it into a separate database and all six allowlisted table counts matched exactly (18 events, 2 findings, 2 listener snapshots, 2 probe observations, 5 query-audit rows, and 7 migrations).

The run used PostgreSQL `18.6 (Debian 18.6-1.pgdg12+2)` on arm64 and completed in
7.03 seconds. See `postgres.json` for source, runner and image identities.
Actual execution was 2026-10-05 23:59:19 EDT.

This proves database semantics, the in-process lost-response fault, and a real agent/control process exchange on loopback. Its loopback connection deliberately used
`sslmode=disable`; it does not prove production database TLS, persistent-volume
durability, off-host backup retention, cross-instance disaster recovery, or systemd/reverse-proxy deployment. The restore test used a separate database in the same disposable PostgreSQL instance.

## Real Ubuntu systemd control and delivery units

A separate clean-worktree run at 2026-10-05 23:34:00 EDT cross-compiled both
ARM64 binaries and installed the shipped control unit plus an isolated-state
copy of the shipped delivery unit in the Ubuntu QEMU guest. `systemd-analyze
verify` accepted the control, delivery, and timer definitions.

The control process ran as systemd's dynamic `watchhouse-control` user with
`NoNewPrivileges=yes`, `ProtectSystem=strict`, and `ProtectHome=yes`. Its five
credentials came through `LoadCredential`. PostgreSQL 18.6 ran in a disposable
container with no published port; the guest reached only its internal Docker
bridge address. The delivery oneshot ran with the same sandbox properties,
received seven exact receipts, reduced its isolated queue from seven to zero,
and left exactly seven `vm-agent` rows remotely. A separate `vm-viewer`
certificate returned a three-record page. The unprivileged agent then collected a real
in-namespace listener snapshot, first committed it to the SQLite v2 outbox,
received an exact snapshot receipt over TLS 1.3, drained that outbox to zero,
and left one PostgreSQL listener row. Ownership blind spots remained explicit. A separate static `watchhouse-probe` user ran the hardened probe unit against a real TLS fixture, queued the observation in SQLite v3, obtained an exact control receipt, drained its probe outbox to zero, and left one certificate-bound PostgreSQL observation. The control unit remained active and
the delivery unit ended with `Result=success` and status 0.

`systemd.json` binds the source, runner, binaries, unit properties, counts, and
PostgreSQL image digest. The database link in this artificial cross-container
lab used `sslmode=disable`; this is not evidence for production database TLS, a
reverse proxy, firewall rules, or public VPS exposure.
