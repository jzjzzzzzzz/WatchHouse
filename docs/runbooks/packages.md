# Installed package inventory

`watchhouse packages` executes a fixed trusted `dpkg-query` command and emits a
sorted inventory of package name, version, and architecture. The collector has
a twenty-second deadline, 16 MiB stdout limit, 16 KiB stderr limit, 100,000-row
limit, duplicate rejection, per-field bounds, and an SHA-256 of the exact raw
query output.

The command accepts no executable, root filesystem, package filter, or format
string from the caller. It is read-only and requires no root privilege.

```sh
watchhouse packages > packages.json
watchhouse evidence-bundle create --input /absolute/export --output /absolute/packages.tar.gz
```

## Vulnerability-management boundary

This inventory is the input to vulnerability management, not a vulnerability
scanner. A version string alone cannot establish exposure: distribution
vendors frequently backport security patches without adopting the upstream
version. Match packages against the correct distribution release and vendor
security tracker, retain advisory timestamps, and record whether a vulnerable
code path is enabled or reachable.

A defensible workflow is:

1. capture OS release identity and this installed-package inventory;
2. compare it with authenticated vendor advisory data;
3. prioritize by exploitability, service exposure, and asset role;
4. patch through reviewed package automation;
5. reboot when kernel/runtime replacement requires it;
6. capture a second inventory and exercise service health plus security
   regression tests.

The current implementation supports Debian-family hosts only. It does not
enumerate language packages, container image layers, firmware, manually copied
binaries, snap/flatpak packages, or packages inside other mount namespaces.
