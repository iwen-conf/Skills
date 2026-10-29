---
name: arc:verify
version: 3.4.0
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

Execute claims with independent oracles, one verdict per claim, designed from the business flows a
change reaches, never from an endpoint list. Green is an observation, never the goal; a failure
against a contract-derived expectation is a product defect, never a test to bend.

## Non-trigger

- Fixing a defect found here, or the reproducer written inside a fix → `arc:code`.
- Static review, code health, security scanning, penetration testing → `arc:audit`.
- Locating code or building the code index only → `arc:explore`.
- PRD, closure model or task documents for a large test build-out → `arc:plan`.
- Optimizing a measured hotspot → `arc:code`; Android trace queries → `android`.
- Deploy, rollback, restart or flag flip → the user's release process.

## Goal

Every claim traces to a flow branch or rule and has a verdict, re-runnable evidence and, when
required, a sensitivity record.

## Inputs

| Input | Required | Default when absent |
|---|---|---|
| Target: feature, diff, PR, service or release candidate | yes | ask once |
| Claims or acceptance criteria | no | derive from flows, contracts, docs and code; label each `derived` |
| Mode | no | first matching Intent Router row |
| Environment: test commands, test DSN, service URLs, devices | no | the project's test setup |
| Tier (T0/T1/T2) | no | classify per contract §2 |
| Cadence: PR, nightly, pre-release | no | PR |
| Authorization for shared-target load, fault injection or destructive reset | no | none; those steps stay `BLOCKED` |

## Graph-based business-chain verification (local Turso)

For requests such as "test all interactions in the academic system", first discover the
*expected* business catalog with the user, independent of implementation. Read
[local graph verification](references/business-graph.md) for contract / implementation
comparison, task generation, runtime observations and CLI use. AI-derived industry
assumptions are RISK only; only user-confirmed RULE may produce a required FAIL/MISSING
claim. Store project-local graph and run ledger in `.arc/verify/data.db`; never use
Turso Cloud, network URLs, permanent workers, or a second database.

Use the short-lived Go `arc-verify` CLI to record graph versions, approvals, tasks,
observations and evidence; do not claim E2E PASS without a real client runner and
independent RAW. Existing FLOW catalog and test modes remain authoritative; graph
verification adds tracking and must not replace browser/device/real-DB evidence.

## Intent Router

| Condition | Mode | Read |
|---|---|---|
| Business flows: inventory, flowcharts, branch → case ledger, flow coverage; T1/T2 behavior no current flow covers | flow | [flow-model](references/flow-model.md), [flow-matrix](references/flow-matrix.md) |
| Test plan, case list or scope only; nothing to execute | design | [case-design](references/case-design.md), [scope-cadence](references/scope-cadence.md) |
| Judge the value of existing tests (测试价值) | design | [test-value](references/test-value.md) |
| Acceptance tests before the code exists (测试先行, TDD) | red | [test-first](../_shared/test-first.md), [case-design](references/case-design.md) |
| Write or run unit, integration, contract, E2E, UI, load or benchmark tests | write | [stacks](references/stacks.md), [case-design](references/case-design.md), [test-value](references/test-value.md) §1–§2 |
| Browser journey on a running or startable app (网页实测) | write | [web-e2e](references/web-e2e.md) |
| Untrusted input into parsers, decoders, uploads or protocols (模糊测试) | write | [fuzz-campaign](references/fuzz-campaign.md) |
| Property or model-based tests; code-to-model conformance | write | [model-conformance](references/model-conformance.md) |
| SQL parse/lint (SQL 语法检查); transaction design per engine (事务设计) | design | [sql-static](references/sql-static.md); [txn-design](references/txn-design.md) |
| Invariant, transaction, concurrency, idempotency, retry, async, fault or recovery claim | reliability | [txn-faults](references/txn-faults.md), [case-design](references/case-design.md) |
| PostgreSQL change heading to release (DDL, migration, plan, locks, replication, backup, upgrade, roles/RLS) | db | [db](references/db.md), then only the gate files the change hits |
| Web page load or interaction speed, Core Web Vitals, Lighthouse (网页性能) | write | [web-perf](references/web-perf.md) |
| Same code passes and fails; intermittent CI failure | flaky | [release-ops](references/release-ops.md) §2, [stacks](references/stacks.md) |
| Release candidate go/no-go, canary, rollback readiness, incident follow-up controls | release | [release-ops](references/release-ops.md), [ops-learning](references/ops-learning.md) |
| Claims come from a closure model or another phase's handoff | any | [workflow.md](../_shared/workflow.md) §2, §4 |
| One end-to-end claim → case → evidence example | any | [worked-example](references/worked-example.md) |

Read ≤3 references per task (shared files excluded). Modes chain: flow → design → red →
write/reliability/db → flaky → release.
HTTP/JSON/upload claims use [API cases](references/api-contract-tests.md) within that budget.

## Workflow

