# Arc 确定性业务验证重构

## 范围和验收契约

用户于 2026-11-17 明确授权重审并修改全部 Arc 系列、共享协议、CLI、数据库事实链、
执行器和工作台，必要时替换错误抽象。目标是完整前态/条件/主体/操作唯一决定已确认
业务预期，执行前冻结，独立观察 Actual，保存历史证据。ETA 只作为依赖系谱与交互参考。

固定基线：Skills `3690d125f3ebd7923e2d4d3e1888a0c8b8b47446`；
pay_gateway `e4f054fc7bb06e091be62d6765bb29b4348ad651`；
ETA `03197d88a54b3b2015f38d425d76baf95216cf27`。

保留统一 `arc`、显式迁移的 `.arc/arc.db`、短生命周期 CLI、静态只读快照和既有 RAW。
不引入 Agent Plugin 生命周期，不启用 GitHub Actions，不以历史投入保留错误判定。

## 架构裁决

| 对象 | 裁决 | 原因 |
|---|---|---|
| 能力树/既有图版本 | 保留并明确职责 | 表达业务归属，历史来源不可删除 |
| 单布尔前提与动作后任选结果 | 强验证路径废弃 | 丢失主体属性，不能证明唯一预期 |
| business_contract | 新增可执行严格层 | typed guards、显式前态、冻结预期、有限域反证 |
| ETA DAG | 用于准备依赖与布局 | 不替代可循环业务状态机 |
| 执行器、RunnerManifest、轮次 | 原位重构 | 已有真实入口/Oracle/账号锁应继续复用 |
| step_facts/assertions/entity bindings | 保留并校验 | 作为可验证事实投影；损坏拒绝通过 |
| arch-diagram 默认入口 | 改为可选展示导出 | 原生四区工作台需要分支与真实事实联动 |
| 历史 legacy 模型 | 兼容并限制声明 | 旧证据不得自动升级成确定性分支证明 |

## 实施记录

- [x] 模型：属性级多主体谓词、唯一匹配、显式前态、执行前冻结、有限域反证、准备 DAG。
- [x] 真实 Runner：实际请求参数进入 Guard；冻结写入数据库成功后才派发；独立回读、逐项比较及离线重验。
- [x] 准备：有限搜索 → 注册 adapter 实际操作 → 独立回读 → 再规划；预测效果不作为 Actual。
- [x] 正确性：多主体条件、跨账号 Oracle、等待身份、CookieJar 竞争、RAW/投影完整性反例。
- [x] 工作台：统一上下文、依赖/状态/运行三视图、分支事实、轨迹与最长依赖层级；真实 Go JSON 已贯通。
- [x] CLI/数据库：统一子命令路由、附加 schema、原生 task-proof 与新任务完成门禁；历史数据保留。
- [x] 全部六个 Arc 技能、共享协议及生成索引同步；代码约束和属性测试优先于新增文字规则。
- [x] pay_gateway 有来源的有限模型、五种 Runner、真实适配器、独立 SQL/机构观察与执行编排已交付。
- [x] Go 全包 race 通过；最后的严格模型探索旁路补丁补验整个受影响 exploration 模块，123 项 race 通过。
- [x] 续建严格 `campaign`：显式输入变体、实际前沿、准备与执行复用、耐久派发、预算、断点重验及工作台贯通。
- [!] pay_gateway 产品 E2E：没有可用的已绑定测试目标、隔离 PG 与账号凭据；没有发出支付项目请求。
- [!] 工作台浏览器视觉验收：环境拒绝 Chromium/browser 本地页面操作，未绕过限制；SSR 不替代视觉验收。

## pay_gateway 合同约束

API10 通用约定说明 Key 标识业务方、不携带 scope；订单作用域是业务方+资源类型。
退款实现/合同明确原单与原账户绑定，网关不判断原支付成功、不累计退款额度，资金资格
由机构裁定。因此用户举例里的“未支付拒绝/退款余额不足”不能直接当作该项目已确认规则。
自动路由比例等在合同中仍标候选的语义保持未确认，不从代码表现反推 Expected。

首条交付业务线使用原项目真实 router/usecase/PostgreSQL 与受控机构 fixture：
创建付款引用 → 机构 paid 事件 → 正常查询学习结果 → 退款 → 同一实体重复退款。
明确区分引用索引状态和资金状态。四个冻结分支是创建付款引用、首次退款、重复退款、缺原付款拒绝；
五种 Runner 包含准备驱动的连续退款场景和 checkpoint 后重新绑定实际实体的重复请求场景。
模型的有限域只声明固定 caller、账户、100 分、CNY 和四组状态/操作分区。

