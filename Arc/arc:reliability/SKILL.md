---
name: arc:reliability
version: 1.1.0
description: >
  Engineers business reliability through risk-based scope, explicit invariants,
  real transaction/fault/recovery scenarios, reproducible evidence and release gates. Use for 可靠性,
  业务不变量, 事务并发, 发布证据, retry/idempotency verification, recovery drills,
  or incident prevention. Not for routine test-suite work, security scanning,
  direct production repairs, or deployment execution.
metadata:
  requires:
    skills:
      - arc:idx
      - arc:test
---

# arc:reliability

把业务承诺串成可追溯的验证链：**业务规则与后果 → 风险范围 → 失败模型 → 场景与独立判定 → 项目执行入口 → 真实隔离环境 → 执行证据 → 发布门禁 → 生产检测与事故学习**。

本技能负责可靠性判断与验证闭环。`arc:test` 负责平台原生测试的实现和执行；`arc:fix` 负责已确认缺陷的根因修复。普通小改动沿用原技能的验证，不为每次任务生成完整可靠性体系。

## Quick Contract

- **Trigger**: 业务不变量、事务并发、重复/超时/重试、部分成功、恢复演练或发布证据需要专门验证。
- **Inputs**: 项目、变更/事故范围、业务合同、目标版本与环境、现有测试和可用证据；未知项如实记录。
- **Outputs**: 风险与未测范围、规则到场景的映射、执行绑定、环境约束、证据状态、阻断项，以及必要的检测/恢复控制。
- **Quality Gate**: 每个必需场景都有能发现目标缺陷的可信判定和当前候选版本证据；缺失、失败或不稳定不能通过，测试数量/比例不能替代业务证明。
- **Decision Tree**: 先遵守任务范围，再按下面的 Intent Router 加载资料；工程分工见 [`Arc 路由`](../../docs/arc-routing-matrix.md)。

## Intent Router

