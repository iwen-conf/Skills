# W13 静态快照导出（取代 workbench 服务）

状态：`[x]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 后端 |
| 优先级 | P1，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D3 |
| 难度依据 | 要在移除已有 HTTP 服务（`workbench.go` 及其测试）的同时，保住原 DTO 合同和证据重验语义；导出涉及六视图、分析、trace、证据四类数据，还要满足不覆盖、超限失败和 `file://` 零请求。 |
| 目标面 | 本仓工作树、本地 Go 与原生 Turso（`tursogo`）；不碰远程业务数据；产物默认写 `.arc/verify/snapshots/`（已被 `.gitignore` 的 `.arc/` 规则忽略） |
| 前置依赖 | [W02](W02-analysis.md)：视图投影与分析 DTO 稳定；[W03](W03-execution.md)：trace 与 StepRAW DTO 稳定 |
| 下游影响 | W05 前端、W06 联合验收、W14 文档 |

## 目标与范围

需求：[BG25](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「静态快照」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

范围：`tools/arc-verify/workbench.go`（删除 `newWorkbenchHandler`、`serveWorkbench`、会话令牌与 `/api/*`）、`workbench_dto.go`（保留为快照 DTO）、`workbench_evidence.go`（证据重验复用）、`main.go`（删 `workbench` 分支，加 `snapshot`）、`workbench_test.go`、`workbench_lifecycle_test.go`（服务测试改写或删除），新增 `snapshot.go`、`snapshot_test.go`。

- `snapshot --graph [--environment] [--round] [--run]... --assets --out`：只取账本里实际存在的上下文，不存在就报错。
- 写出 `snapshot.json`，并把 `--assets` 复制过来，生成内嵌 JSON 的 `index.html`。
- 逐条重验证据，校验失败的写成失败条目；`--out` 非空即拒绝；超过大小上限就失败并报出超限部分。
- 一项 Go 测试比对 `workbench/src/types.ts` 与 DTO 的 JSON 字段。

不做：不保留 HTTP 服务作兜底；页面不加任何写操作；不把凭证或任意本地文件写进快照；不改前端（属 W05）。

## 2026-10-31 实施记录

开工核查：`workbench*.go` 最后修改 09:46–09:54，`workbench/src/api.ts`、`layout.ts` 12:45；`ListAgents` 只有 IM-mongo 仓的一个空闲会话，本仓没有在跑的 Claude 会话（Codex/Grok 看不到）。按用户 2026-10-31 裁决（静态快照）和"允许你直接完成剩余任务"继续；改动前把 `workbench*.go`、`workbench/`（不含 node_modules、dist）和 `main.go` 原样打包到 `.arc/artifacts/verify-snapshot/pre-snapshot-backup/`，因为这些都是未跟踪文件，没有 git 历史。

实际改动：

- 新增 `snapshot.go`：`writeSnapshot`（先校验、后暂存目录、最后改名，拒绝的导出不留产物）、`buildSnapshot`（只取账本里已有的图/环境/轮次/运行；v1 拒绝）、`snapshotRun`（setup、每个 action、fixture 证据逐条重验，失败写成 `error` 条目）、`snapshotPage`（CSP 与内嵌 JSON 紧跟 `<head>`）、`copySnapshotAssets`（只拷普通文件，含 `/api/` 的构建直接拒绝）、超限报各部分大小；`workbenchCatalog` 从 `workbench.go` 移来并按图过滤。
- 删除 `workbench.go`（HTTP 服务、会话令牌、`/api/*`）、`workbench_test.go`、`workbench_lifecycle_test.go`；`workbench_dto.go` 只留目录条目类型。服务测试里仍有意义的语义移进快照测试：环境来自配置、六视图来自真实账本、篡改的恢复证据不能显示为有效。
- `main.go`：`workbench` 分支、`--address`、`--lifetime` 删除；新增 `snapshot` 与 `--out`；命令名在打开账本前校验，未知命令直接报 `unknown command`（命令表与 switch 分支一致，脚本核对过）。
- `workbench/src/types.ts` 按 Go DTO 重写（合同镜像，含 `Snapshot*`、`StepRaw`、`ProofContext`）；`App.tsx`/`api.ts` 仍按旧接口写，W05 改。
- `README.md` 增加"Static snapshot"一节。

与卡片/设计的偏差：`--run` 用逗号分隔（`--run A,B`），沿用既有字符串参数，不引入可重复参数；示例快照不是从 `academic-v1` 导出——它是 v1 模型，多视图分析只支持 v2——改由 `TestSnapshotSampleForWorkbench` 经 CLI 从 v2 course 参考模型导出（含一次 FAIL、一次重试 PASS，验证视图出现 FLAKY 与 BLOCKED），设 `ARC_VERIFY_SAMPLE_OUT` 时写到 `tools/arc-verify/workbench/dev/sample-snapshot.json`（已生成，副本在 `.arc/artifacts/verify-snapshot/real-entry/`）。

验收结果：

- `go -C tools/arc-verify test -race -count=1 ./...`：PASS（210 项，含新测试）；`go vet`：PASS。
- red：`.arc/artifacts/verify-snapshot/red/red-001.json`（断言失败）。
- `go -C tools/arc-verify run . workbench`：退出码 1，`unknown command "workbench"`（`.arc/artifacts/verify-snapshot/real-entry/workbench-removed.log`）。
- 种下缺陷 13 个：丢弃失败证据、覆盖旧快照、去掉 CSP、放行 `/api/` 构建、去掉大小上限、猜环境、接受别的图的 run、去掉命令名校验、内嵌 JSON 与文件不一致都被抓到；"覆盖旧快照"首轮因 `os.Remove` 非空目录也会失败而漏网，把断言收紧到显式检查的报错原文后抓到；"放行 v1"是等价变异（下游分析同样拒绝 v1）。

门禁复跑（`.arc/artifacts/verify-snapshot/reports/gate-20261031T102524Z.txt`，W05 完成后）：Go 部分、red-first PASS；`lint(ts)`/`tests(js)` FAIL 原因同 W05（门禁用 npm，被 bun 工程拒绝），工程自己的 `vpr build`/`vp test`/`vp check` 已通过；scope FAIL 列出的都是 W05 的文件（W13 基线早于 W05），这些文件在 W05 的门禁里 scope PASS。

未验证：Vite 默认产出 `type="module"` 脚本，Chrome 在 `file://` 下会因 CORS 拒绝加载，W05 要改构建产物格式。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (serveWorkbench|newWorkbenchHandler|workbenchCatalog|viewModel)' --format ai-md --context 2 --max 30
arc-idx search 'f:tools/arc-verify/ -f:_test (ExecuteRequest|RunnerManifest|restoreFixture)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 开工前看 `git status` 和 `workbench*.go` 修改时间；有其他会话的在途改动就先报告。
- [x] 先写失败测试：`TestSnapshotUnknownContextErrors`、`TestSnapshotRefusesNonEmptyOut`、`TestSnapshotTamperedEvidenceWrittenAsFailure`、`TestSnapshotSizeLimitFails`、`TestSnapshotHasNoAPIReferences`、`TestSnapshotTypesMatchFrontend`、`TestWorkbenchCommandRemoved`，保存 red。
- [x] 实现 `snapshot`；移除服务代码与对应测试；更新 `tools/arc-verify/README.md` 命令说明。
- [x] 从 `academic-v1` 导出一份样例快照给 W05 用。
- [x] 跑验收命令；回写并同步 tracker。

## 验收

```bash
go -C tools/arc-verify test -race -count=1 -run 'Snapshot|WorkbenchCommandRemoved' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
go -C tools/arc-verify run . workbench; test $? -ne 0   # 期望：unknown command
```

Go 命令在本仓执行（不是 Rust，允许本机编译）。工具内核 PASS 不代替真实业务/客户端 PASS；只有本卡全部适用验收有证据才 `[x]`。
