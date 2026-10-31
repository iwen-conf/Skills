# Case Design

Turn a business promise into cases whose oracles can disprove it. Boundary and cadence choice:
[scope-cadence.md](scope-cadence.md). Commands: [stacks.md](stacks.md).

## 1. Derive the contract

Read existing requirements, API/domain contracts and incident records before writing a case.
Record what the change touches:

- roles and permissions; legal and forbidden state transitions; terminal states;
- input bounds, counts, quotas, time points, units and clock source;
- durable results, emitted events, external side effects, audit records;
- every other entry point, job or consumer that reads or changes the same state (this sets the
  regression scope). Configuration, data migrations and operator tools change behavior too.

Identity values (user type, tenant, source, role, status) come from project enums, columns or
user docs. Placeholder IDs in examples (`A`, `B`, `X`) are never seeds or defaults.

Write each claim as one of:

- **Safety**: a result that must never occur at any moment (negative stock, double charge).
- **Liveness**: progress that must occur under named conditions within a project-given deadline.
  "Eventually consistent" needs a completion condition, an observation window and the
  post-timeout contract; never invent the number. A timeout or failed query is not evidence that
  no side effect happened.

## 2. Declare applicability

Mark each concern `applies`, `n/a: <reason>` or `unknown` (unknown = gap):

`stateful` `permissions` `concurrent` `retryable` `external_side_effects` `time_sensitive`
`compatibility` `migration` `asynchronous` `destructive` `long_running`

- `applies` → the matching model and at least one case are required.
- `stateful` → a state model plus legal, illegal, repeated and after-terminal transition cases.
- `n/a` → no section and no empty table; a non-concurrent path gets no concurrency cases.

Then pick the phases each applicable concern needs:

| Phase | Promise to check |
|---|---|
| Normal | core task, permissions, realistic data distribution, acceptable latency and capacity |
| Boundary | both sides of every limit, empty/null/missing, exact maximum, overflow, precision, time zone |
| Fault | slow dependency, timeout/cancel, duplicate/reorder, partial commit, resource exhaustion: safe failure semantics |
| Recovery | restart, dependency return, replay/compensation, backlog drain, restore: contracted data/effect counts and completion or explicit terminal-failure/manual destination within the project deadline |
| Change | config/flag, old data, mixed versions, upgrade/rollback, interrupted migration, background jobs, operator tools |

A normal success does not prove the fault phase; a healthy process after recovery does not prove
backlog, data and business recovered. Each phase has its own observation and exit condition.

## 3. Case card

| Field | Content |
|---|---|
| Claim / source | rule ID, doc or flow branch (`FLOW-NNN-Nxx-Bxx`), failure consequence, required yes/no |
| Preconditions | reproducible state, data, permissions, version/config, real dependencies |
| Actions / history | entry point and order; for races the interleaving, fault point and recovery step |
| EXPECTED | exact result, durable change, counts, events, external effect, follow-up work |
| FORBIDDEN | duplicate or lost effect, wrong object or amount, unauthorized change, illegal transition |
| Oracle | `kind` (database, api, event, model, audit, metric, device, file) + what is observed + exact assertion |
| Sensitivity | the realistic defect that must make this case fail and the assertion that catches it |
| Binding | selector or command, data setup/cleanup, observer, failure artifacts; mode `automated` (runnable binding), `manual` (steps, observation, operator, time) or `analysis` (model, assumptions, conclusion) |

- Manual and analysis cases never stand in for automated or real-dependency evidence.
- A script that exits 0 without an assertion is not a pass.
- Rule IDs live with the contract; bindings change with refactors.
- Given/when/then is fine for wording but never drops side effects or post-fault results.

Weak oracle: "cancel returns 200". Strong oracle, when the contract says cancel releases the
hold: the entity reaches the contracted state, the hold is released exactly once, a retry of the
same logical cancel creates no new effect, and events/refund/audit are checked only as the
contract defines them (undefined ones are `SPECIFICATION GAP`).

Traceability: requirement → flow branch → rule → case → EXPECTED/FORBIDDEN → evidence → release
criterion ([flow-matrix](flow-matrix.md) §5 ledger). A rule
with no case is a coverage gap; a case with no rule is low value; a critical outcome with no
observable oracle is a testability gap.

## 4. Method by risk shape

