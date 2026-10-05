---
name: arc:code
version: 2.0.0
description: >-
  Every task that changes source code: write code to implement (写代码, 实现, 开发功能, 落地改动)
  a feature or endpoint; fix (修复) a bug, error (报错), incident or failing test by evidence-first
  root-cause repair (根因); refactor (重构) or migrate; change an API or schema; build frontend
  pages (前端, shadcn, 小程序); apply Go backend layering (DIP, usecase, 分层); write comments (注释);
  or add timing probes to a Gin SSR path (计时, 探针, 埋点). Ends with the gate.sh completion check.
  Not for read-only review (use arc:audit), test design or release evidence (use arc:verify), or
  PRDs and task docs (use arc:plan).
---

# arc:code

Change source code so the requested behavior holds on its real entry point, with the smallest complete diff and gate evidence.

## Trigger

- Implement a feature, endpoint, job or behavior change.
- Fix a bug, error, crash, regression, incident or failing test.
- Refactor, rename, move or migrate code, libraries, APIs or data.
- Change an API, DTO or database schema.
- Build frontend pages, components, state or themes.
- Add or correct code comments.
- Add timing probes to a slow request path.

## Non-trigger

- Explain, locate or map code without editing → `arc:explore`.
- Review a diff, PR, project health or security without editing → `arc:audit`.
- Tests are the deliverable, test plans, reliability or PostgreSQL qualification, flaky-test triage, release go/no-go → `arc:verify`.
- PRD, requirement brief, task documents, progress tracking → `arc:plan`.
- Lark documents, tables or project status sync → Lark family (`lark-workflow-project-docs`).
- Frontend web-performance audits → `web-perf`.

## Goal

The requested behavior works through its real entry point (HTTP, CLI, UI, job), every impacted site and sibling path is updated, and `gate.sh check` reports no `FAIL`.

## Inputs

| Input | Required | Default |
|---|---|---|
| Request | yes | — |
| Task document path | no | none; T2 gets an inline brief |
| Failure evidence (error, log, failing test, request ID) | repair | collected in repair step 1 |
| Scope lock (paths, symbols, execution-truth §3 token) | no | the impact list |
| Task slug | no | kebab-case request summary, ≤5 words |

## Intent Router

| Condition | Mode | Read |
|---|---|---|
| New feature, endpoint, job or behavior from a known requirement | implement | the "adds to mode" rows below |
| Bug, error, crash, failing test, regression, incident, failure-justified optimization | repair | [repair.md](references/repair.md) + evidence-first root-cause repair in [execution-truth.md](../../docs/execution-truth.md) §6 |
| Refactor, rename, move, migrate; shape change of an API, DTO, schema, event or config key | refactor | [refactor.md](references/refactor.md) |
| Web, mobile, desktop or mini-program page, component, state or theme | frontend | [frontend.md](../_shared/conventions/frontend.md); colors and theme: [frontend-tokens.md](../_shared/conventions/frontend-tokens.md) |
| Add or correct code comments only | comment | [comments.md](../_shared/conventions/comments.md) |
| Timing probes on a slow request path | probe | [perf-probes.md](references/perf-probes.md) |
| Any mode editing Go backend code | adds to mode | [go-backend.md](../_shared/conventions/go-backend.md) + [go-type-safety.md](../_shared/conventions/go-type-safety.md) |
| Any mode adding or changing an API | adds to mode | [api-contract.md](../_shared/api-contract.md) |
| Any mode writing SQL, transactions, migrations or state transitions | adds to mode | [code-rot.md](../_shared/code-rot.md) families D and E |

## Workflow

1. Read [contract.md](../_shared/contract.md), [code-intel.md](../_shared/code-intel.md) and [execution-truth.md](../../docs/execution-truth.md) §8–§10. Write one line: `Mode · Tier · Scope lock · Task slug`. IF T2 and no task document THEN write an inline brief of ≤10 lines (goal, scope, non-goals, acceptance commands) before any edit.
2. Run the code-intel preflight. For T1/T2 run `bash <skill-dir>/scripts/gate.sh start --task <slug>` before the first edit; `<skill-dir>` holds this file and [gate.sh](scripts/gate.sh). Output: baseline recorded.
3. Read only the matching Intent Router rows. Output: the list of files read.
4. Map impact. For every symbol whose signature or meaning changes, fill all 8 rows of the code-intel §4 checklist and write the impact list (`file:line`) before editing. For pure additions, record the owning package and ≥1 analogous path to reuse.
5. Prepare by mode:
   - implement: write or update the Markdown contract section before dependent code when an API is involved.
   - repair: run [repair.md](references/repair.md) steps 1–6; no production edit before a failing check, a confirmed root cause and the sibling list exist.
   - refactor: run [refactor.md](references/refactor.md) §1–§3: green baseline, characterization tests, step plan.
   - frontend: record the user workflow, platform target and the loading / empty / error / permission-denied states.
   - comment: pick the comment template per code role.
   - probe: record the baseline measurement.