| When | Load |
|---|---|
| 业务规则、状态、适用维度、变更影响与场景选择 | [`references/business-verification.md`](references/business-verification.md) |
| 风险测试范围、用例设计、判定器与生成器盲区 | [`arc:test 用例设计`](../arc:test/references/test-case-design.md) |
| 需要完整的业务用例推导演示 | [`references/worked-example.md`](references/worked-example.md) |
| 行业策略依据、公司实践差异及适用限制 | [`references/industry-testing-strategies.md`](references/industry-testing-strategies.md) |
| 共享状态、锁顺序、事务重试、异步与部分副作用 | [`references/transactions-and-faults.md`](references/transactions-and-faults.md) |
| 隔离环境、证据状态、CI 与发布门禁 | [`references/evidence-and-release.md`](references/evidence-and-release.md) |
| SLI/SLO、故障检测、恢复或事故防复发 | [`references/operations-and-learning.md`](references/operations-and-learning.md) |
| 需要一份本次变更的简短验证卡 | [`references/change-verification.md`](references/change-verification.md) |
| 写/运行具体测试、fuzz、性能基线 | `arc:test` |
| 诊断缺陷、建议行为修复 | [`Evidence-first root-cause repair`](../../docs/execution-truth.md#6-evidence-first-root-cause-repair)，然后 `arc:fix` |
| 只读项目体检 / 安全审计或扫描 | `arc:audit` / `arc:security`；保持只读或既有授权边界 |
| 业务定义存在决定性缺口 / 已明确的实现 | `arc:clarify` / `arc:build` |
| 大型或跨会话整改、发布与运维文档 | `arc:sdlc`；飞书仅在已启用时交 `arc:docs` |

只加载命中的资料。分工是技能切换，不是要求创建新 Agent 或新平台。

## Context Search

使用 `arc:idx` 完成身份、doctor、daemon 会话检查；所有仓内发现先走 `arc-idx files/search/symbol/ast`，再读取命中的文件。先找实际合同、入口/调用者、事务与副作用、现有测试/CI、发布与恢复记录，不能仅按函数名想象业务。

稳定规则、状态和领域标识来自当前项目合同、枚举、数据库列或用户文档。诊断/修复必须先加载上面的 **Evidence-first root-cause repair** 门禁；信号和设计疑点仍是待验证假设。

## Workflow

1. **定范围与规则。** 明确变更、目标版本/环境、允许的操作与业务后果。按风险选择正常、故障、恢复、升级阶段，检查配置、数据和运维工具的影响；标出本次必需项、未测范围及理由。业务定义不清记 `SPECIFICATION GAP`，保留独立可继续的验证工作。
2. **推导失败与判定。** 沿真实架构边界追共享不变量和同类路径。每个关键场景写 `EXPECTED`、`FORBIDDEN`、独立 oracle、安全性和有条件/有时限的活性；用边界、决策表、状态历史和故障点设计用例，说明哪个现实缺陷会触发断言。
3. **绑定最小可信执行。** 复用项目测试、fixtures、CI、环境与观测。按语义选择边界而非测试比例；数据库、消息或持久化主张需要对应真实证据。审视重要判定器与生成器盲区，必要时验证反例敏感性。经 `arc:test` 实现执行，需要的新接缝或产品修复交 `arc:build`/`arc:fix`。
4. **执行并保留证据。** 从最小关键场景开始；保存版本、环境、工具/命令、seed/schedule/corpus、实际终态、日志和重现方法。区分产品失败、验证器失败与环境阻塞；按事实给出 `PASS / FAIL / BLOCKED / NOT_RUN / FLAKY`。只读评估到此仍只评估已有材料，不启动写数据实验。
5. **判门禁与闭环。** 逐项核对本次必需场景及原项目门禁，列出缺失或失败证据、适用的检测/停止/回滚与恢复控制。可靠性结论是发布决策输入，不执行部署。关键事故回到不变量、回归、检测和恢复控制；达到约定证据与退出条件后停止扩展测试。

## Artifacts and Scope

保留四种逻辑职责：**业务规格、执行绑定、环境契约、运行证据**。它们可落在已有合同、测试选择器、CI 配置和制品中；不强制四个新文件，更不复制源技能的整套 TOML/Python runner。

稳定业务知识留在项目已有需求/架构/测试文档；普通变更使用简短验证卡或会话记录。大型、跨会话或需持续跟踪的工作才用 `arc:sdlc` 的 `docs/03-执行任务` 与进度表，任务卡包含精确 `<Context-Queries>`。原始需求仍在 `docs/00-产品需求`。

运行证据复用 CI 制品或 `.arc/artifacts/<task>/reliability/`，脱敏且不默认提交。数据库/缓存/队列/索引/对象存储的测试与同步配置使用 `.xxxx.json` 隐藏文件，明确区分独立测试环境与同步测试环境；字段和种子来自项目，不套固定账号、表名或 key。任务/API 合同仍用 Markdown。

## Red Lines

```text
NO BUSINESS PASS FROM HTTP 200, ROW COUNT, COVERAGE, OR A QUALITY SCORE ALONE.
NO UNIVERSAL TEST PROPORTIONS OR LOW PRIORITY ASSIGNED TO A TEST TYPE.
NO AUTOMATION CLAIM WITHOUT A RUNNABLE BINDING AND OBSERVED BUSINESS ORACLE.
NO MOCK-ONLY PROOF OF REAL DATABASE, BROKER, OR DURABILITY SEMANTICS.
NO PASS FOR MISSING, STALE, SKIPPED, FLAKY, OR BROKEN-OBSERVER EVIDENCE.
NO RETRY-UNTIL-GREEN, WEAKENED ASSERTIONS, OR PARTIAL-TRANSACTION RETRY.
NO REPAIR FROM AN UNCONFIRMED HYPOTHESIS; USE THE ROOT-CAUSE GATE.
NO LOCAL POSTGRES SERVER OR LOCAL CONTAINER RUNTIME BY DEFAULT.
NO UNAUTHORIZED SHARED-TARGET LOAD, DESTRUCTIVE RESET, CHAOS, OR DEPLOYMENT.
NO INVENTED DOMAIN KEYS, BUSINESS RULES, SLOS, OR WAIVERS.
```

本机默认只用客户端；Docker 经用户指定服务器的 context/SSH 执行，PostgreSQL 使用远端已有测试实例或 CI 隔离环境，本机 SSH tunnel 仅绑定回环地址。禁止为取证安装/启动本机 PostgreSQL 服务端或容器运行时；本机例外必须是用户当前任务明确点名。环境不足时继续可完成的工作，并给出准确阻塞项。

沿用当前任务的有效授权，不反复索取确认。共享/第三方目标的负载、故障注入或破坏性操作需要覆盖目标、操作和数据范围的授权及停止/恢复条件；等待必要授权期间只做可独立完成的工作。禁止因“可靠性”扩大只读、只写文档或禁止重启的范围。

## Quality Gates

- 必需场景追溯到真实业务规则和失败后果；预期与禁止结果有独立观测，关键断言能解释要发现的缺陷，未测范围与规格缺口明确。
- 执行、判定器与环境证据匹配当前候选版本/配置；验证器无法观察时不给产品通过结论。
- 涉及并发/重试时检查业务终态与副作用；安全性、活性及性能预算分别判定。
- 所有必需场景逐项满足门禁；重跑或隔离 flaky 不删掉必需主张，风险接受与测试状态分开，SLO 预算不能豁免资金、数据或权限不变量。
- 修复由 `arc:fix` 完成并验证原失败及相关同类路径；大型任务真实状态同步，未运行不勾 `[x]`。

## When to Use

用于支付/库存/配额等业务承诺、跨操作共享状态、重复与未知提交结果、迁移/恢复、可靠性发布评估和事故防复发。由这些具体风险触发，不因“需要写代码”或“加个测试”就启用全流程。

普通测试建设用 `arc:test`，已知故障修复先用 `arc:fix`，通用只读体检用 `arc:audit`，扫描/攻击面验证用 `arc:security`。本技能补足业务可靠性语义，不替换这些技能，也不成为独立部署技能。

## Outputs

```text
Reliability Handoff
- Scope: affected promises/risks, required and excluded scope, candidate/environment, authorized operations
- Claims: applicable lifecycle phases, EXPECTED/FORBIDDEN, independent oracle, defect sensitivity, specification gaps
- Verification: scenario -> binding -> real boundary -> evidence status/artifact
- Gate: required scenarios, missing/failing evidence, accepted risks kept separate
- Follow-through: detection/recovery controls, arc:test/fix/sdlc handoff when needed
```

吸收来源：用户提供的 `reliability-engineering-skill` v3，以及 Google、AWS、Netflix、Uber、Cloudflare、FoundationDB、SQLite、TigerBeetle 和测试专家的一手材料。方法差异与证据边界见 [`行业策略`](references/industry-testing-strategies.md)，原始来源适配见 [`operations-and-learning.md`](references/operations-and-learning.md#吸收来源与适配)。来源目录不作为运行时依赖。
