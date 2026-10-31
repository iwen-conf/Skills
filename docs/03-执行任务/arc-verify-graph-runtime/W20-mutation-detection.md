# W20 注入缺陷检出率

状态：`[!]`

| 属性 | 内容 |
| --- | --- |
| 所属端 | 验证（执行技能 arc-verify） |
| 优先级 | P1，顺序见 tracker「2026-10-31 演化路线核对与剩余工作排序」 |
| 难度 | D3 |
| 难度依据 | 要在一次性副本里改被测系统、部署到隔离环境、跑全套义务再比对，涉及多个缺陷类别和外部环境；不能用工具自测代替。 |
| 目标面 | IM-mongo 一次性 worktree + `.60` 已授权隔离环境；`academic-v1` 只作 CLI 自测 |
| 前置依赖 | [W03](W03-execution.md) 真实执行已验收；IM-mongo 隔离环境中部署注入版本的授权（tracker 记载 IM 共享数据写入需具体任务卡授权、不默认部署，缺授权时本卡标 `[!]`） |
| 下游影响 | W06 联合验收、风险报告里的检出率说明 |

## 目标与范围

需求：[BG29](../../00-产品需求/06-业务模型驱动验证.md)；设计：[05 工作台](../../01-系统设计/05-业务图谱工作台.md)「来源分离、反证与自测」；外部依据见 PRD「外部研究核对与借鉴」。唯一状态源：[tracker](../arc-verify-graph-runtime.md)；前置约束：[00](00-前置约束.md)。

范围：
- CLI 端：`tools/arc-verify/main.go` 新增 `mutation-report`，新增 `mutation.go`、`mutation_test.go`。输入注入清单（缺陷 ID、类别、位置、预期应失败的义务 ID）和运行结果，输出检出/漏检与原因。
- 验证端：在 IM-mongo 一次性 worktree 注入至少 5 类缺陷（越权、竞态/超卖、非法状态转移、幂等破坏、缺服务端校验），每类至少 1 个，部署到隔离环境，跑相关义务，再用 `mutation-report` 出报告。

不做：不在工作树或共享环境改被测源码；不把检出率设成通用门禁阈值；注入版本跑完必须回收（按 W03 恢复协议）；没有授权不部署。

## 2026-10-31 进度

**阻塞：没有部署授权。** 按项目规则（tracker "IM-mongo 共享数据写入需具体任务卡授权，不默认部署或重启"），"允许你直接完成剩余任务"不等于"把注入缺陷的版本部署到 `.60`"的具体授权，所以真实注入、部署、执行、回收都没做，检出率结论为空。恢复条件：用户点名授权本卡在 IM-mongo 一次性 worktree 注入并部署到 `.60` 隔离环境（含回收方式）；另外 IM-mongo 还没有 v2 模型和 runner 清单（归 W06），要先有它们才能跑义务。

CLI 部分已完成：`arc-verify mutation-report --file mutations.json`。

- 输入：`graph` 与 `mutations[]`（`id`、`category` 取 `unauthorized_access`/`race_oversell`/`illegal_transition`/`idempotency`/`missing_server_validation`/`other`、`location`、`expected_failing`（义务定义 ID，必须存在）、`runs`（对注入版本执行的运行，必须属于该图））。
- 判定只看独立运行结论：预期义务有证据完好的 FAIL → `detected`；预期义务全部合格 PASS → `survived`（列入 `survivors`，要设计新义务）；没运行或结论不合格 → `inconclusive`。别的义务失败不算检出。按类别分组；检出加漏检少于 10 标 `insufficient_sample`；不设阈值。
- 新文件 `mutation.go`、`mutation_test.go`；`main.go` 增加命令。验收：`go -C tools/arc-verify test -race -count=1 ./...` PASS；red `.arc/artifacts/verify-mutation-report/red/red-001.json`；种下缺陷 7 个全部抓到（"别的义务失败也算检出"首轮漏网，补了用例）。

## 上下文检索

<Context-Queries>

```bash
arc-idx search 'f:tools/arc-verify/ -f:_test (ExecuteRequest|RunnerManifest|restoreFixture)' --format ai-md --context 2 --max 30
arc-idx search 'f:Arc/arc:verify/references/ (mutation|seeded|sensitivity)' --format ai-md --context 2 --max 30
```

</Context-Queries>

开工先重跑查询和读取命中，确认当前源码/依赖与上游证据，范围变化先回写本卡和 tracker。

## 执行清单

- [x] 先写 CLI 失败测试：`TestMutationReportDetectsExpectedFailure`、`TestMutationReportListsSurvivors`、`TestMutationReportRejectsUnknownObligation`，保存 red，再实现 `mutation-report`。
- [!] 取得隔离环境部署授权；没有就标 `[!]`，只完成 CLI 部分（`[?]`）。——2026-10-31 未取得具体授权
- [ ] 注入、部署、执行、回收；每步留 RAW。
- [ ] 出报告到 `.arc/artifacts/verify-mutation/reports/`；漏检的缺陷生成新义务候选交 `arc-verify` 设计。
- [ ] 回写并同步 tracker。

## 验收

```bash
go -C tools/arc-verify test -race -count=1 -run 'MutationReport' ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
test -s .arc/artifacts/verify-mutation/reports/detection.md   # 真实部分
```

CLI 测试通过只证明报告工具；检出率结论只能来自真实注入运行。
