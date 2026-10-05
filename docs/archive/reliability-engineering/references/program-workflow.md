## 3. Workflow for real project work

For first adoption, use these phases as a dependency order. For a scoped change load only affected phases; for read-only review inspect existing evidence without installing tools or running mutations.

### Phase 1 — Understand the change and business behavior

Inspect:

- requirement/design/issue/incident context;
- affected API/jobs/messages/devices;
- current production code and callers;
- current tests and fixtures;
- database/storage boundaries;
- side effects and integrations;
- feature flags/version/tenant differences;
- operational/release controls.

For an existing codebase, inspect exact files/functions before inventing test cases.

Determine which business rules actually changed. Do not retest the universe blindly.

### Phase 2 — Declare applicability

For the changed capability explicitly decide whether these concerns apply:

```text
stateful
permissions
concurrent
retryable
external_side_effects
time_sensitive
compatibility
migration
asynchronous
destructive
long_running
```

This is a completeness guard, not a score.

If `concurrent=false`, do not create concurrency theater. If `stateful=true`, a state model and transition scenarios are required.

### Phase 3 — Define business rules and oracles

For each important rule state:

```text
rule
consequence if wrong
observable evidence
exact assertion
```

Examples of strong business oracles:

```text
capacity changed exactly once
unauthorized actor caused zero durable state change
one logical command created at most one external effect
waitlist selection equals independent policy model
cancelled entity cannot later become active except through documented reopen flow
```

Prefer independent observation of final state over assertions on internal implementation calls.

If behavior is undefined, record **SPECIFICATION GAP**. Do not invent an answer solely so a test can be written.

### Phase 4 — Build the business scenario portfolio

Use only methods that match the rule shape:

- real user/business journeys;
- semantic boundaries;
- equivalence classes;
- decision tables;
- state-transition tests;
- duplicate/retry/ambiguous-result scenarios;
- cross-business composition;
- pairwise/combinatorial selection for large dimensions;
- property/model-based testing for broad rule spaces;
- exploratory charters when behavior is still emerging.

For every important scenario specify both:

```text
EXPECTED   what must happen
FORBIDDEN  what must never happen
```

Avoid blind Cartesian products. Target combinations by consequence, frequency, coupling, historical incidents, reversibility and detectability.

### Phase 5 — Model the system and shared state

Identify:

```text
entry points
mutable/durable entities
transactions and isolation
lock/entity acquisition order
queues/events/outbox/inbox
external calls
clocks/timeouts/expiry
retry/idempotency keys
caches
background jobs
startup/restart/recovery
migration/rollback
```

For business transactions, create/update the transaction catalog and analyze it:

```bash
python3 "$SKILL_ROOT/scripts/business_txn_lab.py" analyze \
  --catalog business-transactions.toml
```

Treat the static catalog as a **design linter**, not a proof of deadlock freedom. Real databases can deadlock via rows, indexes, predicates, FKs, triggers, upserts, execution plans and dynamic key order not visible in a simplified catalog.

### Phase 6 — Derive the failure model

Derive failures from real boundaries, not a generic checklist.

Common categories when applicable:

```text
invalid/boundary values
illegal state transition
duplicate/replay/reorder
race/lost update/check-then-act
deadlock/livelock/starvation
transaction conflict/ambiguous commit
network timeout/reset/partition
async redelivery/out-of-order callback
partial external side effect
storage/process restart
resource exhaustion/backpressure
version/schema/protocol skew
operator/config error
```

For each high-risk fault, name the threatened business rule/invariant.

### Phase 7 — Improve testability when required

If a real scenario cannot be controlled or observed, introduce the smallest useful seam:

```text
injectable clock
seeded RNG
stable test IDs
failpoint/barrier at meaningful transaction state
explicit state machine
reference model
isolated side-effect adapter
test tenant/namespace
observable outbox/audit/reconciliation view
```

Do not create abstractions with no real uncertainty/failure boundary behind them.

