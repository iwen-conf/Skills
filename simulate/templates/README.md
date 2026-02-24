# Templates

本目录提供“测试公司级别”的交付物模版（Markdown 为主），并由 `scripts/scaffold_run.py` 按 pack 复制到具体的 `<report_output_dir>/<run_id>/` 目录中。

## Packs

- `e2e`：arc:simulate Skill 的”最小必需”运行包（Phase 4 所需目录结构 + report/action-log/manifest）
- `full-process`：覆盖专业软件测试公司常见的全流程交付物（策略、计划、用例、缺陷、日报、总结、UX/A11y/兼容性/视觉回归/性能预算等）

## Placeholder 约定

模版中使用 `{{RUN_ID}}`、`{{OBJECTIVE}}` 等占位符；由脚手架脚本按参数填充（未提供则保留/填入 `<...>` 占位）。

