---
name: industrial-business-reliability-engineer
description: >
  Build and execute business-first verification for real software systems. Use when designing, reviewing,
  testing, hardening, releasing, or investigating application/business logic, transactions, distributed workflows,
  databases, APIs, jobs, messaging, permissions, embedded/device behavior, migrations, and reliability-sensitive code.
  The skill turns business expectations into explicit rules, scenarios, executable bindings, real-dependency tests,
  reproducible evidence, and release gates. It can bootstrap missing verification tooling safely, but tools are chosen
  only after the business risk and correctness contract are understood.
---

# Industrial Business Reliability Engineer

## 0. Mission

Solve real development problems before they become production incidents.

Do **not** optimize for:

- number of test cases;
- line coverage percentage;
- mutation score;
- test-plan completeness for its own sake;
- a synthetic quality/reliability score;
- installing the largest possible testing toolchain.

Optimize for:

```text
business behavior is explicitly defined
  -> dangerous failure modes are identified
  -> important expectations have trustworthy oracles
  -> scenarios exercise real boundaries where semantics live
  -> failures are reproducible
  -> required evidence actually runs
  -> releases are blocked when required evidence is missing/failing
  -> escaped defects become permanent engineering controls
```

The standard is not “looks well tested.” The standard is **evidence that addresses the actual failure modes of this system**.

## 1. Core operating principles

1. **Business first.** Understand what users/business/operators expect before choosing test techniques.
2. **Outcome over transport success.** `HTTP 200`, `rows_affected=1`, or “message published” are not sufficient business oracles unless the contract says they are the final outcome.
3. **Specification and execution are separate.** Business rules should remain stable when implementation changes; repository-specific test bindings may change freely.
4. **Applicability is explicit.** Declare whether state, permissions, concurrency, retries, side effects, time, compatibility, migration, async behavior, destructive behavior, or long-running effects apply. Do not require irrelevant ceremony.
5. **Real semantics require real boundaries.** If correctness depends on PostgreSQL isolation/locks, broker redelivery, filesystem durability, network timeout, protocol behavior, or hardware, include evidence against the real implementation or a faithful harness.
6. **Reuse the repository first.** Prefer existing runners, fixtures, Docker Compose/Testcontainers, test clients, CI and observability before introducing new frameworks.
7. **Tools follow failure models.** Do not install fuzzers, mutation tools, model checkers or chaos tooling merely because they exist.
8. **Tests need oracles.** Randomness without a correctness predicate is activity, not verification.
9. **Verification must be reproducible.** Persist seeds, schedules, corpora, exact tool versions and failure artifacts when relevant.
10. **The test system itself must be trustworthy.** Keep its unit tests hermetic; run real Docker/database/toolchain checks as explicit integration self-tests.
11. **Missing evidence is not PASS.** Use factual states: `PASS`, `FAIL`, `BLOCKED`, `NOT_RUN`, `FLAKY`.
12. **No “production-only” excuse without attempting testability.** Transaction, deadlock, retry, duplicate, cross-business race, timeout, partial failure and recovery behavior are test targets. If blocked, identify the exact missing prerequisite.

Read `references/industrial-operating-model.md` before large reliability/test-program work.

## 2. Standard work products

Use four linked artifacts. Do not merge them into one giant document.

### A. Business verification plan

Describes **what must be true**:

```text
objective/scope
actors and authority
business rules
state model when applicable
side-effect semantics when applicable
primary journeys
expected outcomes
forbidden outcomes
exception/retry/cancellation semantics
cross-business interactions
compatibility/time/migration dimensions when applicable
business oracles
release-required scenarios
```

Use `templates/business-test-plan.md` and, for machine checking, a TOML plan based on `examples/business-test-plan.toml`.

Validate:

```bash
python3 scripts/business_test_plan.py validate --plan business-test-plan.toml
```

Render for review:

```bash
python3 scripts/business_test_plan.py render \
  --plan business-test-plan.toml \
  --out business-test-plan.md
```

### B. Execution bindings

Describes **how this repository exercises a business scenario** without contaminating business rules with implementation detail.

Examples:

```text
HTTP/gRPC request
background job invocation
real SQL assertion
message/event observation
device command/harness
repository test binary
```

Use `examples/business-scenario-bindings.toml` as a pattern.

Validate:

```bash
python3 scripts/business_scenario_runner.py validate \
  --plan business-test-plan.toml \
  --binding business-scenario-bindings.toml
```

Run a scenario:

```bash
python3 scripts/business_scenario_runner.py run \
  --plan business-test-plan.toml \
  --binding business-scenario-bindings.toml \
  --scenario S-002 \
  --evidence-out .reliability/evidence/S-002.json
```

### C. Environment contract

Describes how tests run in an isolated, production-representative environment:

```text
bootstrap
reset
healthcheck
teardown
traffic/data isolation
required environment variables
```

Prefer the project's existing Compose/Testcontainers/dev-stack. The skill is an adapter, not a parallel platform.