续建使用新模型 ID `pay-gateway-index-fixture-v2`，保留 v1 历史。金额按已审查解析合同声明
`integer/encoding:json_i64`，保留实际 JSON token；执行编排先比较文件模型与已导入模型的 Go proof
及合同 hash，防止同 ID 旧模型绕过新约束。非法格式在工具派发前阻断，不声称已经验证网关的负向响应。

适配器按整个已绑定 case 统计创建类机构收件和出站，而非以待验证的索引筛选副作用。
因此“请求已发但没留下索引”不会被统计成零；错误 provider、错误请求 ID、无唯一对应索引也独立计入
`provider.unexpected_receipts`。多行和缺父关系保留为实际事实，不能被一个观测异常提前吞掉。

本地构建/运行身份绑定不是独立可信编译来源证明。支付适配器的真实 PostgreSQL 查询、受控机构
prepare/reset、网络时序及真实业务拒绝仍需要在项目隔离环境验收。

## 具体缺陷与修复证据

| 已复现的问题 | 代码裁决与反例 |
|---|---|
| 同一条件的多个主体被压成一个 | 保留完整 readings，typed predicate 按 subject/attribute 验证 |
| 同名 Oracle 跨账号提取/等待串用 | 使用 actor + action_index + OracleID；A 等到不代表 B 已等到 |
| 并发 CookieJar map 竞争 | 真正 race 红测试后加入同步，账号 cookie scope 分离 |
| Expected 可间接从执行后结果挑选 | 执行前唯一分支与版本耐久冻结，Actual 只和该冻结值比较 |
| Oracle 声称 amount=1，实际请求却是 amount=0 | 从渲染后的真实 body 捕获输入；派发前再核请求 SHA；错误输入零请求 |
| explain 看见事实 JSON 就当完整 | 原 RAW、摘要、投影列、断言及冻结收据重新核对；删除/损坏均不能 PASS |
| 防篡改测试提前在错误 SHA 失败 | 反例使用真实有效 RAW SHA，确实进入篡改后的 resolved-input 判定 |
| setup 借用未来才产生的实体 | 每个步骤只显示该步骤已观察/绑定的对象 |
| 状态重试环被当作依赖环 | 状态图保留环，准备依赖单独检查 DAG、计算最长路径层级 |
| 快照去重按随机 run ID 排序 | 保序去重，保留显式回放顺序和默认 rowid 历史顺序 |
| 真实 setup=-1 被前端解析器拒绝 | 使用真实 Go 导出发现并修复，新增永久回归 |
| 支付禁止副作用按已存在索引过滤 | 整个 owned case 的出站/收件计数与独立关联检查；孤儿出站反例 |
| bootstrap 后改源码却仍测旧进程 | 执行前及 PASS 后复核绑定；保留原业务判定并将交付状态阻断 |
| 旧 discover 可绕过新合同的执行前冻结 | typed run/resume 在 claim/观察/派发前阻断；红例实际发请求，修复后零请求且历史不变 |

## D01–D10 的证据范围

| 编号 | 回归位置 | 验证层次 |
|---|---|---|
| D01 | `business_contract_test.go`：同一完整输入两预期 | 内核反例，MODEL_ERROR |
| D02 | 同文件：同一操作按实际前态选择；旧 v2 图兼容 | 内核与模型协议 |
| D03 | 同文件：部分重叠有限域反例 | 可枚举声明域内歧义，不是无限域证明 |
| D04 | 同文件：未覆盖组合 | SPECIFICATION_GAP，不默认成功 |
| D05 | 同文件及 `runtime_business_test.go` | 实际前态错误时目标 HTTP 零请求 |
| D06 | 同文件、`runtime_business_test.go`、`runtime_input_test.go` | 冻结后偏离 FAIL；输入变化零请求；Actual 不改 Expected |
| D07 | 同文件及 `runtime_facts_regression_test.go` | 真实事件观测决定后续转移；异步等待的独立主体归属 |
| D08 | `business_chain_test.go`、`runtime_checkpoint_test.go` | 工具 HTTP 目标的多步实体/session 传递；checkpoint 保留实际状态 |
| D09 | `runtime_facts_regression_test.go`、`runtime_identity_test.go` | 多账号 Oracle/等待/Cookie/别名隔离，重名默认 UNKNOWN |
| D10 | `preparation_test.go`、`layout.test.ts` | 准备 DAG 与状态环分开；依赖捷径不压低真实层级 |

