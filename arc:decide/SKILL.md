---
name: "arc:decide"
description: "多视角方案决策与收敛：通过 architecture/deep/ui 辩论形成可执行计划；当用户说“技术选型/方案对比/架构决策/architecture trade-off”时触发。"
---

# arc:decide — multi-agent deliberation

## Overview

`arc:decide` 用共享文件系统组织多角色辩论（`architecture`、`deep`、`ui`），在“歧义澄清 → 方案博弈 → 计划固化”后再进入执行。  
核心目标不是“快速给答案”，而是输出可追踪、可复盘、可执行的决策依据。

## Quick Contract

- **Trigger**: 涉及技术选型、架构分歧、高风险改造或多目标权衡的任务。
- **Inputs**: `task_name`、`workdir`、增强提示词、轮次上限（`max_rounds`）。
- **Outputs**: 歧义清单、各角色方案/反驳、收敛报告、可执行计划（OpenSpec）。
- **Quality Gate**: 通过 `## Quality Gates` 的“证据、反驳、收敛、计划完整性”检查。
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:  
"I am using `arc:decide` to run multi-perspective deliberation and convergence before implementation."

## The Iron Law

```
NO CONSENSUS CLAIM WITHOUT CROSS-CRITIQUE EVIDENCE
```

没有跨角色反驳证据，不得宣布“已达成共识”。

## Workflow

1. 先做歧义检查，必要时回问用户补充边界。
2. 多角色并发提出方案，并进行交叉反驳。
3. 形成收敛结论并固化为可执行计划（OpenSpec）。
4. 仅在争议收敛后进入执行阶段。

## Quality Gates

- 每一轮必须有“提案 + 反驳 + 收敛判断”三类证据。
- 最终共识必须给出保留分歧与决策理由。
- 计划必须包含可执行任务、风险列表、回退策略。
- 外部信息引用需要标注来源与时效。

## Expert Standards

- 决策产物必须符合 `ADR` 结构（Context / Decision / Consequences / Alternatives）。
- 方案比较需要“加权评分 + 敏感性分析”，避免单点偏好。
- 高风险决策必须执行 `Pre-Mortem`，明确失败触发条件。
- 关键选型必须定义可验证 `Fitness Function`（性能、成本、稳定性阈值）。
- 结论需包含失效条件与可撤销路径。

## Scripts & Commands

- 运行时主命令：`arc decide`
- 估算模式：`arc decide --mode estimate`
- BDD 种子生成：`python3 arc:decide/scripts/generate_bdd_seed.py --consensus-report <report.md> --output <bdd-seed.yaml>`
- 调度长示例：`arc:decide/references/schedule-task-playbook.md`

## Red Flags

- 未做反驳直接宣布“最终方案”。
- 仅凭偏好，不给证据链与权衡依据。
- 歧义未清空就进入编码。
- 只保留单视角观点且未解释取舍。

## MCP tool usage

### Search external information (Internet)

Use **Exa MCP** to fetch standards, official docs, and recent engineering practices.

### Search project information

Use **ace-tool MCP** for semantic retrieval of code structure, dependencies, and existing patterns.

## When to Use

- **首选触发**: 用户要求“技术选型、架构方案比较、多方案取舍”。
- **典型场景**: 架构升级、跨团队改造、高风险性能/安全决策。
- **边界提示**: 若需求本身不清晰，先用 `arc:clarify`；若方案已定，转 `arc:build`。

## Core Pattern

### Input Arguments

| parameter | type | required | description |
|-----------|------|----------|-------------|
| `task_name` | string | yes | 任务名（用于工作目录命名） |
| `workdir` | string | yes | 工作目录绝对路径 |
| `enhanced_prompt_path` | string | no | 默认 `.arc/arc:decide/<task-name>/context/enhanced-prompt.md` |
| `max_rounds` | number | no | 方案辩论最大轮次，默认 3 |
| `max_ambiguity_rounds` | number | no | 歧义检查最大轮次，默认 3 |

### Directory Structure

```text
<workdir>/.arc/arc:decide/<task-name>/
├── context/
│   └── enhanced-prompt.md
├── agents/
│   ├── architecture/
│   ├── deep/
│   └── ui/
├── convergence/
│   ├── round-N-summary.md
│   └── final-consensus.md
├── openspec/
│   └── changes/<task-name>/
│       ├── proposal.md
│       ├── specs/<capability>/spec.md
│       ├── design.md
│       └── tasks.md
└── execution/
    └── implementation-log.md
```

