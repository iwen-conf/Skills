# implement/CLAUDE.md

## 模块定位

`implement/` 提供 `arc:implement`，负责将方案与需求落地为工程实现并输出可验证交付件。

## 核心产物

- `.arc/implement/<task>/plan/implementation-plan.md`
- `.arc/implement/<task>/execution/execution-log.md`
- `.arc/implement/<task>/reports/execution-report.md`
- `.arc/implement/<task>/handoff/change-summary.md`

## 脚本入口

- `scripts/scaffold_implement_case.py`
- `scripts/render_implementation_report.py`

## 协作关系

- 上游：`arc:refine`、`arc:deliberate`
- 下游：`arc:review`、`arc:simulate`
