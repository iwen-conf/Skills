# Test Scope Matrix

Choose evidence according to affected business promises, failure consequences and real architecture. These are views of risk, not a prescribed pyramid or execution order. Start with [`test-case-design.md`](test-case-design.md) when scope or assertions are unclear; commands live in [`test-stacks.md`](test-stacks.md).

## Functional correctness: choose the credible boundary

| Boundary | Best use | What it does not establish alone |
|---|---|---|
| Unit / component | Pure rules, parsers, algorithms, decision tables and state transitions | Real database isolation, broker delivery, wiring or deployment behavior |
| Integration | Actual collaborating modules, database/driver, messaging or HTTP semantics | Unexercised cross-service paths or user-visible completion |
| Contract / API | Supported request/response and error semantics; producer/consumer compatibility | Actual runtime configuration, routing or external final effects when only stubbed |
| E2E / acceptance | Critical user journeys and cross-system effects in a representative environment | Every input/state combination, rare schedules or performance under real load |

Select the smallest boundary that can reveal the target failure, then add complementary evidence where it leaves a material gap. Critical integration may involve several real services. Avoid repeating all permutations at every layer; use focused tests for combinations and acceptance tests for the business chain.

There is no default E2E percentage. Do not replace a necessary real dependency with a mock merely to make a test smaller or faster. Document what substitutions hide.

## Evidence quality: claims, assertions and coverage

| Evidence | Use | Limit |
|---|---|---|
| Risk / contract coverage | Trace each required promise to cases, an oracle and a current run | An unexecuted mapping is a plan, not proof |
| Code / branch / condition coverage | Find code and decision paths that were not exercised | Execution does not establish meaningful inputs, correct assertions or effects |
| State / input / history coverage | Find missing transitions, data classes, schedules and interaction shapes | Finite samples do not establish all possible behavior |
| Regression and defect sensitivity | Preserve prior reproducers; check important assertions reject realistic errors | A passing previous version can itself contain a shared defect |

Do not set a universal code coverage target. Preserve justified repository thresholds and report their metric and measured scope. Native Go coverage, for example, does not report branch coverage. Name high-risk gaps rather than filling them with assertion-free calls. Smoke is a labeled subset of regression, with its reduced scope explicit.

Use bounded mutation or negative controls when uncertainty about important assertions justifies them. They need not wait until a suite is mature and do not require a mutation-score quota; distinguish equivalent changes, harness failures and performance-only differences.

## Robustness and exploration

| Technique | Useful when | Evidence to retain |
|---|---|---|
| Fuzz / generated inputs | Large input spaces, protocols, codecs, deserialization, hostile or malformed data | Input/corpus, seed and tool version, semantic assertions, minimized reproducer |
| Property / model / differential | Invariants, supported algebraic properties, stateful histories, alternative implementations | Property assumptions, independent model, reachable state/data distributions, counterexample |
| Exploratory testing | Unknown workflows, incomplete specifications, integration or usability uncertainty | Bounded charter, environment/data, observations, reproducers and unresolved questions |

Fuzzing can assert business properties as well as detect crashes; property tests can use generated data. Review correlated generators and oracle independence. Only assume round-trip, commutativity or idempotence when the actual contract supports them. Use controlled schedules for key races, with broader exploration as a complement.

## Performance and capacity

| Surface | Evidence needed when the risk applies |
|---|---|
| Algorithm / resource regression | Benchmark representative sizes, worst relevant distributions and CPU/allocation behavior; compare candidate and baseline under comparable conditions |
| Expected load | Offered and completed work, latency distribution including timeouts, error rate, resource/connection use, data size and cold/warm state |
| Overload / dependency slowdown | Backpressure, queue bounds, deadline/cancellation propagation, retry amplification and business correctness under degraded conditions |
| Recovery / long running | Backlog drain, per-task progress, memory/resource growth and service recovery after load or faults end |

A known hotspot or existing SLA is not the only trigger. A configuration/rule, data growth, scheduling or dependency change may create resource risk before users report slowness. Define targets from the project; missing budgets are gaps, not invented limits.

Keep planned workload arrival and timeout accounting explicit: if the load generator slows its requests whenever the service slows down, it can hide overload and tail latency. Include relevant skew, tenants and data distributions; a throughput number with wrong business results is still a failure.

Judge performance separately from functionality and from correctness under faults. A virtual-time simulation cannot establish real throughput. Run only authorized load/stress targets; reuse remote Docker/PostgreSQL or existing CI environments, without starting local servers by default.

Benchmark measures cost; profiling identifies where it comes from. Capture and inspect CPU/allocation/lock evidence before optimizing, then benchmark again. Native commands and platform-specific tracing routes are in [`test-stacks.md`](test-stacks.md).

## System lifecycle and specialist boundaries

| Surface | Applicable verification |
|---|---|
| Configuration / flags / runtime | Actual effective settings, defaults, supported combinations, disabled/enabled behavior and changes without a binary release |
| Compatibility / migration | Old/new data, clients/producers/consumers, mixed versions, interrupted migrations and the supported rollback floor |
| Fault / recovery / operations | Safety during failure, progress after recovery, backup restore with business queries, operable rollback/repair tools |
| Security | Business permission and forbidden-effect regression here; attack-surface audit and specialist scanning via `arc:audit` / `arc:security` |

For substantial invariants, real transaction/fault scenarios and release evidence, `arc:reliability` defines the claims and environment/evidence contract; execute their native bindings here. Normal/fault/recovery/upgrade are useful scope lenses, not mandatory phases for every change.

## Priority and execution cadence

Reuse the project's risk labels. If it uses P0–P2, assign those labels to consequences and urgency, never to unit/E2E/property/load categories. Explicitly mark the required claims for the current candidate; any required claim with failed or missing evidence blocks its gate, regardless of priority label.

| Decision | Principle |
|---|---|
| Required now | Changed critical contract, known regression, costly/irreversible side effect, or an explicit release requirement; any test type can qualify |
| Additional confidence | Representative supported variants and related paths selected by exposure and known weaknesses |
| Deferred / out of scope | State why, what existing evidence helps, what remains unverified and what would bring it back into scope |

| Cadence | Suitable evidence |
|---|---|
| PR / local | Fast relevant cases plus required real-boundary checks; run the highest-risk uncertainty early |
| Mainline / scheduled | Wider supported combinations, generated histories, fuzz, soak and fault exploration justified by the product |
| Candidate | Current required evidence, actual release configuration, applicable mixed-version/migration/recovery and critical journeys |
| Production observation | Authorized staged release and business/SLI monitoring; supplement prior evidence, never replace it |

Dependency graphs and runtime traces can help select tests, but historical traces can omit new paths, dynamic configuration and unobserved callers. Explain selection limits, include shared contracts/invariants and use a broader relevant regression set when selection evidence is incomplete.

Preserve all attempts during controlled flaky diagnosis. Candidate/control comparisons can help identify environmental noise, but neither both passing nor both failing settles the business contract. Quarantine needs ownership, repair criteria and explicit lost coverage. A quarantined required claim remains `FLAKY` or `NOT_RUN` until credible replacement/fixed evidence is available; authorized risk acceptance remains separate from PASS.
