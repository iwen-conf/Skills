# Industrial verification operating model

The goal is not a higher quality score. The goal is to solve real development risk with repeatable evidence.

## 1. Separate specification from execution

Maintain four linked artifacts, each with a distinct job:

1. **Business verification plan** — what the business must do and must never do.
2. **Execution bindings** — how a scenario is exercised in this repository/environment (HTTP, gRPC, SQL, command, job, device harness, etc.).
3. **Environment contract** — how an isolated, production-representative test environment is provisioned/reset/health-checked/torn down.
4. **Evidence** — what actually ran, with command/tool/version/seed/schedule/status/output sufficient to reproduce a failure.

Do not collapse these into a single giant test document. Business rules should survive refactors; execution bindings are expected to change with implementation.

## 2. Use explicit applicability, not bureaucratic completeness

A plan declares whether concerns such as state, permissions, concurrency, retries, external side effects, time boundaries, compatibility, migration, async delivery, destructiveness, and long-running behavior apply.

When a concern applies, corresponding model/scenario/evidence is required. When it does not apply, no empty section is required.

This avoids two common failures:

- accidental omission because nobody asked the question;
- cargo-cult testing because a template required irrelevant sections.

## 3. Outcome is the oracle

Transport success is not business success.

Weak:

```text
HTTP 200
```

Strong:

```text
HTTP result matches contract
AND durable state is correct
AND aggregate/counter invariants hold
AND external side effects are exactly-once/at-least-once according to contract
AND audit/event state is correct
AND forbidden side effects did not occur
```

For important rules define typed oracles: database, API, event, model/reference implementation, metric/reconciliation, file/device state, or a deliberately independent observer.

## 4. Test real boundaries when semantics live there

If correctness depends on a database transaction, broker redelivery, filesystem durability, network timeout, or real protocol behavior, an in-memory mock is not sufficient evidence for that property.

Use the project's real dependency in a disposable sandbox whenever practical. Mocks/fakes remain useful for deterministic fault injection and unrelated dependencies.

## 5. Reuse project infrastructure first

Industrialization does not mean introducing a new framework into every repository.

Prefer, in order:

1. existing test runner and fixtures;
2. existing docker-compose/Testcontainers/dev stack;
3. existing API/gRPC clients and test helpers;
4. existing CI artifacts and observability;
5. only then add missing tooling justified by the failure model.

The skill should adapt to the repository, not force the repository to adapt to the skill.

## 6. Release gates are claim-based

A release gate should answer concrete questions:

```text
Which business scenarios are required for this change?
Did the required automated scenarios actually run?
Did required manual checks produce recorded evidence?
Are critical invariants protected by credible evidence?
Are any known failures waived, and by whom/for how long?
```

Do not use a single aggregate "quality score" to hide missing P0/P1 evidence.

## 7. Change scope drives execution scope

For every change identify:

- business rules changed;
- states/actors newly reachable;
- shared entities touched;
- side effects moved or added;
- data/schema/protocol changes;
- historical incidents/regressions nearby.

Run the smallest suite that covers the changed claims on the developer/PR path, then broader compatibility/concurrency/fault/load suites on nightly/pre-release cadence.

## 8. Business concurrency is a first-class test target

Build a catalog of business operations and their shared mutable state. Test:

- same operation with duplicate/mirrored parameters;
- different operations touching the same entities;
- different lock/entity acquisition orders;
- retry after ambiguous result;
- cancellation/timeout racing with completion;
- async callback racing with user/operator action;
- out-of-order and repeated events;
- resource contention and long transaction tails.

For databases, observe real lock/wait/transaction behavior; do not infer it solely from application mocks.

## 9. Evidence statuses are factual

Use only:

```text
PASS       ran and satisfied the oracle
FAIL       ran and violated the oracle/contract
BLOCKED    could not run because a named prerequisite is unavailable
NOT_RUN    deliberately not executed in this cycle
FLAKY      non-deterministic result requiring investigation
```

"Written", "looks correct", and "should pass" are not PASS.

## 10. The verification system is production software

Test the harness, runner, environment isolation, observers and gates themselves. Keep unit tests hermetic; put real Docker/database/toolchain checks in explicit integration self-tests.

A reliability system that flakes because Docker is unavailable or silently loses observer data cannot be trusted as a release gate.

## 11. Sources reflected in this operating model

- Google Testing Blog: risk/cost/benefit-driven test planning; testability designed in; testable user journeys.
- Uber Engineering BITS: isolated, production-like backend sandboxes and pre-submit E2E workflows across services.
- AWS Builders' Library: explicit timeout/retry/idempotency contracts and avoiding retry amplification.
- Jepsen: real-system histories checked against explicit correctness models under failure.
- PostgreSQL documentation: real lock/deadlock/isolation behavior and transaction retry semantics.
