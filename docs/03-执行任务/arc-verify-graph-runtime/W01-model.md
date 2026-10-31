# W01 版本模型

状态：`[x]`（2026-10-31，工具内核验收；业务 E2E 见 W06）

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P0：W02–W13 都读写 v2 模型，阻塞后续拆分与实现 |
| 难度 | D3 |
| 难度依据 | 追加 schema 与模型 hash 必须保持旧图/任务/RAW 的冻结含义；model.go、engine.go、binding.go 均已有调用。 |
| 目标面 | 本仓当前工作树、本地 Go/Turso；Rust 仅远程 .60；真实业务用已授权隔离目标 |
| 前置依赖 | 无，设计合同已写入 |
| 下游影响 | W02、W03、W07、W10、W13 及六技能交接与发布证据 |

## 目标与范围

需求为 [BG01–BG25](../../00-产品需求/06-业务模型驱动验证.md)。范围：model.go、db.go、engine.go、binding.go；新增模型类型/校验/持久化与 model_v2_test.go。新增文件属于既有工具模块，文件名在实施时回写。

共享设计与接口：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)。唯一状态源：[tracker](../arc-verify-graph-runtime.md)。不覆盖其他会话，不修改未授权远程业务数据，不用静态演示或历史 PASS 代替运行。

## 上下文检索

<Context-Queries>

```bash
arc-idx files 'tools/arc-verify' --format json
arc-idx search 'f:tools/arc-verify/ (validateGraph|importGraph|graphFingerprint)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 失败断言：三条验收测试首跑即通过（v2 行为已由先前会话实现），`gate.sh red` 拒绝记为 red；改用 6 个人为缺陷做敏感性证明，全部被抓到。记录：`.arc/artifacts/verify-model-v2/reports/sensitivity.md`。
- [x] 实施：本卡没有改生产代码；在 `tools/arc-verify/model_v2_test.go` 新增 `TestModelV1FingerprintMatchesPreV2Golden`、`TestModelV2ImportReadBackThroughCLI`、`TestModelV2RejectsIllegalStructure`（8 个子用例，逐个断言拒绝原因）。
- [x] 验证：
  - 合法三层模型经 CLI `init`→`import` 后由 `model` 读回，内容与导入一致。
  - 未知端点、多父、`contains` 成环/脱离根、反向 `implements`、测试依赖环、重复 ID、嵌套未知字段、重复 JSON 键，`validate` 与 `import` 都拒绝，且账本 5 张表无残留。
  - v1 hash 与 v2 之前的源码一致：黄金值由 `business-graph-workbench` gate 基线重建的旧源码算出，见 `.arc/artifacts/verify-model-v2/reports/v1-fingerprint-golden.md`。
  - 额外义务不丢：既有 `TestModelV2PlannerRetainsDeclaredObligations`、`TestModelV2GateCountsAllDeclaredObligations`。
  - 旧账本升级保留：既有 `integration_test.go` 的 legacy-v1 用例。
- [x] 回写：`go -C tools/arc-verify test -race -count=1 ./...` 与 `vet` 退出 0（`.arc/artifacts/verify-model-v2/logs/acceptance-go.txt`）；gate `--strict` 0 FAIL（`.arc/artifacts/verify-model-v2/reports/gate-20261031T055427Z.txt`）。未验：W10 若要求模型声明历史敏感属性，会追加字段，届时另起变更。

```bash
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行；前端命令在新增工作台工程执行。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
