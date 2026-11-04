---
name: arc:verify
version: 3.7.0
description: >-
  Whole-system testing with project-local DB business graphs (完整测试, 测试整个系统, 所有交互,
  完整业务测试, 业务树图, 业务图), including AI-led candidate modeling from scratch.
  Proves business code meets business expectations: flow inventories (业务流), test plans,
  write tests, TDD (测试先行), test-value review (测试价值), browser E2E (网页实测), coverage, fuzz,
  property tests, SQL lint, transaction design, reliability (concurrency, idempotency, faults),
  PostgreSQL G0-G31, flaky triage, load tests, release go/no-go. Verdicts PASS/FAIL/BLOCKED/NOT_RUN/FLAKY
  with independent oracles. Triggers: 写测试, 测试方案, 用例设计, 测试范围, 覆盖率, 可靠性, 业务不变量,
  事务并发, 幂等, 压测, 发布验收, 发布证据, 模糊测试, SQL 语法检查, 事务设计, 端到端业务流, 分支覆盖, 属性测试, 模型一致性,
  业务能力, 业务边界.
  Not for product repair (arc:code) or static review (arc:audit).
---

# arc:verify

Verify confirmed expectations through real entries and independent oracles; retain original FAILs.

## Non-trigger

- Product defects and fix reproducers → `arc:code`; static/security review → `arc:audit`.
- Code lookup/index → `arc:explore`; PRD/closure/task documents → `arc:plan`.
- Measured optimization → `arc:code`; Android traces → `android`; deploy/restart/rollback → authorized release process.

## Goal

Confirmed claims have IDs, source, oracle, verdict and sensitivity record; candidate graphs stay unverified.

## Inputs

| Input | Required | Default when absent |
|---|---|---|
| Target: feature, diff, PR, service or release candidate | yes | current project; ask only if ambiguous |
| Claims or acceptance criteria | no | draft from user intent/contracts; unconfirmed semantics stay `RISK` or `SPECIFICATION GAP` |
| Mode | no | first matching Intent Router row |
| Environment, runner, fixtures | no | the project's test setup |
| Tier (T0/T1/T2) | no | classify per contract §2 |
| Cadence: PR, nightly, pre-release | no | PR |
| Shared-target load/fault/reset authorization | no | none; unauthorized operations `BLOCKED` |

## Intent Router

| Condition | Mode | Read |
|---|---|---|
| Complete system/business-chain/interaction testing, or a DB business tree/graph, including no existing DB/model | graph | [business-graph](references/business-graph.md); first model/update: [graph-start](references/business-graph-start.md) |
| Local flow inventory/charts/branch cases; no DB graph requested | flow | [flow-model](references/flow-model.md), [flow-matrix](references/flow-matrix.md) |
| Test plan, case list or scope only; nothing to execute | design | [case-design](references/case-design.md), [scope-cadence](references/scope-cadence.md) |
| Judge the value of existing tests (测试价值) | design | [test-value](references/test-value.md) |
| Acceptance tests before the code exists (测试先行, TDD) | red | [test-first](../_shared/test-first.md), [case-design](references/case-design.md) |
| Intermittent test/CI failure | flaky | [release-ops](references/release-ops.md) §2, [stacks](references/stacks.md) |
| PostgreSQL release: DDL, migration, plan, locks, replication, backup, upgrade, roles/RLS | db | [db](references/db.md), then applicable gate files |
| Release go/no-go, canary, rollback readiness, incident follow-up controls | release | [release-ops](references/release-ops.md), [ops-learning](references/ops-learning.md) |
| Invariant, transaction, concurrency, idempotency, retry, async, fault or recovery claim | reliability | [txn-faults](references/txn-faults.md), [case-design](references/case-design.md) |
| Execute SQL parse/lint (SQL 语法检查) | write | [sql-static](references/sql-static.md) |
| Transaction design per engine (事务设计), no execution | design | [txn-design](references/txn-design.md) |
| Web load/interaction speed, Core Web Vitals, Lighthouse (网页性能) | write | [web-perf](references/web-perf.md) |
| Browser journey on a running or startable app (网页实测) | write | [web-e2e](references/web-e2e.md) |
| Untrusted input into parsers, decoders, uploads or protocols (模糊测试) | write | [fuzz-campaign](references/fuzz-campaign.md) |
| Property/model tests; code-to-model conformance | write | [model-conformance](references/model-conformance.md) |
| Other unit, integration, contract, E2E, UI, load or benchmark tests | write | [stacks](references/stacks.md), [case-design](references/case-design.md), [test-value](references/test-value.md) §1–§2 |
| Claims come from a closure model or another phase's handoff | any | [workflow.md](../_shared/workflow.md) §2, §4 |
| One end-to-end claim → case → evidence example | any | [worked-example](references/worked-example.md) |

