# Rollout, Detection and Incident Learning

Release-time and post-release controls. Gate order and verdict rules:
[release-ops.md](release-ops.md).

## 1. SLIs and detection

- Use the project's SLIs, SLOs, windows, exclusions and owners. Without them, propose a reviewable
  definition and list the missing data; never invent 99.9%, latency or recovery numbers.
- An SLI maps to users completing tasks; averages hide failing cohorts and tail latency. Async
  work also needs queue age, retry amplification, backlog and completion time.
- Error budgets govern availability trade-offs only; never "budget" double charges, data
  corruption or permission leaks.
- For each critical rule answer: if tests miss a violation, how is it observed, how fast, who
  acts, how is spread stopped, can it be reconciled or repaired? Pick signals with business
  meaning: illegal-transition counters, duplicate-effect detection, reconciliation diffs,
  audit/state versions, checksums/version markers, synthetic probes. A metric with no action does
  not become an alert.

## 2. Containment and recovery

- Before release confirm the stage scope, business metrics compared against baseline, stop
  conditions and a recovery path that actually works.
- Application rollback does not undo a database migration or external side effects; state when
  forward-fix, compensation, manual repair or backup restore applies.
- Recovery has an RTO and an RPO from the project. A backup that was created is not proof of
  restore: restore in an authorized isolated environment, run business queries and
  reconciliation, and record duration, integrity and readiness to serve.
- Fault and recovery drills first write the steady-state hypothesis, the smallest blast radius,
  the fault action, observers, deadline/stop condition and recovery steps. Use authorized
  environments; production chaos is never default evidence; a drill never breaks read-only or
  no-restart scope.
- Emergency paths are test targets: rollback tools, config distribution, reconciliation and
  restore tools, credentials and consoles must not depend on the failing service. Verify the
  operation and its effect in an isolated drill; a runbook file existing is not evidence.

## 3. Upgrade, config and supported rollback

Old and new versions passing separately does not prove they coexist during a rolling release.
For changes to formats, protocols, state, schema or flags, run representative old data with
continuous business traffic through each stage:

| Stage | Observe |
|---|---|
| Prepare / capability | every reader, consumer and background job handles the new format before any new write is activated; "most instances healthy" is not enough |
| Mixed versions | writes, reads, callbacks and jobs in real deployment order; invariants and error ownership checked continuously |
| Fully upgraded / activated | new writes coexist with old data; background jobs and async cycles covered; effective config matches intent |
| Rollback / recovery | after returning to the project's supported version, written data and events still process; invariants and progress hold |

- Prepare/Activate is one optional design, not a mandatory release mode.
- After activation the rollback floor may be the compatible-reader version, not the original.
- Removing old-format reading needs evidence that old data is migrated.
- Configuration changes get the same staged observation as binaries; a functional pass does not
  prove resource safety (a rule change can exhaust CPU).

## 4. Canary evidence

Only when release verification is authorized:

- Pick metrics and cohorts that reflect user outcomes: success/error, tail latency, business
  invariants, async task age and completion.
- The window covers relevant traffic, work units and background cycles. A few successful requests
  or health checks under low traffic are insufficient; missing samples are missing evidence.
- Compare candidate and control at the same time under comparable conditions, attributable by
  version, config and cohort. Before/after comparisons mix in time effects; shared databases,
  caches or queues let the candidate degrade the control, so also check absolute SLOs and
  invariants and record shared surfaces and observer health. Both groups equally bad is not a pass.
- Synthetic and mirrored traffic misses production states; mirrored writes can duplicate side
  effects or shift cache and load. Canary supplements missing pre-merge evidence, never replaces
  it, and never authorizes a production experiment.

## 5. Incident → durable control

1. **Fix facts.** Version and environment, trigger, timeline, user impact; separate observed facts
   from hypotheses; keep the minimal reproducer, input, seed or interleaving.
2. **Map to the invariant.** With `arc:code`, confirm the owning boundary, violated rule, root
   cause and sibling paths ([execution-truth §6](../../../docs/execution-truth.md)); explain why
   existing verification missed it. "Add a retry" is not a root cause.
3. **Choose controls** the evidence supports: prevention (constraint, fix), detection (assertion,
   reconciliation), containment (boundary, pause), recovery (compensation, replay).
4. **Prove the control catches the known failure** with the saved reproducer or a safe fault
   model, then prove the fix passes. If replay is impossible, record the substitute evidence and
   its limits; never claim the failure was reproduced.
5. **Make it routine.** The regression enters the matching CI cadence; alerts and recovery steps
   have owners and executable steps. Long-running follow-up becomes `arc:plan` task documents
   linking rule, evidence and exact queries.

A finished postmortem is not finished remediation: unverified controls stay `NOT_RUN`, `BLOCKED`
or `[?]`, and a prose-only postmortem is incomplete when an engineering control is feasible.
