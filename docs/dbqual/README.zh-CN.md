# Industrial Database Release Qualification Engineer v4.0.0 — Final

这是一套面向 **PostgreSQL + 真实业务 SQL/事务** 的专家级数据库发布资格（Database Release Qualification）SKILL。

它不把“跑过 EXPLAIN”“压过几分钟 pgbench”“单测通过”当作生产就绪。最终目标是回答一个更严格的问题：

> **这次数据库相关变更，在正确性、并发一致性、死锁/重试、性能与容量、迁移、复制、故障恢复、备份恢复、安全边界以及上线后的真实业务不变量上，是否有足够、可复现、可审计的证据允许发布？**

本包不是对 Google、Cloudflare、Oracle、Stripe 私有内部系统的复刻，也不会声称达到其内部未公开标准。它把这些公司公开工程资料中可验证的原则、PostgreSQL 18 官方语义，以及成熟数据库可靠性实践，转成一套 **fail-closed、claim-based、evidence-driven** 的可执行工程体系。

---

## 1. 最终版的核心原则

```text
没有执行          != PASS
没有采集到指标    != 指标为 0
没有发生死锁      != 证明不会死锁
pg_verifybackup OK != 证明一定可恢复
amcheck 没报错     != 证明绝无损坏
平均延迟很好      != P99/拥塞/恢复正常
SQL 单独正确      != 多事务组合正确
重试最终成功      != 没有重复业务副作用
迁移成功执行      != 在线迁移安全
副本最终一致      != 满足 read-your-writes / monotonic-read 契约
测试跑了很多      != 高风险声明被验证
```

Evidence 只允许：

```text
PASS
FAIL
BLOCKED
NOT_RUN
FLAKY
```

关键 Gate 缺失、观察器失败、测试环境不可信、依赖缺失，都不能被解释成 PASS。

---

## 2. 风险等级

建议先把数据库改动分类，而不是所有 PR 一刀切：

| 等级 | 典型改动 | 最低关注范围 |
|---|---|---|
| DB-R0 | 局部只读、低风险查询 | scope、correctness、基础 plan |
| DB-R1 | 性能关键查询/索引 | dataset、plan、benchmark、baseline |
| DB-R2 | 可变共享状态/并发事务 | locks、isolation、deadlock、retry、idempotency |
| DB-R3 | schema/data lifecycle migration | online migration、compatibility、backfill、rollback |
| DB-R4 | 高完整性业务事实：资金/账本/库存/额度 | history/model checking、fault、reconciliation、restore |
| DB-R5 | replication/HA/DR | replica/CDC、failover/failback、RPO/RTO、PITR |
| DB-R6 | PostgreSQL 大版本/存储/平台级变化 | 全量 qualification、upgrade compatibility、replay、DR、canary |

风险等级不是“分数”；它决定哪些证据必须存在。

---

## 3. G0–G31 最终门禁目录

### 规范与环境

| Gate | 内容 |
|---|---|
| G0 | Change scope / risk classification / business claims |
| G1 | Test environment isolation / safety / tool trust |
| G2 | Representative dataset / skew / hot-key / tenant-size qualification |
| G3 | Schema integrity / relational design hazards / constraints |

### SQL 与优化器

| Gate | 内容 |
|---|---|
| G4 | Query correctness + property/metamorphic verification |
| G5 | EXPLAIN / cardinality / buffers / temp / WAL diagnostics |
| G6 | Parameter sensitivity / prepared statement generic-vs-custom plan |

### 事务、并发与分布式语义

| Gate | 内容 |
|---|---|
| G7 | Isolation-level litmus / anomaly qualification |
| G8 | Static lock-order graph |
| G9 | Deterministic concurrency / deadlock / wait-for graph |
| G10 | Seeded schedule / contention exploration |
| G11 | Retry / idempotency / ambiguous-result qualification |

### 性能、容量与真实负载

| Gate | 内容 |
|---|---|
| G12 | Business workload benchmark / P50/P95/P99/TPS/errors/retries |
| G13 | Overload → failure cliff → recovery |
| G14 | Sanitized production-shape workload replay |
| G15 | Long-run / soak / resource stability |
| G16 | MVCC / vacuum / XID / idle transaction / slot health |
| G17 | WAL / checkpoint / IO / temp / cache / write-amplification health |

### 变更、复制与数据生命周期

