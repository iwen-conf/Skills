---
name: "arc:exec"
description: "统一任务编排入口：理解需求、路由到 arc:* 或多 Agent 协同执行；当用户说“帮我拉团队/不知道用哪个技能/split task/orchestrate this work”时触发。"
---

# arc:exec — unified orchestration entry

## Overview

`arc:exec` 是调度型 Meta-Skill，不直接产出业务方案，而是负责：

1. 识别用户目标与约束；
2. 在 `arc:*` 技能与通用多 Agent 执行之间做路由；
3. 组织并发调度、结果收集、冲突仲裁；
4. 输出可追踪的执行摘要与下一步建议。

## Quick Contract

- **Trigger**: 需求模糊、跨多个技能、或用户只给目标不指定执行路径。
- **Inputs**: `task_description`、`workdir`，可选 `preferred_skill`、`dry_run`、`confirm`、`snapshot`。
- **Outputs**: 路由决策、调度记录、执行预览、聚合结论与后续动作。
- **Quality Gate**: 通过 `## Quality Gates` 的“可解释路由 + 可追踪调度 + 可收敛聚合”检查。
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:  
"I am using `arc:exec` to complete requirement understanding, skill routing, and orchestration planning first."

## The Iron Law

```
ROUTE BEFORE EXECUTE, COLLECT BEFORE CONCLUDE
```

在完成路由与能力校验前不得调度；在收齐并发结果前不得给最终结论。

## Workflow

1. 扫描上下文并理解需求（目标、范围、风险、依赖）。
2. 决策执行路径：单技能、技能链路、或通用多 Agent 并发。
3. 生成执行预览（可选快照、确认、dry-run）。
4. 按波次调度并收集结果，执行冲突仲裁与降级处理。
5. 输出聚合摘要、风险和下一动作。

## Quality Gates

- 路由结论必须可解释（为何选该 Skill / profile）。
- 每次调度必须包含 `capability_profile`、`description`、`prompt`。
- 并发任务必须具备收集点（`collect_task(...)`）和失败处理策略。
- 聚合结论必须包含责任、风险、追踪标识（`task_ref` 或等价字段）。

## Expert Standards

- 编排输出必须给出 `RACI`：决策、执行、审阅、知会角色清晰。
- 调度计划必须标注 `关键路径(CPM)` 与并发波次，避免伪并发。
- 聚合阶段必须遵循 `冲突仲裁规则`（安全 > 正确性 > 一致性）。
- 结果摘要需给出置信度、降级条件和回滚路径。
- 涉及高风险改动时需明确人工确认闸门。

## Scripts & Commands

- 运行时主命令：`arc exec`
- 初始化编排工作区：`python3 arc:exec/scripts/scaffold_exec_case.py --workdir <workdir> --task-name <task-name>`
- 渲染调度与汇总产物：`python3 arc:exec/scripts/render_dispatch_report.py --case-dir <case_dir> --task-name <task-name> --dispatch "deep|[arc:build]|实现后端接口|done|execution/backend.md"`
- 调度示例手册：`arc:exec/references/schedule-task-playbook.md`

## Red Flags

- 未做路由直接进入编码。
- 并发任务无收集点、无失败分支。
- 忽略 `context-hub` 缓存导致重复扫描。
- 多 Agent 冲突未仲裁就直接拼接输出。
- `dry_run=true` 时仍执行真实变更。

## When to Use

- **首选触发**: 用户表达“帮我安排/拉团队执行”，或不确定应调用哪个 `arc:*`。
- **典型场景**: 需求跨多个阶段（澄清→决策→实现→验证）或全栈并发任务分解。
- **边界提示**: 若目标技能已明确且边界清晰，直接调用目标技能，不必经过 `arc:exec`。

## Input Arguments

| parameter | type | required | description |
|-----------|------|----------|-------------|
| `task_description` | string | yes | 用户任务描述 |
| `workdir` | string | yes | 工作目录绝对路径 |
| `preferred_skill` | string | no | 用户指定技能（命中则跳过路由） |
| `dry_run` | boolean | no | 仅输出执行预览，不实际执行 |
| `confirm` | boolean | no | 高影响操作前要求二次确认 |
| `snapshot` | boolean | no | 执行前生成可回滚快照 |

## Dependencies

