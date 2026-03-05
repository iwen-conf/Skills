---
name: "arc:implement"
description: "方案已明确时使用：按契约落地代码变更（含重构/迁移），并同步验证证据与交接文档。"
---

# arc:implement — 方案落地实现

## Overview

`arc:implement` 负责把需求与方案落到代码实现层，先冻结接口契约与兼容策略，再输出可交付的工程变更和执行报告。

本技能强调“可实现、可验证、可回溯”：

- 实现前先产出最小可执行计划
- 实现中记录关键决策与风险
- 实现后产出验证证据与交接摘要

## Quick Contract

- **Trigger**：需求/方案已清晰，需要将计划落实为可提交代码变更。
- **Inputs**：`project_path`、`task_name`、实现目标、变更范围与验证等级。
- **Outputs**：实现计划、代码变更、验证记录与 handoff 摘要。
- **Quality Gate**：交接前必须通过 `## Quality Gates` 的计划先行与证据验证检查。
- **Decision Tree**：输入信号路由图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree)。

## Routing Matrix

- 统一路由对照见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md)。
- 阶段化上手视图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view)。
- 单页速查见 [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md)。
- 若出现冲突，以本技能 `## When to Use` 的**边界提示**为准。

## Announce

开始时明确说明：  
“我正在使用 `arc:implement`，先固化实现计划与验证路径，再落地代码。”

## The Iron Law

```
NO CODE CHANGE WITHOUT PLAN, EVIDENCE, AND ROLLBACK
```

未明确计划、验证证据与回退策略，不得实施代码改动。

## Workflow

1. 消费上游需求/方案产物并建立实现简报。
2. 生成最小实现计划，拆分可审查改动单元。
3. 分阶段实现并同步记录关键决策与风险。
4. 执行验证、输出交接材料并标注影响面。

## Quality Gates

- 必须先产出 `implementation-plan.md` 再编码。
- 每项关键改动必须有验证命令与结果。
- 交接摘要必须覆盖影响模块与回归关注点。
- 失败场景必须可回退，不允许不可逆操作。

## Red Flags

- 跳过计划直接大规模改代码。
- 只给“已完成”结论，没有验证证据。
- 变更范围失控，触碰无关目录。
- 无回退方案却改动关键路径。

## Mandatory Linkage（不可单打独斗）

1. 优先读取 `arc:deliberate` 产物（若存在）作为实现输入。
2. 需求模糊时先走 `arc:refine`，不得直接盲改。
3. 实现完成后建议交给 `arc:review` 或 `arc:simulate` 做验证闭环。
4. 输出标准化交接文件，供下游技能消费。

## When to Use

- **首选触发**：需求或方案已明确，需要产出可提交代码变更。
- **典型场景**：功能开发、接口演进、数据库迁移、行为不变重构、文档同步、缺陷修复并附验证证据。
- **边界提示**：需求模糊先 `arc:refine/arc:deliberate`，全面评估用 `arc:review`。

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `project_path` | string | 是 | 目标项目根目录绝对路径 |
| `task_name` | string | 是 | 本次实现任务名，用于目录命名 |
| `implementation_goal` | string | 否 | 实现目标摘要 |
| `change_scope` | string | 否 | 变更范围限制（模块/目录） |
| `verification_level` | enum | 否 | `basic` / `standard` / `strict`，默认 `standard` |
| `output_dir` | string | 否 | 默认 `<project_path>/.arc/implement/<task-name>/` |

## Dependencies

- **编排契约**（推荐）：遵循 `docs/orchestration-contract.md`，通过运行时适配层分发实现与验证任务。
- **ace-tool MCP**（必须）：定位实现入口、影响面、相关符号。
- **Dispatch API**（必须）：调度实现与验证任务。
- **arc:deliberate**（推荐）：消费上游方案文档。
- **arc:refine**（可选）：需求不清晰时补充上下文。
- **Exa MCP**（可选）：查官方文档与实现参考。

## Context Priority（强制）

