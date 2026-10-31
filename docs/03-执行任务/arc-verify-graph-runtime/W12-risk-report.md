# W12 验收报告与风险报告分离

状态：`[?]`（2026-10-31）待人工检查：用户在真实终端运行一次 `arc-verify accept-risk --graph <id> --risk <id> --scope <范围>`，输入 `ACCEPT <id>` 和原话后，`report --kind risk` 显示 ACCEPTED_RISK。按规定 agent 不能用管道或 PTY 代填。

实现与卡片原文的差异：没有给 `risks` 表加列，改成追加 `risk_meta`（来源）和 `risk_events`（决策事件）两张表，和设计 05「旧表保持列形，新内容追加」一致；`risk_review` 先做成 `gate --risk-review` 开关，由 arc-policy 读取项目配置传入的部分另起 [W23](W23-policy-risk-review.md)。

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P1，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D2 |
| 难度依据 | 只动现有 `risks` 表（追加列）、`gate` 输出和 `risk` 子命令；边界清楚，不涉及执行链路。接受风险要人工确认，复用 `approve` 的交互约束。 |
| 目标面 | 本仓工作树、本地 Go 与原生 Turso（`tursogo`）；不碰远程业务数据 |
| 前置依赖 | 无（基于当前 `gate` 与 `risks` 表） |
| 下游影响 | W09 分流、W13 快照、W14 文档、arc-policy 可选风险审查门禁 |

## 目标与范围

需求：[BG20、BG26](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「补齐的义务、报告与探索」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

范围：`tools/arc-verify/db.go`（`risks` 追加 scope/decision_quote/evidence/updated 列，幂等迁移）、`engine.go`（`gate` 的 `open_risks`）、`main.go`（`risk` 子命令扩展、新增 `report`），新增 `risk_report.go`、`risk_report_test.go`。

- 风险状态：`OPEN` / `ACCEPTED_RISK`（必须有范围和用户原话，交互确认） / `PROMOTED`（已升级为 RULE，关联规则 ID）。
- 风险来源 `origin`（BG26）：`ai_inference` / `industry_practice` / `peer_reference`（附参照条目 ID）；导入缺 `origin` 的旧风险记为 `ai_inference` 并在报告里标注"来源未声明"。
- `report --graph --kind acceptance`：功能完整性、规则符合性、链路覆盖三项；不含风险数。
- `report --graph --kind risk`：风险、观察到的行为与证据、可能影响的业务、待用户决定事项。
- 项目 `.arc/policy.yaml` 打开 `risk_review` 时，OPEN 风险才阻止验收；默认不阻止。

不做：不让 AI 代为接受风险；不把风险计入缺陷数；不默认开启风险审查门禁；不改动 arc-policy 插件以外的门禁语义（插件读取开关另起卡）。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (open_risks|modelGate|accepted)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 失败测试：14 条（含 `TestPromoteRiskNeedsApprovedRule`、`TestOpenRiskCountExcludesDecided`、`TestHandoffShowsEffectiveRiskStatus`、`TestRiskOriginStoredAndReported`）前 13 条在签名桩上全部断言失败，记录为 red-003（`.arc/artifacts/verify-risk-report/red/`）。
- [x] 实现：
  - `risk_report.go` 新增 `riskReport`、`acceptRisk`、`promoteRisk`、`acceptanceReport`、`applyRiskReview`、`riskRows`、`openRisks`；决策写入用 `transition` 守卫，状态编码在 WHERE 里。
  - `model.go`：`Risk` 增加 `origin`/`reference` 字段和校验。
  - `db.go`：追加 `risk_meta`、`risk_events` 两张表和索引。
  - `engine.go`：导入时写来源；gate 只数未决风险。
  - `model_analysis.go`：v2 gate 同步改为只数未决风险。
  - `model_store.go`：读回带来源。
  - `handoff.go`、`main.go`：风险列表显示有效状态；新增 `report`、`accept-risk`、`promote-risk` 命令和 `--risk-review` 开关。
- [x] 验证：
  - 全量 `go test -race -count=1 ./...` 153 条通过，`vet` 通过。
  - 编译后的 CLI 在临时库上跑通：风险报告、管道接受被拒（退出码 1）、验收报告无风险字段、`--risk-review` 才出现 `risk_review_blocked`、非法 `--kind` 报错。日志：`.arc/artifacts/verify-risk-report/logs/real-entry.txt`。
  - gate `--strict --require-red` 0 FAIL，4 条 red-hygiene 提醒已核对：签名桩、改测试接收方、`model_test.go` 的 gofmt。报告：`.arc/artifacts/verify-risk-report/reports/gate-20261031T061047Z.txt`。
- [x] 自查修复：`acceptanceReport` 原先按 `FOUND` 过滤扫描结果，而 `scan` 的真实取值是 `PRESENT`/`NOT_FOUND`/`UNKNOWN`，导致已找到的入口也被列为候选缺口。先补 `TestAcceptanceReportGapsFollowScanObservations` 并确认失败（red-004），再改成按 `PRESENT` 过滤；设计 05 与 W07 卡里的同类笔误一并更正。复查 gate 0 FAIL：`.arc/artifacts/verify-risk-report/reports/gate-20261031T061626Z.txt`。
- [x] 回写 tracker。

## 验收

```bash
go -C tools/arc-verify test -race -count=1 -run 'AcceptanceReport|RiskReport|AcceptRisk|RiskReview|RisksMigration|RiskOrigin|LegacyRiskOrigin' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行（不是 Rust，允许本机编译）。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