Use `examples/sandbox.toml` as a pattern.

Example:

```bash
python3 scripts/sandbox_runner.py \
  --contract sandbox.toml \
  --root . \
  --evidence-out .reliability/evidence/sandbox.json \
  -- python3 <skill>/scripts/business_scenario_runner.py run ...
```

The runner never invokes a shell and always attempts teardown.

### D. Evidence

Evidence is produced by actual execution, not by plan existence.

Record, as applicable:

```text
scenario/claim ID
tool and version
exact command
seed/schedule/corpus
status
elapsed time
observed state/results
failure output
reproduction instructions
```

Release-required business scenarios must be traceable to evidence or an explicit manual/analysis procedure.

Cross-check plan -> binding -> evidence:

```bash
python3 scripts/business_verification_gate.py \
  --plan business-test-plan.toml \
  --binding business-scenario-bindings.toml \
  --evidence-dir .reliability/evidence \
  --require-evidence
```

This gate produces no aggregate quality score. A missing required P0/P1 scenario cannot be hidden by many passing low-risk tests.

## 2.5 Project adoption and per-change planning

Do not make every feature author rewrite the full domain test plan. Keep stable business rules/journeys in durable project artifacts, and create a small change verification plan for each material change.

Recommended persistent layout is documented in `references/project-adoption.md`. Use `templates/change-verification-plan.md` for a PR/feature/release when useful.

The per-change plan should answer only:

```text
which business rules changed
which existing journeys/states/side effects can observe the change
which scenarios are required for this change
which extra probes are new
what evidence blocks release
what rollout/rollback controls apply
```

This keeps the system maintainable: stable domain knowledge is reused, while change-specific work remains small.

## 3. Workflow for real project work

Perform these phases in order. Skip only when irrelevant and record why.

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
python3 scripts/business_txn_lab.py analyze \
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
python3 scripts/verification_toolchain.py doctor --root .
python3 scripts/verification_toolchain.py plan --root . --capability <capability>
python3 scripts/verification_toolchain.py install --root . --capability <capability> --apply
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

Read `references/tool-acquisition.md` before changing acquisition mechanisms.

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
python3 scripts/seed_runner.py --runs 500 --timeout 60 -- <test command>
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
python3 scripts/business_txn_lab.py pg-run \
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

## 7. Required output when reviewing or hardening a project

Return useful engineering artifacts, not a scorecard.

At minimum report:

### Business model
- changed capabilities/rules;
- applicable concerns;
- expected and forbidden outcomes;
- specification gaps.

### Highest-risk failure paths
- what can go wrong;
- business consequence;
- where in architecture it can occur.

### Verification plan
- concrete scenarios;
- oracle for each important scenario;
- chosen evidence layer and why;
- real dependencies/environment required.

### Implementation
- tests/harnesses/bindings added or changed;
- tools installed and repository changes;
- testability seams introduced.

### Execution evidence
- exactly what ran;
- PASS/FAIL/BLOCKED/NOT_RUN/FLAKY;
- failure reproduction data.

### Release decision inputs
- required evidence still missing;
- unresolved P0/P1 defects;
- rollback/containment readiness;
- residual risk stated plainly.

Do not emit an overall “85/100 reliability” number unless a user explicitly asks for a scoring exercise. Scores are not release evidence.

## 8. Anti-patterns

Reject or correct these patterns:

```text
write tests directly from function names without business context
assert only HTTP status and call it business verification
mock the database while claiming transaction/isolation safety
mock a broker while claiming redelivery semantics are safe
use sleeps to hope for a race instead of deterministic synchronization
run random tests without an oracle
install every verification tool regardless of need
count coverage as reliability
retry CI until green
ignore flaky tests indefinitely
silently weaken assertions to make CI pass
claim a test passed when it was not executed
call a documented scenario "automated" with no executable binding
allow a release-required scenario to have no evidence
use a quality score to average away a missing critical invariant
say "can only test in production" before attempting a sandbox/failpoint/real dependency harness
run destructive production chaos without explicit authorization and containment
```

## 9. Stop rule

Testing is not complete in the mathematical sense. Stop/escalate based on evidence and residual risk.

A change is reasonably verified when:

- business behavior and applicable dimensions are explicit;
- critical rules have trustworthy oracles;
- high-risk scenarios are bound to credible evidence;
- real dependency semantics are tested where they matter;
- required scenarios actually ran and passed;
- failures are reproducible or preserved;
- release containment exists where escaped defects remain plausible;
- remaining uncertainty is named rather than hidden.

Do not keep generating low-value cases merely to increase counts.

## 10. Final principle

A professional verification system should answer:

```text
What does the business promise?
What must never happen?
Which real interactions can violate that promise?
How do we force those interactions before production?
How do we independently know the final outcome is correct?
What evidence actually ran for this release?
If we still miss it, how will we detect and contain it in production?
```

If the skill cannot answer those questions with executable evidence, it is not finished.
