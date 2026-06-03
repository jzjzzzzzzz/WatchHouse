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
record. Continuous scheduling, persistence, alerting, and multi-vantage probes
remain later work.
