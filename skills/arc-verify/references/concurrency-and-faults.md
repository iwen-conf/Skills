# Concurrency, Idempotency & Fault Verification (`并发幂等与故障取证规范`)

The unit of concurrency is a **business operation**, not an isolated function call. Load this guide whenever a storyline touches money, inventory, quotas, state transitions, webhooks, or async consumers.

## 1. Minimum Mandatory Experiment Matrix

| Risk Surface | Minimum Real Experiment | Required Independent Oracle |
|---|---|---|
| **Parallel Same-Op Race** (`cancel(X) \|\| cancel(X)`, `withdraw \|\| withdraw`) | `>= 2` concurrent workers synchronized via barrier at the racy entry point (`-race` enabled on Go targets) | Exactly 1 winner transition; loser rejected cleanly; balance/stock never negative; side effects emitted once |
| **Mirrored / Cross-Op Race** (`transfer(A,B) \|\| transfer(B,A)`, `cancel \|\| webhook_paid`) | Concurrent requests in opposite entity order or competing state transitions | Zero deadlock (`40P01`), deterministic winner per contract, end state of order + wallet + outbox mutually consistent |
| **Idempotency Key Matrix** | ① Same key + same params (sequential & concurrent); ② Same key + mutated params; ③ Distinct key on already-settled entity | ① 1 logical effect + deterministic replay response; ② Explicit `409`/`422` parameter conflict; ③ Guarded state rejection |
| **Mid-Transaction / Boundary Crash** | Inject failure between DB write and external RPC/MQ ack, or timeout client while server commits | Outbox/reconciliation converges within SLA or lands in explicit compensation state; timeout never treated as rollback |

## 2. Database & Transaction Evidence Rules

1. **CAS & Row-Lock Proof**:
   - Verify state mutations guard `WHERE status = expected` (or `SELECT ... FOR UPDATE` in canonical primary-key order) by firing concurrent conflicting updates against a cloned PostgreSQL/SQLite shard (`checkpoint` engine `postgres` or `sqlite`).
   - A passing Go `-race` test only checks in-memory goroutines; it **never** proves SQL isolation or prevents `write skew` / `lost update` across DB connections.
2. **PostgreSQL Contention Diagnostics**:
   - `40001` (serialization failure): Whole business transaction must retry cleanly from fresh state readback without duplicating external calls.
   - `40P01` (deadlock): Capture `pg_locks` / `pg_stat_activity` statement paths—treat systematic lock-order inversion as a `FAIL`, not something hidden by silent client retries.
   - Inspect `idle in transaction` leaks and verify connection pool recovery after request cancellation.
3. **Conservation Invariants (`守恒对账`)**:
   - For wallets, points, inventory, or carbon/energy ledgers: assert `sum(buyer_delta) + sum(merchant_delta) + sum(platform_fee) + sum(suspense_escrow) == 0` (or `initial_stock == remaining_stock + allocated_stock`) in `butterfly_probes` after concurrent storms and partial failures.

## 3. The "Cannot Test" Escalation Ladder (Before Claiming `BLOCKED`)

Attempt in strict order before marking concurrency/E2E verification `BLOCKED`:

1. Existing project integration/E2E environment or authorized test server.
2. Ephemeral local/container DB cloned via `arc verify checkpoint build --engine postgres|sqlite`.
3. In-process HTTP/gRPC server wired to real local SQLite/Turso/Postgres schema migrations (zero mocked repositories).
4. Deterministic barrier / failpoint wired through test dependency injection (if production seam is missing, request a minimal DI seam from `arc-code`—never add production rule-bypass flags).
5. Only if all 4 are blocked by missing external credentials/hardware (e.g., unavailable `redroid`/`avdslim` device or third-party sandbox): record `BLOCKED` with runnable test scripts ready and exact missing prerequisite documented.
