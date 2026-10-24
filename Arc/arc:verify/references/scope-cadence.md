# Scope and Cadence

Which evidence a change needs, at which boundary, and when it runs. Case fields and oracles:
[case-design.md](case-design.md). Tiers: [contract §2](../../_shared/contract.md).

## 1. Change type → minimum evidence

Rows combine; take the union for a change that spans several types.

| Change type | Minimum (T1) | Add for T2 or required claims |
|---|---|---|
| Pure logic, parser, calculation | unit/table tests on both boundary sides; decision table for multi-condition rules | property or differential test against an independent model |
| Public API, DTO, error codes | server contract test of request/response/error semantics; each client consumer found by code-intel row 5 | old-client compatibility; [api-contract](../../_shared/api-contract.md) doc and clients in sync |
| Database schema, migration, backfill | migration on representative old data in the project's test database; post-migration invariant queries | db mode |
| Query, index, plan-sensitive SQL | result-correctness test on named parameters | db mode (plans, statistics, parameter sensitivity) |
| Auth, permission, tenant | allow and deny per role; zero durable change after deny; tenant boundary | stale/revoked grants, cache vs source equivalence, batch vs single equivalence, independent reference model for optimized evaluators |
| Money, inventory, quota | conservation invariant; exact amounts | concurrency + idempotency + recovery per [txn-faults.md](txn-faults.md) |
| Transaction, lock, concurrency | real-database parallel test through a barrier | contention matrix, retry ownership, mixed workload |
| Async, messaging, webhooks, jobs | duplicate, reorder, delayed and redelivered messages with a final-state oracle | crash-before-ack, poison message, handler restart, reconciliation |
| Config, flag, runtime setting | enabled and disabled behavior with the effective config | supported combinations; change without a binary release |
| Dependency or toolchain upgrade | existing regression suite on the new version | behavior diff of the touched API; benchmark if on a hot path |
| UI | the four UI checks in [stacks.md](stacks.md) §UI checks | E2E journey at target viewports |
| Performance-sensitive path | benchmark vs baseline, separate verdict | expected-load and overload runs with recovery |
| Embedded, device, protocol | invalid/truncated frames, CRC, byte order, stale reading, disconnect/reconnect | persistence/restart, long-run resources, hardware-in-loop when physical behavior matters |
| Docs or comments only | none; verdict `NOT_RUN: no behavior change` | none |

T0: run the existing tests that cover the touched behavior; add one only when none exists.

## 2. Credible boundary

| Boundary | Best use | Does not establish alone |
|---|---|---|
| Unit / component | pure rules, parsers, algorithms, decision tables, state transitions | DB isolation, broker delivery, wiring, deployment behavior |
| Integration | real collaborating modules, DB/driver, messaging, HTTP semantics | unexercised cross-service paths, user-visible completion |
| Contract / API | request/response/error semantics, producer/consumer compatibility | runtime config, routing, external final effects when stubbed |
| E2E / acceptance | critical journeys and cross-system effects in a representative environment | every input combination, rare schedules, performance under load |

- Pick the smallest boundary that can reveal the target failure; add complementary evidence only
  for a material gap. Critical integration may span several real services.
- Test combinations in focused tests and the business chain in acceptance tests; never repeat
  every permutation at every layer. No default E2E percentage.
- Never swap a needed real dependency for a mock to make a test faster; write down what each
  substitution hides.

## 3. Evidence quality and coverage

| Evidence | Use | Limit |
|---|---|---|
| Claim coverage | each required promise → case → oracle → current run | an unexecuted mapping is a plan |
| Code/branch coverage | find unexercised paths | execution ≠ meaningful input or correct assertion |
| State/input/history coverage | find missing transitions, data classes, schedules | finite samples |
| Regression + sensitivity | keep prior reproducers; prove assertions reject realistic errors | a passing old version can share the defect |

