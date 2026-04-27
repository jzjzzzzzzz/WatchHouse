# TCP listener snapshot

`watchhouse listeners` is a Linux-only, read-only snapshot of TCP listeners in
the process's current network namespace. It accepts no path, PID, namespace, or
command arguments.

```sh
watchhouse listeners | jq .
```

The command reads both `/proc/net/tcp` and `/proc/net/tcp6`, then correlates
socket inodes to visible `/proc/<pid>/fd` links. A process owner is identified by
boot ID, PID, and kernel start-time ticks so a recycled PID is not treated as
the earlier process. Effective UID, bounded `comm`, and an unambiguous `.service`
cgroup component are attached when available. Multiple processes may own the
same inherited socket and are all retained.

Inspect both each listener's `ownership` and the top-level `quality` object:

- `attributed`: at least one stable process identity referenced the inode;
- `unknown_permission`: at least one process FD set was inaccessible, so absence
  of an owner is not a negative finding;
- `unknown_partial`: a race, malformed proc record, or configured scan bound
  prevented complete attribution;
- `unknown_unmapped`: the complete visible scan found no owner.

The ordinary journal collector intentionally lacks permission to inspect most
other users' FDs on a normal Ubuntu host. Do not grant it root merely to turn
unknown into attributed. The integration harness separately demonstrates the
same fixed reader with lab-admin privilege; a future local privileged interface
must expose only this bounded result, authenticate its Unix peer, and never
accept arbitrary proc paths or shell commands.

A listener proves only an in-namespace kernel socket. It does not prove host
firewall policy, Docker/NAT publication, or remote reachability. Compare those
as separate evidence sources rather than collapsing them into one boolean.

Limits in the initial implementation are 8,192 listeners per address family,
32,768 numeric PIDs, 4,096 FDs per PID, 1,024 bytes per proc-net line, 4 KiB per
stat record, and 64 KiB per status/cgroup record. Exceeding a content bound
fails or marks the snapshot partial instead of silently claiming completeness.