Load contract and code-intel, then ≤3 topical references for the current card.
Scope overrides mode: plan-only creates no DB unless requested; read-only never mutates the ledger.
DB graph-only requests stop after modeling/snapshot. Load workflow for handoff/closure work.
Existing-tests-only uses selected commands; skip flow creation, test writes and sensitivity mutations.
HTTP/JSON/upload claims use
[API cases](references/api-contract-tests.md).

## Workflow

Graph mode follows [business-graph](references/business-graph.md), replacing steps 2–10.
Graph/flow diagrams: [arch-diagram](references/business-diagram.md).
Without a confirmed model use [first start](references/business-graph-start.md): the Agent drafts,
imports and exports a candidate for review, then continues authorized verification after confirmation.
Do not ask users to write graph JSON or finish a full test request at a plan.
Execution uses [v2 commands](references/business-graph-v2.md); handoffs use
[workflow §6](../_shared/workflow.md#6-graph-based-business-chain-verification-across-arc-skills).
`arc-verify handoff --to arc-verify --graph <id>` expands bounded local context.
New tests still require sensitivity proof. No Agent handshake or model HTTP.

1. Read [contract](../_shared/contract.md) and existing progress; record tier, scope, mode,
   environment and git baseline.
2. Confirm capabilities from contracts; trace UI/API/RPC → usecase → writes/async for candidates.
   Routes/CRUD/HTTP 200 prove no capability. Unconfirmed items stay `CANDIDATE`/`UNSPECIFIED`/`NOT_RUN`;
   missing/stale flows → flow mode. Output: catalog, IDs, gaps; T0/pure logic may give `n/a: <reason>`.
3. Claims table: `EXPECTED`/`FORBIDDEN`, confirmed source, required yes/no, oracle per branch/RULE.
   AI inference stays `RISK`; undefined semantics are `SPECIFICATION GAP`.
4. Map impact: [code-intel](../_shared/code-intel.md) §4 and flow-matrix §7.
5. Design cases per case-design and flow-matrix §2–§4; name the independent oracle and defect.
6. Bind existing runner, fixtures, CI job and test DSN, in order; record environment or
   `BLOCKED: <missing item>` per claim.
7. Flow/design finish `NOT_RUN (plan only)` with ledger check. Red runs step 8 before implementation;
   hand assertion failures to `arc:code` (test-first §5).
8. Run SQL checks per sql-static without rewriting SQL; otherwise write/run owning-package tests
   per test-value §1. Save command, exit and per-claim output to `.arc/artifacts/<task-slug>/logs/`.
9. Save sensitivity: targeted assertion fails on pre-fix code, known-bad input or a disposable
   worktree mutation, then passes. A red record counts.
10. Retain first divergence/original FAIL. Resume only from independently observed safe state
    after reconstruction in a new run, or linked new task for a changed branch. Recheck the ledger.
11. Run the mode tail: flaky protocol, db gates or release GO/NO-GO.
12. Hand off: each `FAIL` → repair packet ([execution-truth](../../docs/execution-truth.md) §6) for
    `arc:code`; each `SPECIFICATION GAP` → user; gaps and uncovered branches per flow-matrix §8.
    Report per contract §9. Output: report.

## Tool Strategy

- **Preferred**: project test entry → stacks' native runner → a necessary new tool (permission,
  name/version recorded). Start web apps with `scripts/with_server.py`.
