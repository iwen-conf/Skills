# Engineering basis

Primary references used by this Skill:

- PostgreSQL 18 pgbench: https://www.postgresql.org/docs/18/pgbench.html
- PostgreSQL 18 pg_stat_statements: https://www.postgresql.org/docs/18/pgstatstatements.html
- PostgreSQL 18 EXPLAIN: https://www.postgresql.org/docs/18/sql-explain.html
- PostgreSQL 18 Monitoring Database Activity: https://www.postgresql.org/docs/18/monitoring.html
- PostgreSQL 18 Viewing Locks: https://www.postgresql.org/docs/18/monitoring-locks.html
- PostgreSQL 18 pg_locks: https://www.postgresql.org/docs/18/view-pg-locks.html
- PostgreSQL 18 pg_blocking_pids(): https://www.postgresql.org/docs/18/functions-info.html
- PostgreSQL auto_explain: https://www.postgresql.org/docs/current/auto-explain.html
- pgBadger: https://github.com/darold/pgbadger
- pg_wait_sampling: https://github.com/postgrespro/pg_wait_sampling
- SQLancer: https://github.com/sqlancer/sqlancer

## Design implications

### pgbench

PostgreSQL's built-in benchmark driver supports custom scripts, script weights, multiple clients/threads, fixed random seeds, non-uniform random generators (including Zipfian), rate-limited arrival scheduling, latency limits, per-transaction logs, and explicit serialization/deadlock retry/failure reporting.

Therefore the core workload layer uses pgbench instead of inventing a new high-load driver.

### EXPLAIN

EXPLAIN ANALYZE executes the statement and instrumentation can materially change runtime. `TIMING FALSE` reduces node-timing overhead while retaining row counts and total execution timing. This is why EXPLAIN evidence is diagnostic and pgbench evidence is the primary workload latency signal.

### pg_stat_statements

It exposes calls, total/mean/min/max execution time, shared/local/temp block counters/timing, WAL counters and planning metrics (when enabled). This supports ranking by total cost rather than only per-call slowness.

### Locks

`pg_locks`, `pg_stat_activity` and `pg_blocking_pids()` provide real server lock/wait evidence. The docs note frequent `pg_blocking_pids()` calls have overhead; the transaction harness samples it only in controlled test scenarios.

### Optional tooling

pgBadger and pg_wait_sampling are deliberately optional. The core workflow stays functional with PostgreSQL built-ins plus pgbench/psycopg.
