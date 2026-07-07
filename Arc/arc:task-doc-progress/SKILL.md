---
name: arc:task-doc-progress
description: Use for large work needing 需求/任务 docs, pre-constraints, subtasks, indexes, and progress tracking.
---

# Task Doc Progress

## Overview

Use this skill to create and maintain persistent local requirement and task documents for large, multi-step work. The goal is to preserve enough current project context that a later session can resume from `docs/` without rereading the whole codebase.

Keep raw requirements separate from executable tasks:

1. Put original requirements, product notes, data-range notes, and unresolved input under `docs/02-需求`.
2. Put implementation plans, migrations, refactors, fixes, verification work, and progress state under `docs/01-任务`.
3. Do not copy long raw requirements into task docs. Link to requirement docs and restate only actionable scope, constraints, and acceptance criteria.

## When to Use

Use this skill when the user asks to:

1. 整理需求, 创建任务, 拆任务, 制定计划, 建进度表, 更新任务文档, or 维护进度跟踪.
2. Start a larger feature, migration, refactor, cleanup, bug-fix campaign, audit follow-up, or verification campaign.
3. Record prerequisites, non-goals, data or migration constraints, security or performance boundaries, cross-module impact, or open questions before coding.
4. Resume tracked work where `docs/01-任务` already contains progress state.

Skip this skill for a small one-shot edit unless the user explicitly asks for task docs or cross-session tracking.

## Decision Flow

Before writing docs, choose the correct action:

1. If the user gives raw or ambiguous product input, create or update `docs/DD-需求` first, then ask or infer only enough to make executable tasks.
2. If the user gives an executable implementation goal, create or update `docs/DD-任务` directly and link source requirements when they exist.
3. If a matching business-domain task already exists, reuse it and add a task group or subtask there.
4. If the work is a new business domain, pick the next available `NN` and create a numbered task directory.
5. If the repository has a local spec, follow it even when it differs from this default structure, unless it would break the hard gates in this skill.

Do not create a new top-level task just because the wording is new. First check whether it belongs under an existing domain entry such as admin, author, reader, Android, security, testing, data, or observability.

## Required Gate

Before changing production code for large or tracked work:

1. Inspect the latest repository state: existing docs, local task spec, indexes, affected files, tests, routes, configs, migrations, and call sites.
2. Resolve the requirement and task document roots. Do not hardcode paths if the repository has an existing convention.
3. Read the task category `README.md`, task index, and matching existing task directories before creating a new top-level task.
4. Create or update the task document structure.
5. Write `00-前置约束.md` before implementation.
6. Write or update the central progress table with every planned subtask.
7. Make every concrete subtask detailed enough for one implementation pass without hidden context.
8. Mark the first active subtask as `[/]` in both the progress table and the subtask file.

Only discovery commands, code reading, and task-document edits are allowed before this gate is complete.

## Resume Flow

When continuing existing tracked work:

1. Read the task entry, `00-前置约束.md`, central progress table, and only the active or next candidate subtask files.
2. Re-check the project files named by the active subtask before trusting old assumptions.
3. If code, schema, route, config, tests, or user decisions changed, update the pre-constraints, progress table, and affected subtasks before implementation.
4. Pick the next row in this order: `[/]`, then `[?]` if verification is the next action, then highest-priority `[ ]`, then `[!]` only after the blocker is resolved.
5. Do not mark work complete from memory. Completion requires the subtask status, progress row, and verification note to match current evidence.

## Directory Roots

Resolve roots from the project:

1. Treat immediate children of `docs/` as numbered categories when they match `DD-分类名`, for example `01-任务` and `02-需求`.
2. Use the existing task category when present, such as `01-任务`, `00-任务`, or another clear equivalent of "任务".
3. Use the existing requirement category when present, such as `02-需求`, `01-需求`, or another clear equivalent of "需求".
4. If initializing a docs tree from scratch, prefer `docs/01-任务` and `docs/02-需求`.
5. Follow local numbering and category vocabulary. Do not create duplicate task or requirement categories.