以上是工具层证据，不能被改写为 pay_gateway 已实测通过。

## 第一阶段验证汇总

| 项目 | 结果与边界 |
|---|---|
| 模型、实际输入、准备、checkpoint、证据隔离 | 新增反例已执行；执行前零请求和实际 HTTP 路径都有验证 |
| Go 验证包 | 全量 `-race` 419 个顶层测试通过、4 个可选环境测试跳过，274.001s；随后最后旁路补丁的 123 个探索模块测试补验通过（24 + 99，5.635s + 64.554s）；vet 通过 |
| 统一 CLI / 索引 | CLI 全量 race 37 项通过，1.749s；索引包测试通过，0.333s |
| 前端 | 39 项 / 7 文件通过；23 文件 check 无 warning/error；TypeScript 和构建通过 |
| 真实 Go JSON → 前端 | 另 3 项通过，实际工具 run `R-e9bd41ae372a079c7e8f115e349a883b`；assets 与最新 dist 一致 |
| 支付适配器与编排 | 16 个 Python 测试通过；5 种生成 Runner 经过真实 Go validateManifest/validateFixtures |
| 支付有限模型 | `PROVEN_IN_SCOPE` 恰 4 case；持久化、响应、重复出站、错机构等反例检查通过 |
| 属性测试 | `FuzzBusinessContractUniqueFreezeDoesNotFollowActual` 独立 5 秒运行，6,244 次执行通过 |
| 完整 Python 测试 | 214 passed、1 skipped、1 failed；失败是基线已有缺失外部 `cloudflare-platform`，已用原始基线复核 |
| dbqual | 160 passed、1 skipped、139 subtests passed；SQLFluff 可执行文件缺失项跳过 |
| Arc 技能 | 六个入口、共享目录和本次文档链接 0 error；相关 54 项测试与 14 个加载预算通过 |
| 全仓技能链接 | 52 个基线外部缺失：Design 13、Cloudflare 14、ThreeJS 25；没有删检查或抬预算掩盖 |
| 安装与 smoke | 统一三引擎安装、真实 Zoekt/Universal Ctags/ast-grep、索引 outbox、旧图与 Policy 数据迁移实际通过 |
| 构建后 CLI | `TestRunChainBuiltBinaryExitStatus` 实际执行已安装二进制，真实退出码检查通过 |

模型 proof、运行 verdict、证据完整性、交付产物状态分别保留；不会用其中一个绿色字段替代其余字段。
本次逐项工具测试结果、原日志摘要指纹与明确阻断见
[工具验证记录](evidence/arc-deterministic-tools-2026-11-17.json)；它不是产品运行的 PASS 收据。
通用 skill-creator quick_validate 不支持本仓已有 `arc:` 命名与 `version` 元数据，原始基线也失败；
仓库自己的严格 Arc 校验通过。R8 protobuf 解释器和两项 IM-mongo 本机配置未提供时显式 SKIP。

## 续建：从实际状态调度有界业务变体

续建基于已交付提交 `34094b969f8c26ac79b4884fc2a8244aa14434e4`，补齐此前禁止走 legacy
discover 后仍缺少的严格在线执行入口。`campaign` 的每个变体绑定已注册 Runner 和已确认分支，
先独立读取当前状态与输入，再通过同一个准备、冻结、真实执行和比较协议派发 child run。
后续选择使用新观察，遵守已验证 outcome 的 `next_allowed`，不会把 Expected 后态写入前沿。
未覆盖的可达组合停止为 `SPECIFICATION_GAP`，不会因为其他候选 READY 就掩盖缺口。

派发前持久化唯一 child run、来源身份和保守动作预算，使用 revision、lease 与 fence 防止过期
执行器继续发出动作。恢复核对原始派发、实际子运行、RAW 与事件链；无法确认动作是否提交时保留
UNKNOWN 及预算预留，不盲目重试。正常 `retry` 仍不能重开已 PASS 任务；下一个 campaign 变体
必须消费自己的一次性派发授权。新表附加到既有账本，不重写历史事实。

真实工具验收包含两个独立导出：一个运行实际 `/activate` 准备，再观察账号 false→true 后
执行目标；另一个在同一实际实体上完成三次 checkpoint child run：操作、清除状态、再操作。
后者只有首次发生准备，每一步都通过新鲜独立 Oracle 重绑定相同实体 ID，没有复制上一步会话。
这些是隔离 HTTP 工具系统的真实动作与观察，不是 pay_gateway 产品验收。

