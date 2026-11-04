# Transaction and lock verification

## Static review is not enough

Review lock/entity order, transaction duration, isolation, retries and external calls statically, but validate important concurrency behavior against real PostgreSQL.

PostgreSQL can acquire locks through indexes, foreign keys, triggers, predicate locking, upserts and plan-dependent access patterns that a simplified design diagram may omit.

## Deterministic scenario design

Use barriers around business-meaningful milestones:

```text
A acquired first entity
B acquired first entity
both proceed to second entity
one commits / one blocks / conflict appears
```

The oracle is not merely "deadlock happened" or "no exception". Assert final durable business state.

## SQLSTATE

Track at least:

- `40001` serialization_failure
- `40P01` deadlock_detected

When retries are expected, test bounded whole-operation retry and idempotency at the application boundary.

## External IO

HTTP/MQ/RPC inside a long DB transaction should be treated as a risk. A rollback cannot necessarily undo an external side effect. Verify outbox/idempotency/compensation semantics where applicable.
