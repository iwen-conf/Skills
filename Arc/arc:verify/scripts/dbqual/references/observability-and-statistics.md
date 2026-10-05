# PostgreSQL observability standard

## Required distinction

Keep three levels separate:

1. **Client-observed workload latency** — pgbench transaction latency, schedule lag, errors/retries.
2. **Statement attribution** — pg_stat_statements calls/total time/IO/temp/WAL deltas.
3. **Diagnostic plan instrumentation** — EXPLAIN ANALYZE plan rows, loops, buffers, WAL and node structure.

Do not substitute one for another.

## pg_stat_statements

Recommended in benchmark/staging environments.

It requires `shared_preload_libraries` and query IDs. Capture before/after snapshots and compute deltas instead of resetting shared statistics.

Useful dimensions:

```text
calls
total_exec_time
mean_exec_time
rows
shared_blks_hit/read/written
temp_blks_read/written
IO timing when track_io_timing is enabled
wal_records / wal_fpi / wal_bytes
```

The workload runner automatically attempts a before/after delta. Set `require_pg_stat_statements=true` only when the evidence is mandatory.

## pg_stat_io / pg_stat_wal / pg_stat_database

Use suite-level snapshots for environment/resource context. PostgreSQL 16+ exposes pg_stat_io.

Avoid claiming per-query attribution from global counters unless the benchmark environment is isolated and the measurement window is controlled.

## Wait evidence

`pg_stat_activity` exposes current wait events. `pg_locks` and `pg_blocking_pids()` explain lock blockers.

The deterministic transaction runner samples blockers only in test scenarios. Frequent blocker inspection itself has overhead; do not treat the observer as free.
