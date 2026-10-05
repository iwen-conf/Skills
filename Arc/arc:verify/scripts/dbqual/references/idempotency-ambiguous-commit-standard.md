# Retry, idempotency and ambiguous commit standard

A database retry policy is incomplete unless it covers the business operation around the transaction.

Test at least these boundaries when they apply:

```text
failure before transaction begins
failure after work begins but before commit
serialization/deadlock abort
commit succeeds but client loses response
connection reset while commit result is unknown
process restart after durable commit but before external acknowledgment
retry races with the original attempt
same idempotency key + same payload
same idempotency key + conflicting payload
retry storm / backoff / jitter
```

Oracles should independently establish:

```text
one logical operation -> allowed number of durable effects
ledger/stock/capacity invariant
idempotency record/result stability
outbox/audit cardinality
external side effect cardinality if observable
bounded retry count and end-to-end latency
```

`40001` and `40P01` are not the oracle. They are intermediate outcomes. The final oracle is convergence of business truth.