| Risk shape | Method | Usually missed |
|---|---|---|
| Ranges, quantities, thresholds | equivalence classes + boundary values on both sides | units, precision, clock source, exact maximum, overflow |
| Multi-condition rules, permissions | decision table; flip each condition alone; allow and deny sides | all-true/all-false rows hide a dead condition |
| Stateful workflow | state-transition and operation-sequence tests, generated from the model ([model-conformance](model-conformance.md) §4) | transition order; forbidden effect in an intermediate state |
| Concurrency, retry, partial success | operation history + controlled interleaving + fault point; query and replay after an unknown outcome | success-only checks, stress collisions, final-count-only checks |
| Optimized algorithm, broad rule space | independent reference model, differential, property ([model-conformance](model-conformance.md) §2) | oracle sharing the implementation's bug |
| Parser, protocol, file | fuzz campaign ([fuzz-campaign](fuzz-campaign.md)) with semantic oracles; keep failing inputs | crash-only oracle |
| Retry, ambiguous result, async effects | fault injection at the boundary + idempotency, final-state and reconciliation assertions | treating timeout as rollback |
| Migration, restart, recovery | representative old data + lifecycle run, invariants compared before/after | new-data-only fixtures |
| Overload, long-running | load/soak + invariants + queue/leak/recovery telemetry; performance judged separately | throughput with wrong results |
| Small critical concurrent primitive | systematic schedule exploration (`loom`, Go `testing/synctest`) | real-database locks are out of its model |

- Rank large combination spaces by consequence, frequency, shared state, incident history,
  reversibility and detectability. Pairwise reduces combinations but keeps every known dangerous
  3-way interaction. Order-sensitive behavior needs histories; pairwise parameter coverage does
  not replace sequences.
- Reproduce critical paths exactly first; use generated tests to explore beyond them. Random input
  needs an oracle and a recorded seed/schedule/corpus; stress collisions never replace a
  deterministic replay of a key race.
- Use a property only when the contract makes it true (commutativity, lossless round-trip,
  idempotent repeat). For differential tests align spec, precision, ordering and allowed
  differences first; the other implementation is not truth.
- Formal models, simulation and schedule exploration only for critical, modelable boundaries. A
  passing model means no counterexample under its assumptions, never a defect-free system.

## 5. Oracle checks

1. **Independent source.** The expectation comes from the contract, a simple independent model, a
   hand-derived concrete value or a trusted reconciliation. Never call the algorithm under test to
   compute it; shared format parsing may be reused.
2. **Precise.** Check identity, complete set/content, amounts, ordering when contracted, state and
   forbidden side effects. Two wrong lists can have the same length; "one record exists" does not
   prove the right record.
3. **Covers time.** A correct end state does not prove no oversell or privilege leak happened
   mid-run; check histories, constraints, events/audit and intermediate points when the claim is
   "never". Async waits poll a business completion condition up to the project deadline.
4. **Failure is visible.** Observer connection failure, missing permission, lagging data or zero
   checks executed is `BLOCKED`, not "no error found". Feed one known counterexample to each
   critical assertion.
5. **Conflicts are public.** When contract, history, docs and peer products disagree, record the
   difference and its basis; never pick the easiest to pass, and never auto-classify the
   difference as a product defect.

Mock call counts prove only the call contract, never the remote final effect.

## 6. Defect sensitivity

Ask: "If dedup is removed, a boundary flipped, one event dropped or one result omitted, where
does this case fail?" No answer → improve the scenario or assertion; similar extra cases do not help.

For critical new assertions and historical regressions, prove it with, in order: the pre-fix
commit's minimal reproducer, a known-bad oracle input, or a bounded mutation inside a disposable
worktree; then confirm the candidate passes. Record the counterexample, the assertion expected to
fail and the actual result.

- A red run caused by compile or environment failure did not catch the defect.
- Mutation is not a quota or score. Classify equivalent, out-of-contract and performance-only
  mutants separately; functional assertions may miss performance regressions.
- Never modify a shared or production target to prove a test works; never leave a production
  switch that bypasses business rules.
- A confirmed test or oracle defect may be fixed, with evidence kept; never change an expectation
  to fit a product defect.

## 7. Generators and exploration

Review which business states the generator reached, not how many iterations ran: independent vs
correlated fields, sparse/dense data, hit/miss, equal/distinct, hot/cold keys, duplicate/unique
keys, rare and extreme distributions. Constraints exclude only combinations the business forbids,
never real inputs that are inconvenient to judge.

Example: if matching rows are always contiguous in two indexes, an intersection query never
exercises skip-scan. Build uncorrelated keys, sparse intersections and interleaved inserts, and
compare the complete result set. Fuzzing can assert business properties, not only crashes.

Save the failing seed, input, history/schedule and tool version; when shrinking, confirm the
failure cause stays the same. A random seed may not replay a real concurrent schedule.

Exploratory testing for incomplete specs or risky areas: a time-boxed charter naming risk, data
and environment; record actions and findings; turn each reproducible important failure into a
regression case. Findings are leads until checked against the contract and root cause.

## 8. Testability seams

When a case cannot be controlled or observed, add the smallest seam in test code or test wiring:
injectable clock, seeded RNG, stable test IDs, barrier/failpoint at a meaningful transaction
point, reference model, isolated side-effect adapter, test tenant/namespace, observable
outbox/audit/reconciliation view. A seam that must live in production source is a request to
`arc:code`. Never add a seam with no real uncertainty behind it, or a production package-level
`var` that bypasses rules.

## 9. Stop

Stop when every required claim has current-candidate evidence, project gates pass, and untested
scope and residual risk are written down. Expand only for a new change, a failure or a named open
risk; never to raise coverage or case counts.
