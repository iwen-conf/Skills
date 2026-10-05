# Business-first test planning

## Purpose

Technical tests are subordinate to business correctness. Before choosing unit, integration, fuzz, concurrency, load, or chaos techniques, define what the business capability is supposed to accomplish and what outcomes are unacceptable.

The core artifact is a living business test plan kept close to the design and implementation. It should be revised when business rules, states, actors, side effects, integrations, or acceptance criteria change.

## Required model

For each feature, workflow, or materially changed business capability, identify:

1. **Objective and scope** — what user/business outcome the capability exists to deliver; what is explicitly out of scope.
2. **Actors and authority** — users, operators, services, jobs, devices, external systems, and role/tenant distinctions.
3. **Business rules** — eligibility, calculations, limits, precedence, timing, approval, policy, uniqueness, ownership, visibility, and exception rules.
4. **Preconditions and test data** — state required before the action and representative data classes.
5. **Trigger and journey** — how the workflow starts and the meaningful end-to-end steps.
6. **State model** — valid states, valid transitions, forbidden transitions, terminal states, reopen/retry semantics.
7. **Expected outcomes** — response plus durable state, emitted events, external side effects, notifications, audit trail, counters, and follow-up work.
8. **Forbidden outcomes** — duplicate effect, lost effect, unauthorized visibility, impossible state, stale state, wrong recipient, wrong amount/quantity, orphan data, premature side effect, etc.
9. **Exception semantics** — validation failure, dependency failure, timeout, retry, cancellation, partial progress, recovery, compensation.
10. **Cross-business interactions** — other workflows that touch the same entities, resources, quotas, state, schedules, or side effects.
11. **Compatibility dimensions** — old/new clients, schema versions, feature flags, tenant policies, channels, locales, time zones, device versions where relevant.
12. **Observability** — how the expected result and important violations can be observed in test and production.
13. **Acceptance / exit criteria** — what evidence is required before the change can ship.

## Business oracle

A business scenario is incomplete unless it defines how correctness is decided.

Weak oracle:

```text
request returned 200
```

Stronger oracle:

```text
request returned the documented result
AND booking state is Cancelled
AND capacity is released exactly once
AND the next eligible waitlisted user is promoted according to policy
AND one cancellation event exists
AND refund state follows the payment rule
AND audit history identifies actor and reason
AND retrying the same command creates no extra business effect
```

Expected outcomes should be observable independently of the implementation path when practical.

## Scenario design methods

Use the methods that match the business model rather than a fixed checklist.

- **Use-case / journey scenarios** for real user/business flows.
- **Equivalence partitioning** for input classes that should behave alike.
- **Boundary analysis** for limits and thresholds.
- **Decision tables** when multiple business conditions choose an outcome.
- **State-transition testing** for workflow/status logic.
- **Pairwise/combinatorial selection** for large configuration/role/channel matrices, with targeted higher-order combinations for dangerous interactions.
- **Property/model-based testing** when rules can be stated as invariants over many generated cases.
- **Exploratory charters** for poorly specified or novel behavior; findings must feed back into explicit rules/tests when important.
- **Cross-business composition** for workflows sharing entities, quotas, mutable state, locks, events, or external side effects.

Do not create the full Cartesian product blindly. Use risk, historical incidents, frequency, and consequence to prioritize combinations.

## Change impact

For every change, ask:

```text
Which business rules changed?
Which actors or states are newly reachable?
Which existing journeys can observe the changed state?
Which side effects moved earlier/later?
Which other business operations touch the same state?
Which historical incidents or regressions are adjacent?
```

The answer determines the regression scope.

## Traceability

For important systems, maintain traceability:

```text
requirement/design decision
  -> business rule
  -> scenario/journey
  -> expected + forbidden outcome
  -> automated/manual evidence
  -> release criterion
```

A rule with no scenario is a coverage gap. A scenario with no rule/objective may be low-value or accidental. A critical expected outcome with no observable oracle is a specification/testability gap.

## Sources that informed this approach

- Google Testing Blog, "The Inquiry Method for Test Planning" — risk/cost/benefit-driven test planning, testability, and living plans.
- Google Testing Blog, "What Test Engineers do at Google" — testable user journeys and coordination across product, development, UX, release, beta, and users.
- Uber Engineering, "Shifting E2E Testing Left at Uber" — real end-to-end business flows across many services, isolated sandboxes, and pre-submit gating for backend changes.

## v3 industrial execution contract

For production-oriented projects, separate the business plan from repository-specific execution details.

```text
Business rule / journey / expected & forbidden outcome
  -> scenario ID
  -> typed oracle(s)
  -> execution binding
  -> isolated environment
  -> execution evidence
  -> release gate
```

### Explicit applicability

Use an `[applicability]` declaration for concerns such as state, permissions, concurrency, retries, external side effects, time boundaries, compatibility, migration, asynchronous delivery, destructive behavior, and long-running effects.

If a concern is applicable, require its corresponding model and scenario evidence. If it is not applicable, do not create empty ceremony merely to satisfy a template.

### Typed business oracles

For important behavior prefer explicit oracle records containing:

```text
kind       database | api | event | model | audit | metric | device | file | other
observe    what concrete state/result is inspected
assertion  the business truth that must hold
```

The oracle is a specification of truth, not merely the location of a test assertion.

### Execution modes

Every high/critical scenario should declare how it is verified:

```text
automated  -> requires an executable binding
manual     -> requires explicit procedure and recorded evidence
analysis   -> requires a named analysis/model artifact
```

Do not claim automation from the existence of a test-plan row.

### Environment isolation

High-risk integration/E2E verification should document traffic and data isolation. Prefer existing project mechanisms such as disposable databases, Docker Compose projects, Testcontainers, test tenants, isolated topics/queues, and sandbox endpoints.

### No aggregate release score

Release evidence is claim/scenario based. A missing critical scenario remains missing regardless of how many unrelated tests pass.