新增反证覆盖：探测实体 X、派发前读到 Y 时目标零请求；改写全部摘要后移植历史 PASS 仍无法
伪造原子派发来源；改写预算不能在恢复时多发动作；父 deadline 到期后不能进入恢复写操作；
准备探测的 principal、role、scope、round、runner、oracle 或 boundary 身份损坏不能 PASS。
相同生命周期标签但外部条件改变时，后续选择由新观察决定；状态环仍不进入准备 DAG 的环校验。

工作台增加 campaign/variant/child 选择、实际前沿、准备与恢复边界，继续同步八项业务上下文。
`COMPLETED_IN_SCOPE` 只证明选定变体完成，保持 `full_business_coverage=false`；损坏记录保留
可信索引身份与阻断原因，不填入虚构预算或覆盖值。生产 JSON 读取保留 64 位整数及数字原文。

本轮的精确命令、测试统计、原日志与最终导出摘要、反例和阻断记录见
[续建工具验证记录](evidence/arc-campaign-tools-2026-11-17.json)。该文件区分全量测试完成时的源码
与其后单个证据校验补丁的定向 race 补验；既不是支付执行收据，也不把 SSR 当成浏览器验收。

| 续建检查 | 结果与边界 |
|---|---|
| 验证包全量 race | 455 个顶层 Test/Fuzz、265 个子例 PASS，5 个显式环境 SKIP，0 FAIL，382.288s |
| 全量之后的补验 | 证据身份补丁 8 个顶层 + 9 个子例 race PASS；新增 NextAllowed 反例 race PASS；没有重写全量日志声称同一时刻验证 |
| 当前测试入口并集 | 461 个顶层入口已 PASS，2 个 IM-mongo 外部配置项仍 SKIP；可选导出与安装后二进制项已单独实际执行 |
| 安装后二进制 | 公开 `arc` 的 runner → round → campaign → status/resume 真实进程通过，目标仅一次请求；run-chain 真实退出码通过 |
| 公开 CLI / 索引 / vet | CLI 全量 race PASS，索引包 PASS，验证器完整 vet 与最后证据补丁 vet PASS |
| 前端 | 61 PASS、1 个无输入路径的显式 SKIP；31 文件 check 0 warning/error；TypeScript 与构建通过 |
| 实际导出贯通 | 永久集成测试 2 PASS；准备导出 2 个 SSR 上下文、campaign 导出 16 个上下文；两套资源均与冻结 dist 一致 |
| 支付适配器 | 18 个 Python 测试 PASS；有来源的 json_i64 模型及实际输入派发前反例通过，未发出产品请求 |
| Python / dbqual | 全仓 214 PASS、1 SKIP、1 个已复核基线 Cloudflare 缺失；dbqual 160 PASS、1 SKIP、139 子例 PASS |
| 安装 / smoke | 三引擎安装、真实索引 outbox、旧图与 Policy 数据迁移通过；无遗留运行进程 |

最终工具导出记录为准备 run `R-1313254e24c8e45713d15bf0f3fa705f`，以及 campaign
`BC-e4fda5a92842029917c30a13dbb9a6fb`。后者三次动作实际状态依次为
`pending → ready`、`ready → pending`、`pending → ready`，三个 child 都使用 checkpoint。
未映射的 `clear-inactive`、`reject` 分支仍显示未覆盖，不能由这三次通过变绿。

## 复审修复：业务连续性、恢复与工作台语义（2026-11-17）

在已合并的 `9c9fd3ce35c0e7c1f4ce6fc6418ddb5219a9f41d` 上复现八项缺陷后，
将反例转成正式回归并修复执行、证据重验、准备规划及工作台的语义分歧。原测试通过记录保留，
不将其解释为已经覆盖本轮新增反例。

