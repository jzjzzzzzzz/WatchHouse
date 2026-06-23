# Runtime posture evidence contract

The posture command reads a fixed, versioned server profile from the running
Linux kernel. It is evidence about the observed network namespace at one point
in time, not a claim about boot-time persistence or fleet-wide compliance.

Each result contains a stable control ID, exact `/proc/sys` source, observed
value, accepted value set, and one of `pass`, `fail`, or `error`. Missing files
are errors rather than failures because kernel configuration and container
namespaces can remove a setting. Results are sorted by control ID. Exit status
is zero only when every control passes, three for a complete report containing
failure/error results, one for inability to construct a report, and two for
invalid CLI use.

The built-in profile is deliberately narrow. It checks redirect/source-route
handling, reverse-path filtering, SYN cookies, ASLR, kernel-pointer and dmesg
restrictions, and protected link traversal. It does not rewrite sysctls, parse
arbitrary policy supplied by a remote caller, or present itself as a full CIS
benchmark.

## Trust boundary

The command accepts no alternate root or expected-value overrides. Tests inject
an in-memory filesystem below the evaluator, but the production Linux entry
point always opens `/proc/sys`. A compromised host can falsify all local
evidence; signing and remote attestation are explicitly outside this contract.

## Operational interpretation

Some settings depend on the host's role. Routers may intentionally send
redirects or use forwarding-related values that are inappropriate for the
baseline server profile. A failure is a review signal, not permission for an
agent to mutate the kernel. Remediation belongs in reviewed infrastructure
automation, followed by a fresh observation and a reboot-persistence test.
