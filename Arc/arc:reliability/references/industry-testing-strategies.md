# 行业测试策略：证据、适用条件与 Arc 取舍

在设计系统级测试策略、审视现有测试盲区，或需要解释为什么选择某种验证方式时读本页。用例设计入口是 `arc:test` 的 [`测试范围与用例设计`](../../arc:test/references/test-case-design.md)，具体业务合同仍来自当前项目。

核读日期：2026-09-18。下表来自已读取的一手公开材料及其相关章节；历史文章反映当时公开实践，不代表公司当前全部流程，也不能据此宣称这些系统没有事故。未把搜索摘要或仅含评论/导航的抓取结果当作正文证据。

## 公司实践如何改变测试决策

| 公司 / 系统 | 原文实践与所针对的盲区 | Arc 采用的决策 | 适用边界 |
|---|---|---|---|
| Google SRE [G1] | 测试配置、压力边界、运维/自动化工具；已有系统先投入高影响、低成本验证 | 测试面包含二进制之外的配置、运行环境、数据操作和恢复工具；按业务后果排序 | 不把全部 SRE 能力套到每次小改动；原文也区分核心系统与短期脚本 |
| AWS [A1] | 连续运行业务，在旧新版本共存、全部升级、逐步回滚中测试；准备/激活分阶段改变格式 | 验证版本之间的过渡，覆盖读写顺序、历史数据、后台作业和支持的回退下限 | 单版本分别通过不能证明混跑；准备阶段覆盖不全不能激活；激活后不一定能回到最初版本 |
| Netflix FIT [N1] | 沿用户请求链定位故障点，先限制到测试账号/设备，再逐步扩大；验证依赖缺失时仍能完成核心任务 | 从关键用户任务反推依赖故障场景，核对隔离、退化结果和恢复，先做最小实验 | 2014 年的工具名与生产扩量方式不是默认方案；不继承生产 chaos 权限或固定放量比例 |
| Uber BITS [U1] | 高度 mock 的测试漏掉跨模块事故；按架构选择有业务意义的真实服务集合；隔离租户/数据/流量，测试配置 | 测试边界以可观察业务效果为准；真实集成与小测试互补；记录替换依赖的盲区 | 不照搬其生产共享环境或 E2E 数量；运行隔离成立才有可用证据 |
| Apple / FoundationDB [F1] | 确定性模拟执行真实核心代码，重放网络/机器/存储故障；另有真实集群性能和硬件故障测试 | 核心状态机可用可控调度与不变量探索；模拟正确性、真实性能、真实环境行为分别验证 | 一般业务不需要自建整个模拟器；模拟的虚拟时间和故障模型不能证明真实吞吐或硬件行为 |
| SQLite [S1] | 独立测试 harness、SQL 差分、逐个故障位置、崩溃恢复、复合故障、边界与变异；再跑实际交付编译配置 | 独立判定器；在提交/恢复边界选故障点；检查测试能否抓到错误；确认交付构建与测试构建关系 | 其核心高覆盖目标有特定产品背景；不复制测试代码倍数或覆盖率指标，变异也有等价/性能例外 |
| TigerBeetle [T1][T2] | 多个 fuzzer 仍因输入相关性漏掉交集查询结果；协议感知模拟同时检查全局安全性与单副本恢复进展 | 审查生成器状态分布和完整结果集；必要时观察内部不变量，避免整体可用掩盖局部卡死 | 黑盒测试与内部断言互补；其协议/物理确定性是前提，不能作为任意数据库的默认承诺 |
| Cloudflare [C1] | 2019 年规则变更通过功能测试，仍触发 CPU 耗尽；全局规则发布、告警和应急访问亦有缺口 | 功能正确与资源安全分开验证；配置也要分阶段观察；恢复工具和应急通路本身必须可用 | 这是一次事故的已知教训，不把每条配置都升级为全套压力演练；非功能风险由实际变更决定 |

## 专业测试工程师怎样确定范围

| 方法来源 | 核心方法 | 在 Arc 中的落点 |
|---|---|---|
| Anthony Vallone / Google [G2] | 以问题构造计划：风险、架构/平台、哪些不测、测试大小、迁移兼容、版本、工具与发布；权衡收益和成本 | 一份可维护的范围卡，显式记录未测内容；稳定策略与变更计划分离，不填无关模板 |
| James Bach / HTSM [H1] | 项目因素、产品因素、质量维度、测试技术四个视角；v6.3 说明强调状态与边界，鼓励按组织调整 | 把视角当作发现遗漏的提示，不能把启发式清单变成全选门禁 |
| Michael Bolton / FEW HICCUPPS [H2] | 说清预期来自哪里；合同、历史、同类产品、用途等 oracle 都可能失准 | 区分规则依据、实际观察与推断；来源冲突先核对，不能以当前实现或历史行为循环证明正确 |

详细技巧、最小用例字段及缺陷敏感性见 [`测试范围与用例设计`](../../arc:test/references/test-case-design.md)；完整演示见 [`库存、支付通知与取消示例`](worked-example.md)。

## 需要保留的差异和限制

