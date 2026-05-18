# Arc

Lean Arc keeps five workflow skills:

- `arc:define` — 项目定义（project-level，把想法整理成 PRD/Blueprint）
- `arc:clarify` — 需求澄清（task-level，把模糊请求转成执行任务）
- `arc:build` — 代码交付
- `arc:fix` — 故障修复
- `arc:audit` — 项目体检

Repository search and code context discovery use the local `.ai-code-index/` helpers backed by Zoekt, ast-grep, and Universal Ctags. Coordination, Inbox, and cross-agent state are handled by `aitask`.
