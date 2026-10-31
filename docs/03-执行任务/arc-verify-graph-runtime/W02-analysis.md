# W02 分析与完整义务

状态：`[x]`

2026-10-31 审计：工作树已有未提交实现 `model_analysis.go`、`reachability.go`、`model_views.go`、`model_diff.go`、`obligations.go`，来源会话未记录在本卡，完成度与验收均未核；开工先对照本卡清单逐项核实，再补 red 与验收。

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P1，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D3 |
| 难度依据 | 可达性、完整分母、同一上下文与历史有效性共同决定点亮，不能由局部查询代替。 |
| 目标面 | 本仓当前工作树、本地 Go/Turso；Rust 仅远程 .60；真实业务用已授权隔离目标 |
| 前置依赖 | W01：模型/稳定 ID 可用 |
| 下游影响 | 工作台、真实 runner、六技能交接与发布证据 |

## 目标与范围

需求为 [BG01–BG25](../../00-产品需求/06-业务模型驱动验证.md)。范围：engine.go、prepare.go；新增有界求解/模型投影/差异与分析测试。新增文件属于既有工具模块，文件名在实施时回写。

共享设计与接口：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)。唯一状态源：[tracker](../arc-verify-graph-runtime.md)。不覆盖其他会话，不修改未授权远程业务数据，不用静态演示或历史 PASS 代替运行。

## 2026-10-31 追加范围（W10 定稿）

- v2 模型增加可选 `history[]`（校验：transition 属于该 flow、`set_by` 或 `measurement` 至少一个、来源 user/document），见设计 05「状态等价」。
- `exploreFlow`（`reachability.go`）按等价键 `EK` 去重，输出 `merges[]`；相关条件按约束闭包；缺观察不合并并记 `EQUIVALENCE_UNDECIDED`。
- 验收追加：用 W10 原型的两组样例（`.arc/artifacts/verify-route-alignment/research/`）写成正式测试，结果须与原型一致（报名 6 类、IM 草案到达评价且不截断）。

## 2026-10-31 实施记录

核实既有实现（`model_analysis.go`、`reachability.go`、`model_views.go`、`model_diff.go`、`obligations.go`）对照清单的测试覆盖：可达/不可达（`TestAnalysisConditionProofThroughCLI`）、六投影同 ID（`TestAnalysisSixViewsKeepCanonicalIdentity`）、筛选不缩分母（`TestAnalysisPlanningAndFilteringDoNotShrinkObligations`）、依赖传播（`TestAnalysisImpactUsesBusinessDependencies`、`TestAnalysisFlowChangeReachesCapabilityAndTests`）已有。缺的补上：

| 清单项 | 新测试 |
| --- | --- |
| 冲突所有权 / 合法共享 | `TestAnalysisOwnershipConflictNeedsSharedDeclaration` |
| 依赖环诊断 | `TestAnalysisDependencyCycleIsDiagnosed` |
| 错轮次、不同轮次不合并 | `TestAnalysisSeparateRoundsAreNotCombined` |
| 候选变化后陈旧不绿 | `TestAnalysisStalePassIsNotGreen` |
| RAW 改动后不绿 | `TestAnalysisChangedEvidenceIsNotGreen` |
| 预算截断仍有范围 | `TestExplorationBudgetStillTruncates` |

W10 追加范围已实现：新文件 `equivalence.go`（`HistoryAttribute`、`ExplorationMerge`、`equivalenceKey`、`relevantConditions`、`flowAdjacency`、`validateHistory`）；`model.go` 增加 `history` 字段且 v1 拒绝；`model_v2.go` 校验 `history[]`（身份唯一、flow 存在、transition 属于该 flow、`set_by` 或 `measurement` 至少一个、来源 user/document、同一边不能既设又清）；`reachability.go` 的 `exploreFlow` 按"状态 + 声明的历史属性"去重、遵守 `enables` 门控、输出 `merges[]`。草稿里的 `affects` 定名为 `enables`，设计 05 与研究记录已同步。

改了旧合同的测试：`TestAnalysisFlowExplorationIsBoundedAndModelOnly` 原来要求重试自环必须触发截断；按 W10 合同，环靠等价合并收口，改为断言"不截断且有一条合并记录 `submit-flow,retry-ready`"，截断改由 `TestExplorationBudgetStillTruncates` 覆盖。

验收结果：

- `go -C tools/arc-verify test -race -count=1 ./...`：PASS；`go vet`：PASS。
- red：`.arc/artifacts/verify-analysis-equivalence/red/red-001.json`（断言失败）。
- 种下缺陷 15 个全部抓到（键里缺历史、无门控、不合并、所有条件进键、缺观察仍合并、忽略 measurement、无约束闭包、重开不清历史、v1 接受 history、同边既设又清、所有权诊断、ROUND_REQUIRED、两道 STALE 检查同时去掉）。只去掉一道 STALE 检查时另一道仍兜住，属纵深防御，不算漏网。"重开不清历史"首轮漏网，补了"合并键与按路径重放的历史一致"断言。
- 门禁：`gate.sh check --strict --require-red --scope` 0 FAIL；两条 WARN——`equivalence.go` 红灯前的 31 行是三个结构体类型和一个返回空值的函数桩；红灯后测试只加严（约束闭包、合并键与重放历史一致、RAW 改动用例）。报告 `.arc/artifacts/verify-analysis-equivalence/reports/gate-20261031T080349Z.txt`。
- 真实 CLI：`explore` 对带 `history` 的模型输出合并记录，非法 `history` 被 `validate` 拒绝并写明原因，见 `.arc/artifacts/verify-analysis-equivalence/real-entry/cli.log`。

未验证：运行时去重（`equivalence.ignore`、`UNDECLARED_DIFFERENCE`）归 W08/W09；W10 的报名与 IM 草案已写成正式测试，但 IM 草案的转换仍是假设，不是 IM-mongo 合同。

## 上下文检索

<Context-Queries>

```bash
arc-idx files 'tools/arc-verify' --format json
arc-idx search 'f:tools/arc-verify/ (ruleTasks|gate|prepare)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 从合同写可达入口的失败断言，保存 red，之后才改行为。
- [x] 实施本端能力、所有相关调用点/fixture/配置并保持输入验证与错误可见。
- [x] 验证：可达/不可达/预算截断均有范围；六投影同 ID；缺证据/错轮次/陈旧无绿；依赖传播及合法共享/冲突诊断。
- [x] 回写实际文件、命令/结果、未验范围，并同步 tracker。

```bash
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行；前端命令在新增工作台工程执行。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
