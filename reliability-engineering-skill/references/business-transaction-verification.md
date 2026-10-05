# Business Transaction Verification

Business reliability testing treats **business operations** as the unit of concurrency. SQL statements are implementation details; production incidents usually arise because independent business flows compose badly.

## Required transaction inventory

For every transaction-bearing business operation, derive from the real code:

- business operation / endpoint / job name;
- transaction begin/commit boundary;
- isolation level;
- rows/tables/resources read and written;
- intended lock acquisition order when multiple mutable entities are touched;
- predicates read before writes (possible write-skew / serialization surface);
- external network/RPC/MQ/payment calls made while a DB transaction is open;
- retry handling for `40001` and `40P01` where applicable;
- idempotency key and duplicate semantics;
- transaction duration budget;
- hotspot keys or singleton rows;
- whether multi-entity locks use a canonical stable key order independent of request direction;
- business invariant that must hold after commit/rollback/retry.

Do not infer the inventory only from table names. Read the application transaction code and its callers.

## Cross-business conflict matrix

Construct pairwise combinations for operations that touch the same mutable entities. In CRITICAL paths, add important 3-way combinations.

A pair is especially suspicious when:

- both update the same rows or index ranges;
- the **same operation** can lock multiple instances of one resource in request-dependent order (for example transfer A→B racing with B→A);
- they acquire two or more shared resources in different orders;
- one performs check-then-write while another changes the checked predicate;
- both use `SELECT ... FOR UPDATE` over overlapping but differently ordered sets;
- they touch a shared singleton/account/aggregate row;
- one transaction calls an external service while locks remain held;
- retry of one business operation can repeat an external side effect;
- transactions are long enough for otherwise harmless contention to amplify.

## Test layers

### 0. Self-conflict / mirrored-input tests

Before pairwise cross-business testing, test an operation against another execution of itself when it can touch multiple entities. Generate mirrored/permuted entity orders such as `A→B` vs `B→A`, `items=[1,2,3]` vs `[3,2,1]`, and overlapping batches. A canonical sort-before-lock rule should make the physical lock order identical.

### 1. Application-path concurrency tests

Preferred evidence. Invoke the real HTTP/gRPC/job handler concurrently with the real PostgreSQL engine. Use the same ORM/query layer and transaction wrapper as production. Verify final business invariants, not only response codes.

For multi-service business flows, create an isolated sandbox (Compose/Testcontainers/ephemeral namespace or the project equivalent) containing the real services that own the transaction boundaries. Do not replace the interaction with mocks when the property under test depends on real transaction timing, retries, or service/database integration.

To force a specific interleaving, add **test-only transaction hooks/barriers** at semantically meaningful points, for example:

```text
before_lock_order
order_locked
after_balance_read
before_commit
```

The hook must not change production semantics and should be absent/disabled outside tests. A deterministic hook is stronger evidence than hoping a race appears under load.

### 2. Database interleaving harness

Use `scripts/business_txn_lab.py pg-run` against a disposable real PostgreSQL database when a precise lock schedule must be reproduced. It can coordinate independent sessions with barriers and sample `pg_stat_activity` + `pg_blocking_pids()` while the scenario runs.

Prepare the isolated harness with:

```text
python3 scripts/verification_toolchain.py install --root <repo> --capability business-transaction-postgres --apply
<repo>/.reliability/py-tools/bin/python <skill>/scripts/business_txn_lab.py pg-run --scenario <scenario.toml> --dsn <test-dsn>
```

On Windows use the venv Python under `.reliability/py-tools/Scripts/python.exe`. Never point the example deadlock scenario at production.

This layer proves database interaction semantics; it does **not** replace application-path tests.

### 3. Mixed workload / contention campaign

Use production-shaped operation mixes and hot-key distributions. PostgreSQL `pgbench` supports custom scripts, multiple concurrent clients, weighted scripts, latency limits, and detailed serialization/deadlock failure accounting. Use it when the workload can be faithfully expressed as SQL; otherwise drive the application API with an equivalent workload runner.

Monitor at least:

- successful/failed transaction counts by business operation;
- `40001` serialization failures;
- `40P01` deadlock failures;
- retries and exhausted retries;
- p50/p95/p99 transaction duration;
- lock wait duration;
- count and age of `idle in transaction` sessions;
- `pg_stat_database.deadlocks` delta;
- wait-for graph samples for long waits;
- invariant/reconciliation result after the campaign.

A test that creates high TPS while corrupting business state is a failure.

## PostgreSQL-specific facts that shape the test strategy

- PostgreSQL detects deadlocks and aborts one participant. The documented best defense is consistent lock acquisition order across applications.
- Serializable and Repeatable Read transactions may require whole-transaction retry on SQLSTATE `40001`.
- Deadlock detection uses SQLSTATE `40P01`; retry may be appropriate, but retry is not a substitute for fixing a systematic lock-order inversion.
- `pg_blocking_pids(pid)` exposes sessions blocking a backend and is useful for a wait-for graph observer.
- Long-lived transactions amplify lock contention; application code should not hold a transaction open while waiting for user/network latency without an explicit design reason.

## Release gates for business transactions

Do not use a universal numeric threshold. Define budgets per service/operation. A CRITICAL transaction release should normally fail when any of these occur unexpectedly:

- unhandled `40P01` or `40001`;
- retry replays only part of a transaction instead of the complete business attempt;
- duplicate external/business side effect after retry;
- invariant violation after concurrent execution;
- deterministic lock-order inversion scenario exists without mitigation;
- sustained lock wait beyond the operation budget;
- transaction exceeds its documented duration budget under expected contention;
- `idle in transaction` appears in the tested application path;
- retry storm turns contention into outage;
- a business transaction holds database locks while waiting on an uncontrolled remote call and no explicit safety rationale exists.

## "Cannot test" rule

For transaction/concurrency risks, `cannot test` is not an acceptable first response.

Before marking a scenario BLOCKED, attempt in order:

1. run through the existing integration environment;
2. use an existing local/CI PostgreSQL instance;
3. use the project's container/Testcontainers setup;
4. create a disposable PostgreSQL environment matching production major version when authorized;
5. add deterministic test-only barriers/hooks to the application transaction path;
6. use the database interleaving harness for a reduced reproduction;
7. use a generated load campaign for non-deterministic contention surfaces.

Only then report BLOCKED, with the exact missing prerequisite or observability limitation. Never silently replace real transaction evidence with mocks.

## References

- PostgreSQL explicit locking/deadlocks: https://www.postgresql.org/docs/current/explicit-locking.html
- PostgreSQL transaction isolation / serialization retry: https://www.postgresql.org/docs/current/transaction-iso.html
- PostgreSQL error codes (`40001`, `40P01`): https://www.postgresql.org/docs/current/errcodes-appendix.html
- PostgreSQL `pg_blocking_pids`: https://www.postgresql.org/docs/current/functions-info.html
- PostgreSQL `pgbench`: https://www.postgresql.org/docs/current/pgbench.html
