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

`GET /v1/events` and `GET /v1/findings` require one canonical `host` query parameter and accept an
optional `limit` from 1 through 200 plus an optional positive `before` ingest
sequence of their respective record type. Unknown parameters, repeated parameters, invalid host IDs, invalid
numbers, and request bodies reject the request. Results are ordered by server
`ingest_sequence DESC`; the caller paginates with the last returned sequence.
The server sequence is query metadata, not an agent receipt, event identity, or finding identity.

Each record returns authenticated host ID, event ID, ingest sequence, server
commit time, and the validated normalized event. Responses are bounded and
contain no raw journal message, certificate, database URL, or secret.

Every syntactically valid query by an authenticated human reaches an
authorization decision record before data access. Allowed decisions fail closed
if PostgreSQL cannot append the record. Denied decisions are recorded when the
auditor is available but remain denied if audit storage is unavailable. The
append-only table records principal, role when mapped, resource, target host,
decision, server time, and a monotonic audit sequence; a database trigger rejects
row update and deletion. This is an authorization-decision audit, not proof that
the HTTP response was fully transmitted or consumed.

The audit is not yet a full investigation trail: it has no client network
address, request correlation ID, cryptographic export, retention enforcement,
or privileged database-administrator tamper resistance. Redaction policy and
role-map lifecycle also remain required for M2 completion.

PostgreSQL migrations v2 and v4 add monotonic event and finding query sequences;
the binary refuses schemas newer than it understands. Event uniqueness remains
`(host_id,event_id)`, and finding uniqueness remains the deterministic finding
ID. Pagination does not change either identity or receipt semantics.
Migration v5 adds the authorization audit and its mutation-rejecting trigger.
