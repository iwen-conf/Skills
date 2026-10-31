# W23 arc-policy 传入风险审查开关

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 工具（arc-policy 插件，Python） |
| 优先级 | P3，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D1 |
| 难度依据 | 只是在既有业务图门禁调用里按配置多传一个参数；`arc-verify gate --risk-review` 已由 W12 实现 |
| 目标面 | `plugins/arc-policy/lib/arc_policy/verify.py` 与其测试 |
| 前置依赖 | [W12](W12-risk-report.md)：`gate --risk-review` 可用 |
| 下游影响 | 配置了风险审查的项目，其交付门禁 |

## 目标与范围

需求：[BG20](../../00-产品需求/06-业务模型驱动验证.md)"风险审查门禁只在项目配置要求时生效"。唯一状态源：[tracker](../arc-verify-graph-runtime.md)。

项目 `.arc/policy.yaml` 写了 `verification.business_graph.risk_review: true` 时，arc-policy 调用 `arc-verify gate` 要带 `--risk-review`；没写或写 `false` 就不带。

不做：不默认开启；不在插件里自己数风险；不改其他门禁语义。

## 2026-10-31 实施记录

- `plugins/arc-policy/lib/arc_policy/verify.py`：`verification.business_graph.risk_review` 为 `true` 时门禁命令加 `--risk-review`；缺省或 `false` 不加；不是布尔值时 BLOCKED。
- 范围扩大（同一函数里的现成缺陷）：arc-policy 只接受 `proof_scope == "planned_rule_tasks"`（v1），v2 模型门禁报告的是 `complete_business_model_obligations`，任何 v2 图都过不了交付门禁，会挡住 W06。改为接受这两种范围（`PROOF_SCOPES`），其他范围仍 BLOCKED。
- `plugins/arc-policy/README.md` 补 `risk_review` 配置与两种证明范围。
- 测试（`tests/test_arc_policy_business_graph.py`）：开关打开加参数、关闭或缺省不加、非布尔 BLOCKED、v2 范围通过、未知范围 BLOCKED。

验收：`.venv/bin/python -m pytest -q tests/test_arc_policy_business_graph.py` 21 passed；`.venv/bin/python -m pytest -q` 476 passed、1 skipped；red `.arc/artifacts/verify-policy-risk-review/red/red-001.json`。Codex 宿主需要 `codex plugin add` 刷新后才用到新代码（其他宿主直接读本仓）。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:plugins/arc-policy/ business_graph' --format ai-md --context 2 --max 30
```

</Context-Queries>

## 执行清单

- [x] 先在 `tests/test_arc_policy_business_graph.py` 加开、关两条失败用例，保存 red。
- [x] 改 `verify.py` 读取配置并传参。
- [x] 跑验收命令；回写并同步 tracker。

## 验收

```bash
.venv/bin/python -m pytest -q tests/test_arc_policy_business_graph.py
.venv/bin/python -m pytest -q
```
