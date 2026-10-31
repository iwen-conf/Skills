# W25 隔离业务链验收与缺陷检出

状态：`[x]`（隔离 seat-v2）；IM-mongo 真实验收仍归 [W06](W06-acceptance.md) / [W20](W20-mutation-detection.md) `[!]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 集成 |
| 优先级 | P0 |
| 难度 | D3 |
| 难度依据 | 要在已授权隔离目标上跑完整证据链、五类受控注入、静态六视图回放，并区分工具 PASS 与 IM-mongo 未授权。 |
| 目标面 | 本仓 `tools/arc-verify` 进程内 httptest；工作台 `file://` 快照；不写 IM-mongo 共享数据 |
| 前置依赖 | W03、W05、W08–W11、W13、W20 CLI |
| 下游影响 | W06 IM-mongo 联合验收仍需用户确认的 v2 RULE、环境批准与账号授权 |

## 目标

让已实现的 ARC Verify 在一条真实入口业务链上证明：能执行、能发现缺陷、能保留证据、能恢复、能复验，并能用图谱展示已验证与未验证。不重新设计系统，不新增第七个 Arc Skill。

## 前置条件

- IM-mongo `.arc/verify/config.jsonc` 的 `lan50`/`tx` 均为 `approved: false`。本卡不得伪称 IM-mongo PASS。
- 已授权的隔离目标：仓库内 `examples/seat-v2.json` + `isolated_seat_test.go` 进程内 HTTP 夹具。
- 写操作只走 CLI；工作台只读快照。

## 改动范围

- 新增：`tools/arc-verify/examples/seat-v2.json`、`tools/arc-verify/isolated_seat_test.go`
- 既有逻辑修正（冲突见下）：`chain.go`、`derived.go`、`discovery.go`、`handoff.go`、`workbench/src/views.ts`、`workbench/src/App.tsx`
- 文档：本卡、W03/W05/W06/W08–W11/W20、tracker

## 与现有业务的冲突

1. `run-chain`：已知路径为空时由“完成”改为 `blocked`（空路径不是验收通过）。调用方若把空链当 PASS 会失败。
2. `derive`：轮次过期（候选/配置/图变化）时拒绝派生。原先可能对陈旧 FAIL 继续派生。
3. 工作台回放：优先带 `first_divergence` 的首次 FAIL，而不是义务上记录的最近一次 PASS。
4. `discover`：UNSPECIFIED 条目补 `capabilities`；`unspecified()` 现在要能读到图。
5. `handoff --to arc-code`：v2 `MODEL` 义务的已确认 FAIL 可以交接。原先 `conditionsMatch` 只认 POSITIVE/NEGATIVE/FORCED，合法 MODEL FAIL 会被退回 verify。v1 路径不变。

未发现与现有六技能交接合同或门禁分母的其它冲突。

## 执行步骤

1. 审计 W01–W24：有代码、有自动化测试、无 IM-mongo 真实验证。
2. 用 seat-v2 跑正常、异常、多角色/并发、超时/断网、恢复失败持锁。
3. 五类受控注入（仅夹具开关，不改生产）：越权、竞态、非法转移、幂等、缺校验。
4. 验证 explore / derive / discover / 状态等价不去掉历史敏感世界。
5. `snapshot` 导出静态 HTML+JSON，真实浏览器 `file://` 打开。
6. 修复 MODEL 交接拒收；FAIL 后新开 PASS，历史 FAIL 仍可交接。
7. 测量分页与 gate 基准，不做无证据的性能改写。

## 测试方法

```bash
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
go -C tools/arc-verify build -o /tmp/arc-verify .
cd tools/arc-verify/workbench && vp check && vp test && vpr build
.venv/bin/python -m pytest -q
.venv/bin/python -m pytest -q Arc/arc:verify/scripts/dbqual/tests
.venv/bin/python scripts/validate_skills.py
```

浏览器：`file://.../.arc/artifacts/verify-isolated-seat/site/index.html`（agent-browser，会话 `seat-snapshot-ddb08da5272e`）。

## 验收标准

- 至少一条隔离业务链覆盖正常与异常，证据链 `RULE → FLOW → TASK → RUN → STEP → RAW → VERDICT`，绑定图/提交/环境/主体/fixture/轮次。
- 五类注入有检出/漏检/不可判定清单，不设及格阈值。
- 快照节点状态来自执行证据；无证据不为绿；全局 BLOCKED 不把未失败节点染红。
- IM-mongo 缺批准时标 BLOCKED，不模拟 PASS。

## 证据与当前状态

| 项 | 结果 | 位置 |
| --- | --- | --- |
| 正常/异常/多角色/并发链 | PASS | `.arc/artifacts/verify-isolated-seat/reports/business-chain.json`；轮次 `Q-b30bc7265b8aca00a7d65a2f9a880bcc` |
| 超时/断网 | BLOCKED，座位未占用 | `TestIsolatedSeatTimeoutAndNetworkStayBlocked` |
| 恢复失败持锁 | BLOCKED，锁未释放 | `TestIsolatedSeatFailedRecoveryKeepsLocksAfterPass` |
| 五类注入 | 5 detected / 0 survived / 0 inconclusive | `reports/detection.json` |
| 探索/派生/UNSPECIFIED | 合法路径可枚举；FAIL 可派生；契约外保持 UNSPECIFIED 并绑定 `reservation.claim` | `TestIsolatedSeatExploreDeriveAndDiscover` |
| 状态等价 | 声明历史把可评分与用户结束的 resolved 分开 | `TestExplorationKeepsHistoryDistinctWorlds` |
| 快照 | seat-v2 六视图；`R-CLAIM/POSITIVE` = FLAKY；回放首次 FAIL `R-d4a8d1faf1b6e8152b1a071a3bf95bb0` | `site/index.html` |
| 浏览器 | 零外部请求、无控制台错误；义务分母 33/33 | `browser/*.png` |
| 交接 | FAIL `R-26dcef223f4c56e5feb6d465a298416b` 可交 arc-code；随后 PASS 不覆盖 | `reports/handoff-first-fail.json` |
| Go race | 276 passed | 2026-10-31 |
| 基准 | pagination ≈127µs/op；gate ≈75ms/op | 本机，不作倍数声明 |

IM-mongo：`lan50`/`tx` 未批准。主机对根路径返回 HTTP 404，但没有 v2 合同、账号/fixture 授权或注入部署授权。结论：`BLOCKED`，不是 PASS。

## 执行清单

- [x] 对照代码与测试审计 W01–W24，不以文档 `[x]` 代替证据。
- [x] 隔离 seat-v2 正常/异常/多角色/并发/恢复。
- [x] 五类受控注入与 `mutation-report`。
- [x] 探索、派生、UNSPECIFIED、状态等价。
- [x] 静态快照 + 真实浏览器。
- [x] MODEL FAIL 可交接 arc-code；历史 FAIL 不变。
- [x] 记录基准；不改生产路径换速度。
- [ ] IM-mongo BG16 三链：待用户确认 RULE、环境 `approved` 与账号授权。
