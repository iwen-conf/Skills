---
name: arc:verify
version: 3.6.0
description: >-
  Proves business code meets business expectations: business-flow inventories and flowcharts
  tracing branches to cases (业务流, 端到端业务流, 分支覆盖), test plans (测试方案, 用例设计,
  测试范围), write tests (写测试), test-first acceptance tests (测试先行, TDD), test-value review
  (测试价值), regression and browser E2E suites (网页实测), coverage (覆盖率), fuzz campaigns (模糊测试),
  property and model-based tests (属性测试, 模型一致性), SQL lint gates (SQL 语法检查),
  transaction design per engine (事务设计), reliability checks (可靠性, 业务不变量) for transactions and
  concurrency (事务并发), idempotency (幂等), faults and recovery; PostgreSQL release qualification
  (G0-G31); flaky-test triage; load tests (压测); and release go/no-go gates (发布验收, 发布证据).
  Verdicts are PASS/FAIL/BLOCKED/NOT_RUN/FLAKY backed by independent oracles. Not for fixing the defects it finds (use arc:code) or static review (use arc:audit).
---

# arc:verify

Verify confirmed business expectations through real entry points and independent oracles;
retain original verdicts and failures.

## Non-trigger

- Product defects and fix reproducers → `arc:code`; static/security review → `arc:audit`.
- Code lookup/index → `arc:explore`; PRD/closure/task documents → `arc:plan`.
- Measured optimization → `arc:code`; Android traces → `android`; deploy/restart/rollback → authorized release process.

## Goal

Every confirmed claim has a FLOW/RULE ID, independent evidence, verdict and required sensitivity record.

## Inputs

| Input | Required | Default when absent |
|---|---|---|
| Target: feature, diff, PR, service or release candidate | yes | ask once |
| Claims or acceptance criteria | no | draft from user intent/contracts; unconfirmed semantics stay `RISK` or `SPECIFICATION GAP` |
| Mode | no | first matching Intent Router row |
| Environment, runner, fixtures | no | the project's test setup |
| Tier (T0/T1/T2) | no | classify per contract §2 |
| Cadence: PR, nightly, pre-release | no | PR |
| Shared-target load/fault/reset authorization | no | none; unauthorized operations `BLOCKED` |

## Intent Router

| Condition | Mode | Read |
|---|---|---|
| Business inventory, charts, branch → case ledger; T1/T2 behavior no current flow covers | flow | [flow-model](references/flow-model.md), [flow-matrix](references/flow-matrix.md) |
| Test plan, case list or scope only; nothing to execute | design | [case-design](references/case-design.md), [scope-cadence](references/scope-cadence.md) |
| Judge the value of existing tests (测试价值) | design | [test-value](references/test-value.md) |
| Acceptance tests before the code exists (测试先行, TDD) | red | [test-first](../_shared/test-first.md), [case-design](references/case-design.md) |
| Write/run unit, integration, contract, E2E, UI, load or benchmark tests | write | [stacks](references/stacks.md), [case-design](references/case-design.md), [test-value](references/test-value.md) §1–§2 |
| Browser journey on a running or startable app (网页实测) | write | [web-e2e](references/web-e2e.md) |
| Untrusted input into parsers, decoders, uploads or protocols (模糊测试) | write | [fuzz-campaign](references/fuzz-campaign.md) |
| Property/model tests; code-to-model conformance | write | [model-conformance](references/model-conformance.md) |
| SQL parse/lint (SQL 语法检查); transaction design per engine (事务设计) | design | [sql-static](references/sql-static.md); [txn-design](references/txn-design.md) |
| Invariant, transaction, concurrency, idempotency, retry, async, fault or recovery claim | reliability | [txn-faults](references/txn-faults.md), [case-design](references/case-design.md) |
| PostgreSQL release: DDL, migration, plan, locks, replication, backup, upgrade, roles/RLS | db | [db](references/db.md), then applicable gate files |
| Web page load or interaction speed, Core Web Vitals, Lighthouse (网页性能) | write | [web-perf](references/web-perf.md) |
| Intermittent test/CI failure | flaky | [release-ops](references/release-ops.md) §2, [stacks](references/stacks.md) |
| Release candidate go/no-go, canary, rollback readiness, incident follow-up controls | release | [release-ops](references/release-ops.md), [ops-learning](references/ops-learning.md) |
| Claims come from a closure model or another phase's handoff | any | [workflow.md](../_shared/workflow.md) §2, §4 |
| One end-to-end claim → case → evidence example | any | [worked-example](references/worked-example.md) |

