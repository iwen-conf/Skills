---
name: arc:verify
version: 2.0.0
description: >-
  Produces evidence about behavior: write tests (写测试) and test plans (测试方案, 用例设计, 测试范围),
  regression and E2E suites, coverage (覆盖率) and fuzz, reliability checks (可靠性, 业务不变量) for
  transactions and concurrency (事务并发), idempotency (幂等), faults and recovery; PostgreSQL
  release qualification (G0-G31); flaky-test triage; load tests (压测); and release go/no-go gates
  (发布验收, 发布证据). Verdicts are PASS/FAIL/BLOCKED/NOT_RUN/FLAKY backed by independent oracles.
  Not for fixing the defects it finds (use arc:code) or static review (use arc:audit).
---

# arc:verify

Turns behavior claims into executed checks with independent oracles and returns one verdict per claim.

## Trigger

- Design test cases, test scope or a test plan.
- Write or run unit, integration, contract, E2E, UI or regression tests: 补测试, 回归.
- Verify business invariants, transactions, concurrency, idempotency, retries, faults and recovery:
  故障注入, 恢复演练.
- Qualify a PostgreSQL change for release (schema, migration, query plan, isolation, replication,
  backup, upgrade): 数据库资格, dbqual G0–G31.
- Triage an intermittent test: 不稳定测试, 偶发失败.
- Run load tests or benchmarks against a baseline: 基准测试, 性能回归.
- Decide release go/no-go from evidence: 发布门禁, 上线评估.

## Non-trigger

- Fixing a defect found here, or the reproducer written inside a fix → `arc:code`.
- Static review, code health, security scanning, penetration testing → `arc:audit`.
- Locating code or building the code index only → `arc:explore`.
- PRD, task brief, or multi-session task documents for a large test build-out → `arc:plan`.
- Optimizing a hotspot a benchmark or profile found → `arc:code`; Android trace queries →
  `perfetto-trace-analysis`; web page performance → `web-perf`.
- Deploy, rollback, restart or flag flip → the user's release process (this skill gives GO/NO-GO input).

## Goal

Every named claim ends with `PASS`, `FAIL`, `BLOCKED`, `NOT_RUN` or `FLAKY`, a re-runnable command
or artifact path, and, for each required claim, proof that the check fails on the defect it targets.

## Inputs

| Input | Required | Default when absent |
|---|---|---|
| Target: feature, diff, PR, service or release candidate | yes | ask once |
| Claims or acceptance criteria | no | derive from contracts, docs and code; label each `derived` |
| Mode | no | first matching Intent Router row |
| Environment: test commands, test DSN, service URLs, devices | no | the project's existing test setup; real-boundary claims without it are `BLOCKED` |
| Tier (T0/T1/T2) | no | classify per contract §2 |
| Cadence: PR, nightly, pre-release | no | PR |
| Authorization for shared-target load, fault injection or destructive reset | no | none; those steps stay `BLOCKED` |

## Intent Router

| Condition | Mode | Read |
|---|---|---|
| Test plan, case list or scope only; nothing to execute | design | [case-design](references/case-design.md), [scope-cadence](references/scope-cadence.md) |
| Write or run unit, integration, contract, E2E, UI, load or benchmark tests | write | [stacks](references/stacks.md), [case-design](references/case-design.md) |
| Invariant, transaction, concurrency, idempotency, retry, async, fault or recovery claim | reliability | [txn-faults](references/txn-faults.md), [case-design](references/case-design.md) |
| PostgreSQL schema/DDL, migration, query plan or index, isolation or locking, replication/CDC, backup/PITR/failover, major upgrade or role/RLS change heading to release | db | [db](references/db.md) |
| Same code passes and fails; intermittent CI failure | flaky | [release-ops](references/release-ops.md) §2, [stacks](references/stacks.md) |
| Release candidate go/no-go, canary, rollback readiness, incident follow-up controls | release | [release-ops](references/release-ops.md), [ops-learning](references/ops-learning.md) |
| One end-to-end claim → case → evidence example | any | [worked-example](references/worked-example.md) |

Read ≤3 references per task (shared files excluded). Modes chain: design → write/reliability/db →
flaky → release.

## Workflow

1. Read [contract](../_shared/contract.md). Record tier, scope lock, mode and environment.
   Output: header lines in the task log.
