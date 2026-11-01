# verify-hardening 验收

基线 `50f54a028e5bc953b9ff6b71d05f7a9e51ff4b99`。本轮按 Red → Green 修内核旁路，不把 W36–W47 的 `[x]` 当完成证明。

未覆盖其他会话的 `测试流程对话/`。独立可信 Runner 仍未做。

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

- 真实 SIGKILL 杀业务进程后的副作用计数
- 真实大模型供应商调用
- iso28886 三条链的正式 RULE 验收
- 独立可信 Runner / 受保护 CI
