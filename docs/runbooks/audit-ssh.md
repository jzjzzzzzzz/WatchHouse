# Effective SSH configuration audit

`watchhouse audit-ssh` invokes the trusted system `sshd` binary as a short-lived
configuration compiler. It runs the fixed command equivalent to:

```sh
/usr/sbin/sshd -T -C user=root,host=localhost,addr=127.0.0.1
```

The ten-second process deadline, 1 MiB stdout limit, and 16 KiB stderr limit
prevent an invalid or hostile local configuration from producing unbounded
agent memory use. No caller-controlled path, user, host, address, or sshd
argument crosses the command boundary.

The evaluator checks effective values rather than grepping `sshd_config`, so it
includes defaults and `Include` processing. It checks root login, password and
keyboard-interactive authentication, empty passwords, public-key
authentication, X11 forwarding, authentication attempts, rhosts/host-based
authentication, and user environment handling.

## Interpreting results

The fixed root/localhost context exercises one concrete `Match` context. It
does not prove every user/address/host combination has the same policy. A
production rollout should declare approved contexts and execute a bounded list
owned by local configuration management—not contexts received from the control
plane.

Exit codes:

* `0`: complete report and all controls pass;
* `3`: complete report contains a policy failure or missing directive;
* `1`: sshd could not compile configuration or evidence could not be collected;
* `2`: invalid CLI invocation.

Before remediation, run `sshd -t` through the deployment system. Keep the
existing administrative session open, reload rather than blindly restart, and
verify a second public-key session before disconnecting. The audit command is
strictly read-only and never edits or reloads SSH.
