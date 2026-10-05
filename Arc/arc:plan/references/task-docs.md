# Task Documents

Rules for authoring, sizing and maintaining task documents. File templates:
[task-templates.md](task-templates.md). Paths, numbering and indexes: [docs-layout.md](docs-layout.md).
Status values: [contract](../../_shared/contract.md) §4. Completion, scope and domain-key gates:
[execution-truth](../../../docs/execution-truth.md) §2–§4.

## 1. When and where

- Persistent task documents: T2 work spanning sessions or people, finding campaigns, or an explicit
  user request. Single-file or obvious edits get none.
- Raw requirements stay in `docs/00-产品需求`; executable plans, leaves and progress go to
  `docs/03-执行任务`. Link the requirement; restate only actionable scope, constraints and acceptance.
- Task entries, constraints, task trees, cards, trackers and API contracts are Markdown (headings,
  links, tables, checklists). Tool configs and machine output are outside this rule.

## 2. Authoring gate

Complete in order before handing work to an executor:

1. Inspect the latest repository state: docs, local task conventions, affected files, tests, routes,
   configs, migrations, call sites.
2. Resolve the requirement and task roots; read existing indexes.
3. Assess difficulty and dependencies (§4), then build the hierarchy (§3).
4. Write `00-前置约束.md` with 不做范围, the completion definition, and the repository state used
   (time, confirmed files, commands).
5. Write the progress tracker with every planned leaf at `[ ]`.
6. Make every leaf resumable from disk (§6, §7).

Documents are stale when files, scope, assumptions, evidence, tests or status change without a doc
update; update them before work continues.

## 3. Hierarchy

| Level | Role | ID | Document |
|---|---|---|---|
| L1 business group | One functional goal or business flow; overall boundary and acceptance | `T01` | group `README.md` |
| L2 work package | One side's responsibilities and dependencies | `T01-BE`, `T01-FE`, `T01-INT`, or an accurate tag such as `T01-TOOLS` | package `README.md` |
| L3 leaf | One side, one independently verifiable outcome | `T01-BE-01` | one `.md` per leaf |

1. A group touching both ends has separate backend and frontend packages and cards; "前端/后端"
   checkboxes inside one card do not count as a split.
2. Backend owns API behavior, authorization, business rules, persistence and server verification.
   Frontend owns pages, interactions, API consumption, state presentation and client verification.
   Each leaf has its own ID, scope, acceptance, dependencies and status.
3. Parents organize work and aggregate acceptance; execution and difficulty belong to leaves.
   Parents are never counted as tasks.
4. Split by verifiable outcome and real dependency, not by equal counts, file counts or a
   one-page/one-endpoint mapping. Changes that enforce one invariant stay together.
5. Create only the sides in scope: no empty frontend package, no invented backend work. Research or
   operations work uses an accurately named package.
6. Add an `INT` package (`T01-INT-01`) only when a cross-end journey needs separate acceptance; it
   depends on the BE/FE leaves and holds no implementation.
7. Existing local numbering may stay; hierarchy, separate sides, separate cards and acceptance stay
   mandatory. Small documented work still keeps BE and FE cards separate.

## 4. Difficulty and priority

Inspect contracts, code and the verification path first. Rate change scope, business/state
complexity, contract or data risk, external dependency or uncertainty, and verification cost; the
hardest factor sets the level, and the card cites the evidence. Never infer difficulty from a title
or give every leaf the same level.

| Level | Signals | Split |
|---|---|---|
| D1 simple | Existing pattern reusable; local, clear behavior; no contract or data change; short verification | One outcome; never fragment into read-file / edit-line / run-command leaves |
| D2 medium | A few modules or a clear interface/page change; boundaries and verification known | Split by business behavior or per-side outcome; implementation and its verification in one leaf |
| D3 complex | Multiple states, permissions, concurrency, migration, public contract or many callers; many verification combinations | Split by independently acceptable capability, risk boundary and dependency; a kept D3 leaf states the indivisible invariant and its verification plan |
| D4 unexplored | Key business contract, design, external capability, or consistency/migration risk not settled | Bounded research or minimal-proof leaf first (questions, evidence artifact, exit condition); refine implementation leaves from its evidence |

- Difficulty (`D1`–`D4`) and priority (`P0`–`P3`) are separate fields; urgent work may be D1.
- Splitting a risky leaf does not remove the risk from its children.
- Each leaf records difficulty, reason, dependencies and verification burden; each package README
  records its main difficulty and why the split fits.
- A D4 leaf never reads "实现整个模块".
- Independent leaves proceed while research runs; only dependent leaves become `[!]`. Resolve
  routine facts by discovery; never ask the user to design routine implementation details.
- Reassess when a new dependency, contract change or failed verification changes difficulty; update
  cards and tracker before continuing.
- Calibration, not defaults: a filter on an existing list may be a D2 backend query-contract leaf
  plus a D1 frontend leaf reusing an existing filter component; a new cross-service approval flow
  may need a D4 backend research leaf before D3 state/consistency leaves.

| Priority | Meaning |
|---|---|
| P0 | Blocks the main flow, risks data correctness, or blocks task decomposition |
| P1 | Core functionality or high-value governance |
| P2 | Normal implementation, tests, cleanup, documentation |
| P3 | Schedulable optimization or investigation |

User-stated P4, Hold or 明确不做 items are non-goals (§8), not leaves. Finding-sourced priority comes
only from the contract §4 severity mapping.

## 5. Contracts and dependencies

