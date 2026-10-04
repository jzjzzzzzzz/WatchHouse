# Release and deployment acceptance checklist

This checklist prevents a green unit-test run from being presented as a
production-ready release. A release candidate may be tagged only when every
required gate has an attached command result or an explicitly approved scope
exception.

## Source and build

- [ ] clean worktree; reviewed commit and dependency lock selected;
- [ ] `make test`, `make vet`, and `make linux` pass from that commit;
- [ ] amd64/arm64 binary SHA-256 values recorded and artifacts built with
  `-trimpath`;
- [ ] secret scan and generated-artifact inventory reviewed;
- [ ] retrospective Git history audit is reported separately from runtime
  evidence and never described as elapsed development time.

## Host boundary

- [ ] dedicated users/groups exist with no interactive shell or sudo path;
- [ ] unit files pass `systemd-analyze verify` and effective sandbox properties
  pass `audit-units`;
- [ ] state/credential file ownership and modes are verified;
- [ ] sysctl, disk/inode, SSH effective policy and certificate renewal reports
  are captured;
- [ ] service restart and host reboot preserve SQLite cursor/outbox state.

## Network and identity

- [ ] listeners, Docker bindings and nftables evidence are captured separately;
- [ ] DNS/TCP/TLS/HTTP probe runs from the intended second-node vantage;
- [ ] TLS 1.3 server name, CA chain and SPIFFE role/host/probe identities pass;
- [ ] wrong-host, wrong-role, expired-certificate and unmapped-user negatives
  fail closed without deleting pending data;
- [ ] PostgreSQL is not publicly published and its production link uses the
  declared TLS verification mode.

## Data and recovery

- [ ] schema migration is idempotent and conflict rollback remains atomic;
- [ ] receipt-loss retry produces no duplicate remote row;
- [ ] encrypted off-host backup retention policy is active;
- [ ] restore occurs on an independent isolated instance, followed by counts,
  application queries and measured RTO/RPO;
- [ ] evidence bundle verifies, is signed externally, encrypted as needed and
  copied to access-controlled retention.

## Current checkpoint

Unit/race/fuzz/static tests, Linux cross-builds, native Ubuntu systemd collection,
loopback mTLS delivery, Docker binding collection, Nginx failure recovery and a
same-instance PostgreSQL logical restore have evidence. Public VPS deployment,
second-node scheduled probing, production database TLS, independently hosted
restore, off-host retention, write-action approval/rollback, external audit
export and continuous self-use are **not release-complete**.
