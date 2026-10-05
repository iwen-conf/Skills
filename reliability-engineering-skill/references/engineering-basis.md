# Engineering basis for this skill

This file records the public engineering practices that informed the skill. It is not a claim that any one organization has zero incidents; the useful lesson is the method.

## SQLite — diversity of test harnesses and aggressive fault testing

Source: https://www.sqlite.org/testing.html

Imported ideas:
- multiple independently developed harnesses rather than one monoculture;
- branch/MC/DC coverage for an unusually critical library, but coverage is not sufficient;
- OOM, I/O error, crash/power-loss, malformed DB, fuzz, boundary and regression testing;
- mutation testing to verify branches actually matter and tests can detect semantic changes;
- release checklists.

Adaptation here: use independent evidence for P0 claims and mutation/harness self-tests selectively; do not copy SQLite's extreme coverage target blindly into ordinary applications.

## FoundationDB — deterministic simulation

Source: https://apple.github.io/foundationdb/testing.html
Source: https://apple.github.io/foundationdb/client-testing.html

Imported ideas:
- simulate the entire distributed system with controlled time/failures;
- deterministic seeds make failures replayable;
- combine simulation correctness, real performance testing and hardware failure testing;
- generate many component failures while continuously checking invariants.

Adaptation here: recommend deterministic simulation only when state/interleaving complexity justifies architectural support for it.

## TigerBeetle — safety/liveness invariants, inside-out simulation, subsystem fuzzing

Sources:
- https://tigerbeetle.com/blog/2026-08-20-protocol-aware-dst/
- https://tigerbeetle.com/blog/2025-06-06-fuzzer-blind-spots-meet-jepsen/
- https://tigerbeetle.com/blog/2025-04-23-swarm-testing-data-structures/
- https://tigerbeetle.com/blog/2025-11-28-tale-of-four-fuzzers/
- https://tigerbeetle.com/blog/2023-07-06-simulation-testing-for-liveness/

Imported ideas:
- separate safety from liveness;
- check deep internal invariants as well as public API behavior;
- deterministic simulation of network/storage/process faults;
- targeted subsystem fuzzers in addition to whole-system fuzzing;
- reference models and exact oracles;
- audit input distributions for blind spots; use swarm/weighted generation;
- design minimal interfaces with fuzzability/testability in mind.

## Jepsen — independent black-box generative consistency testing

Sources:
- https://jepsen.io/analyses
- https://jepsen.io/analyses/tigerbeetle-0.16.11

Imported ideas:
- run real binaries on real clusters;
- inject distributed failures and collect concurrent histories;
- compare histories to an independent model;
- use an external/independent methodology to expose blind spots in internal testing;
- passing experiments find no violation; they do not prove absence of bugs.

## Microsoft Coyote — systematic concurrency testing

Sources:
- https://microsoft.github.io/coyote/
- https://microsoft.github.io/coyote/concepts/concurrency-unit-testing/

Imported ideas:
- control scheduling/nondeterminism rather than relying only on stress timing;
- explore different interleavings systematically;
- save and replay traces so concurrency bugs are reproducible.

Adaptation here: use ecosystem-appropriate equivalents (e.g. Loom for small Rust concurrency models) rather than requiring Coyote outside .NET.

## AWS — formal methods, idempotency, and automated reasoning

Sources:
- https://www.amazon.science/publications/how-amazon-web-services-uses-formal-methods
- https://aws.amazon.com/blogs/storage/how-automated-reasoning-helps-us-innovate-at-s3-scale/
- https://aws.amazon.com/builders-library/making-retries-safe-with-idempotent-APIs/

Imported ideas:
- model checking can find subtle distributed-design bugs that reviews and conventional testing miss;
- keep specifications close enough to engineering workflows that they evolve with implementations;
- retries require explicit idempotency semantics and request identity;
- ambiguous outcomes are a first-class distributed-systems case.

## Google SRE — reliability is risk management plus safe release/learning loops

Sources:
- https://sre.google/sre-book/testing-reliability/
- https://sre.google/workbook/canarying-releases/
- https://sre.google/sre-book/postmortem-culture/
- https://sre.google/sre-book/embracing-risk/

Imported ideas:
- reliability confidence includes tests at different levels plus production evidence;
- staged/canary releases reduce blast radius and enable rollback;
- SLO/error budgets guide availability/release tradeoffs;
- postmortems should produce preventive actions and organizational learning;
- operational recovery requires rehearsal.

Adaptation here: error budgets apply to service reliability tradeoffs, not permission to corrupt money/data/security invariants.

## Netflix — controlled resilience experiments

Source: https://medium.com/netflix-techblog/chaos-engineering-upgraded-878d341f15fa
Source: https://www.usenix.org/conference/srecon17americas/program/presentation/rosenthal

Imported ideas:
- chaos engineering is hypothesis-driven experimentation, not random breakage;
- define steady-state behavior, vary realistic events, observe system-level outcomes;
- production realism can reveal emergent behavior that isolated tests miss.