2. Write the claims table: one row per claim, `EXPECTED` (must happen) or `FORBIDDEN` (must never
   happen), source (`contract`, `doc`, `user`, `derived`), required for this candidate yes/no. A
   claim whose correct result is undefined becomes `SPECIFICATION GAP`. Output: claims table.
3. Map impact with [code-intel](../_shared/code-intel.md) §4 rows 2, 5, 6 and 7; add rows 3–4 when
   state or serialization changes. Choose boundary and cadence from scope-cadence. Output: scope
   table with required, deferred and reason.
4. Design cases per case-design, covering only the phases the claims need (normal, boundary,
   fault, recovery, change). Each case names its independent oracle and the realistic defect it
   must catch. Output: case table.
5. Bind the environment: existing runner, fixtures, CI job, test DSN, in that order. Output:
   environment line, or `BLOCKED: <missing item>` per affected claim.
6. Design mode stops here: every claim is `NOT_RUN (plan only)`.
7. Write tests into the repository's existing test directories and naming scheme; run them; save
   output to `.arc/artifacts/<task-slug>/logs/`. Output: command, exit code, log path per claim.
8. Prove sensitivity for each required claim: run the new test on the pre-fix commit in a
   disposable `git worktree`, feed a known-bad oracle input, or mutate code inside that worktree.
   It must fail on the targeted assertion. Remove the worktree. Output: fail-then-pass record.
9. Assign verdicts per Decision Rules. Output: verdict column with evidence paths.
10. Run the routed mode tail: flaky protocol, db gate groups, or release gate order. Output:
    flaky record, gate table, or GO/NO-GO.
11. Hand off: each `FAIL` becomes a repair packet (format in
    [execution-truth](../../docs/execution-truth.md) §6) for `arc:code`; each
    `SPECIFICATION GAP` goes to the user. Report per contract §9. Output: report.

## Tool Strategy

- **Preferred**: the project's own test entry (Makefile target, package script, CI job) → the
  native runner in stacks → the smallest new tool a named failure model needs (with permission;
  record name and version).
- **Query order** for tests, entry points and fixtures: code-intel §3 ladder (symbol, exact
  string, AST, file name, broadened text; host search only under its §6), starting from impact row 7.
- **When a query fails**: follow code-intel §5; after the last rung report `not found` with every
  query, and place the test beside the closest owning package. A run that fails to compile or
  start is a harness failure, not a product `FAIL`.
- **Fallback**: no E2E infrastructure → API-level contract test, E2E `NOT_RUN` with reason; no
  browser runner → `agent-browser` exploratory pass with screenshots, scripted E2E `NOT_RUN`; no
  real database or broker → keep the runnable test plus preconditions, verdict `BLOCKED`.
- **Forbidden**:
  - editing production source, including temporary mutations outside a disposable worktree;
  - expected values computed by the code under test; mocking the unit under test; mock-only proof
    of database, broker or durability semantics;
  - updating snapshots or golden files to make a run pass; weakening or deleting assertions;
    raising timeouts or retry counts to hide a failure; retry-until-green;
  - `sleep` as synchronization; waits without a deadline;
  - starting PostgreSQL servers or container runtimes on the workstation;
  - load, stress, fault injection or destructive reset on shared or third-party targets without an
    authorization naming target, operations, data scope and stop condition; production chaos;
  - aggregate quality scores, universal coverage thresholds or E2E ratios used as gates;
  - inventing domain keys, SLOs, budgets, deadlines or waivers.

## Decision Rules

- IF tier is T0 and existing tests already exercise the changed behavior THEN run them and add no test.
- IF the scope is read-only THEN assess existing claims, bindings and evidence only (no test
  writes, installs, fault injection or data-writing runs); unexecuted claims are `NOT_RUN (read-only)`.
- IF a claim concerns concurrency THEN run ≥2 parallel actors through a barrier at the racy point,
  race detector on where available; a clean `-race` run never proves database-level safety.
- IF a claim concerns idempotency THEN send the same key ≥2 times sequentially and ≥2 times
  concurrently, plus once with different parameters; assert exactly one side effect and the
  contracted mismatch response.
- IF a claim concerns a transaction THEN inject a failure between its first write and commit;
  assert no partial effect is visible.
- IF a claim touches money, inventory or quota THEN assert a conservation invariant (sum
  unchanged) during and after the run.
- IF correctness depends on PostgreSQL, broker, filesystem or network semantics THEN use the real
  dependency in the project's test environment; without it `BLOCKED`, never a mock `PASS`.
