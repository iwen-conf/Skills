---
name: lark-workflow-project-docs
version: 2.0.0
description: "飞书项目工作区工作流：创建、连接、刷新项目的飞书/Lark 工作区，并维护项目根目录 .lark.json 索引；支持全量创建、工作区更新、增量同步（task_base 进度行与 lifecycle）和已索引项目的单资源同步。触发：创建飞书项目空间、初始化飞书项目空间、更新/刷新/补齐飞书项目空间、同步到飞书、飞书项目文档、项目工作区、索引飞书资源、项目根目录已有 .lark.json；create / update / refresh Lark project space, Lark project workspace, sync to Lark. Not for writing local PRDs or task docs (use arc:plan), or editing a single Lark doc or Base table outside a project index (use lark-doc / lark-base)."
metadata:
  requires:
    bins: ["lark-cli"]
    skills: ["lark-shared"]
---

# 飞书项目工作区工作流

**CRITICAL — 开始前 MUST 先完整读取 [`../lark-shared/SKILL.md`](../lark-shared/SKILL.md)**。认证、身份（`--as user` / `--as bot`）、scope、写入与删除授权、高风险确认以 lark-shared 为准；每类资源的命令与参数以对应 lark-* skill 为准。本 skill 只负责编排工作区资源和维护 `.lark.json`。

## 适用场景

- **全量创建**：创建项目的飞书空间、创建飞书项目空间、创建完整飞书项目空间、一键创建飞书项目空间、初始化飞书项目空间、把这个项目纳入飞书、create Lark project space、create full Lark workspace、create Lark workspace、initialize Lark project space、enable Lark。
- **工作区更新**：更新飞书项目空间、刷新飞书项目空间、补齐飞书项目空间、同步飞书项目空间、update Lark project space、refresh Lark project space、complete Lark project space。
- **连接 / 恢复**：用户给出已有的项目空间、Drive 文件夹、项目主页、Wiki、Base 或项目链接，要求连接、索引或恢复；索引飞书资源。
- **增量同步**：同步到飞书、sync to Lark、飞书项目文档；项目根目录已有 `.lark.json`，且有被跟踪的功能状态需要回写（包括 Arc 阶段收尾的状态交接）。
- **单资源**（项目已有 `.lark.json` 时）：创建飞书文档、创建飞书项目文档、创建飞书任务表、创建飞书仪表盘、生成飞书PRD、create Lark docs、create Lark task table、create Lark dashboard。

## 不适用

- 编写本地 PRD、简报或任务文档 → `arc:plan`。飞书表格不替代本地任务文档。
- 没有 `.lark.json`、也没有项目空间措辞的单个文档或表格操作 → 直接用 [`lark-doc`](../lark-doc/SKILL.md)、[`lark-base`](../lark-base/SKILL.md) 等对应 skill。
- 认证、身份、scope 问题 → [`lark-shared`](../lark-shared/SKILL.md)。
- 没有任何飞书触发的本地代码工作 → 不激活，不提示。

## 激活状态

| 状态 | 条件 | 行为 |
|---|---|---|
| 已激活 | 项目根目录有 `.lark.json` | 先读并校验索引；后续搜索、更新、删除、创建都用其中的 URL/ID |
| 连接 | 用户给出已有项目链接并要求连接 / 索引 / 恢复 | 解析链接，写入真实索引项，不建重复资源 |
| 全量创建 | 出现全量创建短语 | 按全量契约一次性创建或解析，并逐项写入索引 |
| 工作区更新 | 出现工作区更新短语 | 校验现有索引，修复与刷新，不重复创建 |
| 单资源 | 明确的单个文档 / 表格 / 任务请求，没有项目空间措辞 | 只做该资源；不建 `.lark.json`，不建工作区；已索引时维护对应索引项 |
| 未激活 | 以上都不满足 | 不创建飞书资源，不创建 `.lark.json` |

判定规则：

1. 不从项目规模、仓库结构、长期工作或文档需求推断要建项目空间。
2. `.lark.json` 不存在时，只有"创建 / 连接 / 索引 / 更新项目空间"的明确措辞能创建或修改它。
3. 只为读写某个文档而给出的链接属于单资源，不授权连接或全量工作区。
4. 没有 `.lark.json` 时，"同步到飞书"按单资源处理：只同步用户点名的材料，不建索引。

## 输入

| 参数 | 必填 | 说明 |
|---|---|---|
| `project_path` | 是 | 项目根目录 |
| `project_name` | 否 | 项目名；创建资源时缺失则询问 |
| `lark_home` | 否 | 已有项目主页 URL 或 token |
| `workspace_scope` | 否 | `minimum`（`drive_folder` + `project_home`）、`normal`（minimum 加用户点名的资源）、`full`（全量契约）、`workspace_update`。全量创建短语默认 `full`，除非用户明确缩小范围 |
| `doc_scope` | 否 | `prd`、`requirements`、`architecture`、`progress`、`audit`、`delivery`、`incidents` 或 `all` |
| `progress_mode` | 否 | `doc`、`base`、`task` 或 `none` |
| `owners` | 否 | 负责人；需要飞书 ID 时用 [`lark-contact`](../lark-contact/SKILL.md) 解析 |

