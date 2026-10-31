# W19 同类系统参照库研究

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 研究 |
| 优先级 | P2，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D4 |
| 难度依据 | 参照规则最容易把别人的业务强加到当前系统（PRD 外部研究核对）。来源是否可信、适用条件怎么表达、怎样避免噪声都没定，先研究。 |
| 目标面 | 文档与一次性原型；不改正式源码 |
| 前置依赖 | 无（可与 W07、W12 并行） |
| 下游影响 | W17 三向比对、W12 的 `peer_reference` 来源 |

## 目标与范围

需求：[BG28](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「来源分离、反证与自测」；外部依据见 PRD「外部研究核对与借鉴」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

要回答的问题：

1. 哪些来源可以进参照库？候选包括公开规范、用户其他项目里已确认的 RULE（如 IM-mongo）、成熟开源系统的业务约束。各自的可信度与引用方式是什么？
2. 适用条件怎么写，才能在条件不满足时不生成候选（例如"仅当存在容量概念"）？
3. 怎么压噪声？参考 LogicScan 的聚合去噪思路，但当前目标是企业系统而不是智能合约。
4. 参照条目和 RISK 怎样关联，才能追溯回来源？

证据产物：`.arc/artifacts/verify-route-alignment/research/reference-catalog.md`，至少包括报名、IM 消息两个领域各 5 条草案条目（带来源和适用条件），以及在 `academic-v1` 上的匹配演示。

不做：不批量抓取外部系统代码；不把参照条目直接写成 RULE；需要用户做业务判断的，列成问题。

## 2026-10-31 结论

采用"类型化槽位 + 关系约束"格式，加模型 `reference_bindings` 覆盖；来源分四级，缺来源拒收。合同写入设计 05"参照库"行，研究记录 `.arc/artifacts/verify-route-alignment/research/reference-catalog.md`（含报名、IM 各 5 条带来源的草案条目）。

- 原型（一次性测试，已删除，源码与输出存档）：`academic-v1` 生成 3 条 RISK 候选（容量并发、重复报名、活动关闭），7 条无绑定；course-v2 参考模型因状态名"报名成功"未命中提示词而全部无绑定——同义词漏判的实例，所以"找不到绑定"必须报 `NO_BINDING` 而不是"不适用"，并支持显式绑定。
- 实现卡：[W24](W24-reference-matching.md)。不需要用户先做业务判断；`project_code` 级条目默认允许并标最低级别。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (open_risks|modelGate|accepted)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 读 LogicScan 摘要和 PRD「外部研究核对与借鉴」，写出 ≥2 个参照库格式方案及各自的反例。
- [x] 做两个领域的草案条目，在 `academic-v1` 上演示匹配和不匹配。
- [x] 退出条件：参照库格式与匹配合同写入设计 05「来源分离、反证与自测」，并新建实现卡；需要用户判断的问题标 `[!]`。
- [x] 回写本卡与 tracker。

## 验收

```bash
test -s .arc/artifacts/verify-route-alignment/research/reference-catalog.md
grep -n '参照库' docs/01-系统设计/05-业务图谱工作台.md
```

研究卡完成只代表合同已定；实现另起卡。