0. `.arc/context-hub/index.json`（共享上下文索引，优先复用）
1. `.arc/implement/<task>/context/implementation-brief.md`（24h）
2. `.arc/deliberate/<task>/` 下方案产物（若存在）
3. `codemap.md`（arc:cartography）与 `CLAUDE.md` 层级索引（7天）
4. score 产物（由 `score/` 模块生成）/ `arc:review` / 上次 `arc:implement` handoff（若索引可用）
5. `ace-tool` 语义扫描
6. `Exa` 外部文档

失效回流规则：
- CLAUDE 索引失效：触发 `arc:init:update`
- codemap 失效：触发 `arc:cartography` 更新
- score/review 产物失效：触发 `score` 模块刷新（由 `arc:gate` 编排触发）/ `arc:review` 更新

## Critical Rules

1. **先计划再实现**：必须先产出 `plan/implementation-plan.md`。
2. **小步提交**：优先拆分为可审核的小变更，不做无界大改。
3. **证据验证**：实现后必须有验证记录（命令/结果/结论）。
4. **影响透明**：必须列出受影响模块与回归关注点。
5. **失败可回退**：保留回退思路，不允许“不可恢复式”变更。
6. **不越权执行**：不触碰与任务无关的目录与文件。

## Instructions

### Phase 1: 上下文建档

1. 读取上游产物与项目索引。
2. 识别实现入口、受影响模块、约束条件。
3. 生成 `context/implementation-brief.md`。
4. 执行脚手架命令：

```bash
python implement/scripts/scaffold_implement_case.py \
  --project-path <project_path> \
  --task-name <task_name>
```

### Phase 2: 实现计划

1. 产出 `plan/implementation-plan.md`：
- 目标与范围
- 变更步骤
- 风险与回退方案
- 验证计划
2. 若存在重大技术分歧，先回到 `arc:deliberate` 论证。

### Phase 3: 编码落地

1. 按计划逐步实现。
2. 每个步骤记录到 `execution/execution-log.md`：
- 修改点
- 原因
- 风险
- 验证情况

### Phase 4: 验证与交接

1. 执行最小必要验证（编译/测试/静态检查）。
2. 生成交付文档：
- `reports/execution-report.md`
- `handoff/change-summary.md`
3. 使用脚本渲染最终报告：

```bash
python implement/scripts/render_implementation_report.py \
  --case-dir <output_dir> \
  --task-name <task_name> \
  --result pass
```

## Scripts

```bash
python implement/scripts/scaffold_implement_case.py --project-path <project_path> --task-name <task_name>
python implement/scripts/render_implementation_report.py --case-dir <output_dir> --task-name <task_name> --result pass
```

## Artifacts

默认目录：`<project_path>/.arc/implement/<task-name>/`

- `context/implementation-brief.md`
- `plan/implementation-plan.md`
- `execution/execution-log.md`
- `reports/execution-report.md`
- `handoff/change-summary.md`

## Quick Reference

| 输出物 | 用途 |
|------|------|
| `implementation-plan.md` | 实现步骤与风险控制 |
| `execution-log.md` | 执行过程追踪 |
| `execution-report.md` | 实现结果与验证结论 |
| `change-summary.md` | 对下游技能/评审的交接摘要 |

## Anti-Patterns

**CRITICAL: The following behaviors are FORBIDDEN in arc:implement execution:**

### Planning Anti-Patterns

- **Speculation Implementation**: Writing code without reading arc:deliberate or arc:refine output first — must have approved plan
- **Scope Creep**: Adding features not in the approved plan — strict scope adherence required
- **Pattern Ignorance**: Not reading existing code patterns before implementing — causes inconsistency

### Implementation Anti-Patterns

- **Type Safety Violation**: Using `as any`, `@ts-ignore`, `@ts-expect-error` — forbidden without explicit exception approval
- **Empty Catch Blocks**: `catch(e) {}` — must handle or re-throw errors
- **Magic Values**: Hardcoding configuration values — use constants/config files
- **Commented Code**: Leaving commented-out code blocks — delete or implement

### Process Anti-Patterns

- **Skip Verification**: Not running LSP diagnostics after edits — must verify clean before commit
- **Batch Completion**: Marking multiple todos complete at once — one at a time, verify each
- **Cache Stale Usage**: Using expired codemap.md (7d+) without refresh — triggers arc:cartography
- **Handoff Skip**: Not generating `handoff/` artifacts — breaks downstream skills