**不规定测试金字塔比例。** Google [G2] 建议尽量用低成本的小测试；Uber [U1] 说明在其微服务架构下，有业务意义的边界可能跨许多模块。Arc 选择能承载目标语义的最小可信边界，再为关键用户任务补跨系统证据。不存在通用的 E2E 百分比。

**重试是诊断工具，不能改写事实。** Uber [U1] 使用重试、placebo 基线与自动隔离治理不稳定测试，并给核心流程等设置例外。Arc 吸收其对照诊断和责任归属：保存全部尝试，用相同条件下的已知基线帮助区分变更、环境与测试问题。关键必需场景被隔离后，证据仍是缺失或不稳定；不能因 CI 不再运行它就算通过。

**高覆盖不是无用，也不是充分条件。** SQLite [S1] 的强覆盖有助于发现盲区，同时用变异检查断言、用交付构建重新运行。TigerBeetle [T1] 则展示了大量生成式测试仍可能遗漏状态。Arc 保留项目有依据的覆盖门槛，同时要求业务主张、输入/历史分布和判定器证据；不设通用覆盖率或变异分数。

**金丝雀也会给出误导性信号。** Google [G3] 要求样本、持续时间和指标能代表目标业务；单看变更前后容易混入时间因素，共享依赖又可能让候选和对照同时恶化。Arc 在已有授权的发布验证中对齐候选/对照条件、按版本隔离指标，并同时检查绝对 SLO/业务不变量。异步观察要覆盖真实工作单元，少量成功请求不能证明安全。

**模拟、预生产和生产观察各有证明边界。** FoundationDB [F1] 明确区分模拟正确性、真实性能与真实故障。Arc 先注明每类证据覆盖的假设；生产观察补充真实分布，不能替代缺失的提交前验证，也不能证明从未出现过未观测的错误。

## 来源与已核读范围

| ID | 一手来源 | 本次使用的章节或内容 |
|---|---|---|
| G1 | [Google SRE — Testing for Reliability](https://sre.google/sre-book/testing-reliability/) | 配置/压力测试、建立测试环境、Testing Scalable Tools 与风险隔离 |
| G2 | [Google — The Inquiry Method for Test Planning](https://testing.googleblog.com/2016/06/the-inquiry-method-for-test-planning.html) | Anthony Vallone，2016 年文章；风险、范围、工具、流程与效用提问 |
| G3 | [Google SRE Workbook — Canarying Releases](https://sre.google/workbook/canarying-releases/) | 代表性、指标归因、前后对比、隔离与异步系统 |
| A1 | [AWS — Ensuring rollback safety during deployments](https://builder.aws.com/content/3F04j2yRAAMBuPSPs50xwXZqg01/ensuring-rollback-safety-during-deployments) | Sandeep Pokkunuri；Prepare/Activate、限制、upgrade-downgrade 验证 |
| N1 | [Netflix — FIT: Failure Injection Testing](http://techblog.netflix.com/2014/10/fit-failure-injection-testing.html) | 2014 年文章；Failure Scope、Injection Points、Scenarios、Automated Testing |
| U1 | [Uber — Shifting E2E Testing Left](https://www.uber.com/us/en/blog/shifting-e2e-testing-left/) | 2024 年文章；BITS 边界、隔离、变更选择、flaky 管理与 placebo |
| F1 | [FoundationDB — Simulation and Testing](https://apple.github.io/foundationdb/testing.html) | 确定性模拟、不变量、真实集群性能与故障测试 |
| S1 | [SQLite — How SQLite Is Tested](https://www.sqlite.org/testing.html) | Test Harnesses、Anomaly Testing、覆盖/交付配置、Mutation testing |
| T1 | [TigerBeetle — Fuzzer Blind Spots (Meet Jepsen!)](https://tigerbeetle.com/blog/2025-06-06-fuzzer-blind-spots-meet-jepsen/) | 2025-06-06；交集查询缺失、相关生成器与独立精确模型 |
| T2 | [TigerBeetle — Protocol-Aware Deterministic Simulation Testing](https://tigerbeetle.com/blog/2026-08-20-protocol-aware-dst/) | 2026-08-20；协议/存储不变量、单副本与集群恢复进展 |
| C1 | [Cloudflare — Details of the outage on July 2, 2019](https://blog.cloudflare.com/details-of-the-cloudflare-outage-on-july-2-2019/) | 测试漏检 CPU、规则全局发布、恢复与应急访问、改进事项 |
| H1 | [James Bach — Heuristic Test Strategy Model](https://www.satisfice.com/download/heuristic-test-strategy-model) | 2024-12-02 的 v6.3 发布说明与四类视角；未把旧 v5.7.5 索引页当成最新版本 |
| H2 | [Michael Bolton — FEW HICCUPPS](https://developsense.com/blog/2012/07/few-hiccupps) | 2012 年正文及作者修订；预期来源、启发式局限与冲突 |

上述材料支持方法与取舍；具体范围、业务期限、容量目标和发布决定必须回到项目合同及实际证据。本文没有运行这些公司的测试，也没有验证其所有公开可靠性主张。
