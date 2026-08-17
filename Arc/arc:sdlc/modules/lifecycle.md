# Status And Update Flow

Load this file when starting, pausing, blocking, completing, verifying, or indexing task docs.

## Status Markers

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
- The central progress table and each subtask file must agree.
- Do not create a separate completed-history table. Put completion notes in the central progress table.

## Index Maintenance

When creating or moving task/requirement docs:

1. Update the category `README.md` when it exists.
2. Update `00-任务索引/README.md` when it exists.
3. Keep task links relative and pointing at the active entry doc.
4. Record why any historical duplicate or loose legacy task remains.
5. Do not reorder unrelated historical entries unless the task is explicitly to reorganize docs.

## Update Flow

Before starting or resuming any subtask:

1. Re-check affected files, docs, tests, and existing task status.
2. If the repository changed since the task was written, update `00-前置约束.md`, the central progress table, and affected subtask files before implementation.
3. If the change invalidates scope or acceptance criteria, mark affected rows `[!]` until clarified.

When starting a subtask: change both the progress-table row and the subtask file to `[/]`.

When implementation is done but verification is not: change both to `[?]` and record the pending check.

When verification is done: change both to `[x]` and record commands/results. If tests were not run, record the reason.

When blocked: change both to `[!]` and record what is blocked and who must provide input.

When deferred: change both to `[-]` and record the deferral reason and restart condition.

When the project changes during implementation:

1. Update the current subtask with the new files, symbols, contracts, and verification impact.
2. Add new subtasks for newly discovered necessary work.
3. Mark obsolete subtasks `[-]` with the reason, or rewrite them.
4. Update the central progress table immediately.

## Final Check

1. Requirement input, if any, is under the requirement category or linked from there.
2. The task entry is a numbered directory `README.md` unless local history requires a loose entry.
3. Existing category indexes are updated.
4. `00-前置约束.md` exists and states boundaries before coding, including 不做范围 and completion definition.
5. The docs record the latest repository state used to generate or update the plan.
6. The central progress table contains every subtask.
7. Every subtask file has `状态：...`.
8. Every concrete subtask names current files, scope, outputs, and verification.
9. The progress table and subtask statuses are consistent; `[x]` rows meet the completion definition.
10. There are no stale names, stale paths, obsolete assumptions, or duplicate top-level business-domain task entries.
11. For security/audit-sourced work: 项目定位, 项目口径, 功能角色, Finding 索引, and per-finding subtasks exist.
12. Downstream/cheap-executor tasks include ordered steps, acceptance checks, and forbidden shortcuts when they will be handed off.