6. Edit in order: contract and types → implementation → callers → tests → docs. Close every impact row as `changed file:line` or `not affected: <reason>`.
7. Add tests with the change: every changed behavior gets ≥1 test that fails on the old code; the repair reproducer goes fail → pass.
8. Run `bash <skill-dir>/scripts/gate.sh check --task <slug> --max-files <N>` (N = impact-list files plus their tests); add `--strict` for T2 and `--scope <file>` (one glob per line) under a scope lock. Cite the report path it prints. T0 without `start`: `check` diffs against HEAD, so pre-existing dirty files count as changed.
9. Exercise the real entry point: HTTP call, CLI run, UI action, job trigger, or a test through the handler. A helper-only test does not prove reachability.
10. Update task-document status with contract vocabulary, apply the contract Lark hook, and report in the contract §9 format; repair mode adds the execution-truth §6 repair packet.

## Tool Strategy

- **Query order:** code-intel §3 ladder: exact symbol → exact string → AST → file name → broadened text → host search.
- **Preferred:** `arc-idx`; the project's own format, lint, typecheck and test commands; `gate.sh`; the project's structured logger; language-aware rename/move tools (gopls rename, IDE refactor).
- **Forbidden:**
  - `--no-verify`, disabling lint rules, skipping or deleting tests to get green.
  - Suppressions: `//nolint`, `eslint-disable`, `@ts-ignore`, `# type: ignore`, `pyright: ignore`, a new `any` or `interface{}` that silences a type error.
  - Swallowed errors, retries or sleeps hiding a race, weakened assertions, the execution-truth §5 paper-overs.
  - Repository-wide mutating formatters (`go fmt ./...`, `prettier --write .`); format changed files only.
  - Unreviewed tree-wide rewrites (`sed -i`, `perl -i`, `find … -exec sed`, `xargs sed`).
  - Hand-editing generated output instead of its generator input.
  - Invented config keys, table or column names, enum values, routes, error codes or domain identity keys.
  - Editing an already-applied migration.
  - Reverting, stashing or reformatting user changes recorded in the baseline.
- **Fallback:** `arc-idx` unhealthy after one repair → host search labelled `fallback:<tool>`. `gate.sh` missing or erroring → run its checks by hand (placeholder and suppression scan of changed lines, non-mutating format check, lint/typecheck per stack, affected tests) and record each verdict. A check the project lacks → `NOT_RUN(<reason>)`, never `PASS`.
- **When a query fails:** follow code-intel §5; after the last rung report `not found` with every query run; never guess a path, key or enum. Unresolved dynamic dispatch → `dynamic — confirm at runtime`, covered by a real-entry test.

## Decision Rules

