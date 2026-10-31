# W09 未知业务探索：DISCOVERY 与 UNSPECIFIED

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P2，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D3 |
| 难度依据 | 契约外行为不能判 PASS/FAIL，又要可追溯、可分流；与 `scan`/`gaps`、风险报告、等价去重三处交互，分流结果还会改变 RULE/RISK 集合。 |
| 目标面 | 本仓工作树、本地 Go 与原生 Turso（`tursogo`）；不碰远程业务数据；真实探索只在已授权隔离目标（IM-mongo `.60`）由 runner 执行 |
| 前置依赖 | [W03](W03-execution.md)：runner 可写只读探索观察；[W07](W07-obligation-kinds.md)：EXISTENCE 与 scan 证据合同；[W10](W10-state-equivalence.md)：等价键 |
| 下游影响 | W12 风险报告、W13 快照、W14 文档；分流决策交 `arc-plan` |

## 目标与范围

需求：[BG24](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「补齐的义务、报告与探索」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

范围：`tools/arc-verify/extra.go`（`scan`/`gaps`）、`db.go`（追加 UNSPECIFIED 候选表）、`handoff.go`（分流包交 `arc-plan`），新增 `discovery.go`、`discovery_test.go`；CLI 增加 `discover` 子命令组：`discover record`（导入探索观察）、`discover list`、`discover triage`（交互确认，分流为遗漏需求/允许扩展/不应存在）。

- 探索观察带入口、主体、动作/状态、证据路径与 SHA-256；与契约比对后，契约外的记 UNSPECIFIED。
- 用等价键去重（设计 05「状态等价」运行时规则；未声明差异生成 `UNDECLARED_DIFFERENCE` 风险候选）；每次探索有条目上限。
- `triage` 要交互式确认（同 `approve` 的人工终端要求）；"遗漏需求"转 CANDIDATE 规则，"不应存在"转 RISK，"允许扩展"记录理由后排除。

不做：不判 PASS/FAIL；不自动把 UNSPECIFIED 变成 RULE；CLI 不驱动浏览器（探索动作由已有浏览器/设备能力执行，CLI 只登记）；不写未授权目标。

## 2026-10-31 选型输入（W22）

按[选型记录](../../10-复盘决策/01-选型-API状态化探索.md)：DISCOVERY 主路径模型驱动自建，用 runner 的 HTTP 适配器与独立 Oracle，在声明状态里尝试已声明动作（含被禁止的上下文），被接受但模型未声明的结果记 UNSPECIFIED；借鉴 Schemathesis 的 links 串操作与逐请求记录、CATS 的状态化检查清单。Schemathesis 只作可选命令适配器，IM-mongo 目前没有 OpenAPI，不能用它。

## 2026-10-31 实施记录

命令：`arc-verify discover record --file exploration.json [--budget N]`、`discover list --graph G`、`discover triage --graph G --id ITEM --as missing_requirement|allowed_extension|should_not_exist`（交互终端，需输入 `TRIAGE <id> <决定>` 和用户原话；管道一律拒绝）。

- 探索记录 JSON：`graph`、`exploration`、`observations[]`（`entry role principal action before_state after_state denied conditions measurements evidence`）；证据文件逐条做 SHA-256。CLI 只登记，不驱动浏览器或接口。
- 和模型比对的四类契约外行为：`UNDECLARED_ACTION`（动作不在模型）、`UNDECLARED_STATE`（前后状态不在模型）、`UNDECLARED_TRANSITION`（被接受却没有对应的已声明转移、声明的前提没被满足，或被拒绝却改了状态）、`UNDECLARED_DIFFERENCE`（与已有条目等价键相同、只差未声明的 measurement）。被模型声明的行为只计数（`within_contract`）。
- UNSPECIFIED 不判 PASS/FAIL，不进验收分母（`TestDiscoveryNeverPassOrFail`）。
- 去重：条目 ID = 类别 + 动作 + 观察后状态 + 运行时等价键（W10/W08 的 `runtimeKey`）；同一等价状态再登记只追加来源。每次登记最多 N 条新条目（默认 16，1–1024），超出标 `truncated`。
- 分流只记一次（带用户原话，守卫写入）："遗漏需求"进入 `handoff --to arc-plan` 的 `candidate_requirements`，冻结的图版本不改规则（arc-plan 出新版本）；"不应存在"进入 `report --kind risk` 的 `unspecified_risks`；"允许扩展"记理由后排除。
- 新文件 `discovery.go`、`discovery_test.go`；`db.go` 追加 `unspecified_items`、`unspecified_sources`、`unspecified_triage`；`handoff.go`、`risk_report.go`、`main.go`（`discover` 子命令组、`--as`；交互拒绝提示改成通用措辞）。

偏差：卡片写"遗漏需求转 CANDIDATE 规则"——直接写进规则表会改动已冻结的图版本，改为交给 arc-plan 的候选需求，由 arc-plan 产出新版本模型。

验收结果：

- `go -C tools/arc-verify test -race -count=1 ./...`：PASS；`go vet`：PASS。
- red：`.arc/artifacts/verify-discovery/red/red-001.json`。
- 门禁 0 FAIL（`.arc/artifacts/verify-discovery/reports/gate-20261031T110526Z.txt`）；WARN：红灯前 36 行是类型与桩，红灯后测试只加严（新增"被拒却改状态"用例）。
- 种下缺陷 12 个：11 个抓到；"重复分流"去掉显式检查后仍被守卫写入拦下（要求恰好写入一行），属纵深防御。首轮"被拒却改状态"漏网，补了用例。
- 实现中发现：Turso 的 SQL 解析把 NUL 当字面量结束，存储的身份串改用可见分隔符。

未验证：真实探索（IM-mongo `.60`）由已有浏览器/设备能力执行后再用 `discover record` 登记，归 W06；Schemathesis 适配器未接入（W22 结论：可选，IM-mongo 无 OpenAPI）。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (NOT_FOUND|implementation_scans)' --format ai-md --context 2 --max 30
arc-idx search 'f:tools/arc-verify/ -f:_test (ExecuteRequest|RunnerManifest|restoreFixture)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 先写失败测试：`TestDiscoveryOutsideContractIsUnspecified`、`TestDiscoveryNeverPassOrFail`、`TestDiscoveryDedupByEquivalenceKey`、`TestTriageRequiresInteractiveConfirmation`、`TestTriageMissingRequirementBecomesCandidate`，保存 red。
- [x] 实现候选表、`discover` 子命令与分流交接。
- [x] 跑验收命令；回写并同步 tracker。

## 验收

```bash
go -C tools/arc-verify test -race -count=1 -run 'Discovery|Triage' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行（不是 Rust，允许本机编译）。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