- **Organization Contract**: Required. Follow `docs/orchestration-contract.md`.
- **ace-tool (MCP)**: Required. 用于仓库语义检索与影响面定位。
- **Exa MCP**: Optional. 用于外部资料补充与标准校验。
- **All arc: skills**: 路由到对应技能后必须遵循其 `SKILL.md`。

## Scheduling Playbook

`arc:exec` 的完整 `schedule_task(...)` 示例、参数组合、并发收集模板已抽出到：

- `arc:exec/references/schedule-task-playbook.md`

SKILL 正文只保留决策流程，避免过长与重复。

## Instructions

### Phase 1: Understand

1. 读取 `.arc/context-hub/index.json`（若存在），优先复用已生成产物。
2. 使用 ace-tool MCP 获取代码结构、关键模块和可疑影响面。
3. 必要时使用 Exa MCP 补充外部约束（标准、最佳实践、官方文档）。
4. 形成任务画像：类型、技术域、复杂度、风险等级、是否可并发。

### Phase 2: Route

1. 若 `preferred_skill` 可用，直接路由该技能。
2. 否则按 `docs/arc-routing-matrix.md` 进行技能匹配。
3. 无命中技能时，转入“通用多 Agent 调度”模式（Phase 3）。
4. 写入路由记录：`routing/dispatch-log.md`（理由、证据、置信度、降级路径）。

### Phase 2.5: Preview & Safety

1. 产出执行预览：计划动作、影响面、风险等级。
2. 若 `snapshot=true`，创建快照并记录回滚指令。
3. 若 `confirm=true` 或检测到高影响操作，先获得用户确认。
4. 若 `dry_run=true`，在本阶段结束后退出并返回预览结果。

### Phase 3: Dispatch

1. 将任务拆分为可并发子任务，按波次派发。
2. 每个子任务必须显式指定：
   - `capability_profile`
   - `capabilities`（如需要）
   - 读写路径和预期产物
3. 对同波次任务统一收集 `task_ref`。
4. 使用 `collect_task(...)` 收齐后再进入下一波次或聚合阶段。

### Phase 4: Aggregate

1. 汇总各子任务产物并检测冲突。
2. 发生冲突时按仲裁规则处理；必要时追加 `review` 调度。
3. 输出 `aggregation/final-summary.md`，包含：
   - 已执行动作；
   - 风险与残留问题；
   - 下一步建议与负责人。

## Outputs

```text
<workdir>/.arc/exec/<task-name>/
├── manifest.json
├── context/
│   └── task-brief.md
├── routing/
│   └── dispatch-log.md
├── preview/
│   └── execution-preview.md
├── dispatch/
│   └── task-board.md
├── aggregation/
│   └── final-summary.md
├── snapshots/
└── rollback/
    └── restore-notes.md
```

## Timeouts and downgrades

| condition | handling |
|-----------|----------|
| 单任务超时（>10min） | 重试一次；仍失败则降级为更小子任务 |
| 并发任务部分失败 | 标注 `partial` 并触发补偿调度 |
| 路由不确定 | 降级到 `arc:clarify` 获取约束后重路由 |
| 上下文失效/缓存过期 | 先触发刷新，再进入调度 |

## Quick Reference

| stage | key actions | artifacts |
|-------|-------------|-----------|
| Understand | context preflight + MCP scan | `context/task-brief.md` |
| Route | skill match + confidence + fallback | `routing/dispatch-log.md` |
| Preview | operation list + snapshot + confirm | `preview/execution-preview.md` |
| Dispatch | split + schedule + collect | `dispatch/task-board.md` |
| Aggregate | conflict arbitration + summary | `aggregation/final-summary.md` |

## Anti-Patterns

- **Shotgun Dispatch**: 无边界地启动多个 Agent 导致重复劳动。
- **Blocking Search**: 将 `search/research` 以前台串行执行，拖慢主链路。
- **Orphaned Tasks**: 不追踪 `task_ref` 导致结果不可回收。
- **Cache Blindness**: 明明有可复用产物却重复做同一分析。
- **Unsafe Execution**: 高风险操作无确认、无快照、无回滚。

## Context

调度前按以下优先级读取上下文：

1. `.arc/context-hub/index.json`（共享索引产物）
2. `.arc/<skill>/`（技能缓存产物）
3. 项目级 `CLAUDE.md`（项目规则/架构说明）
4. ace-tool MCP 扫描结果（最新代码现实）
5. Exa MCP（外部知识补充）

See `docs/orchestration-contract.md` and `docs/arc-routing-matrix.md` for details.
