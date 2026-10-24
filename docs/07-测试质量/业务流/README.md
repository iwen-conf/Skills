# ARC 业务流总清单

本目录是 ARC 技能仓自身可执行工具链的业务流目录，格式与维护规则见
[flow-model](../../../Arc/arc:verify/references/flow-model.md) 与
[flow-matrix](../../../Arc/arc:verify/references/flow-matrix.md)，由 `arc-verify` 的 flow 模式维护。

- ARC 的“业务”是把技能交付给 Agent 宿主，并在 Agent 工作的关键节点把关（安装、宿主链接、模型闸门、编码门禁、
  发布门禁）。本仓没有 HTTP 服务、消息队列或租户；这些组件不存在，不为套模板而补画。
- 结构检查：`.venv/bin/python Arc/arc:verify/scripts/flow_ledger.py check docs/07-测试质量/业务流 --json`。
- 证据列中的 `logs/…` 指 `.arc/artifacts/arc-business-flow-testing/logs/`（不入库）；复现命令见
  [T09 任务记录](../../03-执行任务/01-技能发布与规则治理/tasks/T09-业务流测试体系/README.md)。
- 候选版本：`ff3ef7b` + 未提交工作树，2026-10-14。

## 清单

| ID | 名称 | 类型 | 目标 | 入口 | 主要模块与依赖 | 正常 / 异常终点 | 文件 | 风险 | 待确认 |
|---|---|---|---|---|---|---|---|---|---|
| FLOW-001 | 技能打包安装与核验 | 主业务流（含恢复子路径 N08-B02→N09） | 技能源打成完整包装入 `~/.agents/skills` | `scripts/sync_skills.py:341` | build/publish/validate；文件系统 | 安装、dry-run、核验通过 / 拒绝、构建失败、回滚、回滚二次失败（报出备份路径） | [FLOW-001](FLOW-001-技能打包安装与核验.md) | High | 备份保留策略 |
| FLOW-002 | 宿主链接治理 | 主业务流（含恢复子路径 --apply） | 3 个宿主根只含 profile 声明的链接 | `scripts/link_hosts.py:57` | host_profiles.yaml；宿主技能根；`~/.skills-backup` | 一致、已修复 / 漂移报告、无法修复、中途异常（报出备份路径） | [FLOW-002](FLOW-002-宿主链接治理.md) | High | missing-source 是否整体拒绝 |
| FLOW-003 | 业务闭环模型闸门 | 主业务流 | 严格模型检查通过且未改动才放行实现 | `Arc/arc:plan/scripts/closure/check_closure.py:825` | 规则 R1–R20、状态空间穷举；状态记录 | check 通过、gate PASS、render / check 失败、诊断放宽、闸门关闭 | [FLOW-003](FLOW-003-业务闭环模型闸门.md) | Critical | 无 |
| FLOW-004 | 编码完成门禁 | 主业务流 | 红灯先于生产代码；完成前零 FAIL | `Arc/arc:code/scripts/gate.sh:1217` | baseline、red 记录、各语言检查 | baseline、断言红灯、零 FAIL / 拒绝、build 红灯、FAIL | [FLOW-004](FLOW-004-编码完成门禁.md) | Critical | red 命令无超时 |
| FLOW-005 | 数据库发布门禁 | 主业务流 | 证据绑定运行与候选后按策略判定 | `Arc/arc:verify/scripts/dbqual/scripts/release_gate.py:105` | sqlverify_common；run_suite；测试 PostgreSQL | PASS / FAIL（含 invalid_policy、empty_policy） | [FLOW-005](FLOW-005-数据库发布门禁.md) | Critical | 无 |
| FLOW-006 | 技能仓校验门禁 | 辅助业务流 | 本仓 CI：frontmatter、标题、预算、链接、加载上限、退役名 | `scripts/validate_skills.py:21` | src/arc_core 校验 | 通过 / 错误列表 | 未建模 | High | — |
| FLOW-007 | 技能注册表生成 | 辅助业务流 | 生成 `skills.index.json` | `scripts/build_skills_index.py:15` | skill_registry | 写入或保持时间戳 | 未建模 | Low | — |
| FLOW-008 | 被测服务编排 | 辅助业务流 | 起服务、就绪探测、跑测试、退出码透传 | `Arc/arc:verify/scripts/with_server.py:117` | 子进程、端口、就绪 URL | 测试退出码 / 125 环境失败 | 未建模 | Medium | 命令不存在（127）无用例 |
| FLOW-009 | 安全扫描 | 辅助业务流 | 运行已安装扫描器并汇总报告 | `Arc/arc:audit/scripts/security-scan.py:720` | Semgrep 等外部工具 | 报告 / 工具缺失 | 未建模 | Medium | 无自动化测试 |
| FLOW-010 | 代码检索索引 | 辅助业务流 | zoekt + ctags 索引、守护进程、检索 | `tools/arc-idx/main.go:41` | zoekt、ctags、ast-grep | 健康 / 过期、守护进程失活 | 未建模 | Medium | 无 Go 测试 |
| FLOW-011 | 路由探针 | 辅助业务流 | 用 Jev 对 description 做路由回归 | `scripts/route_probe.py:102` | jev CLI、API key | 每条语料 top-1 / 无密钥失败 | 未建模 | Low | 需外部服务 |
| FLOW-012 | 隐私标签过滤 | 辅助业务流 | 过滤 `<private>` 内容 | `src/arc_core/privacy.py:115` | — | 掩码 / 删除 / 抽取 | 未建模 | Medium | — |
| FLOW-013 | 飞书技能导入 | 辅助业务流 | 从 lark-cli 导入技能源 | `scripts/import_lark_skills.py:25` | lark-cli 仓 | 导入 / 源缺失 | 未建模 | Low | 无自动化测试 |
| FLOW-014 | 闭环文档初始化 | 辅助业务流（FLOW-003 前置） | 初始化 `docs/01-系统设计/业务闭环/` 与检查器副本 | `Arc/arc:plan/scripts/closure/init_docs.py:33` | 模板、检查器 | 初始化 / 刷新 / 旧路径沿用 | 未建模 | Medium | — |
| FLOW-015 | Arc 阶段交接 | 跨流程业务流 | explore → plan → code → verify 保留范围、授权、ID 与证据 | `Arc/_shared/workflow.md` §1–§5（Agent 规程，无代码入口） | 五个 Arc 技能 | 下一阶段接手 / 交回负责阶段 | 未建模 | High | 只有静态语料，无宿主运行时证据 |
| FLOW-016 | 测试先行交接 | 跨流程业务流 | verify red → FLOW-004 red → code 转绿 → verify 复跑 | `Arc/_shared/test-first.md` §5 | FLOW-004 | 同一测试先红后绿 | 未建模 | High | 依赖 FLOW-015 |

