---
name: arc:task-doc-progress-conventions
description: Create and update current-state task docs, pre-constraints, detailed subtasks, and progress tables for large work.
---

# Task Doc Progress Conventions

## Overview

Use this skill to turn large or multi-step work into persistent task documents before implementation. The task docs must reflect the latest repository state at the time they are created or updated, so the next agent or later session can continue from `docs/` without rereading the whole codebase.

For small one-shot fixes, do not create this structure unless the user asks for task docs.

## When to Use

- Use before coding when the request is a larger feature, migration, refactor, cleanup, bug-fix campaign, or any task likely to span multiple implementation passes.
- Use when the user asks to "制定计划", "创建任务", "进度跟踪", "任务文档", "拆任务", or says future larger tasks must follow this convention.
- Use when a task has meaningful prerequisites, non-goals, verification risk, cross-module impact, or open questions that should be captured before implementation.
- Skip for a single-file or obvious small edit unless it needs cross-session tracking.

## Pre-Implementation Gate

Before changing production code for a large task:

1. Inspect the latest repository state, including current files, existing docs, local conventions, tests, routes, configs, and affected call sites.
2. Create or update the task document structure under `docs/`.
3. Write `00-前置约束.md` before implementation.
4. Write `进度跟踪表.md` with every planned subtask.
5. Make every concrete subtask detailed enough for one implementation pass without hidden context.
6. Mark the first active subtask as `[/]` in both the progress table and the subtask file.

Only discovery commands, code reading, and task-document edits are allowed before this gate is complete.

Task docs are stale if they are not updated immediately when project files, scope, assumptions, or status change.

## Progress Tracking Hard Gate

`进度跟踪表.md` and subtask `状态：...` are the authoritative local execution state for large or tracked work.

Hard rules:

1. MUST update progress tracking immediately when starting, pausing, blocking, completing, or verifying a subtask.
2. MUST update progress tracking immediately when the next action changes because project files, scope, assumptions, evidence, tests, or user decisions changed.
3. MUST NOT continue implementation while `进度跟踪表.md` or the active subtask status is stale.
4. MUST NOT send a final delivery response for tracked work until progress tracking reflects the actual final state.
5. If verification is skipped or blocked, the progress table completion note MUST record the reason and the next required action.

## Integration With Arc Skills

When another Arc skill is active, use this skill as the local task-planning gate for large or tracked work:

1. Use `arc:clarify` first if scope, acceptance criteria, or user decisions are unclear.
2. Use this skill before `arc:build`, `arc:fix`, `arc:frontend`, or `arc:security` changes project code for large, multi-step, cross-module, or tracked work.
3. Use `arc:project-architecture-conventions` after this skill and before backend code edits.
4. Use `arc:frontend` after this skill for frontend platform, UI state, token, and verification constraints.
5. Use `arc:security` after this skill for multi-finding remediation plans or security-sensitive implementation.
6. Use `arc:docs` only for Lark synchronization when `.lark.json` exists or the user explicitly enables Lark. Local `docs/` task state still needs to stay current.

If this skill and another skill conflict, keep the stricter gate: do not start implementation until the local task docs, status, and required upstream clarification are current.

## Directory Structure

Resolve the task document root from the project, do not hardcode a fixed `docs/` child path.

Rules for `docs/`:

1. Treat immediate children of `docs/` as numbered document categories when they match `DD-分类名`, for example `00-任务` or `01-需求`.
2. Use the existing task category directory when one exists, such as `DD-任务`.
3. If `docs/` exists but has no task category, create `DD-任务` using the next available two-digit category number under `docs/`.
4. If `docs/` does not exist, create `docs/00-任务`.
5. Follow a project's existing category vocabulary if it uses a clear equivalent of "任务"; do not create a duplicate task category with a different number or name.

Default structure for a complex task:

```text
docs/DD-任务/
  NN-具体任务名.md
  NN-具体任务名/
    00-前置约束.md
    进度跟踪表.md
    tasks/
      T01-任务组名称/
        README.md
        T01-01-子任务名称.md
        T01-02-子任务名称.md
```

