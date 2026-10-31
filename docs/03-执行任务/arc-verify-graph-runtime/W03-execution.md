# W03 执行与恢复

状态：`[x]`

2026-10-31 审计：工作树已有未提交实现 `execute.go`、`runner.go`、`execution_store.go`、`runner_process_unix.go`，来源会话未记录在本卡，完成度与验收均未核；开工先对照本卡清单逐项核实，再补 red 与验收。

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P1，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D3 |
| 难度依据 | 真实动作、独立读回、参与主体、截止和外部恢复共享一个运行绑定与 fixture 生命周期。 |
| 目标面 | 本仓当前工作树、本地 Go/Turso；Rust 仅远程 .60；真实业务用已授权隔离目标 |
| 前置依赖 | W01、W02：完整义务与依赖可用 |
| 下游影响 | 工作台、真实 runner、六技能交接与发布证据 |

## 目标与范围

需求为 [BG01–BG25](../../00-产品需求/06-业务模型驱动验证.md)。范围：binding.go、lifecycle.go、track.go；新增 execute runner/恢复协议和执行测试。新增文件属于既有工具模块，文件名在实施时回写。

共享设计与接口：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)。唯一状态源：[tracker](../arc-verify-graph-runtime.md)。不覆盖其他会话，不修改未授权远程业务数据，不用静态演示或历史 PASS 代替运行。

## 2026-10-31 实施记录

核实既有实现（`execute.go`、`runner.go`、`execution_store.go`、`runner_process_unix.go`）对照清单的测试覆盖：

| 清单项 | 测试 |
| --- | --- |
| 动作前真实前提 | `TestExecuteWrongPrincipalBlocksBeforeAction`、`TestExecuteVerifiesIsolationBeforeAnyFixtureWrite`、`TestExecuteConcurrentParticipantPreconditionsAndOracleIdentity`（已有） |
| HTTP/CLI 独立 Oracle | `TestExecuteUsesIndependentOracleAndRestoresFixture`、`TestCommandCaptureFailureCannotPass`、`TestCommandScriptMustBeInFrozenInputs`（已有）；新增 `TestRunnerRejectsActionResponseAsOracle` |
| 属性反例 / 拒绝 / 超时保留证据 | `TestExecutePropertyCounterexampleCannotPassAndFailedRecoveryKeepsLocks`、`TestExecuteDeadlineStopsActionAndRecordsRestoration`（已有）；新增 `TestExecuteModelRejectionNeedsUnchangedState` |
| 恢复失败不释放 fixture、不覆盖原 FAIL | `TestExecuteFailedRecoveryHasDurableRAW`、`TestModelRetryUsesCapturedRecoveryAndKeepsFailure`（已有） |
| 参与账号 / 轮次绑定 | 并发参与者测试（已有）；新增 `TestExecuteRefusesStaleRound`（候选变化后不建 run） |

修复一个真实缺陷：模型路径下，服务端**错误放行**本应拒绝的操作时，结果被记成 BLOCKED 而不是 FAIL。根因：`track.go` 的 `StepRAW.Denied` 标签带 `omitempty`，执行引擎写 RAW 时把"观察到未被拒绝"省略，`parseStepRAW` 对期望 `UNCHANGED` 的步骤要求存在 `denied` 字段，于是拒收这条 RAW，步骤记不下来，run 被中止。修复：`denied` 总是写出（`track.go`）。同类检查只有这一处（`denialObserved`）；旧 RAW 文件的字节和 hash 不变，外部 runner 写的 RAW 照旧解析。

验收结果：

- `go -C tools/arc-verify test -race -count=1 ./...`：PASS；`go vet`：PASS。
- red：`.arc/artifacts/verify-execution-contract/red/red-001.json`（"accepted despite failed condition" 得到 BLOCKED，断言失败）。
- 种下缺陷：恢复 `omitempty`、去掉自引用 Oracle 检查都被抓到；轮次过期有两道独立检查（`validateRound` 与 binding 的 `source_sha256` 比对），单去一道仍拦住，两道都去才失败，属纵深防御。
- 真实入口：新测试都经 CLI `runner`/`round`/`execute` 对本机 HTTP 目标执行。

未验证：真实业务目标（IM-mongo `.60`）上的执行与恢复归 W06；浏览器/设备端 runner 不在本卡。

适配器清单补充（W22 选型）：可选外部命令适配器 Schemathesis 4.32.7（MIT，进程边界，只在目标有 OpenAPI 时用，HAR 逐条转 StepRAW），见 [选型记录](../../10-复盘决策/01-选型-API状态化探索.md)；接入另起卡，未安装。

## 上下文检索

<Context-Queries>

```bash
arc-idx files 'tools/arc-verify' --format json
arc-idx search 'f:tools/arc-verify/ (startBinding|step|finish|retry)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 从合同写可达入口的失败断言，保存 red，之后才改行为。
- [x] 实施本端能力、所有相关调用点/fixture/配置并保持输入验证与错误可见。
- [x] 验证：动作前真实前提；HTTP/CLI 独立 Oracle；属性反例/拒绝/超时保留证据；恢复失败不释放 fixture 或覆盖原 FAIL；全部参与账号/轮次绑定。
- [x] 回写实际文件、命令/结果、未验范围，并同步 tracker。

```bash
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行；前端命令在新增工作台工程执行。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
