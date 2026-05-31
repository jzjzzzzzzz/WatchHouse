# TCP listener snapshot contract

## Purpose and boundary

The M1 listener snapshot answers one narrow question: which TCP listening sockets
exist in the collector's current Linux network namespace, and which processes
can be attributed to their kernel socket inodes at the same observation point?
It is read-only evidence. It does not prove that a port is reachable through a
firewall, NAT, container publication, reverse proxy, or an external network.
Those are separate observations.

The initial implementation reads `/proc/net/tcp`, `/proc/net/tcp6`, numeric
`/proc/<pid>` identities, their `fd` symlinks, and cgroup membership. It accepts
no caller-selected proc root, PID, path, namespace, or command. Bounds apply to
socket rows, PIDs, and file descriptors so a hostile or very large host cannot
force unbounded scanning.

## Identity and attribution

A socket is keyed inside one snapshot by protocol family plus kernel inode. A
process identity is `(boot_id, pid, start_time_ticks)`, not PID alone. The
scanner reads start time before and after its FD scan; if the process disappears
or the value changes, no ownership claim is emitted. All processes referencing
the inode are retained because inherited or shared descriptors are legitimate.

Each listener records:

- address family, canonical local address and port;
- kernel socket UID and inode;
- zero or more process identities, effective UID, bounded comm, and an optional
  raw systemd `.service` cgroup component;
- an ownership conclusion and collection quality.

`attributed` means at least one stable process FD referenced the inode.
`unknown_permission` means attribution was absent while at least one process FD
set could not be read. `unknown_partial` means a PID/FD bound, race, or malformed
proc record prevented a complete attribution pass. `unknown_unmapped` means the
bounded process scan completed without those failures but found no current
owner; the kernel object may no longer be represented by a visible FD.

## Quality and negative claims

The snapshot reports denied, vanished, malformed, and truncated counts. A
listener without an owner is never silently assigned to a unit based only on
port, name, or timing. Failure to read either proc-net file fails the command;
a partial IPv4-only view is not labeled a complete listener snapshot.

The caller's network namespace symlink and boot ID are recorded. The scanner
sees only that namespace. Host sockets in another namespace and Docker NAT
published ports require additional, explicitly labeled collectors. Likewise,
external TCP probes are required to claim reachability.

Reading other processes' FD links commonly requires root or a separately
constrained privileged reader. The ordinary journal collector is not granted
that privilege. The initial command makes missing access visible; a later fixed
local interface may run this exact bounded read operation without exposing an
arbitrary `/proc`, Docker, or shell proxy.

## Authenticated transport and storage

An agent may send one strict snapshot to `POST /v1/listener-snapshots`. The mTLS
URI SAN supplies the host identity; no JSON host field can override it. The
snapshot ID binds host, boot ID, network namespace, and nanosecond observation
time. The receiver independently revalidates canonical addresses, ordering,
owner/quality consistency, process bounds, and collection bounds before writing
PostgreSQL. An existing ID with identical canonical content is an idempotent
retry; different content under that ID is an identity conflict.

The response repeats the exact snapshot ID only after the database commit.
`report-listeners` writes the validated snapshot to the private SQLite v2
outbox before loading credentials or contacting the network. Only an exact
receipt deletes `(local sequence, snapshot ID)`. Transport, certificate, server,
or receipt failures retain the row; `deliver-listeners` retries the oldest row
without requiring another collection. One state directory is scoped to one
agent identity, and the retry command rejects a queued host that differs from
its certificate before network contact. This telemetry still proves neither
Docker publication nor external reachability.
