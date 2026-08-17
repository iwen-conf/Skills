---
name: arc:sdlc
version: 1.0.0
description: >
  Creates current-state local task docs, pre-constraints, subtasks, and progress
  tables. Use when the user says 拆任务, 进度跟踪, 任务文档, 制定计划, 创建任务,
  or before large tracked implementation. Not the default memory for same-session
  cheap-model routing; that belongs to arc:prewalk.
---

# Task Doc Progress

## Overview

Turn large or multi-step work into persistent local docs before implementation. Docs capture the latest repository state so a later session can resume from `docs/` without rediscovering the codebase.

Task docs are the **cross-session / human / audit** memory. They are not the default vehicle for same-session multi-model cost routing. Live cheap-executor handoff belongs to `arc:prewalk`.

Separate raw requirements from executable work:

1. Put original requirements under the requirement category such as `docs/00-产品需求`.
2. Put executable plans, subtasks, and progress under the task category such as `docs/03-执行任务`.
3. Do not copy large raw requirements into task docs. Link them and restate only actionable scope, constraints, and acceptance.

Skip this structure for a single-file or obvious small edit unless the user asks for task docs.

## When to Use

- Before coding a larger feature, migration, refactor, cleanup, bug-fix campaign, or audit follow-up.
- When the user asks to 制定计划, 创建任务, 进度跟踪, 任务文档, 拆任务, or 需求整理.
- When prerequisites, non-goals, verification risk, or cross-module impact must survive across sessions.
- Not for same-session cheap-model memory. Not for a one-line typo fix.

## Intent Router

| When | Load |
|---|---|
| Start or resume large tracked work | [`modules/authoring.md`](modules/authoring.md) then [`modules/docs-architecture.md`](modules/docs-architecture.md) |
| Create numbered task dirs, indexes, or `docs/00`–`11` | [`modules/docs-architecture.md`](modules/docs-architecture.md) |
| Write README / 前置约束 / 子任务 / 进度表 | [`modules/templates.md`](modules/templates.md) |
| Change status, next action, or index links | [`modules/lifecycle.md`](modules/lifecycle.md) |
| Security/audit findings become tasks | [`references/security-audit-task-pipeline.md`](references/security-audit-task-pipeline.md) |
| Same-session multi-model routing | `arc:prewalk` |
| Need executable acceptance first | `arc:clarify` |
| Implementation after docs are current | `arc:build` / `arc:fix` / `arc:frontend` |

Load only the row that matches. Do not load templates until creating or rewriting those files.

## Red Lines

```text
NO LARGE PROJECT CODE CHANGE WITHOUT CURRENT LOCAL TASK DOCS.
NO STALE 进度跟踪表.md WHILE IMPLEMENTATION CONTINUES.
NO PLAN-POSTCARD HANDOFF AS SAME-SESSION MEMORY.
NO [x] FROM DOCS OR MATRIX ALONE.
NO SILENT SCOPE EXPANSION PAST 不做范围.
NO VAGUE SUBTASKS WITHOUT FILES, SEQUENCE, AND VERIFICATION.
```

## Execution Model: Docs vs Prewalk

| Handoff | Mechanism | Use for |
|---|---|---|
| Durable task docs (this skill) | `00-前置约束.md`, subtasks, `进度跟踪表.md` | Humans, new sessions, audit campaigns |
| Same-session multi-model (`arc:prewalk`) | Trajectory + bounded todo + first production edit | Cost/speed routing inside one agent run |

1. MUST NOT market “frontier writes a plan document, cheap model implements from the document alone” as the Arc default.
2. MUST use `arc:prewalk` when the runtime can swap models mid-session.
3. MUST keep `进度跟踪表.md` and subtask `状态：...` as the authoritative local execution state.
4. MUST update progress tracking immediately when starting, pausing, blocking, completing, or verifying a subtask.
5. MUST NOT continue implementation while `进度跟踪表.md` or the active subtask status is stale.
6. MUST NOT send a final delivery response for tracked work until progress tracking reflects the actual final state.

## Workflow

1. Inspect latest repository state, existing `docs/` indexes, affected files, tests, and call sites. Discovery only.
2. Load `modules/authoring.md` and complete the pre-implementation gate.
3. Load `modules/docs-architecture.md` to resolve or scaffold category roots. `DD` is the `docs/` category sequence (`00`–`11`).
4. Load `modules/templates.md` and write `00-前置约束.md`, the task entry, subtasks, and `进度跟踪表.md`.
5. Mark the first active subtask `[/]` in both the progress table and the subtask file.
6. During execution, load `modules/lifecycle.md` for every status change.
7. If the source is `arc:audit` / `arc:security` / multi-finding repair, load `references/security-audit-task-pipeline.md` and act as `R-task` only (docs, no production fixes).
8. After the docs gate, use `arc:prewalk` for same-session multi-model routing; use `arc:arch` before backend edits.

## Integration With Arc Skills

1. Use `arc:clarify` first if scope or acceptance criteria are unclear.
2. Use this skill before `arc:build`, `arc:fix`, `arc:frontend`, or `arc:security` changes project code for large tracked work.
3. Use `arc:audit` mode `appsec` before this skill when remediation must be driven by assets, data map, and finding cards.
4. Use `arc:docs` only for Lark sync when `.lark.json` exists. Local `docs/` still stays current.
5. If this skill and another skill conflict, keep the stricter gate.

## Quality Gates

- Requirement input is under `docs/00-产品需求` or linked from there.
- Task entry is a numbered directory `README.md` unless local history requires a loose entry.
- `00-前置约束.md` exists with 不做范围 and the completion definition from [`docs/execution-truth.md`](../../docs/execution-truth.md).
- Every concrete subtask names current files, scope, outputs, and verification.
- `进度跟踪表.md` and subtask statuses agree; `[x]` means code path + project gate + reachable behavior.
- Security-sourced work has 项目定位, 项目口径, 功能角色, Finding 索引, and per-finding subtasks.
