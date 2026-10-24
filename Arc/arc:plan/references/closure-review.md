# 审查模式：以证据还原现有业务

读取 [Arc 协作合同](../../_shared/workflow.md) 和 [权限合同](../../_shared/contract.md)。
本阶段只读项目业务文件，仅在 `.arc/artifacts/<task-slug>/closure/` 写审查材料。
不运行 `init_docs.py`，不创建或更新 `docs/01-系统设计/业务闭环/`（旧项目沿用 `docs/arc/`），不复制工具，不追加 `AGENTS.md` / `CLAUDE.md`。
用户另行要求把报告归档进 docs 时，在后续 tasks / campaign 模式处理。

## 1. 收集材料

复用 `arc:explore` 的检索、调用链与影响面证据；不存在交接时按共享 code-intel 协议查询。
范围包括需求、角色枚举、状态字段、状态更新入口、定时任务、消息消费者、权限与资源释放路径。
每条结论保留精确 `arc-idx` 命令和已读取的 `file:line`。保存当前代码版本和未覆盖面。

## 2. 还原 as-is

写 `.arc/artifacts/<task-slug>/closure/as-is-spec.yaml`，格式见 [spec-format.md](closure-spec-format.md)。
这是审查快照，不替换 `docs/01-系统设计/业务闭环/03-closure-spec.yaml` 的设计合同。

- 以已确认实现与实际行为为证；文档和代码不一致本身是候选发现。
- 流转注释记录来源，例如 `# 来源: src/order/service.ts:120`，并链接原需求 ID。
- 无证据的出口、终态、角色不得补造。没有终态就写 `terminal: []`，保留 R1/R3 发现；禁止临时假终态。
- 原始需求不明时标记 `needs-triage`，推断不是业务合同；未找到代码不能断言功能不存在。
- 结构错误导致动态检查跳过时，动态模型结论记 `NOT_RUN`；不为获得 PASS 改造现状。

## 3. 运行模型检查

使用当前技能目录下的检查器，并显式指定两类输出路径。下列 `<skill-dir>` 和 `<task-slug>`
须替换为本次已确认路径；先创建所需 artifacts 目录。

```bash
python3 <skill-dir>/scripts/closure/check_closure.py check .arc/artifacts/<task-slug>/closure/as-is-spec.yaml --report .arc/artifacts/<task-slug>/closure/model-report.md --status .arc/artifacts/<task-slug>/closure/model-status.json
```

ERROR/WARN 是候选问题，逐条核对材料并尝试反证，保留脚本反例路径。模型 PASS 不代表系统已闭环、
实现完成或可发布；审查快照的 gate 也不为设计 spec 或实现授权提供凭据。

## 4. 人工核查与对照设计

按 [loophole-checklist.md](closure-loopholes.md) 核对 11 类，结果写 artifacts 下 `loopholes.md`。
多实例竞争、幂等、真实权限、事务提交、通知到达和对账需要运行时证据，交给 `arc:verify`；
静态推演无法确认的项标记 `manual-gap`，不能判为已验证。

如果设计 spec 已存在，比较需求 ID、状态、事件、流转和不变量：设计要求而已确认实现缺失，登记
“未实现”；存在但未登记的实现行为，登记“合同差异”。未搜索全或运行时分派未明时只写“待核实”。

## 5. 发现、结论与交接

用 [审查模板](../scripts/closure/assets/templates/06-audit-report.md) 写 artifacts 下 `audit-report.md`，
并写 `handoff.md`，字段以共享协作合同为准。发现沿用 Arc：

- 严重度：`Critical` / `High` / `Medium` / `Low` / `Info`，由实际影响决定。
- 状态：`confirmed` / `likely` / `needs-triage` / `false-positive` / `manual-gap`。
- `confirmed` 必须有实际复现或完整入口到结果的证据链，并记录反证检查；模型反例不等于产品漏洞已确认。
- 分开报告“模型检查”“材料覆盖”“运行时验证”的结果；缺材料是 `BLOCKED` 或 `NOT_RUN`，附原因。

交给 `arc:audit` 合并同根因发现与复核，转 prd / tasks 模式登记业务决策或拆任务；已授权的小范围已确认
实现缺陷可直接交 `arc:code`，其修复证据交 `arc:verify`。需要修改业务合同时先进入 model 模式的设计/变更。
本阶段不修复代码、不修改 canonical spec，也不把报告写进现有任务进度表。
