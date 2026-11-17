# Arc 业务验证工作台

工作台读取 Go 验证器导出的离线快照，用同一个模型、环境、流程、分支、运行、步骤、主体和实体上下文审阅业务规则与实际证据。模型及账本由 `arc verify` 写入；页面只负责阅读、筛选和定位。

## 四个区域

| 区域                         | 用途                                                                                                                    |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| 能力树与分支导航             | 从用户能力定位条件分支，查看还没有运行的声明；历史测试与未解析运行单独标识。                                            |
| 准备依赖、业务状态、运行画布 | 全模型共享依赖、业务分组计划和分支阻断链；状态图保留合法循环，并叠加准确动作发生范围内的 Actual；运行画布展示记录顺序。 |
| 分支与事实面板               | 查看规则来源、前置条件、冻结预期、请求输入、实际动作响应、独立 DB 回读、条件、实体、会话、等待、断言和恢复记录。        |
| 执行轨迹                     | 在同一次运行内选择步骤、定位首次偏离，并同步节点与已持久化的分支关联。                                                  |

默认从全模型声明图开始，也可限定当前流程或分支。共享节点先展示来源和受影响分支，进入明确分支后再选运行。切换运行会清空旧主体和实体筛选；同一步多个主体使用各自的动作和冻结分支。筛掉中间步骤后，画布会标明省略的步骤，不将剩余节点伪装成相邻操作。

## 全局地图、分组计划与分支观察

“全模型声明 / 当前流程声明 / 当前分支与观察”区分三个阅读范围。前两者将已有分支系谱
投影到共同地图：只有完整声明、规则来源、逻辑操作数及准备前提等价时才共享节点，目标和
前态仍归属于具体分支。每个共享节点和关系保留原分支身份与来源；同名条件不能自动合并。
这些节点始终显示未观察，不汇总不同账号、实体或运行的 Actual。

准备依赖默认使用紧凑分组计划。各业务组可折叠，组间保留同一依赖层级；节点的直接上游和
下游可以点击定位，跨组定位会展开对应业务组。窄屏使用纵向依赖层，另可切换“关系画布”
查看连线。分组来自明确能力归属，跨能力共享条件和准备来源单独成组，不从节点坐标推测业务。

“声明关系最长链”表达声明中的最长必要关系链，不计候选提供者和 OR 替代边。进入一个分支并
选定独立观察后，“前态阻断解释”进一步列出这次观察尚缺的条件、真假方向、依赖路径和候选
准备来源，可直接在图中定位。已满足 OR 的其他假子项不列为必修阻断；NOT 子条件需要为假时，
不会推荐仅能建立 TRUE 的准备动作。证据不足与业务条件不满足分别展示。

## 从一个分支解释完整的准备系谱

选择能力和具体分支后，切换“准备依赖”。新导出中的 `business_lineage` 把允许前态、Guard
逻辑树、属性条件、能够建立条件的声明准备动作及其显式上游连在一起。条件卡片保留主体和
属性，点击后可查看规则来源、候选准备动作以及当前选定运行的实际值和观察来源。
按最长路径分层，并提供上下游、关键依赖链和分层目录；局部筛选会保留范围提示。
从计划或画布选择条件、准备来源时，详情同步定位。图例区分必需关系与候选/OR 替代关系，
同一层并不保证动作可以并行。

`AND`、`OR`、`NOT` 各自保留语义。候选提供者不是已经选定或已经执行的计划；同一条件的
多个提供者属于候选路径。`NOT` 的子条件为假、`OR` 的某一子项为假都不自动等于目标被阻断。
目标是否就绪以整个根条件和准确前态的判定为准。

选定运行、步骤、账号及实体后，只叠加该作用域的 Go 观察结果。同一步同一主体出现多次动作时，
进一步选择“动作发生记录”；不同动作序号不能合并。目标动作的 `before` 来自执行前冻结记录，
执行后的状态另见实际事实和运行视图。准备节点显示效果已观察到，也不能代替真实执行 receipt。
实体按别名、类型、ID、属主和渠道的完整身份匹配；业务主体到实体别名的映射来自冻结 Runner，
不按同名或相同裸 ID 推测。动作后换属主的实体不会覆盖本次动作的前态身份。

执行轨迹中的“前态准备”是单独的 setup 历史。选择它以后，可以在动作发生记录中逐一查看
初始、准备动作前、准备动作后的真实回读。准备成功也不会把目标节点标成已执行或业务 PASS；
准备失败则保留已捕获的条件值和原阻断原因，所有节点保持证据阻断，便于定位未建立的前态。

