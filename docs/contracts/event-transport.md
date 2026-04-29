# Agent event transport v1

## Identity and TLS

Agents send queued normalized events to `POST /v1/events/batch` over TLS with a
verified client certificate. The control endpoint requires a trusted client
chain and derives the host identity from exactly one URI SAN of the form
`spiffe://watchhouse/host/<host_id>`. It does not use Common Name, a proxy
header, request body, or URL parameter as host identity. `<host_id>` uses the
same bounded syntax as event host IDs. Additional or malformed identity URI
SANs reject the request.

The agent verifies the configured server name against a private CA and never
uses insecure skip-verify. TLS private keys and CA bundles are deployment files,
not CLI values, event payloads, Git artifacts, or log fields. Version 1 tests
certificate mismatch, untrusted client, expired/not-yet-valid certificates,
missing client certificates, and an event claiming another host.

## Batch and receipt semantics

The JSON body is bounded to 8 MiB and 500 items. Each item contains the local
SQLite sequence, event ID, and normalized event. Unknown fields, duplicate JSON
keys, trailing values, invalid events, duplicate sequences with conflicting
identities, and inconsistent item/event IDs reject the whole batch.

A successful response contains exact `(sequence, event_id)` receipts for every
request item and no others. The control store durably commits the complete batch
before returning success. `(authenticated_host_id, event_id)` is unique:
repeating identical content is idempotently accepted, while the same identity
with changed normalized content is a conflict. Receipt sequence is transport
correlation only and is not a global server event sequence.

The agent calls local `Ack` only after all of these hold:

1. HTTPS completed with the pinned server identity;
2. status and content type indicate the v1 success response;
3. the bounded response strictly decodes;
4. receipts form an exact set equal to the submitted batch.

Timeout, TLS error, non-success status, malformed/partial/extra receipt, or local
Ack failure leaves unacknowledged records in the queue. A retry may therefore
repeat a remotely committed batch; server idempotency is required. Neither HTTP
nor mTLS provides exactly-once delivery.

## Scope and failure visibility

The endpoint accepts only normalized event schema v1. It does not accept raw
journal lines, commands, policies, findings, shell input, or arbitrary blobs.
Response diagnostics are bounded and must not echo certificates, request
payloads, or secrets.

Initial delivery is an explicit bounded run, suitable for a timer; it is not a
claim of a persistent streaming agent. Queue age, attempts, failures, receipt
validation errors, and remaining records must become observable before M2 is
complete. Certificate enrollment, automated rotation/revocation distribution,
and production reverse-proxy deployment are separate milestones.
