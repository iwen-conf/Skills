# ip-docs/CLAUDE.md

## 模块定位

`ip-docs/` 提供 `arc:ip-docs`，专注于软著/专利申请文档草稿写作，不做可行性裁决。

## 核心产物

- `.arc/ip-docs/<project>/copyright/software-summary.md`
- `.arc/ip-docs/<project>/patent/disclosure-draft.md`
- `.arc/ip-docs/<project>/patent/claims-draft.md`
- `.arc/ip-docs/<project>/reports/doc-writing-log.md`

## 脚本入口

- `scripts/scaffold_drafting_case.py`
- `scripts/render_ip_documents.py`

## 协作关系

- 上游：`arc:ip-audit`（优先读取 `handoff/ip-drafting-input.json`）
- 平级：`arc:init`、`arc:deliberate`（用于上下文补全与术语校正）