- IF the target is PostgreSQL release qualification THEN enter db mode, classify risk `DB-R0`…`DB-R6`,
  and read only the gate groups the change hits; a PostgreSQL transaction claim with no release
  question stays in reliability mode.
- IF a run fails and then passes on the same candidate and environment THEN classify with at most 3
  reruns; any mixed result is `FLAKY`, never `PASS`; continue with the flaky protocol.
- IF the sensitivity check passes on the defective version THEN the test cannot fail and is
  invalid; strengthen the assertion before giving any verdict.
- IF the oracle's observer fails (connection, permission, zero checks executed) THEN the claim is `BLOCKED`.
- IF evidence comes from another commit, config or environment THEN it is `NOT_RUN (stale)`.
- IF the parent commit already fails the test THEN record it as pre-existing; judge only the delta.
- IF a performance claim exists THEN judge it separately: candidate vs baseline under the same
  conditions, ≥10 samples (`-count=10` + `benchstat`) or ≥3 load runs; missing budget = gap;
  fast-but-wrong = `FAIL`.
- IF the expected result is undecided THEN mark `SPECIFICATION GAP`, ask only that decision, and
  continue with independent claims; never adopt current output as the expectation.
- IF verification diagnoses a failure THEN follow root-cause repair in
  [execution-truth](../../docs/execution-truth.md) §6 through "Root cause" and "Affected siblings";
  fix a confirmed test or fixture defect here; send a confirmed product defect to `arc:code` as a
  repair packet.
- IF release mode THEN a required `FORBIDDEN` claim at `FAIL` → NO-GO; a required claim at
  `BLOCKED`, `NOT_RUN` or `FLAKY` → NO-GO unless the user records a waiver (decider, scope,
  expiry); the verdict never changes. An error budget never waives money, data or permission invariants.

## Verification

| Completion claim | Evidence |
|---|---|
| Test added | test name, `file:line`, command, exit code |
| `PASS` | command run on the current commit (hash), oracle observed, log path |
| Sensitivity | failing output on the bad version, passing output on the candidate |
| `FAIL` | failing assertion output plus repair packet |
| `FLAKY` | every attempt's log and the attempt count |
| `BLOCKED` / `NOT_RUN` | missing prerequisite or deferral reason |
| GO / NO-GO | gate table listing each required claim with verdict and evidence |
| Authority kept | `git status --short` lists only tests, fixtures, test configs, `.arc/artifacts/` |

## Failure Recovery

| Failure class | Action | Retry budget | Stop condition |
|---|---|---|---|
| Environment missing (DSN, device, browser, service) | search env samples and CI config once; else `BLOCKED` naming the missing item | 1 pass | continue unaffected claims |
| Test does not compile or start | fix the test code, imports or test config | 3 attempts | `BLOCKED` with the last error |
| Baseline already failing | run the same test on the parent commit | 1 | report as pre-existing; exclude from this change's verdict |
| Assertion fails | rerun once unchanged | 1 | `FAIL` + repair packet |
| Result alternates | flaky protocol | 3 reruns to classify; ≤50 iterations to reproduce | `FLAKY` + quarantine proposal |
| Test hangs | kill at the stack timeout; capture a goroutine/thread dump | 1 | `BLOCKED` or `FAIL` with the dump |
| Observer or oracle breaks | repair the observer (test code) | 1 | `BLOCKED` for dependent claims |
| Specification unclear | one question to the user | 1 | `SPECIFICATION GAP`; continue the rest |
| Authorization missing for a shared-target or destructive step | skip that step | 0 | `BLOCKED`; user decision |

## Completion Criteria

- [ ] Contract read; tier, mode and environment recorded.
- [ ] Claims table complete: EXPECTED/FORBIDDEN, source, oracle, verdict, evidence per row.
- [ ] Every required claim has a sensitivity record or a `NOT_RUN` reason.
- [ ] New tests live in the repository's test directories, not only under `.arc/artifacts/`.
- [ ] Every command is listed and re-runnable; logs saved under `.arc/artifacts/<task-slug>/logs/`.
- [ ] Every `BLOCKED` and `NOT_RUN` carries a reason; no production source changed.
- [ ] Every `FAIL` has a repair packet addressed to `arc:code`.
- [ ] Flaky: quarantine proposal names owner, reason, repair condition, lost claim coverage.
- [ ] Release: GO or NO-GO with the gate table.
- [ ] Report delivered in the contract §9 format.
