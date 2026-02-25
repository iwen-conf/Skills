---
name: "arc:implement"
description: "面向软件需求的工程实现技能。消费 arc:refine/arc:deliberate 等上游产物，将方案落地为可提交代码变更，并输出实现计划、验证记录与交接报告。用于功能开发、重构落地、缺陷修复实现。"
---

# arc:implement — 方案落地实现

## Overview

`arc:implement` 负责把需求与方案落到代码实现层，输出可交付的工程变更和执行报告。

本技能强调“可实现、可验证、可回溯”：

- 实现前先产出最小可执行计划
- 实现中记录关键决策与风险
- 实现后产出验证证据与交接摘要

## Mandatory Linkage（不可单打独斗）

1. 优先读取 `arc:deliberate` 产物（若存在）作为实现输入。
2. 需求模糊时先走 `arc:refine`，不得直接盲改。
3. 实现完成后建议交给 `arc:review` 或 `arc:simulate` 做验证闭环。
4. 输出标准化交接文件，供下游技能消费。

## When to Use

- 需求已经明确，需要开始编码落地。
- 已有方案文档，需要转成可提交代码变更。
- 需要对现有模块进行重构并保持行为一致。
- 缺陷已定位，需要实施修复并给出验证证据。

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

- **ace-tool MCP**（必须）：定位实现入口、影响面、相关符号。
- **Task API**（必须）：调度实现与验证任务。
- **arc:deliberate**（推荐）：消费上游方案文档。
- **arc:refine**（可选）：需求不清晰时补充上下文。
- **Exa MCP**（可选）：查官方文档与实现参考。

## Context Priority（强制）

1. `.arc/implement/<task>/context/implementation-brief.md`（24h）
2. `.arc/deliberate/<task>/` 下方案产物（若存在）
3. 项目 `CLAUDE.md` 层级索引（7天）
4. `ace-tool` 语义扫描
5. `Exa` 外部文档

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