- IF failure evidence exists (error text, failing test, incident) THEN use repair mode, even when the request says "implement".
- IF the work has ≥3 steps THEN keep a numbered step list with status; re-plan when a step fails twice or the impact list grows.
- IF a request mixes refactor and behavior change THEN finish the refactor steps with tests green first, then change behavior in separate steps.
- IF a function signature changes THEN update every caller from impact row 2 in the same change; IF >30 call sites THEN stop and propose an adapter or deprecation wrapper with a staged migration ([refactor.md](references/refactor.md)).
- IF a public struct or DTO field changes THEN impact rows 3–5 cover constructors, reads, JSON tags, API doc, client types and migrations; a rename uses expand–migrate–contract unless every consumer is in this repository and changes in this diff.
- IF an API changes THEN change the server handler and DTO, client calls and types, the Markdown contract, tests and migration notes together; stay backward compatible unless the task explicitly allows a breaking change.
- IF the schema changes THEN add a new migration with up and down, model, queries, indexes and a backfill plan.
- IF writing SQL, a transaction, a migration or a state transition THEN, before writing, check code-rot families D and E (soft-delete filters, N+1, bounded lists, hot-predicate indexes, JSONB on hot paths, primary keys; status constants, declared state transitions, no swallowed errors, concurrency guards) and record how each applicable item is met.
- IF code writes data THEN verify business success (affected rows or returned id) and guard concurrent transitions with a transaction, optimistic lock or state-encoded `WHERE`.
- IF a list or search has no rows THEN return success with an empty collection (pagination metadata only for paginated contracts); keep validation, unauthenticated, permission, not-found, conflict, network/server and processing outcomes distinct.
- IF a value is user-controlled (ownership, role, price or amount, identifier, SQL fragment, sort field) THEN validate it server-side.
- IF a domain value is absent from code, contracts and user documents THEN mark it `to confirm`, block the dependent code and ask.
- IF the repository has an established local pattern that differs from a convention THEN follow the local pattern unless the task asks to migrate; new isolated modules use the convention defaults.
- IF the owning fix lies outside the scope lock THEN stop and report the files needed.
- IF a new dependency seems needed THEN state why the standard library and installed dependencies fail, and get permission before installing.
- IF `gate.sh` lists more changed files than `--max-files` THEN apply the execution-truth §10 traceability test to each extra file and undo own unrelated edits.
- IF comment mode THEN change comments only: no renames, no logic; list vague names as suggestions.
- IF probe mode THEN make one optimization per measurement; before completion remove probes, `console.log`, `debugger` and alert probes, or keep them behind a log level when the task asks to keep them.
- IF the required evidence and gates pass THEN stop adding tests; further test design goes to `arc:verify`.

## Verification

| Claim | Evidence |
|---|---|
| Behavior works | real-entry command or test name with output |
| Impact handled | each impact row closed with `file:line` or a reason |
| Bug fixed | same check: `FAIL` before, `PASS` after |
| Refactor preserves behavior | same tests green before and after, assertions unchanged |
| Gates pass | `gate.sh` lines; each `NOT_RUN` with reason |
| Contract synced | contract doc section plus changed server and client files |
| Probe result | baseline and after numbers plus one emitted JSON line |

## Failure Recovery

| Failure | Action | Budget | Stop |
|---|---|---|---|
| `gate.sh` FAIL on changed lines | Fix the cause; never suppress | 3 attempts per check | Report output and attempts |
| FAIL outside the change set | IF the failing file is unchanged and its trace names no changed symbol THEN it is pre-existing | 0 fixes | Report it |
| Test fails after the change | Edit a test only when it encodes an old contract this task intentionally changes; say why | 3 attempts | Report the conflict |
| Same hypothesis fails twice | Back to repair step 3 with new evidence and ≥1 new hypothesis | 3rd failure | Report falsified hypotheses and next experiment |
| Cannot reproduce | Bounded investigation probes; no speculative fix | 2 probe rounds | Report `hypothesis only` and the data needed |
| Build broken mid-edit | Fix forward or undo own last step only | 2 attempts | Report the broken step |
| Index unhealthy | Repair once (code-intel §1) | 1 | Labelled fallback |
| Toolchain missing | `NOT_RUN(<missing tool>)`; install only with permission | 0 | Report |
| Flaky check | ≤5 identical runs, record counts, verdict `FLAKY`; product race → repair; test or environment cause → `arc:verify` | 5 runs | Hand off with run log |
| Exceeds authority or scope | Finish the phase | 0 | Report and hand off |

## Completion Criteria

- [ ] `Mode · Tier · Scope lock · Task slug` line written; T2 has a brief or task document.
- [ ] Every impact row closed with `file:line` or a reason.
- [ ] Each changed behavior has a test that fails on the old code; repair shows fail → pass.
- [ ] `gate.sh check` shows no `FAIL` and every `NOT_RUN` has a reason (T0: format, lint and test commands cited).
- [ ] Real entry point exercised, or `BLOCKED` with the reason.
- [ ] API contract, client, docs and migrations synced wherever touched.
- [ ] No placeholders, suppressions, debug output or unrequested probes remain.
- [ ] Task-document status updated; Lark status handed off or its blocker stated when `.lark.json` exists.
- [ ] Report written in the contract §9 format, plus the repair packet in repair mode.
