---
name: arc:sdlc
description: "Current-state task docs; security/audit handoff to detailed subtasks with roles and caliber."
---

# Task Doc Progress Conventions

## Overview

Use this skill to turn large or multi-step work into persistent local docs before implementation. The docs must capture the latest repository state, so a later session can resume from `docs/` without rediscovering the whole codebase.

Task docs are the **cross-session / human / audit** memory. They are **not** the default vehicle for same-session multi-model cost routing. Live cheap-executor handoff belongs to `arc:prewalk` (trajectory + todo + first production edit)—see [`docs/prewalk.md`](../../docs/prewalk.md). Do not treat a detailed plan markdown as a substitute for handing off the agent context window.

Separate raw requirements from executable work:

1. Put original requirements, product notes, data-range notes, and unresolved input under the resolved requirement category such as `docs/DD-需求`.
2. Put executable plans, implementation subtasks, migrations, fixes, verification work, and progress state under the resolved task category such as `docs/DD-任务`.
3. Do not copy large raw requirements into task docs. Link to the relevant requirement docs and restate only the actionable scope, constraints, and acceptance criteria.

For small one-shot fixes, do not create this structure unless the user asks for task docs.

## When to Use

- Use before coding when the request is a larger feature, migration, refactor, cleanup, bug-fix campaign, audit follow-up, or any task likely to span multiple implementation passes.
- Use when the user asks to "制定计划", "创建任务", "进度跟踪", "任务文档", "拆任务", "需求整理", or says future work must follow this convention.
- Use when a task has prerequisites, non-goals, verification risk, cross-module impact, data migration, security/performance constraints, or open questions that should survive across sessions.
- Skip for a single-file or obvious small edit unless it needs cross-session tracking.

## Pre-Implementation Gate

Before changing production code for large or tracked work:

1. Inspect the latest repository state, including current docs, local task conventions, affected files, tests, routes, configs, migrations, and call sites.
2. Resolve the requirement/task doc roots and read existing indexes before creating anything.
3. Create or update the task document structure under the existing task category.
4. Write `00-前置约束.md` before implementation.
5. Write or update the central progress table with every planned subtask.
6. Make every concrete subtask specific enough to resume from disk: current files, scope, outputs, and verification—without pretending the markdown replaces a live agent trajectory.
7. Mark the first active subtask as `[/]` in both the progress table and the subtask file.

Only discovery commands, code reading, and task-document edits are allowed before this gate is complete.

Task docs are stale if they are not updated immediately when project files, scope, assumptions, evidence, tests, or status change. They must be updated immediately when project files, scope, assumptions, or status change.

## Execution Model: Docs vs Prewalk

Two different handoffs must not be confused:

| Handoff | Mechanism | Use for |
|---|---|---|
| **Durable task docs** (this skill) | `00-前置约束.md`, subtasks, `进度跟踪表.md` | Humans, new sessions, audit campaigns, pause/resume days later |
| **Same-session multi-model** (`arc:prewalk`) | Keep tool trajectory; Guide does recon + bounded todo + **first production edit**; swap to Executor; prune planning instruction | Cost/speed routing inside one agent run |

Hard rules:

1. MUST NOT market “frontier writes a plan document, cheap model implements from the document alone” as the Arc default cost optimization. That is the plan-postcard anti-pattern (`O(reads)` paid twice).
2. MUST use `arc:prewalk` when the runtime can swap models mid-session and the parent skill wants a cheaper Executor after orientation.
3. MUST keep progress tables and subtask checklists as **steering surfaces** after a prewalk swap (Executor may forget prose plans; it cannot ignore an active todo/progress row that is kept in context).
4. When a **new session** cold-starts from docs only: either short Guide prewalk (re-ground + first edit then swap) or oneshot Guide—not cold cheap execution from plan text alone as the default.
5. Security `R-task` may still author docs only; `R-fix` must apply prewalk (or oneshot) per subtask rather than treating the subtask file as a full substitute for repo exploration context.

## Subtask Specificity (Still Required)

Every concrete subtask must be specific to the current project so progress and verification stay honest. Required detail:

1. State the exact goal in project terms, not a generic engineering label.
2. Name affected files, symbols, APIs, routes, tables, data contracts, commands, tests, or UI states.
3. Describe the intended implementation sequence and where to stop.
4. Spell out non-goals, invariants, compatibility constraints, and known edge cases.
5. Describe expected behavior before and after the change, including error and empty states when relevant.
6. Define verification commands or manual checks with expected results.
7. Include "do not" instructions for tempting but incorrect shortcuts, broad fallbacks, paper-over fixes, or silent error handling.

These details support **resume, review, and acceptance**—not a claim that a cold low-context model can reconstruct full understanding from the postcard alone.

## Progress Tracking Hard Gate

