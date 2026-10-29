---
name: arc:plan
version: 2.6.0
description: >-
  Turns intent into written specs and checkable models without touching source code: PRD or
  blueprint (写PRD, 立项, 需求文档), clarify a vague request into a brief with acceptance criteria (需求澄清,
  验收标准), business closure models — design a lifecycle or check an existing flow for dead ends,
  missing compensation or illegal transitions (业务闭环, 状态机, 状态流转漏洞, 事件联动, 异常恢复,
  闭环检查) with the closure checker and gate (闭环闸门), Markdown API contracts (接口契约, 接口文档), tasks (拆任务, 任务文档,
  进度跟踪), and audit findings into task batches. Not for implementation (use arc:code), code review (use
  arc:audit), or Lark workspace sync (use lark).
---

# arc:plan

Turns intent into specs and checkable models an executor can verify without hidden context.

## Non-trigger

| Situation | Go to |
|---|---|
| Implement, repair, refactor or comment code; inline brief for T0/T1 work | `arc:code` |
| Design or run tests; collect verification or release evidence | `arc:verify` |
| Code findings: diff review, health, security audit, scans | `arc:audit` |
| Locate code, trace call chains, map impact | `arc:explore` |
| Create or sync Lark docs, Base rows or tasks | `lark` (project-docs) |

## Goal

Write the agreed spec with user/`file:line` traceability, scoped runnable tasks,
model check/gate results and no source/test/config edits.

## Inputs

| Input | Required | Default |
|---|---|---|
| Intent: request, idea, change goal | yes | — |
| Mode, project root | no | Intent Router; current workspace |
| Docs root and local task conventions | no | discovered `docs/`; a stricter local spec wins |
| Findings and handoff package | `campaign` only | missing → Decision Rules |
| Scope tokens (只写文档, 只读, Hold, P4, 明确不做) | no | none; copied verbatim into 不做范围 |
| `.lark.json` | no | read when present; never created |

## Intent Router

| Condition | Mode | Read |
|---|---|---|
| Idea or product needs a PRD, or an existing PRD must be refreshed | `prd` | [prd.md](references/prd.md) |
| Request lacks goal, scope, constraints or acceptance; T2 work needs a brief | `brief` | [brief.md](references/brief.md) |
| Design or change a business lifecycle: states, events, invariants, recovery (业务闭环, 状态机) | `model` | [closure-design.md](references/closure-design.md), [closure-spec-format.md](references/closure-spec-format.md) |
| Check an existing flow for lifecycle gaps, read-only (状态流转漏洞, 闭环检查) | `model` (review) | [closure-review.md](references/closure-review.md), [closure-loopholes.md](references/closure-loopholes.md) |
| Modeled behavior is about to be implemented | `model` (gate) | [closure-design.md](references/closure-design.md) §3–§4, [workflow.md](../_shared/workflow.md) §4 |
| Break work into tasks; create, resume or update task documents, statuses or the tracker | `tasks` | [task-docs.md](references/task-docs.md); [task-templates.md](references/task-templates.md) when writing files; [docs-layout.md](references/docs-layout.md) when creating a task directory |
| Create or repair `docs/00–11`, numbering or README indexes | `layout` | [docs-layout.md](references/docs-layout.md) |
| Audit or security findings become tasks | `campaign` | [finding-campaign.md](../_shared/finding-campaign.md), then the `tasks` row |

Read only the matched row. Requests spanning modes run in order: `brief` → `prd` → `model` → `tasks`.
Checker findings: [closure-rules.md](references/closure-rules.md). HTTP specs use
[api-contract.md](../_shared/api-contract.md); review-only → `arc:audit` api-design.

## Workflow

Use [graph handoffs](../_shared/workflow.md) and read-only `arc-verify handoff --to arc-plan --graph <id>`.
Only user-approved rules are normative; document changes remain docs-only.
Keep requirement/transition/scenario IDs; model PASS is not runtime PASS.

1. Read [contract.md](../_shared/contract.md) and [execution-truth.md](../../docs/execution-truth.md)
   §2, §3. Record the tier, scope tokens and a `git status --porcelain` baseline. Output: three
   lines — goal, user or consumer, success measure.
2. Read [code-intel.md](../_shared/code-intel.md), run its preflight, query the modules, routes,
   models, tables and tests the request names; read `docs/` indexes, trackers, `AGENTS.md` and
   `.lark.json` when present. Output: facts with `file:line` or a code-intel §7 query line.