## 覆盖情况（`flow_ledger.py check --json`，2026-10-14，修复后）

| 业务流 | 分支 | 已设计 | 已绑定 | PASS | FAIL | NOT_RUN | 缺口终点 | 未 PASS 的高风险分支 | 剩余风险 |
|---|---|---|---|---|---|---|---|---|---|
| FLOW-001 | 18 | 18 | 18 | 18 | 0 | 0 | — | — | 回滚二次失败需人工恢复；备份无保留策略；并发安装未处理 |
| FLOW-002 | 8 | 8 | 8 | 8 | 0 | 0 | — | — | 只在临时目录验证，未对真实宿主 `--apply` |
| FLOW-003 | 21 | 21 | 21 | 21 | 0 | 0 | — | — | 模型闸门不证明需求或运行时行为 |
| FLOW-004 | 28 | 28 | 28 | 28 | 0 | 0 | — | — | red 命令无超时；P5 内各语言检查未展开成图 |
| FLOW-005 | 8 | 8 | 8 | 8 | 0 | 0 | — | — | 真实 PostgreSQL 上的 run_suite 为 BLOCKED |

- “已设计”= 用例列有编号；“已绑定”= 有测试选择器或探针；PASS 只表示该分支的断言在本候选上通过，不表示业务无缺陷。
- 未建模的 11 条业务流没有流程图和分支台账；其中 FLOW-006、FLOW-008、FLOW-012、FLOW-014 已有自动化测试（见 `tests/`），
  FLOW-009、FLOW-010、FLOW-013 没有。
- 新增用例的敏感性：每条为已有守卫补的测试都在一次性副本里对目标代码做变异并确认失败，记录在 `logs/sensitivity-flow00*.txt`。
