# Task File Templates

Load this file only when creating or rewriting the task entry, `00-前置约束.md`, task-group README, subtask file, or progress table.

All templates produce Markdown documents. Replace example names, paths, ratings, and commands with discovered project facts before delivery. Use the hierarchy and difficulty rules in [`authoring.md`](authoring.md); do not serialize these templates into JSON. Create only the frontend/backend work packages actually in scope.

## Task Entry

`NN-具体任务名/README.md` is the preferred entry document.

```markdown
# 具体任务名

本文是任务入口。具体任务和进度在当前目录。

## 使用方式

1. 先看 `00-前置约束.md` 和 `进度跟踪表.md`。
2. 按“业务任务组 → 后端/前端工作包 → 子任务”打开当前任务；每个子任务单独验收。
3. 完成后同步更新进度表和子任务状态。

## 执行模型假设

1. 本文档用于跨会话恢复、进度权威和验收，不是便宜模型的唯一内存。
2. 同会话多模型成本路由默认走 prewalk：Guide 探索 + 有界 todo + 第一笔生产代码 edit，再把 trajectory 交给 Executor。
3. 子任务仍须写清影响文件、顺序、边界、反例和验证，以便恢复与评审。

## 需求来源

1. [需求名称](../../00-产品需求/NN-需求名称.md)

## 设计边界

1. ...
2. 系统设计：链接到当前设计文档。
3. 接口基准：链接到 `docs/02-API记录/` 中的 Markdown 契约及章节。

## 拆分依据

1. 主要难点：当前业务状态、合同/数据风险、依赖与验证成本。
2. 分层：业务任务组 → 后端/前端工作包 → 独立验收的子任务。
3. 分端：说明后端和前端各自交付什么、哪些工作可并行、哪些依赖契约或真实接口。
4. 高难任务：说明先验证哪些未知项，以及为何保留或继续拆分复杂子任务。
5. 优先级 P0–P4 表示先后，难度 D1–D4 表示挑战程度，分别判断。

## 任务索引

| 编号 | 业务任务组 | 后端工作包 | 前端工作包 | 优先级 |
| --- | --- | --- | --- | --- |
| T01 | [任务组名称](tasks/T01-任务组名称/README.md) | [T01-BE](tasks/T01-任务组名称/backend/README.md) | [T01-FE](tasks/T01-任务组名称/frontend/README.md) | P1 |
```

For single-side work, remove the unused work-package column or write “不在范围内”; do not generate an empty package. Add integration links only when a separate joint acceptance task exists.

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
3. 接口和任务文档格式：Markdown；接口文档为开发、联调、验收基准，默认不使用 Apifox。
4. 前端/后端边界：各自目录、任务编号与允许改动范围；共享契约文档的相对链接。

## 难度与依赖评估

| 业务目标 | 主要难点与当前证据 | 拆分决定 | 前置条件/未知项 |
| --- | --- | --- | --- |
| 当前功能 | 代码、契约或验证路径中的具体复杂点 | 分端结果、D3 继续拆分或保留理由、D4 调研出口 | 上游产物及就绪条件 |

1. 每个叶子任务单独记录优先级、难度和判断依据。
2. 依赖或验证成本变化后，先更新本评估、任务卡与进度表，再继续实施。

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

## 工作包索引

| 编号 | 工作包 | 职责 | 依赖与可并行条件 |
| --- | --- | --- | --- |
| T01-BE | [后端](backend/README.md) | 接口行为、权限、业务规则和数据处理 | 契约与项目事实已对齐 |
| T01-FE | [前端](frontend/README.md) | 页面、交互、接口消费和状态呈现 | 链接同一契约，分别说明 UI 开发与真实联调条件 |

## 拆分理由

1. 说明主要复杂点、各端难度差异、独立验收边界和执行顺序。
2. 说明仍为 D3 的子任务为何不能继续合理拆分，或 D4 探索产出如何解锁实现。

## 验收标准

1. 标准一。
2. 后端和前端分别通过验收；要求跨端闭环时，还需验证真实用户流程。
3. 父级状态从子任务和整体验收汇总，不能用后端已完成代替整个功能已完成。
```

## Frontend / Backend Work Package README

Each side has its own README and leaf task files. The backend example below becomes `T01-FE` with frontend responsibilities in the frontend package. Other real work packages use their own accurate scope.

```markdown
# T01-BE 后端工作包

上级：[T01 任务组名称](../README.md)

## 交付边界

