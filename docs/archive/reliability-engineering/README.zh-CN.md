# Industrial Business Reliability Engineer v3.0

这不是一个“多写测试、刷覆盖率”的 Skill。

目标是：**把真实业务预期变成可执行、可复现、可阻断发布的验证证据，在生产事故发生之前尽可能把错误逼出来。**

## v3.0 的核心变化

v2.x 主要解决“应该测试什么”；v3.0 开始解决“怎样把业务测试真正工业化执行”。

标准链路：

```text
业务需求/设计/事故
  ↓
Business Verification Plan
  ↓
业务规则 + Expected/Forbidden + Oracle
  ↓
Execution Bindings
  ↓
隔离 Sandbox / 真实依赖
  ↓
真实 API / SQL / Job / 并发 / 故障执行
  ↓
Evidence JSON
  ↓
Release Gate
  ↓
生产检测 / Reconciliation / Incident Regression
```

### 不做“综合质量分数”

系统不会因为 3000 个低风险测试通过，就把一个缺失的关键业务验证“平均掉”。

只使用事实状态：

```text
PASS
FAIL
BLOCKED
NOT_RUN
FLAKY
```

## 1. 业务计划不是文档摆设

业务测试计划声明真实业务规则：

- 谁能做什么、不能做什么；
- 什么状态允许转换；
- 哪些业务结果必须发生；
- 哪些结果绝不能发生；
- 重试/重复/超时/取消/恢复是什么语义；
- 哪些业务会共享状态并互相影响；
- 哪些业务结果由数据库、事件、审计、模型或其他独立观察者判断。

示例：`examples/business-test-plan.toml`

检查：

```bash
python3 scripts/business_test_plan.py validate \
  --plan examples/business-test-plan.toml
```

v3.0 新增 `[applicability]`：显式声明 `stateful/concurrent/retryable/...` 是否适用。只有适用的能力才会强制要求对应模型和测试，避免模板官僚化。

## 2. 业务规则与技术执行分离

业务计划只定义“什么叫正确”。

执行绑定定义“这个仓库里怎么跑”：

```text
HTTP
SQL
repository command/test binary
并发请求
真实 PostgreSQL 状态查询
```

示例：`examples/business-scenario-bindings.toml`

验证 binding：

```bash
python3 scripts/business_scenario_runner.py validate \
  --plan examples/business-test-plan.toml \
  --binding examples/business-scenario-bindings.toml
```

执行一个场景并保存证据：

```bash
python3 scripts/business_scenario_runner.py run \
  --binding examples/business-scenario-bindings.toml \
  --scenario S-002 \
  --evidence-out .reliability/evidence/S-002.json
```

同一个业务规则可以在实现重构后继续保留，只替换执行 binding。

## 3. 真实隔离环境

`examples/sandbox.toml` 描述项目已有的 Compose/Testcontainers/dev-stack 生命周期：

```text
bootstrap
reset
healthcheck
run tests
teardown
```

使用：

```bash
python3 scripts/sandbox_runner.py \
  --contract examples/sandbox.toml \
  --root . \
  -- <your test command>
```

Skill 优先复用项目已有测试基础设施，不强迫每个项目建设新的平台。

## 4. Release Gate 不是“测试很多就算过”

`business_verification_gate.py` 检查：

```text
业务计划是否完整
release-required scenario 是否存在
automated scenario 是否有真实 binding
需要 evidence 时是否真的执行且 PASS
manual scenario 是否有明确 procedure/evidence
```

例如：

```bash
python3 scripts/business_verification_gate.py \
  --plan business-test-plan.toml \
  --binding business-scenario-bindings.toml \
  --evidence-dir .reliability/evidence \
  --require-evidence
```

缺一个关键场景就 FAIL，不会计算“总体 92 分所以可以上线”。

## 5. 业务并发/事务是一级测试对象

保留并增强：

```text
跨业务锁顺序分析
同业务镜像参数分析
真实 PostgreSQL barrier 交错
SQLSTATE 40001 / 40P01
pg_stat_activity / pg_blocking_pids()
事务时长 / step 时长预算
真实 contention workload
```

并增加 `require_observer=true`：关键事务场景如果要求锁观察器，观察器本身失败时不能继续把场景标记成功。

## 6. 工具链安装

仍支持按能力准备：

Rust：

```text
nextest
llvm-cov
cargo-mutants
cargo-fuzz
proptest
Loom
Miri
cargo-audit
Testcontainers
```

Go：

```text
go test / race / fuzz / coverage
gotestsum
govulncheck
staticcheck
Testcontainers
```

公共：

```text
Toxiproxy
Psycopg 3 transaction harness
pgbench
```

但 v3.0 明确规定：**先有 failure model，再装工具。**

`plan` 模式现在使用浅探测，不连接 Docker daemon/运行重型环境检查；真正的 authoritative probe 放在 `doctor/install`，让 Skill 自己的单元测试更 hermetic。

## 7. 目录

```text
SKILL.md
README.zh-CN.md
VERSION

scripts/
  business_test_plan.py
  business_scenario_runner.py
  business_verification_gate.py
  sandbox_runner.py
  business_txn_lab.py
  verification_toolchain.py
  reliability_gate.py
  seed_runner.py

templates/
  business-test-plan.md
  business-transaction-review.md
  reliability-review.md
  postmortem.md

examples/
  business-test-plan.toml
  business-scenario-bindings.toml
  sandbox.toml
  business-transactions.toml
  postgres-deadlock-scenario.toml

references/
  industrial-operating-model.md
  business-test-planning.md
  business-transaction-verification.md
  engineering-basis.md
  tool-acquisition.md
```

## 8. 使用原则

对于真实项目，Skill 的第一目标是解决当前风险，而不是把所有框架都引入项目。

例如一个普通 CRUD 改动可能只需要：

```text
业务规则
边界
真实 DB integration
```

而一个报名/库存/支付/审批/客服/设备控制的跨业务改动，可能需要：

```text
状态模型
Decision Table
跨业务并发
真实数据库事务
重复/超时/补偿
事件/审计 Oracle
Sandbox E2E
Release Gate
```

测试深度由业务风险和系统结构决定。
