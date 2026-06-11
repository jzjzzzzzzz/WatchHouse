# External HTTPS probe contract v1

The probe answers whether one exact HTTPS URL was reachable and returned an
expected status and bounded body from the machine where the command ran. It
records canonical DNS results, the exact selected IP:port, DNS/TCP/TLS/total
durations, negotiated TLS version/cipher, leaf-certificate SHA-256, HTTP status,
content type, bounded byte count, body SHA-256, and expected-status outcome.

Only HTTPS is accepted. User info, query strings, fragments, redirects, proxy
environment variables, compressed responses, and bodies above 1 MiB are
rejected. The caller supplies an expected HTTP status and may use either system
roots or one absolute, non-writable CA bundle. The request sends no credentials.

Private, loopback, and link-local results fail by default to reduce accidental
SSRF-style use from a privileged runner. `--allow-private` is an explicit lab
scope change and appears in the result. v1 sorts permitted addresses and pins
the first; it does not implement Happy Eyeballs or fallback across all DNS
answers. A successful local probe is not evidence of Internet, firewall, NAT,
or another region's reachability.

An unexpected but valid HTTP response is emitted as structured JSON and returns
exit status 1. DNS, TCP, TLS, redirect, timeout, trust, and body-bound failures
return 1 with bounded stderr but currently do not emit a complete JSON failure
record. Continuous scheduling, alerting, and multi-vantage probes remain later
work.

`report-probe` uses a distinct certificate URI
`spiffe://watchhouse/probe/<probe_id>`; host and human certificates cannot call
the submission route. The observation ID hashes the authenticated probe ID and
the full canonical result. PostgreSQL stores the validated result idempotently
and returns the exact ID only after commit. The probe writes each validated
result to the private SQLite v3 outbox before loading control credentials or
contacting the control endpoint. Only an exact `(local sequence, observation
ID)` receipt deletes it; `deliver-probes` retries the oldest row independently.
One state directory is scoped to one probe certificate identity.

The shipped collection and retry timers run as the static `watchhouse-probe`
user with an empty capability set, `NoNewPrivileges`, strict filesystem and
home protection, private devices/tmp, namespace restrictions, resource limits,
and systemd credentials. A successful service run is still only one vantage
point; alerting and long-term availability SLOs remain later work.
