# ARC Verify · Business-chain graph runtime implementation work

This is a staged implementation record. Do not mark a real-browser/real-Turso test PASS
when only Go unit tests or model validation ran.

## Completed in this branch (review before merge)

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
- C14: Revalidate gitignored local DB and evidence at arc-policy receipt check; add Turso integration test and CI job (execution pending).

## Next verified work packages (not done)

- T01 **Native Turso integration gate**: use official `tursogo` Go driver in CI, exercise real
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
| Local-only Turso | tursogo embedded driver | Implemented adapter, integration NOT_RUN | native driver, persistence and concurrency |
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
