# W21 分流精度台账

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P3，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D2 |
| 难度依据 | 只是把已有分流事件按 `origin` 汇总；难点是样本少时不误导，所以要同时给出样本量。 |
| 目标面 | 本仓工作树、本地 Go 与原生 Turso（`tursogo`）；不碰远程业务数据 |
| 前置依赖 | [W12](W12-risk-report.md)、[W09](W09-discovery.md)、[W08](W08-derived-obligations.md) 的分流事件已落库 |
| 下游影响 | 风险报告、ops-learning |

## 目标与范围

需求：[BG30](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「来源分离、反证与自测」；外部依据见 PRD「外部研究核对与借鉴」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

范围：`tools/arc-verify/db.go`（追加分流事件表）、`risk_report.go`（按 `origin` 汇总接受/驳回/待定与样本量）。

不做：不作门禁；不把比例写成质量分；样本少于 10 条时标"样本不足"。

## 2026-10-31 追加（W24 交接）

参照匹配要读本卡的驳回记录：同条目同绑定被用户驳回后，`reference-match` 应输出 `SUPPRESSED`（依据写台账事件）。接入点：`references.go` 的 `matchReferences`。

## 2026-10-31 实施记录

- 新决定 `reject-risk --graph G --risk ID`（真实终端，需输入 `REJECT <id>` 和用户原话；管道拒绝）：图声明的风险写 W12 的 `risk_events`（状态 REJECTED，保留原话，不再计入 `open_risks`）；参照风险（`ref/…`）写追加表 `reference_rejections`（条目 + 绑定 + 原话）并移出当前候选，之后 `reference-match` 遇到同条目同绑定输出 `SUPPRESSED`（"rejected by the user: …"）——这就是 W24 交接的抑制。
- `report --kind risk` 新增 `triage_summary`：按来源类别（`ai_inference`、`industry_practice`、`peer_reference`、`undeclared`、`discovery`、`derived_inference`）统计接受、驳回、待定和样本量；样本少于 10 标 `insufficient_sample`。归类：风险的 ACCEPTED_RISK/PROMOTED 为接受、REJECTED 为驳回；UNSPECIFIED 的遗漏需求/不应存在为接受（发现属实）、允许扩展为驳回；派生探索项目前没有分流入口，全部待定。
- 汇总不进门禁（`TestTriageLedgerNotAGate` 比对 gate 的 verdict 与 required_tasks，且 gate 里没有该字段）。
- 新文件 `triage_ledger.go`、`triage_ledger_test.go`；改动 `db.go`、`risk_report.go`、`references.go`（驳回抑制、`bindingDetail`）、`main.go`。

偏差：卡片写"追加分流事件表"——各来源已有自己的决定表（`risk_events`、`unspecified_triage`），只为缺的参照驳回新增 `reference_rejections`，汇总从现有表读，不复制事件。

验收结果：`go -C tools/arc-verify test -race -count=1 ./...` PASS；red `.arc/artifacts/verify-triage-ledger/red/red-001.json`；种下缺陷 9 个全部抓到（"驳回不要求原话"首轮漏网，补了断言）。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (open_risks|modelGate|accepted)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 先写失败测试：`TestTriageLedgerGroupsByOrigin`、`TestTriageLedgerShowsSampleSize`、`TestTriageLedgerNotAGate`，保存 red。
- [x] 实现事件表与汇总。
- [x] 跑验收命令；回写并同步 tracker。

## 验收

```bash
go -C tools/arc-verify test -race -count=1 -run 'TriageLedger' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
