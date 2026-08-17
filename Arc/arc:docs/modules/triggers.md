# Lark Activation And Triggers

Load this file to decide whether Lark is active and which phrase maps to create vs update vs single-resource work.

## Activation

| State | Condition | Required behavior |
|---|---|---|
| Active existing | Project root has `.lark.json` | Read it first, use indexed resource URLs/IDs, then update through `arc:docs`. |
| Existing Lark link provided | User provides a Lark project home, Drive folder, Wiki/Base, or project-space link | Resolve the link, create/update `.lark.json`, and avoid duplicate resources. |
| Project space create requested | User says `创建项目的飞书空间`, `创建飞书项目空间`, `初始化飞书项目空间`, or `create Lark project space` | Create the standard project workspace in one pass and index every created/resolved resource. |
| Full workspace update requested | User says `更新飞书项目空间` or `refresh Lark project space` | Verify the existing workspace, repair/index gaps, refresh SDLC resources, never create duplicates. |
| Inactive | No `.lark.json`, no user-provided Lark project link, and no explicit project-space request | Do not create Lark resources, do not create `.lark.json`, and do not prompt just because the project is large. |

Do not infer project-space creation from project size, repository structure, long-running work, or docs/tracking needs. If `.lark.json` is absent, only explicit project-space creation/connect/index/update wording can create or update `.lark.json`. A request like `创建飞书文档` or `创建飞书任务表` creates only the requested resource unless the user also asks to create/connect the project space.

## Create phrases

`创建项目的飞书空间`, `创建飞书项目空间`, `创建完整飞书项目空间`, `一键创建飞书项目空间`, `初始化飞书项目空间`, `create Lark project space`, `create full Lark workspace`, `enable Lark`, `initialize Lark project space`, `create Lark workspace`, `把这个项目纳入飞书`.

## Update phrases

`更新飞书项目空间`, `刷新飞书项目空间`, `补齐飞书项目空间`, `同步飞书项目空间`, `update Lark project space`, `refresh Lark project space`, `complete Lark project space`.

## Other resource phrases

`创建飞书文档`, `创建飞书项目文档`, `创建飞书任务表`, `创建飞书仪表盘`, `同步到飞书`, `索引飞书资源`, `生成飞书PRD`, `create Lark docs`, `create Lark task table`, `create Lark dashboard`, `sync to Lark`.
