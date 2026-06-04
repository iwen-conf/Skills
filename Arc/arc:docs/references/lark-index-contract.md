# Lark Project Index Contract

This reference defines the project-root `.lark.json` file maintained by `arc:docs` only for Lark-active projects. The file stores durable links, IDs, owners, status, and timestamps only. It must not store secrets, access tokens, app secrets, private credentials, or document bodies.

## Activation Rules

Do not create `.lark.json` for every project.

Lark is active only when one condition is true:

- `.lark.json` already exists in the project root.
- The user provides a Lark URL, token, Base link, Wiki link, or project link.
- The user explicitly triggers Lark setup, indexing, docs, task table, dashboard, or sync.
- The user confirms Lark after a candidate collaboration/docs/tracking prompt.

When Lark is inactive, skip this contract entirely. When `.lark.json` already exists, inspect and verify indexed resources before writes.

## Minimal Shape

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
    "project_home": null,
    "prd": null,
    "requirements": null,
    "architecture": null,
    "progress_base": null,
    "task_base": null,
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
    "reindex_command": ".ai-code-index/reindex.sh",
    "search_command": ".ai-code-index/search.sh \"query\"",
    "struct_search_command": ".ai-code-index/struct-search.sh <language> '<pattern>'",
    "symbols_command": ".ai-code-index/symbols.sh",
    "last_refreshed_at": "",
    "status": "unknown"
  },
  "lifecycle": []
}
```

## Resource Entry

Use this object shape for non-null resource entries. Keep fields that are unknown out of the object rather than inventing placeholders.

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
  "owner": "arc:docs",
  "source": "lark-cli docs +create --api-version v2",
  "updated_at": "2026-01-01T00:00:00Z",
  "status": "active"
}
```

Allowed `type` values:

- `wiki_space`
- `wiki_node`
- `drive_folder`
- `drive_file`
- `docx`
- `base`
- `base_table`
- `task_base`
- `dashboard`
- `tasklist`
- `task`
- `lark_project`
- `calendar`
- `calendar_event`
- `live_meeting`
- `vc_meeting`
- `minutes`
- `im_chat`
- `mail_thread`
- `whiteboard`
- `sheet`
- `approval`
- `okr`
- `app`
- `markdown_file`
- `slide`
- `workflow_report`
- `workflow_automation`
- `event_subscription`
- `attendance_record`
- `native_openapi`
- `custom_lark_skill`
- `local_file`
- `external`

## Resource Map

- `wiki_space`: Lark Wiki space that contains the project knowledge hierarchy.
- `wiki_node`: project node inside the Wiki space, usually pointing to the project home.
- `drive_folder`: project Drive folder for attachments, exports, templates, and binary evidence.
- `project_home`: Docx home page linking the project resources and latest status.
- `prd`: project definition or PRD created by `arc:define`.
- `requirements`: requirements and acceptance criteria, often updated by `arc:clarify`.
- `architecture`: architecture notes, diagrams, data model references, and technical decisions.
- `progress_base`: structured progress tracker for milestones, owners, status, and verification.
- `task_base`: structured Base task table for Lark-active feature state; required for tracked feature updates only when Lark is active.
- `risk_base`: structured risk and audit finding tracker.
- `traceability_base`: requirement-to-code-to-test mapping for non-trivial projects.
- `dashboards`: Lark dashboard views backed by structured Base or Project data; never manual Doc summaries.
- `tasklist`: Lark tasklist for personal reminders and lightweight follow-ups only.
- `lark_project`: Lark Project board or flow used for sprint, milestone, and process-driven progress tracking.
- `calendar`: project calendar or primary calendar reference used for meetings and milestones.
- `live_meetings`: live VC Agent sessions where an agent joins an ongoing meeting.
- `meetings`: calendar or VC meeting records.
- `minutes`: Lark Minutes outputs and meeting media records.
- `im_chat`: project chat or group used for collaboration handoff.
- `mail_threads`: formal external handoff or inbox-based project communication.
- `whiteboards`: architecture, flowchart, ER, sequence, swimlane, use-case, or review boards.
- `sheets`: lightweight spreadsheets when Base is unnecessary.
- `approvals`: formal requirement, release, audit, or thesis approval records.
- `okrs`: objectives and measurable outcomes connected to the project.
- `apps`: published Lark/Miaoda app or static prototype links.
- `markdown_files`: Markdown source documents managed through Lark.
- `slides`: Lark Slides or presentation handoff artifacts.
- `workflow_reports`: meeting-summary or standup workflow outputs.
- `workflow_automations`: status transitions, notifications, approvals, or webhooks used as glue between Lark resources.
- `event_subscriptions`: real-time Lark event subscriptions used by the project.
- `attendance_records`: attendance or compliance evidence when relevant.
- `native_openapi`: raw Lark OpenAPI discoveries not covered by existing skills.
- `custom_lark_skills`: reusable custom Lark skill packages created for repeated operations.
- `delivery`: delivery notes and release handoff documents.
- `incidents`: incident, root-cause, and fix reports.
- `audits`: project audit or review reports.
- `thesis_support`: thesis support documents generated from repository evidence.

