# Transactions, Concurrency and Faults

The unit of concurrency is a business operation, not a SQL statement. Read the real entry points,
callers and transaction wrapper before deriving interleavings; table names and lock lists are only
leads. Boundary, isolation and lock choices per engine: [txn-design.md](txn-design.md). PostgreSQL release qualification (isolation litmus, DDL locks, plans, replication,
recovery) is db mode: [db.md](db.md).

## 0. Minimum cases

These are experiment shapes. Their outputs come from confirmed RULEs, including side-effect
cardinality, accounting sources/sinks and recovery destinations. Undefined semantics remain
RISK/SPECIFICATION GAP; this table never creates a product rule.

| Claim | Run before wider exploration |
|---|---|
| Concurrency | ≥2 parallel actors through a barrier at the racy point; race detector on where available |
| Idempotency | same key ≥2 times sequentially and ≥2 concurrently, plus different parameters: contracted logical effect count and duplicate/mismatch response, including legitimate zero-effect outcomes |
| Transaction | failure between first write and commit: promised atomicity or explicitly contracted compensation and observable destination |
| Money, inventory, quota | the confirmed ledger invariant during and after the run, with system boundary, sources, sinks, fees, consumption and units explicit |

## 1. Transaction inventory

For each affected operation, from the code:

- operation / endpoint / job; begin and commit boundary; isolation level;
- rows, index ranges and predicates read and written (a predicate read before a write is a
  write-skew surface);
- lock and entity acquisition order; whether multi-entity locks use a canonical key order
  independent of request direction;
- network, RPC, MQ or payment calls made while a transaction is open;
- timeout and retry owner; handling of `40001` and `40P01`;
- idempotency key, its tenant/business scope, retention and duplicate semantics;
- outbox/inbox; hot keys and singleton rows; transaction duration budget (from the project; a
  missing budget is an open item);
- the invariant that must hold after commit, rollback, retry and recovery.

Idempotency cases are separate: same key + same parameters, same key + different parameters,
concurrent duplicates, replay after expiry or cleanup, and key scope. The contract decides each
outcome. A static lock-order catalog is a design linter, not proof of deadlock freedom.

## 2. Contention matrix

| Pair | Observe |
|---|---|
| Same op duplicated: `cancel(X) ‖ cancel(X)` | one transition; counts and external effects not duplicated |
| Same op mirrored: `transfer(A,B) ‖ transfer(B,A)` | locks taken in canonical order, not request order |
| Overlapping batches: `[1,2,3]` vs `[3,2,1]`, subsets | per-item lock order, atomicity, partial results |
| Different ops on shared state: cancel/callback, refund/settle, expire/approve, role removal/grant | the contract decides the winner; the loser's result and the end state agree |
| Risky 3-way: user / operator / scheduled job | shared-invariant breaks that every pair misses |

Use rows only where the business has them; start with the shared boundary this change touches,
then expand on evidence. A pair is suspicious when both write the same rows or index ranges, take
≥2 resources in different orders, one checks a predicate the other changes, both run
`SELECT … FOR UPDATE` over overlapping sets in different order, a singleton/aggregate row is
shared, locks are held across an external call, or a retry can repeat an external effect.

## 3. Forcing interleavings

- Prefer the real API, job or usecase and the production transaction/ORM layer against the real
  dependency. A reduced SQL reproduction explains database semantics; it does not prove the
  application's retry and side-effect chain.
- Place explicit barriers or failpoints at semantic points (`after_balance_read`,
  `first_lock_held`, `before_commit`). Every barrier has a timeout, cancel/cleanup and a
  replayable order. Reuse existing seams; new ones enter through test wiring or DI; a seam needed
  in production source goes to `arc:code`. Never `sleep` to guess timing.
- A barrier must be reachable under real lock semantics: never make the lock holder wait for a
  participant that needs the same lock to arrive. Tell an application race from a deadlock the
  test created; keep schedule and lock evidence.