Load ≤3 topical references for the current bounded phase/card. Shared contract, code-intel,
workflow and the configured business-graph protocol are the common baseline. Carry the frozen
claims, scope and evidence in the handoff across flow → design → red → write/reliability/db →
flaky → release. Split a phase when its required topics exceed the budget; never omit a required
protocol to fit it. HTTP/JSON/upload claims use [API cases](references/api-contract-tests.md).

## Workflow

For requested/configured business-chain work read [business-graph](references/business-graph.md)
and [workflow §6](../_shared/workflow.md#6-graph-based-business-chain-verification-across-arc-skills).
Follow confirmed RULE → graph → bounded card → real execution → independent RAW → verdict;
`arc-verify handoff --to arc-verify --graph <id>` expands only local context.

1. Read [contract](../_shared/contract.md); record tier, scope, mode, environment and git baseline.
2. Business catalog first: list expected functionality from user intent/contracts and obtain
   user confirmation; discover implementation separately via code-intel and real UI/API evidence.
   Reuse current flows; missing/stale → flow mode. Output: catalog, FLOW/branch IDs and candidate
   implementation gaps, or `n/a: <reason>` (T0, pure logic).
3. Claims table: `EXPECTED`/`FORBIDDEN`, confirmed source, required yes/no, oracle per branch/RULE.
   AI inference stays `RISK`; undefined semantics are `SPECIFICATION GAP`.
4. Map impact: [code-intel](../_shared/code-intel.md) §4 rows 2, 5–7 (3–4 for state or
   serialization) and flow-matrix §7; design phase records boundary and cadence per scope-cadence.
   Execution consumes that scope table instead of reloading the design references.
5. Design cases per case-design and flow-matrix §2–§4: satisfied conditions, each unsatisfied
   condition, forced operation and known recovery paths; name the independent oracle and defect.
   Output: case table, with uncovered paths explicit.
6. Bind the environment: existing runner, fixtures, CI job, test DSN, in that order. Output:
   environment line, or `BLOCKED: <missing item>` per claim.
7. Flow/design finish with `NOT_RUN (plan only)` and ledger check. Red runs step 8 on pre-implementation
   code, handing targeted assertion failures to `arc:code` (test-first §5).
8. Write tests beside their owning package per test-value §1; run them. Save command, exit and
   per-claim output to `.arc/artifacts/<task-slug>/logs/`.
9. Prove sensitivity: targeted assertion fails on pre-fix code, known-bad input or a disposable
   worktree mutation. A red record counts. Output: fail-then-pass record.
10. Assign verdicts; retain first divergence and original FAIL. Continue from an independently
    observed safe state in a new run after verified reconstruction, or a linked new task for a
    changed branch; recovery never rewrites history. Re-run the ledger check.
11. Run the mode tail: flaky protocol, db gates or release GO/NO-GO.
12. Hand off: each `FAIL` → repair packet ([execution-truth](../../docs/execution-truth.md) §6) for
    `arc:code`; each `SPECIFICATION GAP` → user; gaps and uncovered branches per flow-matrix §8.
    Report per contract §9. Output: report.

## Tool Strategy

- **Preferred**: the project's own test entry (Makefile target, package script, CI job) → the
  native runner in stacks → the smallest new tool a named failure model needs (with permission;
  record name and version). Start web apps under test with `scripts/with_server.py`.
- **Queries** for entry points, tests and fixtures: code-intel §3 ladder, then §5; place tests
  beside the closest owning package. A run that fails to compile or start is a harness failure.
- **Fallback**: no E2E infrastructure → API-level contract test, E2E `NOT_RUN`; no real database
  or broker → keep the runnable test plus preconditions, `BLOCKED`.
- **Forbidden**:
  - editing production source, including mutations outside a disposable worktree;
  - T1/T2 cases generated from an endpoint, function or file list without a flow;
  - expected values computed by the code under test; mocking the unit under test; mock-only proof
    of database, broker or durability semantics;
  - re-blessing snapshots/goldens, weakened/deleted assertions, timeouts/retries hiding failures;
  - `sleep` as synchronization; waits without a deadline;
  - load, stress, fault injection or destructive reset on shared or third-party targets without an
    authorization naming target, operations, data scope and stop condition; production chaos;
  - aggregate quality scores, universal coverage thresholds or E2E ratios as gates;
  - inventing domain keys, SLOs, budgets, deadlines or waivers.

## Decision Rules

- IF T0 and existing tests exercise the changed behavior THEN run them and add no test.
- IF T1/T2 behavior reaches an entry point, state or side effect no current flow covers THEN flow
  mode first.
- IF read-only THEN assess existing flows, claims, bindings and evidence only (no test writes,
  installs, fault injection or data-writing runs); unexecuted claims are `NOT_RUN (read-only)`.
- IF a claim concerns concurrency, idempotency, a transaction or money/inventory/quota THEN run the
  [txn-faults](references/txn-faults.md) §0 minimum cases first.
- IF correctness depends on PostgreSQL, broker, filesystem or network semantics THEN use the real
  dependency in the project's test environment; without it `BLOCKED`, never a mock `PASS`.
- IF the sensitivity check passes on the defective version THEN the test is invalid; strengthen it
  before any verdict.
- IF nothing was checked (no test selected, zero comparisons, empty baseline or policy) THEN `BLOCKED`.
- IF required observations are missing or the runner fails THEN `BLOCKED`; a valid observation
  violating a confirmed rule is `FAIL`. Never send a harness failure as a confirmed product defect.
- IF evidence comes from another run, commit, config or environment THEN `NOT_RUN (stale)`; gates
  check run identity, not file presence ([release-ops](references/release-ops.md) §1).
- IF a performance claim exists THEN prove correctness first, then measure under the same
  candidate/environment ([scope-cadence](references/scope-cadence.md) §4); fast-but-wrong = `FAIL`.
- IF the expected result is undecided THEN `SPECIFICATION GAP`: ask only that decision, continue
  with independent claims; never adopt current output as the expectation.
- IF diagnosing a failure THEN load the evidence-first root-cause repair gate in
  [execution-truth.md](../../docs/execution-truth.md) §6 through root cause and siblings;
  repair confirmed test/fixture defects here and hand product defects to `arc:code`.

## Verification

| Completion claim | Evidence |
|---|---|
| Test added, `PASS` | test name, `file:line`; command on the current commit (hash), exit code, oracle observed, log path |
| Sensitivity, red record | targeted assertion fails on the bad or pre-implementation version, passes on the candidate |
| `FAIL` | failing assertion output plus repair packet |
| `BLOCKED` / `NOT_RUN` | missing prerequisite or deferral reason |
| Authority kept | delta since phase entry touches only tests, fixtures, test configs, the flow catalog or artifacts |
| Mode evidence | applicable reference's evidence/verdict requirements |
| Graph gate | frozen task/RAW revalidated; PASS covers planned positive/negative/forced; recovery, cross-role, concurrency and E2E need separate evidence |

## Failure Recovery

| Failure class | Action | Retry budget | Stop condition |
|---|---|---|---|
| Environment missing; `with_server.py` exit 125 | check env samples, CI and server log; else `BLOCKED` with missing item | 1 pass | continue unaffected claims |
| Test cannot compile/start | fix test/import/config | 3 attempts | `BLOCKED` with last error |
| Baseline already failing | run the same test on the parent commit | 1 | pre-existing: judge only the delta |
| Assertion fails | retain FAIL/RAW; read back actual state; new-run replay needs existing authorization, same candidate/config and verified isolated reconstruction | ≤1 safe rerun | unknown state BLOCKED; valid divergence FAIL + packet |
| Result alternates | flaky protocol with comparable candidate/config/verified initial state; side effects need recovery checkpoint | 3 safe classification reruns; ≤50 authorized iterations | FLAKY + quarantine proposal; unsafe replay BLOCKED |
| Test hangs | stop runner/descendants at task deadline; preserve partial RAW/dump; verify actual state before continuing | 1 | observer failure BLOCKED; independently observed contracted liveness breach FAIL |
| Observer/oracle breaks | repair test observer | 1 | dependent claims `BLOCKED` |

## Completion Criteria

- [ ] Tier/mode/environment recorded; expected catalog confirmed separately from implementation.
- [ ] Every branch has a case/reason; claims retain source/oracle/verdict/RAW, IDs and historical FAIL; ledger check passes.
- [ ] Required sensitivity, meaningful owning tests and logs exist; every BLOCKED/NOT_RUN has a reason.
- [ ] Production source unchanged; product FAIL has an `arc:code` repair packet.
- [ ] Applicable mode tail and contract §9 report complete.
