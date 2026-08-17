# Authoring Gates

Load this file when starting or resuming large tracked work, before creating or rewriting task files.

## Pre-Implementation Gate

Before changing production code for large or tracked work:

1. Inspect the latest repository state, including current docs, local task conventions, affected files, tests, routes, configs, migrations, and call sites.
2. Resolve the requirement/task doc roots and read existing indexes before creating anything.
3. Create or update the task document structure under the existing task category.
4. Write `00-前置约束.md` before implementation.
5. Write or update the central progress table with every planned subtask.
6. Make every concrete subtask specific enough to resume from disk: current files, scope, outputs, and verification.
7. Mark the first active subtask as `[/]` in both the progress table and the subtask file.

Only discovery commands, code reading, and task-document edits are allowed before this gate is complete.

Task docs are stale if they are not updated immediately when project files, scope, assumptions, evidence, tests, or status change.

## Subtask Specificity

Every concrete subtask must be specific to the current project:

1. State the exact goal in project terms, not a generic engineering label.
2. Name affected files, symbols, APIs, routes, tables, data contracts, commands, tests, or UI states.
3. Describe the intended implementation sequence and where to stop.
4. Spell out non-goals, invariants, compatibility constraints, and known edge cases.
5. Describe expected behavior before and after the change, including error and empty states when relevant.
6. Define verification commands or manual checks with expected results.
7. Include "do not" instructions for tempting but incorrect shortcuts, paper-over fixes, or silent error handling.

Do not write generic items such as "实现接口", "补充测试", or "优化代码" without naming files, call sites, contracts, expected behavior, and verification.

## Completion Definition

Use [`docs/execution-truth.md`](../../../docs/execution-truth.md) as the shared rule.

1. Mark `[x]` only when **code path + project gate + reachable behavior** all match the claim on the intended env/branch surface.
2. MUST NOT promote to `[x]` from ability-matrix cells, README claims, or "already fixed" chat without re-check on current repository state.
3. Prefer `[?]` when implementation exists but verification is pending; prefer `[-]` for explicit non-goals; prefer `[!]` when blocked.
4. When re-auditing status, re-read code and gates first; treat stale task docs as suspects.

## Explicit Non-Goals

`00-前置约束.md` and the task entry MUST keep a visible **不做范围** list for large or tracked work.

1. Copy user-stated non-goals (Hold, P4, "明确不做", deferred migration stages, payment freeze, read-only, docs-only) into that list without reinterpretation.
2. Subtasks MUST NOT implement items listed as non-goals "while we are here".
3. If new work is discovered that was not in scope, add a subtask or mark `[!]` / open a new task.
4. Environment/branch/deploy surface for verification and packaging belongs in 前置约束 when dual tracks or multiple hosts exist.

## Downstream Task Authoring

When the user will hand subtasks to a cheaper or lower-context executor:

1. Bound each subtask: in-scope files/symbols, out-of-scope, ordered steps, acceptance checks, and forbidden shortcuts.
2. One subtask = one verifiable outcome; ban vague titles such as "修复安全问题" or "补测试".
3. Name the env/branch surface and dual-track choice when relevant.
4. Include "do not" for invented domain identity keys, wrong deploy surface, compatibility shims the project forbids, and paper-over fixes.
5. Author as a role that can be resumed from disk; still require prewalk or oneshot Guide for live multi-model runs.

## Security / Audit Sourced Work

When the work comes from `arc:audit` (appsec), `arc:security`, multi-finding vulnerability repair, or the user asks to “把审计/扫描结果拆成任务”:

1. Follow [`../references/security-audit-task-pipeline.md`](../references/security-audit-task-pipeline.md).
2. Act as role `R-task`: write docs only; do not implement production fixes in this role.
3. Require a Handoff Package containing project positioning, project caliber, findings, and optional re-ranked scan notes.
4. Put in `00-前置约束.md`: `项目定位`, `项目口径`, `功能角色`, `上游 Handoff`, `Finding 索引`.
5. Split task groups by risk domain, never by scanner tool names alone.
6. Expand each confirmed finding into subtasks that name `finding_id`, files/symbols, permission class, data yield, 执行清单, 不要做, and verify steps.
7. Create research/false-positive subtasks for `likely` / `assumption` findings before fix subtasks.
8. After planning, hand `R-fix` to `arc:fix` / `arc:build` one subtask at a time.
