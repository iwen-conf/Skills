# 同步工作流

各模式的执行步骤。模式判定、授权与输出模板见 [`../SKILL.md`](../SKILL.md)；`.lark.json` 字段见 [`index-contract.md`](index-contract.md)。所有命令按对应 lark-* skill 的参考文档执行，身份与授权按 [`lark-shared`](../../lark-shared/SKILL.md)。

## 1. 单资源

1. 路由到资源对应的 skill（§6），只创建或修改用户请求的资源。
2. 不建 `.lark.json`，不建工作区，不补其他资源。
3. 项目已有 `.lark.json` 且该资源属于某个资源键时：更新或新增对应条目，并追加一条 lifecycle。

## 2. 连接 / 恢复

1. 解析用户给出的链接，取得真实 token / ID，并确认当前身份有访问权限。
2. 从已解析的资源出发查找已有的项目主页、Drive 根目录、Wiki 节点和 Base 应用；已存在的只索引，不新建。
3. 写入 `.lark.json`：`resources.drive_folder` 必须是含 `url` 的非空条目；找不到 Drive 根目录时追加 blocker，不以其他资源充当。
4. 追加 lifecycle 条目，`resource_keys` 列出本次恢复的键。
5. 链接本身不授权全量创建；需要补齐资源时按 §4 的范围规则。

## 3. 全量创建

触发：全量创建短语，`workspace_scope=full`（用户明确缩小范围时只建所列资源）。只就缺失的项目名、负责人、认证输入或未解决的授权边界提问；已明确的授权不重复确认。

顺序：

1. 先解析已有资源（`.lark.json`、用户链接、同名项目主页），已存在的直接索引。
2. 先创建 Drive 根目录，立即写入 `resources.drive_folder`（含 `url`）。
3. 依次创建或解析下列资源；**每成功一个，立即把真实 URL/ID 写入 `.lark.json`**，再进行下一个：
   - Drive 子文件夹 `docs` `design` `engineering` `meetings` `releases` `incidents` `audits` `attachments` `exports` → `drive_folders`。
   - Docx：`project_home`、`prd`、`requirements`、`architecture`，以及交付、事故、审计和会议纪要文档（`delivery`、`incidents`、`audits` 等数组）。
   - Wiki：`wiki_space`、`wiki_node`。
   - Base：`requirements_base`、`sprint_base`、`task_base`、`bug_base`、`release_base`、`progress_base`、`risk_base`、`traceability_base`。
   - 仪表盘：项目概览、迭代 / 进度、风险 / 质量、发布状态，条目写 `source_keys`。
   - `lark_project`：迭代看板、里程碑与项目流程。
   - `calendar` 与计划中的 `meetings`；成员已知时建 `im_chat`；`mail_threads` 只在有正式交接时。
   - 画板：架构、流程、ER / 数据模型、时序 / 交互、评审 → `whiteboards`。
   - 平台支持时，状态、通知、审批、webhook 或 Base / Project 粘合 → `workflow_automations`。
   - `sheets`、`slides`、`apps`、`markdown_files`、`approvals`、`okrs`、`workflow_reports` 只在项目需要该格式或用户请求时创建。
4. 某个资源无法创建时：追加 `status: "blocked"` 的 lifecycle 条目（`resource_keys` + 原因），继续其余资源。
5. 全部完成后交叉链接：项目主页链接 Wiki、Base 表、仪表盘、Project、日历、群、画板和 Drive 文件夹。
6. 追加 `full_workspace_setup` 条目。存在 blocker 时结论为"部分完成"。

中断后恢复：读取 `.lark.json` 与 lifecycle 中的 blocker，先解析已存在的资源，再只创建缺失项。

## 4. 工作区更新

触发：工作区更新短语，`workspace_scope=workspace_update`。

