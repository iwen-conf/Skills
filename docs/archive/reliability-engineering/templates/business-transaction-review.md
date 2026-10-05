# Business Transaction Review

## 1. Business transaction inventory

| Operation | Entry point | Isolation | Transaction boundary | Reads | Writes/locks | Lock order | External calls while open | Retry | Idempotency | Budget |
|---|---|---|---|---|---|---|---|---|---|---|

## 2. Self-conflict cases

List mirrored/permuted inputs for operations that lock multiple entity instances.

| Operation | Input A | Input B | Expected canonical lock order | Test/harness | Result |
|---|---|---|---|---|---|

## 3. Cross-business conflict matrix

| Operation A | Operation B | Shared mutable resources | Order inversion | Predicate conflict | Hot key | External-call amplification | Required test |
|---|---|---|---|---|---|---|---|

## 4. Deterministic schedules

For every high-risk pair, describe the schedule explicitly.

```text
A: begin
A: lock X
B: begin
B: lock Y
A: request Y -> waits
B: request X -> expected deadlock / prevention behavior
```

## 5. Business invariants after concurrency

- 

## 6. Runtime evidence

- SQLSTATE counts (`40001`, `40P01`, others):
- retry attempts / exhausted retries:
- pg_stat_database.deadlocks delta:
- lock wait p50/p95/p99:
- transaction p50/p95/p99:
- longest `idle in transaction`:
- captured wait-for graph / blocking PIDs:
- reconciliation result:

## 7. Design findings

Prioritize fixes such as canonical lock order, shorter transaction scope, moving remote I/O outside locks, database constraints, outbox/saga boundaries, or whole-transaction retry. Do not hide a deterministic design defect behind unlimited retries.
