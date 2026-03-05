# ARC Orchestration Contract（运行时解耦）

本契约定义 ARC 技能在任意 Agent Runtime（Codex、Claude、自研编排器、CI Runner）中的统一调度语义。  
所有 `arc:*` 技能在文档中应使用本契约字段，避免绑定单一平台 API。

## 1) 核心调用语义

使用统一伪接口 `schedule_task(...)`：

```text
schedule_task(
  lane?: string,                 # 领域路由（如 deep / ui / writing）
  role?: string,                 # 专家角色路由（如 architecture / search / research）
  capabilities?: string[],       # 执行时注入的技能/能力上下文
  description: string,           # 短描述
  prompt: string,                # 详细执行指令
  execution_mode: "background" | "foreground",
  task_ref?: string       # 复用历史会话
)
```

约束：
- `capability_profile` 与 `capability_profile` 至少提供一个，推荐二选一。
- `execution_mode="background"` 用于可并发任务。
- 调度返回 `task_ref`（或平台等价字段）用于后续追问/反驳轮。

## 2) 结果收集语义

```text
collect_task(task_ref)
```

要求：
- 并发任务全部收齐后再进入聚合或收敛阶段。
- 失败任务要重试或降级，不得静默忽略。

## 3) 字段映射建议（适配层实现）

以下映射由“运行时适配层”负责，Skill 不直接耦合：

- `capability_profile` → 平台的“领域模型路由”字段
- `capability_profile` → 平台的“专家子代理路由”字段
- `capabilities` → 平台的“技能装载/工具预置”字段
- `execution_mode` → 平台的“同步/异步或后台执行”字段
- `task_ref` → 平台的“会话 ID / task ID / thread ID”

## 4) 写作规范（Skill 作者）

- 在 `SKILL.md` 中使用 `schedule_task(...)`、`collect_task(...)` 表述调度。
- 禁止把某平台专有 API 作为唯一执行路径。
- 如果必须给平台示例，放在“可选适配示例”并注明“非唯一实现”。