- Record the history: invocation, completion or timeout, operation identity, result, observed end
  state. A timeout means outcome unknown, never rollback. Check the history against the
  consistency model the project promises; checking only requests that returned success misses
  side effects of unknown outcomes.

## 4. PostgreSQL semantics

| Observation | Evidence and verification |
|---|---|
| `40001` serialization failure | rerun the complete business attempt in a new transaction, re-reading state; verify the final invariant |
| `40P01` deadlock | save actual locks, waits and statement paths; confirm a systematic lock-order inversion; retry never replaces fixing it |
| Waits and tail latency | `pg_stat_activity`, `pg_locks`, `pg_blocking_pids()` scoped to this run's application or test identifiers |
| Unknown commit outcome | confirm through durable state, the idempotency record or reconciliation; never blindly resend the side effect |
| Hot spots, retry amplification | success/failure/retry-exhausted counts per operation, lock wait, transaction p95/p99, `idle in transaction` count and age, `pg_stat_database.deadlocks` delta, end-state check |

- Real locks also come from FKs, triggers, upserts, indexes, predicates, plans and dynamic key
  order. A passing `-race` run never proves database concurrency safety.
- Exactly one layer owns retries, with max attempts, total deadline, backoff and cancel; stacked
  retry layers multiply load. Retry the whole business transaction; never unconditionally replay
  payments or notifications. Verify the project's idempotency, outbox, compensation or
  reconciliation semantics; never impose a new pattern.
- High throughput with corrupted state is a failure.

For a critical transaction each of these is `FAIL` unless the contract expects it: unhandled
`40001`/`40P01`; a retry that replays only part of the transaction; a duplicate effect after
retry; an invariant violation after a concurrent run; a deterministic lock-order inversion with
no mitigation; lock wait or duration beyond budget under expected contention; `idle in transaction`
on the tested path; a retry storm turning contention into outage; locks held across an
uncontrolled remote call without a stated reason.

## 5. "Cannot test" ladder

For transaction and concurrency risks, try in order before `BLOCKED`:

1. the project's existing integration environment;
2. an existing remote or CI PostgreSQL test instance;
3. the project's container setup on the user-specified remote Docker host;
4. a disposable remote PostgreSQL at the production major version, when authorized;
5. test-only barriers in the application transaction path;
6. a reduced SQL interleaving reproduction on the test database;
7. a generated contention workload for non-deterministic surfaces.

Then report `BLOCKED` with the exact missing prerequisite or observability limit, with the
runnable binding and preconditions already written. Never replace real transaction evidence with
mocks.

## 6. Fault points from boundaries

| Boundary | Fault and assertion |
|---|---|
| DB → message / external API | fail before commit; commit then lose the response; send succeeds but the ack is lost: check loss, duplication, recoverable state |
| Consumer / async job | duplicate, reorder, delay, poison message, crash after processing before ack: check re-entrancy and final destination |
| Timeout / cancel | client cancel races server completion: deadline propagation, resource release, queryable end state |
| Network / resources | reset, slow dependency, pool or queue exhaustion: backpressure, retry amplification, progress after recovery |
| Persistence / migration | interrupted write, process restart, old data / mixed versions: recovery, invariants, supported rollback range |

- Before an authorized minimal fault experiment, write the hypothesis, operation scope (test
  accounts or tenants first), observers and stop condition. If the verdict depends on a lock
  observer or reconciler that is unavailable or fails, that is an evidence gap; a main test
  process exiting 0 is not a pass.
- From a critical user journey, inject each dependency failure along its request chain and check
  isolation, the degraded result and recovery.
- Recovery tests prove safety during the fault and bounded progress after it. Report performance
  and correctness separately.
- Overall health or a serving majority does not prove each affected task, shard or replica
  recovered; check local progress and visible failure destinations.
- When the recovery path itself is high risk, add a fault during recovery (I/O, resource,
  dependency) with conditions and expectations defined first; never stack unbounded chaos.
