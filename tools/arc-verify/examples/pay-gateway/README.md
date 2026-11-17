# pay_gateway：真实网关、独立 PostgreSQL、受控机构

本示例把 Arc 接到 pay_gateway 已有 `browser_fixture`，不实现第二个支付网关。
固定源码为 [`e4f054fc7bb06e091be62d6765bb29b4348ad651`](https://github.com/iwen-conf/pay_gateway/tree/e4f054fc7bb06e091be62d6765bb29b4348ad651)。
**当前仓库交付的是可运行适配器及模型；没有可用隔离环境时，产品验收状态是 BLOCKED。**

## 验证哪一条业务线

核心场景是：独立观察空实例 → 真实创建支付 → 机构确认支付事件 → 通过正常查单入口学习结果
→ 独立 SQL 确认成功事实 → 提交退款 → 再用同一退款号提交 → 验证 409、已有资源 ID、仅一条
退款索引和一次机构出站 → 独立检查后保留实际现场。

目标退款动作前的准备由 `RunnerPreparation` 根据真实状态选择，不靠修改 JSON 里的 state。
机构事件和正常查单是明确的准备步骤，记录实际请求和响应，之后必须重新查询数据库。
`duplicate` 场景可以在核心场景完成后，用同一 caller/实例再次独立读取并绑定同一支付和退款，
验证新的重复请求。跨 run 重新读取实体；不会凭空继承之前的 session 或变量。

| 边界 | 实际来源与限制 |
|---|---|
| 业务入口 | 原项目 REST router、usecase、机构适配器 |
| 业务持久化 | 原项目隔离 PostgreSQL，Arc 的 SQL 连接一律 `BEGIN READ ONLY` |
| 原始机构请求 | 原项目 controlled provider 的实际 HTTP 收件记录，另核 PostgreSQL 出站记录 |
| 机构成功事件 | 已拥有测试实例的 `/__qa/provider_paid`；不代表访问真实支付机构或发生真实资金流 |
| 通知 | 受控测试传输；不声称真实商户 HTTPS 投递成功 |
| 资金状态 | 从通知/回调事实读取；`p_orders` 只作索引，绝不把“有订单行”当作“支付成功” |
| 覆盖 | 固定启用 caller、汇聚账户、100 分、CNY、指定场景；其他权限、金额、故障、机构和时序仍未验收 |

合同来源：

- [API10：业务方接口](https://github.com/iwen-conf/pay_gateway/blob/e4f054fc7bb06e091be62d6765bb29b4348ad651/docs/02-API记录/10-业务方接入接口.md)。
- [退款 usecase](https://github.com/iwen-conf/pay_gateway/blob/e4f054fc7bb06e091be62d6765bb29b4348ad651/src/usecase/forwarding/refunds.rs)。网关不自行裁决原单已支付或累计退款余额；不能擅自添加这两个拒绝分支。
- [受控机构协议](https://github.com/iwen-conf/pay_gateway/blob/e4f054fc7bb06e091be62d6765bb29b4348ad651/tests/fixtures/ratio_providers.rs)。这里要求原单先由机构确认，属于本受控环境的准备前提。
- [真实测试 host](https://github.com/iwen-conf/pay_gateway/blob/e4f054fc7bb06e091be62d6765bb29b4348ad651/tests/fixtures/browser_host.rs) 和 [隔离测试约定](https://github.com/iwen-conf/pay_gateway/blob/e4f054fc7bb06e091be62d6765bb29b4348ad651/tests/fixtures/README.md)。

## 0. 没有支付环境时，先生成可审阅的业务系谱

模型验证、导入和静态工作台导出只需要 Arc 与已经构建的工作台资源；不需要支付服务、caller
凭据或 PostgreSQL。以下流程使用一个全新的本地审阅账本，避免把历史运行混入本次模型展示。
需要已安装的统一 `arc`；工作台依赖已按 [构建说明](../../workbench/README.md#构建与前端检查)
准备好时，先在 `tools/arc-verify/workbench` 执行 `bun run build`。

```bash
ARC_PAY_SKILLS=/absolute/Skills
ARC_PAY_REVIEW=$(mktemp -d "${TMPDIR:-/tmp}/arc-pay-model.XXXXXX")
git -C "$ARC_PAY_REVIEW" init --initial-branch=main
arc db init --repo "$ARC_PAY_REVIEW"
cd "$ARC_PAY_REVIEW"
arc verify init
arc verify validate --file "$ARC_PAY_SKILLS/tools/arc-verify/examples/pay-gateway/model.json"
arc verify contract check --file "$ARC_PAY_SKILLS/tools/arc-verify/examples/pay-gateway/model.json"
arc verify import --file "$ARC_PAY_SKILLS/tools/arc-verify/examples/pay-gateway/model.json"
arc verify snapshot \
  --graph pay-gateway-index-fixture-v2 \
  --assets "$ARC_PAY_SKILLS/tools/arc-verify/workbench/dist" \
  --out "$ARC_PAY_REVIEW/workbench"
```

打开 `$ARC_PAY_REVIEW/workbench/index.html`，保留整个导出目录。这里导出的是实际 CLI 从账本读取
的模型及来源，没有执行目标业务，也没有执行 `approve`。新账本中的模型/规则批准仍为 false，
`runs` 为空，`available_runs=0`；省略环境、轮次和运行参数，不创建虚构账号、实体或观测记录。
有限域检查的 `PROVEN_IN_SCOPE` 与支付 E2E 的 `NOT_RUN/BLOCKED` 可以同时成立。

### 四个分支及其唯一预期

四个分支共同限定：启用 caller、可用的启用 `ca_browser_huiju` 账户、经身份核验的 owned
controlled 环境，以及实际捕获的请求 `amount=100`、`currency=CNY`、
`channel_account_id=ca_browser_huiju`。模型只声明这一有限场景；不同有效金额或其他账户不自动
变成拒绝分支。前态名称表示本实例的引用索引投影。

| 分支 | 独立观察的前态与额外条件 | 唯一预期及关键独立断言 |
| --- | --- | --- |
| `BR_CREATE_PAYMENT_INDEX` | `EMPTY` | HTTP 200；`PAYMENT_INDEXED`；返回 ID 与 SQL 支付 ID 一致；支付索引、出站、机构收件各一次；没有退款索引或退款出站 |
| `BR_CREATE_REFUND_INDEX` | `PAYMENT_INDEXED`，且本受控机构的支付成功已被网关学习并独立回读 | HTTP 200、`processing`；`REFUND_INDEXED`；返回 ID 与 SQL 退款 ID 一致；父支付、caller、账户关系正确；退款出站和机构收件各一次 |
| `BR_DUPLICATE_REFUND` | `REFUND_INDEXED`，普通重复提交同一退款号 | HTTP 409、`duplicate_order`；保持原退款索引和资源 ID；索引、出站、机构收件仍各一次 |
| `BR_MISSING_PARENT` | `EMPTY`，原支付不存在 | HTTP 404、`resource_not_found`；仍为 `EMPTY`；没有支付/退款索引和对应出站、机构收件 |

退款普通重复和缺原单是合法拒绝转移，可以在状态图中形成自环；它们不是准备 DAG 的循环。
成功退款中的机构确认条件仅属于这个 controlled fixture，不能解释为网关通用的资金资格规则。

### 准备依赖和条件来源

| 条件或状态 | 来源 | 系谱中的含义 |
| --- | --- | --- |
| caller 与账户可用 | 独立只读 SQL，加当前 caller 的鉴权账户目录响应 | 外部观察条件；模型没有凭空创建账号或授予权限的准备动作 |
| controlled 环境身份 | `/__qa/identity`、run/revision/binary/数据库指纹比对及 SQL 数据库身份 | 环境准入与观察根；没有观察时不能显示已满足 |
| 请求金额、币种、账户 | 本次真实 HTTP `resolvedBody` 的 `request-capture` | 待执行动作的输入；不是准备动作产出的业务事实 |
| `EMPTY` / `PAYMENT_INDEXED` / `REFUND_INDEXED` | 当前 caller、支付号、退款号限定的 SQL 索引行 | 真实前态投影；不是 `paid` 或退款资金终态 |
| `payment.exists`、`payment.rows`、支付出站与收件次数 | `PREP_CREATE_PAYMENT` 后的独立 SQL 与机构收件观察 | 真实 `POST /api/v1/payments` 能建立的准备效果；模型中的 effects 仍须实际回读 |
| `provider.payment_confirmed` | `PREP_CONFIRM_PAYMENT` 后独立读取 `p_notify_jobs` / `p_callback_receipts` 的成功事实 | 此属性是网关已学习机构成功事实的投影，不从订单行或准备命令返回值推定 |

`PREP_CONFIRM_PAYMENT` 显式依赖 `PREP_CREATE_PAYMENT`。前者需要原支付存在、退款不存在、
机构恰好收单一次且尚未观察到成功事实，执行受控 `provider_paid` 事件和正常鉴权查询，之后
再次读取 SQL。它仍是一个组合准备动作：RAW 保存两次实际请求与响应，模型没有声明两条独立的
支付状态转移；工作台不能替它编造中间事件、异步等待或已经发生的结果。

准备系谱沿属性条件展示候选建立动作、明确的 `depends_on` 和外部观察根。候选动作有来源和
预计效果，不代表一定可执行；运行时仍要匹配真实前态、验证身份、执行并独立回读。
`core` / `refund` 的目标前态由这两个准备动作建立。`core` 随后连续执行退款与同号重复，
`duplicate` 单独运行则只读重绑定已有退款。当前模型没有“创建退款”的 PreparationAction，
因此不能声称可以从空实例自动准备任意 duplicate 目标。`create` / `missing-parent` 所需空实例
来自 owned fixture 的建立和实际观察，不能由修改声明值或清除历史得到。

模型视图中的能力树、条件依赖和有环状态转移可以独立审阅；只有后续真实运行的账本才能填充
Actual、账号/实体绑定、首次偏离、等待和逐项证据。没有运行时，图中的未观察条件保持未知。

## 1. 准备原项目已有隔离环境

在固定版本的干净 pay_gateway checkout 中，按其测试说明提供
`tests/fixtures/.isolated-postgres.local.json`。必须是远程隔离 PostgreSQL，
`environment_kind=isolated_test`，数据库名带 `test` 或 `isolated`，包含 purpose/run_id。
本示例还需 Python 3.11+、`psql`、Rust 工具链和已安装的统一 `arc`。

在 pay_gateway 仓库启动已有 fixture，使用自己这次运行的唯一 ID：

```bash
QA_STORAGE=postgres \
QA_RUN_ID=arc_pay_review_001 \
QA_REVISION=e4f054fc7bb06e091be62d6765bb29b4348ad651 \
QA_READY_FILE=/absolute/private/arc_pay_ready.json \
cargo run --locked --features integration-tests --example browser_fixture
```

从 ready 文件取得真实 loopback `base_url`。不使用生产地址，不启动本地 PostgreSQL。
fixture 自己创建、拥有并最终清理 case database；Arc 不执行准备 SQL 或删除业务流水。
`preflight` 核对真实 identity、storage、run、revision、二进制 SHA、PG 指纹和本地源码。
memory 模式、其他运行、与绑定构建身份不符的二进制和错误数据库都会阻断。
仍须按上述固定源码构建；启动变量 `QA_REVISION` 与本地 SHA 对照不是独立的编译来源证明。

```bash
python3 /absolute/Skills/tools/arc-verify/examples/pay-gateway/adapter.py preflight \
  --pay-repo /absolute/pay_gateway --target http://127.0.0.1:ACTUAL_PORT \
  --run-id arc_pay_review_001
```

## 2. 建立一次 owned fixture，并生成真实 Runner

`bootstrap` 会调用同一 QA_RUN_ID 的 reset。只对专门分配给本次验收的 fixture 使用，
不要复用别人的正在运行实例。它保留新 caller 的私有凭据文件，公开配置只保留 env 引用。
输出目录必须不存在；失败或重跑不得覆盖旧证据。

```bash
python3 /absolute/Skills/tools/arc-verify/examples/pay-gateway/adapter.py bootstrap \
  --pay-repo /absolute/pay_gateway --target http://127.0.0.1:ACTUAL_PORT \
  --run-id arc_pay_review_001 --out /absolute/pay_gateway/.arc/pay-review/instance-001

python3 /absolute/Skills/tools/arc-verify/examples/pay-gateway/bundle.py \
  --session /absolute/pay_gateway/.arc/pay-review/instance-001/session.json \
  --out /absolute/pay_gateway/.arc/pay-review/core-001 --scenario core
```

此时只是环境绑定和 runner 生成，**没有业务 PASS**。其他场景是 `create`、`refund`、
`missing-parent`、`duplicate`。其中 create/missing-parent 要求实际空实例；后续 duplicate
要求已有退款实例。选择错误不会自动更换分支或重置现场。

## 3. 导入并确认有来源的有限契约

在 pay_gateway 仓库运行。已有统一库不再 init；旧 verify 库按现有显式迁移流程激活。

```bash
arc db init --repo /absolute/pay_gateway
arc verify init
arc verify validate --file /absolute/Skills/tools/arc-verify/examples/pay-gateway/model.json
arc verify contract check --file /absolute/Skills/tools/arc-verify/examples/pay-gateway/model.json
arc verify import --file /absolute/Skills/tools/arc-verify/examples/pay-gateway/model.json
arc verify approve --graph pay-gateway-index-fixture-v2
arc verify approve --graph pay-gateway-index-fixture-v2 --rule RULE_CREATE_PAYMENT_INDEX
arc verify approve --graph pay-gateway-index-fixture-v2 --rule RULE_REFUND_INDEX
```

`approve` 是现有 CLI 的交互式模型确认门禁；脚本不代填人工确认、不从产品实际输出批准规则。
已有同版本目录与规则确认可复用，不重复导入或批准。`PROVEN_IN_SCOPE` 只证明声明的有限输入
分区，没有证明全部可达路径或整个支付系统。

本次示例为 `pay-gateway-index-fixture-v2`，新增有来源的 `json_i64` 金额编码；旧 v1 模型和
执行证据保持原样。v2 可以独立导入，不要求先有 v1。执行脚本分别用 Go 内核检查随包模型和
账本模型，并比较规范合同 SHA-256；即使 ID 相同，只要规则、编码、来源或版本不同，也会在
目标执行前 BLOCKED。模型已有不同内容时，应显式建立新版本并正常确认，不覆盖或自动批准。

## 4. 执行、独立回读、导出原生工作台

先按 `tools/arc-verify/workbench` 的构建命令生成 `dist`，然后：

```bash
python3 /absolute/Skills/tools/arc-verify/examples/pay-gateway/execute.py \
  --session /absolute/pay_gateway/.arc/pay-review/instance-001/session.json \
  --bundle /absolute/pay_gateway/.arc/pay-review/core-001 \
  --assets /absolute/Skills/tools/arc-verify/workbench/dist
```

执行前后再次核对干净源码、实际 fixture 二进制和绑定数据库，避免测试旧进程却把 PASS 关联到新源码。
执行器注册原有 fixtures/runner、冻结真实轮次、执行既有 `arc verify execute`，保存 explain
与实际 snapshot。失败返回非零；输出只包含真实 run/round/证据路径，不打印 caller key 或 DSN。
`verdict` 保留实际业务判定；证据不完整、请求的工作台导出失败或源码绑定变化时，
`delivery_status=BLOCKED` 并返回非零，不覆盖已记录的实际结果。
HTTP 请求参数从实际解析后的 body 冻结，响应字段从 response capture 获取，数据库和机构收件
通过独立 Oracle 验证。完成后 `checkpoint` 保存合法后态，不删除支付退款历史。

`request.amount` 显式声明 `type: integer, encoding: json_i64`，依据固定源码
[严格整数解析器](https://github.com/iwen-conf/pay_gateway/blob/e4f054fc7bb06e091be62d6765bb29b4348ad651/src/interface/restful/dto/parse.rs#L100-L159)。
Arc 直接检查实际请求中的 JSON token，接受本例的 `100`，拒绝将 `100.0`、`1e2`、`"100"`
或超出 i64 的数值冻结为合法 100 分请求。其他未限定编码的 `integer` 属性仍使用原有数学整数语义。
词法不合要求或缺值时，本例在目标动作前 BLOCKED；合法整数但超出已声明 100 分场景时是
SPECIFICATION_GAP。这些是 Arc 契约检查，不代表已执行并验收支付 API 的 400/422 拒绝路径。

接着生成 `--scenario duplicate` 的新 bundle，在同一 session 上执行即可检查重复操作。
需要全新空实例时新建 owned fixture；之前的快照/数据库/RAW 必须先保留，不能把 reset 当作恢复证明。

打开产物 `workbench/index.html` 可切换能力树、准备依赖、状态和运行轨迹。图中的 Actual 来自
该次 Arc 账本。静态页面没有执行按钮，不需要本地 Web 服务。

## 工具回归与尚未验收的项目项

```bash
cd /absolute/Skills
python3 -m unittest discover -s tools/arc-verify/examples/pay-gateway -p 'test_*.py' -v
go -C tools/arc-verify test -run 'TestPayGateway' -count=1 .
```

这些测试检查适配器隔离、原始关系和 runner/model 对接，不能替代真实支付项目执行。
未绑定真实环境时保留 BLOCKED；不得用 local HTTP 工具夹具的 PASS 冒充 pay_gateway PASS。
真实机构、真实商户通知、回调验签/乱序、资金竞争、完整权限矩阵、无限金额域与部署环境均需
各自模型及执行证据。本例不会根据旧接口名或状态文字生成未确认的业务结论。

## 2026-11-17 源码复核与当前验收阻断

本次只读核对的 pay_gateway main 为
[`56154994b09af0cc439356ba1fb72916a9c9fc65`](https://github.com/iwen-conf/pay_gateway/commit/56154994b09af0cc439356ba1fb72916a9c9fc65)，
其父提交正是本例固定的 `e4f054fc7bb06e091be62d6765bb29b4348ad651`。
本节记录版本差异，不更新 `model.json`、适配器中的 `REVISION`、原有规则确认或历史证据。

| 最新变化 | 对本例的具体影响 |
| --- | --- |
| 易宝绑卡请求隔离与旧请求归属检查 | 本例没有绑卡动作；不新增或宣称已覆盖绑卡分支 |
| 首信易支付/退款/出款，以及汇聚出款的机构流水冲突检查 | 本例使用汇聚 Payment/Refund；新增 `provider_transaction_id_error` 对这两类仍返回无额外约束，不能把其他 provider/kind 的拒绝规则套用到本例 |
| PostgreSQL `merge_refs` 改为锁住当前订单后复核再提交；回调也在提交锁内复核 | 共用持久化与观察路径变了；即使本例预期未发现语义变化，也需要针对新构建重新运行，不继承旧提交的执行资格 |
| 概览查询的当前/上一时间窗独立加载 | 本例没有管理概览查询；不影响已声明的四分支，也不代表管理概览已验收 |

上述判断依据 [本次提交差异](https://github.com/iwen-conf/pay_gateway/compare/e4f054fc7bb06e091be62d6765bb29b4348ad651...56154994b09af0cc439356ba1fb72916a9c9fc65)、
[流水检查的 provider/kind 范围](https://github.com/iwen-conf/pay_gateway/blob/56154994b09af0cc439356ba1fb72916a9c9fc65/src/usecase/forwarding/orders.rs#L124-L153)
和 [当前 PostgreSQL 引用合并](https://github.com/iwen-conf/pay_gateway/blob/56154994b09af0cc439356ba1fb72916a9c9fc65/src/infrastructure/gateways/persistence/postgres/repository/orders.rs)。
API10、退款 usecase、`browser_host.rs` 和 `ratio_providers.rs` 在这一次提交中没有变化；
[当前退款入口](https://github.com/iwen-conf/pay_gateway/blob/56154994b09af0cc439356ba1fb72916a9c9fc65/src/usecase/forwarding/refunds.rs#L57-L127)
仍将资金资格交给机构，普通重复在出站前拒绝；proven-unsent 特殊恢复仍属于本例之外。

本次工作环境的实际检查结果：

| 所需条件 | 2026-11-17 检查结果 | 可解除阻断的材料 |
| --- | --- | --- |
| 干净、固定版本的完整 pay_gateway checkout | 可见 pay 源码目录只是局部快照，均非 Git 仓库 | 完整 checkout，HEAD 与本例固定版本一致，tracked source 干净 |
| Rust 构建工具与候选 fixture 二进制 | PATH 中没有 `cargo` / `rustc`，没有本次绑定的 `browser_fixture` 二进制 | 原项目支持的工具链与从已审阅源码构建的 fixture |
| 独立 SQL 观察客户端 | PATH 中没有 `psql` | 可用的 PostgreSQL 客户端 |
| 已授权远程隔离数据库 | 两个可见 pay 源码目录均没有 `tests/fixtures/.isolated-postgres.local.json` | 项目已有、获准用于本次测试的隔离配置与服务端身份核验 |
| 活的 owned fixture | 没有本次 ready 文件、loopback 地址及与源码/数据库绑定的运行身份 | 按第 1 节启动的目标及 `preflight` 通过结果 |

因此本次 pay_gateway 产品 E2E 保持 **BLOCKED / NOT_RUN**；没有发起支付、退款、建库或 reset。
[原项目当前隔离约定](https://github.com/iwen-conf/pay_gateway/blob/56154994b09af0cc439356ba1fb72916a9c9fc65/tests/fixtures/README.md#独立数据库配置)
明确使用远程隔离 PostgreSQL，本机只使用客户端；不能用新建本地数据库、memory 模式或工具自测
替代这项验收。第 0 节的真实 CLI 导出仍可完成，结果仅证明模型和工作台链路可用。