没有运行的模型也能生成完整声明系谱，显示“未观察”。没有声明提供者的条件仍可由独立观察
满足，不会自动被判为规则缺口。旧快照没有新系谱字段时继续显示原事实，但需重新导出才能
获得条件系谱。字段、状态和资源边界见 [共同契约](../../../docs/02-API记录/arc-verify-snapshot.md)。

有界探索记录使用 `campaign → variant → child run` 导航，并与上述八项上下文同步。选择变体先查看
其候选观察、准备方案与停止原因；只有已经关联且通过证据核验的 child run 才能跳入实际步骤。
派发意图、预算预留和预测效果不会绘制成已经执行的业务操作。账号选项使用实际参与者别名，
不会把 `STUDENT` 等角色摘要误当作另一个账号。

## 在状态图上审阅 Actual

选定运行后，状态图保留原有契约转移，并另外绘制每次动作的实际前态到实际后态。路径来自
同一次动作的冻结 before、真实请求/动作 receipt 和独立 after Oracle；模型外后态保留为
模型外观察，不改选另一个预期。重复动作、自环及并发动作有独立发生身份，相同端点不会合并。

点击 Actual 边或发生记录，会同时定位 run、step、actor、branch、action_index 和 occurrence，
并保留精确实体身份。右侧动作响应、Oracle 与冻结比较限于这一动作；步骤总断言另标为汇总，
不能替代所选发生记录。缺少冻结、请求、独立后态、实体归属或证据一致性时显示诊断；已记录的
PASS 还必须与必要断言及步骤结果自洽。前端只校验展示依据，不重新计算业务谓词或业务分支。

这里的 Actual 是导出时已经发生的历史。没有运行时只展示声明；准备失败不会出现目标动作边；
异步后续事件只有被独立记录后才有实际路径。图上的排列不证明并发提交先后或环境当前状态。

## 构建与前端检查

在本目录执行。包管理器版本由 `package.json` 的 `devEngines` 固定为 Bun 1.4.4，依赖版本使用 `bun.lock`。

```sh
bun install --frozen-lockfile
./node_modules/.bin/vp check
./node_modules/.bin/vp test
bun run build
```

`build` 先执行 TypeScript 检查，再生成 `dist/`。生产构建使用相对资源路径和单个经典 IIFE 脚本，以便随快照离线分发。`dist/index.html` 只是构建模板；业务数据由 Go 导出命令嵌入，不能把未嵌入数据的模板当成有效工作台。

前端开发入口在开发模式下使用 `dev/sample-snapshot.json`，测试还使用明确标为 synthetic 的 `src/test-fixtures.ts`。它们用于工具回归，不代表任何接入项目的真实业务验收。

## 从 Go 账本导出快照

需要已安装的统一 `arc` CLI、可用的内置验证引擎，以及目标项目中已经登记的 v2 模型。执行命令时位于目标项目的 Git 根目录；激活统一账本的项目由 `arc` 路由到该项目的 `arc.db`。导出不执行支付或其他目标业务动作。

将下面的 `GRAPH_ID`、`ENVIRONMENT`、`ROUND_ID` 和绝对路径替换为已有记录：

```sh
arc verify snapshot \
  --graph GRAPH_ID \
  --env ENVIRONMENT \
  --round ROUND_ID \
  --config .arc/verify/config.jsonc \
  --assets /absolute/path/to/Skills/tools/arc-verify/workbench/dist \
  --out /absolute/path/to/review-export
```

输出目录包含 `index.html`、`snapshot.json` 和本地脚本、样式资源。保留整个目录，在允许打开本地文件的浏览器环境中阅读 `index.html`。输出目录必须不存在或为空，导出器不会覆盖旧快照。

未指定 `--run` 时，导出器读取选中图及显式环境、轮次范围内最近 32 次运行，按照账本插入顺序取最新记录。`available_runs` 和 `runs_truncated` 保留总数与截断边界。只审阅指定运行时追加 `--run RUN_A,RUN_B`；运行不属于所选图、环境或轮次会报错。仅审阅模型时可以省略环境和轮次，页面不会把缺少运行解释为已验证。

