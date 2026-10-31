# 选型：API 状态化探索适配器

任务：[W22](../03-执行任务/arc-verify-graph-runtime/W22-schemathesis-vet.md)；需求 BG31（PRD 06）。原始证据：`.arc/artifacts/verify-schemathesis-vet/reports/`（`github_health.txt`、`manual-releases-docs.txt`、`manual-schemathesis.txt`、`manual-capabilities.txt`、`advisories.txt`）。

## 需求与约束

- 功能边界与调用方：W09 的 DISCOVERY 要像用户一样操作被测 API，发现模型以外的行为（记为 UNSPECIFIED），每个请求都要有可复核的 RAW。调用方是 `tools/arc-verify` 的 runner：动作走 `AdapterCall`（`http` 或 `command` argv），结果只认独立 Oracle 读回。
- 语言 / 版本 / 运行平台：arc-verify 是短生命周期 Go CLI（`go 1.24.0`），不常驻、不起后台进程；本机 macOS，真实目标在已授权隔离环境。
- 关键约束：不改 `go.mod`；外部工具只能作为进程边界的命令适配器；不得把被测 API 的数据发给第三方服务；结论不等于已接入。本仓没有声明许可证（`gh repo view` 的 `licenseInfo` 为 null），只做进程调用、不拷贝代码，许可证兼容问题不成立。
- 检查日期：2026-10-31；GitHub 可访问（`gh` 已认证）。
- 检索关键词族与来源：GitHub 搜索 `stateful REST API fuzzing`、`openapi fuzzer`、`rest api fuzzer`、`OpenAPI property-based testing`、`schemathesis`、`evomaster`、`AutoRestTest`；PyPI JSON API；OSV；各仓库 `docs/` 与 README。
- 本地复用检查：`arc-idx search 'f:tools/arc-verify/ -f:_test (schemathesis|restler|openapi|swagger)'` 无命中；runner 只执行声明的调用（`runner.go` 的 `validateManifest`/`performCall`），没有探索能力；标准库不覆盖。
- 首个试点 IM-mongo（提交 `f644f637`）没有 OpenAPI 文档：`arc-idx files 'swagger|openapi'` 与内容检索均无命中，API 合同只有 `docs/02-API记录/*.md`。下面所有候选都以 OpenAPI 为输入。

## 候选比较

