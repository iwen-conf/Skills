## 4. Business concurrency and transaction standard

Business workflows often fail only when combined. Test interactions among operations that share:

```text
entities
quotas/counters
rows/indexes
locks
state machines
outbox/inbox/events
idempotency keys
external side effects
scheduled jobs
```

Required investigation patterns when applicable:

### Same operation against itself

Examples:

```text
cancel(X) || cancel(X)
transfer(A,B) || transfer(B,A)
reserve(last-seat) || reserve(last-seat)
update(version=N) || update(version=N)
```

Do not assume one function always acquires locks consistently; mirrored parameters can invert entity lock order.

### Different business operations

Examples:

```text
cancel enrollment || operator move participant
payment callback || user cancellation
refund || settlement
inventory deduction || order close
role removal || resource grant
scheduled expiry || manual approval
```

Build a cross-business matrix from shared state and side effects.

### Real PostgreSQL evidence

When PostgreSQL semantics matter:

- use real transactions;
- force important interleavings with explicit barriers/failpoints when useful;
- observe `pg_stat_activity`, `pg_locks`, `pg_blocking_pids()` where possible;
- capture SQLSTATE such as `40001` and `40P01`;
- verify the **entire business transaction** is retried correctly when required;
- verify retry does not duplicate business effects;
- measure lock-wait and transaction-duration tails;
- use representative contention workloads when lock amplification/hot rows are plausible.

Run deterministic scenarios with:

```bash
python3 "$SKILL_ROOT/scripts/business_txn_lab.py" pg-run \
  --scenario postgres-deadlock-scenario.toml \
  --dsn "$TEST_DATABASE_URL"
```

Retry is not a substitute for correcting deterministic lock-order defects.

## 5. Domain directives

### Permissions/authentication

Verify deny paths, tenant boundaries, stale/revoked grants, cache equivalence and batch/single semantic equivalence. Favor a simple independent reference model for optimized permission evaluators.

### Messaging/jobs/webhooks

Verify duplicate delivery, out-of-order delivery, delayed delivery, handler restart, poison payload, idempotency, partial side effect, and reconciliation/final state.

### Embedded/device/protocol

Separate protocol parsing, business state, driver behavior and physical hardware. Test invalid/truncated frames, CRC, byte order, stale measurement, disconnect/reconnect, persistence/restart and long-running resource behavior. Use HIL where physical behavior matters.

### Migration/upgrade

Test representative old data, old->new, mixed-version periods where applicable, interruption/restart, rollback/downgrade where supported, backup restore, and post-migration business invariants.

### Rust

Choose from project needs, e.g. `cargo test`, Clippy, nextest, llvm-cov, proptest, cargo-fuzz, Loom, cargo-mutants, Miri/sanitizers, Testcontainers. Respect `#![forbid(unsafe_code)]` project policy when present.

### Go

Choose from project needs, e.g. `go test`, `-race`, native fuzzing, coverage, `go vet`, staticcheck, govulncheck, Testcontainers. Test goroutine lifetime, context cancellation, channel ownership, connection cleanup and real dependency semantics where relevant.

## 6. CI / cadence standard

Do not make every expensive test a PR test.

### Developer / PR

Target changed claims with fast deterministic evidence:

```text
business plan validation
focused unit/property/component tests
relevant real-dependency integration
static/race checks appropriate to change
required release-scenario subset for changed behavior
```

### Main / nightly

Run broader expensive evidence:

```text
cross-business concurrency matrix
fuzz campaigns
mutation sampling
broader compatibility matrix
fault injection
load/stress/soak slices
migration/recovery suites
```

### Pre-release

Run the complete release-required scenario set, upgrade/rollback/recovery and production-like E2E appropriate to risk.

### Production

Use staged rollout, SLI/invariant monitoring, reconciliation, synthetic probes and rollback/containment.

