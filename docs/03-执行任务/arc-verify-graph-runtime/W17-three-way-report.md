# W17 三向比对汇总

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P2，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D2 |
| 难度依据 | 只汇总已有三类数据（EXISTENCE 候选缺口、UNSPECIFIED、peer_reference RISK），不新增判定；风险在于来源类别不能串。 |
| 目标面 | 本仓工作树、本地 Go 与原生 Turso（`tursogo`）；不碰远程业务数据 |
| 前置依赖 | [W07](W07-obligation-kinds.md) EXISTENCE 候选缺口可查；[W09](W09-discovery.md) UNSPECIFIED 可查；[W12](W12-risk-report.md) `report --kind risk` 与 RISK `origin` 已有；[W19](W19-reference-catalog.md) 参照合同已定 |
| 下游影响 | W13 快照分析空间、W14 文档 |

## 目标与范围

需求：[BG26](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「来源分离、反证与自测」；外部依据见 PRD「外部研究核对与借鉴」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

范围：`tools/arc-verify/risk_report.go`（W12 新增）加三栏输出；测试 `risk_report_test.go`。三栏：要求有而实现无、实现有而要求无、参照有而要求无；每条带来源类别和证据引用。

不做：不把实现证据写进规则表；不把三栏合成一个分数；不判 FAIL。

## 2026-10-31 实施记录

`report --kind risk` 新增 `three_way`，三栏分开、不合成分数、不判 FAIL：

- `required_not_implemented`（来源 `requirement`）：规则的动作最新扫描为 NOT_FOUND，附扫描证据路径与 hash；仍只是候选缺口。
- `implemented_not_required`（来源 `implementation`）：扫描为 PRESENT 但没有任何规则要求的动作；以及全部 UNSPECIFIED 条目（附类别、原因和分流状态）。
- `referenced_not_required`（来源 `peer_reference`）：当前参照风险，附条目引用 `<条目>@<catalog sha>`。
- 实现证据与参照都不写规则表、不进验收报告（`TestImplementationEvidenceNeverBecomesRule`、`TestPeerReferenceOnlyYieldsRisk`）。
- 新文件 `three_way_test.go`；改动 `risk_report.go`。

验收结果：`go -C tools/arc-verify test -race -count=1 ./...` PASS、`go vet` PASS；red `.arc/artifacts/verify-three-way-report/red/red-001.json`；种下缺陷 6 个全部抓到（"已实现的规则动作被当成缺口或多余"首轮漏网，补了已实现且有规则的动作）。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (open_risks|modelGate|accepted)' --format ai-md --context 2 --max 30
arc-idx search 'f:tools/arc-verify/ -f:_test (NOT_FOUND|implementation_scans)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 先写失败测试：`TestThreeWayReportKeepsSourcesApart`、`TestImplementationEvidenceNeverBecomesRule`、`TestPeerReferenceOnlyYieldsRisk`，保存 red。
- [x] 实现三栏汇总。
- [x] 跑验收命令；回写并同步 tracker。

## 验收

```bash
go -C tools/arc-verify test -race -count=1 -run 'ThreeWay|ImplementationEvidenceNever|PeerReferenceOnly' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