## Task Base Contract

`resources.task_base` is the structured Base table for tracked feature state. It is required only when Lark is active and a feature is created, changed, delivered, blocked, or verified.

Minimum task row fields:

- `title`
- `owner`
- `status`
- `related_requirement`
- `changed_files`
- `verification`
- `lifecycle_entry`
- `updated_at`

`tasklist` may hold personal reminders only. It must not replace `task_base`.

## Lifecycle Entry

Append one entry after each meaningful project event.

```json
{
  "date": "2026-01-01T00:00:00Z",
  "skill": "arc:build",
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

Do not rewrite historical lifecycle entries unless a link is broken or the original entry is factually wrong.

## Lifecycle Event Examples

```json
{
  "skill": "arc:define",
  "event": "project_definition",
  "summary": "Created PRD and project home",
  "resource_keys": ["project_home", "prd", "wiki_node"]
}
```

```json
{
  "skill": "arc:clarify",
  "event": "requirements_update",
  "summary": "Added acceptance criteria for billing export",
  "resource_keys": ["requirements", "task_base", "progress_base", "traceability_base", "dashboards"]
}
```

```json
{
  "skill": "arc:docs",
  "event": "sprint_flow_setup",
  "summary": "Indexed sprint board, progress dashboard, and status automation",
  "resource_keys": ["lark_project", "progress_base", "dashboards", "workflow_automations"]
}
```

```json
{
  "skill": "arc:frontend",
  "event": "frontend_delivery",
  "summary": "Delivered dashboard shell and design tokens",
  "resource_keys": ["architecture", "task_base", "progress_base", "whiteboards"]
}
```

```json
{
  "skill": "arc:fix",
  "event": "incident_fix",
  "summary": "Fixed failed order status transition",
  "resource_keys": ["incidents", "task_base", "risk_base", "tasklist"]
}
```

```json
{
  "skill": "arc:audit",
  "event": "audit_report",
  "summary": "Recorded authorization and test coverage findings",
  "resource_keys": ["audits", "risk_base", "tasklist"]
}
```

```json
{
  "skill": "arc:docs",
  "event": "meeting_follow_up",
  "summary": "Indexed VC notes and created follow-up tasks",
  "resource_keys": ["calendar", "meetings", "minutes", "tasklist", "im_chat"]
}
```

## Naming Rules

- Keep one `resources.project_home` per repository.
- Prefer stable English titles: `<project> Project Home`, `<project> PRD`, `<project> Requirements`, `<project> Architecture`, `<project> Progress`, `<project> Dashboard`, `<project> Risks`, `<project> Delivery`, `<project> Incidents`, `<project> Audit`.
- Store generated local docs under the project’s existing docs directory when one exists.
- Use arrays for repeating resources such as meetings, minutes, whiteboards, sheets, approvals, delivery notes, incidents, audits, and thesis-support docs.
- Keep `.lark.json` small enough to review in a diff.

## ID And Token Rules

- `url`: human-openable Lark link.
- `token`: document token or object token returned by Lark CLI.
- `id`: durable resource ID such as Base app token, tasklist GUID, chat ID, calendar ID, approval code, or event ID.
- `table_id`: Base table ID when the resource points to a specific table.
- `record_id`: Base record ID when a lifecycle entry points to a task or tracking row.
- `source_keys`: dashboard or automation source resource keys, such as `progress_base`, `risk_base`, `traceability_base`, or `lark_project`.
- `node_token`: Wiki node token when the resource is placed in a Wiki hierarchy.
- `folder_token`: Drive folder token for file operations.
- Do not store app secrets, user access tokens, refresh tokens, private keys, cookies, or session credentials.
