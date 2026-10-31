# W28 盲测：未知边界发现

状态：`[x]` 2026-10-31：盲测 `TestAutonomousExploreBlindSeat`。解封后检出 B-STATE/B-AUTH/B-IDEM/B-RACE；B-VALID 漏检（适配器没有关课动作）。报告 `.arc/artifacts/verify-explore-e2e/reports/blind-seat.json`。

| 属性 | 内容 |
| --- | --- |
| 所属端 | 验证 |
| 优先级 | P0 |
| 前置依赖 | W27 |
| 目标面 | 隔离 seat 与独立 enroll 进程；探索器不得读取缺陷清单 |

## 前置条件

探索 Agent 执行前不得获得缺陷清单、注入位置或预期答案。完成后解封比对。

## 具体文件

- `tools/arc-verify/explore_blind_test.go`
- `.arc/artifacts/verify-explore-e2e/reports/blind-seat.json`

## 失败断言

- 用“已知五种缺陷 → 预写五种测试 → 全部检出”作为成功 → 失败。
- 探索源码含密封缺陷标识 → 失败。
- 未发现时编造发现数量 → 失败。

## 验收条件

报告自主发现、规则已覆盖、UNSPECIFIED、漏检原因、误报与消耗。
