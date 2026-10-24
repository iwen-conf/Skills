# 设计与变更：先建立可核查的业务合同

读取 [Arc 协作合同](../../_shared/workflow.md)。本阶段只写业务文档与 spec，不写业务代码；
完成后在现有授权内切换阶段，不能把“需切换技能”解释成每次重新请求用户授权。

## 0. 复用现有来源

先按 code-intel 检索现有 PRD、业务合同、canonical spec、任务入口及 tracker，再读取文件。
原始需求由 prd 模式管理；`docs/01-系统设计/业务闭环/03-closure-spec.yaml` 是机器可读的业务模型，
不覆盖用户需求、API 合同或实际运行证据。需求来源相冲突时记录差异，不擅自改业务。

- 已有 PRD：`01-requirements.md` 只链接其章节并复用原 REQ ID，不复制一套需求正文。
- 已有任务和 `进度跟踪表.md`：在 `README.md` 链接，不另创 closure 进度表。
- 已有 spec：增量设计；没有才运行 `python3 <skill-dir>/scripts/closure/init_docs.py <项目根目录>`。
- `--with-rules` 只用于用户授权写项目常驻规则的情况；审查模式不得使用初始化器。
- 项目内检查器副本旧于当前安装版本时，使用当前技能脚本，或运行初始化器的 `--refresh-tool`
  更新该副本后重新 `check`；旧版本记录不能跨检查器更新复用。

## 1. 明确业务范围与决策

复用本会话已有目标、scope、实现授权和上游交接。列出用户、角色、对象、外部依赖、不可违反的
业务断言和本期不做。领域键只从项目来源读取。

缺失信息登记到“假设与待确认”，但不得用猜出的角色、支付规则或状态合同解锁依赖它的实现。
只对会改变业务结果的未决假设、豁免和范围决策请求确认；已给出的信息和授权不重复询问。
无关部分可继续设计。已有明确实现授权且决策均已解决，模型通过后继续交接实现。

## 2. 建立模型

依 [spec-format.md](closure-spec-format.md) 建 actors → entities → events → resources；参考模板只借用格式。
需求 ID 沿用来源。逐实体覆盖主路径、取消、拒绝、超时、失败及补偿；记录 actor、event、emits、
feedback、acquires/releases。跨实体不变量使用稳定 INV ID，每个需求有场景、每条流转可追溯。
任务里以 REQ / INV / transition / scenario ID 引用模型，不复制状态机。

## 3. 模型检查与人工推演

```bash
python3 docs/01-系统设计/业务闭环/tools/check_closure.py check docs/01-系统设计/业务闭环/03-closure-spec.yaml
python3 docs/01-系统设计/业务闭环/tools/check_closure.py gate docs/01-系统设计/业务闭环/03-closure-spec.yaml
```

严格 gate 要求：spec 和检查器指纹匹配，零未豁免 ERROR/WARN，动态模型已执行且未截断。
`--allow-warn` 仅供诊断，不能开启 gate；缺失/损坏/旧检查记录须重跑。修复 spec 后重跑。
豁免必须有具体业务依据及确认记录，不能把跳过检查或截断说成完整证据。

逐类执行 [loophole-checklist.md](closure-loopholes.md)，登记 `05-loophole-register.md`。
模型覆盖和运行时验证分列：场景写进 spec 不等于产品测试已通过；并发、数据库、恢复等真实断言
连同失败模型、独立 oracle 和目标环境交 `arc:verify`，由其 flow 模式把场景和流转映射到业务流分支。

## 4. 同步与交接

使用 `render` 更新 `02-business-design.md` 状态图；更新 `README.md` 和 `changelog.md`。
交接包含共享合同规定的范围/授权、来源与 ID、spec 路径和哈希、checker 指纹、check/gate 命令结果、
影响文件及精确 `<Context-Queries>`、未决事项、下一技能和其验收条件。

- 已有明确实现授权 + gate 通过 + 相关业务决策已确认：同一任务继续 `arc:code`，无需重复确认。
- 用户仅要求设计/文档：交付设计，说明尚未实施，不自动写代码。
- 需要任务拆分：转 tasks 模式，沿用原需求和唯一 tracker；任务完成仍需实现与目标面验证。
- 运行时检查发现实现偏差：返回 `arc:code`；发现合同缺口：回到 model 模式修订，旧 gate 随之失效。

## 变更模式

先用 `arc:explore` 查影响面，比较已有合同及在途数据；将变更映射到 REQ、实体、状态、事件、
资源、不变量和场景。先改 spec、重跑模型检查及人工推演，再同步相关文档和任务引用。
变更记录写清原因与受影响 ID。数据迁移和旧新版本并存的真实验证交 `arc:verify`；模型通过不替代
数据库资格或发布门禁。代码不得暗含未经合同登记的业务状态/流转/事件。
