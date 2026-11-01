# verify-hardening 验收

基线 `50f54a028e5bc953b9ff6b71d05f7a9e51ff4b99`。本轮按 Red → Green 修内核旁路，不把 W36–W47 的 `[x]` 当完成证明。

未覆盖其他会话的 `测试流程对话/`。

## 2026-11-01 remain-close

基线 `a400155d5661`。本轮把上一轮留下的六项做成可独立复跑的证据，不伪造模型调用，不写共享/生产 IM，不把观察当成 RULE 批准。

| 项 | 最终状态 | 证据 |
|---|---|---|
| 真实 SIGKILL 崩溃窗与副作用计数 | PARTIAL | `TestSIGKILLDuringAcceptedClaimCountsOnce`、`TestSIGKILLAfterCommandObservedCountsOnce`：子进程接受 `/claim` 后 SIGKILL，`/state.claims==1` |
| 真实模型探索 | BLOCKED | `TestRealHTTPPlannerLiveOrBlocked`：无 `ARC_VERIFY_PLANNER_*` / `OPENAI_*`，未伪造调用 |
| 发现未登记入口 → 授权 → 执行 → Oracle | PARTIAL | `TestEntryDiscoverAuthorizeExecuteOracle`：`/hidden-close` 未授权不执行，授权后 `/state.hidden_closes>=1` |
| 三模式盲测 | PARTIAL | `TestThreeModeBlindCompareReportsHonestStats`：deterministic/heuristic/scripted-ai，写出 Injected/Reachable/Triggered/Detected/Missed/NotExecuted；AI 行标明 scripted |
| IM-mongo A/B/C | BLOCKED | 本轮锁在 `192.168.0.60`；60 上无 28886 writer，未回退 `.50`，未起 writer。证据 `reports/im-mongo-chains.json` |
| 独立可信 Runner | PARTIAL | `TestIndependentRunnerProcessStampsPID`：子进程 `execute` 写入 `runner_pid`，且不等于父测试 PID。W16 第二台机器 / 独立账号仍 BLOCKED。本仓不启 GitHub Actions |

Red：`.arc/artifacts/verify-remain-close/red/red-001.json`（exit 1, assertion）。

## 2026-11-01 host-60

执行面锁到 `luwen@192.168.0.60`（hostname `nulltech`）。Mac 本地不再 Fatal 仅因不在 60；`ARC_VERIFY_REQUIRE_HOST60=1` 时必须在 60。写目标只允许 `127.0.0.1` / `localhost` / `::1` / `192.168.0.60`。不再 SSH/HTTP 到 `192.168.1.50`，不再探 28885。

| 项 | 最终状态 | 60 上证据 |
|---|---|---|
| 进程身份 | PASS | `.arc/artifacts/verify-remain-close/reports/host60.json`：`on_60=true` `ip=192.168.0.60` `user=luwen`；remain pid `2078158`，整包 `2119741`，race `2151063` |
| 拒绝 lan50 | PASS | `TestHost60LockRefusesLan50`、`TestRefuseSharedIMWriteTarget`：`192.168.1.50:28886/28885` 被拒 |
| SIGKILL / 入口 / 三模式 / Runner | PARTIAL | 同一 `go test` 进程在 60 上 PASS（约 61s）；三模式报告 `reports/three-mode-contrast.json` |
| 真实模型 | BLOCKED | `reports/real-planner.json`：无 planner URL/model/key，未伪造调用 |
| IM A/B/C 与 iso 探索 | BLOCKED | `reports/im-mongo-chains.json`、`verify-explore-e2e/reports/im-mongo-iso28886-blocked.json`：`live=0 ready=0`，`base=http://127.0.0.1:28886` |

`go test -count=1 -timeout 12m -run 'TestRemainEvidenceRanOnHost60|TestHost60LockRefusesLan50|…|TestIMMongo|TestIMISO' .` 在 60 上 exit 0。未启动 isolation writer。

整包：`ARC_VERIFY_REQUIRE_HOST60=1 go test -count=1 -timeout 25m ./...` 在 60 上 **680.835s exit 0**。同环境 `go test -race -count=1 -timeout 25m ./...` **820.326s exit 0**。`go vet ./...` exit 0。kinds/chain/execution 合成夹具超时加宽（20s/4s、deadline 5s），只为 60 上 Turso/HTTP 慢启动仍能跑完同一断言，不改 execute 判定公式。

`gate.sh check --require-red`：Go/Python 测试与 red-first PASS。`tsc`/`vitest` 经 npm 因 workbench `devEngines.packageManager=bun` 报 EBADDEVENGINES；已用 `vp test`（19 passed）与 `bunx tsc --noEmit` 代替。

## 结果矩阵