Requirement category rules:

1. Use the requirement category for raw inputs, product notes, and data-range notes.
2. Maintain the category `README.md` as an index when it exists.
3. Promote requirements into task docs only after goals, scope, boundaries, and acceptance criteria are executable.
4. Link source requirement docs from task entries or `00-前置约束.md`.

Task category rules:

1. Keep the task category root for `README.md`, index directories such as `00-任务索引/`, local specs such as `00-任务文档与进度跟踪规范/`, and numbered task directories.
2. For new work, prefer a numbered task directory with its own `README.md`; do not add loose top-level topic `.md` files unless local history already uses that active entry.
3. Keep one top-level task directory per business domain. Put new topics under the matching existing domain task when one exists.
4. Record why any historical duplicate or loose legacy task remains when touching related indexes.

## Default Task Structure

Use this structure when the project has no stricter local spec:

```text
docs/01-任务/
  README.md
  00-任务索引/
    README.md
  NN-具体任务名/
    README.md
    00-前置约束.md
    进度跟踪表.md
    tasks/
      T01-任务组名称/
        README.md
        T01-01-子任务名称.md
        T01-02-子任务名称.md
```

Compatibility rules:

1. If the project uses `进度跟踪.md`, `00-总进度看板.md`, or direct `TNN-任务组名称/` directories without `tasks/`, follow that local structure.
2. If the project uses paired loose entries such as `NN-具体任务名.md` plus `NN-具体任务名/`, update both when they are active.
3. If both `进度跟踪.md` and `进度跟踪表.md` exist, update the one referenced by the task entry and do not create a second central table.
4. Use two digits for category `DD`, task `NN`, task group `TNN`, and subtask `MM`.
5. Never use suffixes such as "最终版", "新版", "临时版", or duplicate progress tables.

## Status And Priority

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

1. Each subtask file must have `状态：...` near the top.
2. The central progress table and subtask files must agree.
3. Do not create a separate completed-history table. Put completion notes in the central progress table.

Use these priorities:

| 优先级 | 含义 |
| --- | --- |
| P0 | Blocks the main flow, risks data correctness, or blocks task decomposition |
| P1 | Core functionality or high-value governance that reduces follow-up maintenance cost |
| P2 | Normal implementation, tests, cleanup, documentation, or supplementary verification |
| P3 | Schedulable optimization, experience polish, auxiliary tooling, or investigation |
| P4 | Explicitly low-priority or convenience-only backlog |

## Required Documents

Keep documents factual and executable:

1. Record only facts verified from current files, commands, user input, or linked docs.
2. Mark assumptions as assumptions, not conclusions.
3. Put detailed original discussions in requirement docs, not in task subtasks.
4. Write each subtask so another agent can execute it by opening only the task entry, pre-constraints, progress table, task group README, and that subtask file.

### Task Entry

`NN-具体任务名/README.md` is the preferred task entry. Include:

1. Task summary.
2. Usage instructions.
3. Design boundaries or implementation constraints.
4. Source requirement links when relevant.
5. Task group index.
6. Links to `00-前置约束.md` and the central progress table.

Template:

```markdown
# 具体任务名

本文是任务入口。具体任务和进度在当前目录。

## 使用方式

1. 先看 `00-前置约束.md` 和 `进度跟踪表.md`。
2. 再打开当前要做的任务组 README 和子任务文件。
3. 完成后同步更新进度表和子任务状态。

## 需求来源

1. [需求名称](../../02-需求/NN-需求名称.md)

## 设计边界

1. ...

## 任务索引

| 编号 | 任务组 | 优先级 |
| --- | --- | --- |
| T01 | [任务组名称](tasks/T01-任务组名称/README.md) | P0 |
```

### Pre-Constraints

`00-前置约束.md` is mandatory before coding. Include:

1. Goals and non-goals.
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

