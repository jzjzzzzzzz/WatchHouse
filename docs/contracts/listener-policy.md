# Declared TCP listener policy and findings

A listener policy is an operator-owned declaration, not an automatically learned
baseline. Schema version 1 contains a bounded policy ID and at most 256 exact
TCP endpoints. Each declaration has a stable resource ID, address family,
canonical local address, nonzero port, and optionally a bounded set of allowed
systemd service unit names.

Address matching is exact. `0.0.0.0:443`, `127.0.0.1:443`, `[::]:443`, and
`[::1]:443` are different declarations because their exposure implications are
different. Version 1 has no wildcard, CIDR, port range, protocol other than TCP,
or implicit IPv4/IPv6 equivalence. Duplicate resource IDs or endpoints are
invalid rather than resolved by order.

Evaluation of one listener snapshot produces these conclusions:

- `unexpected_listener`: an observed endpoint is not declared;
- `missing_listener`: a declared endpoint was not observed in this snapshot;
- `owner_mismatch`: the endpoint exists and was attributed, but none of its
  stable owners belongs to an allowed unit;
- `ownership_unknown`: an allowed-unit constraint exists but proc permissions,
  collection bounds, races, or missing inode attribution prevent a decision.

An allowed-unit list left empty means the policy constrains only the endpoint;
it does not silently mean any observed process is trustworthy. Findings include
the policy ID, resource ID when known, boot ID, network namespace, observation
time, exact endpoint, evidence conclusion, and current listener/owner evidence.
Finding IDs are deterministic for that evidence identity.

The evaluator does not label a process malicious and does not claim external
reachability. It does not infer Docker publication or firewall policy. A
`missing_listener` is service-state evidence, while an `unexpected_listener` is
exposure-review evidence; severity and response remain control-plane policy.

Policy JSON is strictly bounded, rejects unknown fields, invalid canonical
addresses, trailing values, duplicate declarations, and invalid systemd unit
names. Deployment must place the policy under root ownership; parsing a valid
file does not authenticate who wrote it. Network transport and signed policy
distribution are later boundaries.
