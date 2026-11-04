# Production reconciliation and incident-regression standard

Pre-release testing is finite. Critical systems require an independent production path that detects invariant drift and turns escaped failures into durable controls.

## Reconciliation classes

Examples:

```text
ledger debits + credits + external movements conserve value
inventory/reservation/capacity never violates declared bounds
one idempotency key has at most one logical durable effect
old/new migration stores converge
outbox/inbox/application state converge according to delivery semantics
replica/CDC pipeline reaches declared freshness/convergence
state machine contains no illegal terminal transitions
```

Prefer an oracle sufficiently independent from the code path being checked. Reusing the same buggy function as both writer and checker creates correlated failure.

## Production response

A reconciliation mismatch should have:

- severity and escalation owner;
- containment action (pause rollout, feature flag, traffic cohort isolation, retry reduction);
- evidence capture before destructive repair;
- repair/replay/rebuild procedure;
- audit trail;
- retrospective conversion into deterministic test, corpus case, history counterexample, migration assertion or runtime guardrail.

## G30 evidence

At minimum document the invariant, observer, cadence, detection latency, tolerated mismatch, action threshold and repair/rollback path. “Dashboard exists” is not sufficient evidence.
