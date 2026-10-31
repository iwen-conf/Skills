# W26 测试可信度：恢复断言与确定性竞态

状态：`[x]` 2026-10-31：空恢复分支已改为四类独立判定；`raceClaim` 改为 occupancy barrier。`go test -race` 全包 PASS。修复前证据 `.arc/artifacts/verify-explore-e2e/logs/pre-fix-*.txt`，红灯 `red/red-001`。

Mode · Tier · Scope lock · Task slug：`repair · T2 · tools/arc-verify/isolated_seat_test.go,recovery_class.go · verify-explore-e2e`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 验证 |
| 优先级 | P0 |
| 难度 | D2 |
| 前置依赖 | W25 隔离 seat-v2 |
| 下游影响 | W27 探索循环、W28 盲测 |

## T2 简报（本阶段共用）

- 目标：自主探索闭环 + 修复测试可信度 + 在已授权范围内做真实 E2E。
- 范围：`tools/arc-verify`（Go CLI/账本/测试）、`workbench` 只读展示字段、W26–W30、tracker。
- 非目标：第七个 Arc 技能、工作台服务、GitHub Actions、未授权 IM-mongo 写/部署、重做 v2 模型。
- 验收：Go race/vet/build、技能与 policy 门禁、工作台 check/test/build、浏览器快照、并发/恢复、盲测、七问报告。

## 前置条件

- HEAD `78666ea`；修复前证据 `.arc/artifacts/verify-explore-e2e/logs/pre-fix-*.txt`。
- 现有 `TestIsolatedSeatMutationDetectsFiveDefects` 对部分恢复只有空分支；`raceClaim` 用 `Sleep(25ms)`。

## 与现有业务的冲突

1. 变异测试空恢复分支改为真实断言：以前缺陷检出后恢复失败仍可能 PASS；现在恢复未独立判定则失败。
2. `raceClaim` 的 sleep 改为同步屏障：竞态从碰运气变为双方读完 occupancy 再写。未开 `raceNoLock` 的 last-seat 路径不变。
3. 检出 FAIL 不得解释为恢复 PASS。恢复未知禁止清空 fixture 继续。

未发现与 run-chain、DISCOVERY 分类、gate 分母的冲突。

## 具体文件

- `tools/arc-verify/isolated_seat_test.go`
- `tools/arc-verify/isolated_seat_credibility_test.go`
- `tools/arc-verify/recovery_class.go`

## 修改步骤

1. 保存修复前证据。
2. 先写失败断言（空分支、Sleep、恢复四类）。
3. 补齐恢复分类与屏障，改变异测试。
4. 补同时读/写、读后写、重复提交、跨角色、中断恢复；区分业务竞争与调度失败。

## 执行命令

```bash
go -C tools/arc-verify test -race -count=1 -run 'IsolatedSeat|RecoveryClass|Concurrency' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

## 失败断言

- 空恢复分支仍存在 → 失败。
- `raceClaim` 仍 `Sleep` → 失败。
- 检出 FAIL 被当成恢复 PASS → 失败。
- 恢复未知仍复用 fixture → 失败。
- 屏障超时记成产品 PASS → 失败。

## 预期证据

`.arc/artifacts/verify-explore-e2e/logs/pre-fix-*.txt` 与 red 记录。

## 验收条件

隔离夹具竞态可确定复现；恢复四类可区分；不把检出当恢复。
