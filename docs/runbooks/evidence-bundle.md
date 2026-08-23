# Incident evidence bundles

The bundle command packages a small directory of already-redacted JSON
artifacts into a deterministic `tar.gz` with a manifest of names, lengths, and
SHA-256 digests.

```sh
watchhouse evidence-bundle create \
  --input /var/lib/watchhouse/export/incident-2026-001 \
  --output /var/lib/watchhouse/export/incident-2026-001.tar.gz

watchhouse evidence-bundle verify \
  --archive /var/lib/watchhouse/export/incident-2026-001.tar.gz
```

Creation accepts only top-level `.json` regular files: at most 128 directory
entries, 4 MiB per file, and 32 MiB total. It rejects symlinks, invalid JSON,
and files whose identity changes during reading. Sorted entries, zero archive
timestamps, and normalized modes make repeated bundles byte-deterministic when
the evidence and declared creation time are identical. Publishing uses an
atomic hard link and refuses to overwrite an existing archive.

Verification rejects traversal paths, links and special files, duplicate or
unmanifested entries, expansion beyond the bounds, malformed manifests, length
mismatches, and digest mismatches. It does not extract files to disk.

## Security boundary

Bundling is integrity accounting, not authenticity or secrecy. SHA-256 inside
the same archive detects accidental corruption and internal mismatch but does
not prove who produced it. Sign the archive digest using the organization's
evidence-signing process and store the signature separately. Encrypt the bundle
when its JSON contains sensitive IP addresses, usernames, or incident details.

The command never searches the host automatically and ignores nested
directories. An investigator must first export explicitly selected,
schema-bounded artifacts and review them for secrets. Do not place private
keys, database URLs, environment dumps, raw `/proc` contents, or unrestricted
journal exports in the input directory.
