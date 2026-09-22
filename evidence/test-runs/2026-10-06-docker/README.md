# Docker published-port acceptance

Actual execution: 2026-10-06 00:21:06 EDT.

A clean-worktree run cross-compiled the ARM64 Linux agent, started a non-root
Nginx fixture with a dynamic loopback-only host binding, and executed the agent
inside a disposable Linux Docker CLI container. Both images were selected by
immutable OCI digest. The diagnostic container was read-only, network-disabled,
capability-free, `no-new-privileges`, memory/PID bounded, and given the Docker
socket plus the read-only agent binary.

The collector enumerated 12 running containers and 13 published bindings. It
returned exactly one row matching the fixture's 64-character container ID,
container port 80, dynamic host port 63863, and `127.0.0.1` host address. The
runner then verified its labeled fixture identity before removal.

The committed `result.json` binds the source commit, runner, agent binary,
Docker CLI image, Nginx image, fixture endpoint, counts, and actual execution
time. Container IDs and dynamic ports are lab identifiers, not stable deployment
configuration.

## Scope boundary

This is real Docker Engine metadata from Docker Desktop's Linux VM. Mounting the
daemon socket was root-equivalent even though the container dropped Linux
capabilities; this is why the production service must not receive that socket.
The loopback binding was not probed from a second node, and the run does not
prove nftables traversal, cloud firewall behavior, public reachability, or a
native Linux VPS deployment.