| 编号 | 代码完成 | 反例先失败 | 修复后通过 | 独立运行证据 | 最终状态 |
|---|---|---|---|---|---|
| R01 | 是 | 是 | 是 | 两连接 + 独立进程 claim | PARTIAL：目标侧重叠执行仍靠幂等，不是独立 Runner |
| R02 | 是 | 是 | 是 | OBSERVED 调和与 Attempt ID 单测 | PARTIAL：崩溃窗主要是故障注入，不是真实 SIGKILL 子进程杀业务 |
| R03 | 是 | 是 | 是 | depth/evidence 预算单测 | PARTIAL：内存只记录 rusage.maxrss 或明确标注的 MemStats.Sys |
| R04 | 是 | 是 | 是 | `arc-verify gate` 退出码 + policy 单测 | PARTIAL：本仓 policy verify 不绑 academic-v1 探索账本 |
| R05 | 是 | 是 | 是 | coverage + 工作台字段 | PARTIAL：样本快照仍是旧导出，新字段靠后端生成 |
| R06 | 是 | 是 | 本地协议 | 本地 OpenAI 兼容 mock | BLOCKED：无真实模型凭证，未伪造调用 |
| R07 | 部分 | 是 | 发现/拒绝 | Go AST + OpenAPI | PARTIAL：未授权入口不执行；受控业务自主发现闭环未跑 |
| R08 | 部分 | 否 | 统计字段 | Injected/Triggered 分列 | BLOCKED：无真实三模式盲测数据 |
| R09 | 否 | 否 | 否 | 现有 SYNTHETIC 分级 | BLOCKED：不把自建代理当产品 Oracle |
| R10 | 否 | 否 | 否 | 授权矩阵保持 NOT_RUN | BLOCKED：iso28886 未获本轮新授权 |
| R11 | 部分 | 是 | 正式 gate | CLI 拒绝空 Step 探索 | PARTIAL：工作台读同一 DTO；独立 Runner 未做 |

## 五项 P0

1. fencing 旁路：`saveExploreCheckpoint` 改为 INSERT-only；运行期只走 `saveExploreCheckpointOwned`（campaign/worker/fence/lease/status）。
2. OBSERVED + Request ID：调和不再跳过 OBSERVED；Attempt ID 保留同 Request ID 的多次尝试。
3. 证据门禁：`d.gate()` / `modelGate()` 调用 `attachExploreEvidence()` → `evaluateEvidenceGate()`。
4. 图谱可信：`Trusted` 仅当 `evidence_trust=VERIFIED`；业务 FAIL 可与证据状态分离。
5. 规划审计：`persistPlannerAudit` 写入 `exploration_planner_audit`；真实模型仍 BLOCKED。

## 反例与修复命令

Red：`.arc/artifacts/verify-hardening/red/red-009.json`（exit 1, assertion）。

修复后：

```text
go -C tools/arc-verify test -count=1 -timeout 180s -run 'TestUnownedCheckpoint|TestOfficialGateRejectsBrokenExploreEvidence|TestPlannerAuditIsPersisted|TestIndependentProcessesClaimOnlyOnce' .
# exit 0

go -C tools/arc-verify test -race -count=1 -timeout 20m ./...
# exit 0

go -C tools/arc-verify vet ./...
# exit 0

go -C tools/arc-verify build -o /tmp/arc-verify-hardening .
# exit 0

cd tools/arc-verify/workbench && vp test
# 19 passed

.venv/bin/python -m pytest -q
# 477 passed, 1 skipped

.venv/bin/python -m pytest -q Arc/arc:verify/scripts/dbqual/tests
# 160 passed, 1 skipped

.venv/bin/python scripts/validate_skills.py
# 65 skill files
```

`gate.sh check --require-red`：Go/Python 测试与 red-first PASS。`tsc`/`vitest` 经 npm 因 workbench `devEngines.packageManager=bun` 报 EBADDEVENGINES；已用 `vp test` 与 `bunx tsc --noEmit` 代替。mypy 对改动的 `tests/test_arc_policy_business_graph.py` 报预存的 `arc_policy` stub 缺失，未加 ignore。

## 主要测试

- `TestUnownedCheckpointCannotOverwriteNewWorker`
- `TestExpiredLeaseRejectsOwnedCheckpointBeforeTakeover`
- `TestExpiredLeaseCannotDispatchAnotherAction`
- `TestAbortPreventsRunningWorkerCheckpoint`
- `TestResumeWithStaleFenceCannotRewrite`
- `TestIndependentProcessesClaimOnlyOnce`
- `TestSameRequestIDKeepsSeparateAttempts`
- `TestObservedCrashWindowIsReconciledWithoutReplay`
- `TestEvidenceGateEmptyReportIsNotPass`
- `TestOfficialGateRejectsBrokenExploreEvidence`
- `TestCoverageDoesNotTrustRAWPresenceAlone`
- `TestPlannerAuditIsPersisted`
- `TestBudgetFieldsStopExpansion`
- `TestHTTPPlannerProtocolErrors`
- `test_policy_fails_when_official_gate_rejects_explore_evidence`

## 未验证边界

- 真实大模型供应商调用（仍无凭证）
- iso28886 三条链的正式 RULE 批准（本轮只记 live 观察）
- W16 独立机器 / 独立 Runner 账号 / 受保护 CI
- 目标侧重叠执行的去重仍靠幂等，不是第二执行主体
