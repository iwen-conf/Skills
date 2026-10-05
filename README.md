# Skills

这个仓库包含两层 Skill：

- `Arc/arc:*` 负责跨项目的软件工程工作，共 5 个技能，每类工作对应一个。
- `Android/`、`HarmonyOS/`、`CNB/`、`Lark/` 保留各自平台的垂直能力与参考资料。

代码检索默认使用 `arc-idx`（编译型 Go CLI，内嵌 Zoekt，编排 ast-grep 与 Universal Ctags，源码在 `tools/arc-idx/`），协议见 [`Arc/_shared/code-intel.md`](Arc/_shared/code-intel.md)。任务编排、Inbox 和跨 Agent 状态由 `aitask` 体系负责。飞书项目空间只在用户明确要求、或项目已有 `.lark.json` 时，由 `lark-workflow-project-docs` 维护。

## 目录

```text
Skills/
├── Arc/
│   ├── _shared/        # 共享合同、检索协议、代码腐化清单、接口契约、约定（不被发现为技能）
│   ├── arc:explore/    # 定位、调用链、影响面、架构地图、索引维护
│   ├── arc:plan/       # PRD、需求澄清、任务文档、进度、发现转任务
│   ├── arc:code/       # 实现、根因修复、重构迁移、前端、注释、计时探针；scripts/gate.sh
│   ├── arc:verify/     # 测试、可靠性、PostgreSQL 发布资格（scripts/dbqual）、flaky、发布门禁
│   └── arc:audit/      # 只读评审、体检、AppSec、安全扫描
├── Android/            # Android/Compose 垂直技能
├── HarmonyOS/          # HarmonyOS/ArkTS 垂直技能
├── CNB/                # CNB 平台技能
├── Lark/               # 飞书/Lark 技能（含 lark-workflow-project-docs）
├── tools/arc-idx/
├── docs/
├── schemas/
├── scripts/
├── src/
└── tests/
```

[技能目录与分发边界](docs/skill-catalog.md) 列出全部运行入口。`skills.index.json` 与触发语料只覆盖 Arc；垂直技能保留各自的原生 frontmatter、`references/` 和 `.cursor/skills` 链接。

## Arc 路由

| 工作 | 技能 | 权限 |
|---|---|---|
| 理解代码：在哪、谁调用、改动波及什么、架构地图、索引维护 | `arc:explore` | 只读 |
| 把意图变成规格：PRD、简报与验收标准、任务文档、进度、发现转任务批次 | `arc:plan` | 只写 `docs/` |
| 改源码：实现、修复、重构/迁移、前端、注释、计时探针 | `arc:code` | 改源码 |
| 产出行为证据：测试、可靠性、PostgreSQL 发布资格、flaky、发布门禁 | `arc:verify` | 改测试并执行 |
| 只读评估：diff/PR 评审、体检与代码腐化、AppSec、扫描 | `arc:audit` | 只读，仅产出报告 |

