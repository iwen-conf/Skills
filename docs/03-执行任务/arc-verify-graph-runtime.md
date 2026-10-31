# ARC Verify · Business-chain graph runtime implementation work

This is a staged implementation record. Do not mark a real-browser/real-Turso test PASS
when only Go unit tests or model validation ran.

## Implemented in the merged base (PR #1 merged 2026-10-29)

- C01: Reuse existing `arc:verify` and `arc-policy` rather than introducing a seventh Arc skill.
- C02: Add graph nodes/edges, rule/risk separation and model validation.
- C03: Add short-lived Go CLI and project-local Turso database schema via official `tursogo`.
- C04: Require manual business catalog and per-rule confirmation before planning mandatory tasks.
- C05: Generate positive, per-condition negative and forced-operation task references lazily.
- C06: Record run/step with independent expected-vs-observed comparison and SHA-256 RAW reference.
- C07: Fail closed in `arc-policy task verify` when project graph verification is configured.
- C08: Add model and policy regression tests, sample academic contract and CLI documentation.
- C09: Add fixture reference import/query and implementation observation graph gap inventory (absence remains unconfirmed).
- C10: Enrich task cards with actor, entry, preconditions and independent oracle; confirm the structural rule-edge mapping.
- C11: Add per-step RAW, first divergence, verified before-action conditions, halted/blocked/replan outcomes.
- C12: Add bounded recovery graph traversal with cycle detection; unconfirmed recovery actions remain unresolved.
- C13: Add abort/retry with reason and immutable historic runs.
- C14: Revalidate gitignored local DB and evidence at arc-policy receipt check; add Turso integration test and CI job (CI execution still unverified).

## Next verified work packages (not done)

- T01 **Native Turso integration gate (local PASS, CI NOT_RUN)**: use official `tursogo` Go driver in CI, exercise real
  SQL transactions, serialization, snapshots and concurrent claims; test failure exit codes.
  Do not claim DB integration on Go-only unit tests.
- T02 **Implementation observation/absence (partial)**: evidence-backed action observations and gap listing exist; expand to real UI/API crawler, versioned implementation graph, false-negative review and explicit MISSING confirmation. `NOT_FOUND` stays a candidate gap.
- T03 **Business condition graph traversal**: recovery tasks for OR/AND prerequisites,
  cross-role sequences, cycles and bounded interleavings; deduplicate equivalent states.
- T04 **Runner adapter & oracle**: real browser/mini-app/device adapter, stable step IDs,
  response+page+readback oracles, external-effect observation, trusted RAW provenance.
  Real E2E coverage remains NOT_RUN until an authorized target and credentials exist.
- T05 **Fixture registry & replay (partial)**: alias/secret-reference import exists; add
  per-task fixture locks, state verification before reuse, deterministic reconstruction
  and safe isolation. Never treat a logged checkpoint as automatic rollback of external actions.
- T06 **Policy hardening**: enforce graph freshness at commit/push time independently of
  local receipts; protect source/graph/evidence identities through CI; simulate agent
  bypass attempts. Same-user hooks never become a security boundary by themselves.
- T07 **Reports & graph diff**: inspectable coverage ledger per confirmed rule and edge,
  branches not reached, first divergence and downstream impact with historic runs.

Each task must deliver changed files, real test command and exit, expected+RAW evidence,
an exact verdict (PASS/FAIL/BLOCKED/NOT_RUN), and an explicit list of unverified claims.

## Review/acceptance matrix (do not claim all complete)

| Requirement | Source of truth | Status | Required real evidence |
|---|---|---|---|
| User-reviewed feature catalog | graph approval + scoped RULE | Implemented, trust limited to local terminal | human-reviewed contract and user decision |
| Risk never becomes RULE automatically | validation + approval | Implemented | negative tests for derived-rule rejection |
| Expected vs actual feature inventory | scan/gaps | Partial | actual browser/API traversal and source evidence |
| Directed graph + AND prerequisite recovery | nodes/edges/recover | Partial | recovery chains including OR/cycles and other actors |
| Direct executable task cards | next (actor/entry/conditions/oracle) | Partial | real runner action binding |
| Per-step observed precondition | step RAW | Implemented core | actual independently observed conditions |
| First deviation and safe continuation | step/trace | Implemented core | fault E2E and readback |
| Stable retry and historical attempts | abort/retry | Implemented core | process-crash test and data restoration |
| Accounts/fixtures reuse | fixtures + JSONC refs | Partial | true identity bindings and live state reconstruction |
| Local-only Turso | tursogo embedded driver | Native local tests PASS; CI NOT_RUN | native driver, persistence and concurrency; CI evidence separate |
| Independent, real E2E | browser/device adapter | NOT_RUN | user-like interaction on authorized test server |
| Strong trust against same-user tampering | CI/principal separation | NOT_DONE | protected CI; separate runner authority |

