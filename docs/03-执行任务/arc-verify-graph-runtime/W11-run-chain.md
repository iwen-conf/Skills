# W11 确定性连续执行（异常才交给 AI）

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P2，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D3 |
| 难度依据 | 要在不降低逐步证据强度的前提下串起多张卡：每步仍写 RAW、独立判定、持锁与恢复；停机条件、步数和总时限必须有上限，超时要终止子进程并留证据。 |
| 目标面 | 本仓工作树、本地 Go 与原生 Turso（`tursogo`）；不碰远程业务数据；真实执行只对已授权隔离目标 |
| 前置依赖 | [W03](W03-execution.md)：单卡 `execute` 已验收 |
| 下游影响 | W08 派生、W09 探索、W06 联合验收 |

## 目标与范围

需求：[BG22](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「补齐的义务、报告与探索」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

范围：`tools/arc-verify/execute.go`（`execute`）、`runner.go`、`main.go`（新增 `run-chain` 子命令），新增 `chain.go`、`chain_test.go`。

- `run-chain --graph --env --round --max-steps --deadline`：沿已知路径逐卡调用同一 `execute`，运行完退出。
- 遇到以下任一情况就停：未知状态、前提不满足、偏离、新分支、预算耗尽。停下时输出最小上下文包（当前状态、偏离步、候选动作、证据引用），并以非零退出码区分停机原因。

不做：不常驻、不起后台进程；不跳过逐步 RAW 与独立判定；停机后不自动重试；不替 AI 选择偏离后的分支。

## 2026-10-31 实施记录

命令：`arc-verify run-chain --graph G --env E --round Q --root R --config C --commit SHA --account A --fixture F [--accounts JSON] --max-steps N --deadline D`（N 为 1–1024，D 为 100ms–24h，越界在执行前拒绝）。

- 已知路径 = 图里已规划、状态 READY、并恰好有一个当前有效 runner 的 MODEL 任务，按测试依赖排序；其余 READY 任务列在 `unautomated`（缺 runner、runner 过期或有多个）。一张都跑不了时以 `blocked` 停下，不报完成。
- 逐卡调用同一个 `execute`，每卡照旧写 setup/action/fixture RAW、独立判定、持锁与恢复；总截止时间作为 `execute` 的上下文，进行中的业务动作会被取消，恢复照旧执行。
- 停机原因与退出码：`divergence` 10、`unknown_state` 11（观察到模型里没有的状态）、`precondition` 12（前提不满足或依赖没在本轮通过）、`new_branch` 13（安全地观察到已知但非预期的状态）、`budget` 14（步数或截止时间）、`blocked` 15；完成为 0。上下文包只有 `graph round stop reason steps task run state divergence candidates remaining unautomated`：`state` 取最后一次独立观察的状态（动作没执行时取动作前读数），`candidates` 是该状态出发的已声明转移，`remaining` 是因停机没执行的卡，偏离步只带期望、观察、RAW 路径和 hash，不复制 RAW 内容。
- 新文件 `chain.go`、`chain_test.go`；`main.go` 增加 `run-chain`、`--deadline` 和按停机原因退出的 `exitCode`。

修复两个真实缺陷：
1. 前提不满足、动作没执行时，这一步的 RAW 记不下来（`preconditions must be an object of explicit booleans`），运行被笼统记成 BLOCKED、原因失真、证据丢失。根因：Oracle 事实的 `Conditions` 复用了模型用的 `ConditionAssignment`，它拒收 `null`，而没被读取的"动作后"事实序列化正是 `null`。修复：新类型 `ObservedConditions`（`runtime_model.go`），`null` 表示没读取，出现的对象仍要求每个条件是显式布尔；`readOracle` 跟着改（`runner.go`）。复现测试 `TestExecutePreconditionMismatchRecordsRAW`。
2. `run-chain` 在一张卡都没跑时报"已知路径全部通过"。改为 `blocked` 并写明原因（`TestRunChainWithoutRunnableCardsIsBlocked`）。

偏差：卡片要求的 `TestRunChainDeadlineKillsProcessGroup` 落为 `TestRunChainDeadlineStopsActionAndKeepsRAW`（HTTP 动作被截止时间取消、setup 与恢复 RAW 都在）；命令适配器的进程组终止由既有 `runner_process_unix.go` 负责，本卡没有新增命令进程的截止测试。

验收结果：

- `go -C tools/arc-verify test -race -count=1 ./...`：PASS；`go vet`：PASS。
- red：`.arc/artifacts/verify-run-chain/red/red-001.json`（run-chain）、`red-002.json`（丢失前提 RAW）、`red-003.json`（空路径报完成）。
- 门禁：`gate.sh check --strict --require-red --scope` 0 FAIL（`reports/gate-20261031T103728Z.txt`）；WARN 两条：`chain.go` 红灯前 54 行是类型与退出码常量；红灯后唯一改掉的断言是"偏离时候选转移非空"——那次偏离观察到的是终态 `ready`，本来就没有后续转移，期望写错了，改成断言未执行的卡与观察状态，候选转移改在前提用例里断言为 `submit-flow`。
- 种下缺陷 12 个全部抓到；"步数预算"和"退出码映射"首轮漏网，补了 `TestRunChainStepBudgetStopsAndNamesRemainingCards`、`TestRunChainExitCodes`。
- 真实进程：`TestRunChainBuiltBinaryExitStatus` 用编译好的二进制执行 runner 注册、round、plan、run-chain，退出状态 10 并输出上下文包（`.arc/artifacts/verify-run-chain/real-entry/cli-exit.log`）；只有导入和人工批准在测试进程里做（代理不能代替人批准）。未设 `ARC_VERIFY_CHAIN_BINARY` 时该用例显式 SKIP。
- 观察：注册 runner 会绑定注册时的可执行文件；换二进制后旧 runner 过期，`round` 会拒绝开轮次——这是既有的严格设计，本卡未改。

未验证：IM-mongo 真实目标上的连续执行归 W06。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (ExecuteRequest|RunnerManifest|restoreFixture)' --format ai-md --context 2 --max 30
arc-idx search 'f:tools/arc-verify/ -f:_test (FirstDivergence|first_divergence|REPLAN)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 先写失败测试：`TestRunChainStopsOnDivergence`、`TestRunChainStopsOnUnknownState`、`TestRunChainWritesRAWPerStep`、`TestRunChainDeadlineKillsProcessGroup`、`TestRunChainContextPacketMinimal`，保存 red。
- [x] 实现 `run-chain`，复用 `execute`。
- [x] 跑验收命令；回写并同步 tracker。

## 验收

```bash
go -C tools/arc-verify test -race -count=1 -run 'RunChain' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行（不是 Rust，允许本机编译）。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
