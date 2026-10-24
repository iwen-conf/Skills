# Transaction Design Review

Check that each business operation's transaction design forbids the anomalies its invariants
cannot tolerate, on the engine and version actually deployed. The output is a design table whose
rows become runtime claims ([txn-faults.md](txn-faults.md); PostgreSQL release paths add db mode
G7–G11). Changing the design is `arc:code` work.

## 1. Inputs

| Input | Source |
|---|---|
| Engine, version, managed-service variant | deployment config; `SELECT version()` on the test instance |
| Effective isolation per operation | server default, pool or session setting, explicit `SET TRANSACTION` or ORM option, read from config and code |
| Invariants and the anomalies they forbid | contract, closure model, incident records |
| Transaction code | the real entry point and its transaction helper: boundary, statements in order, calls made inside, retry wrapper |

A missing input makes its row a gap. `BEGIN … COMMIT` text alone supports only generic ACID
remarks; it cannot decide whether a flow needs `SERIALIZABLE`.

## 2. Boundary

- Atomic set: writes an invariant ties together commit in one transaction (debit and credit,
  stock decrement and order line, state transition and its outbox row).
- Everything else is eventually consistent by design, and the row names how: outbox or queue,
  idempotent consumer, deadline, reconciliation query, compensation.
- No network call, broker publish, payment or email inside a transaction; use an outbox, or state
  the timeout and the unknown-outcome path.
- Short transactions: no user think time or unbounded loops; lock and statement timeouts set for
  the operation; multi-row locks taken in one canonical order.
- Read-only work declares `READ ONLY` and avoids long snapshots on the primary.

## 3. Forbidden anomaly → mechanism

Prefer what the database enforces (constraint, single statement) over application checks.

| Anomaly | Shape | Mechanisms (the row names one) |
|---|---|---|
| Lost update | read a value, compute, write back the same row | one statement `UPDATE … SET qty = qty - :n WHERE id = :id AND qty >= :n` with an affected-rows check; `SELECT … FOR UPDATE` then write; version column `WHERE version = :v` (0 rows = conflict); PostgreSQL REPEATABLE READ or SERIALIZABLE with `40001` retry |
| Write skew | each transaction checks a predicate over rows the other one writes (≥1 on call; allocations ≤ quota) | SERIALIZABLE where the engine detects it (PostgreSQL SSI, CockroachDB); lock a shared parent or aggregate row `FOR UPDATE`; a constraint that encodes the invariant |
| Phantom, double booking | check-then-insert on a key or range | unique or exclusion constraint (PostgreSQL `EXCLUDE USING gist` for ranges); `INSERT … ON CONFLICT`; predicate locks (SERIALIZABLE, InnoDB next-key locks under a locking read) |
| Duplicate processing | several workers claim one job | claim with `FOR UPDATE SKIP LOCKED` and the state transition in the same transaction |
| Duplicate effect after retry | client, driver or job retries after a timeout | idempotency key under a unique constraint with the stored result; outbox for external effects |
| Inconsistent multi-statement read | a report spans several reads | one statement, or a read-only snapshot transaction |

## 4. Engine semantics

Defaults and behaviors change across versions and managed services; confirm each against the
deployed version's documentation and record the source.

| Engine | Default | Facts that change the design |
|---|---|---|
| PostgreSQL | READ COMMITTED | Each statement takes a new snapshot. `UPDATE … SET qty = qty - 1` is safe because it re-evaluates the latest row version; a value read in app code and written back loses updates. REPEATABLE READ is snapshot isolation: a concurrent update of the same row raises `40001`, write skew remains possible. SERIALIZABLE (SSI) also detects write skew with `40001`. READ UNCOMMITTED behaves as READ COMMITTED. Deadlock `40P01`. |
| MySQL / MariaDB (InnoDB) | REPEATABLE READ | A plain `SELECT` reads the snapshot taken at the first read, while `UPDATE`, `DELETE` and locking reads act on the latest committed rows with next-key (record + gap) locks: a snapshot read followed by a computed write can lose updates. Gap locks block inserts and add deadlocks; READ COMMITTED drops gap locking for searches. SERIALIZABLE turns plain `SELECT` into locking reads when autocommit is off. Deadlock 1213 rolls back the transaction; lock wait timeout 1205 rolls back only the statement unless `innodb_rollback_on_timeout` is on, so roll back explicitly. |
| SQL Server | READ COMMITTED with locks; row versioning when `READ_COMMITTED_SNAPSHOT` is on (the Azure SQL Database default) | RCSI reads do not block writers, and read-then-write can lose updates (use `UPDLOCK` or a single statement). SNAPSHOT needs `ALLOW_SNAPSHOT_ISOLATION` and fails update conflicts with error 3960. SERIALIZABLE takes key-range locks. Deadlock victim error 1205. |
| Oracle | READ COMMITTED | SERIALIZABLE is snapshot isolation: write conflicts raise ORA-08177 and write skew remains possible. No REPEATABLE READ level. `SELECT … FOR UPDATE` for read-then-write. |
| CockroachDB | SERIALIZABLE | Retry errors use SQLSTATE `40001`; the client retries the whole transaction. READ COMMITTED exists only in newer versions where enabled: check version and cluster setting. |
| Google Spanner | serializable, externally consistent read-write transactions | Aborted transactions are retried by the client library, which re-runs the function: keep it free of side effects. Check the version for other isolation options. |
| SQLite | serializable, one writer | `BEGIN IMMEDIATE` for read-then-write to avoid `SQLITE_BUSY` on lock upgrade; set `busy_timeout`. |

Never map the SQL standard's four-level table onto an engine: the same name gives different
anomalies on PostgreSQL, InnoDB, SQL Server and Oracle.

## 5. Retry and timeouts

- One layer owns retry: on serialization failure or deadlock (codes above) it reruns the whole
  transaction and re-reads state, with max attempts, a total deadline and jittered backoff.
  Never retry only the failed statement; never resend an external effect without an idempotency key.
- After a timeout or a lost connection the commit outcome is unknown: confirm through durable
  state or the idempotency record before retrying.
- Lock and statement timeouts are explicit per operation (PostgreSQL `lock_timeout` and
  `statement_timeout`, MySQL `innodb_lock_wait_timeout`, SQL Server `SET LOCK_TIMEOUT`).

## 6. Output

```text
| Operation | Atomic set | Invariant → forbidden anomaly | Engine · version · isolation | Mechanism | Retry owner · codes | Claim · test |
```

- A mechanism that cannot prevent its anomaly on that engine (an InnoDB snapshot read followed
  by a computed balance write; "REPEATABLE READ prevents write skew" on PostgreSQL) is a confirmed
  design defect: a finding with the row for `arc:code`.
- A sound row is not a runtime `PASS`. Each becomes EXPECTED/FORBIDDEN claims run on the real
  engine with the [txn-faults](txn-faults.md) §0 minimum cases; PostgreSQL releases add the G7
  isolation litmus and G8 lock-order check. A row without a runnable claim is `NOT_RUN`.
