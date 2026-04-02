# ARC Orchestration Contract（运行时解耦）

本契约定义 ARC 技能在任意 Agent Runtime（Codex、Claude、自研编排器、CI Runner）中的统一调度语义。  
所有 `arc:*` 技能必须引用本契约，禁止把某单一平台 API 当作唯一实现。

## 1) 目标与边界

- 目标：让 `arc:*` 技能在不同运行时复用同一套调度语言（调度、收集、重试、降级、追踪）。
- 边界：本文件只定义“语义契约”，不绑定具体 SDK/CLI 参数名。
- 责任划分：Skill 描述“做什么”，适配层负责“如何在具体运行时调用”。

## 2) 核心接口

### 2.1 `schedule_task(...)`

```text
schedule_task(
  capability_profile: string,            # 必填；lane 或 role（如 deep/ui/architecture/search）
  capabilities?: string[],               # 可选；注入技能上下文（如 ["arc:decide", "frontend-ui-ux"]）
  description: string,                   # 必填；任务短描述
  prompt: string,                        # 必填；完整执行指令
  execution_mode?: "background" | "foreground",
  task_ref?: string,                     # 可选；复用历史会话
  timeout_sec?: integer,                 # 可选；超时时间
  output_path?: string,                  # 可选；约定输出产物路径
  metadata?: map<string, string>         # 可选；追踪字段（case_id, round, owner 等）
) -> {
  task_ref: string,
  status: "accepted" | "rejected",
  accepted_at?: string,
  reject_reason?: string
}
```

约束：
- `capability_profile` 必填。
- 并发任务必须显式使用 `execution_mode="background"`。
- 产物需要落盘时，应提供 `output_path` 并在 `prompt` 中重复声明。

### 2.2 `collect_task(...)`

```text
collect_task(
  task_ref: string,
  wait?: boolean,                        # 默认 true
  timeout_sec?: integer                  # wait=true 时生效
) -> {
  task_ref: string,
  status: "queued" | "running" | "succeeded" | "failed" | "timed_out" | "canceled" | "partial",
  output?: string,
  output_path?: string,
  error_code?: string,
  error_message?: string,
  retryable?: boolean,
  finished_at?: string
}
```

约束：
- 聚合前必须收齐全部并发任务状态。
- `failed/timed_out/partial` 不得静默吞掉，必须触发重试、降级或显式中止。

## 3) 任务生命周期与恢复策略

- 生命周期：`accepted -> queued -> running -> succeeded|failed|timed_out|canceled|partial`。
- 重试策略：优先重试 `retryable=true` 的失败，建议最多 2 次并记录重试原因。
- 降级策略：重试失败后可降级到 `quick` 或单 Agent 串行路径，并在结果中标注“降级执行”。
- 幂等要求：同一 `task_ref` 的后续调用只能续跑/补充，不得隐式开启新上下文。

## 4) 适配层映射清单（实现方）

运行时适配层至少要完成下列映射：

- `capability_profile` → 平台的模型/子代理路由字段。
- `capabilities` → 平台的技能加载或工具预置字段。
- `execution_mode` → 平台的同步/异步执行字段。
- `task_ref` → 平台的会话 ID / task ID / thread ID。
- `timeout_sec` → 平台超时参数（若不支持，适配层需自行实现轮询超时）。

## 5) Skill 写作规范（作者）

- 在 `SKILL.md` 中统一写 `schedule_task(...)` 与 `collect_task(...)`，不要写平台专有函数名。
- 若给平台示例，必须放在“可选适配示例”并注明“非唯一实现”。
- 调度示例应包含：`capability_profile`、`description`、`prompt`、`execution_mode`。
- 多 Agent 协同时必须写明：并发波次、收集点、冲突仲裁规则。

## 6) 最小合规检查（审计）

满足以下条件视为“符合编排契约”：

1. Skill 文档未绑定单一平台 API。
2. 所有并发任务均可被 `collect_task(...)` 收集。
3. 失败分支具备重试/降级/中止中的至少一种。
4. 最终报告包含 `task_ref` 或等价追踪标识。

## 7) 输出范式复用（聊天表现层）

- `arc:*` 技能可以复用融合型 generic skill 作为**聊天输出范式**，但不得因此篡改磁盘工件的原生格式。
- 当最终回答天然是紧凑的二维结构（如状态矩阵、对比表、短清单汇总）时，优先组合 `terminal-table-output`，在 fenced `text` 代码块中输出 Unicode 盒线表，而不是 Markdown 表。
- 当内容包含长段解释、文件路径、URL、代码、JSON、日志、长注释时，不得为了“表格化”而强行入格；应降级为列表、分组块或正文。
- Skill 作者在 `SKILL.md` 中引用这类输出范式时，必须明确区分：
  - **聊天摘要**：可使用 `terminal-table-output`
  - **落盘工件**：保持 Markdown / JSON / text / HTML 等原约定格式