For a graph-enabled business-chain request, use the shared
[workflow §6](../_shared/workflow.md#6-graph-based-business-chain-verification-across-arc-skills)
and `arc-verify handoff --to arc-verify --graph <version>` to fetch only
the next executable cards. `arc:plan` owns user-approved rule wording,
`arc:explore` supplies implementation locations, and `arc:verify` alone
writes test graph execution state. Confirmed FAIL → `arc:code` via
`arc-verify handoff --to arc-code --run <id>`; specification gaps → plan;
unconfirmed NOT_FOUND/risk → investigate without business FAIL; dependency
selection → vet. Retain existing FLOW/branch-ledger proof and environment gates.

1. Read [contract](../_shared/contract.md). Record tier, scope, mode, environment and entry `git diff`/status baseline.
   Output: task-log header.
2. Flows first: find the target's flows in the catalog (flow-model §1); missing or `stale` → flow
   mode traces entry points and writes rows, charts and branch tables. Output: FLOW and branch IDs,
   or `n/a: <reason>` (T0, pure logic).
3. Claims table: one row per branch or rule, `EXPECTED` or `FORBIDDEN`, source (`contract`, `doc`,
   `user`, `derived`), required yes/no; an undefined result is a `SPECIFICATION GAP`. Output: claims table.
4. Map impact: [code-intel](../_shared/code-intel.md) §4 rows 2, 5–7 (3–4 for state or
   serialization) and flow-matrix §7; boundary and cadence per scope-cadence. Output: scope table.
5. Design cases per case-design and flow-matrix §2–§4; each names its oracle and the realistic
   defect it must catch. Output: case table.
6. Bind the environment: existing runner, fixtures, CI job, test DSN, in that order. Output:
   environment line, or `BLOCKED: <missing item>` per claim.
7. Flow and design modes stop here: claims `NOT_RUN (plan only)`; `scripts/flow_ledger.py check
   <catalog>` exits 0. Red mode runs step 8 on the pre-implementation code and hands each targeted
   assertion failure to `arc:code` (test-first §5).
8. Write tests in the repository's test directories and naming scheme, each answering test-value
   §1; run them; save output to `.arc/artifacts/<task-slug>/logs/`. Output: command, exit code, log
   path per claim.
9. Prove sensitivity per required claim: the test fails on its targeted assertion against the
   pre-fix commit, a known-bad input or a mutation in a disposable `git worktree` (removed after);
   a red record counts. Output: fail-then-pass record.
10. Assign verdicts per Decision Rules to claims and branch rows; re-run the ledger check. Output:
    verdicts with evidence paths.
11. Mode tail: flaky protocol, db gate groups or release gate order. Output: flaky record, gate
    table or GO/NO-GO.
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
  - re-blessing snapshots or golden files, weakening or deleting assertions, raising timeouts or
    retries to hide a failure, retry-until-green;
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
- IF evidence comes from another run, commit, config or environment THEN `NOT_RUN (stale)`; gates
  check run identity, not file presence ([release-ops](references/release-ops.md) §1).
- IF a performance claim exists THEN judge it apart from correctness
  ([scope-cadence](references/scope-cadence.md) §4); fast-but-wrong = `FAIL`.
- IF the expected result is undecided THEN `SPECIFICATION GAP`: ask only that decision, continue
  with independent claims; never adopt current output as the expectation.
- IF verification diagnoses a failure THEN follow root-cause repair in
  [execution-truth](../../docs/execution-truth.md) §6 through root cause and affected siblings;
  fix confirmed test or fixture defects here; send product defects to `arc:code` as a repair packet.

## Verification

| Completion claim | Evidence |
|---|---|
| Test added, `PASS` | test name, `file:line`; command on the current commit (hash), exit code, oracle observed, log path |
| Sensitivity, red record | targeted assertion fails on the bad or pre-implementation version, passes on the candidate |
| `FAIL` | failing assertion output plus repair packet |
| `BLOCKED` / `NOT_RUN` | missing prerequisite or deferral reason |
| Authority kept | delta since phase entry touches only tests, fixtures, test configs, the flow catalog or artifacts |
| Mode evidence | the mode reference's evidence and verdict section: flow-model §1, fuzz-campaign §8, model-conformance §7, release-ops §2–§3, db.md |

## Failure Recovery

| Failure class | Action | Retry budget | Stop condition |
|---|---|---|---|
| Environment missing (DSN, device, browser, service; `with_server.py` exit 125) | search env samples, CI config and the server log once; else `BLOCKED` naming the missing item | 1 pass | continue unaffected claims |
| Test does not compile or start | fix the test code, imports or test config | 3 attempts | `BLOCKED` with the last error |
| Baseline already failing | run the same test on the parent commit | 1 | pre-existing: judge only the delta |
| Assertion fails | rerun once unchanged | 1 | `FAIL` + repair packet |
| Result alternates | flaky protocol | 3 reruns to classify; ≤50 iterations to reproduce | `FLAKY` + quarantine proposal |
| Test hangs | kill at the stack timeout; capture a thread dump | 1 | `BLOCKED` or `FAIL` with the dump |
| Observer or oracle breaks | repair the observer (test code) | 1 | `BLOCKED` for dependent claims |

## Completion Criteria

- [ ] Contract read; tier, mode and environment recorded.
- [ ] Flows in scope current; every branch has a case or a reason; ledger check exits 0.
- [ ] Claims table complete: EXPECTED/FORBIDDEN, source, oracle, verdict, evidence per row.
- [ ] New tests answer test-value §1 and live in the repository's test directories; logs under
  `.arc/artifacts/<task-slug>/logs/`.
- [ ] Required claims have a sensitivity record; every `BLOCKED`/`NOT_RUN` has a reason.
- [ ] Production source unchanged; every `FAIL` has a repair packet for `arc:code`.
- [ ] Mode tail done: flaky quarantine proposal, db gate table or release GO/NO-GO (release-ops §2–§3).
- [ ] Report in the contract §9 format.
