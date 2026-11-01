> 历史执行记录：本报告中的 vendor-model BLOCKED 及脚本 AI 对照是当时实验状态，不代表现行需求。当前使用已有编程 Agent 推理，不要求 API Key；只用 Markdown 状态与需求记录，不创建 Agent 交接协议。见 [当前状态与需求](当前状态与需求.md)。

# ARC Verify · 证据驱动未知边界发现验收

任务：`verify-boundary-discovery`。基线 `0fa6477`。不新增 Skill，不绕过 R01–R11。

## 对照夹具

`examples/hidden-owner-v1.json` + httptest：两个领取都返回 HTTP 200，独立 Oracle `owner` 只保留最后写入者。图合同只确认「领取会赋值」，不确认排他归属。

## A/B/C

| 组 | 模式 | 规划器 | 期望 |
| --- | --- | --- | --- |
| A | deterministic | 无 | 只走已声明 AGENT 领取 |
| B | heuristic | 无 | 可重复领取/恢复，通常仍是同一主体 |
| C | ai | 脚本规划器 | 在预算内尝试第二主体，并用 owner Oracle 区分解释 |

有效发现 = 两次 HTTP 200 + 两个主体 + 最终一个 owner，且解释不是「双重归属」。假设数量、提问数量不算发现。

本地 httptest 实跑（`.arc/artifacts/verify-boundary-discovery/reports/boundary-abc.json`）：

| 组 | 动作 | 主体 | 当前 owner | 有效发现 | HTTP 结案 | 说明 |
| --- | --- | --- | --- | --- | --- | --- |
| A | 1 | 1 | agent-a | 0 | 0 | 只走已声明领取 |
| B | 3 | 2 | 无（末步恢复清空） | 0 | 0 | 有解释但无稳定 last-writer |
| C | 3 | 2 | agent-b | 1（EX-last） | 0 | 脚本规划器；`unique_to_c=1` |

真实 HTTP 模型未配置：C 组增量只对脚本规划器成立，vendor-model 保持 BLOCKED。

## 验收题

1. 已知事实不再提问：`TestEvidenceFirstDoesNotAskKnownFacts`
2. 不擅自生成 RULE：`TestEvidenceFirstDoesNotInventRULE`、`TestHypothesisNeverBecomesApprovedRule`
3. 假设有业务差：`TestCounterfactualsAreBusinessBound`、`TestPreferDiscoveryPicksOtherRoleAfterFirstClaim`
4. 同 HTTP 不同副作用：`TestCompetingExplanationsNeedOracleNotHTTP`、`TestHiddenOwnerHTTP200IsNotBusinessIdentity`
5. 修订保留历史：`TestUnderstandingRevisionKeepsHistory`
6. 最小实验比喷扫便宜：`TestMinimalExperimentBeatsSpray`
7. 闭环与盲测：`TestBoundaryBlindContrastRecordsHonestABC`、`TestBoundaryDiscoveryPersistsWithoutApprovingRules`

## 门禁

`.arc/artifacts/verify-boundary-discovery/reports/gate-20261101T051630Z.txt`：`red-first: PASS`，`tests(go@tools/arc-verify): PASS`，`format(go): PASS`。`lint(ts)` / `tests(js)` 因 gate 走 npx/npm 触发 `EBADDEVENGINES`（已知 bun 工程问题）；替代：`bunx tsc -b` 无错误，`vp test` 19 passed。

## 未完成

- 真实模型凭证未配置，AI 规划器 HTTP 路径 BLOCKED（W40/R06）。
- IM-mongo 生产链 A/B/C 仍 NOT_RUN（R10）。
- 独立可信 Runner 与受保护 CI 未做。

