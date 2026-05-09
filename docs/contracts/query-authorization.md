# Human query identity and authorization v1

The control API distinguishes machine and human certificate identities before
routing. An agent certificate has exactly one URI SAN
`spiffe://watchhouse/host/<host_id>`. A human certificate has exactly one URI
SAN `spiffe://watchhouse/user/<user_id>`. Common Name, request headers, query
parameters, and JSON fields cannot change principal type or ID.

A root-owned strict JSON role map binds user IDs to exactly one role: `viewer`,
`operator`, or `admin`. Version 1 grants all three roles read-only event query;
future approval and registration endpoints must check their own explicit
permissions. Agents cannot call human query routes, and human certificates
cannot ingest agent events. Unknown users fail closed. The role map is loaded at
process start; reload and external identity-provider integration are later work.

`GET /v1/events` requires one canonical `host` query parameter and accepts an
optional `limit` from 1 through 200 plus an optional positive `before` ingest
sequence. Unknown parameters, repeated parameters, invalid host IDs, invalid
numbers, and request bodies reject the request. Results are ordered by server
`ingest_sequence DESC`; the caller paginates with the last returned sequence.
The server sequence is query metadata, not an agent receipt or event identity.

Each record returns authenticated host ID, event ID, ingest sequence, server
commit time, and the validated normalized event. Responses are bounded and
contain no raw journal message, certificate, database URL, or secret. A query is
not a full investigation audit trail yet: access audit events, retention,
redaction policy, and role-map lifecycle remain required for M2 completion.

PostgreSQL migration v2 adds the monotonic query sequence and refuses schemas
newer than the binary. Event uniqueness remains `(host_id,event_id)`; pagination
does not change idempotency or receipt semantics.
