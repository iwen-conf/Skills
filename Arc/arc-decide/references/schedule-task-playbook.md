# arc-decide 调度示例手册

本文件承载 `arc-decide` 的长示例，`SKILL.md` 仅保留核心流程与入口。

## 1. 角色映射

| 角色 | capability_profile | capabilities | 主要职责 |
|------|--------------------|--------------|----------|
| architecture | `architecture` | `["arc-decide"]` | 架构推理、全局一致性 |
| deep | `deep` | `["arc-decide"]` | 工程可行性、性能与安全 |
| ui | `ui` | `["arc-decide", "frontend-ui-ux"]` | 体验、交互与可维护性 |

## 2. Phase 1（歧义检查）示例

```text
schedule_task(
  capability_profile="architecture",
  capabilities=["arc-decide"],
  description="architecture ambiguity analysis",
  prompt="读取 enhanced-prompt.md，输出边界、术语、约束歧义。",
  execution_mode="background",
  output_path="agents/architecture/ambiguity-round-1.md"
)
```

```text
schedule_task(
  capability_profile="deep",
  capabilities=["arc-decide"],
  description="deep ambiguity analysis",
  prompt="从工程实现角度识别歧义，覆盖性能/安全/依赖假设。",
  execution_mode="background",
  output_path="agents/deep/ambiguity-round-1.md"
)
```

```text
schedule_task(
  capability_profile="ui",
  capabilities=["arc-decide", "frontend-ui-ux"],
  description="ui ambiguity analysis",
  prompt="从交互与体验角度识别歧义并给出澄清问题。",
  execution_mode="background",
  output_path="agents/ui/ambiguity-round-1.md"
)
```

## 3. Phase 2（方案辩论）示例

```text
schedule_task(
  capability_profile="architecture",
  capabilities=["arc-decide"],
  description="architecture proposal round N",
  prompt="输出架构方案并说明取舍依据。",
  execution_mode="background",
  output_path="agents/architecture/proposal-round-N.md"
)
```

```text
schedule_task(
  capability_profile="deep",
  capabilities=["arc-decide"],
  description="deep proposal round N",
  prompt="输出工程实现方案并评估性能与安全成本。",
  execution_mode="background",
  output_path="agents/deep/proposal-round-N.md"
)
```

```text
schedule_task(
  capability_profile="ui",
  capabilities=["arc-decide", "frontend-ui-ux"],
  description="ui proposal round N",
  prompt="输出 UI/UX 与前端实现方案，强调可访问性与维护性。",
  execution_mode="background",
  output_path="agents/ui/proposal-round-N.md"
)
```

## 4. Phase 3（OpenSpec 计划）示例

```text
schedule_task(
  capability_profile="deep",
  capabilities=["arc-decide"],
  description="generate openspec artifact",
  prompt="基于 final-consensus.md 生成 proposal/specs/design/tasks。",
  execution_mode="background",
  output_path="openspec/changes/<task-name>/tasks.md"
)
```

```text
schedule_task(
  capability_profile="architecture",
  capabilities=["arc-decide"],
  description="architecture plan review",
  prompt="复核 OpenSpec 计划，指出架构一致性与依赖问题。",
  execution_mode="background",
  output_path="agents/architecture/plan-review.md"
)
```

## 5. Phase 4（执行）示例

```text
schedule_task(
  capability_profile="deep",
  capabilities=["arc-decide", "arc-build"],
  description="execute final plan",
  prompt="按 tasks.md 顺序实现并记录验证证据。",
  execution_mode="foreground",
  output_path="execution/implementation-log.md"
)
```

## 6. 收集与迭代

```text
collect_task(task_ref="<ref>", wait=true, timeout_sec=600)
```

- 若结果为 `failed/timed_out`：最多重试 2 次并记录原因。
- 若多轮仍冲突：由主流程输出 `convergence/round-N-summary.md`，进入下一轮或强制收敛。
