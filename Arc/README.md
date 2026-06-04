# Arc

Lean Arc keeps seven software engineering lifecycle skills:

- `arc:define` — 项目定义，把想法整理成 PRD/Blueprint
- `arc:clarify` — 需求澄清，把模糊请求转成执行任务
- `arc:docs` — 可选飞书工作区，在已有 `.lark.json` 或用户触发时维护飞书资源索引
- `arc:build` — 代码交付
- `arc:frontend` — 前端工程基线、UI 交付和前端进度沉淀
- `arc:fix` — 故障修复
- `arc:audit` — 项目体检

Repository search and code context discovery use the local `.ai-code-index/` helpers backed by Zoekt, ast-grep, and Universal Ctags. Lark resources are optional: existing Lark projects live in each project root’s `.lark.json`, and new Lark resources are created only after an explicit trigger or confirmation. Coordination, Inbox, and cross-agent state are handled by `aitask`.
