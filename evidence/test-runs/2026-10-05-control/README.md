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
- reusing that certificate for a spool claiming `claimed-host` failed, left all seven local records pending, and inserted zero rows for the claimed identity;
- a viewer certificate mapped to `alice` returned two exclusive three-record pages, while a valid but unmapped `eve` certificate was denied;
- SIGTERM stopped the control process with exit code 0.

The run used PostgreSQL `18.6 (Debian 18.6-1.pgdg12+2)` on arm64 and completed in
5.74 seconds. See `postgres.json` for source, runner and image identities.
Actual execution was 2026-10-05 22:07:00 EDT.

This proves database semantics, the in-process lost-response fault, and a real agent/control process exchange on loopback. Its loopback connection deliberately used
`sslmode=disable`; it does not prove production database TLS, persistent-volume
durability, backup/restore, or systemd/reverse-proxy deployment.