| Gate | 内容 |
|---|---|
| G18 | Migration / backfill / online compatibility / rollback |
| G19 | Logical replication / CDC / subscription conflicts |
| G20 | Read replica visibility / monotonic/read-your-writes contract |
| G21 | Physical integrity / checksums / pg_amcheck |
| G22 | Backup artifact verification |
| G23 | Actual restore / PITR / business-level restore oracle |
| G24 | Failover / failback / RTO / RPO |
| G25 | PostgreSQL/extension/collation/upgrade compatibility |
| G26 | Database security boundary / roles / RLS / SECURITY DEFINER |

### 高完整性与发布闭环

| Gate | 内容 |
|---|---|
| G27 | History/model checking for high-risk concurrent systems |
| G28 | Trusted statistical baseline regression |
| G29 | Canary/control database guardrails |
| G30 | Production reconciliation / anomaly detection / incident regression |
| G31 | Evidence completeness / provenance / waiver / test-system trust |

`qualification_coverage_gate.py` 支持 `strict_complete=true`：G0–G31 每一个 Gate 都必须被显式声明为 applicable 或 non-applicable；non-applicable 必须有 rationale。

---

## 4. 最关键的并发/死锁体系

本包不会把“跑 100 个并发没死锁”当成死锁安全证明。

### 4.1 静态锁顺序

`lock_order_analyzer.py` 从业务 operation 的资源获取顺序构造有向图：

```text
transfer: account:min -> account:max
refund:   account:max -> account:min
```

形成环时直接暴露设计风险。这只是 design linter，不代替真实 PostgreSQL 验证。

### 4.2 Deadlock detector self-test

测试体系首先必须证明自己能检测真实死锁：

```text
Session A lock row 1
Session B lock row 2
barrier
A waits row 2
B waits row 1
```

要求同一 observer snapshot 中出现 wait-for cycle，并捕获 `40P01`。跨时间累积的边不能拼成假环。

### 4.3 Production concurrency contract

真实业务组合要求独立定义：

```text
40P01 budget
40001 budget
lock-wait P95/P99
max transaction age
statement/lock timeout
retry attempts
idempotency invariant
final durable-state invariant
```

PostgreSQL 的 Serializable/Repeatable Read 冲突必须围绕**完整事务逻辑**重试，不能只重放最后一条 SQL。

### 4.4 Seeded schedule exploration

`contention_fuzz_runner.py` 用固定 seed 探索更多交错；失败 seed、schedule、SQLSTATE 和 invariant evidence 必须持久化。高价值随机失败应转换成确定性 barrier regression。

### 4.5 History / model checking

DB-R4+ 不把 fuzz 当穷举证明。必要时通过 `history_checker_adapter.py` 接入 Jepsen/Elle 或项目自己的 checker：

```text
operation history
model/version
fault schedule
checker output
minimal counterexample
```

有限实验可以证明发现了异常，不能宣称证明所有异常不存在。

---

## 5. SQL 性能不是单一耗时阈值

正式 benchmark 至少同时看：

```text
P50 / P95 / P99 / max
TPS / throughput
failure / retry rate
schedule lag
lock wait
CPU / IO / WAL / temp
pg_stat_statements total cost
plan/cardinality drift
connection saturation
recovery after overload
```

`pgbench_runner.py` 支持业务脚本权重、并发/rate 矩阵、warmup、多 repetition、固定 base seed + 每轮派生 seed。采样日志只用于延迟分布；全量成功/失败/重试从 pgbench summary 获取，避免 sampling-rate 导致失败率失真。

Baseline 比较不是一次结果 `>20%` 就武断失败。`regression_compare.py` 支持多轮结果与 bootstrap confidence interval；同时验证 PostgreSQL major、settings hash、schema fingerprint、dataset profile、workload/script hash，防止不可比环境之间硬算回归。

---

## 6. 数据必须“代表生产形态”，不是随机填充

`dataset_audit.py` 与参数 corpus 支持把这些作为 Oracle：

```text
总规模
热点 key / Zipf skew
大中小租户占比
状态分布
时间分布
字段相关性
parent-child fan-out
NULL / rare value
高选择性 / 低选择性
历史事故参数
边界/空结果/超大结果集
```

随机生成只有在有明确 property/oracle 时才是验证；无 Oracle 的随机轰炸只是 activity。

---

## 7. Migration / Backfill / Upgrade

`migration_rehearsal.py` 不是只检查 DDL 能执行：

```text
precondition
online lock contention
lock_timeout / statement_timeout
apply
postcondition
old-reader/new-writer compatibility
new-reader/old-writer compatibility
backfill correctness
backfill throttling/resume
reconciliation
down migration or forward-fix
rollback assertion
```

