# arc-verify 业务链执行与数据库事实中心 · 改造任务追踪

设计规范：[06 业务链执行与数据库事实中心](file:///Users/iluwen/Documents/Code/Skills/docs/01-系统设计/06-业务链执行与数据库事实中心.md)。前置 tracker：[arc-verify-graph-runtime](file:///Users/iluwen/Documents/Code/Skills/docs/03-执行任务/arc-verify-graph-runtime.md)。
基线：`Skills@5758cc4`、[pay_gateway](file:///Users/iluwen/Documents/Code/chensi/pay_gateway) `@3fff0a5d`。

## 阶段一：本机账本核查事实（已完成只读核查）

2026-11-16 通过只读副本实测 `pay_gateway/.arc/verify/data.db`（附 `data.db-wal` 865,232 字节，最后写入 `2026-11-04 18:05`）：

- `.arc/arc.db`：尚未创建激活；`.arc/artifacts/` 已推进到 `2026-11-14`。
- `graphs`：`2`（`pay-gateway-system-20261104-v1`、`pay-gateway-system-20261104-v2`，`approved=0`）。
- `nodes`：`227`（`v2` 含 `1` system、`6` domain、`28` capability、`64` action、`5` role、`13` condition、`70` state、`5` boundary）；`edges`：`362`；`rules`：`64`（全部 `approved=0`）。
- `tasks`：`2`（仅 `MODEL_SCHEMA_GUARD` `BLOCKED` 占位项；真实测试任务 `0`）。
- `runs` / `steps` / `fixtures` / `registered_runners` / `run_bindings` / `verification_rounds` / `model_runs` / `derived_obligations`：全部 `0`。

## 改造工作包清单（BC01–BC10）

| 批次 | 状态 | 范围与涉及文件 | 核心改造与验收标准 |
| --- | --- | --- | --- |
| BC01 | [x] | 跨能力流程与模型校验<br>`tools/arc-verify/model.go`<br>`tools/arc-verify/model_v2.go`<br>`tools/arc-verify/obligations.go`<br>`tools/arc-verify/equivalence.go`<br>`tools/arc-verify/model_views.go`<br>`tools/arc-verify/model_diff.go`<br>`tools/arc-verify/workbench/src/types.ts` | `BehaviorFlow` 支持 `capabilities[]`，`validateSequence` 放行声明的多能力流，能力覆盖计算计入所有触达能力；新增可选 `ConditionSubject` 说明前置条件主体与属性；不破坏既有 v1/v2 校验回归。 |
| BC02 | [x] | 统一数据库事实表与离线自包含存储<br>`tools/arc-verify/db.go`<br>`tools/arc-verify/track.go`<br>`tools/arc-verify/execute.go`<br>`tools/arc-verify/explore_runtime.go`<br>`tools/arc-verify/workbench_evidence.go`<br>`tools/arc-verify/engine.go` | 新增 `step_facts`、`run_entity_bindings`、`step_assertions` 与旧库自动增量建表；修复 `exploration_actions.verdict` 写入步骤号缺陷并同步写 `step_facts`；`execute` 脱敏保存完整 `raw_json` + `facts_json` 入库，干净目录仅凭单库可离线回放并过 SHA-256 校验。 |
| BC03 | [x] | 动态数据提取、实体绑定与会话传递<br>`tools/arc-verify/runner.go`<br>`tools/arc-verify/execute.go`<br>`tools/arc-verify/runtime_model.go`<br>`tools/arc-verify/model_proof.go` | `AdapterCall` 支持 `Headers`、`SessionScope`/`RetainSession`、`Extract[]`（`response_body/response_header/state_cookie/oracle`）及 `{{var.*}}`/`{{actor.*}}`/`{{run.*}}` 渲染；敏感字段或 `Redact` 提取不泄露原始凭据；`modelObservationVerdict` 校验 `TemplateSHA256` + `RequestSHA256` 双绑定。 |
| BC04 | [x] | 有界异步等待与稳定窗口契约<br>`tools/arc-verify/runner.go`<br>`tools/arc-verify/execute.go`<br>`tools/arc-verify/model_proof.go` | `RunnerStep` 支持 `AsyncWait`（`oracle_id`、`until_state`、`until_conditions`、`until_checks`、`allowed_intermediate_states`、`max_wait_ms`、`poll_interval_ms`、`stable_window_ms`、`rule_source_ref`）；记录轮询中间态轨迹、非法中间态即时 FAIL 与超时判定。 |
| BC05 | [x] | `explain` 视图、完备性口径与静态工作台<br>`tools/arc-verify/explain.go`<br>`tools/arc-verify/main.go`<br>`tools/arc-verify/chain.go`<br>`tools/arc-verify/model_analysis.go` | 新增 `arc verify explain`（`--run` / `--graph --node` / `--seq`）从 `step_facts` 回答 6 类业务状态问题；`run-chain` 诚实标注部分子集与未自动化卡片；`gate`/`report` 输出 `completeness` 分解；`TestSnapshotTypesMatchFrontend` 与 `vp check/test/build` 通过。 |
| BC06 | [x] | `arc_tasks` ↔ 测试义务绑定与技能文档<br>`tools/arc/db.go`<br>`tools/arc/resources.go`<br>`tools/arc/state_test.go`<br>`Arc/arc:verify/references/business-graph-v2.md` | 新增 `arc_task_obligations` 表与 `arc task link-verify` / `arc task verify-status` 查询研发任务关联业务义务与最新运行结果（`TestTaskLinkVerifyAndStatus` PASS）；精简更新技能参考文档并重建 `skills.index.json`（满足 <14k token 上限）。 |
| BC07 | [x] | 工具端到端多角色连续旅程与敏感性测试<br>`tools/arc-verify/business_chain_test.go` | 真实 Turso + 多角色模拟支付网关（登录会话 → 创建支付提取 `payment_order_id` → 机构回调 → `AsyncWait` 通知送达 → 原单关联退款提取 `refund_order_id`）：验证正反向流程、实体/路径篡改、漏提取、非法中间态、异步超时、`explore` 统一入库与无 `raw/` 离线 DB 回放，`-race` 全通过。 |
| BC08 | [x] | `pay_gateway` 账本迁移激活<br>`pay_gateway/.arc/` | 备份 `.arc/verify/data.db*` 至 `.arc/backups/pre-migrate-20261116/`（SHA-256 `2bcfad6941...`）并完成 WAL checkpoint 后，执行 `arc db migrate --confirm-offline` 激活 `.arc/arc.db`，`arc db status` 核验 `active: true`、`parity_graphs: 2`。 |
| BC09 | [x] | `pay_gateway` v3 业务模型与连续主链建模<br>`pay_gateway/.arc/verify/pay-gateway-system-20261116-v3-rev2.json`<br>`pay_gateway/docs/07-测试质量/业务流/` | 发布并导入候选 `pay-gateway-system-20261116-v3-rev2`（`previous` 链回 `v3 -> v2`，保留历史）：`nodes=200, edges=364, rules=67, flows=8, tests=42, condition_subjects=13`；拆解 `R-ACCOUNT-HISTORY` 为 4 条分角色原子规则；新增 `FLOW-PAY-CALLBACK-NOTIFY-REFUND` / `T-FLOW-PAY-CALLBACK-NOTIFY-REFUND`；规范化 Oracle ID 并导出新静态快照 `.arc/verify/snapshots/pay-gateway-system-20261116-v3-rev2/`。 |
| BC10 | [x] | `pay_gateway` Runner 模板与隔离验证边界收口（本地模板已完成；交互审批与 `.60` 实测保持 `[!]` BLOCKED/NOT_RUN）<br>`pay_gateway/.arc/verify/runners/t-flow-pay-callback-notify-refund.json`<br>`pay_gateway/.arc/verify/fixtures.isolated.example.json`<br>`pay_gateway/.arc/verify/config.isolated.example.jsonc`<br>`pay_gateway/.arc/verify/review-system.py` | 生成对齐 `v3-rev2` 与 `browser_host.rs` 的多角色 Runner 与夹具/环境模板（已通过脚本交叉核对每个步骤的 Rule Oracle、Invariant Boundary/Measurements 与 State），升级 `review-system.py` 改用 `arc verify approve`（`--check` 67/67 通过）；真实审批须人类交互终端，真实执行须上 `.60` 启动 `browser_host` + 隔离 PG + 只读 `pg_oracle_reader.py`。 |

## 本轮验收命令清单

### Skills 仓库
```bash
go -C tools/arc test -count=1 ./...
go -C tools/arc-idx test -count=1 ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
(cd tools/arc-verify/workbench && vp check && vp test && vp build)
bin="$(mktemp -d)" && bash scripts/install_arc.sh "$bin" && bash tools/arc/smoke.sh "$bin" "$PWD"
bash scripts/install_arc.sh
.venv/bin/python -m pytest -q
.venv/bin/python -m pytest -q Arc/arc:verify/scripts/dbqual/tests
.venv/bin/python scripts/build_skills_index.py
.venv/bin/python scripts/validate_skills.py
```

### pay_gateway 仓库（本地静态/账本边界）
```bash
arc db status --repo /Users/iluwen/Documents/Code/chensi/pay_gateway
arc verify validate --file /Users/iluwen/Documents/Code/chensi/pay_gateway/.arc/verify/pay-gateway-system-20261116-v3-rev2.json
arc verify model --db /Users/iluwen/Documents/Code/chensi/pay_gateway/.arc/arc.db --graph pay-gateway-system-20261116-v3-rev2
arc verify explain --db /Users/iluwen/Documents/Code/chensi/pay_gateway/.arc/arc.db --graph pay-gateway-system-20261116-v3-rev2 --node A-PAY-CREATE
```

## 阶段二实测结果（2026-11-17）

- `tools/arc`: `go -C tools/arc test -count=1 ./...` -> PASS (`4.256s`，含 `TestTaskLinkVerifyAndStatus`)
- `tools/arc-idx`: `go -C tools/arc-idx test -count=1 ./...` -> PASS (`0.799s`)
- `tools/arc-verify`: `go -C tools/arc-verify vet ./...` -> PASS；`go -C tools/arc-verify test -race -count=1 ./...` -> PASS (`255.519s`，含 `TestBusinessChain`)
- `tools/arc-verify/workbench`: `vp check && vp test && vp build` -> PASS（19 tests PASS）
- `.venv/bin/python -m pytest -q`: 215 passed, 1 skipped in 65.88s（`R8_PROTO_PYTHON` 未配置）
- `Arc/arc:verify/scripts/dbqual/tests`: 160 passed, 1 skipped（`SQLVERIFY_SQLFLUFF_CMD` 未配置）
- `scripts/validate_skills.py`: PASS（65 skill files）
- `scripts/install_arc.sh`: PASS（已更新 `~/.local/bin/arc` 与 `~/.local/libexec/arc/{arc-idx,arc-verify}`）
- `tools/arc/smoke.sh`: pre-existing 失败于 line 54（`commit 6667148` 将 `arc-verify` 移至 `libexec/arc` 后未更新 `smoke.sh` 中的 `"$bin_dir/arc-verify"`）
- `pay_gateway`: `arc db migrate --confirm-offline` 成功迁移 2 图（原 `.arc/verify/data.db*` 已备份）；`v3-rev2` 验证/导入/静态快照与 `.60` 模板交叉核对均 PASS；交互审批与 `.60` 运行仍诚实保持 `BLOCKED` / `NOT_RUN`。
