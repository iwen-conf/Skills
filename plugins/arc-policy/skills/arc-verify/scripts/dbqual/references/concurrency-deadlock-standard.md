# Concurrency, deadlock and transaction verification standard

## 1. Three different questions

Do not conflate them:

1. **Can the test harness detect a deadlock?** A deliberate deadlock self-test should force an opposite lock order, observe the wait-for cycle when possible, and see PostgreSQL resolve it with `40P01`.
2. **Can the business code avoid avoidable deadlocks?** Production-contract scenarios should exercise real operation pairs and normally require `40P01 = 0`, unless the product explicitly accepts retryable deadlocks.
3. **If a transaction aborts, is retry safe?** The whole logical operation must be retried or reconciled, including idempotency, outbox/audit rows, external effects and response ambiguity.

## 2. Static lock-order design linter

`lock_order_analyzer.py` consumes an explicit business transaction catalog and builds a directed resource-acquisition graph. A cycle is a design risk and requires either redesign or a real DB scenario proving why it cannot occur.

It is not a proof of deadlock freedom. PostgreSQL can deadlock through rows, indexes, FKs, triggers, predicates, advisory locks, upserts, DDL and dynamic key order that a symbolic catalog may not model.

## 3. Deterministic concurrency

Use `concurrency_scenario_runner.py` when the dangerous interleaving can be named.

Evidence includes:

- per-session SQLSTATEs;
- per-step elapsed time;
- `pg_stat_activity` wait events;
- `pg_blocking_pids()` wait-for edges;
- observed wait-for cycles;
- approximate lock-wait tail;
- independent final-state assertions.

Typical cases:

- transfer A→B vs B→A;
- cancel vs worker claim/complete;
- reserve capacity vs cancel/release;
- approve vs revoke;
- callback vs operator action;
- idempotent retry vs original request;
- duplicate message delivery;
- same logical operation with mirrored keys;
- FK parent delete vs child insert;
- upsert on adjacent unique keys.

## 4. Seeded contention exploration

`contention_fuzz_runner.py` varies per-step delays using a persisted seed. It is intended to find schedules not covered by hand-written scenarios.

A random schedule without an oracle is not verification. Every fuzz portfolio needs final invariants and an explicit policy for `40P01`, `40001`, timeout and unexpected SQLSTATEs.

A failing example must preserve:

- base seed;
- iteration number;
- generated per-step delay schedule;
- SQLSTATEs;
- final assertion results.

This provides an approximate reproducer. OS/database scheduling is not fully deterministic, so important discovered races should be converted into deterministic barrier scenarios or application failpoints.

## 5. Serializable / repeatable-read behavior

At PostgreSQL `SERIALIZABLE`, `40001` is expected under some conflicting histories; its presence is not automatically a bug. The application must be able to retry the entire logical transaction safely.

For critical financial/inventory/allocation state, consider an additional history/model checker such as Jepsen/Elle-style dependency-cycle verification when the operation model is tractable. Do not claim complete serializability verification from a handful of invariants.

## 6. Deadlock budgets

For most ordinary OLTP application paths, prefer a production contract of zero unexpected deadlocks in the tested workload. If deadlock retry is intentionally part of the design, define:

- which operation classes may deadlock;
- maximum retry count;
- backoff/jitter;
- end-to-end latency budget including retries;
- idempotency semantics;
- side-effect behavior;
- alert threshold on production `40P01` rate.

Never hide deadlocks by simply increasing retry counts.
