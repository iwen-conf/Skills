# Backup, Restore, PITR, RPO and RTO Qualification

A backup is not qualified merely because a backup job succeeded.

## 1. Four separate claims

1. **Backup integrity** — files/manifests/checksums are internally consistent.
2. **Restorability** — a real PostgreSQL instance can start from the backup.
3. **Recoverability** — WAL/PITR can recover to the intended target/timeline.
4. **Business truth** — recovered data satisfies the application invariants and expected recovery point.

Each claim needs evidence.

## 2. `pg_verifybackup`

Use `pg_verifybackup` for base-backup manifest/file/WAL verification where applicable. PostgreSQL explicitly warns that this does not replace a test restore. The release/operations program therefore requires restore drills for critical data.

## 3. Restore drill

An isolated restore rehearsal should record:

```text
backup identifier/timestamp
backup manifest hash
WAL/archive range
PostgreSQL version
restore commands
start-to-healthy RTO
recovery target/timeline
business invariant results
row/count/checksum/reconciliation results
application smoke tests
cleanup result
```

## 4. PITR markers and RPO

For PITR drills, create known logical markers around the target window. Example:

```text
M1 committed before target => MUST exist
M2 committed after target  => MUST NOT exist
```

This is stronger than only checking that PostgreSQL started.

## 5. RTO and RPO are measured properties

Do not publish an RTO/RPO based only on configuration. Measure:

```text
RTO = incident/recovery start -> service and invariants healthy
RPO = latest business event guaranteed present after recovery
```

Include object-storage/download/decompression/WAL-replay/bootstrap time if they are part of the real recovery path.

## 6. Older-backup sampling

Testing only the newest backup can hide retention/archive problems. Periodically sample older recoverable points and different timelines according to the organization's retention policy.

## 7. Failure injection for recovery systems

Where safe, test missing/corrupt backup pieces, missing WAL, permissions, expired credentials, unavailable archive storage and incorrect recovery targets. The goal is to validate detection and operator guidance, not to damage production.
