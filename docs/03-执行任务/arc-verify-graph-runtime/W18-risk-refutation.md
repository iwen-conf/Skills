# W18 风险反证记录

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P2，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D2 |
| 难度依据 | 在 W12 的 risks 表上追加反证字段与证据校验；反证本身由 AI/runner 产出，CLI 只登记并校验证据 hash，边界清楚。 |
| 目标面 | 本仓工作树、本地 Go 与原生 Turso（`tursogo`）；不碰远程业务数据 |
| 前置依赖 | [W12](W12-risk-report.md)：RISK 状态与 `origin` 已落库 |
| 下游影响 | W17、W21、风险报告 |

## 目标与范围

需求：[BG27](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「来源分离、反证与自测」；外部依据见 PRD「外部研究核对与借鉴」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

范围：`tools/arc-verify/db.go`（risks 追加 `refutation`、证据路径与 SHA-256 列，幂等迁移）、`main.go`（`risk refute --graph --id --result supported|refuted|not_found --evidence`）、`risk_report.go`。未登记的显示 `not_attempted`。

不做：反证结果不关闭风险、不升级为 RULE；不接受无证据的 `refuted`；不让 CLI 自己去做反证推理。

## 2026-10-31 实施记录

命令：`arc-verify risk refute --graph G --risk ID --result supported|refuted|not_found --evidence PATH`。

- 反证由 AI 或 runner 在别处完成，CLI 只登记：结果必须是三者之一，证据文件必须存在且是普通非空文件（SHA-256 绑定），风险必须在风险报告里（图声明的风险或 `ref/` 参照风险）。
- 追加表 `risk_refutations`（不改 W12 的表）；`report --kind risk` 的每条风险多出 `refutation`（`not_attempted`/`supported`/`refuted`/`not_found`/`evidence_changed`）、证据路径、hash 和全部历史；展示最新一次，并当场重验证据 hash，文件改过就显示 `evidence_changed`。
- 反证不改风险状态：`refuted` 的风险仍是 OPEN，`open_risks` 照算，仍由用户 `accept-risk`/`promote-risk` 决定。
- 新文件 `refutation.go`、`refutation_test.go`；改动 `db.go`、`risk_report.go`、`main.go`（`risk refute` 子命令、`--result`）。

偏差：卡片写"risks 追加列"——按仓库只追加表、不改既有表的迁移约定，改为独立表 `risk_refutations`。派生探索项和 UNSPECIFIED 风险各有自己的类型，本卡只覆盖 `RiskEntry`（图声明风险与参照风险）。

验收结果：

- `go -C tools/arc-verify test -race -count=1 ./...`：PASS；`go vet`：PASS。
- red：`.arc/artifacts/verify-risk-refutation/red/red-001.json`。
- 种下缺陷 8 个：7 个抓到；"不检查证据非空"仍被 `loadEvidence` 拒绝（空路径解析成目录），属纵深防御。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (open_risks|modelGate|accepted)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 先写失败测试：`TestRefutationNeedsEvidenceHash`、`TestRefutedRiskStaysOpen`、`TestUnrefutedRiskShownAsNotAttempted`、`TestRefutationEvidenceTamperDetected`，保存 red。
- [x] 实现迁移、`risk refute` 与报告展示。
- [x] 跑验收命令；回写并同步 tracker。

## 验收

```bash
go -C tools/arc-verify test -race -count=1 -run 'Refutation|RefutedRisk|UnrefutedRisk' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