The terminal approval prompt can be spoofed by an agent able to drive a PTY.
It is a workflow guard, not cryptographic user authorization.
The RAW runner identity is currently a declarative string; do not call it a signature.
The embedded Turso integration job must actually report PASS before this is considered
production-qualified. No automatic fixture restoration, real browser driving or
complete graph-path enumeration is claimed yet.

## Cross-skill integration (2026-10-29)

- Connected the six existing Arc SKILL.md files to the existing shared
  handoff §6; did not install a seventh Arc entry.
- Added `arc-verify handoff --to arc-* --graph/--run` with bounded, read-only
  context packets. `arc:code` requires an actual FAIL run to consume a repair
  packet; `arc:plan` and `arc:audit` cannot change graph state.
- Made `arc-policy` business-graph gate phase-aware: `arc:verify` must
  prove the graph, while `arc:code` can repair a red graph under its
  ordinary source/test checks. Explicit `require_for_skills` adds phases.
- Added static cross-skill handoff/registry tests and phase-aware policy tests.
- Synchronized the registry and enforced SKILL heading/token budgets.
- Still NOT_RUN: full skill distribution validation on a cloned repository,
  official Turso integration CI, real E2E and detached evidence principal.
  `REQ → RULE → FLOW` remains a documented mapping rather than a
  fully enforced foreign-key relation; do not claim otherwise.

## 2026-10-29 性能优化说明

- 原 Shell 方案每条 SQL 单独 fork/exec，已切换官方 Go `tursogo@v0.8.3` in-process driver。CLI 仍不常驻。
- SQL 结果直接按列扫描，移除 HEX+分隔符序列化。流式 gate 复用扫描缓冲，按任务扫描并一次读取、校验每份 RAW。
- 业务验收从按任务 N+1 数据库查询改为有序 JOIN；索引覆盖就绪任务、规则审批、运行历史和实现入口。
- 保持严格 RAW 完整性验收；不能因为追求速度跳过证据或者将风险判为规则。
- 添加 `BenchmarkTaskCardPagination`、`BenchmarkGateWithRAW` 和原生 Turso 往返集成测试。没有实际跑出性能数据之前，**不声称提升倍数或者最优值**。
- 需要至少三组可重复基准：100/1000/10000 条规则及任务规模、多进程写入冲突、长历史 RAW。必须同时报告 p50/p95、allocs/op、内存峰值和功能一致性。

## 最新实现边界（性能）

- `plan --limit` 使用 事务式规则/条件游标按固定批次生成任务（无需反复扫描整个规则集），不把整张图全部物化在 Go 或 AI 上下文；`gate` 确认所有约束生成、实际执行及证据完整，否则 FAIL。
- `recover` 仅查找当前条件的带索引邻域，深度 ≤8，恢复动作 ≤128、单条件返回 ≤512 行，越界明确标记 `truncated`，不能宣称已穷尽。
- `gate` 采用单 JOIN 流式遍历，完整复核证据且不保留全部 Step 数据；保留用户明确确认规则、非法操作服务端拒绝及实际状态读回要求。
- 真实速度数据仍需要在 Go1.24+/官方 tursogo 的运行环境通过测试和基准产生；没有实际数据前拒绝声称达到最优。

- 规划游标与任务卡同一事务持久化；晚确认的新规则会使游标回到起点，保留已生成任务和 Run 历史。增量规划只扫描未访问的规则段，运行结束后无常驻进程。已有 `.arc/verify/data.db` 升级后先执行 `arc-verify init` 完成幂等建表/索引。

- [x] Fresh local Turso initialization: ensure all CREATE TABLE operations precede dependent indexes; wrap idempotent DDL in a transaction. Tests cover fresh/repeated initialization and preserving an existing graph. Latest GitHub Actions results must still confirm the tests before claiming PASS.