Go 在导出时重新校验实际运行的 RAW、冻结上下文及统一步骤事实。证据损坏会保留错误与 `BLOCKED` 状态。`snapshot.json` 与页面中 `#arc-snapshot` 的 JSON 来自同一次导出；前端读取内嵌数据，不请求本地 API。

探索记录同样限定图、环境与轮次，最多导出最近 32 个 campaign，并保留 `available_campaigns` 和
`campaigns_truncated`。child run 不在本次导出时明确标缺失，不根据其他运行补齐。损坏 campaign
只显示可核实的索引身份和阻断原因，不补造零预算或成功覆盖。生产读取入口保留 JSON 数字原文，
避免把 `100.0`、`1e2`、`-0` 或 64 位整数静默改写成另一种业务输入。

## 只读和 CSP 边界

导出页面不提供执行、批准、重试或写入账本的按钮。模型修改、规则确认、真实执行和恢复都通过 CLI 完成，然后重新导出快照。快照是导出时刻的记录，不会自动刷新为目标环境的当前状态。

Go 导出器在页面头部插入以下 CSP：

```text
default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; base-uri 'none'; form-action 'none'
```

脚本和样式仅来自快照目录；网络连接沿用 `default-src 'none'`，表单提交被禁止。CSS 内联样式用于图形定位。JSON 以 `application/json` 嵌入，Go 序列化会转义 HTML 特殊字符。复制资源时拒绝符号链接等非普通文件，以及仍含旧 `/api/` 引用的构建。

## 如何解读证据

| 数据                             | 能说明什么                                            | 边界                                                                                          |
| -------------------------------- | ----------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `business_contract`              | 已声明的主体、分支条件、来源、预期与准备动作          | 声明本身不是执行记录；缺失、冲突或未知不能补写为允许。                                        |
| `business_lineage`               | 分支目标、前态、逻辑树、条件及有依据的准备提供关系    | 派生自合同，不是运行；候选提供者和并列图层不代表已选择的实际调度。                            |
| `runs[].business_lineage`        | 独立核验的同运行、步骤、主体、动作序号观察            | 目标 before 不混入 after；证据损坏、来源缺失或选择歧义不能显示满足。                          |
| `contract_analysis`              | 明确有限域内的分支分区证明                            | `PROVEN_IN_SCOPE` 不表示真实系统已经执行，也不表示域外条件已覆盖。                            |
| `contract_execution`             | 按实际冻结分支归属的运行覆盖                          | 同一个测试或节点通过不会自动让其他分支通过。                                                  |
| `runs[].facts`                   | 本次运行、步骤、账号、实体范围内的持久化事实          | 页面拒绝跨运行或跨模型事实；不根据节点相同推测身份关联。                                      |
| `verdict` 与 `evidence_status`   | 业务结果与证据可信状态                                | 二者分别展示；记载 PASS 但证据 BLOCKED/LEGACY 不能算强验证通过。                              |
| `requests`、`actions`、`oracles` | 冻结前的实际请求捕获、动作响应、独立前后回读          | 响应值不能替代独立 DB 事实；缺失值保留未知。                                                  |
| `preparation`                    | 计划历史、实际准备步骤、独立 readbacks 和目标就绪状态 | `predicted_effects` 是预计效果；实际是否执行和是否满足条件以捕获及回读为准。                  |
| `recovery.mode=checkpoint`       | 只读确认并保留运行现场                                | 不表示已回滚或恢复；旧记录缺少 mode 时不能推断为 checkpoint。跨运行自动继承变量仍需单独实现。 |
| `business_campaigns`             | 指定输入变体的观察、派发、child run、预算和恢复历史   | `COMPLETED_IN_SCOPE` 仅表示所选变体通过，`full_business_coverage` 仍为 false。                |
| `campaign.last_actual`           | 最近目标动作的实际后态                                | 后续 restore 可能改变现场；恢复观察单独展示，下次候选必须重新读取实际状态。                   |

前端只展示 Go 已持久化的匹配与比较结果，不在浏览器重新选择业务分支或重算证据判定。历史数据继续可读，但没有新的冻结身份、独立判据或完整 RAW 时保持 `LEGACY` 或 `BLOCKED`。

## 维护与贯通验证

