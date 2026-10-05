# `.lark.json` 索引契约

项目根目录的 `.lark.json` 只在飞书已激活的项目中存在（激活条件见 [`../SKILL.md`](../SKILL.md) 的激活状态）。字段结构保持不变，已有项目的文件无需迁移。

## 1. 结构

```json
{
  "schema_version": "1.1.0",
  "project": {
    "name": "Project Name",
    "root": ".",
    "description": "",
    "owners": [],
    "repository": ""
  },
  "lark": {
    "tenant": "",
    "identity": "user",
    "created_at": "2026-01-01T00:00:00Z",
    "updated_at": "2026-01-01T00:00:00Z"
  },
  "resources": {
    "wiki_space": null,
    "wiki_node": null,
    "drive_folder": null,
    "drive_folders": {},
    "project_home": null,
    "prd": null,
    "requirements": null,
    "architecture": null,
    "requirements_base": null,
    "sprint_base": null,
    "progress_base": null,
    "task_base": null,
    "bug_base": null,
    "release_base": null,
    "risk_base": null,
    "traceability_base": null,
    "dashboards": [],
    "tasklist": null,
    "lark_project": null,
    "calendar": null,
    "live_meetings": [],
    "meetings": [],
    "minutes": [],
    "im_chat": null,
    "mail_threads": [],
    "whiteboards": [],
    "sheets": [],
    "approvals": [],
    "okrs": [],
    "apps": [],
    "markdown_files": [],
    "slides": [],
    "workflow_reports": [],
    "workflow_automations": [],
    "event_subscriptions": [],
    "attendance_records": [],
    "native_openapi": [],
    "custom_lark_skills": [],
    "delivery": [],
    "incidents": [],
    "audits": [],
    "thesis_support": []
  },
  "local_index": {
    "tooling": ".ai-code-index",
    "reindex_command": "arc-idx index",
    "search_command": "arc-idx search \"query\" --format json",
    "struct_search_command": "arc-idx ast --lang <language> '<pattern>'",
    "symbols_command": "arc-idx symbol <name>",
    "last_refreshed_at": "",
    "status": "unknown"
  },
  "lifecycle": []
}
```

- 未使用的资源保持 `null` 或空数组；只在实际使用时填入条目。
- 重复出现的资源（会议、妙记、画板、表格、审批、交付、事故、审计等）用数组。
- 文件保持在 diff 中可审阅的大小；不存密钥、令牌、cookie、私钥、凭据或文档正文。

## 2. 资源条目

非空资源使用以下对象。未知字段不写，不用占位值。

```json
{
  "type": "docx",
  "title": "Project Name Requirements",
  "url": "https://...",
  "token": "optional-document-token",
  "id": "optional-resource-id",
  "table_id": "optional-base-table-id",
  "record_id": "optional-base-record-id",
  "node_token": "optional-wiki-node-token",
  "chat_id": "optional-chat-id",
  "folder_token": "optional-drive-folder-token",
  "local_path": "docs/requirements.md",
  "owner": "lark-workflow-project-docs",
  "source": "lark-cli docs +create",
  "updated_at": "2026-01-01T00:00:00Z",
  "status": "active"
}
```

`type` 取值：`wiki_space` `wiki_node` `drive_folder` `drive_file` `docx` `base` `base_table` `task_base` `dashboard` `tasklist` `task` `lark_project` `calendar` `calendar_event` `live_meeting` `vc_meeting` `minutes` `im_chat` `mail_thread` `whiteboard` `sheet` `approval` `okr` `app` `markdown_file` `slide` `workflow_report` `workflow_automation` `event_subscription` `attendance_record` `native_openapi` `custom_lark_skill` `local_file` `external`。

ID 与 token 字段：

| 字段 | 含义 |
|---|---|
| `url` | 可直接打开的飞书链接 |
| `token` | lark-cli 返回的文档或对象 token |
| `id` | 持久资源 ID：Base app token、tasklist GUID、chat ID、日历 ID、审批 code、事件 ID |
| `table_id` / `record_id` | Base 表 ID / 记录 ID（指向具体表或跟踪行时） |
| `source_keys` | 仪表盘或自动化的数据源资源键，如 `progress_base`、`risk_base`、`traceability_base`、`lark_project` |
| `node_token` | 放入 Wiki 层级时的节点 token |
| `folder_token` | Drive 文件操作用的文件夹 token |

## 3. 资源键

