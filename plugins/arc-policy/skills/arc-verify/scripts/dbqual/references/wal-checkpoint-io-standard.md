# WAL, checkpoint, temporary I/O and write-amplification qualification

Database latency can remain acceptable during a short benchmark while hidden write amplification, checkpoint pressure or temporary-file growth makes the system unstable under sustained traffic. G17 treats these as release evidence, not after-the-fact DBA diagnostics.

## Observe over a bounded workload window

Prefer pre/post counter deltas and time series for:

```text
pg_stat_wal
pg_stat_io
pg_stat_checkpointer
pg_stat_database temp_files/temp_bytes
pg_stat_archiver when archive/PITR is in scope
replication slot retained WAL
pg_stat_statements shared/local/temp blocks and WAL bytes
```

For PostgreSQL 18, `pg_stat_io` carries richer I/O activity including WAL-related rows; capture the server version with the evidence because statistics schemas evolve between majors.

## Questions

- Does a query/migration produce disproportionate WAL relative to useful work?
- Is WAL buffer pressure rising?
- Do checkpoints become more frequent/expensive under the new workload?
- Does sorting/hash aggregation spill to temporary files?
- Does a backfill or index build create unacceptable replica/archive lag?
- Can inactive/lost replication slots retain unbounded WAL?
- Does overload recovery include I/O/WAL recovery, not merely request latency recovery?

## Gate philosophy

Avoid universal numerical thresholds. Baseline on representative hardware and workload, then define budgets tied to storage throughput, replica/archive capacity, RPO and service SLOs. Missing metrics must be reported as unavailable, never zero.