旧名称（`arc:build`、`arc:fix`、`arc:test` 等 16 个）到新技能的对照见 [`Arc/README.md`](Arc/README.md#retired-names-200)。不安装别名。

## 共享规则

| 文档 | 用途 |
|---|---|
| [`docs/execution-truth.md`](docs/execution-truth.md) | 仓库级门禁：环境/分支/部署面、完成定义、范围锁、域主键、证据优先的根因修复、编码前思考、简洁梯度、精准修改、教训闭环。Android、CNB、HarmonyOS 的诊断类技能同样链接它 |
| [`Arc/_shared/contract.md`](Arc/_shared/contract.md) | Arc 合同：权限等级、任务分级 T0–T2、统一词表（PASS/FAIL/BLOCKED/NOT_RUN/FLAKY、严重度→P0–P3）、产物路径、环境规则、飞书钩子、提交格式、完成报告 |
| [`Arc/_shared/code-intel.md`](Arc/_shared/code-intel.md) | 检索协议与 8 项影响面清单 |
| [`Arc/_shared/code-rot.md`](Arc/_shared/code-rot.md) | AI 代码腐化 36 条（A–F 六个家族） |
| [`Arc/_shared/api-contract.md`](Arc/_shared/api-contract.md) | 仓库 Markdown 接口契约（默认 `docs/02-API记录/`，Apifox 仅在用户要求时作为下游副本） |
| [`docs/skill-engineering.md`](docs/skill-engineering.md) | Skill 工程契约：固定标题、token 预算、触发语料、校验 |
| [`CHANGELOG.md`](CHANGELOG.md) | 版本与发布说明 |

## 收敛原则

- 一个阶段一个主技能；跨阶段按名称交接，技能之间不互相加载。
- 任何技能诊断疑似缺陷、回归、性能或安全问题，都先遵守 [`execution-truth.md` §6](docs/execution-truth.md#6-evidence-first-root-cause-repair)：未确认问题或根因，不改生产行为、不宣称已修复。
- 修改函数、类型、字段、路由、表列、配置键、错误码之前，按 code-intel 的 8 项影响面清单逐项查询并记录。
- 完成声明必须附证据（命令与结果、`file:line`、测试名）；`arc:code` 用 `scripts/gate.sh check` 做基线感知的完成门禁。
- 不按项目大小自动创建飞书空间；项目已有 `.lark.json` 时，Arc 阶段结束把状态交给 `lark-workflow-project-docs`。
- 垂直业务 Skill 的前端面按 `Arc/_shared/conventions/frontend.md` 的平台默认栈交付，不按业务差异另起默认栈。
- 图表、浏览器、Lazycat、纯设计等垂直能力由对应专门 Skill 负责，不在 Arc 内重复建设。

## 校验

```bash
arc-idx doctor --format json
# 仅索引缺失/过期时：arc-idx index --quiet
.venv/bin/python scripts/validate_skills.py
.venv/bin/python -m pytest tests -q
.venv/bin/python scripts/build_skills_index.py
```

## 发布 / 同步到 Agent

Arc 技能的 SSOT 是本仓 `Arc/arc:*`；垂直技能的 SSOT 分别是本仓 `Android/`、`HarmonyOS/`、`CNB/`、`Lark/`。宿主通常通过 `~/.agents/skills` 或项目内 `.cursor/skills` 的符号链接加载技能。

发布器先在临时目录打完整包（技能目录内的 `agents/`、`scripts/`，所有可达的文件引用，以及 `metadata.requires.skills` 传递依赖；`Arc/_shared/` 与 `docs/` 中被引用的文件生成到各包的 `references/_shared/`），校验通过后再替换目标目录，并保留可恢复副本。每个宿主只安装一种名称：`--naming dash`（默认，`arc-code`）或 `--naming colon`（`arc:code`）。

```bash
# 改完技能后：校验 → 索引 → 先 dry-run，再安装
.venv/bin/python scripts/validate_skills.py
.venv/bin/python -m pytest tests -q
.venv/bin/python scripts/build_skills_index.py
.venv/bin/python scripts/sync_skills.py --dry-run
.venv/bin/python scripts/sync_skills.py --check
.venv/bin/python scripts/sync_skills.py --naming dash
.venv/bin/python scripts/sync_skills.py --only arc:code --only arc:verify --dry-run
.venv/bin/python scripts/sync_skills.py --family Lark --dry-run
.venv/bin/python scripts/import_lark_skills.py lark-meeting --dry-run
```

规则：

1. **先改本仓，再 sync**；禁止只改 `~/.agents/skills` 而不回写本仓。
2. 按宿主选择单一名称；Markdown/YAML 中的 `arc:*` 在 dash 宿主下会改写成 `arc-*`，`agents/openai.yaml` 的 `$name` 必须与安装名一致。
3. 缺失引用或声明依赖时安装失败，目标目录保持原样；成功替换时打印备份路径，失败则回滚本次变更。
4. 发布器只替换本次安装的名称；改名或删除技能后，旧名称的安装副本要手动移除，否则宿主会同时发现新旧两套。
5. Arc `SKILL.md` 带 `version`（semver）；破坏性约定变更时升 minor/major 并更新 `CHANGELOG.md`。
6. 若改动了 `tools/arc-idx`，另执行 `go build -o ~/.local/bin/arc-idx ./tools/arc-idx`。
