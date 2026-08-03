# Certificate lifecycle audit

`watchhouse audit-cert --cert /absolute/path.pem` parses exactly one X.509
certificate and reports its subject, issuer, serial number, validity interval,
DNS/URI identities, DER SHA-256 fingerprint, and remaining lifetime. The
default renewal window is 30 days.

The reader requires an absolute regular-file path, rejects a final symlink,
caps input at 1 MiB, and checks file identity before and after open. Certificate
material is public, but path and identity checks prevent confusing evidence
from a swapped deployment artifact. Private keys are never read by this
command.

Exit status 3 covers not-yet-valid, expired, and renewal-window certificates;
status 1 indicates unreadable or malformed evidence. A passing lifetime check
does not validate a chain, revocation status, key possession, or suitability
for a particular TLS role. The mTLS connection itself performs chain and name
validation.

## Rotation procedure

1. Issue a replacement with the same intended SPIFFE/DNS identity and correct
   EKU.
2. Validate the new certificate and trust chain outside the live credential
   path.
3. Atomically replace certificate and matching private key with owner-only key
   permissions.
4. Restart one service, inspect its journal, and confirm mTLS delivery plus an
   exact receipt.
5. Roll the remaining services and retain the prior credential only for the
   documented rollback interval.
6. Revoke/retire the prior certificate and rerun this audit.

Do not extend the warning window beyond one year or treat local clock skew as a
certificate failure without checking NTP synchronization first.