## Native driver and CI readiness (2026-10-29)

- `arc-verify init` now executes the table-first, index-last schema transactionally, fixing fresh local DB creation; regression tests cover first initialization, idempotence and preexisting data.
- `arc-verify selftest` checks the real embedded Turso driver in a disposable local file without contacting the test server. `arc-policy doctor` invokes it when graph verification is configured.
- `.github/workflows/arc-verify.yml` supports both main-branch changes and manual `workflow_dispatch`. The latest workflow must report real PASS; a compatible SQLite simulation is not a substitute for native Go/Turso test evidence.
- Real browser E2E remains NOT_RUN without authorized target URL, fixture accounts and a trustworthy runner; automatic fixture restoration is not complete.

## 2026-10-30：按完整对话强化（verify-hardening）

已完整核对《自动化测试规范设计》5543 行对话。沿用用户确认的路线：应有业务清点并核对 →
独立实现清单 → RULE/业务图 → 有界任务卡 → 真实执行 → 独立 RAW → 首次偏离与历史 FAIL。
图由官方 Go `tursogo` 进程内访问本仓本地文件；测试服务器只做被测目标。

- [x] H01：缺失/null/空白 RAW、重复 JSON 键及大小写别名不能通过；拒绝操作必须有显式 denial 和前后读回。非法写入保留 FAIL；Runner 失败为 BLOCKED。
- [x] H02：规则前提与 `requires` 双向一致；冻结任务含义由规则重建；任务 ID 冲突、幂等规划中的不同义务、finish/修复交接中的证据变化被拒绝。
- [x] H03：流程账本重复行不能覆盖第一条失败；既有 DBqual 严格验证证据/策略、有效指标、当前输入绑定、完整 RAW 基线和历史隔离。
- [x] H04：强化 SKILL、业务图/浏览器协议、共享交接及项目事实；只有用户明确确认才是 RULE；恢复和安全继续不能覆盖原 FAIL。
- [x] H05：全量 Python/DBqual、原生 Go race/vet/build/selftest、技能分发与 completion 门禁通过；arc-policy 配置加入真实 Go 编译、race 和 vet。最终 receipt 与逐项结果保存在本轮私有验收报告。
- [!] H06：全仓技能结构门禁仍失败：既有 GitHub workflow 与仓库禁令冲突，arc:audit 为 3535/3500 tokens；主仓基线已复现，需归属该规范/技能的修复。
- [x] H07：实际安装演练发现 DBqual 的 Ruff 缓存被复制；在既有分发复制路径排除 Ruff/Pytest/mypy 缓存，保留脚本内容与可执行权限。新反例先记录为 red-007，再修复；安装回归 30 passed。
- [x] H08：对照当前 OpenAI Docs 与内置 Skill Creator 校验，源码继续保留 `version`，安装包将其放入 `metadata.version`，保留其他元数据、正文和依赖；冲突版本在替换安装前报错。red-008 先记录两项反例，修复后安装回归 32 passed，197 文件的独立安装包及 Skill Creator 校验通过。

新增反例先于对应修复记录在 `.arc/artifacts/verify-hardening/red/`。最近针对性验证：
`go -C tools/arc-verify test -count=1 ./...` PASS；
`.venv/bin/python -m pytest -q tests/test_flow_ledger.py Arc/arc:verify/scripts/dbqual/tests/test_run_binding.py`
为 52 passed、51 subtests passed。它们验证工具内核，不替代目标业务 E2E 或 PostgreSQL 发布资格。

安装缓存修复后的全量验证：根 Python 429 passed、1 skipped（R8 protobuf 环境未配置）；DBqual 87 passed、
1 skipped（真实 SQLFluff 命令未配置）、51 subtests passed；DBqual selftest PASS；Go race/vet、
编译及真实 CLI selftest PASS。`sync_skills.py --only arc:verify --dry-run` 验证 1 个包。
核心 29 文件的严格 `gate.sh check --require-red --max-files 29` 为 0 FAIL、0 NOT_RUN；red-hygiene 提醒的
3 个测试文件只做格式/导入调整，原断言未削弱。报告：
`.arc/artifacts/verify-hardening/reports/gate-20261030T062540Z.txt`。
加入安装过滤和元数据兼容后的最终门禁、主仓回执、最新测试计数与本机安装读回统一记录在
`.arc/artifacts/verify-hardening/reports/final-verification.md`。