1. 负责的行为、允许改动的目录、调用点及验证面。
2. 不由本端实现的内容和下游交接产物。

## 难度与拆分理由

1. 主要难点及证据：已确认的代码、业务规则、数据风险、未知项和验证成本。
2. 拆分选择：为什么这些结果可以分别验收，哪些改动必须一起保证共同不变量。

## 契约与依赖

1. 共享 Markdown 接口文档及章节的相对链接。
2. 上游任务 ID、所需产物和可开始/可验收条件；无依赖时写“无”。
3. 可并行任务及受阻任务的范围。

## 子任务索引

| 子任务 | 优先级 | 难度 | 难度依据 | 依赖与就绪条件 |
| --- | --- | --- | --- | --- |
| [T01-BE-01 后端行为](T01-BE-01-后端行为.md) | P1 | D2 | 本项目的具体复杂点 | 无，或任务 ID + 所需产物/条件 |

## 验收

1. 本端结果的项目门禁与可达行为；执行状态统一查中心进度表和对应任务卡。
```

## Subtask File

Use a separate file per leaf. Frontend cards use `FE` IDs and frontend scope. Copying the same mixed implementation checklist into both sides is not a split.

````markdown
# T01-BE-01 后端行为

状态：`[ ]`

| 属性 | 内容 |
| --- | --- |
| 上级 | [T01-BE 后端工作包](README.md) |
| 所属端 | 后端（前端任务单独用 FE 编号） |
| 优先级 | P1，并写明业务先后依据 |
| 难度 | D2，按当前事实评估 |
| 难度依据 | 具体模块/状态、契约与数据风险、未知项、验证成本；附证据位置 |
| 前置依赖 | 任务链接 + 所需产物 + 就绪条件；无依赖写无 |
| 下游影响 | 消费本产物的任务或调用端 |

## 目标

一句话说明本端要完成的独立可验证结果。

## 范围与不做

1. 允许改动的文件、符号与行为；新文件说明已确认的所属模块。
2. 不属于本任务的另一端实现、无关重构和明确禁区。

## 输入

1. 输入文件或上游任务。
2. 最新项目状态依据：`path/to/file`、`command`、调用点或测试结果。
3. 共享 Markdown 接口契约及具体章节；无需接口时写明原因。

## 上下文检索

<Context-Queries>

```bash
arc-idx files '<已确认的文件片段>' --format json
arc-idx search 'f:<已确认的路径> <已确认的符号或文案>' --context 3 --max 10 --format ai-md
```

</Context-Queries>

以上占位命令必须在任务交付前换成已验证可用的精确查询；执行者开工先跑这些命令，再读取和编辑文件。

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
4. 难度或依赖发生变化时重新拆分并回写；D4 探索任务需写清待验证问题、证据产物和退出条件。

## 完成标准

1. 本端代码路径、项目门禁和目标面可达行为；列出命令/操作及预期结果。
2. 需要更新进度跟踪表。
3. 若本任务要求真实接口联调，Mock 通过后仍为待验证；后端通过不代替前端验收。
````

## Progress Table

Required columns: 状态, 编号, 所属端, 优先级, 难度, 子任务, 上级, 依赖/就绪条件, 下一步, 完成说明. Difficulty reasons live in each linked task card; use D1–D4 from [`authoring.md`](authoring.md), not priority values. List all executable leaves; parents are linked grouping nodes and are not extra completed tasks.

| 优先级 | 含义 |
| --- | --- |
| P0 | Blocks the main flow, risks data correctness, or blocks task decomposition |
| P1 | Core functionality or high-value governance |
| P2 | Normal implementation, tests, cleanup, documentation |
| P3 | Schedulable optimization or investigation |
| P4 | Explicitly low-priority backlog |

```markdown
# 进度跟踪表

| 状态 | 编号 | 所属端 | 优先级 | 难度 | 子任务 | 上级 | 依赖/就绪条件 | 下一步 | 完成说明 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `[ ]` | T01-BE-01 | 后端 | P1 | D2 | [后端行为](tasks/T01-任务组名称/backend/T01-BE-01-后端行为.md) | T01-BE | 无或已确认的上游条件 | 下一步动作 |  |
| `[ ]` | T01-FE-01 | 前端 | P1 | D1 | [前端行为](tasks/T01-任务组名称/frontend/T01-FE-01-前端行为.md) | T01-FE | T01-BE-01：契约已对齐可做 UI；真实接口可达后验收联调 | 下一步动作 |  |
```
