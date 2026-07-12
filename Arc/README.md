# Arc

Lean Arc keeps ten software engineering lifecycle skills:

- `arc:define` — 项目定义，把想法整理成 PRD/Blueprint
- `arc:clarify` — 需求澄清，把模糊请求转成执行任务
- `arc:docs` — 可选飞书工作区，在已有 `.lark.json` 或用户触发时维护飞书资源索引
- `arc:build` — 代码交付
- `arc:frontend` — 前端工程基线、UI 交付和前端进度沉淀
- `arc:fix` — 故障修复
- `arc:test` — 分层测试策略与执行；按风险启用单元/集成/契约/E2E、覆盖率、模糊/属性、性能，用平台原生工具（Android=Maestro、Go/Rust 原生、HarmonyOS=arkxtest、前端=Vitest/Playwright）
- `arc:audit` — 项目体检；AppSec 模式做跨项目只读漏洞审计；多 finding 时产出 Handoff（定位/口径/角色）
- `arc:security` — 本地安全 CLI 自动化、数据价值重排；多 finding 并入 Handoff 再拆任务
- `arc:task-doc-progress-conventions` — 任务文档；安全来源时按 finding 写细子任务（见 security-audit-task-pipeline）

Repository search and code context discovery use `arc:code-index` plus the local `.ai-code-index/` helpers backed by Zoekt, ast-grep, Universal Ctags, and Rust/Go helper CLIs. Lark resources are optional: existing Lark projects live in each project root’s `.lark.json`, and new Lark resources are created only after an explicit trigger or confirmation. Coordination, Inbox, and cross-agent state are handled by `aitask`.