| 问题 | 本次修复与验证 |
|---|---|
| F1：普通 SEQUENCE 忽略后续动作限制 | 下一批真实操作前检查上一已验证结果；离线逐步链也重验原始前一步。禁止动作零请求；切换账号不能绕过，合法跨账号链仍可通过 |
| F2：空交集变成无限制 | `BusinessSuccessorPolicy` 统一匹配、执行和重验；省略/null、显式空集合、非空允许集合分别保留。模型冻结、RAW、campaign 入库与 UI 不再丢失空集合 |
| F3：有限验证宣称完整业务 E2E | gate 与 acceptance report 返回 `complete_in_declared_scope`；`business_e2e_status=NOT_EVALUATED`，兼容字段 `complete_business_e2e=false` |
| F4：无法建立的前置条件仍被准备 | 提升 PREPARABLE 前检查全部目标条件；失败布尔条件只能通过同主体、同 Oracle、同响应指针的明确属性映射预测修复。无映射反例为零准备、零目标、零恢复 |
| F5：落账失败污染停止记录 | 在完整独立副本中生成子运行投影，事务成功后才替换当前对象；原 pending 子运行的归账与 UNKNOWN 解除原子提交。PASS/FAIL/BLOCKED 故障恢复均不重发，保留首次偏离 |
| F6：拒绝条件画在成功边 | 从分支 × 前态 × ExpectedOutcome 生成状态边；UNCHANGED、自环、拒绝、允许结果集合各自保留语义。未声明后态只展示约束占位，旧迁移单独标识 |
| F7：流程筛选隐藏全局分支 | 可选 flow 与内核一致；未限定流程的分支不消失，实际运行仍按自己的 flow 隔离 |
| F8：追踪命中同动作其他分支 | 已选分支精确定位，缺失时不回退到其他分支；多个前态/允许结果取全部适用边，无分支身份时才按动作展开 |

历史反证先使 RAW、引擎捕获、事实投影与哈希自洽，确认两步各自局部验证通过，再证明新的跨步
重验拒绝违规链；没有用错误 SHA 提前失败冒充目标缺陷测试。旧的合法省略/非空后续限制记录仍可
重验，历史违规记录转为 BLOCKED 而不修改原 RAW。没有 SQL schema 变更；现有 JSON 列保留显式
`next_allowed: []`，不执行历史数据改写。

整合后的 Go 生产源码共 85 个文件，摘要与全量测试开始时一致。详细命令、日志摘要、红测指纹、
两个新导出和剩余边界见 [复审修复验证记录](evidence/arc-review-fixes-2026-11-17.json)。

| 整合验证 | 实际结果 |
|---|---|
| verify 全量 race | 474 个顶层 Test/Fuzz、301 个子测试 PASS，0 FAIL；两项外部 IM-mongo 配置测试 SKIP，470.479s |
| 公开 CLI / 索引 / vet | CLI 37 个顶层及 40 个子测试 race PASS；索引 2 PASS；verify vet PASS |
| 安装与真实进程 | 三引擎安装、统一 CLI smoke、索引 outbox 与旧图/Policy 迁移通过；全量测试实际使用新安装二进制执行 run-chain 与 campaign |
| 工作台 | 86 PASS；默认导出测试因无路径 SKIP，随后提供本轮真实 Go 导出完成 2 项集成 PASS；43 文件格式、34 文件 lint/type、TypeScript 与构建通过 |
| 新导出贯通 | 准备运行与三次连续 checkpoint campaign 通过生产解析器、统一选择和 SSR，分别检查 2、16 个选择上下文；资产与最终 dist 一致 |
| Python / dbqual | Python 214 PASS、1 SKIP、1 个原有 Cloudflare 源包缺失 FAIL，原基线精确复现；dbqual 160 PASS、139 子例 PASS、1 个外部 SQLFluff SKIP |
| Skill 校验 | Arc 无新增错误或警告、加载预算通过；全仓原有 52 个外部 Cloudflare/Design/ThreeJS 链接错误与基线一致，未放宽门禁 |

新生成图 ID 已逐段编码，不把引号和分隔符直接拼入绘图库选择器。构建仍保留单个离线 IIFE
超过 500 kB 的体积提示。本轮工具 HTTP、SSR 和源码检查都不计作 pay_gateway 产品 E2E 或浏览器
视觉验收；这两项仍维持明确的 NOT_RUN/BLOCKED。

## 第二轮复审：准备主体、规则身份与事实作用域（2026-11-17）

本轮从已合并的 `ba5c789a4d6ba5127462521f38b714d2a85a17cc` 继续复审，新增十项可复现问题。
修复覆盖真实派发前的约束、离线证据重验和工作台选择语义；每项反例均进入正式回归，保留合法
执行的正向控制，没有通过修改预期结果、重写 RAW 或删除旧门禁使测试变绿。

