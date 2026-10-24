# T09 业务流测试体系

- 状态：`[x]`（技能修订、检查器与 ARC 业务流目录已落地并验证；第二轮修复了遗留 1–5；6–11 见下文，未安装到宿主）
- 来源：用户 2026-10-14 任务“全面审查并重构 ARC 项目的测试 SKILL，建立以闭环业务流为核心的测试体系”；
  设计依据 [ARC 测试技能业务流审计](../../../../01-系统设计/03-ARC测试技能业务流审计.md)
- 范围：`Arc/arc:verify`（SKILL、flow-model、flow-matrix、case-design、flow_ledger.py、agents）、`Arc/_shared/contract.md`、
  `Arc/_shared/workflow.md`、`Arc/arc:plan/references/{closure-design,docs-layout}.md`、`Arc/README.md`、触发与行为语料、
  `src/arc_core/domain/skill.py`（加载上限）、`tests/test_flow_ledger.py`、`docs/07-测试质量/`、本任务记录、CHANGELOG
- 不做范围（第一轮）：修复业务代码（release_gate 缺陷只登记修复包）；安装到 `~/.agents/skills` 与宿主；垂直家族测试技能；
  为未建模业务流批量写测试
- 第二轮（用户 2026-10-14“修复，依次进行修复”）：按遗留表顺序修复 1–5；需要用户定数值或会删数据的 6、7、11 不在本轮

## 完成内容

| 交付物 | 结果 | 证据 |
|---|---|---|
| A 审计报告 | [03 ARC 测试技能业务流审计](../../../../01-系统设计/03-ARC测试技能业务流审计.md)：盘点、12 项能力评估、问题清单、方案 | 每条结论附 `file:line` 或检索依据 |
| B 业务流总清单 | [业务流总清单](../../../../07-测试质量/业务流/README.md)：16 条（主 5、辅助 9、跨流程 2），5 条已建模 | `flow_ledger.py check` 0 错误 |
| C 流程图 | FLOW-001～005 各一张 Mermaid 图，共 83 个分支、34 个终点，缺口终点 3 个 | 同上；负向对照删除一条分支后报 4 个错误、退出 1 |
| D 测试矩阵 | 每条流：终点表、分支台账（用例、绑定、判定、证据）、用例卡、A–H 维度适用性 | 绑定测试实跑：FLOW-001 10 passed、FLOW-003 selftest 全部通过、FLOW-004 11 passed、FLOW-005 42 OK |
| E 技能修订 | arc-verify 3.1.0：flow 模式、先流后断言、禁止按接口列表生成用例、缺口终点规则；两份新 reference；台账检查器 | `validate_skills.py` 通过；SKILL.md 3486/3500 token；`verify-flow` 加载 13,506/14,000 |
| F 遗留与计划 | 下文 | — |

门禁：`validate_skills.py` 通过（16 个可发现入口、48 个库条目）；`pytest tests -q` 162 passed、1 skipped（R8 protobuf 环境，既有跳过）；
`gate.sh check --strict --require-red --scope` 共 28 个变更文件：baseline、placeholders、suppressions、lint、tests、red-first、scope 均 PASS；format(python) FAIL 仅剩 `src/arc_core/domain/skill.py`，HEAD 版本同样不过 `ruff format`（既有紧凑写法，本次 5 行沿用该写法，未整体重排）；red-hygiene WARN 为 `ruff format` 改了测试文件空白，19 条断言与 12 个变异用例未变；Arc 打包到临时目录后 `sync_skills.py --check` 通过，安装包内的 `flow_ledger.py` 可运行。

## 第二轮修复（2026-10-14）

| # | 修复 | 红灯 → 绿灯 | 敏感性 |
|---|---|---|---|
| 1 | `release_gate.py` 按 `schemas/gate-policy.schema.json` 检查策略，零断言记 `empty_policy`、schema 外的段或字段与非列表段记 `invalid_policy`，均 FAIL 退出 2；`qualification_coverage_gate.py` 无 claim 时 FAIL | `test_policy_that_checks_nothing_fails_closed`、`test_coverage_manifest_without_claims_fails_closed`：修复前 7 个子用例 `(0, 'PASS')`、1 个崩溃（`.arc/artifacts/fix-flow-gaps/before-D1.txt`），修复后通过 | 红灯本身即敏感性记录 |
| 2 | 新增 `tests/test_link_hosts.py`（5 例）；`link_hosts.py --apply` 遇 OSError 停在出错条目、报出备份路径、退出 1 | 失败用例修复前为未捕获 OSError | 删除代替备份、不建链、错目标检测失效三个变异各杀死 2 例 |
| 3 | `tests/test_sync_skills.py` 增加目标与源仓重叠（仓内与父目录两例） | 守卫既有，补特征测试 | 去掉守卫后 2 例失败 |
| 4 | 同文件补 `--also-root` 缺 `--check`、空选择、dry-run；closure selftest 补非法参数、不可读 spec、render；gate 测试补 red 无 baseline、baseline HEAD 丢失（red 与 check）、命令 127 | 守卫既有，补特征测试 | 每个变异都被对应用例杀死（`logs/sensitivity-flow00*.txt`） |
| 5 | `sync_skills.publish_installation` 回滚本身失败时抛出含 `rollback failed` 与 `previous entries remain in <backup>` 的 OSError | 修复前报错只有原异常文本 | 红灯即敏感性记录 |

