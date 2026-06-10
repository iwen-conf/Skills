---
name: arc:docs
description: "Feishu/Lark project space and .lark.json owner for explicit setup and indexed document lifecycle."
---

# arc:docs

## Overview

`arc:docs` creates and maintains the project Lark workspace plus the project-root `.lark.json` index only after an explicit project-space request, an existing `.lark.json`, or a user-provided Lark project link. It is the only Arc skill allowed to create or update durable Lark project resources.

Read [`references/lark-index-contract.md`](references/lark-index-contract.md) when creating or changing `.lark.json`.

## Quick Contract

- **Trigger**: `.lark.json` exists, the user provides a Lark project link, or the user explicitly asks to create/connect/update/index Lark project resources.
- **Inputs**: Project path, project name, desired workspace scope, owners, optional existing Lark links.
- **Outputs**: `.lark.json`, Lark resource links/IDs, local index status, lifecycle entries.
- **Quality Gate**: Every indexed resource exists, every claim has evidence, every lifecycle event is traceable.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:define` before `arc:docs` if the PRD/project concept is missing.
- Use `arc:clarify` before `arc:docs` if requirement acceptance criteria are missing.
- Use `arc:build`, `arc:frontend`, `arc:fix`, or `arc:audit` for delivery, UI, repair, or review; then update `.lark.json` here only when Lark is active.

Lark activation:

| State | Condition | Required behavior |
|---|---|---|
| Active existing | Project root has `.lark.json` | Read it first, use indexed resource URLs/IDs as the entry point, verify resources when relevant, then update through `arc:docs`. |
| Existing Lark link provided | User provides a Lark project home, Drive folder, Wiki/Base, or project-space link | Resolve the link, create/update `.lark.json`, and avoid duplicate resources. |
| Project space create requested | User says `创建项目的飞书空间`, `创建飞书项目空间`, `初始化飞书项目空间`, or `create Lark project space` | Create the standard project workspace in one pass, store the project file-space URL in `.lark.json.resources.drive_folder.url`, and index every created/resolved resource. |
| Full workspace update requested | User says `更新飞书项目空间` or `refresh Lark project space` | Verify the existing workspace, repair/index gaps, refresh SDLC resources, and never create duplicates. |
| Inactive | No `.lark.json`, no user-provided Lark project link, and no explicit project-space request | Do not create Lark resources, do not create `.lark.json`, and do not prompt just because the project is large. |

Explicit trigger phrases include:

- `创建项目的飞书空间`
- `初始化飞书项目空间`
- `启用飞书项目空间`
- `接入飞书`
- `创建飞书文档`
- `创建飞书项目文档`
- `创建飞书任务表`
- `创建飞书仪表盘`
- `同步到飞书`
- `索引飞书资源`
- `生成飞书PRD`
- `把这个项目纳入飞书`
- `初始化项目空间`
- `enable Lark`
- `initialize Lark project space`
- `create Lark workspace`
- `create Lark docs`
- `create Lark task table`
- `create Lark dashboard`
- `sync to Lark`

Project-space creation trigger phrases:

- `创建项目的飞书空间`
- `创建飞书项目空间`
- `创建完整飞书项目空间`
- `一键创建飞书项目空间`
- `初始化飞书项目空间`
- `create Lark project space`
- `create full Lark workspace`

Explicit creation rule:

- Do not infer project-space creation from project size, repository structure, long-running work, or docs/tracking needs.
- If `.lark.json` is absent, only explicit project-space creation/connect/index/update wording can create or update `.lark.json`.
- A request like `创建飞书文档` or `创建飞书任务表` creates only the requested resource unless the user also asks to create/connect the project space.
- If required auth, owner, project name, or remote-write confirmation is missing, stop Lark writes and ask only for the missing setup detail.

Material capture contract:

- Any durable project material discovered or produced during research, search, development, design, review, meetings, or handoff MUST be routed into the Lark project space when `.lark.json` exists.
- Store source URLs, local file paths, titles, timestamps, owners, summary, and evidence links in the correct resource; never store access tokens or full secret-bearing document bodies in `.lark.json`.
- Use Docx/Wiki for narrative findings, Base for structured requirements/tasks/risks/traceability, Drive for files/exports/attachments, Whiteboard/UML for visual models, Slides for presentation assets, and lifecycle entries for provenance.
- If `.lark.json` is absent, keep durable materials local unless the user explicitly asks to create/connect a Lark project space.

Workspace update trigger phrases:

- `更新飞书项目空间`
- `刷新飞书项目空间`
- `补齐飞书项目空间`
- `同步飞书项目空间`
- `update Lark project space`
- `refresh Lark project space`
- `complete Lark project space`

Lark resource router:

| Resource | Required Lark skill | `.lark.json` key |
|---|---|---|
| Auth, identity, high-risk write protocol | `lark-shared` | `lark` |
| Project home, PRD, requirements, architecture, delivery, audit, incident docs | `lark-doc` | `project_home`, `prd`, `requirements`, `architecture`, `delivery`, `audits`, `incidents` |
| Wiki hierarchy | `lark-wiki` | `wiki_space`, `wiki_node` |
| Drive root and project subfolders | `lark-drive` | `drive_folder`, `drive_folders` |
| Structured requirements, sprints, tasks, bugs, releases, progress, risks, traceability | `lark-base` | `requirements_base`, `sprint_base`, `task_base`, `bug_base`, `release_base`, `progress_base`, `risk_base`, `traceability_base` |
| Feature task table and delivery state | `lark-base` | `task_base` |
| Dashboards from structured Base/Project data | `lark-base` | `dashboards` |
| Personal reminders and lightweight follow-up | `lark-task` | `tasklist` |
| Sprint board, project flow, milestones | `lark-openapi-explorer` if no dedicated Project skill exists | `lark_project` |
| Future meetings and milestones | `lark-calendar` | `calendar`, `meetings` |
| Live meeting participation | `lark-vc-agent` | `live_meetings` |
| Ended meeting notes and transcripts | `lark-vc`, `lark-minutes` | `meetings`, `minutes` |
| Project chat and notifications | `lark-im` | `im_chat` |
| Formal external or email handoff | `lark-mail` | `mail_threads` |
| Files, exports, attachments | `lark-drive` | `drive_folder`, `drive_folders` |
| Architecture, flow, ER, sequence, class, use-case diagrams | `lark-whiteboard`, `lark-uml:*` | `whiteboards` |
| Lightweight tables | `lark-sheets` | `sheets` |
| Formal gates | `lark-approval` | `approvals` |
| Objectives and measurable project outcomes | `lark-okr` | `okrs` |
| Published static prototype or app | `lark-apps` | `apps` |
| Markdown source docs | `lark-markdown` | `markdown_files` |
| Slides and presentation handoff | `lark-slides`, `lark-uml:ppt` | `slides` |
| Workflow summaries | `lark-workflow-meeting-summary`, `lark-workflow-standup-report` | `workflow_reports` |
| Status automation, notifications, webhooks | `lark-base` workflow or `lark-openapi-explorer` | `workflow_automations` |
| Real-time Lark events | `lark-event` | `event_subscriptions` |
| Attendance/compliance evidence | `lark-attendance` | `attendance_records` |
| Missing native API capability | `lark-openapi-explorer` | `native_openapi` |
| Repeated custom Lark operation | `lark-skill-maker` | `custom_lark_skills` |
| Owner and attendee IDs | `lark-contact` | resource owners |

Core component boundaries:

| Component | Use for | NEVER use for |
|---|---|---|
| Doc | PRD, technical plan, architecture narrative, meeting notes, design discussion | SDLC state machine, issue tracking, dashboard data |
| Base | Requirement records, feature task table, Sprint data, bugs, releases, risks, traceability, dashboard source data | Long-form discussion or prose-only knowledge |
| Tasks | Personal or small-team reminders | Feature delivery state, complex SDLC model, sprint source of truth |
| Project | Sprint board, milestone flow, workflow-driven project progress | Knowledge archive or loose discussion |
| Wiki | Long-term architecture/API/process archive | Active issue state or transient execution |
| Workflow | Status transitions, approval glue, notifications, webhooks | Hidden business state or undocumented side effects |
| Dashboard | Project health, sprint, risk, release, and quality visibility | Manual Doc summaries without structured backing data |

## Context Search

- MUST inspect the project root, existing docs, `.lark.json`, and `.ai-code-index/` status first.
- MUST use local `.ai-code-index/` search before broad repository reads.
- MUST load `lark-shared` before any Lark write.
- If `.lark.json` exists, MUST use its indexed Lark addresses before searching or changing remote Lark resources.
- If `.lark.json` is absent and no explicit project-space request or existing Lark project link exists, Lark is inactive.

## Announce

Begin by stating clearly:
"I am using `arc:docs` to initialize or maintain the Lark project workspace and `.lark.json` index."

## The Iron Law

```text
NO LARK PROJECT RESOURCE WITHOUT .lark.json.
NO .lark.json ENTRY WITHOUT A REAL RESOURCE.
NO PASSIVE .lark.json CREATION.
NO PROJECT SPACE CREATION WITHOUT AN EXPLICIT USER REQUEST OR USER-PROVIDED LARK PROJECT LINK.
NO LARK WRITE WITHOUT EXISTING INDEX, EXPLICIT TRIGGER, OR USER CONFIRMATION.
NO CLAIM WITHOUT REPOSITORY OR USER-PROVIDED EVIDENCE.
NO SECRET OR DOCUMENT BODY IN .lark.json.
NO LARK-ACTIVE TRACKED FEATURE UPDATE WITHOUT task_base UPDATE.
NO DURABLE PROJECT MATERIAL LEFT ONLY IN CHAT WHEN .lark.json EXISTS.
```

## Hard Constraints

- MUST choose the smallest useful Lark resource set for explicitly scoped work.
- MUST NOT create a Lark project space because a project is large or long-running.
- MUST treat `创建项目的飞书空间` / `创建飞书项目空间` / `create Lark project space` as `workspace_scope=full` unless the user explicitly limits scope.
- MUST treat `更新飞书项目空间` / `refresh Lark project space` as `workspace_update`; it requires existing `.lark.json` or a user-provided existing Lark link.
- MUST NOT create a duplicate workspace, project home, Base app, dashboard, or Lark Project when updating an existing Lark project space.
- MUST treat missing `.lark.json` as local-only unless the user explicitly asks to create/connect/index/update Lark project space or provides an existing Lark project link.
- MUST index-check existing `.lark.json` before any Lark update.
- MUST create or update `.lark.json` at the project root for every Lark project workspace.
- MUST store the project file-space Feishu address in `.lark.json.resources.drive_folder.url` when a project space is created or resolved.
- MUST store durable URLs, IDs, owners, status, and timestamps only.
- MUST use `docs +create/+fetch/+update --api-version v2` for Lark Docx operations.
- MUST record local code index status in `.lark.json.local_index`.
- MUST append lifecycle events; NEVER rewrite history except to fix a broken link or factual error.
- MUST capture durable project material into Lark resources when `.lark.json` exists: research sources, new docs, API notes, architecture facts, decisions, screenshots, reports, exports, and handoff artifacts.
- MUST treat `.lark.json.resources.task_base` as the structured feature task table for tracked projects.
- MUST create or resolve `task_base` with `lark-base` before marking a Lark-active tracked feature update complete.
- MUST update `task_base` after every Lark-active tracked feature update with title, owner, status, related requirement, changed files, verification, linked lifecycle entry, and `updated_at`.
- MUST resolve owners/attendees through Contact when Lark IDs are required.
- MUST use Base for structured progress/risk/traceability and Docx for narrative documents.
- MUST use Dashboard only from structured Base or Project data; NEVER fake a dashboard with a manually written Doc summary.
- MUST use Tasks only for lightweight execution; NEVER treat Tasks as the SDLC system of record.
- MUST use Lark Project for sprint board, milestone flow, and process-driven project tracking when those are required.
- MUST use Wiki for long-lived knowledge hierarchy and archival; NEVER use Wiki as active workflow state.
- MUST use Workflow automation only as glue; the authoritative state MUST remain in Base, Project, Approval, or `.lark.json`.
- MUST ask for confirmation through the relevant Lark skill before destructive or high-risk writes.
- MUST use Calendar for planned meetings, VC/Minutes for ended meetings, and VC Agent only for live meeting participation.
- MUST use IM for fast internal collaboration and Mail for formal external or inbox-based handoff.
- MUST use Whiteboard/UML when architecture, workflow, data model, or interaction structure is easier to verify visually than in prose.
- MUST use Approval or OKR only when there is a real gate, objective, owner, or metric to track.
- MUST use OpenAPI Explorer only when existing `lark-*` skills cannot cover the required Lark API.
- MUST use `lark-skill-maker` only for repeated Lark operations worth packaging as a reusable skill.
- NEVER guess Lark URLs, tokens, table IDs, chat IDs, tasklist IDs, calendar IDs, meeting IDs, or approval IDs.
- NEVER store access tokens, app secrets, cookies, private keys, credentials, or full document bodies in `.lark.json`.

## Full Workspace Contract

When the user says `创建项目的飞书空间`, `创建飞书项目空间`, or `初始化飞书项目空间`, MUST create the standard project workspace in one pass. Ask only for missing project name, owner, auth data, or remote-write confirmation that is required to create resources.

Full workspace MUST create or resolve and index:

- Drive root folder plus subfolders: `docs`, `design`, `engineering`, `meetings`, `releases`, `incidents`, `audits`, `attachments`, `exports`.
- Project home Docx plus docs for PRD, requirements, architecture, delivery log, incident log, audit log, and meeting notes.
- Wiki space/node for long-term knowledge hierarchy.
- Base app/tables for requirements, sprint, tasks, bugs, releases, progress, risks, and traceability.
- Dashboards for project overview, sprint/progress, risk/quality, and release status.
- Lark Project board or flow for sprint, milestones, and project process.
- Calendar resource for milestones and project meetings.
- Project IM chat or indexed chat when members are available.
- Whiteboards for architecture, process flow, data model, and sequence/interaction views.
- Workflow automations for status notifications and Base/Project glue when supported.
- Drive/Markdown/Slides/Sheets/App resources when project artifacts require those formats.

Full workspace MUST write every created or resolved durable resource to `.lark.json.resources` with real URLs/IDs/tokens. The project file-space URL MUST be stored at `.lark.json.resources.drive_folder.url`. If a resource cannot be created because of permission, missing owner/member data, or unsupported API coverage, MUST append a `.lark.json.lifecycle[]` blocker entry naming the missing resource and reason; do not silently omit it.

## Workspace Update Contract

When the user says `更新飞书项目空间`, MUST update the existing Lark project space instead of recreating it.

Workspace update MUST:

- Read `.lark.json` first; if absent, require an existing Lark project home/link before any write.
- Verify every indexed resource that is relevant to the request.
- Repair stale URLs/IDs, missing index entries, and broken cross-links.
- Create missing standard full-workspace resources only when the workspace was or should be full.
- Refresh `task_base`, Project flow, dashboards, Base views, Workflow automations, and lifecycle status.
- Append a `full_workspace_update` lifecycle entry with changed `resource_keys`.
- Record blockers for missing permission, missing owner/member data, or unsupported API coverage.

## Task Table Contract

`task_base` is mandatory only when Lark is active and a tracked feature is created, changed, delivered, blocked, or verified.

Each feature update MUST write one current Base row with:

- `title`
- `owner`
- `status`
- `related_requirement`
- `changed_files`
- `verification`
- `lifecycle_entry`
- `updated_at`

Use `tasklist` only for personal reminders; it cannot replace `task_base`.

## Workflow

1. Determine Lark state: active existing, existing Lark link provided, project space create requested, workspace update requested, or inactive.
2. At project start or before any Lark work, look for project-root `.lark.json`; if present, read it before remote operations.
3. If inactive, stop Lark work and continue local-only without prompting.
4. If project space creation was explicitly requested, create the standard workspace and then write `.lark.json` with `resources.drive_folder.url`.
5. If an existing Lark project link was provided, resolve/index it into `.lark.json` without creating duplicates.
6. Read project instructions, existing docs, `.lark.json`, and local index status.
7. Create or update `.lark.json` from the reference contract.
8. Refresh local code index when needed and record status.
9. Choose workspace scope and operation: `minimum`, `normal`, `full`, or `workspace_update`; exact project-space creation triggers MUST use `full` unless user-limited, and update triggers MUST use `workspace_update`.
10. Select Lark tools by evidence:
   - narrative knowledge -> Docx/Wiki;
   - feature task state -> Base `task_base`;
   - structured state -> Base/Project/Approval/OKR;
   - visibility -> Dashboard backed by Base or Project;
   - lightweight personal reminders -> Task;
   - meetings -> Calendar/VC/Minutes/VC Agent;
   - collaboration -> IM/Mail/Contact;
   - artifacts -> Drive/Markdown/Sheets/Slides/Apps;
   - visual models -> Whiteboard/UML;
   - automation -> Workflow;
   - missing native capability -> OpenAPI/Skill Maker.
11. Create, search, update, delete, or resolve Lark resources through `.lark.json` resource URLs/IDs and the resource router; confirm before destructive deletes.
12. Store each durable resource in `.lark.json.resources`.
13. Capture durable project material into the relevant Lark resource and append source/evidence references.
14. For every Lark-active tracked feature update, create or update the `task_base` row before the delivery is considered complete.
15. Append `.lark.json.lifecycle[]` for Arc events: define, clarify, build, frontend, fix, audit, research/material capture, meeting follow-up.
16. Finish with resource links, task table status, material capture status, evidence used, unresolved gaps, and next lifecycle step.

## Quality Gates

- `project_home`, local index status, and project identity are present once remote docs exist.
- Project-space creation happens only after explicit user request or provided Lark project link.
- Created/resolved project spaces store `.lark.json.resources.drive_folder.url`.
- Full workspace trigger creates or explicitly blocks every standard workspace resource and records each result in `.lark.json`.
- Workspace update trigger verifies the existing index, refreshes structured SDLC resources, repairs gaps, and records blockers without duplicating resources.
- No `.lark.json` is created for inactive projects.
- Wiki, Drive, Docx, Base, Dashboard, Task, Project, Calendar, VC/Minutes, IM, Whiteboard, Sheets, Approval, Workflow, and Contact entries appear only when actually used.
- Requirements, delivery, incidents, audits, meetings, risks, and tasks are cross-linked when more than one resource exists.
- Dashboards reference structured source resources such as `progress_base`, `risk_base`, `traceability_base`, or `lark_project`.
- Every Lark-active tracked feature update has a `task_base` row linked to the lifecycle entry; `tasklist` alone is never enough.
- Durable project materials discovered or created during work are linked from Docx/Wiki/Base/Drive/Whiteboard/Slides as appropriate.
- Thesis-support claims trace to code, SQL, routes, tests, config, or explicit source docs.
- Every selected Lark tool has a lifecycle reason; unused "nice to have" resources are omitted.

## Expert Standards

- Treat the workspace as `Information Architecture`: one project home, stable hierarchy, clear ownership.
- Maintain a `Traceability Matrix` across requirements, code, tests, risks, tasks, meetings, and handoffs when the project is non-trivial.
- Use Lark as collaboration infrastructure: Doc for narrative, Base for structured SDLC data, Project for flow, Dashboard for visibility, Task for execution, Wiki for archive, Workflow for glue, Calendar/VC/Minutes for meetings, IM/Mail for communication, Drive for files, Whiteboard for diagrams, Approval for gates.
- Keep `.lark.json` small, reviewable, and machine-readable.

## Scripts & Commands

Use project-local commands and Lark CLI through the relevant Lark skill:

```bash
.ai-code-index/reindex.sh
lark-cli docs +create --api-version v2 --content '<title>Project Home</title>'
lark-cli docs +update --api-version v2 --doc "<doc-url-or-token>" --command append --content '<p>...</p>'
```

## Red Flags

- Lark resource created but missing from `.lark.json`.
- `.lark.json` created without explicit trigger, existing Lark link, or user confirmation.
- Creating a full Lark workspace without an explicit full workspace trigger.
- Updating a Lark project space by creating a duplicate workspace or project home.
- Multiple project home docs for one repository.
- Progress recorded only in chat.
- Lark-active feature delivered while `task_base` is missing or stale.
- Dashboard created without structured Base or Project source data.
- Base treated as implementation truth without checking code.
- Tasks used as the complex SDLC system of record.
- Workflow automation hiding authoritative state outside Base/Project/Approval.
- Owner, attendee, or approval data guessed.

## When to Use

- **Preferred Trigger**: `.lark.json` exists, the user provides a Lark project link, or the user asks to create/connect/update a project Lark space, create Lark docs, maintain Lark progress, index Lark resources, or preserve Lark lifecycle state.
- **Typical Scenario**: Kickoff, PRD handoff, requirements tracking, architecture docs, delivery notes, incident/audit records, meeting follow-up, thesis support.
- **Boundary Tip**: Use `arc:build` for code-only delivery and `arc:fix` for failure-only repair; use this skill when documentation, collaboration, traceability, or Lark indexing is in scope.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `project_name` | string | no | Human-readable project name |
| `lark_home` | string | no | Existing project home URL or token |
| `workspace_scope` | string | no | `minimum`, `normal`, `full`, or `workspace_update` |
| `doc_scope` | string | no | PRD, requirements, architecture, progress, audit, delivery, incidents, thesis, or all |
| `progress_mode` | enum | no | `doc`, `base`, `task`, or `none` |

## Outputs

```text
Project Documentation Handoff
- .lark.json created or updated
- Lark resources created or indexed
- Feature task table updated when Lark is active
- Local code index status
- Lifecycle entries appended
- Evidence sources used
- Open gaps and next step
```
