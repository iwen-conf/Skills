# ARC 测试技能业务流审计（2026-10-14）

范围：本仓 ARC 技能与测试相关规则，以及 ARC 自身可执行工具链的业务流。候选版本 `ff3ef7b` + 未提交工作树
（会话开始时已有 199 处改动，均保留）。检索全部经 `arc-idx`。修订落地见
[T09 业务流测试体系](../03-执行任务/01-技能发布与规则治理/tasks/T09-业务流测试体系/README.md)。

## 1. 盘点结果

| 文件 | 职责 | 与测试的关系 |
|---|---|---|
| `Arc/arc:verify/SKILL.md` + 15 份 references | 唯一测试入口：design / red / write / reliability / db / flaky / release | 主审对象 |
| `Arc/arc:verify/references/case-design.md` | 断言 → 用例卡 → 预言机 → 敏感性 | 用例设计核心 |
| `Arc/arc:verify/references/scope-cadence.md` | 变更类型 → 最低证据、测试边界、节奏 | 范围选择 |
| `Arc/arc:verify/references/txn-faults.md`、`worked-example.md` | 事务、并发、幂等、故障与恢复 | 异常与恢复 |
| `Arc/arc:verify/references/test-value.md`、`web-e2e.md`、`api-contract-tests.md` | 测试价值、浏览器旅程、HTTP 边界 | 分层测试 |
| `Arc/_shared/test-first.md` | 红灯先于代码（verify red 与 code 共用） | 测试先行 |
| `Arc/_shared/contract.md`、`workflow.md` | 权限、判定词表、阶段交接、产物归属 | 约束与交接 |
| `Arc/arc:plan`（model 模式）+ `scripts/closure/` | 业务闭环 spec、R1–R20 检查器、11 类漏洞推演 | 生命周期模型，运行时断言交 verify |
| `Arc/arc:code/scripts/gate.sh` | 完成门禁、红灯记录 | 执行层门禁 |
| `schemas/trigger_corpus.yaml`、`behavior_corpus.yaml`、`src/arc_core/domain/skill.py` | 路由词表、运行时场景、加载上限 | 技能本身的回归 |
| `docs/archive/reliability-engineering/` | 已归档的旧可靠性技能（含“业务优先测试规划”） | 对照来源 |
| 根目录 `所有测试.md` | 测试类型名词解释 | 未被任何技能引用，不参与执行 |

垂直家族（Android `testing-setup`、HarmonyOS 测试、agent-browser QA）各有原生测试指引，测试设计口径不归它们，
本次不改。宿主入口按 `schemas/host_profiles.yaml`：arc-verify 在 Claude、Codex（core profile）和 Grok 中可发现，
Gemini 与 Pi 不装 arc-verify；本次不扩展宿主配置。

## 2. 能力评估（修订前）

| # | 能力 | 结论 | 依据 |
|---|---|---|---|
| 1 | 先理解业务再写测试 | 部分具备 | test-value §1 要求每个测试回答“哪条业务规则”；但工作流从断言表开始（原 SKILL.md 步骤 2），没有业务调查步骤 |
| 2 | 识别完整业务流 | 尚未具备 | verify 没有业务流清单、ID 或类型；case-design §1 只把“其他入口”用来定回归范围 |
| 3 | 从入口、服务、存储、消息、定时任务还原链路 | 部分具备 | plan closure-review §1 会从代码还原状态字段、更新入口、定时任务和消费者，但目的为生命周期缺口，verify 不调用 |
| 4 | 业务流转为流程图 | 部分具备 | 只有 `check_closure.py render` 输出单实体 `stateDiagram-v2`（:810），没有带判断节点、分支和终点的流程图 |
| 5 | 正常、异常、边界、状态转换 | 部分具备 | case-design §2 五个阶段（Normal/Boundary/Fault/Recovery/Change）与 stateful 用例要求；没有逐分支落到终点的要求 |
| 6 | 跨接口、跨模块、跨服务的端到端流 | 部分具备 | scope-cadence §2 E2E 用于“critical journeys”，worked-example 演示一条；没有“每条流主路径必须有 E2E”的规则 |
| 7 | 全流程数据一致性 | 部分具备 | case-design §5 精确预言机、守恒不变量；没有执行前后存储快照比对的步骤 |
| 8 | 前后状态变化与最终结果 | 部分具备 | 原则在（“绝不能只看 200”），步骤不在 |
| 9 | 重复、并发、超时、重试、部分成功、补偿 | 已经具备 | txn-faults §0–§6、worked-example S2–S4，“超时 ≠ 未执行”明确 |
| 10 | 失败后数据未被错误修改、重试可恢复 | 部分具备 | 权限拒绝后“零持久变化”只写在 scope-cadence 权限行；没有推广为所有拒绝/失败分支 |
| 11 | 通过与失败判定标准 | 已经具备 | 五值判定词表、敏感性证明、`BLOCKED`/`NOT_RUN` 必须带原因 |
| 12 | 未覆盖分支转为后续任务 | 部分具备 | FAIL → 修复包、SPECIFICATION GAP → 用户、延期项写原因；没有分支级缺口清单和去向表 |

