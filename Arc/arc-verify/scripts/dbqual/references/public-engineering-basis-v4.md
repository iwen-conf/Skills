# Public engineering basis — final edition

Last reviewed: 2026-09-20.

This package does **not** claim to reproduce any company's private internal release system. It converts publicly documented engineering principles into an auditable PostgreSQL-oriented qualification model.

## Google / SRE

Public references:

- https://sre.google/workbook/canarying-releases/
- https://sre.google/sre-book/testing-reliability/
- https://sre.google/sre-book/reliable-product-launches/
- https://sre.google/resources/practices-and-processes/twenty-years-of-sre-lessons-learned/

Principles reflected here:

- releases should be repeatable and automated;
- canary changes against representative live signals before global rollout;
- recovery mechanisms should be rehearsed before an emergency;
- stress testing should identify capacity/failure boundaries, not merely prove a nominal case;
- canary signals should be representative and attributable to the change.

Mapped gates: G13, G15, G23-G24, G29-G31.

## Oracle Real Application Testing

Public references:

- https://docs.oracle.com/en-us/iaas/autonomous-database-serverless/doc/autonomous-real-application-testing-about.html
- https://docs.oracle.com/en/database/oracle/oracle-database/19/ratug/introduction-to-database-replay.html
- https://docs.oracle.com/en/database/oracle/oracle-database/21/ratug/replaying-a-database-workload.html

Principles reflected here:

- realistic workload testing must preserve timing, concurrency and transaction shape;
- test-system data should be logically similar enough to avoid meaningless replay divergence;
- system-level workload replay and SQL-centric plan/performance comparison are complementary.

Mapped gates: G2, G6, G12-G14, G28.

The package's replay layer is an application-level PostgreSQL harness; it does **not** claim Oracle Database Replay kernel-level capture fidelity.

## Stripe

Public references:

- https://stripe.com/blog/idempotency
- https://stripe.com/blog/online-migrations

Principles reflected here:

- network/result ambiguity makes retries unsafe unless operations are designed to converge;
- idempotency keys and whole-operation retry semantics matter for side effects;
- retries require backoff/jitter to avoid amplification;
- large online migrations use staged compatibility, dual writes/backfill/read cutover/reconciliation rather than one destructive switch;
- migration load should ramp while operational signals are watched.

Mapped gates: G11, G13, G18, G29.

## Cloudflare

Public reference:

- https://developers.cloudflare.com/d1/best-practices/read-replication/

Cloudflare D1 is not PostgreSQL. This package uses the public D1 material only for the distributed-database principle that asynchronous replicas require an explicit application consistency contract. The D1 Sessions API documents sequential consistency, monotonic reads and read-your-writes through bookmarks while noting that replicas can lag.

Mapped gate: G20.

## PostgreSQL 18 official documentation

Primary technical authority for PostgreSQL behavior:

- Concurrency control: https://www.postgresql.org/docs/18/mvcc.html
- Serialization failure handling: https://www.postgresql.org/docs/18/mvcc-serialization-failure-handling.html
- Application consistency: https://www.postgresql.org/docs/18/applevel-consistency.html
- EXPLAIN: https://www.postgresql.org/docs/18/sql-explain.html
- pgbench: https://www.postgresql.org/docs/18/pgbench.html
- Monitoring: https://www.postgresql.org/docs/18/monitoring.html
- Runtime statistics: https://www.postgresql.org/docs/18/runtime-config-statistics.html
- Vacuum/XID maintenance: https://www.postgresql.org/docs/18/routine-vacuuming.html
- Backup/PITR: https://www.postgresql.org/docs/18/backup.html
- pg_verifybackup: https://www.postgresql.org/docs/18/app-pgverifybackup.html
- amcheck: https://www.postgresql.org/docs/18/amcheck.html
- Data checksums: https://www.postgresql.org/docs/18/checksums.html
- Logical replication: https://www.postgresql.org/docs/18/logical-replication.html
- Logical replication conflicts: https://www.postgresql.org/docs/18/logical-replication-conflicts.html

Important consequences encoded by this package:

- `40001` requires retrying the complete transaction logic, not a single failed SQL statement;
- `40P01` may also be retryable, but a retry policy does not excuse avoidable deadlock design;
- prepared statements may use custom or generic plans; parameter-sensitive SQL must test both when relevant;
- `EXPLAIN ANALYZE` executes the statement and adds measurement overhead, so its timing is diagnostic rather than a substitute for workload latency;
- `pg_verifybackup` is not a substitute for a real restore test;
- `amcheck` can establish corruption when it finds it but cannot prove total absence of corruption;
- asynchronous replication requires explicit lag/visibility/conflict monitoring;
- vacuum/freeze/statistics/replication-slot health are correctness and availability concerns, not only DBA housekeeping.