下一步仍是 T04：连接既有浏览器能力与图任务卡，绑定实际目标、账号/fixture 和独立读回。
自动 fixture 恢复、完整跨角色/并发链、独立可信 Runner 与 CI 成功记录尚未完成/取得；
`REQ → RULE → FLOW` 完整映射仍需独立证据。现有 graph gate 只验收已规划的
POSITIVE/NEGATIVE/FORCED，不能据此宣称恢复链或完整业务通过。

## 2026-10-30：生产验收续批（进行中）

目标：按原文最终裁决及当前“生产级、完整”要求继续强化；同一 tracker 保留全部未完成工作。
范围：verify CLI、其 DBqual/流程工具、六技能共享交接与直接调用/示例/测试；IM-mongo 当前只读发现业务与执行边界。
先固定反例，再改实现；沿用本地短生命周期 Go/Turso，不新增测试账本服务，不覆盖他会话改动。
验收命令：Go native/race/vet/selftest，Python/DBqual，严格 completion gate，安装读回及真实目标的逐 claim 证据。

| 批次 | 状态 | 已复现问题 / 完成条件 |
|---|---|---|
| P01 | [/] | finish 与 abort 竞争不能出现 run=BLOCKED/task=PASS；start/step/abort/retry 的兄弟路径也检查实际状态转换与影响行数 |
| P02 | [/] | RAW 绑定 graph/task/run、同一候选、配置、实际目标、actor/account/fixture/oracle；不得把不同运行拼成一次验收 |
| P03 | [/] | 输出目录不能覆盖/归档候选或输入；报告验证当前 run、candidate、完整 policy/RAW binding；子进程有明确执行截止和终止证据 |
| P04 | [ ] | V01–V26 要求矩阵与所有 references 的现状引用/冲突核对；未执行/截断/不适用不借局部 PASS 放行 |
| P05 | [/] | 读取 IM-mongo 的现行合同、已存在流程/真实测试、fixtures、环境边界；原证据保留版本，不转为新候选 PASS |
| P06 | [ ] | T02–T07 的真实执行、恢复/跨角色/并发、fixture 还原、完整覆盖及保护 CI 边界逐项补证 |

新反例位于 `tools/arc-verify/production_hardening_test.go` 与
`Arc/arc:verify/scripts/dbqual/tests/test_production_hardening.py`，从当前 3.5.0 源码基线执行。
该续批 completion 基线独立保存在 `.arc/artifacts/verify-production-hardening/`，不覆盖原 hardening 基线。

IM-mongo 已确认默认 API 面为 `.50:28885`；共享数据写入需具体任务卡授权，不默认部署或重启。
较新 `docs/02-运维/测试账号与隔离环境.md` 明确 `.63` 已于 2026-10-22 移除，旧 AGENTS 仍列该机。
可用测试机和隔离资源需按最新事实核实；不因旧脚本、历史 PASS 或专属旧任务授权扩大本轮权限。

## 2026-10-30：业务图谱工作台（当前主线）

需求：[BG01–BG25](../00-产品需求/06-业务模型驱动验证.md)；设计/快照合同：[05 工作台](../01-系统设计/05-业务图谱工作台.md)；前置约束：[00](arc-verify-graph-runtime/00-前置约束.md)。本表继续作为唯一 tracker，旧 T/P 批次未被新目标删除。

当前源码为 `f230ae9efa42` 加既有 staged/unstaged hardening，多个会话并发；不重置、清理或统一格式化。新 Go 功能在既有 `tools/arc-verify/`；新前端在其 `workbench/`。只读/测试产物在 `.arc/artifacts/business-graph-workbench/`。