1. 读取 `.lark.json`；不存在时必须有用户提供的已有项目主页或链接，否则不写入，先按 §2 连接。
2. 只校验与本次请求相关的索引项；修复失效的 URL/ID、缺失的索引项和断开的交叉链接。
3. 缺失的标准资源：只有索引记录为全量工作区，或用户要求补齐时，才按 §3 清单创建并索引；原范围未知不等于允许扩充。
4. 刷新 `task_base`，需求 / 迭代 / 缺陷 / 发布 / 进度 / 风险 / 追溯的 Base 视图，仪表盘，Project 流程，工作流自动化和 lifecycle 状态（各自存在时）。
5. 不建重复的 `project_home`、Base 应用、仪表盘集、`lark_project`、Wiki 节点或 Drive 根目录。
6. 权限、成员数据、认证或 API 覆盖阻塞某项更新时，追加 blocker，不省略该资源。
7. 追加 `full_workspace_update` 条目，`resource_keys` 列出变更的键。

## 5. 增量同步（task_base 与 lifecycle）

触发：项目已激活，且被跟踪的功能发生创建、变更、交付、阻塞或验证（包括 Arc 阶段收尾的状态交接、"同步到飞书"）。

1. `task_base` 不存在时，先用 lark-base 创建或解析并写入索引，再标记本次同步完成。
2. 每个功能保持一行当前记录：按 lifecycle 的 `task_record_id` 或 `title` 找到已有行则更新，否则新建。字段：

| 字段 | 内容 |
|---|---|
| `title` | 功能或任务名 |
| `owner` | 负责人（经 lark-contact 解析，不猜） |
| `status` | 用 Base 表已有的选项；没有时按来源阶段的状态写。未验证的工作不得写成完成 |
| `related_requirement` | 关联需求记录或文档链接 |
| `changed_files` | 来源阶段报告中的改动文件 |
| `verification` | 命令与结果、测试名；未执行项写 NOT_RUN / BLOCKED 与原因 |
| `lifecycle_entry` | 对应 lifecycle 条目的 `date` + `event` |
| `updated_at` | 本次写入的 UTC 时间 |

3. 写入前核对仓库：`changed_files` 存在于当前代码，`verification` 来自实际执行的结果。
4. 本次产生的持久材料（调研来源、文档、API 说明、架构事实、决策、截图、报告、导出、交接产物）写入 Docx / Wiki / Base / Drive / 画板 / 幻灯片，并从索引链接。
5. 追加 lifecycle 条目，`task_record_id` 写该行的 `record_id`。

## 6. 资源路由

