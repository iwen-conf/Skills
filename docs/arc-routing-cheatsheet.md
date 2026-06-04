# Arc Lifecycle Cheatsheet

Arc now keeps seven software engineering lifecycle skills. Task orchestration, memory, and cross-agent collaboration stay in `aitask`; Lark resources are optional and are indexed only when a project has `.lark.json` or the user explicitly enables Lark.

## Selection Rules

| Input signal | Use |
|---|---|
| Project idea needs PRD/Blueprint structure | `arc:define` |
| Requirement is vague or missing acceptance criteria | `arc:clarify` |
| `.lark.json` exists, a Lark link is provided, or the user explicitly asks to enable/create/sync Lark resources | `arc:docs` |
| User says `创建飞书项目空间` / `create Lark project space` | `arc:docs` with full workspace provisioning |
| User says `更新飞书项目空间` / `refresh Lark project space` | `arc:docs` with existing workspace update |
| Implementation is scoped and ready | `arc:build` |
| Frontend baseline, UI, theme, or frontend progress is needed | `arc:frontend` |
| Failure evidence, incident, regression, or failing test exists | `arc:fix` |
| Read-only health check, risk review, or code audit is needed | `arc:audit` |

## Default Order

1. Use `arc:define` when the project is not yet defined.
2. Use `arc:docs` only when Lark is active: existing `.lark.json`, provided Lark link, explicit Lark trigger, or user confirmation.
3. Treat `创建飞书项目空间` as full provisioning: create standard folders, docs, Base tables, dashboards, project flow, calendar, collaboration resources, whiteboards, automations, and index all durable links in `.lark.json`.
4. Treat `更新飞书项目空间` as workspace update: verify `.lark.json`, repair broken links/index gaps, complete missing standard resources, refresh task tables, dashboards, Project flow, and automations without duplicating resources.
5. Use `arc:clarify` when the task is not executable yet.
6. Use `arc:frontend` for frontend lifecycle work.
7. Use `arc:build` for scoped implementation.
8. Use `arc:fix` when failure evidence exists.
9. Use `arc:audit` for read-only risk and quality review.