- No universal coverage target. Keep the repository's thresholds; report the metric and measured
  scope actually produced (Go's native coverage is statement coverage, not branch coverage).
- Name high-risk gaps; never fill them with assertion-free calls.
- Regression is the CI default; smoke is a labeled subset with its reduced scope stated.
- Bounded mutation or negative controls when doubt about an important assertion justifies them;
  no mutation-score quota.
- Coverage instrumentation, sanitizers, simulated time and substitute dependencies change runtime
  behavior; release claims need a run on the delivered build and config.

## 4. Performance and capacity

| Surface | Evidence when the risk applies |
|---|---|
| Algorithm/resource regression | representative sizes and worst distributions, CPU/allocations; candidate vs baseline under comparable conditions, ≥10 samples (`-count=10` + `benchstat`) or ≥3 load runs |
| Expected load | offered vs completed work, latency distribution including timeouts, error rate, connections/resources, data size, cold/warm state |
| Overload, slow dependency | backpressure, queue bounds, deadline/cancel propagation, retry amplification, correctness while degraded |
| Recovery, long-running | backlog drain, per-task progress, memory/resource growth, recovery after load or faults end |

- Triggers include config/rule changes, data growth, scheduling and dependency changes, not only
  known hotspots or an existing SLA.
- Targets come from the project; a missing budget is a gap, not an invented limit.
- Use an open (arrival-rate) workload so a slowing service cannot slow the generator and hide
  overload and tail latency; count timeouts in latency.
- Include skew, tenant size and real data distributions. A virtual-time simulation never
  establishes real throughput.
- Judge performance apart from functional correctness and fault correctness. A benchmark measures
  cost; a profile shows where it comes from; optimizing is `arc:code` work.

## 5. Lifecycle surfaces beyond the binary

| Surface | Verify |
|---|---|
| Config / flags / runtime | effective settings, defaults, supported combinations, enable/disable, change without release |
| Compatibility / migration | old/new data, clients, producers/consumers, mixed versions, interrupted migration, supported rollback floor |
| Fault / recovery / operations | safety during failure, progress after recovery, restore checked with business queries, rollback/repair tools actually run |
| Security | permission and forbidden-effect regressions here; attack-surface review and scanning → `arc:audit` |

## 6. Priority

Reuse the project's risk labels; attach them to consequence and urgency, never to test types.

| Decision | Rule |
|---|---|
| Required now | changed critical contract, known regression, costly or irreversible side effect, explicit release requirement; any test type qualifies |
| Additional confidence | representative variants and related paths chosen by exposure and known weakness |
| Deferred / out of scope | why, which existing evidence helps, what stays unverified, what brings it back |

Any required claim with failing or missing evidence blocks its gate whatever its label. Cost alone
never defers a required claim. Run the required case with the highest uncertainty first.

## 7. Cadence

| Cadence | Runs |
|---|---|
| Local / PR | changed claims: focused unit/property/component tests, required real-dependency integration, static and race checks for the change, required release-scenario subset |
| Mainline / nightly | cross-business concurrency matrix, fuzz campaigns, mutation sampling, wider compatibility matrix, fault injection, load/stress/soak slices, migration/recovery suites |
| Pre-release candidate | the full required set on the release config: upgrade/rollback/recovery, mixed versions, critical E2E journeys |
| Production | authorized staged rollout, SLI and invariant monitoring, reconciliation, synthetic probes; supplements earlier evidence, never replaces it |

- Never put every expensive suite on PR. Stop when the agreed evidence exists; repeat only for a
  new change, a failure or an open risk.
- Test selection by dependency graph or historical traces misses new paths, dynamic config and
  unobserved callers. State the selection limits, always include shared contracts and invariants,
  and run the broader related regression set when selection evidence is incomplete.

## 8. Change verification card

Merge into the PR description, test notes or task card; do not create a separate document.
Delete rows that do not apply; write `gap` for unknowns. Link stable rules instead of copying them.

```text
Change / candidate / target env / allowed operations:
Rules touched (source) · callers and shared state · applicable concerns:
Consequence · exposure · detection/recovery basis · project priority:
EXPECTED · FORBIDDEN · SPECIFICATION GAPs:
Phases in scope; deferred items with reason and residual risk:
Rollout / rollback / containment controls that apply:
```

| Case / rule / required | Conditions · interleaving · fault | EXPECTED / FORBIDDEN | Oracle | Defect caught · sensitivity check | Selector · real boundary |
|---|---|---|---|---|---|

| Case | Candidate · config · env | Run · tool · seed/schedule | Observation · artifact | Verdict · gap |
|---|---|---|---|---|

The first table holds specification and binding; the second holds each actual run. No verdict is
written before an observation exists. Record differences between the tested and delivered
configuration and their effect on each claim.
