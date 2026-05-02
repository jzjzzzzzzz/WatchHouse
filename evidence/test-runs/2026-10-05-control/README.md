# PostgreSQL control-store acceptance — 2026-10-05 local time

A clean-worktree run started the Docker Official Image for PostgreSQL 18.6,
pinned by multi-platform OCI index digest, on a dynamic loopback-only port with
data on a bounded disposable tmpfs. A random password was passed through a
mode-0600 mounted file and was not included in the report or command arguments.

The real database run proved:

- embedded schema migration succeeds and a second migration is idempotent;
- retrying the same authenticated host/event content keeps one row;
- a two-item batch that inserts one new event and then encounters a conflicting
  existing identity rolls the whole transaction back;
- eight concurrent identical batch commits converge on one stored event;
- the resulting count is two distinct events, not ten delivery attempts.

The run used PostgreSQL `18.6 (Debian 18.6-1.pgdg12+2)` on arm64 and completed in
3.02 seconds. See `postgres.json` for source, runner and image identities.
Actual execution was 2026-10-05 21:47:21 EDT.

This isolates database semantics. Its loopback connection deliberately used
`sslmode=disable`; it does not prove production database TLS, persistent-volume
durability, backup/restore, or that the HTTP mTLS receiver has yet been assembled
into a deployable control process.
