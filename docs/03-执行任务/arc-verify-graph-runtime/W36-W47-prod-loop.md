# W36–W47 生产级强化

状态见 [tracker](../arc-verify-graph-runtime.md)。基线 `a7215ea`。不新建第二套需求系统。

## T2 简报

- 目标：探索领取原子化、动作生命周期可恢复、并发组独立证据、有界 Frontier、可替换模型规划器、只读入口发现、独立 Oracle 溯源、证据门禁、六视图下钻。
- 范围：`tools/arc-verify/*`、本目录任务卡、tracker、技能交接说明。
- 非目标：GitHub Actions、未授权 IM 写、自动 APPROVE 未确认 RULE、用脚本冒充真实模型验收。
- 验收命令：`go -C tools/arc-verify test -race -count=1 ./...`、`vet`、`build`；Python/DBqual/技能校验；工作台 `vp check`/`vp test`/`vp build`。

## W36 原子领取

- 价值：同一 Campaign 任意时刻只有一个有效所有者。
- 源码问题：`claimExploreCampaign` 先查后改。
- 前置：W33。
- 文件：`explore_claim.go`、`db.go`、`explore_resume.go`。
- 测试：`TestExploreConcurrentClaimOnlyOneWinner`（两连接）、`TestExploreStaleWorkerCannotCommit`、`TestExploreWorkerCrashAndSafeTakeover`、`TestExploreClaimTransactionRollback`、`TestExploreWorkerOwnershipAfterRestart`。
- 阻塞：无。

## W37 动作生命周期

- 价值：请求已发出但响应丢失时保持 `OUTCOME_UNKNOWN`，禁止盲目重放。
- 源码问题：`_ = d.saveExploreCheckpoint(...)`。
- 测试：`TestExploreCheckpointErrorsAreNotDiscarded`、`TestExploreCrashWindowsKeepSideEffectingSafe`。
- 阻塞：真实进程杀复现仍用故障注入窗口，不是独立 Runner。

## W38 并发组

- 价值：部分成功不能被单方成功代表。
- 文件：`explore_group.go`、`performExplore`。
- 测试：`TestPerformExploreKeepsBothConcurrentResults`、`TestExploreGroupPartialIsNotSingleSuccess`。
- 阻塞：真实业务并发史仍要 iso28886 运行证据。

## W39 等价与预算

- 价值：无法证明等价不合并；预算耗尽保留 Frontier。
- 测试：`TestExploreBudgetExhaustionKeepsFrontier`。
- 阻塞：完整 CLI 进程峰值需在本机全量跑时记录，不编造倍数。

## W40 真实模型规划器

- 价值：可替换 HTTP 规划器，密钥只走环境变量。
- 文件：`explore_llm.go`。
- 测试：`TestHTTPPlannerUnavailableWithoutCredentials`。
- 状态：`BLOCKED`。本机无 `ARC_VERIFY_PLANNER_URL` / 模型密钥，`planner_calls=0` 不得标真实 AI 完成。

## W41 未知入口

- 价值：从源码路由发现 Adapter 未登记入口，未授权只记录。
- 测试：`TestDiscoverUndeclaredSourceEntries`。
- 阻塞：真实 IM-mongo 源码树发现需对产品仓只读扫描授权。

## W42 假设闭环

- 价值：每轮观察后可再问规划器；规划器不能改 RULE。
- 实现：`shouldAskPlanner` 在未覆盖、失败、部分成功、新副作用后再次触发。
- 阻塞：真实反馈闭环等待 W40 模型。

## W43 三模式对照

- 价值：`contrastExploreModes` 按缺陷 ID 去重，不把 RISK 当漏洞。
- 阻塞：C 组无真实模型调用，不能声称 AI 增量。

## W44 独立 Oracle

- 价值：字段分级 OBSERVED/DERIVED/SYNTHETIC/UNKNOWN；SYNTHETIC 不能单独 PASS。
- 文件：`oracle_observe.go`；`refuseSharedIMTarget` 收紧到登记主机，`:28886` 不再等于授权。
- 测试：`TestOracleSyntheticCannotPass`、`TestRefuseSharedIMWriteTarget`。

## W45 IM-mongo E2E

- 状态：`NOT_RUN`/`BLOCKED`。链 A/B/C 未在本轮扩大授权；27 条 RULE 仍须 TTY APPROVE。
- 测试：`TestIMMongoBusinessChainsStayAuthorized`。

## W46 证据门禁

- 测试：`TestEvidenceGateRejectsTamperAndMissingRAW`（缺 RAW、移植、过期、无规划器）。
- 阻塞：独立可信 Runner / 受保护 CI 仍未建立。

## W47 六视图

- 工作台仍是静态快照。节点下钻增加确认规则、AI 假设、首次偏离、可信标记、人工/AI 区分。
- 未执行节点不点绿。

## 兼容

- 既有 `discover run/resume/status/abort` 参数不变。
- `claimExploreCampaign` 改为返回 `ExploreLease`。
- 检查点失败从忽略改为可见错误：调用方必须处理。
- IM 任意 `:28886` 主机不再视为隔离写面。
