# Release Evidence, Flaky Policy and Gate

Evidence records, verdict semantics, flaky handling and the go/no-go gate. Rollout, detection,
recovery and incident controls: [ops-learning.md](ops-learning.md).

## 1. Evidence record

Use the CI report or project format; these are required facts, not a new schema.

| Part | Record |
|---|---|
| Claim | rule, case ID, EXPECTED/FORBIDDEN, required yes/no |
| Candidate | commit or build ID, fingerprint of uncommitted changes, case/config version, environment and real dependency versions |
| Execution | working directory, command or selector, tool version, start/end/duration, exit status |
| Reproduction | test data identity, seed, schedule/barrier, corpus or failing input, replay steps |
| Observation | actual business end state, assertions, required observer status, logs/history/reconciliation artifacts |

- Replace DSNs, tokens and keys in commands with variable names; keep enough non-secret config and
  data version to replay. Redaction is never a reason the run cannot be traced.
- Evidence belongs to the current candidate and applicable environment. Green from another commit,
  config or migration stage carries over only with shown applicability; otherwise the case is
  re-run. A plan, CI YAML or test file existing is preparation, not evidence.
- A gate that reads evidence files binds them to one run: a run ID stamped into every record, the
  candidate fingerprint (commit, uncommitted changes, config) recorded at the start and re-checked
  at the gate, and earlier records moved aside before the run. A file that merely exists in the
  evidence directory, or a comparison that compared nothing, never satisfies a required claim.

Verdict semantics (vocabulary in [contract §4](../../_shared/contract.md)):

| Verdict | Meaning |
|---|---|
| `PASS` | executed, required observers healthy, business assertions held, traceable to this candidate |
| `FAIL` | executed and violated the contract; a runner, observer or teardown failure is recorded separately and proves no product defect, the affected claim is `BLOCKED`, and the gate still fails |
| `BLOCKED` | a named prerequisite is missing; list it and the independent work already done |
| `NOT_RUN` | not executed this cycle (cut, skipped, stale); give the reason |
| `FLAKY` | unstable under the same conditions, cause unconfirmed; every attempt kept |

`SPECIFICATION GAP` is an undefined rule or oracle, never a passing state.

## 2. Flaky protocol

1. Keep the first failure and every rerun; record candidate, test and tool versions, environment
   and data differences.
2. Classify with at most 3 reruns under unchanged conditions. Mixed results → `FLAKY`.
3. Reproduce with repeat runs ([stacks.md](stacks.md) §Repeat runs): up to 50 iterations, stopping
   after 3 captured failures; vary one factor at a time (parallelism, CPU count, order, seed, race
   detector). Not reproduced in 50 → the verdict stays `FLAKY` from history; report the attempt.
4. Run a known baseline or placebo (parent commit, same conditions) to separate candidate,
   environment and test causes. Control results are diagnostic only: both passing or both failing
   never settles the business claim.
5. Diagnose with root-cause repair ([execution-truth §6](../../../docs/execution-truth.md)).
   Common test causes: sleep or wall-clock dependence, order dependence, shared fixtures or ports,
   unseeded randomness, async waits without a completion condition, leaked goroutines or timers.
   A confirmed test cause is fixed here and proven by zero failures in twice the iterations that
   reproduced it (at least 20); a product race goes to `arc:code` as a repair packet.
6. Quarantine is a proposal the user approves, recording owner, reason, repair condition and lost
   claim coverage. A quarantined required claim stays `FLAKY` or `NOT_RUN` and keeps blocking its
   gate until fixed evidence, credible replacement evidence, or a separately recorded risk
   decision exists. A risk decision never changes the verdict.
7. After a fix, a new valid run can be `PASS`; old failures stay in history.

Never retry until green, add blanket retries or longer timeouts, delete the test, or treat "CI no
longer runs it" as passing.

## 3. Gate order (release mode)

1. Fix the required claim set from change impact and consequence ([scope-cadence.md](scope-cadence.md)
   §6), reusing stable cases. The set never shrinks because a result failed; any change needs a
   contract or scope reason.
2. Each claim has a rule, an execution mode and a credible oracle: automated → runnable binding;
   manual → steps and an actual observation; analysis → assumptions and a model. Claims needing
   real runtime semantics cannot be met by analysis.
3. Validate each evidence record: candidate identity, environment, result, observer health,
   artifact present. Corrupt, conflicting, field-missing or assertion-less evidence is a gap; never
   skip it silently or take the last `PASS`.
4. GO only when every required claim is `PASS` and the project's own gates pass. Any required
   `FAIL`, `BLOCKED`, `NOT_RUN` or `FLAKY` → NO-GO; low-risk test counts, coverage or scores never
   offset it. Unresolved P0/P1 defects block unless a named, time-bounded waiver exists.
5. Write residual risk and the detection, stop, rollback and recovery conditions
   ([ops-learning.md](ops-learning.md) §2). Waivers come only from the project's risk process and
   record decider, scope, expiry and compensating control, kept apart from the original verdict.
   The agent never approves its own waiver. Money, data and permission invariants cannot be
   waived by an availability error budget.

```text
| Claim | Required | Verdict | Evidence | Waiver |
Decision: GO | NO-GO (blocking claims: …)
Residual risk: …
Containment: stage · stop condition · rollback or forward-fix · owner
```

The gate is decision input: it never deploys, restarts, rolls back or flips flags. When the user
already asked for those, hand the current evidence to that process without adding approval steps.

### Installed and published surfaces

When the change affects a skill, plugin, CLI, installer, generated mirror or package archive,
include the installed-runtime claim: identify the real installation path, inspect the built
artifact for every required reference/helper and executable permission, and exercise discovery
and the changed entrypoint from an isolated temporary installation. A source import or manifest
check proves neither package completeness nor installed behavior. Instructions need observed
tool actions and resulting artifacts for behavioral claims; link checks prove only packaging.

Apply the [installed runtime procedure](../../../Waza/check/references/installed-runtime.md)
when this surface is in scope. Keep smoke configuration separate through supported host options;
do not repurpose the operator's `HOME` or active configuration. Missing host/runtime prerequisites
remain explicit gaps.

For release evidence, name the exact lane and candidate. Generated files, package contents,
remote assets and deployed services each need their own observation. Download/read back required
assets and compare contents or digests where available; release-page text and CI success alone do
not establish what users received. A reworked candidate may allow continued preview testing while
remaining NO-GO for stable release; bind each decision to its own required claims.

These checks adapt Waza's release-surface methods under the [MIT license](../../../Waza/LICENSE).
Actual authorized commit/push/publish or public follow-through may continue through `check`;
release evidence itself grants no new external action.

## 4. Reliability report

For reliability and release work, write `.arc/artifacts/<task-slug>/reports/verify.md`:

- **Business model**: changed rules, applicable concerns, EXPECTED/FORBIDDEN, specification gaps.
- **Highest-risk failure paths**: what fails, business consequence, where in the architecture.
- **Verification plan**: cases, oracle per case, boundary and why, real dependencies required.
- **Implementation**: tests, harnesses and bindings added; tools installed; seams requested from `arc:code`.
- **Execution evidence**: exactly what ran, verdicts, failure reproduction data.
- **Release inputs**: missing required evidence, unresolved P0/P1 defects, containment readiness,
  residual risk in plain words.

No overall reliability score unless the user explicitly asks for a scoring exercise.