## 工作流

```
项目根目录 ─► 读 .lark.json / docs/ ─► 判定状态与模式 ─► 授权（lark-shared）
                                            │
              ┌──────────┬──────────┬───────┴──────┬──────────┐
              ▼          ▼          ▼              ▼          ▼
           单资源      连接      全量创建      工作区更新   增量同步
              └──────────┴──────────┴───────┬──────┴──────────┘
                                            ▼
                    每次创建后立即写索引 ─► 回读校验 ─► lifecycle ─► 交接报告
```

### Step 1: 发现与判定（任何写入之前）

1. 检查项目根目录的 `.lark.json` 和已有 docs 目录。
2. 按激活状态判定模式，输出一行：`模式 · workspace_scope · 涉及的资源键`。未激活时直接结束飞书部分，继续本地工作。
3. 已有 `.lark.json` 时，先读取并校验与本次请求相关的索引项（链接可打开、类型匹配），再做任何飞书更新。
4. 需要仓库证据时，先用 `arc-idx search` / `arc-idx symbol` 定位再读文件，不线性通读目录。

### Step 2: 授权与身份

按 lark-shared 处理身份、scope 与写入授权。同一目标、动作和影响的既有明确授权继续有效；只就未解决的目标、内容、影响或缺失的必填输入提问。

### Step 3: 按模式执行

逐模式的步骤、全量资源清单、更新契约与 task_base 契约见 [`references/sync-workflow.md`](references/sync-workflow.md)。资源与 skill 的对应关系见其中的资源路由表。

### Step 4: 写入 `.lark.json`

按 [`references/index-contract.md`](references/index-contract.md) 写入。每创建或解析成功一个资源，立即写入它的真实 URL/ID；创建失败时追加 `status: "blocked"` 的 lifecycle 条目。写入 `local_index` 前运行 `arc-idx doctor --format json`，按索引契约映射 `status`。

### Step 5: 回读校验与交接

回读每个本次创建或更新的资源，确认存在且可打开，再按输出模板交接。部分创建不是完整成功。

## 规则

1. 索引项必须对应真实资源。`.lark.json` 只存 URL、ID、token、负责人、状态和时间戳；不存密钥、访问令牌、刷新令牌、cookie、私钥、凭据或文档正文。
2. 托管资源未写入索引时，不得宣称成功；绝不预填猜测的 ID。
3. 一个仓库只有一个 `project_home`。更新时不建重复的工作区、项目主页、Base 应用、仪表盘集、Wiki 节点、Drive 根目录或飞书项目。
4. lifecycle 只追加；只有链接失效或原条目事实错误时才改写历史。
5. 已激活项目中被跟踪的功能发生创建、变更、交付、阻塞或验证时，必须更新 `task_base`；`tasklist` 不能替代。
6. 已激活项目在工作中产生的持久材料（调研来源、新文档、API 说明、架构事实、决策、截图、报告、导出、交接产物）必须写入飞书资源并从索引链接，不能只留在对话里。
7. 写入飞书的每条结论都能追溯到仓库（代码、SQL、路由、测试、配置）或用户提供的来源。Base 记录不是实现真相：写进度前核对代码与验证结果。
8. 不猜 URL、token、表 ID、chat ID、tasklist ID、日历 ID、会议 ID 或审批 ID；负责人、参会人和审批数据不猜，经 lark-contact 或用户确认。
9. 只选择本次范围需要的最小资源集；每个被选用的飞书组件都要有生命周期理由。组件边界见 sync-workflow 的资源路由。
10. 一旦存在远端文档，`.lark.json` 中必须已有 `project_home`、`local_index` 和 `project` 信息；存在多个资源时，需求、交付、事故、审计、会议、风险与任务互相链接。

## 输出

```text
项目工作区交接
- 模式与范围：<模式> · <workspace_scope>
- .lark.json：创建 / 更新 / 未改动（变更的资源键）
- 资源：<键> → <链接> · created / resolved / updated / blocked（原因）
- task_base：<record_id> · <status>（已激活且有被跟踪功能时）
- local_index：<status> · <last_refreshed_at>
- lifecycle：追加的 event 与 resource_keys
- 证据来源：仓库文件、命令结果或用户提供的链接
- 结论：完成 / 部分完成 / BLOCKED / NOT_RUN，以及缺口和下一步
```

## 参考

- [lark-shared](../lark-shared/SKILL.md) — 认证、身份、写入授权（必读）
- [索引契约](references/index-contract.md) — `.lark.json` 结构、资源条目、lifecycle、命名与 ID 规则
- [同步工作流](references/sync-workflow.md) — 各模式步骤、全量 / 更新 / task_base 契约、资源路由
- [lark-doc](../lark-doc/SKILL.md)、[lark-base](../lark-base/SKILL.md)、[lark-drive](../lark-drive/SKILL.md)、[lark-wiki](../lark-wiki/SKILL.md) — 文档、多维表格、云空间、知识库