The central progress table and subtask `状态：...` are the authoritative local execution state for large or tracked work. `进度跟踪表.md` and subtask `状态：...` are the authoritative local execution state.

Hard rules:

1. MUST update progress tracking immediately when starting, pausing, blocking, completing, or verifying a subtask.
2. MUST update progress tracking immediately when the next action changes because project files, scope, assumptions, evidence, tests, or user decisions changed.
3. MUST NOT continue implementation while the central progress table or active subtask status is stale. MUST NOT continue implementation while `进度跟踪表.md` or the active subtask status is stale.
4. MUST NOT send a final delivery response for tracked work until progress tracking reflects the actual final state.
5. If verification is skipped or blocked, the progress table completion note MUST record the reason and the next required action.

## Integration With Arc Skills

When another Arc skill is active, use this skill as the local task-planning gate for large or tracked work:

1. Use `arc:clarify` first if scope, acceptance criteria, or user decisions are unclear.
2. Use this skill before `arc:build`, `arc:fix`, `arc:frontend`, or `arc:security` changes project code for large, multi-step, cross-module, or tracked work.
3. After this skill’s docs gate, use `arc:prewalk` for same-session multi-model routing when the runtime supports model swap; do not replace prewalk with plan-document cold handoff.
4. Use `arc:arch` after this skill and before backend code edits.
5. Use `arc:frontend` after this skill for frontend platform, UI state, token, and verification constraints.
6. Use `arc:security` **after** this skill only when implementing multi-finding remediation or re-scanning fixed paths; use `arc:security` **before** this skill when the user still needs scanner evidence for the handoff package.
7. Use `arc:audit` mode `appsec` before this skill when remediation must be driven by assets, data map, and finding cards rather than ad-hoc bug lists.
8. Use `arc:docs` only for Lark synchronization when `.lark.json` exists or the user explicitly enables Lark. Local `docs/` task state still needs to stay current.

If this skill and another skill conflict, keep the stricter gate: do not start implementation until local task docs, status, and required upstream clarification are current.

## Security / Audit Sourced Work (Narrow Pipeline)

When the work comes from `arc:audit` (appsec), `arc:security`, multi-finding vulnerability repair, or the user asks to “把审计/扫描结果拆成任务”:

1. MUST follow [`references/security-audit-task-pipeline.md`](references/security-audit-task-pipeline.md).
2. MUST act as role `R-task` (任务作者): write docs only; do not implement production fixes in this role.
3. MUST require a **Handoff Package** (or build a minimal one with user consent) containing project positioning, project caliber, findings, and optional re-ranked scan notes—not a chat-only bullet list.
4. MUST put in `00-前置约束.md`:
   - `## 项目定位` — product form, user roles, critical data, trust boundaries, why this engagement exists
   - `## 项目口径` — deploy shape, naming/storage rules, protected paths, env order, architecture limits, test authorization, secrets handling
   - `## 功能角色` — who is R-recon / R-scan / R-task / R-fix / R-verify for this engagement
   - `## 上游 Handoff` — links to assets / data-map / findings / scan reports
   - `## Finding 索引` — every in-scope finding → subtask id
5. MUST split task groups by risk domain (config/secrets, AuthN, AuthZ, payment, upload, injection, dependency), never by “Semgrep/Trivy” tool names alone.
6. MUST expand each **confirmed** finding into subtasks that name `finding_id`, files/symbols, permission class, data yield, step-by-step 执行清单, 不要做, and verify steps. Prefer one finding → one fix subtask (+ optional verify).
7. MUST create research/false-positive subtasks for `likely` / `assumption` findings before fix subtasks.
8. MUST NOT create vague subtasks such as “修复安全问题”, “加固鉴权”, “处理 High 漏洞” without finding ids and paths.
9. MUST map data-value priority into P0–P3 using the pipeline table; do not copy CVSS blindly.
10. After planning is complete, hand `R-fix` to `arc:fix` / `arc:build` **one subtask at a time**, preferring `arc:prewalk` inside each fix session when multi-model routing is available; hand re-scan requests back to `arc:security` only with explicit scope.

For non-security large work, the security pipeline sections are optional; the rest of this skill still applies.

## SDLC Document Architecture (Directory Roots)

For new projects, or when upgrading a project's documentation architecture, enforce a strict, structurally rigorous Software Engineering Lifecycle (SDLC) partitioning under `docs/`. Every document must belong to a logical phase, and context must flow sequentially without duplication.