| 问题 | 修复与实测边界 |
|---|---|
| R2-01：UNKNOWN 结果候选丢失后续限制 | 所有未被 Actual 否定的候选参与 `next_allowed` 交集；当前动作可通过仍不等于唯一结果已知。原反例后续确认请求实际发出一次，修复后为零；同样限制离线重验，保留 UNKNOWN 明细 |
| R2-02：精确数值先被 float64 拒绝 | JSON 重复键扫描、结构解码、请求/响应和 Oracle 捕获保留数字原文。模型允许范围内的 `1e400` 可真实传输、精确比较和重验；原长度/指数限制及 `json_i64` 的 token 规则不变 |
| R2-03：次级 Oracle 跨作用域证据仍可通过 | 重验全部注册 Oracle 的 action/actor/ID、观察边界、principal/role/scope、字段与唯一性；自洽摘要的历史错账号/角色/边界记录也被拒绝。真实错误读者保持目标零请求 |
| R2-04：Oracle 数组顺序改变一致性判定 | 一致性层绑定最终动作的 action_index；同 actor、同名 Oracle 的两个动作重排后不再把 FAIL 变 PASS，或把 PASS 变 FAIL；无法唯一定位的 legacy 记录阻断 |
| R2-05：弱 Ensures 跳过必要准备及依赖 | 判断已完成时同时检查 Ensures 和全部声明 Effects；规划、依赖层、真实动作前回读及离线重算共用该规则，不把预测写进 Actual |
| R2-06：准备写入错误账号 | 准备 Goal 或所选计划 Effects 涉及、且与初始目标 batch 相关的主体，其 observer 与显式 adapter 绑定必须同目标一致；实际 campaign/execute 错账号反例修复后准备、目标、恢复全部零请求。保留合法 ADMIN 代办、静态 setup 创建及自动准备创建实体；重验不借用未来提取或伪造初始绑定 |
| R2-07：其他环境或轮次的节点 PASS 被误用 | 节点汇总展示来源；选中不同 environment/round/variant 时不借用原 PASS，聚合结果也不替代当前运行和步骤判定 |
| R2-08：模型、metadata 与视图身份错配 | 生产解析器核对声明的模型 ID/hash 与 proof context；无轮次初始模型保持可读但不显示已证明 PASS。这里只验证声明一致性，不声称浏览器重新验证 RAW 真实性 |
| R2-09：动作前实体在失败后无法选择 | 实体选择取 `before_entities ∪ entities`，保留 alias/kind/ID/owner/channel 身份；展示动作前后捕获情况，不将后态缺记录推断为删除 |
| R2-10：执行器借用同操作的另一条规则 | 显式分支注册、执行前冻结与离线重建核对 branch.rule 与 step rule；原反例同角色或另一角色均实际发请求并 PASS，修复后请求为零。正确绑定另一规则的执行和重验仍通过 |

主体检查约束已声明的业务身份，不能推断任意 opaque HTTP/command adapter 在服务器内部实际修改
了谁；独立读回仍承担验证真实状态的职责。准备作用域限定初始目标 batch，不禁止后续步骤合法
切换主体。独立复核也补上了创建前允许无 ID 的兼容正控，避免因身份检查误封正常准备路径。

详细测试命令、源码与日志指纹、实际工具导出和验收边界见
[第二轮复审验证记录](evidence/arc-review-round2-2026-11-17.json)。本轮不改变 SQL schema，
不修改原始历史证据，不新增公共 CLI，不启用 GitHub Actions。

| 最终整合验证 | 实际结果 |
|---|---|
| verify 全量 race | 493 个顶层 Test/Fuzz、360 个子测试 PASS，0 FAIL；两项原有 IM-mongo 环境测试 SKIP，514.704s |
| 公开 CLI / 索引 / vet | CLI 37 个顶层及 40 个子测试 race PASS；索引 2 PASS；verify vet PASS |
| 安装与进程 | 三引擎安装、统一 CLI smoke、真实索引及迁移检查通过；完整 race 使用本次安装的 run-chain 与 campaign 二进制 |
| 工作台 | 101 PASS、1 个默认无导出路径的 SKIP；另提供本轮实际 Go 快照后 2 项集成 PASS；44 文件格式、35 文件 lint/type、TypeScript、构建通过 |
| 实际导出贯通 | 自动准备运行及三次连续 checkpoint campaign 通过生产解析器、选择上下文与 SSR；分别验证 2、16 个上下文，导出资产与本次 dist 一致 |
| Python / dbqual | 配置正确 Go 工具链后，Python 214 PASS、1 SKIP、1 个原有 Cloudflare 源包缺失 FAIL；dbqual 160 PASS、139 子例 PASS、1 个外部 SQLFluff SKIP |
| Skill 校验 | Arc 0 error、0 warning，14 项加载预算未变；全仓原有 52 个外部技能链接错误与本轮基线逐项一致 |

