---
name: arc:plan
version: 2.0.0
description: >-
  Turns intent into executable written specs without touching source code: write a PRD or
  blueprint (写PRD, 立项, 需求文档), clarify a vague request (需求澄清, clarify) into a brief with
  acceptance criteria (验收标准), break work into tasks (拆任务), create or maintain docs/00-11
  task documents (任务文档) and progress tracking (进度跟踪), or turn audit findings into task
  batches. Not for implementation (use arc:code) or Lark workspace sync (use
  lark-workflow-project-docs).
---

# arc:plan

Turns intent into written specs (PRD, brief, task documents, progress tracker) that an executor can
verify without hidden context.

## Trigger

- Write or refresh a PRD or project definition: 写PRD, 需求文档, 立项, 产品定义, blueprint.
- Clarify a vague request into a brief with acceptance criteria: 需求澄清, 需求不清, 验收标准, clarify.
- Break work into tasks; create or update task documents: 拆任务, 制定计划, 创建任务, 任务文档, 需求整理.
- Update task status and the progress tracker: 进度跟踪, 更新进度表, 任务状态.
- Create or repair the `docs/00–11` structure and README indexes: 文档结构, 整理文档目录, 建索引.
- Turn audit or security findings into task batches: 把审计/扫描结果拆成任务, 安全修复战役, 审计跟进.
- Plan multi-session or multi-person T2 work before its first edit.

## Non-trigger

| Situation | Go to |
|---|---|
| Implement, repair, refactor or comment code; inline brief for T0/T1 work | `arc:code` |
| Design or run tests; collect verification or release evidence | `arc:verify` |
| Produce findings: diff review, health check, security audit, scans | `arc:audit` |
| Locate code, trace call chains, map impact | `arc:explore` |
| Create or sync Lark docs, Base rows or tasks | `lark-workflow-project-docs` |

## Goal

The requested spec exists at its agreed path; every requirement traces to the user's words or a cited
`file:line`; every task carries scope, non-goals and copy-runnable acceptance checks; no source, test
or config file changed.

## Inputs

| Input | Required | Default |
|---|---|---|
| Intent: request, idea, change goal | yes | — |
| Mode | no | chosen by the Intent Router |
| Project root | no | current workspace |
| Docs root and local task conventions | no | discovered `docs/`; a stricter local spec wins |
| Findings and handoff package | `campaign` only | missing → Decision Rules |
| Scope tokens (只写文档, 只读, Hold, P4, 明确不做) | no | none; copied verbatim into 不做范围 |
| `.lark.json` | no | read when present; never created |

## Intent Router

| Condition | Mode | Read |
|---|---|---|
| Idea or product needs a PRD, or an existing PRD must be refreshed | `prd` | [prd.md](references/prd.md) |
| Request lacks goal, scope, constraints or acceptance; T2 work needs a brief | `brief` | [brief.md](references/brief.md) |
| Break work into tasks; create, resume or update task documents, statuses or the tracker | `tasks` | [task-docs.md](references/task-docs.md); [task-templates.md](references/task-templates.md) when writing files; [docs-layout.md](references/docs-layout.md) when creating a task directory |
| Create or repair `docs/00–11`, numbering or README indexes | `layout` | [docs-layout.md](references/docs-layout.md) |
| Audit or security findings become tasks | `campaign` | [finding-campaign.md](../_shared/finding-campaign.md), then the `tasks` row |

Read only the matched row. Requests spanning modes run them in order: `brief` → `prd` → `tasks`.

## Workflow

1. Read [contract.md](../_shared/contract.md) and [execution-truth.md](../../docs/execution-truth.md)
   §2, §3, §8. Record the tier, scope tokens and a `git status --porcelain` baseline. Output: three
   lines — goal, user or consumer, success measure.
2. Read [code-intel.md](../_shared/code-intel.md), run its preflight, then query the modules,
   routes, models, tables and tests the request names. Read existing `docs/` indexes, progress
   trackers, `AGENTS.md`, `README.md` and `.lark.json` when present. Output: fact list, each fact
   with `file:line` or a query line in code-intel §7 format.
3. Classify unknowns. Output: Facts; Assumptions (labeled, non-blocking); Questions (blocking, at
   most 3, each with a recommended default). List competing readings of the request and any
   materially simpler path instead of picking one silently.
4. Pick the mode from the Intent Router and read that row's references. Output: the artifact drafted
   from the reference's template.
5. `tasks` and `campaign`: give every executable task a goal, scope lock (files, directories,
   symbols), non-goals, ordered steps, impact list, acceptance commands with expected results or a
   named manual check, status, priority, difficulty with reason, and dependencies with readiness
   conditions. Output: task cards, each ≤1 day or ≈10 files.
6. Rewrite every unverifiable criterion ("更健壮", "优化体验", "make it robust", "works well") into a
   command, test name or observable behavior. Output: zero such phrases in acceptance sections.
7. Write files at the docs-layout paths; update the category README, task index and progress
   tracker in the same pass. Output: list of written paths.
8. Run Verification; on failure follow Failure Recovery. Output: check results.
9. Report per contract §9 and name the next owner: `arc:code` for implementation, `arc:verify` for
   test or evidence design, `lark-workflow-project-docs` when contract §7 applies.

## Tool Strategy

