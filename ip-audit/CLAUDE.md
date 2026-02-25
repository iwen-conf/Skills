# ip-audit/CLAUDE.md

## 模块定位

`ip-audit/` 提供 `arc:ip-audit`，负责软件专利/软著可行性审查，不负责最终申请文书写作。

## 核心产物

- `.arc/ip-audit/<project>/reports/ip-feasibility-report.md`
- `.arc/ip-audit/<project>/reports/filing-readiness-checklist.md`
- `.arc/ip-audit/<project>/handoff/ip-drafting-input.json`

## 脚本入口

- `scripts/scaffold_audit_case.py`
- `scripts/render_audit_report.py`

## 协作关系

- 上游：`arc:init`、`arc:review`（提供上下文）
- 下游：`arc:ip-docs`（消费 handoff JSON）
