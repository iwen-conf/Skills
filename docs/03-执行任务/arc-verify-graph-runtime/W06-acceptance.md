# W06 完整验收

状态：`[!]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 集成 |
| 优先级 | P1，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D3 |
| 难度依据 | 工具、完整技能安装、真实目标/客户端与环境边界分别证明，不能把其中任意一个 PASS 推广。 |
| 目标面 | 本仓当前工作树、本地 Go/Turso；Rust 仅远程 .60；真实业务用已授权隔离目标 |
| 前置依赖 | W01–W03、W05、W07–W14（W04 已作废）；IM-mongo 当前有效隔离账号/fixture/真实端与截止证据 |
| 下游影响 | 工作台、真实 runner、六技能交接与发布证据 |

## 目标与范围

需求为 [BG01–BG25](../../00-产品需求/06-业务模型驱动验证.md)。范围：测试、fixture、配置、Arc/_shared/workflow.md、Arc/arc:verify/ 等现有六技能文档、项目事实与验收产物；业务缺陷转 arc-code。新增文件属于既有工具模块，文件名在实施时回写。

共享设计与接口：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)。唯一状态源：[tracker](../arc-verify-graph-runtime.md)。不覆盖其他会话，不修改未授权远程业务数据，不用静态演示或历史 PASS 代替运行。

## 2026-10-31 进度

**工具层验收：已完成**（`.arc/artifacts/verify-joint-acceptance/tool-acceptance.log`、`policy-verify.txt`）：

| 项 | 结果 |
| --- | --- |
| `go -C tools/arc-verify test -race -count=1 ./...` | PASS |
| `go vet` / `go build` | PASS / PASS |
| `arc-verify selftest`（临时本地 Turso，无网络） | PASS |
| `.venv/bin/python -m pytest -q` | 476 passed、1 skipped（protobuf 解释器，既有） |
| DBqual `pytest` | 160 passed、1 skipped |
| `scripts/validate_skills.py` | 通过（65 个技能文件） |
| 静态工作台 `vp check` / `vp test` / `vpr build` | 通过 / 17 项通过 / 2/2 成功 |
| 真实浏览器 `file://` 打开快照（W05） | 通过，零外部请求 |
| `arc-policy task verify` | PASS（architecture、tests、snapshot、cli_receipt） |

**真实业务验收：阻塞**，原因与需要的输入：

1. IM-mongo 还没有 v2 业务模型。BG16 三张卡（小店首讯/排队/领取、主动联系与审批介入、群发/recall/离线链）的 RULE 必须来自用户确认或文档；代理整理的只能先当 RISK。需要：用户确认从 `docs/02-API记录/` 与 `docs/00-产品需求/` 起草的规则，并在真实终端 `approve`。
2. 执行环境：`.60` 隔离环境的账号与 fixture 清单、执行与恢复授权（项目规则：共享数据写入需具体任务卡授权，不默认部署或重启）。
3. runner 清单：每张卡的入口与独立 Oracle（IM-mongo 没有 OpenAPI，W22 结论是走模型驱动的 HTTP 适配器）。
4. 39 族逐族登记适用、已测、未测，依赖前三项。

恢复条件：用户提供第 1、2 项（至少一张卡的确认规则和授权）后，按 W03/W11 在 `.60` 跑并留 RAW。工具层 PASS 不代表真实业务 PASS。

## 2026-11-01：iso28886 已授权写入（不是共享 28885）

用户批准把夹具改到 `iso28886` 并在该面写数据、做业务级故障。ARC Verify 已在 `192.168.1.50:28886` 跑通进线已知规则与探索，见 [W35](W35-cross-project-e2e.md)。共享 `lan50/28885` 仍 `approved: false`，只读 live=200。本卡图谱 TTY 批准与 39 族登记仍 `[!]`。

## 2026-10-31 晚：隔离替补（不是 IM-mongo）

W25 在仓库内已授权 httptest 目标 `seat-v2` 上完成一条业务链的正常/异常/多角色/并发/恢复，证据在 `.arc/artifacts/verify-isolated-seat/`。IM-mongo `lan50`/`tx` 仍为 `approved: false`；对根路径探测得到 HTTP 404，没有执行或注入授权。本卡保持 `[!]`，不得把 seat-v2 PASS 记成 IM-mongo PASS。

## 上下文检索

<Context-Queries>

```bash
arc-idx files 'tools/arc-verify' --format json
arc-idx search 'f:tools/arc-verify/ (TaskCard|StepRAW|HandoffPacket)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [ ] 从合同写可达入口的失败断言，保存 red，之后才改行为。
- [ ] 实施本端能力、所有相关调用点/fixture/配置并保持输入验证与错误可见。
- [/] 验证：原生 Go/Turso race/vet/build/selftest、根 Python/DBqual、技能结构/分发/安装读回与 arc-policy verify；UI 真联调；IM 三卡/39族每项绑定真实合同和适用范围，缺输入单列 BLOCKED/NOT_RUN。
- [ ] 回写实际文件、命令/结果、未验范围，并同步 tracker。

```bash
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
```

Go 命令在本仓执行；前端命令在新增工作台工程执行。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