## Dependencies

- **Organization Contract**: Required. Follow `docs/orchestration-contract.md`.
- **ace-tool MCP**: Required. 负责仓库语义检索与证据定位。
- **Exa MCP**: Recommended. 补充标准、规范和外部实现依据。
- **OpenSpec**: Required in planning phase (`proposal → specs → design → tasks`)。
- **arc:build**: Optional in execution phase when implementation is delegated.

## Agent Orchestration Profiles

| role | capability_profile | capabilities | output baseline |
|------|--------------------|--------------|-----------------|
| architecture | `architecture` | `["arc:decide"]` | 架构取舍与全局一致性 |
| deep | `deep` | `["arc:decide"]` | 工程可行性、性能与安全 |
| ui | `ui` | `["arc:decide", "frontend-ui-ux"]` | 体验、交互与可维护性 |

详细 `schedule_task(...)` 示例见 `arc:decide/references/schedule-task-playbook.md`。

## Instructions

### Phase 1: Ambiguity Check

1. 用 ace-tool MCP 与 Exa MCP 先补齐证据。
2. 三角色并发输出歧义清单（边界、术语、约束、依赖）。
3. 交叉反驳后汇总到 `convergence/round-N-summary.md`。
4. 仍有关键歧义时向用户澄清，直到可进入辩论阶段。

### Phase 2: Deliberation

1. 三角色并发提出独立方案（同轮次）。
2. 每个角色必须审阅并反驳另外两个角色方案。
3. 主流程汇总争议点并判断是否收敛。
4. 收敛后生成 `convergence/final-consensus.md`。

### Phase 3: Plan Generation (OpenSpec)

1. 初始化 OpenSpec 变更目录。
2. 按顺序生成 `proposal → specs → design → tasks`。
3. 三角色并发复核计划文件并提出修改建议。
4. 主流程合并评审意见，完成最终任务序列。
5. 运行 `openspec validate --change <task-name>` 确保结构完整。

### Phase 4: Execution

1. 根据 `tasks.md` 执行实现（可委托 `deep` 或转交 `arc:build`）。
2. 记录实施证据到 `execution/implementation-log.md`。
3. 验证通过后归档变更（`openspec archive <task-name>`）。

## Timeouts and downgrades

| condition | handling |
|-----------|----------|
| 角色任务超时（>10min） | 重试一次；仍失败则降级单角色补充论证 |
| 达到 `max_ambiguity_rounds` 仍有歧义 | 标记未解问题并请求用户决策 |
| 达到 `max_rounds` 仍未收敛 | 产出“保留分歧”版本并给推荐路径 |
| OpenSpec 命令失败 | 降级为手工生成 `tasks.md` 并标记风险 |

## status feedback

```text
[arc:decide] <task-name>
Phase 1: ambiguity check      [running/completed]
Phase 2: deliberation         [running/completed]
Phase 3: openspec planning    [running/completed]
Phase 4: execution            [running/completed]
```

## Quick Reference

| stage | key outputs |
|-------|-------------|
| 歧义检查 | `agents/*/ambiguity-round-N.md` |
| 方案辩论 | `agents/*/proposal-round-N.md`, `agents/*/critique-round-N.md` |
| 收敛 | `convergence/final-consensus.md` |
| 计划生成 | `openspec/changes/<task-name>/{proposal,design,tasks}.md` |
| 执行记录 | `execution/implementation-log.md` |

## Anti-Patterns

- **Premature Consensus**: 未完成交叉反驳就宣布收敛。
- **Echo Chamber**: 角色输出高度同质、没有真实冲突与取舍。
- **Spec Skip**: 不经 OpenSpec 校验直接进入实施。
- **Risk Blindness**: 忽略失败路径、回退条件与观测指标。
- **No Decision Trail**: 没有 ADR 证据链，后续无法复盘。

## Quick call method

| role | call pattern | mode |
|------|--------------|------|
| architecture | `schedule_task(capability_profile="architecture", capabilities=["arc:decide"], ...)` | background |
| deep | `schedule_task(capability_profile="deep", capabilities=["arc:decide"], ...)` | background |
| ui | `schedule_task(capability_profile="ui", capabilities=["arc:decide", "frontend-ui-ux"], ...)` | background |
| aggregation/finalization | 主流程直接处理 | foreground |
