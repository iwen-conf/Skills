# W07 补齐 EXISTENCE / SEQUENCE / CONSISTENCY 义务

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P1，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D3 |
| 难度依据 | 新义务必须和模型校验、完整分母、gate 同时生效，否则会出现"有义务却不计入分母"的漏判；涉及 `model_v2.go` 校验分支、`obligations.go`、`model_tasks.go`、`model_analysis.go` 多个调用点。三者共享同一个不变量（声明的义务不能消失），不拆。 |
| 目标面 | 本仓工作树、本地 Go 与原生 Turso（`tursogo`）；不碰远程业务数据 |
| 前置依赖 | [W01](W01-model.md)：v2 模型可导入、读回、hash 稳定（W01 验收命令通过） |
| 下游影响 | W02 完整分母、W09（EXISTENCE 与 scan 证据共用）、W13 快照、W14 文档 |

## 目标与范围

需求：[BG19](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「补齐的义务、报告与探索」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

范围：`tools/arc-verify/model_v2.go`（`TestKind` 常量与校验分支）、`obligations.go`（`modelObligations`）、`model_tasks.go`（`planModel`）、`model_analysis.go`（覆盖统计）、`extra.go`（`scan`/`gaps` 证据读取），新增测试放在 `model_v2_test.go` 或新 `obligation_kinds_test.go`。

- EXISTENCE：规则或能力声明的必须存在入口 → 一条义务；当前候选上的 `scan` PRESENT 证据满足它；NOT_FOUND 记候选缺口，状态不是 FAIL。
- SEQUENCE：引用一条已声明 flow 的有序流转 → 一条义务；逐步判定，第一个偏离步就是该义务的首个偏离。
- CONSISTENCY：同一动作声明 ≥2 个观察层（UI/API/数据库/异步）及各自的 Oracle → 一条义务；任一层缺观察为 BLOCKED。
- 三类义务全部进入完整分母和 gate；v1 图与旧 hash 不变。

不做：不自动判 MISSING（需扩大检索并经用户确认）；不做 DISCOVERY（属 W09）；不改 v1 图的 hash 与旧任务 ID；不为凑覆盖率把缺观察记成 PASS。

## 2026-10-31 实施记录

声明方式：三类义务都写在 v2 模型 `tests[]` 里，用新的 `kind` 值；旧图不声明就不生成，义务集合与 round 里存的 `required_sha256` 不变。

| 种类 | 声明字段 | 判定 |
| --- | --- | --- |
| `EXISTENCE` | `rule`；不得带 `preconditions`/`expected`/`denied`/`actors`；每条规则最多一条 | 不走 runner（注册直接拒绝）。读该规则动作最新一次 `scan`：无 scan → NOT_RUN；NOT_FOUND → BLOCKED「candidate gap」；UNKNOWN → BLOCKED；PRESENT 时依次要求证据文件 hash 未变（否则 STALE）、scan 带 `commit`（否则 BLOCKED）、所选 round 当前有效（否则 STALE）、`commit` 等于 round 的候选提交（否则 STALE），全满足才 PASS |
| `SEQUENCE` | `flow`、`steps`（2–64 条 `transitions_to` 边，从 flow 初始状态起首尾相接，动作属于 flow 能力）、`rule` 必须是最后一步的规则、`expected`/`denied` 必须与最后一步一致；不带 `preconditions` | runner 每步一个批次且顺序与 `steps` 相同；每步按该步 transition 的规则取 Oracle、条件和拒绝标志；执行器遇第一个非 PASS 步即停，trace 的 `first_divergence` 就是该步；批准要求每一步的规则都已批准 |
| `CONSISTENCY` | 普通测试字段 + `layers`（2–4 个，层取 `ui`/`api`/`database`/`async`，层和 Oracle 都不重复） | runner 最后一步必须挂上每层的 Oracle（注册时校验）；最后一步各层读回状态都等于期望才 PASS，某层不一致 → FAIL，某层读不到 → BLOCKED |

同时补上 `NodeReadiness.Implemented`/`ImplementationStatus`：动作取最新 scan 状态，规则跟随其动作，能力取所有实现动作中最弱的状态（NOT_FOUND < UNKNOWN < UNOBSERVED < PRESENT），只有全部动作都通过 EXISTENCE 同样的绑定检查才算 implemented。`scan` 输入新增可选 `commit`（40/64 位小写十六进制），存进追加表 `implementation_scan_candidates`；证据路径改存绝对路径。

范围变化：除卡片列出的文件外，还改了 `model_index.go`（按边索引 transition、按 ID 索引 flow）、`runner.go`（注册校验）、`execute.go`、`model_proof.go`（逐步规则与层比对）、`db.go`（追加表）、`engine.go`（`TaskCard` 增加 `steps`/`layers`）。原因：SEQUENCE 与 CONSISTENCY 要真实执行才能判定，只改义务生成会出现"有义务却永远跑不了"。非 SEQUENCE 义务的 `stepRule` 返回原规则，行为不变。

验收结果：

- `go -C tools/arc-verify test -race -count=1 -run 'Existence|Sequence|Consistency|NewKinds|V1Hash' ./...`：PASS（新增 9 个测试含子用例）。
- `go -C tools/arc-verify test -race -count=1 ./...`：182 项 PASS；`go vet`：PASS。
- red：`.arc/artifacts/verify-obligation-kinds/red/red-001.json`（断言失败）。
- 种下缺陷：12 个全部被抓到，见 `.arc/artifacts/verify-obligation-kinds/sensitivity.md`（首轮"SEQUENCE 只查末步规则批准"漏网，补了 `TestSequenceNeedsEveryStepRuleApproved`）。
- 门禁：`gate.sh check --strict --require-red --scope`，0 FAIL；`red-hygiene` WARN 是因为红灯后改过测试：把"边节点的 Required"断言（边本来就没有节点状态，断言写错）换成 SEQUENCE 目标必须覆盖两步、两条规则和 flow，外加新测试 `TestSequenceNeedsEveryStepRuleApproved`，没有放松任何断言。报告 `.arc/artifacts/verify-obligation-kinds/reports/gate-20261031T074555Z.txt`。
- 真实 CLI：`validate → init → import → scan（拒绝大写短提交号）→ gaps → gate → view`，见 `.arc/artifacts/verify-obligation-kinds/real-entry/cli.log`；gate 的完整分母 16 条含三类新义务。

未验证：CLI 的人工 `approve` 只能由用户在真实终端执行，批准后的 plan/next/execute 只在 Go 测试里经 `d.approve` 走过；IM-mongo 真实业务未跑（W06）；NOT_FOUND → MISSING 的用户确认流程按「不做」未实现。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (TestKind|testFault|modelObligations)' --format ai-md --context 2 --max 30
arc-idx search 'f:tools/arc-verify/ -f:_test (NOT_FOUND|implementation_scans)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 先写失败测试并保存 red：`TestExistenceObligationNeedsScanEvidence`、`TestExistenceNotFoundStaysCandidate`、`TestSequenceObligationKeepsFirstDivergence`、`TestConsistencyMissingLayerBlocked`、`TestNewKindsCountedInDenominator`、`TestV1HashUnchangedWithNewKinds`。
- [x] 加 `TestKind` 常量与校验；义务生成、规划、分析、gate 一起改。
- [x] 跑验收命令；回写实际文件、命令/结果、未验范围，同步 tracker。

## 验收

```bash
go -C tools/arc-verify test -race -count=1 -run 'Existence|Sequence|Consistency|NewKinds|V1Hash' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行（不是 Rust，允许本机编译）。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