对于大表迁移，必须评估表 rewrite、ACCESS EXCLUSIVE、索引构建方式、FK/constraint validation、WAL 量、replica lag、autovacuum 影响和恢复策略。

`compatibility_audit.py` 负责 PostgreSQL major、扩展、collation version 和外部 upgrade/smoke command 证据。

---

## 8. 复制、CDC 与读取一致性

不同系统的一致性契约不同，因此 Gate 不写死“所有 replica 必须立即一致”，而是测试产品声明：

```text
eventual visibility budget
read-your-writes
monotonic reads
session consistency
CDC ordering
duplicate delivery
apply conflict
schema compatibility
slot/WAL retention
consumer replay/recovery
```

`replica_consistency_runner.py` 验证可见性与 monotonic contract；`logical_replication_audit.py` 检查 subscription worker、apply error/conflict 和 freshness。应用如果使用自己的路由/bookmark/session token，应把它纳入业务 binding，而不是只绕过应用直接连数据库。

---

## 9. Backup / Restore / PITR / Failover

必须区分：

```text
backup exists
backup artifact verifies
restore can start
PITR reaches target
business data/invariants correct
application can reconnect
RPO satisfied
RTO satisfied
failback works
```

`backup_restore_rehearsal.py`、`failover_recovery_runner.py`、`physical_integrity_runner.py` 分别提供执行层。`pg_verifybackup` 成功不允许替代真实 restore rehearsal；物理检查工具没有发现错误也不能被解释成“数学证明绝无 corruption”。

---

## 10. 数据库安全边界

`security_boundary_audit.py` 可检查：

```text
unexpected superuser / CREATEROLE
required RLS tables
SECURITY DEFINER function search_path
extension allow-list
custom privilege assertions
```

对于敏感系统还应在项目 binding 中补充：

```text
least privilege
cross-tenant isolation
RLS bypass attempts
DDL/DML privilege boundaries
secret rotation
TLS/auth configuration
auditability
backup access boundary
```

这是数据库发布资格的一部分，不应完全留给另一个安全流程。

---

## 11. Canary 与上线后闭环

`canary_evaluator.py` 比较 control/canary 的数据库与业务指标，并要求最小 bake window / sample count。建议至少覆盖：

```text
query/transaction P99
lock-wait P99
deadlock rate
40001 rate
DB errors
connection saturation
WAL/temp amplification
replica lag
business reconciliation mismatch
migration divergence
```

上线后不应该“发布成功即结束”。G30 要求关键业务 invariant 有 reconciliation / illegal-state detection / duplicate-effect detection / incident regression 路径。

---

## 12. PR / Nightly / Pre-release / Post-deploy 分层

### Developer / PR

适合确定性、快速、可重复证据：

```text
scope/risk
schema integrity
query/property
EXPLAIN
parameter-plan sensitivity
lock-order
关键 deterministic concurrency
isolation litmus
migration static/fast rehearsal
config/security checks
```

### Nightly

扩大状态空间：

```text
seeded contention exploration
多并发 benchmark
pg_stat_statements attribution
overload/recovery
较长 migration/backfill
soak
replica/CDC
physical integrity
```

### Pre-release / DB-R4+

执行高成本真实边界：

```text
sanitized production-shape replay
fault injection
restore/PITR
failover/failback
history/model checking
upgrade/extension compatibility
trusted statistical baseline
full qualification coverage
```

### Post-deploy

```text
canary/control
reconciliation
lock/deadlock/40001 guardrails
replica/CDC guardrails
migration divergence
rollback/pause criteria
incident evidence capture
```

---

## 13. 标准执行方式

先检查配置：

```bash
python3 scripts/sqlverify_config.py examples/sql-verification.toml \
  --root . --json
```

框架自测：

```bash
python3 scripts/selftest.py
```

工具环境检查：

```bash
python3 scripts/toolchain.py doctor --out .sqlverify/evidence/toolchain.json
```

执行主套件：

```bash
python3 scripts/run_suite.py \
  --config examples/sql-verification.toml \
  --root . \
  --evidence-dir .sqlverify/evidence \
  --qualification-manifest examples/qualification-manifest-prerelease.json
```

Release gate：

```bash
python3 scripts/release_gate.py \
  --policy examples/gate-policy-expert-prerelease.json \
  --evidence-dir .sqlverify/evidence \
  --out .sqlverify/report/gate.json
```

Qualification completeness：