Rules:

- Use two digits for `DD`, task `NN`, `TNN`, and subtask `MM`.
- `DD` is the `docs/` category sequence; task `NN` is the sequence inside the resolved task category.
- Pick the next available `NN` by scanning existing task entries.
- Do not use suffixes such as "最终版", "新版", "临时版", or duplicate progress tables.
- For simple tracked tasks, a single `NN-具体任务名.md` is acceptable, but it must include goal, scope, status, and acceptance criteria.
- If the repository already has a stricter local task-document spec, obey the local spec while keeping the pre-constraint and progress-table requirements from this skill.

## Status Markers

Use these exact status markers:

| 状态 | 含义 | 使用时机 |
| --- | --- | --- |
| `[ ]` | 未开始 | 尚未分析或编码 |
| `[/]` | 进行中 | 正在分析、编码或整理 |
| `[?]` | 待验证 | 已完成改动，等待测试或人工确认 |
| `[x]` | 已完成 | 已满足验收标准 |
| `[!]` | 阻塞 | 需要用户输入、外部环境或前置任务 |
| `[-]` | 暂缓 | 明确本阶段不做 |

Keep statuses synchronized:

- Each subtask file must have `状态：...` near the top.
- `进度跟踪表.md` and each subtask file must agree.
- Do not create a separate completed-history table. Put completion notes in the central progress table.

## Required Files

### Task Entry

`NN-具体任务名.md` is the entry document. Include:

1. Task summary.
2. Usage instructions.
3. Design boundaries or implementation constraints.
4. Task group index.
5. Links to `NN-具体任务名/00-前置约束.md` and `NN-具体任务名/进度跟踪表.md`.

Template:

```markdown
# 具体任务名

本文是任务入口。具体任务和进度在 `NN-具体任务名/` 目录。

## 使用方式

1. 先看 `NN-具体任务名/00-前置约束.md`。
2. 再看 `NN-具体任务名/进度跟踪表.md`。
3. 只打开当前要做的子任务文件。
4. 完成后同步更新进度表和子任务状态。

## 任务索引

| 编号 | 任务组 | 优先级 |
| --- | --- | --- |
| T01 | [任务组名称](NN-具体任务名/tasks/T01-任务组名称/README.md) | P0 |
```

### Pre-Constraints

`00-前置约束.md` is mandatory before coding. Include:

1. 目标 and non-goals.
2. Scope boundaries and affected modules.
3. Latest project-state snapshot used to generate the task.
4. Assumptions that still need verification.
5. Existing local conventions that must not be broken.
6. Data, compatibility, migration, security, and performance constraints when relevant.
7. Verification commands or manual checks expected before marking work done.
8. Blockers and required user decisions.

Template:

```markdown
# 前置约束

## 目标

1. ...

## 不做范围

1. ...

## 边界约束

1. ...

## 已知影响范围

1. `path/to/file`

## 最新项目状态

1. 调研时间：YYYY-MM-DD HH:MM。
2. 已确认文件/模块：`path/to/file`
3. 已确认测试/命令：`command`
4. 本次计划基于以上状态生成；项目变动后必须回写本文件和进度跟踪表。

## 待确认假设

1. ...

## 验证要求

1. ...

## 阻塞项

1. 无。
```

### Task Group README

Each `tasks/TNN-任务组名称/README.md` must include:

1. Goal.
2. Known scope.
3. Subtask index.
4. Acceptance criteria.

Template:

```markdown
# T01 任务组名称

## 目标

说明这个任务组要解决什么问题。

## 已知范围

1. `path/to/file`

## 子任务索引

| 状态 | 子任务 | 说明 |
| --- | --- | --- |
| `[ ]` | [T01-01 子任务名称](T01-01-子任务名称.md) | 子任务说明 |

## 验收标准

1. 标准一。
```

### Subtask File

Each subtask must independently guide one implementation pass. Include:

