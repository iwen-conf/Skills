# Arc Lifecycle Cheatsheet

Arc now keeps ten software engineering lifecycle skills. Task orchestration, memory, and cross-agent collaboration stay in `aitask`; Lark project space is created only after an explicit user request or existing project link, and reused through `.lark.json`.

## Selection Rules

| Input signal | Use |
|---|---|
| Project idea needs PRD/Blueprint structure | `arc:define` |
| Requirement is vague or missing acceptance criteria | `arc:clarify` |
| `.lark.json` exists, a Lark project link is provided, or the user explicitly asks to create/connect/sync Lark project space | `arc:docs` |
| User says `创建飞书项目空间` / `create Lark project space` | `arc:docs` with full workspace provisioning |
| User says `更新飞书项目空间` / `refresh Lark project space` | `arc:docs` with existing workspace update |
| Implementation is scoped and ready | `arc:build` |
| Same-session multi-model cost routing (Guide → Executor after first edit) | `arc:prewalk` |
| Frontend baseline, UI, theme, or frontend progress is needed | `arc:frontend` |
| Failure evidence, incident, regression, or failing test exists | `arc:fix` |
| Read-only health check, risk review, or code audit is needed | `arc:audit` |
| Vulnerability / AppSec audit (assets, data map, no scanners) | `arc:audit` mode `appsec` |
| Local security scanners / DAST / secrets / SCA report | `arc:security` |
| Design/generate/run tests, coverage/regression gate, fuzz/property, or benchmark/load | `arc:test` |
| Turn audit/scan findings into detailed local tasks | `arc:sdlc` + security-audit-task-pipeline |
| Multi-finding security fix campaign | handoff → task docs → `arc:fix`/`arc:build` per subtask |

## Default Order

1. Use `arc:define` when the project is not yet defined.
2. Use `arc:docs` when Lark is active: existing `.lark.json`, provided Lark project link, or explicit Lark project-space trigger.
3. Capture durable project materials into Lark only when `.lark.json` exists: research sources, new docs, API notes, architecture facts, decisions, screenshots, reports, meeting notes, delivery evidence.
4. Treat `创建项目的飞书空间` / `创建飞书项目空间` as full provisioning: create standard folders, docs, Base tables, dashboards, project flow, calendar, collaboration resources, whiteboards, automations, and index all durable links in `.lark.json`.
5. Treat `更新飞书项目空间` as workspace update: verify `.lark.json`, repair broken links/index gaps, complete missing standard resources, refresh task tables, dashboards, Project flow, and automations without duplicating resources.
6. Use `arc:clarify` when the task is not executable yet.
7. Use `arc:sdlc` before large tracked implementation work, and for audit/scan finding task expansion (docs = durable progress; not plan-postcard cold handoff).
8. Use `arc:frontend` for frontend lifecycle work.
9. Use `arc:build` for scoped implementation.
10. Use `arc:prewalk` inside build/fix/frontend when multi-model cost routing is available.
11. Use `arc:fix` when failure evidence exists.
12. Use `arc:audit` for read-only risk and quality review; use mode `appsec` for vulnerability/project security methodology.
13. Use `arc:security` for local scanner automation and data-value re-ranked security reports.
14. Use `arc:test` for layered testing and quality gates: enable unit/integration/contract/E2E, coverage, fuzz/property, and performance by risk with platform-native tooling (Android=Maestro, Go/Rust native, HarmonyOS=arkxtest, frontend=Vitest/Playwright), running and judging performance separately from functional.
