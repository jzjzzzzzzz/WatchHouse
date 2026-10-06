# Engineering checkpoint and handoff

## Reproducible verification

```sh
make test
make vet
make linux
```

Environment-dependent acceptance runners are intentionally separate:

```sh
make smoke
make crash
GO=/path/to/go python3 scripts/postgres-integration.py
GO=/path/to/go make docker-ports-integration
python3 scripts/systemd-integration.py
python3 scripts/systemd-control-integration.py
```

They create labeled disposable resources, preserve bounded private failure
diagnostics under `lab/local/`, and publish only reviewed summaries. Never run
them against an unrelated database/container or treat a skipped environment as
a pass.

## Implemented trust boundaries

- unprivileged native journal collection with verified cursor and crash-safe
  SQLite outboxes;
- certificate-bound host/probe/human identities, TLS 1.3 transport, exact
  receipts, idempotent PostgreSQL persistence and append-only query audit;
- deterministic SSH finding evidence plus bounded human queries;
- listener/process/unit attribution with explicit permission quality;
- layered HTTPS probe and Nginx 200/504/recovery lab;
- read-only nftables, sysctl, privileged-file, effective sshd/systemd, disk,
  certificate, package and Docker-binding diagnostics;
- same-instance PostgreSQL logical restore reconciliation and deterministic
  JSON evidence bundles with offline verification.

## Next owner priorities

1. Deploy agent/control/probe to real separate VPS nodes using production DB TLS
   and capture a 24-hour baseline without elevating current lab evidence.
2. Join listener, Docker, nftables and second-node probe observations by bounded
   time windows while preserving each source's uncertainty.
3. Add per-rule processing watermarks and a documented late-event policy before
   increasing control-plane event volume.
4. Export authorization audit to independently retained signed storage.
5. Implement one typed Nginx rollback action with approval digest, stale-state
   rejection, crash recovery and external verification; do not introduce a
   generic remote command channel.
6. Run encrypted off-host backup and independent-instance restore tests with
   measured RPO/RTO before claiming disaster-recovery readiness.

The repository is a substantial working security-operations foundation, not a
finished production management plane. The remaining gaps above are product and
deployment work, not hidden behind a “complete” label.