整合后 86 个 Go 生产文件的摘要在全量测试前后保持一致；提交到 GitHub 的代码树也逐提交与本地
核对一致。工作台仍有单个离线包超过 500 kB 的体积提示。pay_gateway 产品 E2E 和实际浏览器
视觉验收继续维持 `NOT_RUN/BLOCKED`，不以工具 HTTP 目标或 SSR 代替。

## 第三轮复审：批次不变量、失败边界与步骤身份（2026-11-17）

本轮从已合并的 `3f5a41d31e111affb5f0fd02689ae1997908083c` 继续检查，按用户要求直接修改
`main`。新增五项已复现问题；修复仍复用原账本、冻结契约和观察作用域，没有 SQL schema 迁移，
没有改写历史 RAW，也没有修改业务预期来接受偏离。

| 问题 | 修复与反例证据 |
|---|---|
| R3-01：并发批次只判定最后一个动作的不变量 | 对每个动作选取自己的规则，以 actor、action_index、Oracle ID 和 boundary 定位独立观察。真实双账号反例中，前一个账号 `enrolled=3/capacity=2`、末尾账号正常，原实现错误 PASS；typed/legacy 与账号顺序均复现。修复后为业务 FAIL，并保留有效证据。重复同账号的不同 action_index 及自洽哈希历史回放也进入回归 |
| R3-02：其他前态分支的 flow 缺失错误阻断当前分支 | 先按动作与实际前态排除不适用分支，再检查候选 flow。pending 状态的完整互斥分区不再被 ready-only 分支阻断；当前前态适用的分支缺少必要 flow 仍 BLOCKED，有限域证明不扩大声明范围 |
| R3-03：改写停止投影能恢复已失败的 campaign | 从真实 child 重算首次偏离后，拒绝后续派发，要求停止状态符合失败事实或显式 ABORTED。反例保留原 child/RAW，只把外层状态改为 BUDGET 并重算事件哈希；原 FAIL、BLOCKED 两例分别多发一次目标请求，修复后均不再派发。健康取消后的预算停止仍可恢复 |
| R3-04：未冻结步骤借用同运行其他分支 | 有明确 BLOCKED 决策或在前置 Oracle 失败时尚无决策的 typed 动作，均保留诊断身份，不用 run.branch_id 补齐。混合运行同时保留已知分支和诊断入口；步骤、主体与预期面板同步切换。没有 unified step facts 的旧导出保持兼容 |
| R3-05：RAW 子记录可混入其他运行或步骤 | 生产解析器核对外层 run/phase/step、RAW 内容身份及已声明的 graph/task，拒绝重复证据身份；保留 setup 摘要 step=0 对 RAW step=-1、旧 action 缺 phase 及 error-only 条目。这里验证声明一致性，不声称浏览器重验 RAW 哈希或真实性 |

R3-01 的实际 FAIL 与 `EvidenceStatus=PASS` 分开：后者表示偏离事实可独立重验，不代表业务
成功。R3-03 反例针对自洽改写后的语义检查，不声称正常执行会自然损坏停止状态。R3-04 另通过
真实执行器导出连续运行：第一步正常冻结，第二步 before Oracle 返回 404，确认操作没有发出，
后一步事实确实没有 `business_expectations`；生产 reader、选择上下文与 SSR 共同验证诊断关联。

详细命令、日志与源码指纹、真实工具导出及验收边界见
[第三轮复审验证记录](evidence/arc-review-round3-2026-11-17.json)。工具 HTTP fixture 和 SSR
不替代 pay_gateway 产品 E2E 或浏览器视觉验收，两项仍保持 `NOT_RUN/BLOCKED`。

### 本轮整合验证

