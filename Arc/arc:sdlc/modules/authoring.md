# Authoring Gates

Load this file when starting or resuming large tracked work, before creating or rewriting task files.

## Document Format

Task entries, pre-constraints, task trees, subtask cards, progress, and API contracts MUST be `.md` documents. Use Markdown headings, links, tables, and checklists; do not generate JSON task manifests or JSON blocks to stand in for these documents. Existing tool configuration and machine output are outside this document-format rule.

## Pre-Implementation Gate

Before changing production code for large or tracked work:

1. Inspect the latest repository state, including current docs, local task conventions, affected files, tests, routes, configs, migrations, and call sites.
2. Resolve the requirement/task doc roots and read existing indexes before creating anything.
3. Assess the work's difficulty and dependencies, then create or update the task hierarchy under the existing task category using the rules below.
4. Write `00-前置约束.md` before implementation.
5. Write or update the central progress table with every planned subtask.
6. Make every concrete subtask specific enough to resume from disk: owning side, difficulty with reasons, dependencies, current files, scope, outputs, `<Context-Queries>`, and verification.
7. Mark the first active subtask as `[/]` in both the progress table and the subtask file.

Only discovery commands, code reading, and task-document edits are allowed before this gate is complete.

Task docs are stale if they are not updated immediately when project files, scope, assumptions, evidence, tests, or status change.

## Hierarchy And Frontend/Backend Boundaries

Use three levels inside a domain task directory:

| 层级 | 作用 | 默认编号 | 文档 |
| --- | --- | --- | --- |
| L1 业务任务组 | 一个功能目标或业务流程，定义整体边界和验收 | `T01` | 任务组 `README.md` |
| L2 分端工作包 | 后端、前端分别组织职责与依赖 | `T01-BE`、`T01-FE` | `backend/README.md`、`frontend/README.md` |
| L3 可执行子任务 | 单端、一个可独立验证的结果 | `T01-BE-01`、`T01-FE-01` | 各端目录中的独立 `.md` 文件 |

1. A group that touches both ends MUST have separate backend and frontend work packages and task files. Merely adding “前端/后端” checkboxes to one implementation task does not satisfy this boundary.
2. Backend owns API behavior, authorization, business rules, persistence, and server verification. Frontend owns pages, interactions, API consumption, state presentation, and client verification. Each has its own ID, scope, acceptance, dependencies, and status.
3. Parents organize work and aggregate acceptance; execution and difficulty assessment belong to leaves. Do not count a parent as an extra implementation task.
4. Split by verifiable outcome and real dependencies, not equal task counts, file counts, or an assumed one-page/one-endpoint mapping. Keep changes that enforce one indivisible invariant together.
5. Create only the sides actually in scope. Frontend-only work must not invent backend work; backend-only work must not generate empty frontend tasks. Pure research/operations may use an accurately named work package instead of pretending to be a UI/API task.
6. If a cross-end user journey needs separate integration acceptance, create an `INT` verification package and tasks (for example `T01-INT-01`), dependent on the required BE/FE tasks. It verifies the joint behavior; it must not hide mixed implementation work.
7. Existing local numbering may be preserved, but explicit hierarchy, separate sides, independent task files, and acceptance must remain. Small work needs no extra hierarchy files unless task docs are requested; if both ends are documented, their executable cards still remain separate.

## Difficulty Before Sizing

First inspect the affected contracts, code, and verification path. Assess **change scope, business/state complexity, contract or data risk, external dependencies/uncertainty, and verification cost**. Record the meaningful reasons with current evidence; do not infer difficulty from the title or assign every task the same default level.

| 难度 | 判断依据 | 拆分方式 |
| --- | --- | --- |
| D1 简单 | 已有模式可直接复用，行为局部且明确，无合同/数据结构变化，验证路径短 | 一个独立结果即可，不拆成读文件、改一行、跑命令等碎片 |
| D2 中等 | 少量模块协作或明确的接口/页面行为变化，边界和验证手段已知 | 按业务行为或分端交付结果拆，相关实现与必要验证在同一子任务 |
| D3 复杂 | 多状态、权限、并发、迁移、公共契约或多调用端影响，验证组合较多 | 按可独立验收的能力、风险边界和依赖拆；保留 D3 叶子时说明不可再拆的共同不变量及验证方案 |
| D4 高难/待探索 | 关键业务合同、技术方案、外部能力或一致性/迁移风险尚未收敛 | 先独立安排有边界的调研/最小验证任务，明确产出和退出条件，再据证据细化实现任务 |

Rules:

- Difficulty (`D1`–`D4`) measures implementation challenge; priority (`P0`–`P4`) measures urgency/value. An urgent task may be D1; a deferred task may be D3. Store both separately.
- Judge by the hardest material factor, not an average that hides data or verification risk. Splitting a high-risk task does not erase the risk from its children.
- Every leaf records its difficulty, the concrete reason, dependencies, and verification burden. A work-package README records the main difficulty and why this decomposition is appropriate.
- D3 work must be reviewed for further meaningful splits; do not mechanically split a cohesive transaction/invariant or create a task for each file. D4 must not remain a vague “实现整个模块” executable task: isolate the unknown first, with bounded questions and evidence-based exit criteria.
- Independent work may proceed while another task explores an unknown. Mark only dependent work blocked; use discovery to resolve facts rather than asking the user to design routine implementation details.
- Reassess when a new dependency, contract change, or failed verification changes the difficulty. Update task cards and progress before continuing with the changed scope.

For example, adding a filter to an existing list might produce a D2 backend task for the query contract and a D1 frontend task using an existing filter component. A new cross-service approval flow might need D4 backend exploration before D3 state/consistency tasks, while its frontend form and state presentation are separately assessed. These are sizing examples, not default ratings for every list or approval feature.

## Contracts And Dependencies

1. API work uses the shared [Markdown contract rules](../../arc:build/references/api-contract.md). Link the actual API document and section from both BE and FE tasks; Apifox is not a prerequisite or the contract authority.
2. Write dependencies as **task ID/link + needed output + readiness condition**, not “后端完成后再做”. If there is no dependency, say so explicitly.
3. Align a changed contract before dependent code. If the contract already exists and is current, link it instead of inventing an unnecessary contract task.
4. Frontend UI/state work may proceed against an aligned contract with Mock data; a task requiring real integration remains `[?]` until that integration is verified. Backend completion cannot mark a frontend task or the whole feature complete.
5. Distinguish parallel tasks from sequential dependencies. Resolve dependency cycles by extracting the shared contract/decision prerequisite; do not assign mutually blocking implementation tasks.

## Subtask Specificity

Every concrete subtask must be specific to the current project:

1. State the exact goal in project terms, not a generic engineering label.
2. Name affected files, symbols, APIs, routes, tables, data contracts, commands, tests, or UI states.
3. Describe the intended implementation sequence and where to stop.
4. Spell out non-goals, invariants, compatibility constraints, and known edge cases.
5. Describe expected behavior before and after the change, including error and empty states when relevant.
6. Define verification commands or manual checks with expected results.
7. Include "do not" instructions for tempting but incorrect shortcuts, paper-over fixes, or silent error handling.
8. Record parent group/work package, owning side, priority, difficulty with evidence, upstream readiness conditions, and downstream consumers.
9. Include a `<Context-Queries>` section with exact `arc-idx files/search/symbol/ast` commands grounded in discovery. Use real paths, symbols, and configured profiles, with `--format json` or `--format ai-md`; do not leave placeholders. The executor runs these commands before editing.

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