- **Preferred:** code-intel commands (`arc-idx symbol`, `search`, `ast`, `files`) and direct file
  reads for facts; writes only under `docs/` and the task-document root.
- **Query order:** (1) existing docs indexes, trackers and task cards; (2) the code-intel ladder for
  symbols, routes, models and config keys; (3) project agent files, PRDs and architecture docs for
  项目定位 and 项目口径; (4) the user, only for what (1)–(3) cannot answer.
- **When a query fails:** widen per code-intel §5; after the last rung write
  `not found (queries …)` and turn the fact into a Question or a `[?]` item. Never write an
  unconfirmed path, symbol or key into a document.
- **Fallback:** `arc-idx` missing or unhealthy after one repair → host search labeled
  `fallback:<tool>` with the reason (code-intel §6).
- **Forbidden:**
  - Editing source, tests, configs, migrations, CI or deploy files, including small fixes noticed
    while planning.
  - Inventing module, file, symbol, route, table or domain identity key names.
  - Model-routing or "执行模型假设" sections in any document.
  - JSON task trees or JSON API documents in place of Markdown.
  - `[x]` based on checkboxes, matrix cells, README claims or "already fixed" chat.
  - Priority ranges ("P0 或 P1") or any severity→priority mapping other than contract §4.
  - One task that mixes frontend and backend implementation.
  - Generic PRD filler, another project's 项目口径, or vague tasks ("实现接口", "补充测试",
    "修复安全问题", "优化代码") without files, behavior and acceptance.
  - New loose `.md` files at the `docs/` or task category root; suffixes such as 最终版, 新版, 临时版;
    a second progress tracker for one task.
  - Creating Lark resources or `.lark.json`.

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
- IF difficulty is D4 → write a bounded research task (questions, evidence artifact, exit
  condition) first; mark only dependent tasks `[!]`.
- IF frontend and backend are both in scope → separate work packages and cards; add an integration
  task only for a separate joint acceptance outcome.
- IF acceptance needs a real integration that does not exist yet → the card states it stays `[?]`
  after mock checks.
- IF the source is audit findings → priority from contract §4 only; handoff fields, status handling
  and dedup follow finding-campaign.
- IF the handoff lacks 项目定位, 项目口径 or finding cards → one discovery pass over project agent
  files; still missing → mark dependent tasks `[!]` and report to `arc:audit`.
- IF the repository has its own doc structure or stricter task spec → keep it; add only missing
  indexes and required files.
- IF the repository changed since the docs were written → redo step 2 and rewrite affected cards and
  the tracker before handoff; invalidated scope or acceptance → `[!]` plus a Question.
- IF the user marks items Hold, P4 or 明确不做 → copy them verbatim into 不做范围; existing tasks for
  them become `[-]` with a restart condition.
- IF the request needs a code, test or config change → finish the docs phase, report, and hand off
  to `arc:code` or `arc:verify`.

## Verification

| Claim | Evidence |
|---|---|
| Acceptance is executable | Each card has a command with expected result, a test name, or a manual check naming who, steps and expected outcome |
| Requirements trace | Each goal or requirement line cites a user quote or `file:line` |
| Names are real | `test -e <path>` or a code-intel hit per named path or symbol; misses are Questions |
| Links resolve | Link script below prints nothing |
| Status vocabulary legal | Status grep below prints nothing; every `[x]` row cites a command result, test or `file:line` |
| Tracker matches cards | Every leaf appears once in the tracker with the status of its card's `状态：` line |
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
| Blocking question unanswered | Continue on recommended defaults; list them at the top; dependent tasks `[?]` | 1 follow-up round | Default changes an interface, persisted data, auth or money → mark `[!]`, report, stop |
| `arc-idx` unhealthy | One repair per code-intel §1, then labeled fallback | 1 repair | Target unreadable → facts become Questions |
| Named file or symbol not found | Widen per code-intel §5 | 5 rungs | Report `not found` with every query; never guess |
| Doc structure conflicts with defaults | Keep existing structure; add missing indexes only | 0 | User asks to reorganize → `layout` mode |
| Campaign handoff incomplete | One discovery pass for 项目定位 and 项目口径 | 1 pass | Still missing → `[!]` on dependent tasks; hand back to `arc:audit` |
| Verification check fails | Fix the documents; re-run all checks | 2 rounds | Still failing → list items under Not verified |
| Requirement conflict unresolved | Record under 待确认假设 and ask | 1 round | No answer → affected tasks `[!]` |
| Work needs code, test or config edits | Finish docs, hand off | 0 | Immediately |

## Completion Criteria

- [ ] Contract read; tier, scope tokens and `git status` baseline recorded.
- [ ] Artifact written at its layout path, or an inline brief returned for T0/T1 work.
- [ ] Every requirement traces to the user's words or a cited `file:line`.
- [ ] Every task has goal, scope lock, non-goals, steps, impact, acceptance command or named manual
      check, status, priority, difficulty with reason, and dependencies.
- [ ] Unknowns are Questions (≤3, each with a default), labeled assumptions, or `[?]`.
- [ ] Category README, task index and progress tracker updated; link and status checks print nothing.
- [ ] No change outside `docs/` or the task-document root versus the baseline.
- [ ] Report per contract §9 names the next owner.
