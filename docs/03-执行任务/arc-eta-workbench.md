# Arc ETA 工作台完善

## Task Brief

- Mode / Tier：implement + frontend / T2；任务 `arc-eta-workbench`。
- 基线：`8a673702ca890feb5e8ecdb11e8bb53025bbc5eb`；用户授权直接修改并交付 `main`，开始时工作树干净。
- 目标：跨分支共享依赖、业务分组与窄屏计划、作用域限定的阻断解释、Actual 状态路径叠加。
- 用户流程：全局业务地图 → 共享条件/准备来源 → 明确分支 → 一次运行/账号/实体 → 阻断或真实状态路径 → 原始事实。
- 平台：现有离线 React / TypeScript / ReactFlow 工作台；桌面分组计划与窄屏纵向步骤；保留只读边界。
- 数据边界：沿用已有快照、冻结契约、RAW 校验和历史事实；全局共享只合并声明，不合并跨分支 Actual。
- 验收：正式入口反例、前端全量/类型/构建、生产 Go 导出贯通、CLI 安装/smoke、适用 Go 回归、可用的实际浏览器检查。
- 环境边界：真实支付 E2E 仍需已授权隔离环境；工具回归、布局算法或 SSR 不代替产品及浏览器验收。

## 顺序与职责

1. [x] 共享依赖与运行投影：严格共享身份、完整来源、Actual 与 Expected 独立；反例先行。
2. [x] 分组阅读与阻断定位：可折叠业务组、依赖导航、纵向步骤、统一选择与证据定位。
3. [x] 实际导出、独立审查与发布：命令、产物摘要、未验收范围和最终提交一致；发布身份以 Git ref 回执为准。

## ETA 借鉴依据与边界

