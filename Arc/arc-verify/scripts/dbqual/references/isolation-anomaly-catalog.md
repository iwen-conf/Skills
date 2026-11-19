# Transaction Isolation and Concurrency Anomaly Catalog

Use this catalog to derive deterministic scenarios. It is not a claim that every anomaly applies to every workload.

## 1. Core anomaly families

### Check-then-act race

```text
T1: SELECT state/capacity
T2: SELECT same state/capacity
T1: UPDATE/INSERT based on observation
T2: UPDATE/INSERT based on stale observation
```

Oracle: capacity, uniqueness, money, state-machine or ownership invariant.

### Lost-update family

Two logical operations derive new values from the same prior state. Test both SQL-level atomic updates (`SET x=x+1`) and application read-modify-write patterns.

### Write skew

Two transactions update disjoint rows after checking a cross-row invariant. This is a classic reason that row-level locks on only the modified rows may be insufficient. Test under the isolation level actually used.

### Non-repeatable read / phantom-sensitive logic

A transaction observes changing rows or newly matching rows and then makes a decision. Decide whether Read Committed is acceptable or whether stronger isolation/locking is required.

### Unique-key race / idempotency race

Two callers create the same logical object. Verify the database uniqueness contract, application error mapping, retry semantics and external side effects. PostgreSQL documentation notes that some `23505`/`23P01` failures can represent transaction-retry situations depending on how keys are selected.

### Worker vs user/operator race

Examples:

```text
cancel vs delivery
refund vs capture
expire vs renew
approve vs revoke
retry worker vs original callback
```

The test must define the product semantics first, then force both orderings.

### Lock-order deadlock

Force opposite acquisition orders and prove the harness observes a simultaneous wait-for cycle. Production scenarios should normally enforce consistent ordering and require unexpected `40P01 == 0`.

### Serialization conflict

Under Repeatable Read/Serializable, expect `40001` where PostgreSQL rejects a history that cannot safely commit. Verify whole-transaction retry and final-state invariants.

### Starvation / unfair retry loop

Even without deadlock, one operation may repeatedly lose lock/serialization races. Track attempt count, time-to-success and retry amplification.

### Advisory-lock protocol violation

If advisory locks protect logical resources, test inconsistent key derivation, missing acquisition, different lock modes, session-vs-transaction scope and crash/reconnect semantics.

### `SKIP LOCKED` work queue semantics

Verify no item is permanently stranded, duplicated or starved; row ordering and lease/retry semantics matter more than raw query success.

## 2. Isolation-level portfolio

For every write workflow record the actual isolation level. Then test the assumptions made by the application.

### Read Committed

Each command can see a new committed snapshot. Multi-command business checks require care. Prefer atomic DML, constraints or explicit locking where possible.

### Repeatable Read

Provides a stable transaction snapshot but can reject conflicting updates. Application must handle `40001`. It does not make every cross-row business invariant automatically safe.

### Serializable

Use when the application wants outcomes equivalent to some serial execution, but budget for serialization failures and retries. Test retry storms and starvation, not only correctness.

## 3. Deterministic scenario requirements

A critical concurrency scenario should record:

```text
initial durable state
session identities
transaction isolation
statement_timeout / lock_timeout
barrier/failpoint positions
observed wait-for graph snapshots
SQLSTATE counts (explicit zero included)
transaction attempts/retries
final durable state
independent invariant assertions
external side-effect assertions
seed/schedule when generated
```

## 4. Retry contract

Do not write “retry 40001” without answering:

- Is the entire transaction function re-run from the beginning?
- Are IDs/random choices regenerated or stable?
- Are external calls inside or outside the transaction?
- Is an idempotency key stable across attempts?
- Does retry create duplicate outbox/audit records?
- What is the maximum attempts/time budget?
- Is backoff/jitter needed under contention?
- What happens after retry exhaustion?

## 5. History/model checking

For DB-R4/R5, hand-crafted examples and seeded fuzz remain finite experiments. When correctness depends on a broad transactional history, export operations as a history and use an explicit model/checker such as Elle/Jepsen or a domain-specific checker. Persist the model, checker version, failure history and minimized counterexample.
