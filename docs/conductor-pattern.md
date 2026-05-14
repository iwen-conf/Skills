# Conductor Pattern

本仓库不再维护 Arc conductor。代码库上下文检索由 `.ai-code-index/` 本地搜索脚本负责；跨 Agent 编排、Inbox 消费和任务分派由 `aitask` 负责。

Arc Skill 可以作为 `aitask` 编排中的局部工作方法：

- `arc:clarify`：澄清任务输入。
- `arc:build`：执行代码交付。
- `arc:fix`：修复有证据的问题。
- `arc:audit`：只读评估质量和风险。
