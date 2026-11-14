# Skills 项目事实

- 主技能源码在 `Arc/`；共享协议在 `Arc/_shared/`。六个 Arc 技能由 `scripts/sync_skills.py` 与 `scripts/link_hosts.py` 安装到各宿主。
- 唯一公共命令是 `arc`（`tools/arc/`），它把检索转给 `tools/arc-idx/`、把业务图转给 `tools/arc-verify/`；三者都是短生命周期 Go CLI，用官方 `tursogo` 进程内驱动，由 `bash scripts/install_arc.sh` 一起安装。技能文档检索写 `arc idx <子命令>`、业务图写 `arc verify <子命令>`，不写裸 `arc search` 等旧形式。Python 脚本负责既有测试与校验工具。
- 索引 profile：`skills`、`docs`、`code`、`meta`。
- 技能注册表从源码生成：`.venv/bin/python scripts/build_skills_index.py`；不要手改 `skills.index.json`。

## 2026-10-30：verify 路线裁决

- 按完整《自动化测试规范设计》对话及 [PR #1](https://github.com/iwen-conf/Skills/pull/1) 强化现有技能。
- 先列应有业务并与用户核对；实现清单独立发现。用户确认的 RULE → 业务图 → 有界任务卡 → 真实执行 → 独立 RAW → 判定与历史 FAIL。
- AI 推断与行业惯例只算 RISK。覆盖条件满足、不满足、强制操作及已知恢复路径；保留首次偏离，按验证后的实际状态继续新任务。
- 图和历史保存在被测项目本地：显式 `arc db migrate` 激活后为 `.arc/arc.db`，否则为旧账本 `.arc/verify/data.db`；JSONC 保存环境、账号别名与凭证引用。测试服务器是被测目标。
- 当前图门禁只覆盖已规划的正向、逐条件负向和强制义务；恢复、跨角色、并发、真实 E2E 与独立可信 Runner 另需执行证据。
- 实施与未完成边界记录在 [任务文档](docs/03-执行任务/arc-verify-graph-runtime.md)。

## 2026-10-31：verify 演化路线核对裁决

- 已逐轮对照对话导出（352 条消息，SHA-256 `458e8ecc810028a59f5dd08b3e6b5383c48a6e38efe41691f9b09747065618ac`）和多视图附件；偏差、排序和顺序以 tracker「2026-10-31 演化路线核对」为准。
- 工作台改为**静态快照**：CLI 一次性导出 HTML+JSON，用浏览器只读打开；不起本地服务，页面里不执行、不批准、不重试，写操作只走 CLI。原 `workbench` 前台服务方案作废。
- 首个真实试点是 IM-mongo（源码 `~/Documents/Code/chensi/IM-mongo`；BG16 三条链，再扩到 39 族）；`academic-v1` 只作工具自测示例。
- 本仓不启用 GitHub Actions（`tasks/lessons.md`）；不拿受保护 CI 当可信边界，独立可信 Runner 仍未做。

## 2026-11-04：verify 首次入口裁决

- 完整系统、全部业务、所有交互测试和 DB 业务树请求直接走 verify graph；无现有图时由当前 Agent 独立发现应有业务与实现入口，生成候选 JSON、导入本地 DB 并导出静态快照供核对，不要求用户先写图。
- 候选能力树可先于 RULE、环境、账号和 Runner 建立，始终保持未验证。复用已有确认；补齐未决规则后继续已授权测试，不止于计划。新模型版本保留旧图、RAW 与首次 FAIL。
- 普通局部单测、SQL lint 不强制建图；只计划、只读、只建图的范围优先。缺少真实环境、授权、独立 Oracle 或 Runner 时，如实 BLOCKED/NOT_RUN，候选图和工具回归不能代替真实业务 E2E。

## 2026-11-04：verify 业务图绘制裁决

- verify 的业务图、业务树和流程图统一用 [arch-diagram 插件](https://github.com/bassnova42/arch-diagram-skill) 绘制，业务用中文表述，节点命名为 `中文业务名【实际英文业务包名】`。包名以被测项目目录、包声明和配置为依据，不自行翻译或编造；未定位实现时标明业务包待核实。
- 插件只负责只读可视化；本地 DB、CLI 快照、流程台账和真实 RAW 继续保留。具体绘图约定见 [business-diagram](Arc/arc:verify/references/business-diagram.md)。

## 2026-11-07：E2E 技能强化裁决

- 用户指定 [tester-army/e2e](https://github.com/tester-army/e2e) 强化现有 `arc:verify`；按需执行指引见 [tester-army-e2e](Arc/arc:verify/references/tester-army-e2e.md)，源码核对固定到 `232ce75ab4b68ded93525162e1121bf973792ea8`。
- 采用逐目标操作、确定性断言、失败现场诊断和 Web/移动端执行接口；业务判定继续绑定已确认 RULE、独立回读及任务/轮次/环境/账号。每次执行单独保存报告，缓存与自动修复不覆盖首次 FAIL；退出码 0、跳过或过滤不充当业务 PASS。
- SDK 模型配置只在被测项目执行器中按已授权配置使用；远程设备兼容、真实产品 E2E 和独立可信 Runner 仍需各自的真实执行证据。

## 本仓验收命令

```bash
go -C tools/arc test -count=1 ./...
go -C tools/arc-idx test -count=1 ./...
go -C tools/arc-verify test -race -count=1 ./...
go -C tools/arc-verify vet ./...
bin="$(mktemp -d)" && bash scripts/install_arc.sh "$bin" && bash tools/arc/smoke.sh "$bin" "$PWD"
.venv/bin/python -m pytest -q
.venv/bin/python -m pytest -q Arc/arc:verify/scripts/dbqual/tests
.venv/bin/python scripts/validate_skills.py
```
