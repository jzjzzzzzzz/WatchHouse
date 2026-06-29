# Privileged file evidence contract

`watchhouse audit-files` inspects a fixed built-in list of account, SSH, and
sudo policy files. The production collector uses `lstat(2)` and reports the
numeric owner, group, and permission bits. It never reads file contents.

The evaluator rejects a final-component symlink, a non-regular file, unexpected
owner UID, group/world write access, and permissions outside the control's
allowed mask. Missing or unreadable paths are observation errors rather than a
silent pass. The complete report is emitted even when individual controls fail;
exit status 3 makes that state visible to automation.

This is point-in-time metadata evidence, not a race-free authorization check.
`lstat` does not prove that every parent directory avoided symlink traversal,
and metadata can change immediately after collection. The current command is
therefore suitable for inventory and alerting, not for authorizing use of a
secret. Secret loading uses its own narrower checks, and a future privileged
collector should use directory file descriptors with `openat2` resolution
constraints when content identity matters.

The built-in list is intentionally small:

* `/etc/passwd` and `/etc/group`
* `/etc/shadow` and `/etc/gshadow`
* `/etc/ssh/sshd_config`
* `/etc/sudoers`

Distribution-specific include directories are not inferred from these six
paths. They require separate enumeration with explicit bounds and ownership
rules. Absence of a finding does not establish that all effective SSH or sudo
configuration is safe.
