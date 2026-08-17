# Lark Workspace Contracts

Load this file for full workspace create, workspace update, `task_base`, and the resource router.

## Full Workspace Contract

When the user says `创建项目的飞书空间`, `创建飞书项目空间`, or `初始化飞书项目空间`, create the standard project workspace in one pass. Ask only for missing project name, owner, auth data, or remote-write confirmation.

Create or resolve and index:

- Drive root folder plus subfolders: `docs`, `design`, `engineering`, `meetings`, `releases`, `incidents`, `audits`, `attachments`, `exports`.
- Project home Docx plus docs for PRD, requirements, architecture, delivery log, incident log, audit log, and meeting notes.
- Wiki space/node for long-term knowledge hierarchy.
- Base app/tables for requirements, sprint, tasks, bugs, releases, progress, risks, and traceability.
- Dashboards for project overview, sprint/progress, risk/quality, and release status.
- Lark Project board or flow for sprint, milestones, and project process.
- Calendar, IM chat when members are available, whiteboards, and workflow automations when supported.

Write every created or resolved durable resource to `.lark.json.resources` with real URLs/IDs. Store the project file-space URL at `.lark.json.resources.drive_folder.url`. If a resource cannot be created, append a `.lark.json.lifecycle[]` blocker naming the missing resource and reason.

## Workspace Update Contract

When the user says `更新飞书项目空间`, update the existing Lark project space instead of recreating it.

- Read `.lark.json` first; if absent, require an existing Lark project home/link before any write.
- Verify indexed resources relevant to the request.
- Repair stale URLs/IDs, missing index entries, and broken cross-links.
- Create missing standard full-workspace resources only when the workspace was or should be full.
- Refresh `task_base`, Project flow, dashboards, Base views, workflow automations, and lifecycle status.
- Append a `full_workspace_update` lifecycle entry with changed `resource_keys`.

## Task Table Contract

`task_base` is mandatory only when Lark is active and a tracked feature is created, changed, delivered, blocked, or verified.

Each feature update MUST write one current Base row with: `title`, `owner`, `status`, `related_requirement`, `changed_files`, `verification`, `lifecycle_entry`, `updated_at`.

Use `tasklist` only for personal reminders; it cannot replace `task_base`. Lark task tables do not replace local task docs.

## Resource Router

| Resource | Required Lark skill | `.lark.json` key |
|---|---|---|
| Auth, identity, high-risk write protocol | `lark-shared` | `lark` |
| Project home, PRD, requirements, architecture, delivery, audit, incident docs | `lark-doc` | `project_home`, `prd`, `requirements`, `architecture`, `delivery`, `audits`, `incidents` |
| Wiki hierarchy | `lark-wiki` | `wiki_space`, `wiki_node` |
| Drive root and project subfolders | `lark-drive` | `drive_folder`, `drive_folders` |
| Structured requirements, sprints, tasks, bugs, releases, progress, risks, traceability | `lark-base` | `requirements_base`, `sprint_base`, `task_base`, `bug_base`, `release_base`, `progress_base`, `risk_base`, `traceability_base` |
| Dashboards | `lark-base` | `dashboards` |
| Personal reminders | `lark-task` | `tasklist` |
| Sprint board / project flow | `lark-openapi-explorer` if no dedicated Project skill exists | `lark_project` |
| Files, exports, attachments | `lark-drive` | `drive_folder`, `drive_folders` |
| Architecture / flow diagrams | `lark-whiteboard` | `whiteboards` |
| Missing native API capability | `lark-openapi-explorer` | `native_openapi` |

| Component | Use for | NEVER use for |
|---|---|---|
| Doc | PRD, technical plan, architecture narrative | SDLC state machine, issue tracking |
| Base | Requirement records, feature task table, sprint, bugs, releases, risks | Long-form discussion |
| Tasks | Personal reminders | Feature delivery state |
| Project | Sprint board, milestone flow | Knowledge archive |
| Wiki | Long-term archive | Active issue state |
| Dashboard | Health/sprint/risk visibility | Manual Doc summaries without structured data |
