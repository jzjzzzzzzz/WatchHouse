# Network exposure correlation contract

`watchhouse exposure` samples two local layers in sequence: TCP listeners from
the current Linux network namespace and published bindings for running Docker
containers. It records same-family, same-port, address-compatible matches and
the TCP listeners that did not match a published binding.

A match means only that a compatible userspace socket existed during the
listener sample. It does not prove the socket belongs to the named container;
process ownership evidence and Docker metadata come from different APIs without
a shared transaction. A non-match is also not a failure: Docker can publish
through kernel NAT without a userland proxy/listening socket. UDP bindings
cannot match the current TCP-only listener collector.

The two observations are not atomic. A container can start, stop, or republish
between `/proc` scanning and Docker inspection. Each source retains its own
observation details, and the correlation has a separate timestamp. Consumers
must not rewrite these into a single fact such as “port is open.”

External reachability additionally depends on nftables rule order and sets,
routing, reverse proxies, tunnels, cloud security groups, upstream controls,
and the probe vantage point. Only an authenticated application-layer probe from
the intended network position can establish that a particular request worked
at a particular time.

Because Docker daemon access is root-equivalent, this combined command is an
administrator diagnostic, not part of the unprivileged long-running service.
Failure to access Docker is reported rather than bypassed with sudo or group
membership changes.