1. API leaves on both sides link the same Markdown contract document and section under
   `docs/02-API记录/`, written per [api-contract.md](../../_shared/api-contract.md). Apifox is
   optional, never a prerequisite or the contract authority. A current contract is linked, not
   re-planned as a contract task.
2. Dependencies read `task ID/link + needed output + readiness condition`, never "后端完成后再做".
   No dependency → write "无".
3. A changed contract is aligned before dependent code.
4. Frontend UI/state may proceed against an aligned contract with mock data; a leaf whose acceptance
   needs real integration stays `[?]` until that integration is verified. Backend completion never
   completes a frontend leaf or the feature.
5. Mark parallel versus sequential leaves explicitly. Break dependency cycles by extracting the
   shared contract or decision into its own prerequisite.

## 6. Leaf specificity

Every leaf states, in project terms:

1. The goal (not a generic engineering label).
2. Affected files, symbols, APIs, routes, tables, data contracts, commands, tests or UI states.
3. The implementation sequence and where to stop.
4. Non-goals, invariants, compatibility constraints, known edge cases.
5. Behavior before and after, including error and empty states for API and UI leaves.
6. Acceptance: commands or manual checks with expected results.
7. Parent, owning side, priority, difficulty with evidence, upstream readiness, downstream consumers.
8. `<Context-Queries>`: exact `arc-idx files/search/symbol/ast` commands with real paths, symbols
   and `--format json` or `--format ai-md`; the author runs each once and confirms it returns the
   target. No placeholders. The executor runs them before editing.

Leaves sourced from `arc:verify` scenarios keep the rule/scenario IDs, EXPECTED/FORBIDDEN outcomes,
oracle, target environment, execution binding and required evidence; evidence status does not become
`[x]` until execution-truth §2 holds.

## 7. Writing for lower-capability executors

Every leaf must be finishable by a lower-capability model from the card alone.

1. Bound it: in-scope files and symbols, an explicit out-of-scope list, execution order, acceptance
   checks, forbidden shortcuts.
2. One leaf = one verifiable outcome. Titles such as "修复安全问题", "补测试" or "fix all security
   issues" are invalid.
3. Point at current paths from the latest repository state; the executor re-checks them before
   marking complete.
4. Name the env/branch/deploy surface and the dual-track choice whenever the project has more than
   one (execution-truth §1).
5. Write explicit "不要做" items for: compatibility shims or fallbacks, the wrong deploy surface or
   track, invented domain identity keys, and paper-over fixes (test-only changes, swallowed errors,
   weakened assertions, silent error handling, broad refactors in place of the goal).

## 8. Non-goals

1. `00-前置约束.md` and the task entry keep a visible 不做范围 list.
2. Copy user-stated non-goals verbatim: Hold, P4, 明确不做, deferred migration stages, payment
   freeze, 只读, 只写文档.
3. No leaf implements a non-goal "while we are here".
4. Newly found out-of-scope work becomes a new leaf, a new task, or a `[!]` row; never a silent
   expansion.
5. With dual tracks or several hosts, 前置约束 names the verification and packaging surface.

## 9. Status lifecycle

| Marker | Set when | Record |
|---|---|---|
| `[ ]` | Planned, not started | — |
| `[/]` | Analysis or editing started | Start date |
| `[?]` | Change done, verification pending (including mock-only frontend checks) | The pending check |
| `[x]` | execution-truth §2 holds on the intended surface | Commands and results or test names; skipped tests with reason |
| `[!]` | Needs user input, an environment or an upstream leaf | What is blocked and who must act |
| `[-]` | Dropped this phase or obsolete | Reason and restart condition |

1. Each card has `状态：` near the top; the tracker row and the card change in the same edit and
   always agree.
2. No separate completed-history table; completion notes go in the tracker's 完成说明 column.
3. Statuses are independent per side. A parent is done only when every in-scope child and the
   parent's own acceptance (including real integration when required) are done.
4. Index summary statuses are refreshed from the same leaf evidence.
5. Re-auditing status: re-read code and gates first; stale task docs are suspects. Unknown → `[?]`.
6. Before starting or resuming a leaf: re-check affected files, docs, tests and status. Repository
   changed → update 前置约束, tracker and cards first. Scope or acceptance invalidated → affected rows
   `[!]` until clarified. Then run `<Context-Queries>`, confirm dependency readiness, reassess
   difficulty.
7. Project changes during work: update the current card (files, symbols, contracts, verification
   impact); add leaves for newly necessary work under the right package; mark obsolete leaves `[-]`
   with the reason or rewrite them; update the tracker immediately.
8. Work does not continue while the tracker or the active card is stale, and tracked work gets no
   final delivery reply until the tracker shows the actual final state.

Rules 6–8 bind every executor; the task entry template carries them.

## 10. Final check

1. Requirement input is under `00-产品需求` or linked from there; the entry is a numbered directory
   `README.md` unless local history requires a loose entry.
2. `00-前置约束.md` states goals, 不做范围, boundaries, completion definition and the repository state
   the plan used.
3. The tracker lists every leaf; every card has `状态：`; statuses agree; `[x]` rows meet
   execution-truth §2.
4. Every leaf names current files, scope, outputs, acceptance, difficulty with reason, priority,
   dependency readiness and verified `<Context-Queries>`; parent and leaf metadata match the
   latest split.
5. BE and FE leaves are separate and link the same current API contract; no leaf is blocked only by
   the absence of Apifox.
6. No stale names, paths or assumptions; no duplicate top-level business-domain entries.
7. Finding-sourced work also passes the finding-campaign gate before coding.
