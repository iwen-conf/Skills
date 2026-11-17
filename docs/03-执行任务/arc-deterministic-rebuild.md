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

## 验证汇总

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
2. 通用 typed 在线探索调度器。本轮提供有限条件分区证明、有界准备搜索和真实分支执行覆盖；
   旧 discover 执行器尚无 typed 冻结协议，严格模型必须明确拒绝走该旁路。既有 legacy 探索和历史保留。
3. 无限域覆盖、全业务路径可达性和状态等价的形式证明。有限域不可达排除必须有来源，不能任意删组合。
4. 自动准备的并行调度和准备动作异步等待；当前准备串行且每步独立回读。目标业务步骤仍支持有界等待。
5. 跨 run 自动继承内存变量、会话和 cookie。checkpoint 只保留现场；新 run 需要实际重新观察并绑定实体。
6. 浏览器视觉/可访问性验收、实际 pay 自动准备轨迹的 UI 验收。SSR 已验证实际请求/冻结/响应/checkpoint，
   本次导出的工具 fixture 没有自动准备动作，准备面板另有合成回归。
7. 独立可信外部 Runner、远端证据签名与构建来源证明。当前完整性检查防止缺失/损坏/不一致，不对控制宿主者作真实性担保。
8. TLA+、Lean、Bombadil 集成未实现。当前是可执行反例、属性测试与直接 import 边界检查，不能称为形式化全证明。
9. 支付输入模型不覆盖 JSON 金额词法：当前 integer 按数学整数比较，示例发送固定字面量 `100`；
   `100.0`、`1e2` 等 API 词法拒绝需独立补充合同及测试，不属于这四个 case 的验收声明。

本轮不以“全部完成”掩盖这些缺口。下一次实际验收应从已交付、已验证的内循环继续，
补齐真实环境和剩余业务规则，再扩大自动探索与外循环。
