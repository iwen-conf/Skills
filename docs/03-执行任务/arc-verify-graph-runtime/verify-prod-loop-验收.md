# verify-prod-loop 验收

- 来源提交：基于 `a7215ea164b0f0e0b877ebed9df79a0ca607d0a2`
- 运行时间：2026-11-01
- 执行环境：本机 darwin / Go race / 本地 Turso
- 证据：任务卡 [W36–W47](W36-W47-prod-loop.md)；测试在 `tools/arc-verify/*_test.go`

## A. 业务探索执行引擎

1. 多 Worker 会不会重复领取？不会。`TestExploreConcurrentClaimOnlyOneWinner` 用两个独立 DB 连接竞争，只有一个 winner。
2. 检查点写入失败会不会继续？不会。`TestExploreCheckpointErrorsAreNotDiscarded` 禁止 `_ = d.saveExploreCheckpoint`。
3. 请求成功但响应丢失？保持 `OUTCOME_UNKNOWN`，禁止重放。见崩溃窗口测试。
4. 崩溃后能否安全恢复？过期租约可接管，旧 fencing token 不能提交。
5. 并发两个请求是否有独立证据？`ExploreOperationGroup` 保留双方记录，部分成功不再用单方结果代表整组。
6. 预算耗尽是否保留 Frontier？`TestExploreBudgetExhaustionKeepsFrontier`。
7. 跨进程证明：两连接竞争与重启后所有权测试已跑过。不是独立可信 Runner。

结论：工具层 **PARTIAL / 可复核**。本地夹具证明执行正确性；独立 Runner 仍未做。

## B. AI 未知业务边界

1. 是否实际调用了大模型？**否**。本机无 `ARC_VERIFY_PLANNER_URL` / 密钥。
2–10. HTTP 规划器、入口发现、反馈触发、三模式对照函数已落地；`planner_calls=0`，W40 **BLOCKED**。不编造 AI 独有缺陷。

## C. 生产级可信验证闭环

1–3. iso28886 仍是已授权写面；本轮未扩大授权，链 A/B/C **NOT_RUN**。
4. 门禁校验 RAW 与绑定。
5. 未批准 RULE 不进确认分母。
6–8. 历史 FAIL / 恢复失败 / 篡改证据有门禁测试。
9. 六视图静态快照增加下钻字段。
10. 未完成：IM-mongo 正式 RULE 批准、独立 Runner、真实模型调用。

任意 P0 缺真实业务证据，**不得**标完整生产级验收成功。
