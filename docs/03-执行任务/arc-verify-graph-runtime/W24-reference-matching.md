# W24 参照库导入与匹配

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P2，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D3 |
| 难度依据 | 新增导入校验、有界槽位绑定、四种抑制和追溯记录；匹配结果只能是 RISK，不能串进 RULE 或验收 gate；与 W12 的 RISK 来源字段、W21 驳回台账交互。 |
| 目标面 | 本仓工作树、本地 Go 与原生 Turso（`tursogo`）；不碰远程业务数据 |
| 前置依赖 | [W19](W19-reference-catalog.md)：合同已定；[W12](W12-risk-report.md)：RISK `origin`/`reference` 已落地 |
| 下游影响 | W17 三向比对的"参照有而要求无"一栏、W13 快照、W14 文档 |

## 目标与范围

需求：[BG28](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「来源分离、反证与自测」的"参照库"行；研究记录：`.arc/artifacts/verify-route-alignment/research/reference-catalog.md`。唯一状态源：[tracker](../arc-verify-graph-runtime.md)。

范围：`tools/arc-verify/` 新文件 `references.go` 与测试；`main.go` 增加 `reference-import`、`reference-match` 两个子命令；`db.go` 追加表（参照条目快照、匹配记录）；v2 模型可选字段 `reference_bindings`（`model_v2.go` 校验）。

- 导入：读取 `.arc/verify/references/*.jsonc`，按研究记录 §3 校验（来源、可信度、槽位、关系），保存条目与文件 SHA-256。
- 匹配：按 §4 求绑定；恰好一个绑定 → 生成 `origin=peer_reference` 的 RISK，`reference=<条目ID>@<SHA前12位>`；无绑定 → `NO_BINDING`（写明槽位）；多个 → `AMBIGUOUS` 诊断；四种抑制与 50 条上限。
- 匹配记录可查，RISK 能追回条目、文件 hash 与绑定。

不做：不把匹配结果写成 RULE；不进验收 gate；不抓取外部代码；不做语义/LLM 相似度匹配。

## 2026-10-31 实施记录

命令：`arc-verify reference-import --file <catalog.jsonc>`、`arc-verify reference-match --graph G`。

- 导入：JSONC 去注释后严格解码，按研究记录 §3 校验（唯一 ID、四级 `trust`、`source`/`source_ref`/`invariant` 非空、1–8 个槽位且种类是模型节点种类、提示词小写、关系只连接已声明槽位），按文件 SHA-256 存进 `reference_catalogs`；不合格的整份拒收、不留行。
- 匹配：按可信度、再按 ID 排序；槽位候选为种类相同且 ID 或名称含提示词的节点（每槽最多 64 个），回溯求满足所有关系的绑定。恰好一个 → `RISK`（`origin=peer_reference`、`reference=<条目>@<SHA 前 12 位>`，描述带绑定、可信度和来源）；没有 → `NO_BINDING` 并写明槽位或 `relations`；多个 → 一条 `AMBIGUOUS`，列出候选，不生成风险；模型 `reference_bindings` 标不适用或 `covered_by` → `SUPPRESSED` 并写明依据；单次最多 50 条风险，超出记 `TRUNCATED`。每次匹配写 `reference_matches` 历史，当前候选替换写入 `reference_current`。
- 结果只出现在 `report --kind risk` 的 `reference_risks`；验收 gate、`open_risks` 和规则表都不受影响（测试比对 gate 的 verdict、required_tasks、open_risks 与规则数）。
- v2 可选字段 `reference_bindings`（条目、`bindings` 槽位到节点、`not_applicable` 加理由、`covered_by` 指向规则/测试/不变量、user/document 来源），v1 拒绝；`types.ts` 同步该字段。
- 示例参照库 `tools/arc-verify/examples/references/enrollment.jsonc`、`im.jsonc`（W19 草案；报名领域 4 条，IM 领域 5 条）。`TestExampleReferenceCatalogsAreValid` 复现 W19 演示：`academic-v1` 上 3 条风险候选，IM 条目全部无绑定。
- 新文件 `references.go`、`references_test.go`；改动 `model.go`、`model_v2.go`、`db.go`、`risk_report.go`、`main.go`。

偏差：卡片写"W21 台账里驳回过的同绑定不再生成"——W21 尚未实现，这一项抑制留给 W21 接入；"已存在同 reference 与同绑定的 RISK"由每次匹配替换 `reference_current` 实现，不会重复。

验收结果：

- `go -C tools/arc-verify test -race -count=1 ./...`：PASS；`go vet`：PASS；`vp check`：通过。
- red：`.arc/artifacts/verify-reference-matching/red/red-001.json`。
- 门禁（`reports/gate-20261031T111235Z.txt`）：Go 部分、scope、red-first PASS；`lint(ts)`/`tests(js)` FAIL 是门禁用 npx、被 bun 工程拒绝，前端只改了 `types.ts` 一个字段，`vp check` 通过、`vp test` 17 项通过。WARN：`model.go` 红灯前 21 行是新增一个字段后 gofmt 重排了整个结构体的对齐；`references.go` 红灯前 36 行是类型与桩；红灯后测试只修正了导入夹具（去掉不存在的 `previous`）并新增示例参照库用例。
- 种下缺陷 10 个全部抓到（来源校验、关系约束、歧义当风险、上限、显式绑定、覆盖、不适用、绑定校验、结果丢失、风险进 gate）。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (validateRiskOrigin|peer_reference|riskRows)' --format ai-md --context 2 --max 30
arc-idx search 'f:tools/arc-verify/ -f:_test (decodeJSONC|jsonc|readApprovedEnvironment)' --format ai-md --context 2 --max 20
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 先写失败测试并保存 red：`TestReferenceImportRejectsUnsourcedEntry`、`TestReferenceMatchUniqueBindingCreatesPeerRisk`、`TestReferenceNoBindingNamesSlot`、`TestReferenceAmbiguousGivesOneDiagnostic`、`TestReferenceSuppressedByCoversAndBindings`、`TestReferenceRiskNeverEntersAcceptanceGate`、`TestReferenceMatchIsCapped`。
- [x] 实现导入、匹配、抑制与记录；把研究记录的两组样例（`academic-v1` 3 条 RISK / 7 条无绑定；course-v2 用 `reference_bindings` 解决"报名成功"同义词）写成正式测试。
- [x] 跑验收命令；回写并同步 tracker。

## 验收

```bash
go -C tools/arc-verify test -race -count=1 -run 'Reference' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```