| 批次 | 状态 | 端 / 优先级 / 难度 | 范围与完整验收 |
| --- | --- | --- | --- |
| W00 | [x] | 工具 / P1 / D2 | Stop 的只读并发归属、明确编辑缺 Post、否定/示例误判修复；已安装缓存入口 5 案 PASS，严格专项门禁 0 FAIL/0 NOT_RUN。报告 `.arc/artifacts/stop-hook-isolation/reports/gate-20261030T142025Z.txt`；旧会话归属不伪造，其他仓实际 verify 分开记录 |
| W01 | [x] | 后端 / P0 / D3 | [版本模型](arc-verify-graph-runtime/W01-model.md)：类型关系/测试与不变量、追加迁移和冻结兼容。2026-10-31：新增 3 条验收测试（含 v1 hash 旧源码黄金值），6 个人为缺陷全被抓到；race/vet 通过，gate 0 FAIL |
| W02 | [x] | 后端 / P1 / D3 | [分析与完整义务](arc-verify-graph-runtime/W02-analysis.md)：可达/有界探索、六投影、完整覆盖/历史/影响。2026-10-31 核实既有实现并补 6 类缺失测试；实现 W10 的 `history[]`、等价去重与 `merges[]`；15 个种下缺陷全抓到 |
| W03 | [x] | 后端 / P1 / D3 | [执行与恢复](arc-verify-graph-runtime/W03-execution.md)：2026-10-31 核实既有实现、补 3 个缺失测试，修复"错误放行被记成 BLOCKED 而非 FAIL"（`StepRAW.Denied` 的 `omitempty`）；真实业务目标归 W06 |
| W04 | [-] | 后端 / — / D2 | [本地接口](arc-verify-graph-runtime/W04-server.md)：2026-10-31 用户裁决"改成静态快照"，前台服务与 `/api/*` 作废，由 W13 取代。重启条件：用户重新要求页面内可执行/批准 |
| W05 | [x] | 前端 / P1 / D3 | [静态快照工作台](arc-verify-graph-runtime/W05-workbench.md)：2026-10-31 改为只读读取内嵌快照，删除网络层与写操作；`vp check`/`vp test`(14)/`vp build` 通过，真实浏览器 `file://` 零外部请求、六视图/下钻/回放验收通过，截图在 `.arc/artifacts/verify-static-workbench/` |
| W06 | [!] | 集成 / P1 / D3 | [完整验收](arc-verify-graph-runtime/W06-acceptance.md)：工具层通过；隔离 seat-v2 链见 W25。IM-mongo 仍阻塞：缺用户确认的 v2 规则、环境 `approved` 与账号/fixture 授权。不得把 seat PASS 记成 IM-mongo PASS |
| W07 | [x] | 后端 / P1 / D3 | [补齐 EXISTENCE/SEQUENCE/CONSISTENCY 义务](arc-verify-graph-runtime/W07-obligation-kinds.md)（BG19）：2026-10-31 完成，全量 race 182 项 PASS，12 个种下缺陷全抓到；人工 approve 后的 CLI 路径与 IM-mongo 未验 |
| W08 | [x] | 后端 / P2 / D3 | [任务卡自动繁殖](arc-verify-graph-runtime/W08-derived-obligations.md)（BG23）：2026-10-31 `derive` 落地，规则派生义务进分母、推断项只进风险报告，等价键去重、预算截断、原 FAIL 不变；14 个种下缺陷全抓到 |
| W09 | [x] | 后端 / P2 / D3 | [DISCOVERY 与 UNSPECIFIED](arc-verify-graph-runtime/W09-discovery.md)（BG24）：2026-10-31 `discover record/list/triage` 落地，四类契约外行为、等价键去重与预算、交互分流（遗漏需求交 arc-plan、不应存在进风险报告） |
| W10 | [x] | 研究 / P1 / D4 | [状态等价判定研究](arc-verify-graph-runtime/W10-state-equivalence.md)（BG21）：2026-10-31 定稿等价键合同（设计 05「状态等价」），实现落在 W02/W08/W09 |
| W11 | [x] | 后端 / P2 / D3 | [确定性连续执行](arc-verify-graph-runtime/W11-run-chain.md)（BG22）：2026-10-31 `run-chain` 落地，六种停机原因各有退出码与最小上下文包；顺带修复"前提不满足时 RAW 丢失"和"空路径报完成"两个缺陷；真实二进制退出码已实测 |
| W12 | [?] | 后端 / P1 / D2 | [验收报告与风险报告分离](arc-verify-graph-runtime/W12-risk-report.md)（BG20、BG26 来源类别）。2026-10-31：`report`/`accept-risk`/`promote-risk`/`--risk-review` 落地，13 条新测试，race 153 条通过，gate 0 FAIL；待用户在真实终端确认一次 `accept-risk` |
| W13 | [x] | 后端 / P1 / D3 | [静态快照导出](arc-verify-graph-runtime/W13-snapshot.md)（BG25）：2026-10-31 `snapshot` 命令落地，`workbench` 服务与 `/api/*` 删除（原文件已备份），`types.ts` 按 Go DTO 重写并有字段比对测试，示例快照已导出 |
| W14 | [x] | 文档 / P2 / D2 | [技能文档同步](arc-verify-graph-runtime/W14-skill-docs.md)：2026-10-31 business-graph 义务表补四种新义务，新 reference `business-graph-v2.md`（v2 字段、等价、全部新命令），README 命令速查；注册表重新生成，技能校验与 pytest 通过 |
| W15 | [?] | 交付 / P3 / D1 | [分支清理](arc-verify-graph-runtime/W15-branch-cleanup.md)：2026-10-31 只读核查完成；可删本地已合并分支 `verify-hardening-2921` 与清理失效 worktree，等用户确认；分离 HEAD worktree 有 33 处未提交改动保留；远端分支作者不是本机用户，只列出 |
| W16 | [-] | 治理 / — / — | 受保护 CI 作为可信边界：本仓不启用 GitHub Actions（`tasks/lessons.md`）。重启条件：用户指定独立的 Runner 主体（独立账号或机器）
| W17 | [x] | 后端 / P2 / D2 | [三向比对报告](arc-verify-graph-runtime/W17-three-way-report.md)（BG26）：2026-10-31 风险报告 `three_way` 三栏（要求有而实现无、实现有而要求无、参照有而要求无），各带来源与证据，不判 FAIL |
| W18 | [x] | 后端 / P2 / D2 | [风险反证记录](arc-verify-graph-runtime/W18-risk-refutation.md)（BG27）：2026-10-31 `risk refute` 落地，证据 hash 绑定、报告展示 `not_attempted`/结果/`evidence_changed`，反证不关闭风险 |
| W19 | [x] | 研究 / P2 / D4 | [同类系统参照库研究](arc-verify-graph-runtime/W19-reference-catalog.md)（BG28）：2026-10-31 定稿槽位+关系格式、四级来源与降噪合同（设计 05"参照库"），实现卡 W24 |
| W20 | [!] | 验证 / P1 / D3 | [注入缺陷检出率](arc-verify-graph-runtime/W20-mutation-detection.md)（BG29）：CLI 完成；隔离 seat 五类注入 5/0/0（`insufficient_sample`）。IM-mongo `.60` 注入部署仍未授权 |
| W21 | [x] | 后端 / P3 / D2 | [分流精度台账](arc-verify-graph-runtime/W21-triage-ledger.md)（BG30）：2026-10-31 `reject-risk` 与风险报告 `triage_summary`（按来源统计接受/驳回/待定与样本量，不足 10 标注），驳回的参照同绑定不再生成；不进门禁 |
| W22 | [x] | 选型 / P2 / D2 | [Schemathesis 选型](arc-verify-graph-runtime/W22-schemathesis-vet.md)（BG31）：2026-10-31 结论"借鉴设计"，DISCOVERY 模型驱动自建；Schemathesis 4.32.7 作为有 OpenAPI 时的可选命令适配器；IM-mongo 无 OpenAPI。记录 [docs/10-复盘决策/01](../10-复盘决策/01-选型-API状态化探索.md) |
| W24 | [x] | 后端 / P2 / D3 | [参照库导入与匹配](arc-verify-graph-runtime/W24-reference-matching.md)（BG28）：2026-10-31 `reference-import`/`reference-match` 落地，RISK/NO_BINDING/AMBIGUOUS/SUPPRESSED 四种结果、上限 50、只进风险报告；示例参照库复现 W19 演示 |
| W23 | [x] | 工具 / P3 / D1 | [arc-policy 传入风险审查开关](arc-verify-graph-runtime/W23-policy-risk-review.md)：2026-10-31 `risk_review: true` 时传 `--risk-review`；顺带修复 arc-policy 不接受 v2 证明范围（`complete_business_model_obligations`）导致 v2 图永远过不了门禁 |
| W25 | [x] | 集成 / P0 / D3 | [隔离业务链与缺陷检出](arc-verify-graph-runtime/W25-isolated-real-chain.md)：seat-v2 正常/异常/并发/恢复、五类注入、六视图真实回放、MODEL FAIL 可交 arc-code。IM-mongo 仍归 W06/W20 `[!]` |

