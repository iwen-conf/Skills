# Task File Templates

Load this file only when creating or rewriting the task entry, `00-前置约束.md`, task-group README, subtask file, or progress table.

## Task Entry

`NN-具体任务名/README.md` is the preferred entry document.

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

## Pre-Constraints

`00-前置约束.md` is mandatory before coding.

```markdown
# 前置约束

## 目标

1. ...

## 不做范围

1. ...
2. （用户明确 Hold / P4 / 只读 / 禁止项必须原样写入）

## 边界约束

1. ...
2. 目标环境/分支/部署面：local / `.26` / production / 双轨选择（如有）

## 完成定义

1. `[x]` 需要：代码路径 + 项目门禁 + 目标面上的可达行为（见 `docs/execution-truth.md`）。
2. 禁止仅凭能力矩阵、文档勾选或口头「已修好」结案。

## 执行模型假设

1. 任务文档是跨会话与进度权威；同会话多模型默认 prewalk，禁止「只交 plan 文件给冷启动便宜模型」。
2. 不允许用模糊描述替代具体文件、调用点、数据契约、边界条件、错误处理和验证要求。
3. 新会话从文档恢复时：短 Guide 再 grounding + first edit，或 oneshot Guide。

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

When the source is security/audit, also include 项目定位, 项目口径, 功能角色, 上游 Handoff, Finding 索引.

## Task Group README

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

## Subtask File

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

## Progress Table

Required columns: 状态, 编号, 优先级, 子任务, 上级, 下一步, 完成说明.

| 优先级 | 含义 |
| --- | --- |
| P0 | Blocks the main flow, risks data correctness, or blocks task decomposition |
| P1 | Core functionality or high-value governance |
| P2 | Normal implementation, tests, cleanup, documentation |
| P3 | Schedulable optimization or investigation |
| P4 | Explicitly low-priority backlog |

```markdown
# 进度跟踪表

| 状态 | 编号 | 优先级 | 子任务 | 上级 | 下一步 | 完成说明 |
| --- | --- | --- | --- | --- | --- | --- |
| `[ ]` | T01-01 | P0 | [子任务名称](tasks/T01-任务组名称/T01-01-子任务名称.md) | T01 | 下一步动作 |  |
```
