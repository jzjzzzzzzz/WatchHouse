# nftables inventory

`watchhouse firewall` runs one fixed command, `/usr/sbin/nft --json list
ruleset` (or the equivalent trusted `/sbin/nft` path), with a ten-second
deadline. It accepts no command, path, namespace, or family supplied by a
caller. The executable must be a regular file and must not be group/world
writable.

The command needs enough privilege to read the ruleset. Prefer a dedicated,
root-owned oneshot collector over adding `CAP_NET_ADMIN` to the long-running
agent. Store its JSON output in a root-controlled state directory and let the
unprivileged agent consume only the summarized artifact in a later phase.

## Output and limits

The output is a lossy inventory: table names, base-chain boundaries, aggregate
object counts, observation time, and SHA-256 of the exact source document.
Rules and expressions are deliberately omitted because they can disclose
addresses and operational policy. Stdout is capped at 4 MiB, stderr at 16 KiB,
and the parser accepts at most 100,000 top-level objects.

```sh
sudo ./bin/watchhouse firewall | jq .
```

## Interpretation boundary

A `drop` input policy is not proof that a service is unreachable, and an
`accept` rule is not proof that it is reachable. Network namespace selection,
rule order, sets, NAT, routing, conntrack, Docker-managed chains, cloud security
groups, and upstream firewalls all matter. Compare this snapshot with
`watchhouse listeners`, Docker port bindings, and a probe from an independent
network node. Never report those layers as one inferred fact.

## Failure triage

* `trusted nft executable not found`: install nftables or record that this host
  uses a different firewall backend.
* `Operation not permitted`: run the short-lived collector with the minimum
  required privilege; do not make the agent root.
* timeout or output limit: preserve stderr, inspect ruleset scale locally, and
  do not silently truncate evidence.
* digest changed: compare two privileged raw captures locally; the summary is
  intentionally insufficient for rule-level diffing.