W01 的持久化合同、W02 的义务与状态、W03 的执行与恢复分别不可拆散其不变量；W13/W05 分别开发与验收（快照导出与只读前端），W06 承接真实联合行为。Go 使用原生 Turso；Rust 编译仅在 `.60`。前端不安装 ELK/Dagre 或新的图服务。

2026-10-31 需求重申已回写既有 PRD、设计和 W05：六视图职责、同 ID/上下文跨视图导航、四维完整度证据与真实回放验收。没有新建平行模型或 tracker；本次只更新文档，W01–W06 状态不变。W00 的已有专项测试不能证明用户提供的历史 Stop 拦截已彻底解决，仍需区分具体会话归属与业务仓回执失效原因。

IM 增量发现已找到 57 专属 `.60` 隔离授权，范围见现行 `57/evidence/T00-OPS-01.md`；不能用旧47登记替代当前候选、完整fixture/主体、通知/设备与截止预算。新版55介入/回收和 Legacy/V2版本隔离按现行合同分别绑定。运行证据仍需取得，不把授权或发现本身标为 E2E PASS。

## 2026-10-31：演化路线核对与剩余工作排序

已逐轮对照对话导出（352 条消息）与多视图附件；结论和依据见 [PRD 06 演化路线核对补充](../00-产品需求/06-业务模型驱动验证.md#2026-10-31演化路线核对补充)。主线一致；补出 BG19–BG25，W04 作废、W16 不做，其余按下面的顺序执行。优先级与顺序由用户授权本会话决定（"优先级你来定就行"）。

| 顺序 | 批次 | 理由 |
| --- | --- | --- |
| 1 | W01 `[x]` | 所有后续批次都读写 v2 模型；先定持久化合同 |
| 2 | W07 `[x]`、W12 `[?]`、W10 `[x]`、W19 `[x]`（可并行） | W07 的新义务要进 W02 的完整分母；W12 只依赖现有 gate 与 risks 表，同时加上 RISK 来源类别（BG26）；W10、W19 是研究，不阻塞前两项 |
| 3 | W02 `[x]` | 依赖 W01、W07；W10 定稿后补进有界探索的去重 |
| 4 | W03 `[x]` | 依赖 W01、W02 的义务与任务卡 |
| 5 | W13 `[x]` → W05 `[x]` | 快照导出依赖 W02 视图与 W03 trace/证据 DTO；前端依赖快照 JSON 合同 |
| 6 | W11 `[x]` → W08 `[x]` → W22 `[x]` → W09 `[x]` | 都建在 W03 的真实执行上；W08、W09 去重依赖 W10；W09 的探索适配器先等 W22 选型 |
| 7 | W24 `[x]` → W18 `[x]` → W17 `[x]` → W21 `[x]` | 参照匹配依赖 W19 合同与 W12 来源字段；反证依赖 W12 的风险字段；三向比对依赖 W07、W09、W12、W24；精度台账依赖前面的分流事件 |
| 8 | W20 `[!]`（CLI + 隔离 seat 5/5；IM-mongo 待授权） | 检出率要用真实执行和已授权隔离环境，放在联合验收前，作为"凭什么确信"的证据 |
| 9 | W14 `[x]` | 在功能定型后同步技能文档，避免反复改 |
| 10 | W06 `[!]`（工具层完成，真实业务待输入） | 最终 IM-mongo 联合验收 |
| 11 | W15 `[?]`（清单待确认） | 交付收尾；删分支前逐个核对独有提交并请用户确认 |

并发提醒：2026-10-31 上午仍有其他会话修改 `tools/arc-verify/workbench*.go` 与 `workbench/src/`。W04 作废、W05 改造会让这部分在途改动失效；开工 W13/W05 前先看 `git status` 和文件修改时间，发现在途改动先报告，不静默覆盖。

2026-10-31 发现（本 tracker 范围外，待单独立卡）：arc-code 的 `gate.sh`（源码 `Arc/arc:code/scripts/gate.sh`，安装副本 `~/.agents/skills/arc-code/`）用 `npx` 跑 `prettier`/`tsc`/`vitest`，与全局规则"前端用 Vite+ 与 bun、禁止 npm/npx"冲突；bun 工程（`devEngines` 指定 bun）每次都报 `EBADDEVENGINES` 假失败。W13/W05/W08 的前端检查改用 `vp check`/`vp test`/`vpr build` 复验。建议给 arc-code 立卡：检测到 Vite+/bun 工程时改用 `vp`。

2026-10-31 授权：用户回复"允许你直接完成剩余任务"，本会话按上表顺序连续执行，不再逐项停下确认；删分支（W15）、提交推送和改远程共享数据仍按全局规则单独确认。

2026-10-31 追加：用户贴来外部研究评估，要求结合进 Verify。核对结果和借鉴结论见 [PRD 06 外部研究核对与借鉴](../00-产品需求/06-业务模型驱动验证.md#2026-10-31外部研究核对与借鉴)；新增 W17–W22，已排进上表。

## 2026-10-31：隔离 seat-v2 真实链与缺陷检出

目标不是重做框架，而是证明现有 CLI 能在已授权入口上执行、发现缺陷、留证据、恢复并展示。IM-mongo 环境未批准，改用仓库内 seat-v2 httptest（W25），明确不是 IM-mongo 验收。

本轮修正：空 `run-chain` 记 blocked；`derive` 拒陈旧轮次；UNSPECIFIED 绑定能力；工作台回放首次 FAIL；v2 MODEL FAIL 可交 `arc-code`。Go race 276 passed；Python 476；DBqual 160；技能 65；工作台 18 项；快照 `.arc/artifacts/verify-isolated-seat/site/index.html`。

下一轮能否进正式项目验证：工具与隔离链可以；IM-mongo 正式验证仍要用户确认的 v2 RULE、环境 `approved:true` 和账号/fixture 授权。

## 2026-10-31：自主探索与独立进程 E2E（verify-explore-e2e）

基线 `78666ea`。不重做 v2 图、DISCOVERY、run-chain、六视图或 seat-v2 合同。

| 批次 | 状态 | 范围与验收 |
| --- | --- | --- |
| W26 | [x] | [测试可信度](arc-verify-graph-runtime/W26-test-credibility.md)：恢复四类与 occupancy barrier；检出 FAIL ≠ 恢复 PASS |
| W27 | [x] | [自主探索](arc-verify-graph-runtime/W27-autonomous-explore.md)：`discover run` 真实执行 + Frontier + 预算停止 |
| W28 | [x] | [盲测](arc-verify-graph-runtime/W28-blind-explore.md)：4 检出 / 1 漏检（B-VALID 无关课动作）；不是预写五测 |
| W29 | [x]/`[!]` | [独立进程 E2E](arc-verify-graph-runtime/W29-independent-e2e.md)：`enroll-sut` 通过；IM-mongo 未批准，保持 BLOCKED |
| W30 | [x] | [探索快照](arc-verify-graph-runtime/W30-explore-snapshot.md)：能力节点展示探索/义务，不按探索次数点绿 |

报告：[verify-explore-e2e-验收.md](arc-verify-graph-runtime/verify-explore-e2e-验收.md)；运行证据 `.arc/artifacts/verify-explore-e2e/`。
W06 / W20 的 IM-mongo 项仍为 `[!]`。

## 2026-10-31：盲测归因、续探与 AI 规划（verify-blind-resume）

基线 `923186a`。不新增第七个 Skill，不恢复常驻工作台。

| 批次 | 状态 | 范围与验收 |
| --- | --- | --- |
| W31 | [x] | [盲测归因](arc-verify-graph-runtime/W31-blind-attribution.md)：独立评估器 + 旧字符串误判证明 |
| W32 | [x] | [业务判定](arc-verify-graph-runtime/W32-judgment-evidence.md)：允许被拒 FAIL；并发双方入账 |
| W33 | [x] | [Frontier 续探](arc-verify-graph-runtime/W33-frontier-resume.md)：`discover status/resume/abort` |
| W34 | [x] | [AI 规划](arc-verify-graph-runtime/W34-ai-planner.md)：可替换接口，建议须过白名单 |
| W35 | [x] | [跨项目 E2E](arc-verify-graph-runtime/W35-cross-project-e2e.md)：iso28886 已知规则 8/8 + 三种探索；图谱 TTY 仍未批 |

报告：[verify-blind-resume-验收.md](arc-verify-graph-runtime/verify-blind-resume-验收.md)。