| 检查 | 实际结果 |
|---|---|
| verify 全量 race | 499 个顶层 Test/Fuzz、376 个子测试 PASS，0 FAIL；两项原有 IM-mongo 环境测试 SKIP，577.156s |
| 公开 CLI / 索引 / vet | CLI 37 个顶层、40 个子测试 race PASS；索引 2 个 race PASS；verify vet PASS |
| 安装与真实进程 | 三引擎安装、统一 CLI smoke、索引 outbox 和迁移检查通过；全量测试使用本轮安装的 run-chain 与 campaign 二进制 |
| 工作台 | 123 PASS、0 SKIP、0 FAIL（含真实 Oracle 失败导出）；46 文件格式、37 文件 lint/type、TypeScript 和构建通过 |
| 最终实际导出 | 准备运行、三次连续 checkpoint campaign、前置 Oracle 失败运行，3 项生产 reader → selection → SSR 集成 PASS；分别验证 2、16、3 个上下文，三套资产均与冻结 dist 一致 |
| Python / dbqual | Python 214 PASS、1 SKIP、1 个原有 Cloudflare 源包缺失 FAIL；dbqual 160 PASS、139 子例 PASS、1 个外部 SQLFluff SKIP |
| Skill 校验 | Arc 0 error、0 warning；全仓 52 个原有外部链接错误与基线逐项一致。本轮新增参考文档曾超预算，压缩到 3,418 ≤ 3,500 后复验通过 |

整合前后 86 个 Go 生产文件及最终前端源文件指纹保持一致。全量测试中较早生成的 campaign
快照保留原样，另用最终 dist 真实执行并导出新 campaign，不手工拼接事实。静态包仍有超过
500 kB 的非阻断提示。本轮按用户授权直接提交 main；最终代码树与测试冻结树之间只增加
任务验证汇总和本轮证据记录。

## 使用与迁移

工具仍由 `bash scripts/install_arc.sh` 安装，公共命令只有 `arc`。升级现有激活库后显式
执行 `arc verify init` 增加新表，迁移测试验证原 `step_facts` 与 RAW 字节不变。
`arc verify contract check/match/compare/prepare` 直接调用同一个确定性内核；文件输入只作模型分析，
不伪装成实时观测。已有 `execute`/`run-chain` 使用严格冻结、实际动作和独立回读。

新建任务的完成必须关联当前可验证义务，`task-proof` 独立重验最近运行及源码/配置/模型身份，
旧 Policy receipt 继续可读但不授权新的完成。工作台为只读静态导出，写操作只走 CLI。

完整支付接入步骤在 [pay-gateway README](../../tools/arc-verify/examples/pay-gateway/README.md)，
原生工作台构建/导出在 [workbench README](../../tools/arc-verify/workbench/README.md)。

## 尚未完成、不能声称已证明的范围

1. pay_gateway 真实隔离环境执行及所有支付、确认、回调、通知、查询、权限、并发、超时/恢复分支验收。
2. 通用参数组合生成、跨生命周期准备搜索和状态等价归约仍未完成。续建的 `campaign` 已支持显式注册
   输入变体的严格在线探索，复用 prepare/freeze/execute/compare；限定单目标操作、无静态 setup，
   准备可行性只使用已声明属性效果。旧 discover 继续拒绝 typed 模型，legacy 探索和历史保留。
3. 无限域覆盖、全业务路径可达性和状态等价的形式证明。有限域不可达排除必须有来源，不能任意删组合。
4. 自动准备的并行调度和准备动作异步等待；当前准备串行且每步独立回读。目标业务步骤仍支持有界等待。
5. 跨 run 不自动继承内存变量、会话和 cookie。campaign 只允许通过已注册独立 Oracle 的新鲜观测
   重绑定非敏感实体，且同字段 measurement 要能离线复核；checkpoint 保留现场而不复制内存状态。
6. 浏览器视觉/可访问性验收、实际 pay 自动准备轨迹的 UI 验收仍阻断。续建已用真实 HTTP 工具运行导出
   自动准备、独立 false→true 回读、请求/冻结/响应/checkpoint，并经过生产解析器与 SSR；这不是产品 E2E。
7. 独立可信外部 Runner、远端证据签名与构建来源证明。当前完整性检查防止缺失/损坏/不一致，不对控制宿主者作真实性担保。
8. TLA+、Lean、Bombadil 集成未实现。当前是可执行反例、属性测试与直接 import 边界检查，不能称为形式化全证明。
9. 支付示例 v2 已按实际解析合同使用有来源的 `integer/encoding:json_i64`，在请求派发前拒绝
   `100.0`、`1e2`、字符串、空值和越界整数；普通 integer 保留旧数学语义。这保护四个选定 case
   的合法输入范围，不等于已真实执行这些非法请求并验证网关拒绝；后者仍需独立负向分支与产品环境。

本轮不以“全部完成”掩盖这些缺口。下一次实际验收应从已交付、已验证的内循环继续，
补齐真实环境和剩余业务规则，再扩大自动探索与外循环。