Standard `docs/` SDLC Architecture (Comprehensive Agent & Developer Ready):
- `docs/00-产品需求/`: PRDs, user stories, business rules, and domain glossaries. (The *Why* and *What*).
- `docs/01-系统设计/`: High-level architecture, technical RFCs, sequence diagrams, and DB/Domain models. (The *How*).
- `docs/02-API记录/`: API mapping and 3rd-party integration docs. **CRITICAL**: Do NOT write API contracts manually here. The single source of truth for APIs is Apifox (managed via `apifox-cli`). This folder holds exported syncs/indexes.
- `docs/03-执行任务/`: Feature/bugfix task plans, execution subtasks, progress tables, and `00-前置约束.md`. (The active execution layer).
- `docs/04-性能优化/`: Dedicated optimization plans (e.g., SQL tuning, latency reduction, memory profiling, UI responsiveness).
- `docs/05-代码治理/`: Technical debt management, code standardization, refactoring plans, and deprecated API removal.
- `docs/06-环境运维/`: CI/CD pipelines, Docker/K8s configs, DB migration records, and Runbooks.
- `docs/07-测试质量/`: Test plans, E2E scenarios, test execution reports, and coverage metrics.
- `docs/08-安全审计/`: SAST/DAST reports, permission models, audit findings, and data privacy notes.
- `docs/09-线上缺陷/`: Production bug tracking, incident records, and troubleshooting guides.
- `docs/10-复盘决策/`: Architectural decision records (ADRs), post-mortems, and sprint retrospectives.
- `docs/11-参考备忘/`: Reference documents, external links, meeting minutes, and context notes.

Rules for `docs/`:

1. **Scaffold on Initialization**: When asked to create docs for a new project, or when encountering an unformatted project, the AI MUST proactively scaffold the complete `00` through `11` directory tree (including `README.md` files for active folders) to initialize the SDLC structure.
2. **Strict Placement**: Do not dump loose markdown files at the root of `docs/`. Every document must be placed into the appropriate numbered SDLC category.
3. **Context Completeness**: Ensure rigorous cross-referencing. A task in `03-执行任务` MUST link back to the source requirement in `00-产品需求` and the architecture in `01-系统设计`. Do not copy-paste large blocks of text across phases; use relative links to maintain a single source of truth.
4. **Index Maintenance**: Use `README.md` at the root of each category to act as the index and routing map for that phase.

Task Category (`03-执行任务`) Rules:

1. Keep the task category root for `README.md`, index directories such as `00-任务索引/`, local specs such as `00-任务文档与进度跟踪规范/`, and numbered task directories.
2. For new work, do not add loose top-level topic `.md` files under the task category. Create or reuse a numbered task directory with a `README.md` entry.
3. Scan `README.md`, `00-任务索引/README.md`, and existing numbered task directories before creating a new top-level task.
4. Keep one top-level task directory per business domain. If a matching domain exists, add a task group, topic doc, or subtask inside it instead of creating another top-level number.

Requirement Category (`00-产品需求`) Rules:

1. Store unresolved raw input and product/data notes in `docs/00-产品需求`.
2. Promote requirements into `docs/03-执行任务` only after they are executable: goals, scope, boundaries, and acceptance criteria are known.

## Default Task Structure

Use a directory entry by default:

```text
docs/03-执行任务/
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

Local compatibility:

1. If the project spec uses `进度跟踪.md`, `00-总进度看板.md`, or direct `TNN-任务组名称/` directories without `tasks/`, follow that existing structure.
2. If the project already uses paired loose entries such as `NN-具体任务名.md` plus `NN-具体任务名/`, update both when they are active.
3. If both `进度跟踪.md` and `进度跟踪表.md` exist, update the one referenced by the task entry and do not create a second central table.
4. If no local convention exists, use the default structure above.

Rules:

- Use two digits for `DD`, task `NN`, `TNN`, and subtask `MM`; `DD` is the `docs/` category sequence.
- Pick the next available `NN` by scanning existing task directories, loose legacy entries, and indexes.
- Do not use suffixes such as "最终版", "新版", "临时版", or duplicate progress tables.
- For simple tracked tasks, still create `NN-具体任务名/README.md`; include goal, scope, status, acceptance criteria, and the current state.
- If the repository already has a stricter local task-document spec, obey the local spec while keeping this skill's pre-constraint and progress-tracking gates.

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
- The central progress table and each subtask file must agree.
- Do not create a separate completed-history table. Put completion notes in the central progress table.

## Required Files

### Task Entry

`NN-具体任务名/README.md` is the preferred entry document. Include:

1. Task summary.
2. Usage instructions.
3. Design boundaries or implementation constraints.
4. Source requirement links when relevant.
5. Task group index.
6. Links to `00-前置约束.md` and the central progress table.
7. A note that docs are for durable progress/resume; same-session multi-model execution uses `arc:prewalk` (trajectory handoff), not plan-postcard cold start.

Template:

```markdown
# 具体任务名

本文是任务入口。具体任务和进度在当前目录。

## 使用方式

1. 先看 `00-前置约束.md` 和 `进度跟踪表.md`。
2. 再打开当前要做的任务组 README 和子任务文件。
3. 完成后同步更新进度表和子任务状态。

## 执行模型假设