- `selection.ts`、`explorer-model.ts`：统一选择上下文与显式身份映射。
- `layout.ts`、`graph-model.ts`、`GraphCanvas.tsx`：准备 DAG、合法状态环和实际运行轨迹。
- `lineage-map.ts`、`DependencyPlan.tsx`、`DependencyInspector.tsx`：全局共享声明、业务分组与来源/影响追踪。
- `lineage-blockers.ts`：按所选观察解释 AND/OR/NOT 的实际阻断义务及候选准备来源。
- `state-overlay.ts`、`state-overlay-evidence.ts`：独立 Actual 状态路径、证据自洽与准确发生身份。
- `lineage-types.ts`、`lineage.ts`、`LineagePanel.tsx`：共同系谱协议、准确观察选择及条件/来源细节。
- `lineage-validation.ts`、`lineage-schema.ts`、`lineage-evidence.ts`：结构、身份、原始记录与派生观察的一致性；不重新计算业务预期。
- `FactPanel.tsx`、`contract.ts`、`facts.ts`：声明、冻结预期、实际结果与来源。
- `CampaignViews.tsx`、`campaign.ts`、`campaign-validation.ts`：有界探索的身份、历史与选择边界。
- `snapshot.ts`、`types.ts`、`json-values.ts`：导出 DTO、原始数字和可见的读取错误边界。
- `../snapshot.go`、`../business_lineage*.go`、`../facts_integrity.go`：Go 导出、统一系谱投影与证据重验。

修改 DTO 后，在 `tools/arc-verify` 中运行 `go test -run '^TestSnapshotTypesMatchFrontend$' -count=1`，并把真实 Go 工具测试执行产生的快照交给 `parseSnapshot` 与 React 服务端渲染做贯通检查。前端合成 fixture、Go 的隔离 HTTP 工具测试、接入项目 E2E 必须分别标明范围。

永久集成测试可以直接读取实际导出。先在 `tools/arc-verify` 中运行以下可选工具测试，目录应为全新的绝对路径：

```sh
ARC_WORKBENCH_EXPORT_DIR=/absolute/prepared-export \
ARC_CAMPAIGN_EXPORT_DIR=/absolute/campaign-export \
ARC_PREPARATION_BLOCKED_EXPORT_DIR=/absolute/blocked-preparation-export \
ARC_WORKBENCH_ASSETS=/absolute/Skills/tools/arc-verify/workbench/dist \
go test -run '^(TestWorkbenchExportObservedToolRun|TestWorkbenchExportObservedBusinessCampaign|TestBusinessLineagePreparationFailureRemainsDiagnosticWithoutTargetFreeze)$' -count=1 -v .
```

随后在本目录运行：

```sh
ARC_WORKBENCH_SNAPSHOT_PATHS='["/absolute/prepared-export","/absolute/campaign-export","/absolute/blocked-preparation-export"]' \
  ./node_modules/.bin/vp test run src/go-export.integration.test.tsx src/state-overlay.export.test.ts
```

单个导出也可使用 `ARC_WORKBENCH_SNAPSHOT_PATH=/absolute/export/snapshot.json`；两种变量不要同时设置。
未设置时明确 SKIP，不用合成数据替代。测试使用生产 JSON 解析器与 SSR 检查实际前态、准备、
冻结预期、请求捕获、账号实体选择和 child 关联，每个目录写入 `frontend-validation.json` 与
`ssr-verified.html`，并核对导出资源与当前 dist 是否一致。测试失败会覆盖旧成功报告。
这一步只审阅已有导出，不重新执行目标动作，也不代替 Go 对 RAW 的重验。

静态检查、状态测试、SSR 和构建通过并不等于实际浏览器布局与交互已验收。浏览器环境受限时记录 `BLOCKED`，保留已有验证结果，不绕过环境策略，也不补造截图。


## Source-first genealogy (v2)

The exporter now draws independent `input_data` nodes for each typed attribute used by a leaf guard, links them with `reads` edges, and exports every contract outcome as `expected` connected with `expects`. The contract hash still binds the same approved business definition. Version 1 snapshots remain readable; only version 2 snapshots require the complete field/outcome projection. Re-export a snapshot to see new nodes.

The precondition ledger deliberately separates **before** data (frozen independent readings) and **after** assertions (required Oracle checks). No before value is treated as evidence that an outcome occurred. Candidate preparation edges are suggestions backed by declared effects; they are never a preparation receipt. An unknown source is shown as unknown, never PASS.

For verification use `go test ./...` under `tools/arc-verify`, then `./node_modules/.bin/vp test` and `bun run build` under `workbench`. These commands must actually execute before claiming green. Finally inspect a generated business snapshot in a real browser at narrow and wide viewport. Static SSR is not visual acceptance.
