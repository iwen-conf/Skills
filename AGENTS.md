# Skills 项目事实

- 主技能源码在 `Arc/`；共享协议在 `Arc/_shared/`。现有六个 Arc 技能与 `plugins/arc-policy/` 协作。
- `tools/arc-verify/` 是短生命周期 Go CLI，使用官方 `tursogo` 进程内驱动。Python 脚本负责既有测试与校验工具。
- `arc-idx` profile：`skills`、`docs`、`code`、`meta`。
- 技能注册表从源码生成：`.venv/bin/python scripts/build_skills_index.py`；不要手改 `skills.index.json`。

## 2026-10-30：verify 路线裁决

- 按完整《自动化测试规范设计》对话及 [PR #1](https://github.com/iwen-conf/Skills/pull/1) 强化现有技能。
- 先列应有业务并与用户核对；实现清单独立发现。用户确认的 RULE → 业务图 → 有界任务卡 → 真实执行 → 独立 RAW → 判定与历史 FAIL。
- AI 推断与行业惯例只算 RISK。覆盖条件满足、不满足、强制操作及已知恢复路径；保留首次偏离，按验证后的实际状态继续新任务。
- 图和历史保存在被测项目本地 `.arc/verify/data.db`；JSONC 保存环境、账号别名与凭证引用。测试服务器是被测目标。
- 当前图门禁只覆盖已规划的正向、逐条件负向和强制义务；恢复、跨角色、并发、真实 E2E 与独立可信 Runner 另需执行证据。
- 实施与未完成边界记录在 [任务文档](docs/03-执行任务/arc-verify-graph-runtime.md)。

## 2026-10-31：verify 演化路线核对裁决

- 已逐轮对照对话导出（352 条消息，SHA-256 `458e8ecc810028a59f5dd08b3e6b5383c48a6e38efe41691f9b09747065618ac`）和多视图附件；偏差、排序和顺序以 tracker「2026-10-31 演化路线核对」为准。
- 工作台改为**静态快照**：CLI 一次性导出 HTML+JSON，用浏览器只读打开；不起本地服务，页面里不执行、不批准、不重试，写操作只走 CLI。原 `workbench` 前台服务方案作废。
- 首个真实试点是 IM-mongo（源码 `~/Documents/Code/chensi/IM-mongo`；BG16 三条链，再扩到 39 族）；`academic-v1` 只作工具自测示例。
- 本仓不启用 GitHub Actions（`tasks/lessons.md`）；不拿受保护 CI 当可信边界，独立可信 Runner 仍未做。

## 本仓验收命令

```bash
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
.venv/bin/python -m pytest -q
.venv/bin/python -m pytest -q Arc/arc:verify/scripts/dbqual/tests
.venv/bin/python scripts/validate_skills.py
```