| 键 | 内容 |
|---|---|
| `wiki_space` / `wiki_node` | 项目知识空间 / 空间内项目节点（通常指向项目主页） |
| `drive_folder` | 项目 Drive 根目录；项目空间创建或连接后必须是含 `url` 的非空条目，作为后续文档搜索、更新、删除、创建与归档的入口 |
| `drive_folders` | 命名子文件夹：`docs` `design` `engineering` `meetings` `releases` `incidents` `audits` `attachments` `exports` |
| `project_home` | 项目主页 Docx，链接各资源与最新状态；每个仓库一个 |
| `prd` / `requirements` / `architecture` | PRD / 需求与验收标准 / 架构说明、图、数据模型与技术决策 |
| `requirements_base` | 需求表：负责人、优先级、验收标准、状态、关联文档 |
| `sprint_base` | 迭代表：日期、范围、负责人、状态、关联任务 |
| `progress_base` | 里程碑进度：负责人、状态、验证 |
| `task_base` | 被跟踪功能的结构化任务表（§4） |
| `bug_base` | 缺陷：严重度、负责人、状态、复现、修复、验证 |
| `release_base` | 发布：版本、范围、状态、检查单、审批、交付链接 |
| `risk_base` / `traceability_base` | 风险与审计发现 / 需求 → 代码 → 测试追溯 |
| `dashboards` | 基于 Base 或 Project 结构化数据的仪表盘，条目带 `source_keys` |
| `tasklist` | 个人提醒与轻量跟进 |
| `lark_project` | 迭代看板、里程碑与流程 |
| `calendar` / `meetings` / `minutes` / `live_meetings` | 项目日历 / 会议记录 / 妙记 / 实时参会会话 |
| `im_chat` / `mail_threads` | 项目群 / 正式外部交接邮件 |
| `whiteboards` | 架构、流程、ER、时序、泳道、用例或评审画板 |
| `sheets` / `slides` / `apps` / `markdown_files` | 轻量表格 / 演示 / 已发布应用或原型 / 经飞书管理的 Markdown |
| `approvals` / `okrs` | 需求、发布或审计审批 / 与项目关联的目标与指标 |
| `workflow_reports` / `workflow_automations` | 会议纪要、日程待办等工作流产出 / 状态流转、通知、审批、webhook 粘合 |
| `event_subscriptions` / `attendance_records` | 实时事件订阅 / 考勤或合规证据 |
| `native_openapi` / `custom_lark_skills` | 现有 skill 未覆盖的原生 OpenAPI / 为重复操作封装的 skill |
| `delivery` / `incidents` / `audits` | 交付说明 / 事故、根因与修复报告 / 审计或评审报告 |
| `thesis_support` | 论文支撑文档；保留已有条目 |

## 4. `local_index`

写入前运行 `arc-idx doctor --format json`，`last_refreshed_at` 记为该次运行的 UTC 时间：

| doctor 结果 | `status` |
|---|---|
| `healthy: true` 且 `index_stale: false` | `healthy` |
| `index_stale: true` | `stale` |
| `healthy: false` | `unhealthy` |
| `arc-idx` 不可用 | `unknown` |

`*_command` 字段原样保留。

## 5. lifecycle

每个有意义的项目事件追加一条；只有链接失效或原条目事实错误时才改写历史。旧条目中的 `skill` 值保持原样，不改成新名称。

```json
{
  "date": "2026-01-01T00:00:00Z",
  "skill": "arc:code",
  "event": "delivery",
  "summary": "Implemented login flow",
  "local_paths": ["src/auth/Login.tsx"],
  "remote_urls": ["https://..."],
  "resource_keys": ["project_home", "progress_base", "task_base"],
  "task_record_id": "optional-base-record-id",
  "owner": "open_id_or_name",
  "status": "completed"
}
```

- `skill`：产生该事件的阶段技能名，或本 skill 名 `lark-workflow-project-docs`。
- 阻塞条目：`status: "blocked"`，`resource_keys` 写预期资源，`summary` 写简短原因。

常用 `event`：

| event | 典型 `skill` | `resource_keys` |
|---|---|---|
| `project_definition` | `arc:plan` | `project_home` `prd` `wiki_node` |
| `requirements_update` | `arc:plan` | `requirements` `task_base` `progress_base` `traceability_base` `dashboards` |
| `full_workspace_setup` | `lark-workflow-project-docs` | 全量契约中创建或解析的全部键 |
| `full_workspace_update` | `lark-workflow-project-docs` | 本次变更的键 |
| `sprint_flow_setup` | `lark-workflow-project-docs` | `lark_project` `progress_base` `dashboards` `workflow_automations` |
| `delivery` / `frontend_delivery` | `arc:code` | `task_base` `progress_base`，以及 `architecture` `whiteboards`（有架构变化时） |
| `incident_fix` | `arc:code` | `incidents` `task_base` `risk_base` `tasklist` |
| `verification` | `arc:verify` | `task_base` `release_base` |
| `audit_report` | `arc:audit` | `audits` `risk_base` `tasklist` |
| `meeting_follow_up` | `lark-workflow-project-docs` | `calendar` `meetings` `minutes` `tasklist` `im_chat` |

## 6. 命名

- 标题使用稳定英文：`<project> Project Home`、`<project> PRD`、`<project> Requirements`、`<project> Architecture`、`<project> Requirements Base`、`<project> Sprint Base`、`<project> Task Base`、`<project> Bug Base`、`<project> Release Base`、`<project> Progress`、`<project> Dashboard`、`<project> Risks`、`<project> Delivery`、`<project> Incidents`、`<project> Audit`。
- 生成的本地文档放在项目已有的 docs 目录下。