- **Queries**: code-intel §3, then §5; tests stay beside their owner. Compile/start errors are harness failures.
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
- IF a local T1/T2 test lacks a covering flow THEN flow first; complete-system/DB-graph requests
  stay graph mode and reuse existing mappings.
- IF read-only THEN assess existing flows, claims, bindings and evidence only (no test writes,
  installs, fault injection or data-writing runs); unexecuted claims are `NOT_RUN (read-only)`.
- IF a claim concerns concurrency, idempotency, a transaction or money/inventory/quota THEN run the
  [txn-faults](references/txn-faults.md) §0 minimum cases first.
- IF correctness depends on PostgreSQL, broker, filesystem or network semantics THEN use the real
  dependency in the project's test environment; without it `BLOCKED`, never a mock `PASS`.
- IF the sensitivity check passes on the defective version THEN the test is invalid; strengthen it
  before any verdict.
- IF nothing was checked (no test selected, zero comparisons, empty baseline or policy) THEN `BLOCKED`.
- IF observations are missing or runner fails THEN `BLOCKED`; observed RULE violation is `FAIL`.
  Harness failure never proves a product defect.
- IF evidence comes from another run, commit, config or environment THEN `NOT_RUN (stale)`; gates
  check run identity, not file presence ([release-ops](references/release-ops.md) §1).
- IF a performance claim exists THEN prove correctness first, then measure under the same
  candidate/environment ([scope-cadence](references/scope-cadence.md) §4); fast-but-wrong = `FAIL`.
- IF the expected result is undecided THEN `SPECIFICATION GAP`: ask only that decision, continue
  with independent claims; never adopt current output as the expectation.
- IF diagnosing a failure THEN follow the evidence-first root-cause repair gate in
  [execution-truth.md](../../docs/execution-truth.md) §6; fix test/fixture causes here,
  hand product causes and affected siblings to `arc:code`.

## Verification

| Completion claim | Evidence |
|---|---|
| Test added, `PASS` | test name, `file:line`; command on the current commit (hash), exit code, oracle observed, log path |
| Sensitivity, red record | targeted assertion fails on the bad or pre-implementation version, passes on the candidate |
| `FAIL` | failing assertion output plus repair packet |
| `BLOCKED` / `NOT_RUN` | missing prerequisite or deferral reason |
| Authority kept | scoped changes only: tests, fixtures, test configs, graph ledger, flow catalog or artifacts |
| Graph gate | frozen task/RAW revalidated; PASS covers planned positive/negative/forced; recovery, cross-role, concurrency and E2E need separate evidence |

## Failure Recovery

| Failure class | Action | Retry budget | Stop condition |
|---|---|---|---|
| Environment missing / server exit 125 | inspect env samples, CI, server log; name missing item | 1 pass | BLOCKED; continue independent claims |
| Compile/start failure | fix test/import/config | 3 | BLOCKED with last error |
| Baseline fails | same test on parent commit | 1 | judge only delta |
| Assertion fails | retain FAIL/RAW, read actual state; replay needs authorization, same candidate/config, verified isolated reconstruction | ≤1 safe rerun | unknown state BLOCKED; divergence FAIL + packet |
| Alternating result | flaky protocol; comparable candidate/config/verified initial state, side effects checkpointed | 3 safe classification reruns; ≤50 authorized iterations | FLAKY + quarantine proposal; unsafe replay BLOCKED |
| Hang | stop runner/descendants at deadline; keep partial RAW/dump, read actual state | 1 | observer error BLOCKED; independently observed contracted liveness breach FAIL |
| Broken oracle | repair observer | 1 | dependent claims BLOCKED |

## Completion Criteria

- [ ] Scope/mode/context recorded; catalog separate from implementation, unresolved confirmation explicit.
- [ ] Scoped branches/cards retain cases, source/oracle/verdict/RAW and historical FAIL; runtime completion needs ledger/graph PASS.
- [ ] Required sensitivity, meaningful owning tests and logs exist; every BLOCKED/NOT_RUN has a reason.
- [ ] Production source unchanged; product FAIL has an `arc:code` repair packet.
- [ ] Applicable mode tail and contract §9 report complete.
