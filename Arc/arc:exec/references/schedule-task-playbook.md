# arc:exec 调度手册（`schedule_task` 示例）

## 1. 通用模板

```text
schedule_task(
  capability_profile="<profile>",
  capabilities=["<skill-a>", "<skill-b>"],
  description="<short description>",
  prompt="<detailed instruction>",
  execution_mode="background"
)
```

```text
collect_task(task_ref="<ref>", wait=true, timeout_sec=600)
```

## 2. 常用调度画像

| 场景 | capability_profile | capabilities |
|------|--------------------|--------------|
| 后端实现 | `deep` | `[]` 或 `["arc:build"]` |
| 前端实现 | `ui` | `["frontend-ui-ux"]` |
| 架构评审 | `architecture` | `["arc:decide"]` |
| 需求澄清 | `planning` | `["arc:clarify"]` |
| 方案复核 | `review` | `["arc:decide"]` |
| 仓库检索 | `search` | `[]` |
| 外部调研 | `research` | `[]` |
| 简单修复 | `quick` | `[]` |

## 3. 典型示例

### 3.1 后端子任务

```text
schedule_task(
  capability_profile="deep",
  capabilities=["arc:build"],
  description="实现订单服务 API",
  prompt="读取 <workdir>/services/order/ 下代码，新增查询接口并补充测试。",
  execution_mode="background",
  output_path="<workdir>/.arc/exec/order-api.md"
)
```

### 3.2 前端子任务

```text
schedule_task(
  capability_profile="ui",
  capabilities=["frontend-ui-ux", "arc:build"],
  description="实现订单详情页组件",
  prompt="基于现有设计系统实现详情页，保持响应式并补充交互状态。",
  execution_mode="background",
  output_path="<workdir>/.arc/exec/order-ui.md"
)
```

### 3.3 规划与复核

```text
schedule_task(
  capability_profile="planning",
  capabilities=["arc:clarify"],
  description="识别需求歧义并输出问题清单",
  prompt="输出范围边界、验收标准和依赖条件，写入 <output_path>。",
  execution_mode="foreground"
)
```

```text
schedule_task(
  capability_profile="review",
  capabilities=["arc:decide"],
  description="复核执行计划",
  prompt="检查任务顺序、风险与回滚路径，给出 OK/REVISE 结论。",
  execution_mode="foreground"
)
```

## 4. 并发收集与仲裁

1. 对同一波次并发任务保存 `task_ref` 列表。
2. 使用 `collect_task(...)` 逐个收集状态，直到全部完成或超时。
3. 若出现冲突（同一文件被不同任务修改）：
   - 优先级顺序：安全风险 > 业务正确性 > 代码风格。
   - 必要时追加 `review` 任务做冲突仲裁。
4. 聚合结论写入 `aggregation/final-summary.md`。

## 5. 调度提示词最小清单

每个 `prompt` 至少包含：

- 读取路径（必须明确到目录或文件）。
- 输出路径（必须可落盘）。
- 约束条件（风格、测试、性能或安全要求）。
- 失败处理（重试、降级或中止规则）。
