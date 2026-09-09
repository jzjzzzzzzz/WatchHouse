# Docker published-port inventory

`watchhouse docker-ports` records host bindings for running containers without
claiming that those endpoints are reachable. It runs two fixed Docker CLI
operations: list full running-container IDs, then inspect only those validated
IDs with a fixed Go template that emits minimal JSON.

The collector accepts no socket, context, container, command, or template from
the caller. It has a shared twenty-second deadline, bounded stdout/stderr,
1,024-container limit, strict per-row JSON, full 64-character ID validation,
and IP/port validation. The result records container name/ID, network mode,
protocol, container endpoint, and each Docker host binding.

## Privilege boundary

Access to the Docker daemon is effectively root-equivalent: a client that can
create containers can normally mount the host filesystem. Do **not** add the
long-running Watchhouse service to the `docker` group. Run this inventory as a
short-lived, locally initiated diagnostic under an administrator account, or
place a narrow independently reviewed read-only proxy in front of the Docker
API. Calling only read operations in this binary reduces accidents but does
not make daemon credentials least-privilege.

## Correlation

Compare a binding with three independent observations:

1. `watchhouse listeners` — socket present in the observed network namespace;
2. `watchhouse firewall` — local nftables ruleset summary;
3. `watchhouse probe-https` — protocol result from another network position.

`0.0.0.0:PORT` and `[::]:PORT` are wildcard bindings, not proof of Internet
exposure. Loopback binding is not proof of safety when an on-host reverse proxy
or tunnel forwards traffic. Docker NAT/userland proxy behavior, host routing,
cloud security groups, upstream firewalls, and the probe vantage point remain
separate layers.
