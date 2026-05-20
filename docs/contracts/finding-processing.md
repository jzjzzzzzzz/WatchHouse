# Persistent finding processing v1

The control ingest path commits an authenticated event batch before evaluating
the SSH rule for that host. It returns receipts only after evaluation succeeds.
If evaluation fails after the event commit, the agent retains its SQLite items
and retries. PostgreSQL event identity makes the repeated commit idempotent, and
the deterministic finding ID makes repeated evaluation idempotent.

Version 1 deliberately rebuilds detector state from the host's stored events in
`observed_at, ingest_sequence` order after each accepted batch. The scan is
bounded at 50,000 events in the control process (the library hard ceiling is
100,000). Exceeding the bound fails closed without issuing receipts. This is a
correctness-first implementation for a small self-hosted fleet, not a scalable
stream processor. A later version needs transactional per-rule watermarks and a
specified policy for late events before the bound can be raised or removed.

`control_findings` stores the deterministic finding ID, authenticated host,
rule ID, event-time observation, priority, summary, the exact rule window and
threshold, and the ordered event IDs used as evidence. Re-evaluation may update
only `last_evaluated_at` when every identity-bound field is identical. A
conflicting row under the same finding ID aborts processing.

Findings have a server-assigned monotonic sequence solely for stable reverse
pagination. `GET /v1/findings` uses the same human certificate and role-map
authorization as event queries. `host` is mandatory; `limit` is 1..200; and a
positive `before` is exclusive. The client rejects malformed digests,
out-of-order pages, host changes, empty evidence, and invalid rule metadata.

No finding triggers a server mutation, firewall change, account lock, or remote
command. Version 1 has no acknowledgement/workflow state, access audit, rule
reload, retention, cross-host correlation, or automatic response.
