# Client, connection-pool, pooler and timeout qualification

Many “slow SQL” incidents are not execution-time problems. Requests can wait for a client pool slot, a transaction can remain open while application code performs unrelated work, cancellation may arrive after a write committed, and transaction-pooling proxies can change prepared-statement/session behavior.

## Model the full latency budget

Separate when observable:

```text
application queue
client pool acquire
network/connect/TLS
server execution
lock wait
statement execution
result transfer
commit acknowledgement
retry/backoff
```

Do not attribute total API latency to `EXPLAIN Execution Time`.

## Required review questions

- pool max/min/idle/lifetime and acquisition timeout;
- max application concurrency versus max DB connections and pooler capacity;
- transaction scope: no network RPC/user wait inside DB transaction without explicit justification;
- `statement_timeout`, `lock_timeout`, `idle_in_transaction_session_timeout` and client deadline relationship;
- server cancellation semantics and ambiguous outcomes after client timeout;
- connection reset/failover recovery;
- session state (`SET`, temp tables, advisory locks, prepared statements) compatibility with the chosen pooler mode;
- generic/custom prepared-plan behavior if prepared statements are used;
- retry amplification when pool/DB is already saturated.

## Evidence

Combine application tracing/pool metrics with PostgreSQL waits and workload evidence. A database-only benchmark cannot prove the application has a healthy pool/deadline contract.