| 资源 | Skill | `.lark.json` 键 |
|---|---|---|
| 认证、身份、高风险写入 | [lark-shared](../../lark-shared/SKILL.md) | `lark` |
| 项目主页、PRD、需求、架构、交付、审计、事故文档 | [lark-doc](../../lark-doc/SKILL.md) | `project_home` `prd` `requirements` `architecture` `delivery` `audits` `incidents` |
| Wiki 层级 | [lark-wiki](../../lark-wiki/SKILL.md) | `wiki_space` `wiki_node` |
| Drive 根目录、子文件夹、文件、导出、附件 | [lark-drive](../../lark-drive/SKILL.md) | `drive_folder` `drive_folders` |
| 需求、迭代、任务、缺陷、发布、进度、风险、追溯；仪表盘；Base 工作流 | [lark-base](../../lark-base/SKILL.md) | `*_base` `dashboards` `workflow_automations` |
| 个人提醒 | [lark-task](../../lark-task/SKILL.md) | `tasklist` |
| 迭代看板 / 项目流程；原生 API | [lark-openapi-explorer](../../lark-openapi-explorer/SKILL.md)（无专用 Project skill 时） | `lark_project` `native_openapi` |
| 计划中的会议与里程碑 | [lark-calendar](../../lark-calendar/SKILL.md) | `calendar` `meetings` |
| 已结束会议、妙记、实时参会 | [lark-meeting](../../lark-meeting/SKILL.md) | `meetings` `minutes` `live_meetings` |
| 架构 / 流程图 | [lark-whiteboard](../../lark-whiteboard/SKILL.md) | `whiteboards` |
| 项目群 / 正式邮件 | [lark-im](../../lark-im/SKILL.md) / [lark-mail](../../lark-mail/SKILL.md) | `im_chat` / `mail_threads` |
| 负责人、参会人 ID | [lark-contact](../../lark-contact/SKILL.md) | `project.owners` |
| 审批 / OKR | [lark-approval](../../lark-approval/SKILL.md) / [lark-okr](../../lark-okr/SKILL.md) | `approvals` / `okrs` |
| 表格 / 幻灯片 / Markdown / 应用 | [lark-sheets](../../lark-sheets/SKILL.md) / [lark-slides](../../lark-slides/SKILL.md) / [lark-markdown](../../lark-markdown/SKILL.md) / [lark-apps](../../lark-apps/SKILL.md) | `sheets` / `slides` / `markdown_files` / `apps` |
| 会议纪要汇总、日程待办报告 | [lark-workflow-meeting-summary](../../lark-workflow-meeting-summary/SKILL.md) / [lark-workflow-standup-report](../../lark-workflow-standup-report/SKILL.md) | `workflow_reports` |
| 事件订阅 / 考勤 | [lark-event](../../lark-event/SKILL.md) / [lark-attendance](../../lark-attendance/SKILL.md) | `event_subscriptions` / `attendance_records` |
| 重复操作封装为 skill | [lark-skill-maker](../../lark-skill-maker/SKILL.md) | `custom_lark_skills` |

组件边界：

| 组件 | 用于 | 不用于 |
|---|---|---|
| Docx | PRD、技术方案、架构叙述 | 状态机、问题跟踪 |
| Base | 需求记录、功能任务表、迭代、缺陷、发布、风险、追溯 | 长篇讨论 |
| Dashboard | 健康度、迭代、风险可视化，数据来自 Base 或 Project | 手写的 Doc 摘要 |
| Tasks | 个人提醒 | 功能交付状态、SDLC 主记录 |
| Project | 迭代看板、里程碑流程 | 知识归档 |
| Wiki | 长期知识层级与归档 | 活跃问题或工作流状态 |
| Workflow 自动化 | 资源之间的粘合 | 权威状态（权威状态在 Base、Project、Approval 或 `.lark.json`） |
| Calendar / 会议 / 实时参会 | 计划中的会议 / 已结束会议的纪要与妙记 / 正在进行的会议 | 互相替代 |
| IM / Mail | 内部快速协作 / 正式外部或收件箱交接 | 互相替代 |
| Whiteboard | 架构、流程、数据模型、交互结构比文字更易核对时 | 纯文字内容 |
| Approval / OKR | 存在真实关卡、目标、负责人或指标时 | 装饰性记录 |
| OpenAPI Explorer | 现有 lark-* skill 覆盖不到的 API | 已有 skill 覆盖的操作 |
| lark-skill-maker | 值得封装的重复飞书操作 | 一次性操作 |

## 7. 失败处理

| 失败 | 处理 | 停止条件 |
|---|---|---|
| 认证、scope 或身份错误 | 按 lark-shared 修复后重试同一调用 | 同一错误出现两次 → 追加 blocker 并报告 |
| 创建成功但写索引失败 | 立即重试写入；仍失败时在报告中列出资源链接与 ID | 不再创建新资源，先修复索引 |
| 批量或分页结果不完整 | 继续翻页或逐项检查结果 | 未读全时结论为"部分完成" |
| 资源已存在但未索引 | 解析并索引，不新建 | — |
| 索引项指向的资源已删除或无权限 | 标记该条目 `status` 并追加 lifecycle 说明；需要重建时按 §4 的范围规则 | 无权限 → BLOCKED，说明所需权限 |