另见：变更后识别受影响业务流（部分具备，case-design §1 末条 + scope-cadence §7，但没有持久目录可查，每次重新推导）。

## 3. 问题清单

| 分析维度 | 当前实现 | 存在的问题 | 改进方向 | 优先级 |
|---|---|---|---|---|
| 业务流发现 | 断言表来自“合同、文档、代码”，标 `derived` | 测试单位是断言而不是流程；接口列表直接生成用例不被禁止 | flow 模式：从真实入口追踪、建 `FLOW-NNN` 清单，T1/T2 先建流再写断言 | 高 |
| 流程图建模 | 仅闭环检查器的状态图 | 无判断节点、条件分支、终点、补偿路径 | Mermaid 子集 + 分支编号 + 终点闭环类型；未实现的补偿画成缺口终点 | 高 |
| 正常闭环测试 | E2E 为可选边界 | 没有“每条流主路径”的要求 | 分层规则：每条流主路径与高风险分支走真实入口 | 高 |
| 异常闭环测试 | 故障与恢复阶段齐全 | 拒绝、失败、恢复、异步没有逐分支的终点与断言定义 | 六种闭环类型及各自必须断言的结果 | 高 |
| 状态与数据一致性 | 精确预言机原则 | 缺执行前后快照比对 | flow-matrix §4 前后状态差分 | 高 |
| 测试执行与追踪 | 需求 → 规则 → 用例 → 证据 | 不到流程节点与分支；无台账；“已写测试”与“已覆盖”无法区分 | 分支台账（用例、绑定、判定、证据）+ `flow_ledger.py` 机器检查 | 高 |
| 变更回归 | 每次重新推导影响面 | 无持久目录 | 目录文件记录节点 `file:line`，改动即 `stale`，回归集 = 受影响流的绑定测试 | 中 |
| 维护 | SKILL.md 3497/3500 token；pg-reliability 加载 15433/15500 | 新规则只能压缩旧文字换取空间 | 新增两份 reference，SKILL.md 只放顺序与红线；去掉与引用文档重复的规则 | 中 |
| 路由 | “业务闭环/闭环检查/状态机” → arc-plan | “业务流测试/分支覆盖”无触发词 | 新触发词进 description 与语料，闭环模型仍归 plan | 中 |

重复与冲突：追溯链在 case-design §3 与归档文档各一份（不冲突）；拒绝后零变化只在权限一行（已推广）；
plan 的闭环模型与 verify 的业务流此前没有映射规则（已在 flow-model §6 和 workflow §4.5 写明：流程引用模型 ID，
不重定义状态，差异交 plan 审查）。未发现相互矛盾的规则。

## 4. 修订方案（已实施）

1. `arc-verify` 新增 flow 模式与两份参考：[flow-model](../../Arc/arc:verify/references/flow-model.md)（目录、从入口发现、
   作图规则、闭环类型、流程卡、与闭环模型的关系）和 [flow-matrix](../../Arc/arc:verify/references/flow-matrix.md)
   （风险排序、A–H 维度、分支 → 用例、前后差分、台账、分层、变更影响、缺口去向）。
2. 工作流第 2 步强制“先流后断言”；禁止 T1/T2 按接口/函数/文件列表生成用例；无实现的补偿只能画成缺口终点。
3. 新增 `Arc/arc:verify/scripts/flow_ledger.py` 与 `tests/test_flow_ledger.py`（17 项，含 12 种结构缺陷的变异用例）。
4. 共享合同授予 verify 维护 `docs/07-测试质量/业务流/`；workflow 增加归属行与“现有行为补测试”路径；
   plan 的 closure-design 与 docs-layout 指向 flow 模式。
5. 单元、集成、E2E、可靠性、db、flaky、release 原有能力全部保留。
6. 用 ARC 自身工具链落地：[业务流总清单](../07-测试质量/业务流/README.md)，5 条核心流建模、16 条登记。