结果：5 条已建模业务流 83 个分支全部 PASS（`flow_ledger.py check` 0 错误、0 缺口终点、0 个未 PASS 的高风险分支）；
dbqual 75 项单测（1 项真实工具用例按既有规则跳过）与 selftest 通过；包内 `CHECKSUMS.sha256` 全部校验通过，
MANIFEST 与 BUILD-VALIDATION 记录本次本地修订；`gates-release.md`、`db.md`、`docs/dbqual/README.zh-CN.md` 写明新失败类型及 Arc 判定映射。

第二轮门禁：`validate_skills.py` 通过；`pytest tests -q` 177 passed、1 skipped（既有 R8 跳过）；Arc 打包到临时目录后
`--check` 通过，安装包内 dbqual selftest PASS。`gate.sh check --strict --require-red --scope`（task `fix-flow-gaps`）：
baseline、placeholders、suppressions、tests、red-first、scope PASS；三项 FAIL 均非本轮引入：
format(python) 的 hunk 大多落在未改动的行（这些文件在基线即不过 `ruff format`，本轮新增代码沿用所在文件写法，新文件已格式化）；
mypy 报 `yaml` 缺类型桩（环境未装 `types-PyYAML`，安装需授权）；dbqual mypy `release_gate.py:100` 与 HEAD 第 72 行同一条既有错误。

## 已确认缺陷与修复包

**D1 `release_gate.py` 空策略放行（High，confirmed）**

```text
Observed: 策略为 {} 或键名拼错（required_evidnce）时，release_gate.py 输出 status PASS、退出 0，
  即使证据目录里有绑定本次运行的 FAIL 记录；正确键名时同一证据 FAIL、退出 2。
Expected: 没有任何断言被检查 → BLOCKED/非零退出（arc-verify 判定规则“零比较即 BLOCKED”；同包 canary_evaluator.py
  对无指标策略判 BLOCKED 并有测试 test_policy_without_metrics_is_blocked；regression_compare 对空基线判 BLOCKED）。
Surface: 本机，ff3ef7b + 工作树，dbqual v4.0.0。
Evidence: .arc/artifacts/arc-business-flow-testing/logs/probe-release-gate-{empty,typo,ok}-policy.json、probe-release-gate-exits.txt。
Root cause: evaluate()（release_gate.py:42-102）对 required_evidence、metric_rule 都用 .get(…, []) 缺省，status 只由 failures 是否为空决定；
  main()（:105）不校验策略键，未知键被静默忽略。
Affected siblings: qualification_coverage_gate.evaluate_coverage 对 {} 或拼错的 claims 键同样零失败（函数级已确认，
  probe-coverage-gate-empty-manifest.txt；仅 strict_complete: true 时关闭）；baseline_promote 只看 gate status，会接受空策略 PASS。
Complete fix boundary: release_gate 与 qualification_coverage_gate 拒绝未知顶层键、零断言时判 BLOCKED 并非零退出；
  dbqual tests 增加空策略、拼错键两例；更新 MANIFEST/CHECKSUMS/BUILD-VALIDATION；db.md 说明退出码。
Verification: 新用例先红后绿；dbqual 全部单测与 selftest 仍通过；FLOW-005-N03-B01 改为 PASS。
Residual risk: 策略声明了断言但覆盖不足属于策略编写问题，由 coverage gate 的 strict_complete 管。
```

处置（第二轮）：已修复。与上面“Complete fix boundary”的差异：门禁输出仍只用 PASS/FAIL（沿用包内 `invalid_evidence_selector`
的先例和 db.md 的判定映射），新增失败类型 `invalid_policy`、`empty_policy`；db.md 把 `empty_policy` 映射为 Arc 判定 `BLOCKED`，
`invalid_policy` 映射为 `FAIL`。coverage gate 只要求至少一条 claim，不拒绝未知顶层键（manifest 没有 schema，避免误伤带备注字段的现有 manifest）。

## 遗留问题

