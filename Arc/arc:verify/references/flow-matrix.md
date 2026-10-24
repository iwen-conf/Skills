# Flow Test Matrix and Ledger

Turns modeled flows ([flow-model.md](flow-model.md)) into cases, verdicts and a coverage ledger.
Case fields and oracles: [case-design.md](case-design.md) §3 and §5; concurrency and faults:
[txn-faults.md](txn-faults.md).

## 1. Rank flows by risk

Rank before designing by consequence (money, quota, data loss, cross-user or cross-tenant exposure,
irreversible external effect), likelihood (traffic, retries, concurrency, incident history),
recoverability (automatic, manual, none) and detectability (immediate, at reconciliation, never).
Severity uses contract §4. High-risk closures run before single-endpoint boundary tests: duplicate
processing, illegal core transitions, cross-tenant leaks, record and related-record mismatch,
external success with wrong local state, retry with a duplicate effect, lost async work,
unrecoverable failure, committed data whose result cannot be confirmed.

## 2. Dimensions per flow

Mark each `applies`, `n/a: <reason>` or `unknown` (a gap). `applies` needs a case on every branch
the dimension touches.

| Dim | Cases to consider | Detail |
|---|---|---|
| A normal | the common valid input; other valid combinations that change the path; step order; final state and related data | case-design §1–§3 |
| B parameters, rules | missing, malformed, boundary values; unsupported enum; rule unmet; related object missing or in the wrong state; resource of another owner | contract-relevant fields only; [api-contract-tests](api-contract-tests.md) |
| C state, lifecycle | initial, intermediate, repeat after completion, after cancel or close, illegal transition, follow-up before its prerequisite, state written but next step failed, re-run after recovery | state-transition tests |
| D transaction, consistency | failure mid-transaction, commit failure or conflict, partial write, missing related row, out-of-transaction side effect failing, query before async sync | the real boundary decides rollback versus compensation |
| E idempotency, concurrency | duplicate submit, concurrent same operation, same key reused, different key same intent, concurrent update, mutually exclusive operations, retry after timeout, response lost after commit | txn-faults §0–§3 |
| F timeout, retry, external | dependency ok, business error, system error, timeout, disconnect, succeeded but unconfirmed, retry fails again, retries exhausted, later recovery | txn-faults §6 |
| G permission | unauthenticated, unauthorized, authorized, cross-user or tenant, forged IDs, role change mid-flow, unauthorized cancel, approve or delete | zero durable change on deny |
| H compensation, recovery | compensation succeeds, fails, repeats, races the normal flow, runs after recovery or manual handling | only where implemented; missing → `gap` |

Cut combinations by risk (case-design §4). Never add a case per field without a business consequence.

## 3. Branch → case

Every branch row gets ≥1 case ID or a recorded reason. A case card is case-design §3 plus:

| Field | Content |
|---|---|
| Flow, branch | `FLOW-NNN-Nxx-Bxx` (one or more) and the terminal the run must reach |
| Pre-state | data set up, plus a snapshot of every store the flow card lists |
| Trigger | entry point, inputs or event, step order |
| EXPECTED | response, business state, exact store delta, external effect, log, event or audit when contracted |
| FORBIDDEN | any other delta, a duplicate effect, an illegal state |
| Cleanup | how this run's data and side effects are removed or isolated |
| Pass criterion | the exact assertions; never "returns 200", "no error" or "works" |
| Binding | test selector, file and command; the implementation location |

## 4. Pre/post state diff

1. Before the trigger, snapshot the stores the flow touches: rows by business key, files, queue
   contents, records in the external service's test API.
2. Run the trigger through the real entry point.
3. Snapshot again; assert the expected delta exactly and an empty delta everywhere else. Reject and
   failure branches expect an empty delta or only the contracted failure record.
4. Async ends poll the completion condition up to the project deadline, then diff.

An observer that cannot read a store makes the case `BLOCKED`.

## 5. Ledger

Terminal table `| 终点 | 结果 | 闭环 | 观测点 |`, IDs `FLOW-NNN-Txx`. Branch table, columns in order:

`| 分支 | 条件 | 去向 | 闭环 | 风险 | 用例 | 绑定 | 判定 | 证据 |`

- `去向` starts with the target ID of that branch's edge; `闭环` is a flow-model §4 kind; `风险` a
  contract §4 severity; empty cells are `—`.
- `判定`: `PASS`, `FAIL`, `FLAKY`, `BLOCKED (<reason>)` or `NOT_RUN (<reason>)`. Progress is read from
  the columns: designed = `用例` set, implemented = `绑定` set, executed = a verdict other than `NOT_RUN`.
- `PASS`, `FAIL` and `FLAKY` need case, binding and evidence (log path, or command and result) from
  the current candidate; evidence from another revision is `NOT_RUN (stale)`.
- A written test is not coverage, and a `PASS` is not "business correct": the README records
  residual risk per flow.
- `flow_ledger.py check` fails on a missing or extra branch or terminal row, a node that cannot
  reach a terminal, a decision with <2 branches or a bare yes/no condition, a target that differs
  from the chart, an illegal verdict, or a verdict without its reason or evidence. It warns on `缺口`
  terminals and Critical/High branches without `PASS`; `--json` prints the per-flow counts the README
  coverage table copies.

## 6. Layers

Keep every lower layer: unit tests for local rules (prices, transitions, parsers), integration
tests for module, database and broker collaboration, E2E through the real entry for each flow's
main path and its high-risk branches, and regression = every test bound to the affected flows.
Choose the cheapest layer that reveals each branch's defect ([test-value](test-value.md) §4); no fixed ratio.

## 7. Change impact

1. List changed files and symbols from the diff (code-intel §4 rows 1–2).
2. Find affected flows: `arc-idx search 'f:docs/07-测试质量/业务流 <file or symbol>' --format ai-md`,
   plus flows whose entry points or stores the change reaches.
3. Mark them `stale`, re-trace the changed nodes, update charts and rows; new branches enter as
   `NOT_RUN (no case)`.
4. Regression set: the bound tests of every branch in the affected flows plus new cases for changed
   branches; state selection limits per [scope-cadence](scope-cadence.md) §7.

## 8. Gaps to follow-up

| Finding | Route | Record |
|---|---|---|
| Business defect: a contract-derived assertion fails | `FAIL` + repair packet → `arc:code` | branch ID, evidence |
| Test gap: branch without case or binding | this phase, or an `arc:plan` task card when it exceeds the session | branch IDs, dimension, layer |
| Missing handling in code (`缺口` terminal) | `arc:plan` model review, or the user | node, consequence, options |
| Infrastructure limit | `BLOCKED (<missing item>)` | what unblocks it |
| Rule ambiguity | `SPECIFICATION GAP` → user | both readings and how the tests differ |
| Not automatable | manual case card: who, steps, observation | why; alternative evidence |

Each item carries impact, priority (contract §4), action and a done condition.