### Phase 8 — Choose evidence technique

Choose the cheapest technique that can actually falsify the claim.

| Failure shape | Primary evidence |
|---|---|
| deterministic business rule | examples + semantic boundaries |
| multi-condition policy | decision table + focused combinatorial tests |
| stateful workflow | state-transition/model tests + integration |
| optimized algorithm | reference/differential + property |
| parser/protocol/file | fuzz + corpus + semantic invariant |
| DB transaction/isolation | real database integration |
| cross-business concurrency | real app/DB concurrent scenario + deterministic barriers where useful |
| small critical concurrent primitive | systematic scheduler/model checker where available |
| retry/ambiguous outcome | idempotency model + fault-injected integration |
| async events | duplicate/reorder/redelivery + final-state oracle |
| migration/upgrade | representative lifecycle test |
| overload/backpressure | load/stress + correctness oracle |
| long-running resource behavior | soak + leak/queue/resource telemetry |
| subtle distributed protocol | model/generative history checking; formal spec when tractable |

Do not substitute a weaker technique solely because its tool is installed.

### Phase 9 — Realize missing toolchain safely

Only now inspect/install missing tools.

Use:

```bash
python3 "$SKILL_ROOT/scripts/verification_toolchain.py" doctor --root .
python3 "$SKILL_ROOT/scripts/verification_toolchain.py" plan --root . --capability <capability>
python3 "$SKILL_ROOT/scripts/verification_toolchain.py" install --root . --capability <capability> --apply
```

Rules:

- built-in/existing tool first;
- project dependency only when justified;
- user-scoped package-manager installation next;
- pinned helper container next;
- privileged/system changes are never silently performed;
- do not change default Rust toolchain merely for one test;
- do not install every suite preemptively;
- verify installed tools immediately;
- pin long-lived CI tooling when practical.

Project-mutating dependencies require the explicit `--modify-project` path.

Read [references/tool-acquisition.md](../references/tool-acquisition.md) before changing acquisition mechanisms.

### Phase 10 — Provision real isolated environment

For material integration/E2E evidence:

- use real dependency versions compatible with production;
- isolate test traffic/data from production;
- make reset deterministic;
- health-check dependencies before execution;
- prevent test workers from consuming production topics/leader elections;
- teardown reliably;
- keep test data synthetic or appropriately sanitized.

If the repository already has a reliable environment mechanism, use it rather than inventing another.

### Phase 11 — Execute and capture evidence

Run the smallest useful scenario first. On failure, preserve evidence and minimize/reproduce before expanding scope.

Generated/random tests must persist seed/schedule/corpus:

```bash
python3 "$SKILL_ROOT/scripts/seed_runner.py" --runs 500 --timeout 60 -- <test command>
```

Never report PASS for something only written, inspected, or planned.

### Phase 12 — Release gate and containment

For high/critical business changes:

- required release scenarios are explicit;
- automated scenarios must have bindings;
- required evidence must exist and PASS;
- manual scenarios must have procedures and recorded evidence;
- unresolved P0/P1 failures block release unless a named, time-bounded waiver exists;
- rollback/feature-flag/canary containment is documented where appropriate.

Pre-production tests are not omniscient. Prefer staged/canary rollout and automatic pause/rollback for suitable services.

### Phase 13 — Production detection and incident learning

For critical business rules ask:

```text
If our tests miss this violation, how will production detect it?
How quickly?
Can we contain it?
Can we reconcile/repair it?
```

Use runtime assertions, reconciliation, illegal-transition counters, duplicate-effect detection, audit logs, queue age/retry amplification, checksums/version markers and synthetic probes where appropriate.

After an incident:

```text
reproduce if possible
identify violated business rule/invariant
identify why existing evidence missed it
add durable prevention/detection/containment/recovery control
add regression scenario/corpus/seed when meaningful
verify the new control catches the known failure
```

A postmortem that only produces prose is incomplete when an engineering control is feasible.

