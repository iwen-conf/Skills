# 业务闭环文档索引

- 系统：<系统名>
- 当前阶段：<设计 / 变更 / 已交接实现>
- 原需求：<既有 PRD 路径与章节；复用其 REQ ID>
- 执行任务与唯一进度表：<既有任务入口及 tracker；不存在写“未建任务”>
- 模型闸门：<PASS / FAIL / BLOCKED / NOT_RUN + 日期与原因>
- 业务决策与实现授权：<本会话已有依据、范围、未决项>
- 运行时验证：<PASS / FAIL / BLOCKED / NOT_RUN + 证据；模型通过不得自动填 PASS>

| 文件 | 内容 | 维护方式 |
|---|---|---|
| 01-requirements.md | 原需求引用、REQ 映射、假设 | 复用原 PRD，不复制需求账本 |
| 02-business-design.md | 人读模型与业务设计 | 与 spec 同步 |
| 03-closure-spec.yaml | 机器可读业务模型 | 不覆盖原需求或运行时事实 |
| 04-closure-report.md | 模型检查报告 | 脚本生成 |
| 05-loophole-register.md | 人工推演、决策及运行时缺口 | 引用来源和验证证据 |
| changelog.md | 模型变更记录 | 记录受影响 ID 和哈希 |

审查快照与报告只放 `.arc/artifacts/<task-slug>/closure/`；不要把审查结果直接写入设计模型。

## Arc 交接

- 当前/下一技能：<arc:plan model → arc:plan tasks / arc:code / arc:verify；交接原因>
- 目标、范围和不做：<继承上游范围与授权，不能由 gate 推导授权>
- 来源：<REQ / INV / transition / scenario ID 与路径>
- 模型证据：<spec 路径和 SHA256、checker SHA256、check/gate 命令及结果、状态文件>
- 影响面：<代码文件和已核实调用点；复用 explore 结果>
- 上下文检索：<执行者需复跑的精确 arc-idx 查询>
- 下游验收：<任务卡/测试场景、独立 oracle、目标环境与命令>
- 未决项：<假设、豁免、范围决策；无则写“无”>
- 返回条件：<实现偏差返 code；合同缺口返 closure；证据缺口返 verify>

已有明确实现授权且模型 gate 通过、相关决策已解决时继续实现，不重复等待确认。
仅要求设计时在本阶段交付；模型结果不代表任务完成或发布资格。
