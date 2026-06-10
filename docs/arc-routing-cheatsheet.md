# Arc Lifecycle Cheatsheet

Arc now keeps seven software engineering lifecycle skills. Task orchestration, memory, and cross-agent collaboration stay in `aitask`; Lark project space is created only after an explicit user request or existing project link, and reused through `.lark.json`.

## Selection Rules

| Input signal | Use |
|---|---|
| Project idea needs PRD/Blueprint structure | `arc:define` |
| Requirement is vague or missing acceptance criteria | `arc:clarify` |
| `.lark.json` exists, a Lark project link is provided, or the user explicitly asks to create/connect/sync Lark project space | `arc:docs` |
| User says `创建飞书项目空间` / `create Lark project space` | `arc:docs` with full workspace provisioning |
| User says `更新飞书项目空间` / `refresh Lark project space` | `arc:docs` with existing workspace update |
| Implementation is scoped and ready | `arc:build` |
| Frontend baseline, UI, theme, or frontend progress is needed | `arc:frontend` |
| Failure evidence, incident, regression, or failing test exists | `arc:fix` |
| Read-only health check, risk review, or code audit is needed | `arc:audit` |

## Default Order

1. Use `arc:define` when the project is not yet defined.
2. Use `arc:docs` when Lark is active: existing `.lark.json`, provided Lark project link, or explicit Lark project-space trigger.
3. Capture durable project materials into Lark only when `.lark.json` exists: research sources, new docs, API notes, architecture facts, decisions, screenshots, reports, meeting notes, delivery evidence.
4. Treat `创建项目的飞书空间` / `创建飞书项目空间` as full provisioning: create standard folders, docs, Base tables, dashboards, project flow, calendar, collaboration resources, whiteboards, automations, and index all durable links in `.lark.json`.
5. Treat `更新飞书项目空间` as workspace update: verify `.lark.json`, repair broken links/index gaps, complete missing standard resources, refresh task tables, dashboards, Project flow, and automations without duplicating resources.
5. Use `arc:clarify` when the task is not executable yet.
6. Use `arc:frontend` for frontend lifecycle work.
7. Use `arc:build` for scoped implementation.
8. Use `arc:fix` when failure evidence exists.
9. Use `arc:audit` for read-only risk and quality review.
