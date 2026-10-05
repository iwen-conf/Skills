# PostgreSQL Maintenance, MVCC, Statistics and WAL Qualification

Fast tests can still hide failures that appear only after hours or days. This standard covers steady-state database health.

## 1. Long transactions and idle-in-transaction

Track the oldest transaction and oldest idle-in-transaction session. Long-lived snapshots can delay tuple cleanup and contribute to bloat. They can also make schema changes wait unexpectedly.

Gate examples:

```text
oldest transaction < service-specific budget
idle in transaction < strict budget
no unexplained transaction older than deployment/migration window
```

## 2. VACUUM/autovacuum

Measure:

- dead tuple ratio/trend;
- last vacuum/autovacuum;
- last analyze/autoanalyze;
- `n_mod_since_analyze` relative to live rows;
- vacuum progress during stress tests;
- workload latency while vacuum/analyze is active for high-write tables.

Do not infer that autovacuum defaults are adequate from a short benchmark.

## 3. XID/freeze safety

Monitor database and table XID age. PostgreSQL must periodically freeze old row versions to avoid transaction-ID wraparound. For high transaction-rate systems, qualification should verify that autovacuum can keep up and that long transactions/replication slots do not block cleanup.

## 4. Planner-statistics drift

A plan can regress without code changes because data distribution changes. Track statistics age and modified-since-analyze ratio. For correlated columns, evaluate extended statistics rather than simply raising statistics targets everywhere.

## 5. WAL lifecycle

Observe:

```text
WAL bytes / records / full-page images
WAL buffer pressure
checkpoint behavior
archive failures
replication-slot retained WAL
logical-slot confirmed/restart LSN
```

A stalled slot can retain large amounts of WAL. Treat inactive/lost slots and retained bytes as operability risks with explicit thresholds.

## 6. Replication feedback trade-offs

Hot-standby feedback can reduce standby query cancellation but may increase primary bloat. A replica configuration change should be tested as a system trade-off, not as an isolated knob.

## 7. Sequence exhaustion

For non-cycling sequences, observe utilization when identifiers have business longevity requirements. Sequence exhaustion is rare with bigint, but an expert gate should not assume all schemas use bigint correctly.

## 8. Prepared transactions

Unexpected entries in `pg_prepared_xacts` can retain locks and resources. If two-phase commit is not part of the architecture, a nonzero count is suspicious and may be a release blocker.

## 9. Invalid indexes / unvalidated constraints

After interrupted concurrent index creation or staged constraint validation, explicitly check catalog state. “Migration command returned” is not enough evidence that all indexes/constraints are valid and ready.

## 10. Soak test

During a long-run test capture periodic samples for:

```text
connections
idle-in-transaction
lock waiters
oldest transaction
P95/P99 latency
errors/retries
temp files/bytes
WAL generation
checkpoint counters
deadlocks
40001/40P01
n_dead_tup / bloat proxy
replication lag where applicable
```

Use trends, not only final values. A monotonic growth pattern can be a stronger signal than a threshold breach.
