# Prewalk Contract（轨迹交接，不是计划明信片）

本契约定义 Arc 在**同一次 agent session** 内做多模型成本路由时的默认语义。  
来源思想：Stencil `/prewalk`（[You only need the frontier model for one single edit](https://stencil.so/blog/prewalk)）。  
所有会改代码的 `arc:*` 技能在运行时支持模型切换时，必须遵守本契约；不支持切换时退回 oneshot，不得假装用「写完 plan 文档再冷启动便宜模型」来省钱。

## 1) 核心判断

- Agent 账单近似 `O(reads)`，不是 `O(edits)`。读代码占绝大部分 token；编辑通常 <10%。
- **理解活在 context 里**（已读文件、排除的死胡同、试过的假设），不在 2K 字的 plan 文档里。
- 因此默认禁止：`/plan` 式「frontier 只读规划 → 落盘 plan → 便宜模型从冷 context 按 plan 执行」。
- 默认允许并优先：`/prewalk` 式「frontier 探索 + todo + **第一笔有效 edit** → 把 trajectory 交给 executor，并剪掉 planning 指令」。

## 2) 角色

| 角色 | 能力档 | 职责 |
|---|---|---|
| **Guide**（frontier / deep） | 高价高能力 | 读代码、形成方案、初始化 todo、落地**第一笔**真实代码 edit |
| **Executor**（quick / flash 类） | 低价 | 继承同一 context 继续改、测、修；按 todo 推进 |

Guide 只为「读 + 想 + 起手第一刀」付费；Executor 承担后续 reads 与 edits。

## 3) 强制流程

```text
1. 以 Guide 启动任务
2. 注入隐藏规划指令（仅 Guide 可见）：plan deeply → capture plan as todo → start
3. Guide 探索代码库（search/read/grep/bash 等）
4. Guide 写出有界 todo（含每项验证步骤；限制条目数，避免 60 项刷分）
5. Guide 完成第一笔 **production code edit**（不是只改 docs/todo/comment）
6. Runtime 在 first-edit 边界：
   a. swap model → Executor
   b. prune 隐藏规划指令
   c. 保留完整 trajectory（工具结果、todo、已发生的 edit）
7. Executor 继续执行；todo / 进度表持续 steering
8. 验证失败时优先在 Executor 上修；Executor 连续失能再 escalate 回 Guide
```

### 3.1 切换门闩（Swap Gate）

同时满足才可 swap：

1. Todo 列表已初始化，且含至少一项带验证的 checklist。
2. 已发生至少一次 **production code** 的 `edit`/`write`（任务文档、进度表、注释-only 不算）。
3. 该 edit 与当前子任务目标一致，不是无关清理。

禁止的 swap 条件：

- 固定 turn 数（「第 4 轮就切」）—— Guide 可能仍在迷路，或已做完。
- 仅有 prose plan / 仅有 markdown 任务文档、尚无代码 edit。
- 清空 context 后只塞 plan 文件路径（冷启动 postcard）。

### 3.2 交接物

| 交接 | 状态 |
|---|---|
| 完整对话 / tool trajectory | **必须保留** |
| Todo / 进度表状态 | **必须保留并持续更新** |
| 已落地的第一笔 edit | **必须保留**（in-context 示范） |
| 隐藏 planning 指令 | **必须 prune** |
| 单独的 plan.md 作为主交接 | **禁止作为默认**；仅作人类/跨会话归档 |

## 4) 与 `arc:sdlc` 的分工

| 关切 | 权威 |
|---|---|
| 跨会话恢复、人类可读进度、审计战役拆分 | `arc:sdlc` 本地任务文档 |
| 同会话多模型成本路由 | **本契约 / `arc:prewalk`** |
| 任务文档是否详细 | 仍要具体（文件/边界/验证），但是为**可恢复与可验收**，不是为了替代 trajectory |

规则：

1. 大任务仍先过 `arc:sdlc` 门禁（docs + 进度表）。
2. 开始改代码时，若 runtime 支持模型切换：走 prewalk，不要「写完子任务 md → 新开 cheap 会话只读 md」。
3. 新会话冷启动时：用 Guide 做短 prewalk（读关键路径 + 对齐进度表 + 第一笔 edit）再 swap；或全程 oneshot Guide。**不要**默认 cheap 冷启动只读 plan。

## 5) Runtime 语义（编排契约扩展）

适配层应支持等价于：

```text
prewalk_session(
  guide_profile: string,              # e.g. deep / frontier
  executor_profile: string,           # e.g. quick / flash
  prompt: string,
  planning_instruction?: string,      # 默认内置；swap 时 prune
  swap_on: "first_production_edit",
  max_todo_items?: integer,           # 建议 ≤ 12
  escalate_on?: string[]              # e.g. executor_stuck, verification_loop
) -> {
  session_ref: string,
  phase: "guide" | "executor" | "escalated" | "done",
  swap_at?: string,                   # 何时发生 first edit
  status: ...
}
```

也可用已有 `schedule_task` 模拟：同一 `task_ref` 内 `capability_profile` 从 guide 切到 executor，**禁止**新建无 trajectory 的 task_ref 只塞 plan 路径。

## 6) 反模式清单

1. **Plan postcard**：Guide 读完全库写 plan，Executor 从零再读一遍。
2. **Telephone 规划**：复杂任务只做只读 plan 一轮，却不 sub-agent、不保留 context。
3. **无 todo 的 first-edit swap**：小模型容易宣告「做完了」；todo 提供持续 steering。
4. **无上限 todo**：Guide 造 60 项 checklist 刷进度；必须限项并要求验证步骤。
5. **把任务文档当成便宜模型的唯一内存**：跨会话归档可以，同会话执行不行。

## 7) 合规检查

1. 若宣称「多模型省成本」，是否在 first production edit 后仍保留 trajectory？
2. 是否 prune 了 planning 指令，避免 Executor 卡在 planning 身份？
3. 是否用 todo/进度表 steering，而不是依赖 Executor 背诵 plan 全文？
4. 是否避免「frontier 双倍读 + cheap 再读」？
5. 失败是否可 escalate，而不是静默用更烂的结果交差？
