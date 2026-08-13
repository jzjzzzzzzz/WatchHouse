# PostgreSQL logical backup and restore test

A backup is not accepted until it has been restored and reconciled. The local
acceptance runner creates a custom-format `pg_dump`, records its byte length and
SHA-256, restores it into `watchhouse_restore`, and compares row counts for the
six owned control-plane tables.

```sh
python3 scripts/postgres-restore-test.py \
  --container watchhouse-postgres-INSTANCE \
  --output lab/local/restore-evidence
```

The runner refuses containers without the integration label, never drops the
source `watchhouse` database, restricts the restore database to a validated
identifier, uses `--exit-on-error`, and writes its result mode 0600. It is for
the disposable integration environment. Production credentials, retention,
encryption, and object storage are intentionally not embedded in this script.

## Production procedure

1. Take the backup from a least-privilege backup role and record PostgreSQL
   version, schema migration, start/end time, archive digest, and object-store
   version ID.
2. Encrypt before leaving the database trust boundary. Keep encryption keys
   outside the backup bucket and test recovery of the key material.
3. Restore into a new isolated PostgreSQL instance of a supported version—not
   into the source server.
4. Run `pg_restore --list`, restore with errors fatal, apply no new migrations,
   and reconcile table counts plus application invariants.
5. Start a control process against the restored database with networking
   restricted, query known events/findings through mTLS, and capture evidence.
6. Destroy the restore instance and record the measured recovery time and
   recovery point.

The committed local evidence proves archive creation, parser acceptance, and
same-instance separate-database row reconciliation. It does **not** prove
off-host retention, encrypted storage, persistent-volume recovery, point-in-time
recovery, cross-version restore, or the production RTO/RPO.
