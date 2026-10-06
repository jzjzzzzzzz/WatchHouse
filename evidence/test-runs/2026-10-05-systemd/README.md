# Ubuntu systemd collector acceptance — 2026-10-05 local time

This run installed the fixed, read-only collector into a separate Ubuntu ARM64
QEMU guest with a real Linux kernel and systemd PID 1. The harness used only the
generated lab SSH identity over a loopback-only forwarded port. `service.json`
is the machine-readable result; `manifest.json` binds it to the guest image,
runner, source commit, and compiled binary digests.

Verified in the guest:

- a dedicated UID 999 `watchhouse` account has no login shell, `sudo` group, or
  effective capabilities;
- the actual transient sandbox reported `NoNewPrivileges=true`, seccomp mode 2,
  and an all-zero effective capability mask;
- the installed oneshot finished with `Result=success` and status 0 under
  `ProtectSystem=strict` and `ProtectHome=yes`;
- a world-readable fixture under `/home` was readable by the service UID without
  the sandbox and unreadable inside the mirrored sandbox, demonstrating the
  mount boundary rather than relying on home-directory DAC permissions;
- native journal collection produced 112 pending records, and the independent
  queue audit reconciled all 112 records and 66,617 logical payload bytes;
- the periodic timer became active.

The clean second run built source commit `ab18054` with no tracked or untracked
worktree changes. Actual execution was 2026-10-05 21:15:38 EDT (2026-10-06
01:15:38 UTC).

This proves a local VM deployment of the current read-only collector. It does
not prove a public VPS, long-running reliability, physical power-loss survival,
network transport, or any privileged remediation path. Private keys, raw
journal records, disk images, and VM overlays remain under ignored
`lab/local/` and are not published here.

## Native journal and reboot run

A separate clean-worktree run at 2026-10-05 21:22:42 EDT exercised the native
journal path rather than an imported fixture. It consumed 737 existing journal
records, generated exactly five rejected Ed25519-key authentications, collected
37 new records, and proved that the resulting five failure event IDs were the
exact evidence set of a new failed-then-success finding. The persisted queue
then reconciled 131 records and 78,061 logical bytes.

A separate state directory capped at one record accepted exactly one record and
recorded four blocked attempts without advancing through the unseen data. The
harness rebooted only the guest, observed a changed kernel boot ID, observed the
same 131 pending records before and after reboot, then resumed from the native
journal cursor and inserted 15 additional matched records. See
`journal-integration.json`; raw journal JSON and replay output stay private.

## TCP listener ownership and policy run

A clean-worktree run at 2026-10-05 21:36:19 EDT started a controlled loopback
TCP listener as a transient systemd service. The ordinary account saw the
socket but could not read 95 other-process FD sets; its result was
`unknown_permission` with no invented owner. The same fixed read-only command
run with lab-admin privilege scanned 96 process directories without denial and
correlated the listener inode to PID 2261, start time 97224, effective UID 0,
`python3`, and `watchhouse-listener-fixture.service`.

The harness then evaluated two strict policies. The correctly declared fixture
produced no resource finding. A drift policy produced one deterministic
`unexpected_listener` finding for port 18081 and one `missing_listener` finding
for declared port 18082. `listeners.json` records both outcomes and the shared
network namespace.

This is inode/PID/unit and declaration evidence, not evidence of firewall
allowance, Docker port publication, NAT behavior, or external reachability. The
lab fixture was stopped after the assertions.
