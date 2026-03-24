# DB Evidence (Read-only)

本目录用于存放只读数据库验证的证据文件：

- `query-0001.txt`：执行的 SQL / 命令（只读 SELECT）
- `result-0001.txt`：输出结果

注意：禁止 INSERT/UPDATE/DELETE。

## Change Control（数据库迁移/改动必须经过同意）

- 默认：仅允许只读验证（SELECT）。
- 若确需执行数据库迁移/DDL/DML（例如为了让修复生效或补历史数据）：
  1) 必须先获得用户明确同意（把同意原文写入 `migration-approval.md`）
  2) 写清楚迁移计划（`migration-plan.md`）与回滚策略
  3) 把执行证据落盘（`migration-execution.md`，以及相关输出文件）