3. Classify unknowns: Facts; Assumptions (labeled); Questions (blocking, ≤3, each with a default).
   List competing readings and any materially simpler path instead of picking one silently.
4. Pick the mode from the Intent Router and read that row's references. Output: the artifact drafted
   from the reference's template.
5. `model`: init only inside authorized doc scope (`python3 <skill-dir>/scripts/closure/init_docs.py
   <project>`), model from sources, run `check` then strict `gate`; review writes only
   `.arc/artifacts/<task-slug>/closure/`. `tasks`/`campaign`: every task gets goal, scope lock,
   non-goals, ordered steps, impact list, acceptance commands with expected results (or a named manual
   check), status, priority, difficulty with reason, dependencies. Output: model and results, or task
   cards each ≤1 day or ≈10 files.
6. Rewrite every unverifiable criterion ("更健壮", "优化体验", "make it robust", "works well") into a
   command, test name or observable behavior. Output: zero such phrases in acceptance sections.
7. Write files at the docs-layout paths; update the category README, task index and progress
   tracker in the same pass. Output: list of written paths.
8. Run Verification; on failure follow Failure Recovery. Output: check results.
9. Report per contract §9 and name the next owner: `arc:code` for implementation, `arc:verify` for
   test or evidence design, `lark` (project-docs) when contract §7 applies; modeled
   runtime claims go to `arc:verify`.

## Tool Strategy

- **Preferred:** code-intel commands (`arc-idx symbol`, `search`, `ast`, `files`) and direct file
  reads for facts; writes only under `docs/` and the task-document root.
- **Query order:** docs indexes, trackers and cards → code-intel ladder → project agent files and
  PRDs (项目定位, 项目口径) → the user, only for what remains.
- **When a query fails:** widen per code-intel §5, then write `not found (queries …)` as a Question
  or `[?]` item; never write an unconfirmed path, symbol or key.
- **Fallback:** `arc-idx` missing or unhealthy after one repair → host search labeled
  `fallback:<tool>` with the reason (code-intel §6).
- **Closure checker** (`<skill-dir>` holds this file):
  `python3 <skill-dir>/scripts/closure/check_closure.py check|gate|render <spec>`; self-test
  `python3 <skill-dir>/scripts/closure/selftest.py`. Needs PyYAML; missing → `BLOCKED`.
- **Forbidden:**
  - Editing source, tests, configs, migrations, CI or deploy files, including small fixes noticed
    while planning.
  - Inventing module, file, symbol, route, table or domain identity key names.
  - Model-routing or "执行模型假设" sections in any document.
  - JSON task trees or JSON API documents in place of Markdown.
  - `[x]` based on checkboxes, matrix cells, README claims or "already fixed" chat.
  - Priority ranges ("P0 或 P1") or any severity→priority mapping other than contract §4.
  - One task that mixes frontend and backend implementation.
  - Generic PRD filler or vague tasks ("实现接口", "补充测试", "优化代码") without files and acceptance.
  - New loose `.md` files at the `docs/` or task category root; suffixes such as 最终版, 新版, 临时版;
    a second progress tracker for one task.
  - Creating Lark resources or `.lark.json`.
  - `--allow-warn` as a gate, hand-edited checker reports or status, waivers without a recorded
    decision, or canonical spec edits in model review.

## Decision Rules

- IF tier is T0/T1 and the work fits one session → return an inline brief in the reply; write no
  files unless the user asks for task documents.
- IF tier is T2 in one session → write a brief before the executor's first edit. IF T2 spans
  sessions or people, comes from findings, or the user asks → persistent task documents.
- IF an unknown survives step 2 → ask only when it touches an interface, persisted data, an
  architecture trade-off, or blocks every reading; bug fixes and single-answer work proceed without
  questions. At most 3 questions per round, each with a recommended default.
- IF the user does not answer → proceed on the defaults, list them at the top of the document as
  assumptions, and mark tasks that depend on them `[?]`.
- IF a requirement conflicts with code, docs or another requirement → record both sources under
  待确认假设 (PRD: 待确认问题) and ask. Resolve opposite defaults by instruction hierarchy → user
  scope → owning contract; combine only compatible safety requirements.
- IF a task exceeds 1 day, ≈10 files, or holds two independently verifiable outcomes → split it;
  keep changes that enforce one invariant together and state why.