Each task group `README.md` must include:

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
9. Files, symbols, routes, migrations, data contracts, UI states, or test cases expected to be touched.
10. Verification command or manual check.

Do not write generic items such as "实现接口", "补充测试", or "优化代码" without naming affected files, call sites, data contracts, expected behavior, and verification.

Subtask quality bar:

1. Name the exact current files, directories, APIs, routes, tables, collections, commands, or UI states involved.
2. State observable behavior before and after the change.
3. Define verification that can be run or manually checked.
4. Keep one subtask to one coherent implementation pass. Split it if it spans unrelated modules or cannot be verified in one pass.
5. Add newly discovered necessary work as a new subtask instead of hiding it inside a broad checklist item.

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

Use one central progress table per task. Name it according to the local convention, usually `进度跟踪表.md` or `进度跟踪.md`.

Required columns:

| 字段 | 含义 |
| --- | --- |
| 状态 | Unified status marker |
| 编号 | Subtask ID, for example `T01-01` |
| 优先级 | `P0`, `P1`, `P2`, `P3`, or `P4` |
| 子任务 | Link to subtask file |
| 上级 | Task group ID |
| 下一步 | Next action for the current status |
| 完成说明 | Verification command, result, or reason not verified |

Template:

```markdown
# 进度跟踪表

| 状态 | 编号 | 优先级 | 子任务 | 上级 | 下一步 | 完成说明 |
| --- | --- | --- | --- | --- | --- | --- |
| `[ ]` | T01-01 | P0 | [子任务名称](tasks/T01-任务组名称/T01-01-子任务名称.md) | T01 | 下一步动作 |  |
```

## Index Maintenance

When creating or moving task or requirement docs:

1. Update the category `README.md` when it exists.
2. Update `00-任务索引/README.md` when it exists.
3. Keep links relative and pointing at active entry docs.
4. Do not reorder unrelated historical entries unless the user asks to reorganize docs.

## Update Flow

Before starting or resuming any subtask:

1. Re-check affected files, docs, tests, and existing task status.
2. If the repository changed since the task was written, update `00-前置约束.md`, the central progress table, and affected subtask files before implementation.
3. If the change invalidates scope or acceptance criteria, mark affected rows `[!]` until clarified or rewrite the plan based on the latest state.

When starting a subtask:

1. Change its row in the central progress table to `[/]`.
2. Change the subtask file status to `状态：` `[/]`.

When implementation is done but verification is not done:

1. Change both statuses to `[?]`.
2. Record pending verification in the subtask file or progress-table next step.

When verification is done:

1. Change both statuses to `[x]`.
2. Record verification commands and results in the central progress table.
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
3. Mark obsolete subtasks `[-]` with the reason, or rewrite them if still valid.
4. Update the central progress table immediately so status and next action match reality.

## Common Failure Modes

Avoid these mistakes:

1. Creating a duplicate top-level task when an existing domain task should be reused.
2. Writing tasks from stale memory without checking current files and indexes.
3. Mixing raw requirements into execution subtasks.
4. Creating more than one central progress table for the same task.
5. Marking `[x]` without a verification command, manual check, or explicit reason verification could not run.
6. Leaving the progress row at `[ ]` while editing code for that subtask.
7. Writing broad subtasks that do not name files, contracts, or expected behavior.

## Final Check

Before considering task-document setup complete, verify:

1. Requirement input, if any, is under the requirement category or linked from there.
2. The task entry is a numbered directory `README.md` unless local history requires a loose entry.
3. Existing category indexes are updated.
4. `00-前置约束.md` exists and states boundaries before coding.
5. Docs record the latest repository state used to generate or update the plan.
6. The central progress table contains every subtask.
7. Every subtask file has `状态：...`.
8. Every concrete subtask names current files, scope, outputs, and verification.
9. The progress table and subtask statuses are consistent.
10. There are no stale names, stale paths, obsolete assumptions, duplicate progress tables, or duplicate top-level business-domain task entries.
