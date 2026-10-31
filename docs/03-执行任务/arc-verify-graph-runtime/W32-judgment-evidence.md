# W32 业务判定与证据

状态：`[x]`

`judgeExploration` 按允许/拒绝/副作用写 PASS 或 FAIL，不再只看 HTTP。并发两项请求都写入 `ExploreStep.Calls`。分类失败会停在 `classify_failed` 并带上步骤。恢复失败保留锁；截断续探不提前恢复。
