---
name: "arc:ip-docs"
description: "面向软件项目的知识产权申请文档写作助手。基于项目上下文与审查结论，辅助撰写软件著作权与发明专利申请文档草稿，包括说明书、技术交底书、权利要求草案与附图文字说明。"
---

# arc:ip-docs — 专利/软著文档写作

## Overview

`arc:ip-docs` 专注于文档写作，不做可行性裁决。它消费 `arc:ip-audit` 的审查结果，并结合项目代码上下文输出可编辑申请文档草稿。

## Mandatory Linkage（不可单打独斗）

1. 默认先读取 `arc:ip-audit` 产物：`handoff/ip-drafting-input.json`。
2. 若交接文件缺失，优先提示先执行 `arc:ip-audit`；仅在用户明确要求时做最小化兜底草稿。
3. 文档内容必须回连项目上下文：`CLAUDE.md` + `ace-tool` 证据。
4. 对术语冲突或技术路线不明确，串联 `arc:deliberate` 做定稿前校正。

## When to Use

- 已完成审查评估，开始准备软著/专利申请文档。
- 需要撰写软著说明书、代码材料说明、专利技术交底书。
- 需要形成权利要求草案和附图文字说明供代理人加工。

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `project_path` | string | 是 | 目标项目根目录绝对路径 |
| `project_name` | string | 否 | 默认从路径推导 |
| `audit_case_dir` | string | 否 | `arc:ip-audit` 输出目录；默认 `<project_path>/.arc/ip-audit/<project-name>/` |
| `software_name` | string | 否 | 软件名称，未提供则从交接文件读取 |
| `applicant_name` | string | 否 | 申请主体名称 |
| `target_docs` | enum | 否 | `copyright` / `patent` / `both`，默认 `both` |
| `output_dir` | string | 否 | 默认 `<project_path>/.arc/ip-docs/<project-name>/` |

## Dependencies

- **arc:ip-audit**（强推荐）：作为主输入。
- **ace-tool MCP**（必须）：校正文档中的技术细节与代码证据。
- **arc:init**（推荐）：复用模块索引，减少重复扫描。
- **Task API**（推荐）：调用 `deep`/`writing` 生成高质量文本。

## Context Priority（强制）

1. `audit_case_dir/handoff/ip-drafting-input.json`
2. `.arc/ip-docs/<project>/context/doc-context.md`
3. 项目 `CLAUDE.md`
4. `ace-tool` 源码补扫

## Critical Rules

1. **不编造**：任何技术细节必须能回溯到代码或审查交接信息。
2. **术语一致**：同一对象全文统一称谓，不得自由替换同义词。
3. **结构完整**：交底书、权利要求、说明文档必须按模板章节完整输出。
4. **草稿定位**：输出为“可编辑申请草稿”，不得声称最终法律文本。
5. **双轨拆分**：软著材料与专利材料分目录产出，不混写。

## Instructions

### Phase 1: 读取审查交接与项目上下文

1. 读取 `ip-drafting-input.json`。
2. 提取目标资产、优先级和风险提示。
3. 用 `ace-tool` 二次核对关键技术描述。

### Phase 2: 软著文档草稿（如需要）

输出：

- `copyright/software-summary.md`
- `copyright/manual-outline.md`
- `copyright/source-code-package-notes.md`

### Phase 3: 专利文档草稿（如需要）

输出：

- `patent/disclosure-draft.md`
- `patent/claims-draft.md`
- `patent/drawings-description.md`

### Phase 4: 交付记录

输出：`reports/doc-writing-log.md`，记录输入来源、假设、待人工补充项。

## Scripts

```bash
python ip-docs/scripts/scaffold_drafting_case.py --project-path <project_path>
python ip-docs/scripts/render_ip_documents.py --case-dir <output_dir> --handoff-json <audit_case_dir>/handoff/ip-drafting-input.json --target-docs both
```

## Artifacts

默认输出目录：`<project_path>/.arc/ip-docs/<project-name>/`

- `context/doc-context.md`
- `copyright/software-summary.md`
- `copyright/manual-outline.md`
- `copyright/source-code-package-notes.md`
- `patent/disclosure-draft.md`
- `patent/claims-draft.md`
- `patent/drawings-description.md`
- `reports/doc-writing-log.md`

## Quick Reference

| 输出物 | 用途 |
|------|------|
| `disclosure-draft.md` | 专利技术交底书草稿 |
| `claims-draft.md` | 权利要求草案 |
| `software-summary.md` | 软著申请摘要 |
| `manual-outline.md` | 软著操作说明书提纲 |
