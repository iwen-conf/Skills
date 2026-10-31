# W27 自主探索循环：候选、Frontier、真实执行

状态：`[x]` 2026-10-31：`discover run` 落地（观察→候选→HTTP/Command 执行→独立 Oracle→RAW→DISCOVERY→Frontier）。推断候选保持 RISK，不升 RULE。

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P0 |
| 难度 | D3 |
| 前置依赖 | W09 DISCOVERY、W03 执行器、W11 run-chain、W26 |
| 下游影响 | W28 盲测、W29 E2E、W30 快照 |

## 前置条件

DISCOVERY 只能登记外部观察。本卡增加 `discover run`：观察 → 推理候选 → 执行 → 独立 Oracle → RAW → 分类 → 更新 Frontier。

## 具体文件

- `tools/arc-verify/explore_runtime.go`
- `tools/arc-verify/explore_runtime_test.go`
- `tools/arc-verify/main.go`（`discover run`）
- `tools/arc-verify/db.go`（exploration_* 表）

## 修改步骤

1. 红灯：`discover run` 仍被当成非法子命令。
2. 从模型/适配器/推断构造候选；后两者不升 RULE。
3. 持久化 Frontier；按风险、未覆盖、新颖性、成本选下一步。
4. 已知路径用同一 HTTP/Command 适配器执行；未知/分歧/缺前提/无恢复才重规划。
5. 预算：状态、路径、动作、时间。停止时写明原因和剩余空间。

## 执行命令

```bash
go -C tools/arc-verify test -race -count=1 -run 'Explore|DiscoverRun' .
go -C tools/arc-verify test -race -count=1 ./...
```

## 失败断言

- 仅 `discover record` 手写 JSON 不能算自主探索。
- 推断候选被写成 RULE → 失败。
- 无预算停止原因 → 失败。
- 未证明等价却合并 → 失败。

## 验收条件

Agent 能选动作、真执行、按观察调整、登记 UNSPECIFIED、异常后安全停或恢复。