审阅 ETA 版本 `1a1109984c5819cc30c0783949e7b01f32d2e8f9` 的
[`task-plan.ts`](https://github.com/0xfe-project/eta/blob/1a1109984c5819cc30c0783949e7b01f32d2e8f9/apps/web/src/components/workspace/task-plan.ts)、
[`task-plan-view.tsx`](https://github.com/0xfe-project/eta/blob/1a1109984c5819cc30c0783949e7b01f32d2e8f9/apps/web/src/components/workspace/task-plan-view.tsx) 和
[`task-run-log.tsx`](https://github.com/0xfe-project/eta/blob/1a1109984c5819cc30c0783949e7b01f32d2e8f9/apps/web/src/components/workspace/task-run-log.tsx)。
吸收依赖层级、分组阅读、宽/窄视图和历史定位方法；Arc 的 Guard、冻结预期和独立后态继续由
业务契约与执行事实确定。当前工作台为不可变离线快照，不声称提供 ETA 的在线日志订阅或调度。

## 影响核对

首次搜索结果保存在 `.arc/artifacts/arc-eta-workbench/reports/impact-before.json`。

| 面 | 命中与裁决 |
|---|---|
| 定义 | `graphFor / CanvasGraph / reduceSelection / Selection`：2 文件；只扩展工作台内部投影与选择状态 |
| 调用 | `graphFor / reduceSelection / GraphCanvas`：12 文件；App 接线、画布和相关入口测试 |
| 构造 | `initialSelection / CanvasGraph / Partial<Selection>`：14 文件；默认值兼容，新增选择需原子校验 |
| 读取 | node/branch/canvas/occurrence：7 文件；全局图只读声明，单分支及 Actual 保留完整发生作用域 |
| 契约 | 快照契约、business-diagram：2 文件；不改变已有持久化 JSON 和 DB 表，不增加新的业务判定器 |
| 配置 | 既有 ReactFlow / Vite+ 工具链：1 文件；不增加运行依赖或关闭包管理器约束 |
| 测试 | 现有 15 个前端测试文件；新增拥有测试放在 `workbench/src`，Go 贯通仍用正式导出入口 |
| 文档 | 工作台 README、工具 README、业务绘图约定：3 文件；描述实际实现，不把未运行内容写成通过 |

独立复核后扩展的必要范围：`snapshot.ts` 提前拒绝损坏的身份/比较结构；`FactPanel.tsx` 复用
Actual 路径的完整实体关联，避免同主体并发动作混入右栏；这些修复由正式入口反例证明。

公共 `arc` 预检确认版本 0.5.0 及三个引擎可用。索引一次修复因现有 HOME 缓存盘
`no space left on device` 失败，记录 `fallback:rg`；不为检索修改仓库配置或重复安装工具。

## 实现语义

- 全局地图按模型声明合并具备相同完整语义和来源的条件/准备定义，保留每条原分支节点及边的来源；显示名和裸实体 ID 不构成等价依据。
- 选共享节点先显示受影响分支，用户选择明确分支后才能进入其运行和观察；全局图不得累计或借用某次运行的绿色状态。
- 阻断解释只消费当前发生记录中后端已判定的观察；区分条件为假、未观察和证据阻断。OR 的失败兄弟不是全部必修条件，NOT 保留所需真假方向。
- 状态图保留契约边，Actual 单独来自同一次动作的真实 before/after；模型外状态和偏离单独显示，不改变冻结预期。
- 缺证据、缺后态、未冻结、主体或实体作用域冲突只显示诊断，不用 Expected 或相邻步骤填空。
- 图上关系深度不等于可并行的执行批次；时间、资源和锁竞争没有证据时不作调度承诺。
- 记录的 PASS 必须与冻结结果义务、状态/拒绝标记、逐项断言及共同不变量完整一致；不得凭一个
  状态字符串点绿。契约允许集合中的未命中备选仍可为 FALSE/UNKNOWN，不重算业务谓词。
- 依赖布局采用迭代 SCC 与拓扑遍历，分组计划按层一次分桶；保留合法状态自环，候选边不进入
  必需关键链。65,536 节点长链回归仅证明算法边界，不代表浏览器在同规模下已验收。

## 验证与未完成边界

### 本轮实际导出

通过现有 Go owning tests 执行独立 HTTP 工具目标，导出全新目录；pay_gateway 则经安装后的
公共 `arc` 导入真实来源模型，只生成未批准、零运行的声明快照。快照与页面内嵌 JSON 一致，
本轮 `frontend-validation.json` 逐一核对所有原节点/边来源、分组、Actual 发生身份和构建资源。

| 导出范围 | SSR 视图 | 原分支节点 → 共享投影 | 共享节点 | 目标 Actual |
|---|---:|---:|---:|---:|
| 工具前态准备与执行 | 11 | 12 → 12 | 0 | 1 |
| 工具有界探索和连续实体 | 38 | 20 → 16 | 4 | 3 |
| 工具准备失败诊断 | 9 | 10 → 10 | 0 | 0 |
| pay_gateway 模型声明 | 13 | 60 → 44 | 8 | 0 |

共享图的节点减少来自等价声明和来源合并，不是删去条件。四份导出共 71 个 SSR 视图，
14 个独立条件观察范围；4 次工具目标转移和 3 次精确实体选择有单独断言。模型声明和工具
运行均不能替代 pay_gateway 的真实支付执行。

### 正式入口与反证

- 前端全量：30 文件、255 测试通过，无跳过；命令 `vp test run` 显式提供四份新导出路径。
- `vp check`：格式、Lint、TypeScript 全部通过，零警告。生产构建通过；单个离线 IIFE 资源
  约 576 kB，保留构建器大于 500 kB 的体积提示，没有关闭检查或改变离线 CSP。
- 官方入口 `readSnapshotText → graphFor / reduceSelection → App` 覆盖跨分支来源、OR/NOT、
  共享等价、全局/分支选择、Actual 偏离、身份延续、同名 Oracle、同实体多动作、损坏比较和
  图/事实面板同步。长链 65,536 节点和 96 个有环样本校验布局算法，不构成浏览器性能验收。
- 独立审查追加的 red018–023 均已通过：空/矛盾检查、步骤阻断、实体别名碰撞、实体与动作
  混展示、前后主体/流程改变、图已阻断而右栏仍绿、全局分支适用范围等问题已有正式反例。
- red024 发现并修正 `next_allowed: null` 的兼容回归，继续区别未声明、显式空集合和具体列表。
  原有 allowed-outcomes 展示测试原先只填比较标签，未冻结其声称的结果集合；本轮补全合法
  声明、冻结、RAW 和实际 receipt，保留全部原断言，新增 Actual PASS 断言，未放宽判定。
- CLI 三引擎安装、公共 `arc` smoke、`tools/arc` 与 `tools/arc-idx` 全量测试通过；
  `tools/arc-verify` 的 `go vet` 通过。完整 `go test -race -count=1 -timeout 20m ./...` 通过，
  验证器包实际用时 724.154 秒，命令总耗时 727.533 秒，没有报告竞态错误。
- DB qualification：160 通过、139 子断言通过、1 跳过；跳过项需要显式配置真实 SQLFluff。

### 未全绿的门禁与环境边界

`gate-strict-final` 的实际结果为 **2 FAIL、1 NOT_RUN、1 WARN**。不得把它改写成全绿：

1. 全仓 Python 为 246 通过、1 失败、1 跳过。失败是原有 `cloudflare-platform` 源包缺失；
   `tests/test_sync_skills.py`、`scripts/sync_skills.py` 与相关 Skill 文件均与基线一致，堆栈
   没有本轮修改符号。protobuf 验证因指定解释器未配置而跳过。
2. 第一个 red 从仓库根目录误启动 Vite 工作区，工具将工作区错误误分类为 assertion；该记录
   原样保留且不计有效反证。正确工作区的后续反例及修复已通过，严格回放仍报告 23 条去重
   命令中的这 1 条失败。没有删记录、换工作目录篡改历史或将其伪装成 PASS。
3. `red-hygiene` 对两个新模块约 40 行类型/签名 stub 给出提醒；原始内容和 `--allow` 记录保留。
   这些 stub 只声明接口和返回零值，后续 14 项真实行为断言先失败，再实现业务投影。
4. Python Lint 为 `NOT_RUN(no changed Python files)`。当前修改范围、占位符、抑制项、前端
   格式、类型、Lint、全量测试均通过；完整严格报告与原始命令日志保留。
5. 全仓 Skill 校验另有 52 个原有外部相对链接缺失：Design 13、Cloudflare 14、ThreeJS 25；
   所有拥有文件与基线一致，本轮未删除或更改它们。
6. 原始 CUA 浏览器拒绝离线 `file://` 协议，桌面、窄屏和实际点击验收为 `BLOCKED`，没有
   绕过策略、改开服务器或生成冒充真实浏览器的截图。
7. pay_gateway 缺已授权隔离环境和可用业务身份，真实支付 E2E 为 `BLOCKED`；本轮模型未批准，
   支付运行数为 0。前端结果不证明支付全业务覆盖，也不证明环境当前仍然就绪。

### 影响闭合

定义/构造/调用由 `graph-model.ts`、`selection.ts`、`App.tsx` 与生产入口测试闭合；来源/读取由
`lineage-map.ts`、`lineage-blockers.ts`、`state-overlay*.ts`、`FactPanel.tsx` 和对应反证闭合。
`snapshot.ts` 仅加强既有 DTO 的读取校验；未新增 HTTP API、DB 表、迁移、运行依赖或页面写入
权限。共同契约、工作台 README、工具 README 与绘图 Skill 引用已同步。所有 owning 源码与
测试都在正式 `tools/arc-verify/workbench/src`，`.arc/artifacts` 只保存命令和证据。

本轮原始命令、日志摘要、浏览器拒绝及源码冻结记录位于
`.arc/artifacts/arc-eta-workbench/reports/`，并随离线导出交付；不以任务文档替代原始证据。

发布前对最终生产源码与验证时冻结版本逐文件核对；验证后只有文档措辞和结果记录调整，
运行源码、测试及构建输入没有改变。Git 对象内容须与暂存区 blob 和整树 SHA 一致后才更新
`main`，保留基线提交和全部历史证据。