- IF difficulty is D4 → first a bounded research task (questions, evidence, exit condition);
  only dependent tasks `[!]`.
- IF frontend and backend are both in scope → separate work packages and cards; add an integration
  task only for a separate joint acceptance outcome.
- IF the repository changed since the docs were written → redo step 2 and rewrite affected cards and
  the tracker before handoff; invalidated scope or acceptance → `[!]` plus a Question.
- IF items are Hold, P4 or 明确不做 → copy verbatim into 不做范围; their tasks become `[-]` with a
  restart condition.
- IF the request only asks for a PRD or task split → do not model an unrelated lifecycle; IF a
  current model covers the behavior → reuse it and check freshness.
- IF the spec or checker changed → rerun `check` and `gate` before any handoff; IF errors, unwaived
  warnings or truncation remain → gate `FAIL`, and the affected behavior gets no implementation task.
- IF a waiver or assumption changes business semantics → ask; IF concurrency, ordering, durability or
  database behavior matters → register a runtime claim for `arc:verify`.

## Verification

| Claim | Evidence |
|---|---|
| Acceptance is executable | Each card has a command with expected result, a test name, or a manual check naming who, steps and expected outcome |
| Requirements trace | Each goal or requirement line cites a user quote or `file:line` |
| Names are real | `test -e <path>` or a code-intel hit per named path or symbol; misses are Questions |
| Links resolve | Link script below prints nothing |
| Status vocabulary legal | Status grep below prints nothing; every `[x]` row cites a command result, test or `file:line` |
| Tracker matches cards | Every leaf appears once in the tracker with the status of its card's `状态：` line |
| Model current | `check` + `gate` commands, zero unwaived problems, matching spec and checker fingerprints |
| Authority respected | `git status --porcelain` differs from the baseline only under `docs/` or the task-document root |

```bash
python3 -c "import re,sys,pathlib as p
for f in p.Path(sys.argv[1]).rglob('*.md'):
  for t in re.findall(r'\]\(([^)#\s]+)',f.read_text()):
    if '://' not in t and not (f.parent/t).exists(): print(f,t)" <written-docs-dir>
rg -n '`\[[^ /?x!-]\]`' <written-docs-dir>
```

## Failure Recovery

| Failure | Action | Retry budget | Stop condition |
|---|---|---|---|
| Blocking question unanswered | Continue on top-listed defaults; dependents `[?]` | 1 round | Default touches an interface, data, auth or money → `[!]`, stop |
| `arc-idx` unhealthy | One repair per code-intel §1, then labeled fallback | 1 repair | Target unreadable → facts become Questions |
| Named file or symbol not found | Widen per code-intel §5 | 5 rungs | Report `not found` with every query; never guess |
| Doc structure conflicts with defaults | Keep existing structure; add missing indexes only | 0 | User asks to reorganize → `layout` mode |
| Campaign handoff incomplete | One discovery pass for 项目定位 and 项目口径 | 1 pass | Still missing → `[!]` on dependent tasks; hand back to `arc:audit` |
| Verification check fails | Fix the documents; re-run all checks | 2 rounds | Still failing → list items under Not verified |
| Requirement conflict unresolved | Record under 待确认假设 and ask | 1 round | No answer → affected tasks `[!]` |
| Spec error or model counterexample | Fix from the source rule, or record the decision | 2 passes | Report errors; block dependent tasks |
| Enumeration truncated | Narrow modeling scope | 1 | `BLOCKED`; never waive completeness |
| Work needs code, test or config edits | Finish docs, hand off | 0 | Immediately |

## Completion Criteria

- [ ] Contract read; tier, scope tokens and `git status` baseline recorded.
- [ ] Artifact written at its layout path, or an inline brief returned for T0/T1 work.
- [ ] Every requirement traces to the user's words or a cited `file:line`.
- [ ] Every task has goal, scope lock, non-goals, steps, impact, acceptance command or named manual
      check, status, priority, difficulty with reason, and dependencies.
- [ ] Model mode: check and gate results current; loopholes, assumptions and waivers dispositioned.
- [ ] Unknowns are Questions (≤3, each with a default), labeled assumptions, or `[?]`.
- [ ] Category README, task index and progress tracker updated; link and status checks print nothing.
- [ ] No change outside `docs/` or the task-document root versus the baseline.
- [ ] Report per contract §9 names the next owner.
