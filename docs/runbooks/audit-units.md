# Watchhouse systemd sandbox audit

`watchhouse audit-units` asks PID 1 for the effective properties of the five
Watchhouse service units. Unlike inspecting files under `/etc/systemd/system`,
this observes the merged unit after drop-ins and `daemon-reload` state.

The unit list and property list are compiled into the binary. Callers cannot
turn this interface into arbitrary systemd enumeration. Each `systemctl show`
invocation has bounded stdout/stderr and all five calls share a fifteen-second
deadline.

The current profile requires:

* a loaded unit;
* `NoNewPrivileges=yes`;
* `ProtectSystem=strict`;
* a protected home namespace;
* private temporary storage;
* an empty capability bounding set.

Active state is recorded but not treated as a hardening pass/fail condition:
oneshot collection and delivery services are normally inactive between timer
runs. Timer health must be evaluated from timer state and recent invocation
results separately.

```sh
sudo -u watchhouse /opt/watchhouse/bin/watchhouse audit-units | jq .
```

A failed property means the effective service boundary differs from the
profile. A missing/unloaded unit or absent property is an evidence error. First
inspect `systemctl cat UNIT`, `systemctl show UNIT`, and the deployment package;
then update the source unit, run `systemd-analyze verify`, reload, restart one
unit at a time, and rerun the audit. Never auto-rewrite unit files from the
monitoring agent.