| 项目 | 功能适配 | 最近实质维护 | 最近稳定版 | 许可证 | 安全 / 依赖 / 集成风险 | 结论 |
|---|---|---|---|---|---|---|
| [schemathesis/schemathesis](https://github.com/schemathesis/schemathesis) | OpenAPI 属性测试；`--phases stateful` 按 OpenAPI links 串操作；`--report har,vcr,ndjson` 逐请求记录请求与响应；自定义检查与 CLI 扩展走 Python 钩子（`SCHEMATHESIS_HOOKS`） | 2026-10-31（非机器人提交 `4455a4bb`） | v4.32.7，2026-10-27 | MIT（仓库与 PyPI 一致） | OSV 与仓库安全公告均为 0；需要 Python ≥3.11；以命令适配器调用即可 | 条件采用（可选外部适配器） |
| [Endava/cats](https://github.com/Endava/cats) | 160+ 负向与边界 fuzzer，含 IDOR、PUT 幂等、更新后读一致等状态化检查；报告逐条含请求、响应和重放命令，默认脱敏认证头 | 2026-10-30（`38b5f8af`） | cats-15.3.0，2026-10-30 | Apache-2.0 | 公告未核实（Maven 坐标未查）；JVM 或原生二进制 | 备选（负向测试补充） |
| [microsoft/restler-fuzzer](https://github.com/microsoft/restler-fuzzer) | 生产者-消费者依赖推断的状态化序列，内置 checker（UseAfterFree、InvalidDynamicObject 等） | 2026-02-13（`6d984dee`） | v9.3.1，2025-01-30（发布已 21 个月，`维护关注`） | MIT | .NET 加 Python 两套运行时；本次未取到逐请求日志格式的文档 | 借鉴设计 |
| [WebFuzzing/EvoMaster](https://github.com/WebFuzzing/EvoMaster) | 进化搜索生成测试；6.0 起默认黑盒 | 2026-10-30（`39ce5dda`） | v6.2.0，2026-09-15 | LGPL-3.0 | JVM；产出是生成的测试代码（`--outputFormat`），不是逐请求 RAW | 借鉴设计 |
| [TNO-S3/WuppieFuzz](https://github.com/TNO-S3/WuppieFuzz) | 基于 LibAFL 的覆盖率引导 REST fuzzer | 2026-10-26（`e3e6a2df`） | v1.8.0，2026-10-07 | Apache-2.0 | 要发挥作用需被测服务提供覆盖率；黑盒业务探索适配度低 | 淘汰（不适配） |
| [selab-gatech/autoresttest](https://github.com/selab-gatech/autoresttest) | 图建模加 LLM 加多智能体强化学习 | 2026-10-10（`60e33231`） | 无稳定版（只有 `rest-league-2027` 标签） | MIT | 运行时必须调用 LLM 提供方（README：需要 API key），默认会把规格与数据发给第三方 | 淘汰（研究原型、数据外发） |
| [matusf/openapi-fuzzer](https://github.com/matusf/openapi-fuzzer) | OpenAPI 黑盒 fuzzer | 2023-10-16（`5a7d5944`） | v0.2.0，2023-08-30 | AGPL-3.0 | 停更超过 36 个月 | 淘汰（否决） |

## 证据

- Schemathesis：`docs/reference/cli.md` 第 96–107 行（`--phases`，含 `stateful`）、第 510–597 行（`--report` 可选 `junit, vcr, har, ndjson, json, wfc, pytest, allure, html`，以及各自的路径参数）；`docs/guides/stateful-testing.md`（按 links 串操作，`schema.as_state_machine()`）；`docs/guides/extending-cli.md`（`SCHEMATHESIS_HOOKS` 加载扩展）。PyPI `schemathesis` 4.32.7，许可证 MIT，`requires_python >=3.11`；OSV 查询 `PyPI/schemathesis` 无公告；仓库安全公告数 0。
- CATS：README 第 37–40 行（fuzzer 种类与状态化检查、标识复用、生产者优先）、第 88 行（逐条查看请求、响应、重放命令）、第 176 行（报告默认脱敏认证信息）。
- RESTler：标签 `v9.3.1` 的打标时间 2025-01-30；`docs/user-guide/Checkers.md`（checker 机制与开关）。
- EvoMaster：`docs/blackbox.md` 第 25–51 行（黑盒命令、`--outputFormat`、`--ratePerMinute` 防止压垮目标）。
- AutoRestTest：README 第 20–25 行（使用 LLM，需要提供方 API key）。
- 维护信号统一来自 `scripts/github_health.py`（非机器人提交与稳定发布）；未做逐个核心路径 diff 的人工审阅，记为限制。

## 否决与例外

- openapi-fuzzer：停更 ≥36 个月 → 否决；另为 AGPL-3.0。
- AutoRestTest：无稳定版，且运行依赖第三方 LLM，与"不得把被测 API 数据发给第三方"冲突 → 不作依赖；图建模思路可借鉴。
- WuppieFuzz：覆盖率引导需要被测服务插桩，和黑盒业务探索不匹配 → 淘汰。
- RESTler：发布 21 个月未更新（`维护关注`），两套运行时成本高 → 不作依赖，借鉴生产者-消费者推断。
- EvoMaster：产出是测试代码而不是逐请求 RAW，接入 RAW 证据链成本高 → 借鉴设计。
- 未核实：CATS 的 Maven/二进制发布包安全公告；各候选核心路径 diff 的人工审阅。复核命令：`curl -s https://api.osv.dev/v1/query -d '{"package":{"name":"<坐标>","ecosystem":"Maven"}}'`；`gh api repos/<owner>/<repo>/commits?path=<核心目录>`。

## 决定

- 方案层级：**借鉴设计，模型驱动自建 DISCOVERY**（W09）；**Schemathesis 作为可选外部命令适配器**，只在目标提供 OpenAPI 时启用，不作为 Go 依赖。
- 理由：DISCOVERY 要判断的是"业务模型没声明的行为"，需要我们的业务状态、条件与独立 Oracle。现有工具判断的是"响应是否符合 OpenAPI 与通用检查"，不知道业务状态，也不会在每步读独立 Oracle。所以主路径用 runner 已有的 HTTP 适配器与 Oracle：在模型声明的状态里尝试已声明动作（含被禁止的上下文），以独立读回判断；被接受但未声明的结果记 UNSPECIFIED。借鉴的设计：Schemathesis 的 links 串操作与 HAR 式逐请求记录、已接受失败的基线文件；RESTler 的生产者-消费者推断；CATS 的 IDOR、幂等、更新后读一致等状态化检查清单与报告脱敏。选 Schemathesis 而不是 CATS 作为可选适配器：HAR、VCR、NDJSON 都是逐请求的标准格式，便于转成 RAW；自定义检查可写 Python 钩子；Python 工具链本仓已有（uv）。
- 版本锁定：`schemathesis==4.32.7`（PyPI，MIT），用 `uv tool install schemathesis==4.32.7` 装在隔离工具环境；接入前不安装。
- 最小验证（接入卡执行）：对本机合成 HTTP 夹具和一份小 OpenAPI 文档运行 `st run spec.yaml --url http://127.0.0.1:<port> --phases stateful --report har --report-har-path <raw>/run.har`；把 HAR 每条记录转换成逐步 StepRAW，核对条数、请求/响应 hash，并确认非零退出码不会被记成 PASS。
- 安全与回归：只对已授权隔离目标运行；Schemathesis 的发现只能成为 UNSPECIFIED 或 RISK 候选，不直接判业务 FAIL。
- 首个试点的前提：IM-mongo 目前没有 OpenAPI 文档。要用 Schemathesis，得先由用户确认从 `docs/02-API记录/` 生成或维护一份 OpenAPI；没有它就只走模型驱动的 DISCOVERY。
- 退出方案：适配器只是一个 `command` 类型的 runner 步骤，删除对应清单即可退出；模型驱动的 DISCOVERY 不依赖它。
