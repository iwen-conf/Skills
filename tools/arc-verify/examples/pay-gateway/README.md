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