1. Title.
2. Status.
3. Goal.
4. Inputs.
5. Outputs.
6. Execution checklist.
7. Completion criteria.
8. Latest project facts this subtask depends on.
9. Files or symbols expected to be touched.
10. Verification command or manual check.

Every concrete subtask must be specific to the current project. Do not write generic items such as "实现接口", "补充测试", or "优化代码" without naming the affected files, call sites, data contracts, expected behavior, and verification.

Template:

```markdown
# T01-01 子任务名称

状态：`[ ]`

## 目标

一句话说明本子任务要完成什么。

## 输入

1. 输入文件或上游任务。
2. 最新项目状态依据：`path/to/file`、`command`、调用点或测试结果。

## 输出

1. 产出文件、清单或代码改动。

## 执行清单

- [ ] 第一步。
- [ ] 第二步。

## 完成标准

1. 可验证标准。
2. 需要更新进度跟踪表。
```

### Progress Table

`进度跟踪表.md` is the only central progress table for the task. It must include all subtasks.

Required columns:

| 字段 | 含义 |
| --- | --- |
| 状态 | Unified status marker |
| 编号 | Subtask ID, for example `T01-01` |
| 优先级 | `P0`, `P1`, or `P2` |
| 子任务 | Link to subtask file |
| 上级 | Task group ID |
| 下一步 | Next action for the current status |
| 完成说明 | Verification command, result, or reason not verified |

Priority rules:

| 优先级 | 含义 |
| --- | --- |
| P0 | Blocks the main flow, risks data correctness, or blocks task decomposition |
| P1 | Clearly reduces follow-up maintenance cost but does not block the main flow |
| P2 | Naming, tests, cleanup, or supplementary verification |

Template:

```markdown
# 进度跟踪表

| 状态 | 编号 | 优先级 | 子任务 | 上级 | 下一步 | 完成说明 |
| --- | --- | --- | --- | --- | --- | --- |
| `[ ]` | T01-01 | P0 | [子任务名称](tasks/T01-任务组名称/T01-01-子任务名称.md) | T01 | 下一步动作 |  |
```

## Update Flow

Before starting or resuming any subtask:

1. Re-check the affected files, docs, tests, and existing task status.
2. If the repository changed since the task was written, update `00-前置约束.md`, `进度跟踪表.md`, and the affected subtask files before implementation.
3. If the change invalidates scope or acceptance criteria, mark the affected rows `[!]` until clarified or rewrite the plan based on the latest state.

When starting a subtask:

1. Change its row in `进度跟踪表.md` to `[/]`.
2. Change the subtask file status to `状态：` `[/]`.

When implementation is done but verification is not done:

1. Change both statuses to `[?]`.
2. Record pending verification in the subtask file or progress-table next step.

When verification is done:

1. Change both statuses to `[x]`.
2. Record verification commands and results in `进度跟踪表.md`.
3. If tests were not run, record the reason.

When blocked:

1. Change both statuses to `[!]`.
2. Record what is blocked, who must provide input, and what is needed.

When deferred:

1. Change both statuses to `[-]`.
2. Record the deferral reason and restart condition.

When the project changes during implementation:

1. Update the current subtask with the new files, symbols, contracts, and verification impact.
2. Add new subtasks for newly discovered necessary work instead of hiding it in a broad checklist item.
3. Mark obsolete subtasks `[-]` with the reason, or rewrite them if they remain valid under the new state.
4. Update `进度跟踪表.md` immediately so status and next action match reality.

## Final Check

Before considering task-document setup complete, verify:

1. The entry document links to the task directory.
2. `00-前置约束.md` exists and states boundaries before coding.
3. The docs record the latest repository state used to generate or update the plan.
4. `进度跟踪表.md` contains every subtask.
5. Every subtask file has `状态：...`.
6. Every concrete subtask names current files, scope, outputs, and verification.
7. The progress table and subtask statuses are consistent.
8. There are no stale names, stale paths, obsolete assumptions, or duplicate progress tables.