```bash
python3 scripts/qualification_coverage_gate.py \
  --manifest examples/qualification-manifest-prerelease.json \
  --evidence-dir .sqlverify/evidence \
  --out .sqlverify/evidence/qualification.coverage.json
```

报告：

```bash
python3 scripts/evidence_report.py \
  --evidence-dir .sqlverify/evidence \
  --gate .sqlverify/report/gate.json \
  --config examples/sql-verification.toml \
  --out-md .sqlverify/report/report.md \
  --out-html .sqlverify/report/report.html
```

---

## 14. Evidence 最低契约

每个测试结果应尽可能包含：

```text
schema_version
kind
id
status
started_at / finished_at / elapsed_ms
exact command or binding
redacted environment identity
tool + version
PostgreSQL version/settings/schema fingerprint
seed / schedule / corpus / trace identity
observed metrics
violations
warnings
reproduction information
artifact hashes
```

**Evidence 是执行结果，不是测试计划。** “已经写了测试”“理论上应该通过”“代码看起来没问题”都不能成为 PASS。

---

## 15. Waiver 规则

高风险 Gate 如果暂时无法执行，只允许显式 waiver：

```text
claim/gate
reason
risk accepted
owner
expiry
containment
production detection
rollback/repair plan
```

永久 waiver、无 owner waiver、无 expiry waiver 不应进入专家级 release gate。

---

## 16. 安全原则

- 默认拒绝疑似生产数据库；测试库名必须匹配 safe pattern。
- 写 SQL 的 `EXPLAIN ANALYZE` 必须在隔离环境并显式授权。
- fault/failover/restore 等破坏性实验要求显式 isolated-environment acknowledgement。
- DSN/密码必须脱敏。
- production trace 只保留脱敏后的 workload shape；禁止把真实敏感数据打进包。
- destructive test、restore、PITR、failover、security test 应使用专用账号/环境。
- Gate 失败不能通过“提高阈值直到绿”解决；阈值变化本身必须接受 review。

---

## 17. 工具边界

本 Skill 会尽量复用项目已有基础设施，再补充缺失能力。常见底层工具：

```text
PostgreSQL 18 compatible test instance
psql
pgbench
pg_stat_statements
auto_explain (按需)
pg_amcheck / amcheck
pg_verifybackup
pg_basebackup / pg_restore / recovery harness
Testcontainers / Docker Compose
Toxiproxy or project fault harness
Jepsen/Elle or project history checker (DB-R4+ 按需)
OpenTelemetry / existing metrics backend
```

工具存在不等于必须使用；先定义 failure model 和 business oracle，再选择最便宜且足够有力的证据技术。

---

## 18. 目录

```text
Arc/arc:verify/references/db.md     最终操作规范（arc:verify db 模式入口）
Arc/arc:verify/references/db/*.md   G0–G31 门禁分组
docs/dbqual/README.zh-CN.md         中文使用说明（本文件）
Arc/arc:verify/scripts/dbqual/      包根；以下路径均相对于包根

BUILD-VALIDATION.md/json         包自身验证状态
VERSION / MANIFEST / CHECKSUMS

scripts/                         可执行 runners / gates / report
examples/                        配置、manifest、policy、trace 示例
references/                      专家标准与工程依据
templates/                       PR / nightly / prerelease / postdeploy 模板
schemas/                         Evidence / Gate schema
tests/                           框架自身测试
```

最关键的参考：

```text
references/final-gate-catalog.md
references/expert-risk-model.md
references/concurrency-deadlock-standard.md
references/isolation-litmus-standard.md
references/idempotency-ambiguous-commit-standard.md
references/workload-replay-and-overload.md
references/migration-and-recovery-standard.md
references/backup-restore-pitr-standard.md
references/replication-consistency-standard.md
references/security-boundary-standard.md
references/canary-postdeploy-standard.md
references/public-engineering-basis-v4.md
```

---

## 19. 最终验收哲学

这套体系不输出一个“数据库质量 96 分”。最终结论必须是可解释的：

```text
哪些业务声明被验证？
哪些真实失败模式被主动制造？
死锁/序列化失败是否按合同处理？
性能是否在代表性数据和压力下满足 SLO？
超过容量后是否可控并能够恢复？
迁移是否支持在线兼容与恢复？
备份是否真的恢复过？
主从/CDC 是否满足产品的一致性契约？
高风险事务历史是否通过独立模型检查？
生产 canary/reconciliation 如何发现测试漏网问题？
所有关键结论能否从 Evidence 重现？
```

只有这些问题有可信证据，才有资格叫“生产级数据库发布门禁”。
