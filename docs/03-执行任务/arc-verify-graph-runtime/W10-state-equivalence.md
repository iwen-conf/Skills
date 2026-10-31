# W10 状态等价判定研究

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 研究 |
| 优先级 | P1，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D4 |
| 难度依据 | 对话 L660–L668 指出这是"最关键"的未解问题：合并过粗会漏路径，过细会状态爆炸；等价依据需要模型声明哪些历史属性影响后续行为，可能要改 W01 的模型字段，先研究再定实现卡。 |
| 目标面 | 文档与本地原型测试；不改正式源码（原型放在一次性测试文件或 `.arc/artifacts/`） |
| 前置依赖 | [W01](W01-model.md)：v2 模型字段清单已定 |
| 下游影响 | W02 有界探索去重、W08 派生去重、W09 探索去重 |

## 目标与范围

需求：[BG21](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「补齐的义务、报告与探索」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

要回答的问题：

1. 等价键由哪些部分组成：状态节点、条件取值、主体/角色，以及哪些"历史敏感属性"（通知、审计、名额占用等）需要模型显式声明？
2. 模型如何声明这些属性（新字段还是复用 invariants/data 节点）？需不需要改 W01 的 schema？
3. 有界探索遇到"可能等价但依据不全"的状态时怎样处理（不合并并记诊断）？
4. 合并记录怎么保存，才能从被合并路径追溯回来？

证据产物：`.arc/artifacts/verify-route-alignment/research/state-equivalence.md`，至少包括：
- 对话 L622–L630"报名成功"五种变体，在 `academic-v1` 上的建模与判定结果；
- IM-mongo 一张卡（排队/领取）的判定草案；
- 一个能跑的原型测试结果。

不做：不直接改 `reachability.go` 等正式实现；不替用户决定业务上"哪些历史算不同"——这类业务判断列为问题交给用户。

## 2026-10-31 结论

采用"状态 + 相关条件 + 角色/主体别名 + 声明的历史属性"作等价键；合同已写入设计 05「状态等价」行，研究记录 `.arc/artifacts/verify-route-alignment/research/state-equivalence.md`。

- 原型（一次性测试，已从包中删除，源码与输出存档于同目录）：对话"报名成功"变体中，只看状态的方案把 4 种不同下一步行为合成一类；采用方案 6 类、0 个不健全合并，缺观察不合并。IM-mongo 排队/领取草案中，只看状态的方案漏掉"评价"路径；采用方案 11 次展开即终止并能到达评价；现有 `exploreFlow` 无去重，500 步预算耗尽被截断。
- 不需要用户先做全局业务判断：历史属性由各项目在模型中显式声明（带来源）。IM-mongo 的 `responsible_agent` 只是代码里看到的实现事实（`support_service_cycle.go:53-54`），写入 IM-mongo 模型前由用户确认，归 W06。
- 交接：W02 实现 `history[]` 字段、按 `EK` 去重与 `merges[]`；W08、W09 按运行时规则去重。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (exploreFlow|ExplorationBudget|solveConditions)' --format ai-md --context 2 --max 30
arc-idx search 'f:tools/arc-verify/ -f:_test (TestKind|testFault|modelObligations)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 读对话 L113–L150、L616–L670、L1601–L1616 与现有 `exploreFlow`，写出问题与候选方案（≥2 个），附各自的反例。
- [x] 在一次性测试里实现候选等价键，跑 `academic-v1` 变体与 IM-mongo 草案，记录合并/不合并结果。
- [x] 退出条件：等价键合同写入设计 05「补齐的义务、报告与探索」，W02/W08/W09 卡按它更新；若需要用户做业务判断，列成问题并把依赖卡标 `[!]`。
- [x] 回写本卡与 tracker。

## 验收

```bash
test -s .arc/artifacts/verify-route-alignment/research/state-equivalence.md
grep -n '等价键' docs/01-系统设计/05-业务图谱工作台.md
```

研究卡完成只代表合同已定，不代表实现完成；实现落在 W02/W08/W09。
