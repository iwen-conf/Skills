# verify-blind-resume 验收

基线 `923186a`。任务 `verify-blind-resume`。

## 十问

| # | 结论 | 证据 |
| --- | --- | --- |
| 1 AI 能否提出未预写探索 | PARTIAL | 脚本规划器可在未给漏洞答案时提出 force 边界；未绑定在线模型供应商 |
| 2 AI 独有发现 | PARTIAL | 对照记录 planner_calls 与三种模式动作数；AI 不能判 PASS |
| 3 哪些是规则复测 | 已区分 | 确定性模式只走已批准 declared 规则 |
| 4 盲测是否字符串匹配 | PASS | `TestLegacyBlindStringMatchMisattributesHealthyRecords` 证明旧误判；`AttributeBlindCampaign` 绑定 FAIL/RAW |
| 5 漏检原因 | 已记录 | 未激活=unreachable；激活无联动=false_negative；B-VALID 无关课入口 |
| 6 崩溃后续探 | PASS | `TestExploreResumeContinuesFrontierWithoutReplay`、CLI resume |
| 7 并发双方是否入账 | PASS | `TestPerformExploreKeepsBothConcurrentResults` |
| 8 恢复失败是否阻断 | PASS | `TestExploreResumeRefusesFailedRecovery` |
| 9 独立产品 E2E | PASS | iso28886 已知规则 8/8；探索 det/heur/ai 均有动作；28885 只读 200 |
| 10 图谱能否看到探索 | PASS | 工作台 EntityPanel 增加路径/Frontier/AI/FAIL/恢复/模式 |

## IM-mongo

`PASS`（隔离写面）：夹具改绑 `iso28886`；`192.168.1.50:28886` 写入；28885 只读 live=200。证据 `.arc/artifacts/verify-im-mongo-iso/reports/im-mongo-iso28886.json`。变异二进制未部署（不重启 writer）。产品图谱 TTY 批准仍未做。