1. 本文档用于跨会话恢复、进度权威和验收，不是便宜模型的唯一内存。
2. 同会话多模型成本路由默认走 prewalk：Guide 探索 + 有界 todo + 第一笔生产代码 edit，再把 trajectory 交给 Executor。
3. 子任务仍须写清影响文件、顺序、边界、反例和验证，以便恢复与评审。

## 需求来源

1. [需求名称](../../00-产品需求/NN-需求名称.md)

## 设计边界

1. ...

## 任务索引

| 编号 | 任务组 | 优先级 |
| --- | --- | --- |
| T01 | [任务组名称](tasks/T01-任务组名称/README.md) | P0 |
```

### Pre-Constraints

`00-前置约束.md` is mandatory before coding. Include:

1. Goal and non-goals.
2. Scope boundaries and affected modules.
3. Latest project-state snapshot used to generate the task.
4. Assumptions that still need verification.
5. Existing local conventions that must not be broken.
6. Data, compatibility, migration, security, and performance constraints when relevant.
7. Verification commands or manual checks expected before marking work done.
8. Blockers and required user decisions.
9. Execution model note: durable docs vs prewalk (no plan-postcard default).
10. When the source is security/audit: 项目定位, 项目口径, 功能角色, 上游 Handoff, Finding 索引 (see Security / Audit Sourced Work and the pipeline reference).

Template:

```markdown
# 前置约束

## 目标

1. ...

## 不做范围

1. ...

## 边界约束

1. ...

## 执行模型假设

1. 任务文档是跨会话与进度权威；同会话多模型默认 prewalk（trajectory + todo + first edit），禁止「只交 plan 文件给冷启动便宜模型」作为默认省钱法。
2. 不允许用模糊描述替代具体文件、调用点、数据契约、边界条件、错误处理和验证要求。
3. 新会话从文档恢复时：短 Guide 再 grounding + first edit，或 oneshot Guide；不要默认 cold cheap 只读 plan。

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

Each `tasks/TNN-任务组名称/README.md` or local equivalent must include:

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
9. Files, symbols, routes, migrations, data contracts, or UI states expected to be touched.
10. Verification command or manual check.
11. Implementation notes: exact sequence, invariants, edge cases, and shortcuts to avoid; note that live multi-model runs should inherit trajectory via prewalk, not re-read this file as the sole memory.

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

- [ ] 第一步：说明要改哪个文件/符号、为什么这样改、不要改什么。
- [ ] 第二步：说明要处理的边界条件、错误路径或兼容行为。
- [ ] 验证：命令或手工检查与期望结果。

## 执行注意

1. 同会话执行优先 prewalk：保留探索轨迹与本清单，不要只把本文件丢给冷启动 Executor。
2. 不要引入兜底逻辑吞掉错误，不要用宽泛重构替代本任务目标，不要修改未列入范围的模块。
3. 完成后同步 `进度跟踪表.md`。

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

Priority rules:

| 优先级 | 含义 |
| --- | --- |
| P0 | Blocks the main flow, risks data correctness, or blocks task decomposition |
| P1 | Core functionality or high-value governance that clearly reduces follow-up maintenance cost |
| P2 | Normal implementation, tests, cleanup, documentation, or supplementary verification |
| P3 | Schedulable optimization, experience polish, auxiliary tooling, or investigation |
| P4 | Explicitly low-priority or convenience-only backlog |

Template:

```markdown
# 进度跟踪表

| 状态 | 编号 | 优先级 | 子任务 | 上级 | 下一步 | 完成说明 |
| --- | --- | --- | --- | --- | --- | --- |
| `[ ]` | T01-01 | P0 | [子任务名称](tasks/T01-任务组名称/T01-01-子任务名称.md) | T01 | 下一步动作 |  |
```

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
3. Mark obsolete subtasks `[-]` with the reason, or rewrite them if they remain valid under the new state.
4. Update the central progress table immediately so status and next action match reality.

## Final Check

Before considering task-document setup complete, verify:

1. Requirement input, if any, is under the requirement category or linked from there.
2. The task entry is a numbered directory `README.md` unless local history requires a loose entry.
3. Existing category indexes are updated.
4. `00-前置约束.md` exists and states boundaries before coding.
5. The docs record the latest repository state used to generate or update the plan.
6. The central progress table contains every subtask.
7. Every subtask file has `状态：...`.
8. Every concrete subtask names current files, scope, outputs, and verification.
9. The progress table and subtask statuses are consistent.
10. There are no stale names, stale paths, obsolete assumptions, duplicate progress tables, or duplicate top-level business-domain task entries.
11. For security/audit-sourced work: 项目定位, 项目口径, 功能角色, Finding 索引, and per-finding detailed subtasks exist per [`references/security-audit-task-pipeline.md`](references/security-audit-task-pipeline.md).