| # | 状态 | 类别 | 问题 | 影响范围 | 优先级 | 建议措施 | 完成条件 |
|---|---|---|---|---|---|---|---|
| 1 | `[x]` | 业务缺陷 | D1 release_gate 空策略放行；coverage gate 同类 | 数据库发布 GO/NO-GO | P1 | 已修复（第二轮 #1） | FLOW-005-N03-B01 PASS；两个新测试先红后绿 ✅ |
| 2 | `[x]` | 测试缺口 | FLOW-002 宿主链接治理无任何自动化测试 | 所有宿主技能发现 | P1 | 已补 `tests/test_link_hosts.py`（第二轮 #2） | 8 个分支 PASS ✅ |
| 3 | `[x]` | 测试缺口 | FLOW-001-N03-B01 安装目标与源仓重叠的守卫无测试 | 本仓源文件 | P1 | 已补（第二轮 #3） | 分支 PASS，tmp 树前后快照一致 ✅ |
| 4 | `[x]` | 测试缺口 | FLOW-001、FLOW-003、FLOW-004 共 10 个未设计分支 | 各工具边角路径 | P2 | 已补（第二轮 #4） | 分支 PASS ✅ |
| 5 | `[x]` | 缺口终点 | FLOW-001-T09 回滚二次失败、FLOW-002-T05 中途 OSError 不报备份位置 | 维护者恢复手段 | P3 | 已改为报错给出备份路径（第二轮 #2、#5）；仍无自动还原 | 终点改为“失败”并有用例 ✅ |
| 6 | `[x]` | 待确认规则 | `skills-backup-*` 与 `~/.skills-backup` 无保留策略，持续累积 | 磁盘与可读性 | P3 | 2026-10-24 用户决定手动清理：已删 `~/.agents/skills-backup-*`（约 78M）与 `~/.skills-backup`（约 38M）；README 写明不自动清理、确认后手动删除 | 规则写入 README 或脚本 |
| 7 | `[ ]` | 待确认规则 | gate.sh red 运行测试命令无超时，挂起即卡住 | arc-code 流程 | P3 | 用户决定是否加超时及时长（不能自定预算） | FLOW-004 维度 F 有用例 |
| 8 | `[ ]` | 未建模 | FLOW-006～016 共 11 条只登记未建模；FLOW-009 安全扫描、FLOW-010 arc-idx、FLOW-013 飞书导入没有自动化测试 | 辅助与跨流程 | P2 | 按风险先建 FLOW-006（仓库 CI 门禁）、FLOW-015（阶段交接） | 文件存在且 `flow_ledger.py check` 通过 |
| 9 | `[!]` | 基础设施限制 | FLOW-005 端到端 run_suite 需要项目测试 PostgreSQL；本机禁止起 PostgreSQL | 数据库发布门禁的真实运行 | P2 | 用户提供远程测试实例后跑 run_suite | 维度 F 由 BLOCKED 变为执行结果 |
| 10 | `[ ]` | 暂无法自动化 | FLOW-015/016 Agent 阶段交接只有静态语料（keyword winner），无宿主运行时证据 | 技能实际执行顺序 | P2 | 宿主侧行为评测跑 behavior_corpus 新增两条 | 两条场景有运行记录 |
| 11 | `[x]` | 安装 | 新版 Arc 已装入 `~/.agents/skills`（用户 2026-10-14 授权；旧版在 `~/.agents/skills-backup-3qcxy3lz`） | Claude、Codex、Grok 链接直接指向新版 | P1 | 已完成；`--check` 与 `link_hosts.py --check` 的剩余报错均在 Arc 之外，见下一行 | arc-verify 3.1.0、flow 参考与 `flow_ledger.py` 已在安装目录；Arc 相关 0 错误 ✅ |
| 12 | `[ ]` | 安装漂移（非 Arc，安装前已存在） | 2026-10-24 已处理：删除 `lark-suite` 与独立 `lark-workflow-project-docs`、装回本仓 `lark` 路由；`apifox-cli` 从配置移除；`link_hosts.py --check` 三个宿主全 OK。剩余：cnb 包内 node_modules 文档链接不可达 | 飞书相关技能、Codex 的 apifox | P2 | 用户决定保留 lark-suite 还是改回本仓 `lark`（改回会把 lark-suite 等移入备份） | `link_hosts.py --check` 全 OK |

## 未验证

- 宿主真实路由：新增触发词只经静态词表与语料验证；`route_probe.py`（需 jev 与密钥）未运行。
- Mermaid 渲染：图只经 `flow_ledger.py` 解析的子集校验，未在 Mermaid 渲染器中实际渲染。
- 新 flow 模式在业务项目（HTTP/数据库/消息）上的实际执行：本仓没有这类组件，未在真实业务仓运行。

## 复现命令

```bash
.venv/bin/python Arc/arc:verify/scripts/flow_ledger.py check docs/07-测试质量/业务流 --json
.venv/bin/python -m pytest tests/test_flow_ledger.py -q
.venv/bin/python -m pytest -v tests/test_sync_skills.py tests/test_gate_script.py
python3 Arc/arc:plan/scripts/closure/selftest.py
(cd Arc/arc:verify/scripts/dbqual && python3 -m unittest tests.test_run_binding tests.test_core tests.test_v4)
.venv/bin/python scripts/validate_skills.py && .venv/bin/python -m pytest tests -q
# 安装（待用户确认）
.venv/bin/python scripts/sync_skills.py --family Arc --naming dash
.venv/bin/python scripts/sync_skills.py --family Arc --naming dash --check && .venv/bin/python scripts/link_hosts.py --check
```

D1 的探针：在临时 git 仓用 `sqlverify_common.begin_run` 绑定一条 `status: FAIL` 证据，分别以 `{}`、
`{"required_evidnce":[{"id":"claim.a"}]}`、`{"required_evidence":[{"id":"claim.a"}]}` 运行 `release_gate.py`。
