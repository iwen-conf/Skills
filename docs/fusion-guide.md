# Fusion Guide

Arc 只保留 5 个软件工程技能（`arc:explore`、`arc:plan`、`arc:code`、`arc:verify`、`arc:audit`）。组合原则：

- 任务编排、记忆和协作交给 `aitask`。
- 一个阶段一个主技能，按名称交接：理解代码用 `arc:explore`，写规格和任务文档用 `arc:plan`，改源码用 `arc:code`，产出证据用 `arc:verify`，只读评估用 `arc:audit`。
- 飞书项目空间不按项目大小自动创建：只有用户明确要求创建/连接项目空间，或项目已有 `.lark.json`，才由 `lark-workflow-project-docs` 维护飞书资源。
- 已有 `.lark.json` 的项目，Arc 阶段结束时把状态、资料和交付证据交给 `lark-workflow-project-docs` 写入对应飞书资源。
- 垂直领域能力交给对应 Skill，不在 Arc 内重复建设；Arc 只记录必要链接和生命周期状态。
