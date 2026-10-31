# W08 任务卡自动繁殖（派生验证义务）

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P2，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D3 |
| 难度依据 | 派生义务会改变完整分母；需要去重（依赖 W10 等价键）、数量预算、来源区分（RULE 才进验收，推断只进风险探索），且不能改原任务含义或原 FAIL。 |
| 目标面 | 本仓工作树、本地 Go 与原生 Turso（`tursogo`）；不碰远程业务数据 |
| 前置依赖 | [W03](W03-execution.md)：真实执行写出的 step 观察与首个偏离可读；[W10](W10-state-equivalence.md)：等价键合同已定稿 |
| 下游影响 | W09、W11、W13 快照中的派生关系、W14 文档 |

## 目标与范围

需求：[BG23](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「补齐的义务、报告与探索」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

范围：`tools/arc-verify/track.go`（step 判定与 REPLAN/RESET/HALT 决策）、`engine.go`（`plan`）、`obligations.go`、`model_tasks.go`、`db.go`（追加派生义务表与索引，幂等迁移），新增 `derived.go` 与测试。

- 观察到偏离或异常时，按规则模板派生义务，记录来源 run/step、触发观察和模板 ID，挂在原节点。
- 运行时去重需要的 v2 字段 `equivalence.ignore`（measurement 名、原因、来源）在本卡加入；`equivalenceKey`（`equivalence.go`）已能吃 measurement。
- 用等价键去重（设计 05「状态等价」运行时规则：`EK` + 全部声明 measurement，`equivalence.ignore` 除外）；每个来源 run 有派生数量上限，超限记 `truncated`；按风险排序输出。
- 来源是已批准 RULE → 进入验收分母；来源是推断 → 记为 RISK 探索任务，不进分母。

不做：不改原任务冻结含义、原 run 判定或首个偏离；不无上限递归派生；不让推断来源的派生义务变成验收义务。

## 2026-10-31 实施记录

命令：`arc-verify derive --run RUN [--budget N]`（N 为 1–256，默认 16）。只接受 FAIL 的模型运行，并且它的轮次仍然有效（轮次过期时派生会拒绝，因为证据上下文已变）。

- 读首个偏离步里被判定 Oracle 的独立读数（动作后状态；动作没执行时取动作前）。
- 模板 `FROM_OBSERVED_STATE`（来源 RULE，进验收分母）：观察到的状态是模型已知状态时，对每条从它出发、规则已批准的转移派生一条义务："基础义务 `transition/<边>` 必须从等价的世界状态再验证一次"。满足条件：本轮里该基础义务有一次合格 PASS，且第一步动作前的独立读数与派生时的等价键相同；同起点的 FAIL 记 FAIL；证据变了记 STALE。
- 模板 `UNDECLARED_STATE`（来源推断）：观察到模型里没有的状态时记一条探索项，只出现在 `report --kind risk` 的 `derived_risks`，不进分母。
- 去重：派生 ID = 模板 + 基础义务 + 运行时等价键（设计 05 的运行时规则，排除 `equivalence.ignore` 的 measurement）的 hash；同一等价状态再次派生只追加来源（`derivation_sources`），重复派生同一运行是幂等的。等价键无法判定时（缺观察）以来源运行区分，不合并。
- 预算与排序：每个来源运行最多派生 N 条，超出的写进 `skipped` 并标 `truncated`；按风险分（推断 2、规则 1）降序再按 ID 排。
- v2 新增可选字段 `equivalence.ignore`（measurement、原因、user/document 来源，校验唯一性），v1 拒绝。
- 新文件 `derived.go`、`derived_test.go`；`db.go` 追加 `derived_obligations`、`derivation_sources`、`derivation_runs`；`model_analysis.go` 把规则派生义务并入完整分母；`risk_report.go` 增加 `derived_risks`；`main.go` 增加 `derive`、`--budget`；`types.ts` 同步模型字段 `equivalence`（字段比对测试当场发现了缺口）。

验收结果：

- `go -C tools/arc-verify test -race -count=1 ./...`：PASS；`go vet`：PASS。
- red：`.arc/artifacts/verify-derived-obligations/red/red-001.json`。
- 测试：来源链接（经 CLI `derive`）、按等价键去重与幂等、预算截断、等价起点满足、非等价起点不满足、证据改动后 STALE、推断项不进分母且出现在风险报告、原 FAIL/首个偏离/冻结任务不变、PASS 运行拒绝派生、未批准规则不派生、`equivalence.ignore` 的校验与生效。
- 门禁（`reports/gate-20261031T104947Z.txt`）：Go 部分、scope、red-first PASS；`lint(ts)`/`tests(js)` FAIL 是门禁用 npx 调用、被 bun 工程拒绝（`EBADDEVENGINES`），本卡在前端只给 `types.ts` 加了一个字段，`vp check` 无警告、`vp test` 17 项通过。red-hygiene WARN：`derived.go` 红灯前 31 行是类型与常量；红灯后测试只新增用例并把一处直接调用改为经 CLI 调用。
- 种下缺陷 14 个全部抓到；首轮"任意起点都算满足""不校验合格性""未批准规则也派生""ignore 不校验/不生效"漏网，逐一补了测试。

偏差：卡片写的"挂在原节点"按派生义务的目标落实——目标是基础义务的全部目标加上起始状态，所以它出现在规则、转移、状态等节点的必需项里；`run-chain` 停机后不自动派生，派生是显式的 `derive` 命令（停机包里有 `run`，可以直接接着派生）。

未验证：IM-mongo 真实运行上的派生归 W06。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (FirstDivergence|first_divergence|REPLAN)' --format ai-md --context 2 --max 30
arc-idx search 'f:tools/arc-verify/ -f:_test (TestKind|testFault|modelObligations)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 先写失败测试：`TestDerivedObligationLinksSourceStep`、`TestDerivedObligationDedupByEquivalenceKey`、`TestDerivedObligationBudgetTruncates`、`TestInferredDerivationStaysRisk`、`TestOriginalFailUnchangedAfterDerivation`，保存 red。
- [x] 实现派生表、派生逻辑与规划接入；`init` 幂等迁移。
- [x] 跑验收命令；回写并同步 tracker。

## 验收

```bash
go -C tools/arc-verify test -race -count=1 -run 'Derived|InferredDerivation|OriginalFail' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行（不是 Rust，允许本机编译）。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
