# PRD

Write or refresh a project definition (PRD / blueprint) under `docs/00-产品需求/NN-名称.md` and add it
to `docs/00-产品需求/README.md`.

## Inputs

| Input | Required | Use |
|---|---|---|
| Idea or existing PRD | yes | The one-sentence restatement |
| Project path | no | Brownfield grounding |
| Constraints | no | Users, platform, deadline, rules, non-goals |
| Prior notes | no | Drafts, slides, chat logs, existing docs |

## Rules

1. Brownfield: search existing domain terms with `arc-idx search` first, then `arc-idx symbol` for
   entity names; use the repository's vocabulary.
2. One canonical name per domain concept; never introduce a second name for an existing one.
3. Read `.lark.json` before drafting when it exists, so PRD, owners, requirements and Wiki links stay
   consistent.
4. Follow the template section order unless the user supplies a required format.
5. Fill sections with project-specific content only. A missing fact becomes a 待确认问题; never fill
   a gap with generic text.
6. Record platform and existing project constraints. Preserve the existing stack; a PRD names
   required capabilities, not implementation libraries.
7. No implementation task lists, code design or task planning inside a PRD. When the definition is
   ready for large implementation, name `tasks` mode as the next planning step.
8. Each statement in 核心功能, 主要对象 and 业务流程 cites the user's words, a prior note, or
   `file:line`, listed under 依据.

## Refresh

1. Re-read the PRD, current code and docs; list every claim that contradicts them.
2. Correct a contradicted claim with its new citation, or move it to 待确认问题 when the truth is
   unclear.
3. Keep user-authored sections; never delete them without the user's approval.

## Quality gate

- 项目定位 is specific to this project; "all-in-one" or "intelligent platform" fails.
- 核心概念 is one central abstraction, not a feature list.
- 核心功能 are user-verifiable. Three to six is a writing guide, never a reason to invent or remove
  scope.
- 主要对象 use existing repository vocabulary when a repository exists.
- 业务流程 is a short ordered path.
- No claim contradicts existing code, docs or `.lark.json`.

## Lark

- `.lark.json` exists, or the user explicitly asked for Lark/Feishu output → hand the PRD content and
  intended resource key (`resources.prd`) to `lark-workflow-project-docs`.
- Otherwise never suggest a Lark project space and never create `.lark.json`.

## Template

```markdown
## 项目名称
填写项目名称。

## 项目类型
说明它是网站、App、后台系统、工具软件、管理系统，还是其他类型。

## 项目定位
用 1～3 句话说明这个项目的核心用途。

## 解决的问题
说明这个项目主要解决什么问题。

## 核心概念
说明项目最重要的概念、对象或隐喻。
例如："人生 K 线"的核心概念是"用 K 线图表达人生运势变化"。

## 核心功能
列出 3～6 个最重要的功能，每条可由用户验证。

## 主要对象
列出项目中最重要的实体，沿用仓库已有命名。
例如：用户、课程、订单、任务、命盘、图表、设备、车辆、文章等。

## 业务流程
简单说明用户如何使用这个项目。
例如：用户输入信息 → 系统计算 → 生成结果 → 可视化展示 → AI 解读。

## 项目特色
说明它和普通同类项目相比有什么不同。

## 项目关键词
列出 5～10 个最能代表项目的词。

## 约束与不做范围
平台、既有项目约束、截止时间、用户明确不做的内容。

## 待确认问题
1. 问题 —— 推荐默认：...

## 依据
1. 用户原话 / 资料 / `file:line`
```
