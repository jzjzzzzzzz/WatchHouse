# Filesystem exhaustion audit

`watchhouse audit-disk` uses `statfs(2)` on `/` and
`/var/lib/watchhouse`. It records filesystem type, total/available bytes,
available percentage, and inode counts using `Bavail` rather than `Bfree`, so
the capacity reflects blocks available to the unprivileged service.

The fixed server profile requires at least 1 GiB **and** 15 percent available
blocks, plus 10 percent available inodes on filesystems that report inode
counts. Both an absolute and percentage threshold matter: percentages alone
warn too late on small volumes and too early on very large volumes.

Exit status 3 means the complete observation contains a threshold failure or an
unreadable/missing path. Missing `/var/lib/watchhouse` is an evidence error and
usually means deployment is incomplete. The command accepts no caller paths or
threshold overrides, preventing remote requests from turning the agent into a
filesystem oracle.

## Triage

1. Compare `df -h / /var/lib/watchhouse` and `df -i` with the report.
2. Use bounded, same-filesystem tools such as `du -x` rather than crossing
   mounts blindly.
3. Inspect journald retention, SQLite WAL/checkpoint state, container layers,
   package caches, and deleted-but-open files (`lsof +L1`).
4. Preserve incident evidence before deletion. Do not truncate active database
   files or logs in place.
5. Expand the filesystem or apply a reviewed retention change, then rerun the
   audit and validate event delivery.

This check predicts common availability failures; it does not replace storage
latency, read-only remount, SMART/cloud-volume, or database-specific monitoring.