Adaptation here: production chaos is never the default. Require explicit authorization, bounded blast radius, abort criteria, observability, and rollback/stop controls.

## Stripe — robust retry semantics

Sources:
- https://stripe.com/blog/idempotency
- https://docs.stripe.com/api/idempotent_requests

Imported ideas:
- idempotency keys make retry behavior explicit across ambiguous network outcomes;
- the same request identity should preserve semantic equivalence;
- retry/backoff/jitter and server-side identity handling must be designed together.

## Uber — shift realistic business integration testing left

Sources:
- https://www.uber.com/us/en/blog/shifting-e2e-testing-left/
- https://www.uber.com/us/en/blog/accounting-data-testing-strategies/

Imported ideas:
- outage reviews found that basic, heavily mocked unit tests left meaningful business regressions uncovered;
- create isolated, production-shaped sandboxes so real cross-service/E2E flows can gate changes before merge;
- treat business accounting correctness as transaction-level and aggregate-level validation, not only service-local function tests;
- data/test isolation is part of the testing architecture, not an afterthought.

Adaptation here: transaction/concurrency verification prefers the real application path and real database. Deterministic test hooks and disposable environments are engineering requirements when needed to make business races reproducible.

## PostgreSQL / AWS database practice — contention and deadlocks are testable workload properties

Sources:
- https://www.postgresql.org/docs/current/explicit-locking.html
- https://www.postgresql.org/docs/current/transaction-iso.html
- https://www.postgresql.org/docs/current/pgbench.html
- https://aws.amazon.com/blogs/database/troubleshooting-row-lock-contention-in-amazon-aurora-postgresql-part-1-understanding-row-lock-contention-in-postgresql/
- https://aws.amazon.com/blogs/database/deep-dive-into-amazon-aurora-postgresql-lock-analysis-with-cloudwatch-database-insights/

Imported ideas:
- consistent acquisition order is a primary deadlock defense;
- applications must handle transaction abort classes such as serialization failures and, where appropriate, deadlocks;
- lock contention is observable through wait events/blocking relationships and can be reproduced with deliberate high-contention workloads;
- custom concurrent transaction workloads belong in pre-production verification, not only post-incident diagnostics.

Adaptation here: maintain a business transaction catalog, self-conflict and cross-business conflict matrices, deterministic interleaving tests, wait-for graph observation, and mixed-workload gates.

## Business-first test planning additions (v2.3)

### Google Testing Blog — The Inquiry Method for Test Planning

Absorbed principles:
- test planning is risk/cost/benefit driven rather than a fixed universal template;
- testability belongs in product/design work, not only after implementation;
- stable strategy can coexist with smaller change/feature-specific plans;
- important plans must be maintained as the product changes.

### Google Test Engineering — testable user journeys

Absorbed principles:
- test engineers connect product, development, UX, release, beta, and user perspectives;
- business/user journeys are explicit testing objects, not accidental consequences of lower-level tests.

### Uber BITS — Shifting E2E Testing Left

Absorbed principles:
- microservice-local correctness is insufficient for business logic spanning many services;
- isolated production-like sandboxes allow real E2E business flows before merge/deploy;
- testing configuration changes and full user journeys can be part of pre-submit gating;
- production probing is a containment/detection layer, not a substitute for pre-production business validation.

## v3 industrialization — executable business verification

Additional public practices used to refine v3:

### Google Testing Blog — planning by risk, cost, benefit and testable journeys

Sources:
- https://testing.googleblog.com/2016/06/the-inquiry-method-for-test-planning.html
- https://testing.googleblog.com/2016/09/what-test-engineers-do-at-google.html

Applied here:
- test plans select relevant content rather than enforcing a universal checklist;
- testability is designed alongside the feature;
- business/user journeys are first-class verification targets;
- verification depth follows consequence and expected value, not a quota.

### Uber BITS — isolated real backend flows before merge

Source:
- https://www.uber.com/us/en/blog/shifting-e2e-testing-left/

Applied here:
- separate business specification from repository execution bindings;
- prefer isolated, production-shaped sandboxes for cross-service flows;
- treat traffic/data isolation, provisioning, reset and teardown as test architecture;
- allow required business workflows to gate changes before production.

### AWS Builders' Library — retry/idempotency contracts

Source:
- https://aws.amazon.com/builders-library/making-retries-safe-with-idempotent-APIs/

Applied here:
- ambiguous outcomes and repeat requests are explicit business scenarios;
- idempotency is a business contract that must be observed through final effects, not merely handler return codes.

### Jepsen — histories checked against a model

Sources:
- https://jepsen.io/analyses
- https://jepsen.io/consistency/models

Applied here:
- real executions produce histories/evidence;
- correctness is decided by explicit business/system models and oracles;
- real implementation behavior under concurrency/failure matters more than mocked control-flow success.
