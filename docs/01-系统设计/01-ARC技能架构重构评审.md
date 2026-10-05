# ARC 技能体系架构重构评审

- 日期：2026-10-05
- 状态：方案，尚未实施
- 范围：`Arc/` 下 16 个技能；`reliability-engineering-skill/` 源包；仓库外已安装的 `arc:dbqual`；Arc 依赖的共享文档（`docs/execution-truth.md`、`docs/code-rot-taxonomy.md`、`docs/arc-routing-matrix.md`、`docs/arc-routing-cheatsheet.md`、`docs/orchestration-contract.md`、`docs/conductor-pattern.md`、`docs/prewalk.md`）；`Arc/scripts/`、`Arc/templates/`；校验器 `src/arc_core/` 与打包器 `scripts/sync_skills.py`。

评审方法：

- 逐个阅读上述文件，把内容拆成"规则、步骤、知识点"三类条目后交叉比对。
- Token 按"ASCII 字符 ÷ 4 + 中日韩字符 × 1"估算，误差约 ±15%，只用于相对比较。
- 重叠度是评审估计：A 的规范性条目中，能在 B 找到语义等价表述的比例。不是工具自动计算的结果。
- "重构后"的数字都是预算目标，不是测量值。
- `arc:dbqual` 在仓库外，本次只评审了它的结构与边界，没有逐条评审其 G0–G31 内容。

---

## 0. 结论摘要

ARC 现在按"动作"（build / fix / test / trace / comment）和"技术栈"（frontend / arch）切成 16 个技能。一个普通任务要串联 3–6 个技能，同时有 8 个横切规则族被复制到 5–14 个技能里。应当改为按"权限 × 产出物"切成 5 个技能，每个任务只加载 1 个主技能，按需读取它自己的 reference 或共享规范。

| 新技能 | 权限 | 产出物 | 吸收的现有技能 |
|---|---|---|---|
| `arc:explore` | 只读 | 定位结果、调用链、影响面清单、架构地图 | arc:idx |
| `arc:plan` | 只写文档 | PRD、任务简报、任务文档、进度 | arc:define、arc:clarify、arc:sdlc |
| `arc:code` | 修改源码 | 可达的代码变更 + 门禁证据 | arc:build、arc:fix、arc:frontend、arc:arch、arc:comment、arc:trace |
| `arc:verify` | 修改测试 + 执行 | 每条主张的 PASS / FAIL / BLOCKED / NOT_RUN / FLAKY 判定 | arc:test、arc:reliability、reliability-engineering-skill、arc:dbqual |
| `arc:audit` | 只读 | 分级、带证据的发现 | arc:audit、arc:security |

此外：`arc:docs` 迁出到飞书家族，改名 `lark-workflow-project-docs`；`arc:prewalk` 删除；团队规范与跨技能协议下沉到 `Arc/_shared/`。`Arc/_shared/` 没有 SKILL.md，不参与技能发现，由打包器按链接闭包随包分发。

| 指标 | 现在 | 重构后（预算） |
|---|---|---|
| Arc 可发现技能数 | 16（另有源包 1 个、仓库外 arc:dbqual 1 个） | 5 |
| Arc SKILL.md 总量 | 33.0K tokens | ≈14K |
| Arc 运行时语料（SKILL + references + Arc 专用 docs） | ≈104K | ≈56K |
| 每个任务需要加载的技能数 | 3–6 | 1 |
| 技能之间的交叉引用 | 96 条 | 只有"交接"一种，不再互相加载 |
| Go 后端缺陷修复的加载量 | 17.3K | ≈10.9K |
| 全栈功能的加载量 | 21.5K | ≈13.1K |
| 涉及 PostgreSQL 主张的可靠性检查 | 26.6K | ≈12.4K |

没有任何现有技能得到 KEEP：16 个技能都携带重复的横切规则或模板化区块，至少需要重写。能力上不做删减，而是新增 6 项：影响面检索协议、diff 评审模式、重构/迁移协议、基线感知的完成门禁、失败恢复阶梯，以及重新接回写代码时的数据层腐化门禁。完整去向见附录 A。

---

## 1. 当前 SKILL 总体诊断

### 1.1 规模与形态

| 技能 | SKILL.md | references | H2 节数 | 模板化区块占比 | 指向其他 arc 技能的引用 |
|---|---:|---:|---:|---:|---:|
| arc:arch | 2,074 | 6,664 | 10 | 11% | 4 |
| arc:audit | 2,968 | 2,453 | 13 | 59% | 7 |
| arc:build | 1,233 | 1,955 | 5 | 0% | 10 |
| arc:clarify | 1,337 | 0 | 13 | 64% | 5 |
| arc:comment | 762 | 2,363 | 5 | 17% | 0 |
| arc:define | 1,310 | 303 | 13 | 66% | 5 |
| arc:docs | 3,172 | 6,905 | 14 | 68% | 4 |
| arc:fix | 1,205 | 0 | 5 | 0% | 10 |
| arc:frontend | 3,432 | 3,195 | 13 | 76% | 7 |
| arc:idx | 1,244 | 0 | 6 | 0% | 4 |
| arc:prewalk | 2,116 | 60 | 14 | 59% | 5 |
| arc:reliability | 3,213 | 15,117 | 9 | 27% | 9 |
| arc:sdlc | 2,201 | 12,247 | 8 | 37% | 10 |
| arc:security | 3,068 | 1,684 | 14 | 64% | 5 |
| arc:test | 1,848 | 7,582 | 6 | 0% | 9 |
| arc:trace | 1,859 | 0 | 10 | 3% | 2 |
| **合计** | **33,042** | **60,528** | — | **42%** | **96** |

"模板化区块"指 Overview、Quick Contract、Hard Constraints、Quality Gates、Red Flags、Input Arguments、Outputs、Scripts & Commands、Context Search 这 9 种节。

隐性加载（不在仓库 requires 闭包里，但技能要求必须读）：

- `ponytail`：每次 Go 后端编辑都要加载，约 1.5K。arch、build、fix 都写了这条前置。
- `arc:dbqual`：test 和 reliability 遇到 PostgreSQL 主张时加载，约 7.8K。它的 SKILL.md 超过 1,100 行，是仓库 500 行上限的两倍多。本机同时装有 `arc-dbqual` 和 `arc:dbqual` 两份。
- `wxskills`：frontend 遇到微信平台时加载。

Arc 专用共享文档：

| 文档 | 大小 |
|---|---:|
| arc-routing-matrix | 2.7K |
| arc-routing-cheatsheet | 1.1K |
| orchestration-contract | 2.0K |
| prewalk | 1.8K |
| code-rot-taxonomy | 2.9K |
| conductor-pattern | 0.4K |

`execution-truth.md`（2.8K）是仓库级文档，Android、CNB、HarmonyOS 的 10 个技能也引用它。

### 1.2 结构性问题

1. **切分维度错误。** 按动作和栈切分，导致一个任务跨多个技能。修一个 Go 后端缺陷，要依次加载 fix → idx → arch → ponytail → test，再加 execution-truth，共 6 个文件、4 个 Arc 技能。全栈功能还要再加 frontend 和 sdlc。
2. **网状依赖。** 16 个技能互相引用 96 次，build、fix、sdlc 各 10 次。低能力模型在网状指引里容易漏读，也容易循环：audit 把扫描交给 security，security 又把资产表交回 audit。
3. **模板化复述。** 7 个技能共用一套 13–14 节模板。禁令在 Red Lines、Hard Constraints、Quality Gates、Red Flags 四个列表里重复出现；触发条件在 description、Overview、Quick Contract、When to Use 四处复述。这些区块占 Arc SKILL.md 总量的 42%，其中 frontend 76%、docs 68%、define 66%。
4. **执行骨架不完整。** 16 个技能中没有一个同时写明 Non-trigger、工具失败时的降级、失败恢复和完成判据。只有 arc:idx 写了工具降级，只有 build、fix、frontend、sdlc 提到完成定义。
5. **门禁停在纸面。** `Arc/scripts/` 下的 6 个门禁脚本和 `Arc/templates/` 下的 2 个模板没有被任何技能引用，只有 code-rot-taxonomy 提到这些脚本。所谓"验证"全靠文字承诺。
6. **依赖宿主不具备的能力。** arc:prewalk 要求会话中途切换模型，当前宿主不向技能开放这种能力。它的规则却被复制进 sdlc 模板：每份任务 README、前置约束、子任务各一段，并在每个用户项目里重复生成。
7. **外部依赖未声明。** ponytail、arc:dbqual、wxskills 都不在仓库里。打包器的 `metadata.requires` 闭包看不到它们，所以安装后的行为和仓库测试不一致。

### 1.3 最大重复区域：测试与可靠性

arc:test、arc:reliability、reliability-engineering-skill 和 arc:dbqual 的交叉部分，把同一套知识写了 3–7 遍。这套知识包括：

- 四阶段（正常 / 故障 / 恢复 / 变更）；
- EXPECTED / FORBIDDEN 主张；
- 独立 oracle 与缺陷敏感性；
- 生成器盲区；
- 状态词表；
- flaky 隔离；
- PR / 主干 / 候选 / 生产四级节奏；
- 依赖图选测的注意事项；
- "本机禁止启动 PostgreSQL 或容器"。

最后这一条单独出现在 7 个文件里。arc:test 与 arc:reliability 的重叠度约 75%，而且互相声明依赖："reliability 定义，test 执行"。

### 1.4 最大 token 浪费：横切规则复制

下表统计各 Arc SKILL.md 中涉及该规则族的行：

| 规则族 | 出现的技能数 | 合计 | 宿主技能之外的复制 |
|---|---:|---:|---:|
| 飞书激活 / 路由 / task_base | 12 | 3.9K | 2.0K |
| 任务分级 / 持久化文档（sdlc） | 14 | 2.2K | 1.9K |
| 根因修复门禁指针（校验器强制 9 个技能复制） | 9 | 0.9K | 0.9K |
| arc-idx 优先 | 12 | 1.0K | 0.6K |
| prewalk / 禁止 plan-postcard | 5 | 0.9K | 0.4K |
| 同类路径 / 兄弟实现 | 7 | 0.4K | 0.4K |
| 本机禁 PostgreSQL / 容器 | 3 | 0.3K | 0.3K |
| 不掩盖缺陷（重试、吞错、弱化测试） | 5 | 0.3K | 0.3K |

宿主之外的纯复制约 6.9K，占 SKILL.md 总量 21%。再加上 1.2 第 3 点的模板化复述，约 40% 的 SKILL.md token 不提供新信息。

另有两类非行为内容：

- `industry-testing-strategies.md`（3.0K）是出处与论证，不改变模型行为。
- `backend-architecture.md`（6.7K）中有大量 DTO / Entity 概念解释，模型本来就懂。

### 1.5 最大能力缺口：没有可执行的"影响面"方法

所有改代码的技能都说"检查同类路径、兄弟实现"，但没有一个说怎么查：

- 调用方；
- 结构体的构造和字段写入点；
- 接口实现；
- 序列化键；
- 路由消费方；
- 配置键；
- 测试与 mock；
- 文档与生成代码。

arc:idx 只是工具手册，不是方法。

其他缺口：

- **没有 diff / PR 评审方法。** arc:audit 只有 `changed_files` 这个输入参数，没有逐符号核对影响面的步骤。
- **没有重构 / 迁移协议。** 缺少特征测试先行、expand → migrate → contract 的分阶段步骤。
- **没有失败恢复阶梯。** 缺少重试预算、检索放宽顺序和停止条件。
- **数据层的写时门禁丢失。** code-rot-taxonomy 的 D / E 类指向 build 的 `## SQL Standards` 和 fix 的 `## SQL Failure Checks`，这两个节都已不存在。

### 1.6 矛盾与失效引用（逐条可复核）

| # | 位置 | 问题 |
|---|---|---|
| 1 | `Arc/scripts/check-scope.sh` | `MAX_FILES=${1:-5}` 超出即失败。这与 fix 的"修复完整的归属边界及所有兄弟路径"矛盾。它还统计整个脏工作区（包括用户自己的改动），却忽略未跟踪文件。 |
| 2 | `Arc/scripts/verify-project.sh` | 名为"验证"，却执行会改文件的 `go fmt ./...`。它用 if/elif 链识别栈，monorepo 中只会检查第一种栈。 |
| 3 | sdlc 模板与 execution-truth §7 vs arc:prewalk | 前者要求"为便宜的执行模型写好任务"，后者规定"禁止让便宜模型从任务文档冷启动"。 |
| 4 | appsec-playbook vs security-tooling | 发现状态有两套词表：confirmed / likely / assumption / manual-gap，与 confirmed / likely / needs triage / false positive / manual gap。 |
| 5 | 严重度 | 并存三套刻度：Critical–Info、数据价值 P0–P3 + Gap、任务优先级 P0–P4。pipeline §5.5 的映射里有"P0 或 P1（写明理由）"这种不确定项。 |
| 6 | arc:comment SKILL vs `modules/struct.md` | SKILL 禁止在只加注释的任务里重命名；模块却要求"把 comments、reports 重命名为暴露依赖角色的名字"。 |
| 7 | arc:idx | 要求 `arc-idx version` 精确输出 `arc-idx 1.0.0 — local AI context engine (zoekt + ctags + ast-grep)`。引擎一升级，所有会话都会停下。 |
| 8 | arc:idx vs 宿主 | 绝对禁止宿主原生搜索。工具优先级应当是"偏好 + 明确降级"，而不是与宿主指令对抗。 |
| 9 | clarify / define / docs | 仍写"用 `.ai-code-index/` helpers 搜索"。这些 helper 在 b523a50 中已被删除，现在的工具是 arc-idx。 |
| 10 | `docs/arc-routing-matrix.md` | 声称每个技能都有 `## Code Rot Gates` 节，实际一个都没有。 |
| 11 | `docs/orchestration-contract.md` | 声称"所有 arc:* 技能必须引用本契约"，实际没有一个技能引用。 |
| 12 | 产物路径 | 三种写法并存：`.arc/artifacts/<task>/logs/`、`tmp/logs/`、`.arc/security/<ts>/`。 |
| 13 | arc:arch | 强制先读外部 ponytail，却预先声明了与 ponytail 的 DIP 冲突。仓库内的 execution-truth §9 简洁梯度已经覆盖了 ponytail 的意图。 |
| 14 | 8 个技能的 "Decision Tree" | 都指向 2.7K 的路由矩阵。任务一来先读路由文档，再读技能。 |
| 15 | arc:docs `evidence-checklist.md` | 内容是学位论文 / Word 交付（python-docx 三线表），与飞书工作区无关。 |
| 16 | arc:frontend 脚手架（待核实） | `tailwindcss init -p`（Tailwind v4 已移除）、`@tanstack/router-devtools`（已改名）、shadcn `toast`（已被 sonner 取代）可能过时。重写时需对照当前官方文档。 |

---

## 2. 能力矩阵

| SKILL | 核心能力 | 输入 | 输出 | 使用工具 | 工作流 | 重叠度 | 建议 |
|---|---|---|---|---|---|---|---|
| arc:idx | arc-idx 引擎操作：身份与健康预检、files / search / symbol / ast / stats、索引与守护进程生命周期 | 查询意图 | 路径、符号、片段 | arc-idx | 预检 → 查询 → 有界阅读 → 报告查询证据 | 中：规则被 11 个技能复述 | **REWRITE** → arc:explore |
| arc:define | 需求定义：复述意图、对照代码现实、范围与非目标、用户故事、验收、开放问题 | 想法、需求 | docs/00 PRD | arc-idx、读文件 | 复述 → 检查 → 范围 → 故事 → 验收 → 交接 | 高：与 clarify 约 60% | **MERGE** → arc:plan（prd） |
| arc:clarify | 把模糊请求变成任务简报：目标、约束、假设、问题 | 模糊请求 | 任务简报 | arc-idx | 复述 → 检查 → 约束 → 问题 → 交给 sdlc | 高：与 define 约 60%，与 sdlc 约 40% | **MERGE** → arc:plan（brief） |
| arc:sdlc | docs/00–11 架构、任务文档编写、生命周期与状态、模板、安全发现任务流水线 | 需求、发现、计划 | 文档目录、任务 README、进度表 | 文件写入 | 分级 → 建档 → 编写 → 维护状态 | 中：与 clarify 约 40%，与 prewalk 约 35%；authoring 与 execution-truth §7 近乎逐字重复 | **MERGE** → arc:plan（主宿主） |
| arc:build | 功能实现、API 与迁移、交付证据 | 需求、任务 | 代码变更 + 证据 | arc-idx、项目工具链 | 分级 → 上下文 → 实现 → 验证 → 文档 | 高：与 fix 约 70% | **MERGE** → arc:code（implement，主宿主） |
| arc:fix | 证据优先修复：复现、假设证伪、根因、兄弟路径 | 报错、日志、失败测试 | 修复 + 复现证据 | arc-idx、调试器、日志 | 证据 → 复现 → 假设 → 修复 → 验证 | 高：与 build 约 70% | **MERGE** → arc:code（repair） |
| arc:frontend | React / TS 栈约定（Vite、TanStack、shadcn、Tailwind）、UI 状态、色彩 token、脚手架、四项 UI 检查 | UI 需求 | 页面、组件 | 包管理器、浏览器 | 识别栈 → 脚手架 → 实现 → UI 检查 | 中：与 build 约 45%；另复述 sdlc、prewalk、飞书、根因 | **MERGE** → arc:code + `_shared/conventions/frontend*.md` |
| arc:arch | Go 后端分层（handler / service / repo、DTO / Entity）、DIP、漂移模式、ponytail 前置 | 后端改动 | 符合分层的结构 | 外部 ponytail | 预检 → DIP → 清单 | 低：工作流重叠约 25%，但总是和 build / fix 一起加载 | **MERGE** → `_shared/conventions/go-backend.md` |
| arc:comment | Go 中文注释规范：函数、结构体、控制器 | 待注释代码 | 注释 | — | 按模块套规则 | 低：与 arch 约 20%（Repo 后缀命名两边都写） | **MERGE** → `_shared/conventions/comments.md` + arc:code（comment 模式） |
| arc:trace | slog 阶段计时探针、SSR 阶段模板、升级到 pprof | 慢请求 | 计时日志 | slog、pprof | 埋点 → 测量 → 报告 | 低：与 test-stacks 的 pprof / trace 约 30% | **MERGE** → `arc:code/references/perf-probes.md` |
| arc:test | 用例与执行合同、原生工具与环境、结果与交接 | 功能、变更 | 测试 + 结果 | 项目测试框架、Playwright | 设计 → 实现 → 运行 → 分类 | 高：与 reliability 约 75% | **MERGE** → arc:verify（主宿主） |
| arc:reliability | 业务不变量、事务与故障、发布证据、运维与事故学习 | 服务、变更 | 可靠性证据与判定 | 测试框架、arc:dbqual | 主张 → 范围 → 设计 → 执行 → 判定 | 高：与 test 约 75%，与源包约 80% | **MERGE** → arc:verify |
| arc:audit | 项目健康审计、AppSec Phase 0–6、发现卡、交接包 | 仓库、范围 | 审计报告 | arc-idx | 分阶段审查 | 中：与 security 约 55% | **REWRITE** → arc:audit（吸收 security，新增 review 模式） |
| arc:security | 运行与安装扫描器、DAST 授权、数据价值排序 | 仓库、目标 | 扫描结果分诊 | security-scan.py、semgrep、gitleaks 等 | 侦察 → 扫描 → 分诊 → 交接 | 中：与 audit 约 55% | **MERGE** → arc:audit（appsec / scan） |
| arc:docs | 飞书项目工作区：`.lark.json`、wiki / doc / base / task 同步；另含论文证据清单 | 项目 | 飞书资源 + 索引 | lark-cli | 激活 → 全量 / 增量 / 单资源同步 | 中：飞书规则被 11 个 Arc 技能复述 | **REWRITE**，迁出为 `Lark/lark-workflow-project-docs` |
| arc:prewalk | Guide → Executor 中途换模型 | 大任务 | 模型交接 | 宿主的模型切换能力 | 引导 → 首次生产编辑后切换 | 中：与 sdlc 约 35% | **DELETE** |
| reliability-engineering-skill | 可靠性方法论源包：工程依据、工具获取、运营模型、模板 | — | — | — | — | 高：与 arc:reliability 约 80% | **DELETE**（退出技能发现，有行为价值的部分并入 arc:verify） |
| arc:dbqual（仓库外） | PostgreSQL 发布资格 G0–G31、证据 JSON、门禁 | 数据库变更 | 资格报告 | psql、自带脚本 | 风险分级 → G 门禁 → 报告 | 高：与 reliability 的事务 / 发布部分重叠 | **MERGE** → arc:verify（db 模式）；纳入仓库，拆成按需 references |

---

## 3. 重复分析

### 3.1 职责重复

| 技能对 | 重叠度 | 重叠内容 | 各自独有 |
|---|---:|---|---|
| test ↔ reliability | 约 75% | 四阶段、主张、oracle、敏感性、状态词表、隔离、节奏、本机禁 PG | test：技术栈工具；reliability：事务 / 锁矩阵、发布排序、运维与事故 |
| reliability ↔ reliability-engineering-skill | 约 80% | 前者由后者吸收而来 | 源包：工具获取、工业运营模型 |
| build ↔ fix | 约 70% | 同一套 6 步骨架；Intent Router 10 行中 8 行指向相同目标；掩盖缺陷、捏造键名、飞书、sdlc、prewalk 等红线 | fix：证据捕获、复现、假设证伪、修复包 |
| define ↔ clarify | 约 60% | 复述 → 检查 → 范围 / 约束 → 开放问题 → 交接；飞书红线 | 粒度不同：PRD vs 任务简报 |
| audit ↔ security | 约 55% | playbook、数据价值排序、发现卡、交接包、人工缺口 | security：扫描器、安装、DAST 授权；audit：健康 rubric |
| build ↔ frontend | 约 45% | frontend 的工作流就是 build 的工作流加 UI 约定 | frontend：栈约定、token、脚手架 |
| clarify ↔ sdlc | 约 40% | sdlc 依赖 clarify 的输出；验收具体化；非目标 | sdlc：文档架构、生命周期 |
| sdlc ↔ prewalk | 约 35% | 交接语义；"执行模型假设"段落在 sdlc 模板中出现 3 次 | prewalk：换模型协议 |
| trace ↔ test-stacks | 约 30% | 先测基线、升级到 pprof / go tool trace | trace：slog 辅助函数、SSR 模板 |
| build ↔ arch | 约 25% | 工作流重叠低，但每次后端编辑都同时加载 | arch 本质上是规范，不是流程 |
| comment ↔ arch | 约 20% | 契约方法注释、Repo 后缀命名 | — |

重叠度 >70% 的配对必须合并。30–70% 的配对，在同一权限、同一产出物的前提下合并为模式。重叠 <30% 但"总是一起加载、没有独立流程"的技能（arch、comment），降为规范 reference。

### 3.2 流程重复

- "复述 → 检查代码现实 → 范围 / 约束 → 开放问题 → 交接"出现在 define、clarify、audit 三处。
- "实现 → 验证 → 更新任务文档 → 飞书交接"出现在 build、fix、frontend、test 四处。
- Handoff Package 与 R-recon / R-scan / R-task / R-fix / R-verify 角色描述在 audit、security、sdlc pipeline、路由矩阵、orchestration-contract 共 5 处出现。

### 3.3 规则重复

见 1.4 的规则族表。另有以下单条规则多处重复：

- "空结果是成功，不是错误"出现在 6 处：build、arch SKILL、arch reference、comment controller 模块、frontend、code-rot #8。
- "服务端校验归属与价格"出现在 5 处：build、fix、code-rot C 类、security-tooling 人工清单、appsec playbook。
- "接口文档用 Markdown 不用 JSON"出现在 6 处：build、sdlc 三处、api-contract、docs-architecture。
- "调试证据放 `.arc/artifacts/<task>/logs/`"有 3 种不同措辞：fix、frontend、arch reference。

### 3.4 工具说明重复

- arc-idx 命令示例出现在 idx、audit 的 Scripts & Commands、docs、sdlc 模板的 Context-Queries。
- `security-scan.py` 的调用方式在 security SKILL 和 sdlc pipeline 中各写一遍。
- pprof / go tool trace 在 trace SKILL 和 test-stacks 中各写一遍。
- Playwright / agent-browser 出现在 test、test-stacks、frontend、路由矩阵。

### 3.5 知识重复

- test-case-design ↔ business-verification：oracle 独立性、敏感性、生成器盲区、四阶段。
- evidence-and-release ↔ test-scope-matrix：节奏表、隔离、flaky、按调用图选测。
- appsec-playbook ↔ security-tooling ↔ data-value-ranking：人工清单、收益分级、P 分桶。
- backend-architecture ↔ templates/strict-type-safety ↔ code-rot A 类：响应 DTO 组合规则、Go 标准库常量。
- execution-truth §7 ↔ sdlc authoring "Downstream Task Authoring"：近乎逐字重复。
- 路由描述共 5 份：路由矩阵、cheatsheet、conductor-pattern、skill-engineering 生命周期、README。
- 飞书触发短语共 4 份：lark-index-contract、triggers.md、arc:docs Hard Constraints、路由矩阵快速规则。

---

## 4. 合并方案

### M1. arc:build + arc:fix + arc:frontend + arc:arch + arc:comment + arc:trace → NEW arc:code

理由：

- 这 6 个技能的权限相同（修改源码），产出物相同（可达的代码变更 + 证据）。
- 它们之间的差别是"模式"（实现 / 修复 / 重构 / 注释 / 性能）和"栈规范"（Go 后端 / 前端 / 注释），都不是独立的流程。
- 合并后，"修 Go 后端缺陷"只需加载 arc:code、repair 和 go-backend 规范；不再需要 arch、ponytail、test 三次跳转。

规范部分（arch、frontend 的栈约定、comment 规范）下沉到 `Arc/_shared/conventions/`，因为 arc:audit 也要把它们当评审 rubric，arc:explore 画架构地图时也要用。

### M2. arc:test + arc:reliability + reliability-engineering-skill + arc:dbqual → NEW arc:verify

理由：

- 四者的权限相同（写测试并执行），产出物相同（每条主张的判定），知识高度重叠（约 75–80%）。
- 目前 "reliability 定义、test 执行" 的分工要求两个技能同时加载。
- arc:dbqual 是 PostgreSQL 这一具体栈的深度资格门禁，应当成为 verify 的 `db` 模式，G0–G31 按组拆成按需读取的 references。这样回答一个"事务隔离"问题时，只读 G7–G11 那一组，不必加载 7.8K。

### M3. arc:audit + arc:security → NEW arc:audit

理由：

- 两者权限相同（只读），产出物相同（分级发现），共享 playbook、排序与发现卡。
- 现在二者循环交接资产表。合并后，扫描只是 appsec 的一个步骤。
- 同时补上缺失的 diff / PR 评审模式。

### M4. arc:define + arc:clarify + arc:sdlc → NEW arc:plan

理由：

- 三者都是"把意图变成可执行规格"，权限相同（只写文档），只是粒度不同：PRD、简报、任务文档。
- 合并后统一一套"可验收"标准，并顺带清除 sdlc 模板里的 prewalk 段落。

### M5. arc:idx →（重写为）NEW arc:explore + `_shared/code-intel.md`

理由：

- 代码情报（定位、调用链、影响面、架构地图）是独立且高频的只读能力，现在只有工具手册，没有方法。
- 工具合同与影响面协议放进 `_shared/code-intel.md`，供 code / verify / audit / plan 读取，不再要求加载第二个技能。

### M6. 共享文档合并

- **路由文档：** routing-matrix + routing-cheatsheet + conductor-pattern + orchestration-contract → `Arc/README.md` 中一张 5 行路由表。
- **门禁脚本：** `Arc/scripts/*.sh`（6 个）→ `arc:code/scripts/gate.sh`，基线感知、不修改文件，详见 6.7。
- **模板：** `Arc/templates/strict-type-safety.md` 并入 `conventions/go-backend.md`；`git-commit-convention.md` 的 4 行规则并入 `_shared/contract.md` 的"提交"节。
- **腐化分类：** `docs/code-rot-taxonomy.md` 移入 `Arc/_shared/code-rot.md`，作为 audit 的 rubric；arc:code 按编号引用其中的写时门禁项。
- **发现转任务：** `arc:sdlc/references/security-audit-task-pipeline.md` 精简为 `Arc/_shared/finding-campaign.md`，audit 与 plan 共用。
- **接口契约：** `arc:build/references/api-contract.md` 移到 `Arc/_shared/api-contract.md`，code 写、plan 引用、audit 核对。

### M7. arc:docs →（迁出）Lark/lark-workflow-project-docs

理由：

- 飞书工作区编排不是工程能力，却让 11 个 Arc 技能各复制一段飞书规则。
- 迁到飞书家族后，Arc 侧只保留 `_shared/contract.md` 中的一段钩子：项目存在 `.lark.json` 且任务有跟踪状态时，在收尾交给 lark-workflow-project-docs；否则从不创建飞书资源。
- 命名沿用飞书家族的 `lark-workflow-*` 约定。

---

## 5. 删除方案

| 删除对象 | 原因 | 内容去向 |
|---|---|---|
| arc:prewalk（技能） | 依赖宿主不提供的中途换模型能力，运行时价值约为 0，且与"为执行者写任务"矛盾 | `docs/prewalk.md` 移到 `docs/01-系统设计/` 作为宿主侧设计记录；从 sdlc 模板、build / fix / frontend / sdlc 红线、execution-truth §7 中删除 prewalk 段落 |
| reliability-engineering-skill（技能发现） | 已被 arc:reliability 吸收，重叠约 80%，再保留一个可发现的 SKILL.md 只会造成重复路由 | 工具获取、工业运营模型里有行为价值的部分并入 `arc:verify/references/release-ops.md`；其余作为出处归档，不参与发现 |
| `docs/arc-routing-matrix.md`、`arc-routing-cheatsheet.md`、`conductor-pattern.md`、`orchestration-contract.md` | 5 份路由描述只需 1 份；矩阵中有失效声明（`## Code Rot Gates`），契约中有无人遵守的强制条款 | `Arc/README.md` 的 5 行路由表 |
| `Arc/scripts/*.sh` | 无人引用；scope 与 verify 两个脚本的行为有缺陷 | `arc:code/scripts/gate.sh` |
| `Arc/templates/*.md` | 无人引用；内容与 backend-architecture 重复 | conventions/go-backend.md 与 `_shared/contract.md` |
| `arc:docs/references/evidence-checklist.md` | 学位论文交付与飞书工作区无关 | 删除；用户若仍需要，应作为独立的非 Arc 技能 |
| `arc:reliability/references/industry-testing-strategies.md` | 出处与论证，不改变行为 | 移到非运行时 docs |
| 所有模板化区块（Overview、Quick Contract、Hard Constraints、Quality Gates、Red Flags、Input Arguments、Outputs、Scripts & Commands、Context Search） | 复述 description 与 Red Lines | 由第 10 节的 10 个固定标题取代 |
| 运行时文件中的"吸收来源 / absorbed from"说明 | 对执行无作用 | 移到 CHANGELOG |
| arc:arch 的 ponytail 前置 | 外部依赖，且与 DIP 预设冲突 | execution-truth §9 简洁梯度 |
| arc:idx 精确版本串检查 | 引擎一升级就全部中断 | 只检查主版本兼容 |
| 重复安装的 `~/.claude/skills/arc-dbqual` 与 `arc:dbqual` | 两份同时被发现 | 纳入仓库后由打包器统一发布一份 |

---

## 6. 重写方案

### 6.0 共享层（先于各技能落地）

**`docs/execution-truth.md`（仓库级，保留路径，瘦身到约 1.8K）**

- 保留 §1–§6、§8–§10、§11 的核心条款。
- §6 根因修复的标题文字不变，因为 Android、CNB、HarmonyOS 的 10 个技能链接它，校验器也检查这个标题。
- §7"为便宜执行者写任务"移到 `arc:plan/references/task-docs.md`。
- 删除全部 prewalk 措辞。

**`Arc/_shared/contract.md`（约 0.9K，所有 Arc 技能第一步读取）**

1. **权限等级：** explore、audit 只读源码；plan 只写 `docs/`；code 修改源码；verify 修改测试并执行。越权时停止并说明。
2. **任务分级：**
   - T0：只改 1 个文件，不改契约、schema、安全逻辑，不加依赖。
   - T1：改动在单个模块内，不影响外部契约。
   - T2：命中任一条件即为 T2，包括改公共 API / DTO、数据库 schema、鉴权、支付或金额、并发，跨 ≥3 个包，迁移，新增依赖。
   - T2 必须有简报。跨会话或多人协作时，还要用 arc:plan 建任务文档。
3. **范围锁：** 用户或任务文档给出范围（文件、目录、符号）时，改动范围外的文件前必须停下报告。
4. **统一词表：**
   - 判定：PASS / FAIL / BLOCKED / NOT_RUN / FLAKY。
   - 任务状态：`[ ]` `[/]` `[?]` `[x]` `[!]` `[-]`。
   - 发现状态：confirmed / likely / needs-triage / false-positive / manual-gap。
   - 严重度：Critical / High / Medium / Low / Info。
   - 任务优先级由严重度唯一映射：Critical → P0，High → P1，Medium → P2，Low / Info → P3。
5. **产物路径：** 统一为 `.arc/artifacts/<task-slug>/{logs,reports,security}/`。除非用户要求，不提交产物；输出前脱敏密钥与个人信息。
6. **环境：**
   - 破坏性命令（删除数据、强推、重置、DROP）必须先获得明确批准。
   - 安装工具前要获得许可。
   - 不在本机启动 PostgreSQL 或容器运行时；用项目提供的测试环境或 mock，缺失时记 BLOCKED。
7. **飞书钩子：** 见 M7，只有一段，任何 Arc 技能都不直接写飞书。
8. **提交：** 只在用户要求时提交，格式为 `<type>: <description>`，type 取 feat / fix / refactor / docs / chore / test。
9. **完成报告格式（5 行）：** 做了什么、证据（命令 + 结果）、改动文件、未验证项及原因、后续建议。

**`Arc/_shared/code-intel.md`（约 1.2K，由 arc:explore 拥有，其他技能读取）**

- **预检：** 每个仓库每个会话执行一次 `arc-idx doctor --format json`。不健康时执行一次 `arc-idx index --quiet`；守护进程不在时执行 `arc-idx daemon start`（返回 `already running` 视为正常）。版本只检查主版本。
- **查询阶梯（按顺序）：** 精确符号 → 精确字符串 → AST 结构 → 文件名 → 放宽正则（大小写、部分匹配、snake/camel 变体）→ 宿主原生搜索（仅限降级条件）。
- **影响面清单。** 修改任何函数、结构体、字段、路由、表列、配置键、错误码之前，逐项执行。每项记录"查询 → 命中数 → 文件"。没有命中时要写"无（查询 X）"，不能跳过。

| # | 面 | 查询模板（首次使用先在当前仓库验证语法） |
|---|---|---|
| 1 | 定义与实现 | `arc-idx symbol <Name> --exact --format json`；接口实现：`arc-idx ast --lang go 'func ($_ $_) <Method>($$$) $$$' --format json` |
| 2 | 调用方与引用 | `arc-idx ast --lang go '<Name>($$$)'`、`'$R.<Name>($$$)'`；文本兜底：`arc-idx search 'lang:go case:yes <Name>'`（覆盖函数值传递和按名字反射） |
| 3 | 构造与写入 | `arc-idx ast --lang go '<Type>{$$$}'`、`arc-idx symbol New<Type> --exact`、`arc-idx search 'case:yes .<Field> ='` |
| 4 | 读取与序列化 | `arc-idx search 'case:yes json:"<key>'`；同时搜索模板、SQL 中的列名 |
| 5 | 契约边界 | `arc-idx search '<route-path>'`，同时覆盖服务端注册、前端 API 模块、Markdown 接口文档；DTO 在前端的同名类型；消息 schema；迁移文件 |
| 6 | 配置与装配 | `arc-idx search 'case:yes <CONFIG_KEY>'`，覆盖代码、env 样例、部署清单；依赖注入注册；定时任务与消费者注册 |
| 7 | 测试 | `arc-idx search 'f:_test\.go$ <Name>'`、`arc-idx search 'f:\.(test|spec)\.tsx?$ <Name>'`；fixture、golden 文件；生成的 mock 需要重新生成 |
| 8 | 文档与生成代码 | `docs/` 中的引用；生成代码要追到生成器输入（proto、openapi、sqlc），不改生成产物 |

- **零命中处理（放宽顺序）：**
  1. 检查拼写、大小写和命名变体；
  2. 搜索字面量形式（路由串、JSON tag）；
  3. 改用 AST 查询；
  4. `doctor` 检查索引新鲜度；
  5. 对未索引目录降级为宿主搜索。
  
  走完 5 级仍无结果时，报告"未找到"并列出所有查询，不得猜测路径。
- **命中过多（>200）：** 按目录、语言、单词边界逐步收窄；仍过多时按目录抽样分类，并注明是抽样。
- **动态分发：** 遇到接口、反射、依赖注入、字符串注册的路由时，同时按方法名和注册字符串搜索。仍无法解析的，标记"动态，需运行时确认"。
- **降级条件：** arc-idx 不存在；修复一次后仍不健康；目标在索引之外。降级证据标注 `fallback:<tool>`。

### 6.1 arc:explore

```yaml
name: arc:explore
description: >
  Use when the user wants to understand code without changing it: locate files or
  definitions, find callers, trace a call chain or data flow, list what a planned
  change would affect, or map a module or system architecture. Also maintains the
  arc-idx index (doctor/index/daemon). 中文：代码在哪、谁调用、调用链、影响面、模块梳理、
  架构地图、索引维护。Not for edits (arc:code), read-only risk review (arc:audit),
  or tests (arc:verify).
```

- **Trigger：** 定位、追链、影响面、架构理解、索引维护。
- **Non-trigger：**
  - 需要改代码 → arc:code（自带检索协议）；
  - 找问题 → arc:audit；
  - 已知确切路径的单文件阅读 → 直接读；
  - 第三方库文档 → 宿主的网页或文档工具。
- **Goal：** 用可复现的查询证据回答"在哪里、谁依赖、改动会波及什么"，输出可以直接交给 arc:code 的清单。
- **Inputs：**
  - question（必填）；
  - scope 路径（可选）；
  - 种子：符号、路由、表、配置键、报错串（可选）；
  - depth（默认 3 跳）。
- **Modes：** locate / trace / impact / map / index-ops。
- **Workflow：**
  1. 判定模式。问题里出现"改、换、删、重命名"时，一律走 impact 模式。
  2. 读 `_shared/contract.md`，执行 code-intel 预检，并记录索引状态。
  3. 把问题转成种子：标识符、路由串、报错串、表名或列名、配置键。不用自然语言句子做查询。
  4. 按模式执行：
     - locate：按查询阶梯查找。
     - trace：定义 → 调用方 → 逐层向上，直到入口（路由注册、main、cron、消费者）或达到深度上限。
     - impact：按影响面清单 8 项逐项执行。
     - map：`arc-idx stats` → 按层列 `files` → 读入口文件 → 对照 `conventions/go-backend.md` 或 `frontend.md` 标注分层。
  5. 打开每个命中的文件阅读，不依赖片段下结论。
  6. 对 impact 模式，每个面用两类查询交叉核对（符号 + 文本），两者不一致时以 AST 为准，并重新阅读相关文件。
  7. 按完成判据输出结果。
- **Tool Strategy：**
  - 首选：arc-idx。
  - 降级：见 code-intel。
  - 禁止：
    - 猜路径；
    - 只用一种查询就断言"没有调用方"；
    - 线性通读大目录；
    - 用自然语言做查询。
- **Decision Rules：**
  - 深度达到上限时，列出未展开的前沿节点，不默默截断。
  - 生成代码一律追到生成器输入。
  - 命中位于 vendor 或第三方目录时，只报告不展开。
- **Verification：**
  - 每条结论都有阅读过的 `file:line`；
  - 每个"无"都列出所用查询；
  - impact 的 8 项全部填写。
- **Failure Recovery：**
  - 索引故障：修复一次，仍失败就降级并标注。
  - 结果互相矛盾：重新阅读，以 AST 为准。
  - 种子太泛（>200 命中）：先收窄 scope，再继续。
- **Completion Criteria：** 输出包含：
  1. 答案；
  2. 证据表（查询 → 命中 → 文件）；
  3. impact 模式下的 8 项表；
  4. 动态或未解析项；
  5. 下一步建议（例如交给 arc:code 的编辑清单）。

### 6.2 arc:code

```yaml
name: arc:code
description: >
  Use for any task that changes source code: implement a feature or endpoint, fix a
  bug, error, crash or failing test, refactor or migrate, change an API or database
  schema, build frontend pages or components, add code comments, or add timing probes
  to a slow path. 中文：实现、开发、修复、报错、重构、迁移、改接口、改表、前端页面、加注释、
  加耗时埋点。Not for read-only review (arc:audit), test design or release evidence
  (arc:verify), PRDs or task docs (arc:plan).
```

- **Trigger：** 任何会修改源码的请求。
- **Non-trigger：**
  - 只解释代码 → explore；
  - 只评审 → audit；
  - 测试本身是交付物，或需要发布判定 → verify；
  - 只写需求或任务 → plan；
  - 飞书操作 → 飞书家族。
- **Goal：** 用最小但完整的改动，让请求的行为在真实入口上成立；同类路径一并修正；用项目门禁证明。
- **Inputs：**
  - 请求（必填）；
  - 任务文档路径（可选）；
  - repair 模式下的报错、日志或失败测试；
  - 范围锁（可选）。
- **Modes：** implement / repair / refactor / comment / perf。
- **Workflow：**
  1. 读 `_shared/contract.md`，判定模式与 T0 / T1 / T2。T2 没有简报时，先在回复中写不超过 10 行的内联简报（目标、范围、非目标、验收命令）再继续；涉及破坏性操作时先请求批准。
  2. 执行 `scripts/gate.sh start`，记录基线（HEAD、已有的脏文件和未跟踪文件），避免把用户先前的改动算进本次。
  3. 只读取本次涉及的栈规范：Go 后端读 `conventions/go-backend.md`，前端读 `conventions/frontend.md`（改主题时再读 `frontend-tokens.md`），注释读 `conventions/comments.md`。
  4. 对每个要改签名或语义的符号，执行 code-intel 影响面清单，在动手前写出影响列表（`file:line`）。
  5. 按模式补充：
     - repair：读 `references/repair.md`，捕获并复现失败；列出 ≥2 个假设并逐个证伪；确定归属边界；按缺陷模式搜索兄弟位置。
     - refactor：读 `references/refactor.md`，先补特征测试固定现有行为。API 或数据库按 expand → migrate → contract 分阶段进行。
     - perf：读 `references/perf-probes.md`，先测基线。
     - comment：只读 `conventions/comments.md`。
  6. 按"契约 / 类型 → 实现 → 调用方 → 测试 → 文档"的顺序修改。影响列表逐项处理，或写明为什么不受影响。
  7. 随变更测试：每个变更的行为至少有一个"旧代码会失败"的测试。repair 模式的复现测试必须先失败、后通过。T2 或测试设计较重时，读 `arc:verify/references/case-design.md`。
  8. 执行 `scripts/gate.sh check`，见 6.7。T2 还要执行项目的全量测试命令。
  9. 走一遍真实入口：HTTP 调用、CLI、UI 操作或经过 handler 的测试。只测到 helper 不算可达。
  10. 若存在任务文档，更新状态；若存在 `.lark.json`，按飞书钩子交接；然后按 5 行格式汇报。
- **Tool Strategy：**
  - 检索：只按 code-intel 执行。
  - 首选：项目自带的格式化、lint、类型检查和测试命令。
  - 禁止：
    - `--no-verify`；
    - 关闭 lint 或测试；
    - 用 `any`、`interface{}`、`@ts-ignore`、`//nolint`、`# type: ignore` 压制诊断；
    - 吞掉错误；
    - 用重试或 sleep 掩盖竞态；
    - 对整个仓库运行会改文件的格式化；
    - 直接修改生成产物；
    - 捏造配置键、表名或枚举值。
  - 降级：项目没有某项门禁时，该项记 NOT_RUN 并写明原因，绝不记 PASS。
- **Decision Rules：**
  - **改函数签名：** 同一改动内更新影响列表中的全部调用方。调用点超过 30 个时停下，提议加适配层或弃用包装，分阶段迁移。
  - **改公共结构体 / DTO 字段：** 必须覆盖构造点、读取点、JSON tag、接口文档、前端类型、迁移。重命名按 expand / contract 进行，除非所有消费方都在本仓库并且本次一起修改。
  - **改 API：** 同时修改服务端 handler、DTO、客户端调用与类型、`_shared/api-contract.md` 格式的接口文档、测试、迁移说明。默认保持向后兼容，除非任务明确允许破坏性变更。
  - **改数据库 schema：** 新增迁移（含 up / down）、模型、查询、索引，以及回填方案。绝不修改已经执行过的迁移。写 SQL 时执行 `_shared/code-rot.md` 中 D、E 类的写时门禁。
  - **错误与空值：** 返回或传播带类型的错误。空列表是成功，除非契约另有规定。
  - **修复位置：** 在归属边界修复，不在症状处打补丁。按模式搜到的兄弟位置一并修复。若需要改动范围锁以外的文件，停下报告。
  - **简洁梯度：** 复用现有函数 → 扩展现有函数 → 新增局部函数 → 新增模块。新增依赖前，说明为什么标准库和现有依赖不够用。
  - **comment 模式：** 只改注释，不重命名、不改逻辑。命名含糊之处列为建议，不直接修改（同时消除 1.6 第 6 条矛盾）。
  - **perf 模式：** 每次只改一处，并重新测量。探针按任务要求保留在日志级别开关后，或在完成前移除。
- **Verification：**
  - gate.sh 每项为 PASS，或是有理由的 NOT_RUN；
  - repair 模式有失败 → 通过的证据；
  - 真实入口已走通；
  - 影响列表全部处理完毕。
- **Failure Recovery：**
  - **门禁在本次 diff 之外失败：** 与基线运行对比。基线本来就失败的，作为既有问题报告，不顺手修改。
  - **改后测试失败：** 先读失败原因，不弱化断言。只有当测试编码的旧契约被本任务有意改变时，才修改测试并说明原因。
  - **同一假设两次修复失败：** 回到第 5 步，重新取证并扩充假设。第 3 轮仍失败时停下，报告已证伪的假设和下一步实验。
  - **无法复现：** 在边界加探针或日志取证，不做推测性修复，结论标记为"仅假设"。
  - **工具链缺失：** 记 NOT_RUN 并说明缺什么，不声称通过。
- **Completion Criteria：**
  - 请求的行为在真实入口上成立；
  - 影响列表全部处理；
  - 测试已加入并通过；
  - gate.sh 无 FAIL；
  - 契约和文档已同步；
  - 没有占位实现；
  - 已按 5 行格式汇报。
- **References：**
  - `repair.md`（约 1.0K）：证据、复现、假设证伪、修复包；
  - `refactor.md`（约 0.8K）：特征测试、分阶段迁移；
  - `perf-probes.md`（约 0.9K）：原 trace 的 slog 辅助函数与 SSR 模板，加 pprof 升级路径。

### 6.3 arc:verify

```yaml
name: arc:verify
description: >
  Use when the goal is evidence about behavior: design or write tests, a test plan,
  E2E or regression suites, coverage for a change, reliability checks (transactions,
  concurrency, idempotency, fault and recovery), PostgreSQL qualification, flaky
  tests, load or performance tests, or a release go/no-go gate. 中文：写测试、测试方案、
  回归、E2E、可靠性、并发、幂等、事务、数据库资格、压测、发布验收。Not for fixing the defects
  found (arc:code) or static review (arc:audit).
```

- **Trigger：** 测试或证据本身就是交付物。
- **Non-trigger：**
  - 修复发现的缺陷 → code；
  - 修复过程中顺带写复现测试 → code 自带；
  - 静态评审 → audit。
- **Goal：** 为每条明确的主张给出带证据的判定，并且所用测试能够检出它所针对的缺陷。
- **Inputs：**
  - target：功能、变更、diff 或服务；
  - 主张或验收标准。缺失时从代码和文档推导，并标记为"推导"。
  - 可用环境。
- **Modes：**
  - test：写并运行测试；
  - plan：只出验证方案，不执行；
  - reliability：事务、并发、故障；
  - db：PostgreSQL 资格；
  - release：发布门禁；
  - perf：负载与性能。
- **Workflow：**
  1. 读 `_shared/contract.md`，把主张逐条写成 EXPECTED（必须发生）或 FORBIDDEN（绝不能发生），并注明来源。
  2. 用 code-intel 影响面把变更映射到受影响的行为，再按 `references/scope-cadence.md` 选择层级：单元、集成、契约、E2E、可靠性。
  3. 按 `references/case-design.md` 设计用例，覆盖正常、边界、故障、恢复、变更五个阶段。每个用例写明独立 oracle（不复用被测逻辑）和它能检出的缺陷。
  4. 准备环境：用项目已有的测试框架和测试环境。环境缺失时，对应主张记 BLOCKED。
  5. 把测试写进仓库的测试目录，执行并保存输出到 `.arc/artifacts/<task>/logs/`。
  6. 敏感性验证：每条关键主张至少证明一次"测试在坏版本上会失败"，方法是临时回退修复或注入缺陷，验证后恢复。
  7. 逐条给出 PASS / FAIL / BLOCKED / NOT_RUN / FLAKY 判定，并附证据路径。
  8. release 模式套用门禁：
     - 任一 FORBIDDEN 主张为 FAIL → NO-GO；
     - 关键主张为 BLOCKED → NO-GO，除非用户明确豁免；
     - FLAKY 按隔离规则处理。
  9. 交接：FAIL 项连同证据形成修复清单交给 arc:code；规格缺口列为 SPECIFICATION GAP。
- **Tool Strategy：**
  - 首选：项目的测试命令。
  - 按栈：
    - Go：`go test -race`；
    - 前端：vitest / jest；
    - UI：Playwright 或 agent-browser，执行四项 UI 检查；
    - 负载：k6 或 vegeta；
    - db：按 `references/db/` 的分组执行。
  - 禁止：
    - 对被测对象本身打 mock；
    - 为了通过而更新快照；
    - 用重试掩盖 flaky；
    - 用 sleep 做同步。
  - 降级：缺 E2E 设施时改做 API 级契约测试，并把 E2E 记为 NOT_RUN。
- **Decision Rules：**
  - **并发主张：** 必须有真正的并发测试（并行 goroutine 或请求），并打开竞态检测。
  - **幂等主张：** 用相同幂等键重复请求至少 2 次，断言副作用只发生一次。
  - **事务主张：** 在事务中途注入失败，断言整体回滚。
  - **金额或库存：** 加守恒不变量（总和不变）。
  - **flaky 判定：** 最多重跑 3 次用于分类。flaky 永远不记 PASS。
  - **db 模式：** 先按风险分级，只读取命中的 G 组 reference，不整份加载。
- **Verification：**
  - 每条主张都有判定和证据；
  - 每条关键主张至少有一次敏感性证明；
  - 所有命令可复制执行。
- **Failure Recovery：**
  - **环境缺失：** 记 BLOCKED，写明缺什么、已经用什么替代。
  - **测试设施本身坏了（基线即失败）：** 记录基线，不当成本次结论。
  - **规格不清：** 记 SPECIFICATION GAP，询问用户，或在标注后按推导继续。
- **Completion Criteria：**
  - 主张表完整；
  - 测试已进入仓库，不只存在于产物目录；
  - 列出了所有运行命令；
  - release 模式给出 GO / NO-GO。
- **References：**

  | 文件 | 预算 | 来源 |
  |---|---:|---|
  | `case-design.md` | 2.8K | test-case-design + business-verification |
  | `scope-cadence.md` | 2.0K | test-scope-matrix + evidence-and-release 的节奏部分 |
  | `stacks.md` | 2.0K | test-stacks，移出 pprof；加入 frontend 的 UI 检查 |
  | `txn-faults.md` | 0.9K | transactions-and-faults |
  | `release-ops.md` | 1.8K | evidence-and-release 的发布部分 + operations-and-learning + change-verification + 源包中有行为价值的部分 |
  | `worked-example.md` | 1.5K | 原 worked-example 精简 |
  | `db/*.md` | 每组 ≤2.5K | 原 arc:dbqual 的 G0–G31 按 5 组拆分，脚本放 `scripts/dbqual/` |

### 6.4 arc:audit

```yaml
name: arc:audit
description: >
  Use for read-only evaluation that produces findings: review a diff, PR or branch,
  project health or code-rot audit, tech-debt inventory, security or AppSec audit,
  threat model, secrets, dependency or SAST scans, and authorized DAST. Never edits
  code. 中文：代码评审、审查、体检、技术债、安全审计、漏洞扫描。Not for fixes (arc:code),
  tests (arc:verify), or posting comments on CNB PRs (cnb-code-review).
```

- **Trigger：** 评审、体检、安全审计、扫描。
- **Non-trigger：**
  - 修复 → code；
  - 测试 → verify；
  - 在 CNB 平台上发评审评论 → cnb-code-review。
- **Goal：** 给出分级、带证据、可直接执行的发现，过程中不修改代码。
- **Inputs：**
  - target：diff 范围、PR、路径或整个仓库；
  - mode；
  - DAST 目标与书面授权（仅 scan 模式需要）；
  - 安装许可（可选）。
- **Modes：**
  - review：diff 评审；
  - health：按 code-rot rubric 体检；
  - appsec：人工威胁驱动审计；
  - scan：工具扫描。
- **Workflow：**
  1. 读 `_shared/contract.md` 并记录 `git status` 基线，用于完成时证明没有修改。确定模式和范围。
  2. 读取涉及栈的 conventions 作为 rubric。
  3. 按模式执行：
     - review：
       1. 运行 `git diff <base>...HEAD --stat`，列出变更符号。
       2. 对每个变更符号执行 code-intel 影响面，找出 diff 之外被打破的地方。
       3. 逐项检查错误路径、空值、并发、事务边界、鉴权，以及测试是否随行为一起修改。
       4. 规则见 `references/diff-review.md`。
     - health：按 `_shared/code-rot.md` 的 36 项，用每项列出的查询逐类检查。用 `arc-idx stats` 和 `git log` 的变更频率选热点。
     - appsec：按 `references/appsec.md` 的 Phase 0–6 执行：资产表 → 数据地图 → 入口清单 → 逐入口检查鉴权、输入和数据流，最后按数据价值排序。
     - scan：执行 `scripts/security-scan.py`，只使用已安装的工具。安装需要许可；DAST 需要书面授权和明确目标。每条命中都要分诊到 confirmed / likely / false-positive / manual-gap。
  4. 写发现卡。字段：ID、严重度、状态、`file:line`、证据（查询或运行结果）、影响、修复方向、验证建议。
  5. 去重并排序，输出覆盖表：查了什么、没查什么、人工缺口。
  6. 只有用户要求生成任务时，才按 `_shared/finding-campaign.md` 把发现转成任务批次。
- **Tool Strategy：**
  - 首选：code-intel；扫描器只有在已安装或已获许可时使用，按 `references/scanners.md`。
  - 禁止：
    - 修改代码；
    - 未授权执行 DAST 或破坏性探测；
    - 在输出中暴露密钥；
    - 把未经验证的扫描结果标为 confirmed。
  - 降级：工具缺失时，记为人工缺口，并执行对应的人工检查清单。
- **Decision Rules：**
  - **confirmed：** 必须有复现路径，或有从入口到汇点的代码追踪。
  - **review 模式分两级：**
    - 阻断项：正确性、安全、数据丢失、契约破坏；
    - 建议项：可读性、风格。
  - **可执行性：** 每条发现都必须给出具体修复方向。"建议加强校验"这类表述不合格。
  - **仓库过大：** 按数据价值和入口优先，未覆盖的区域写进覆盖表。
- **Verification：**
  - 每条 confirmed 都有追踪证据；
  - 覆盖表完整；
  - `git status` 与基线一致。
- **Failure Recovery：**
  - 扫描器报错：记录命令与错误，改用人工清单。
  - 结果冲突：回到代码重新追踪，以代码为准。
- **Completion Criteria：**
  - 有发现表；
  - 有覆盖与缺口表；
  - 有 Top-N 优先级；
  - 没有修改任何代码。
- **References：**
  - `diff-review.md`（约 0.9K，新增）；
  - `appsec.md`（约 3.0K，playbook + 数据价值排序合并去重）；
  - `scanners.md`（约 1.2K）。

### 6.5 arc:plan

```yaml
name: arc:plan
description: >
  Use when the deliverable is a spec or plan rather than code: write a PRD or
  requirements, turn a vague request into a task brief, break work into executable
  tasks, set up or maintain docs/00-11 and progress tracking, or convert audit
  findings into task batches. 中文：需求文档、PRD、需求澄清、任务拆解、任务文档、进度跟踪、
  文档结构。Not for implementation (arc:code) or Lark workspace sync
  (lark-workflow-project-docs).
```

- **Trigger：** 交付物是需求、简报、任务或文档结构。
- **Non-trigger：**
  - 实现 → code；
  - T0 / T1 任务的简报 → code 内联完成；
  - 飞书同步 → 飞书家族。
- **Goal：** 产出规格，每个任务都写明范围、文件、验收命令，执行者不依赖隐藏上下文就能验收。
- **Inputs：**
  - 意图或需求（必填）；
  - 现有 docs 路径（可选）；
  - 发现清单（campaign 模式需要）。
- **Modes：** prd / brief / tasks / campaign / docs-layout。
- **Workflow：**
  1. 读 `_shared/contract.md`，用 3 行复述意图：目标、用户、成功度量。
  2. 用 code-intel 查看代码现实：现有模块、路由、模型。不对着想象中的结构做规划。
  3. 列出未知项并分类：
     - 阻断项：问用户，最多 3 个问题，每个附推荐默认值；
     - 非阻断项：做出假设，并标注。
  4. 按模式模板产出：`references/prd.md`、`brief.md`、`task-docs.md`、`docs-layout.md`；campaign 模式读 `_shared/finding-campaign.md`。
  5. 每个任务必须包含：
     - 目标；
     - 范围锁（文件或目录）；
     - 非目标；
     - 步骤；
     - 影响列表；
     - 可复制的验收命令；
     - 状态标记。
     
     任务规模不超过 1 天或约 10 个文件，依赖关系写明。
  6. 自检：每条验收标准都能用命令或可观察的行为验证。出现"更健壮""优化体验"这类不可验收的措辞时，必须重写。
  7. 按 docs-layout 写入文档，更新索引 README 和进度表。
- **Tool Strategy：**
  - 首选：code-intel 与读文件。
  - 禁止：
    - 修改源码；
    - 捏造模块名或接口名；
    - 写入 prewalk 或"执行模型假设"段落。
- **Decision Rules：**
  - **何时建持久化文档：** T2、跨会话或多人协作时建；否则给出内联简报。
  - **campaign 优先级：** 严格按 contract 中"严重度 → 优先级"的映射，不留"P0 或 P1"。
  - **需求冲突：** 记入决策日志，并向用户确认。
- **Verification：**
  - 每个任务都有验收命令；
  - 索引中的链接都能解析；
  - 状态词表合法。
- **Failure Recovery：**
  - 用户暂时无法回答：按标注的假设继续，并把假设列入文档顶部。
  - 文档结构与现有项目冲突：保留现有结构，只补缺失的索引。
- **Completion Criteria：**
  - 产物已写到约定路径；
  - 所有任务可验收；
  - 索引与进度表已更新。
- **References：**

  | 文件 | 预算 | 来源 |
  |---|---:|---|
  | `prd.md` | 0.6K | define 的模板 |
  | `brief.md` | 0.5K | clarify 的模板 |
  | `task-docs.md` | 4.0K | authoring + lifecycle + templates 合并，删除 prewalk 段与重复内容，并接收 execution-truth §7 |
  | `docs-layout.md` | 1.0K | docs-architecture |

### 6.6 lark-workflow-project-docs（迁出后的 arc:docs）

- 保留 `.lark.json` 索引契约、激活状态、全量 / 增量 / 单资源同步、task_base 约定。
- 删除 `evidence-checklist.md`，把 4 份飞书触发短语合并为 1 份。
- 改用飞书家族的 `lark-shared` 处理身份与安全规则，不再自带。
- 现有项目的 `.lark.json` 格式不变，可以继续使用。

### 6.7 `arc:code/scripts/gate.sh`（替代 6 个旧脚本）

```text
gate.sh start
  记录 HEAD、git status --porcelain、已有的未跟踪文件
  → .arc/artifacts/<task>/baseline.json

gate.sh check [--max-files N] [--strict]
  1. 变更集 = 当前 diff 与未跟踪文件 − 基线（排除用户先前的脏改动）
  2. 占位扫描（只扫变更行）：TODO / FIXME / panic("not implemented") / throw new Error("TODO")
  3. 压制扫描（只扫变更行）：//nolint、eslint-disable、@ts-ignore、# type: ignore、新增的 any、interface{}
  4. 格式检查（不修改文件）：gofmt -l、prettier --check、ruff format --check
  5. lint 与类型检查：识别仓库中的所有栈并逐一执行（不再用 elif 短路）
  6. 测试：只测受影响的包（go test ./<pkg>/...、vitest related）；--strict 时全量
  7. 范围：变更文件数 > N 时输出 WARN 和清单；只有存在范围锁且越界时才 FAIL
  输出：每项 PASS / FAIL / NOT_RUN(原因)；存在 FAIL 时退出码非 0
```

check-destructive 的模式黑名单不保留为脚本，规则写在 contract 第 6 条，由宿主的审批机制执行。

### 6.8 校验器与语料

- **标题：** `REQUIRED_HEADINGS` 对 `arc:*` 改为第 10 节的 10 个固定标题；其他家族暂时保持原样，避免一次打破 64 个技能。
- **根因门禁：** `ROOT_CAUSE_GATED_SKILLS` 改为 {arc:code, arc:verify, arc:audit}。
- **新增检查：**
  - Arc SKILL.md ≤3.5K tokens，单个 reference ≤3.5K；
  - Arc 的 description 必须含 "Not for" 子句；
  - 链接到 `_shared/` 的路径必须能解析；
  - 含糊措辞 lint（认真、仔细、酌情、视情况、as needed、when relevant、as appropriate）以警告形式提示。
- **`schemas/trigger_corpus.yaml`：**
  - 旧语料全部保留，并重新标注到新技能，作为回归集；
  - 每条语料必须恰好对应 1 个技能；
  - 新增相邻技能之间的近似负例：explore ↔ code、audit ↔ verify、plan ↔ code、verify ↔ code。
- **`schemas/behavior_corpus.yaml`：** 按新模式重写场景。例如：
  - repair 场景必须断言"有失败 → 通过证据"；
  - audit 场景必须断言"git status 无变化"。

---

## 7. 新 ARC 架构

```text
Arc/
├── README.md                          # 唯一路由表（5 行）+ 旧名 → 新名对照
├── _shared/                           # 无 SKILL.md，不参与发现；打包器按链接闭包分发
│   ├── contract.md                    # 权限、分级、范围锁、词表、产物、环境、飞书钩子、提交、汇报格式
│   ├── code-intel.md                  # arc-idx 工具合同 + 查询阶梯 + 影响面清单 + 降级
│   ├── code-rot.md                    # 36 项腐化分类（audit 的 rubric；code 引用写时门禁编号）
│   ├── api-contract.md                # Markdown 接口契约格式
│   ├── finding-campaign.md            # 发现 → 任务批次
│   └── conventions/
│       ├── go-backend.md              # 原 arch + backend-architecture + strict-type-safety
│       ├── frontend.md                # 原 frontend 的栈约定、UI 状态、脚手架
│       ├── frontend-tokens.md         # 原 color-tokens
│       └── comments.md                # 原 comment + modules
├── arc:explore/
│   ├── SKILL.md
│   └── references/architecture-map.md
├── arc:plan/
│   ├── SKILL.md
│   └── references/{prd.md, brief.md, task-docs.md, docs-layout.md}
├── arc:code/
│   ├── SKILL.md
│   ├── references/{repair.md, refactor.md, perf-probes.md}
│   └── scripts/gate.sh
├── arc:verify/
│   ├── SKILL.md
│   ├── references/{case-design.md, scope-cadence.md, stacks.md, txn-faults.md,
│   │               release-ops.md, worked-example.md, db/G00-G06.md … db/G26-G31.md}
│   └── scripts/dbqual/                # 原 arc:dbqual 脚本、schema、测试
└── arc:audit/
    ├── SKILL.md
    ├── references/{diff-review.md, appsec.md, scanners.md}
    └── scripts/{security-scan.py, install-security-tools.sh}

Lark/lark-workflow-project-docs/       # 原 arc:docs
docs/execution-truth.md                # 仓库级执行合同（Android/CNB/HarmonyOS/Arc 共用，路径不变）
docs/01-系统设计/prewalk.md             # 宿主侧设计记录（非运行时）
```

### 7.1 路由（Arc/README.md 的唯一路由表）

| 你要做的事 | 是否改源码 | 主技能 |
|---|---|---|
| 弄清楚代码在哪、谁调用、改动会波及什么 | 否 | arc:explore |
| 写需求、澄清、拆任务、维护任务文档 | 否（只写 docs） | arc:plan |
| 任何会修改源码的事 | 是 | arc:code |
| 用测试或运行结果证明行为、可靠性，或做发布判定 | 只改测试 | arc:verify |
| 只读地找问题：评审、体检、安全 | 否 | arc:audit |

平局规则：

- **"修复并测试"：** 用 arc:code，它自带随变更测试。只有当测试本身是交付物时才用 arc:verify。
- **"审计并修复"：** 分两个阶段。先用 arc:audit 出发现，再按发现交给 arc:code。同一阶段只有一个主技能。
- **"拆任务并实现"：** T2 先用 arc:plan；T0 / T1 由 arc:code 写内联简报。

### 7.2 依赖方向

单向依赖，没有技能加载技能：

```text
_shared/contract.md, _shared/code-intel.md   ← 全部 5 个技能
_shared/conventions/*                        ← code（写）、audit（评审）、explore（画图）、plan（约束）
_shared/code-rot.md                          ← audit（rubric）、code（写时门禁编号）
_shared/finding-campaign.md                  ← audit、plan
arc:verify/references/case-design.md         ← code（仅 T2 或测试设计较重时）
```

---

## 8. Before / After

### 8.1 数量与路由

| 维度 | Before | After |
|---|---|---|
| Arc 可发现技能 | 16 + 源包 1 + 仓库外 1 | 5 |
| 技能发现阶段的 description 总量 | 约 1.1K（16 条） | 约 0.9K（5 条，每条更完整） |
| 容易混淆的技能对 | build / fix、test / reliability、audit / security、define / clarify / sdlc、build / frontend、build / arch、fix / trace，共 7 组以上 | explore / code、audit / verify 两组，用"是否改源码""是否执行"两个问题判定 |
| 一个任务加载的技能 | 3–6 个，网状引用 96 条 | 1 个主技能 + 按需 reference |
| 路由描述 | 6 份（路由矩阵、cheatsheet、conductor、orchestration、skill-engineering、README），约 7.7K | `Arc/README.md` 中 1 张表，约 0.3K |

### 8.2 典型任务加载量

| 场景 | Before 加载内容 | Before | After 加载内容 | After（预算） | 变化 |
|---|---|---:|---|---:|---:|
| Go 后端缺陷修复 | fix + idx + arch + backend-architecture + test + execution-truth + ponytail | 17.3K | code + contract + code-intel + repair + go-backend + execution-truth | 10.9K | −37% |
| 全栈功能 | build + idx + arch + backend-architecture + frontend + test + sdlc + execution-truth（+ ponytail） | 21.5K（23.0K） | code + contract + code-intel + go-backend + frontend + api-contract + execution-truth | 13.1K | −39%～−43% |
| AppSec 审计 + 扫描 + 任务 | audit + security + idx + playbook + ranking + tooling + pipeline + execution-truth | 17.6K | audit + contract + code-intel + appsec + scanners + finding-campaign + execution-truth | 12.9K | −27% |
| 可靠性检查（涉及 PostgreSQL） | reliability + test + idx + 3 份 reliability references + test-case-design + execution-truth + arc:dbqual | 26.6K | verify + contract + code-intel + case-design + txn-faults + 1 组 db reference + execution-truth | 12.4K | −53% |

### 8.3 语料总量

| 部分 | Before | After（预算） |
|---|---:|---:|
| Arc SKILL.md | 33.0K | 约 14K（5 × 2.8K） |
| Arc references 与模块 | 60.5K | 约 25.7K（不含 `db/`；原 arc:dbqual 在仓库外，Before 也未计入） |
| `_shared/`（含 conventions） | — | 约 15.8K |
| Arc 专用 docs（路由 × 4、prewalk、code-rot） | 10.9K | 0（code-rot 已计入 `_shared`） |
| 合计 | 约 104K | 约 56K（−46%） |
| arc:docs 迁出部分 | 10.1K | 约 6K（计入飞书家族） |

### 8.4 能力得失

- **能力损失：**
  - 只删除了两项：prewalk 的中途换模型（宿主不支持，运行时无法执行）、论文证据清单（领域之外）。
  - 其余能力全部有去向，见附录 A。
- **能力增益：**
  1. 影响面 8 项清单（code-intel）；
  2. diff / PR 评审模式；
  3. 重构与迁移协议；
  4. 基线感知、不修改文件的完成门禁，且已接入完成判据；
  5. 每个技能的失败恢复阶梯与重试预算；
  6. 数据层写时门禁重新接回 arc:code；
  7. PostgreSQL 资格按需分组加载，并纳入仓库版本管理。

---

## 9. 升级顺序

每个阶段在同一个 PR 内"新增并删除"，避免新旧技能同时被发现。每个阶段结束时，以下检查都必须通过：

- `python3 scripts/validate_skills.py`
- `pytest`
- `scripts/sync_skills.py` dry-run 与 `--check`
- 旧触发语料全部路由到唯一的新技能

| 阶段 | 内容 | 依赖 | 退出条件 |
|---|---|---|---|
| **P0 统一合同与止血**（不改技能边界） | 新建 `_shared/contract.md`，各技能把重复段落替换为一行链接；统一词表与严重度映射；修复 1.6 中的第 3、4、5、7、9–14 条，包括移除 ponytail 前置、去掉精确版本串、把 Decision Tree 指针改为指向 contract；声明或移除 arc:dbqual 与 wxskills 依赖；删除 arc:prewalk，并清除各处 prewalk 段落；execution-truth 瘦身（§6 标题不变） | 无 | Arc SKILL.md 总量下降 ≥5K；上述 10 条全部消除（第 1、2、6、16 条在 P2 处理，第 8 条在 P1 处理，第 15 条在 P6 处理） |
| **P1 arc:explore** | 重写 arc:idx 为 arc:explore；新建 `_shared/code-intel.md`（影响面清单） | P0 | 旧 idx 语料 100% 路由到 explore；在一个 Go 后端仓库和一个前端仓库上试跑影响面清单 |
| **P2 arc:code** | 合并 build、fix、frontend、arch、comment、trace；规范下沉到 `_shared/conventions/`；新建 gate.sh 并删除 `Arc/scripts`、`Arc/templates`；`code-rot` 移入 `_shared`；核实前端栈的待核实事实 | P0、P1 | 6 个旧技能的语料路由到 code；repair 行为语料断言"失败 → 通过"；gate.sh 在 Go、Node、monorepo 三类仓库上通过自测 |
| **P3 arc:verify** | 合并 test、reliability；吸收源包；把 arc:dbqual 纳入仓库，并按 G 组拆分 references | P1；与 P2 的随变更测试规则对齐 | test 与 reliability 的语料路由到 verify；PostgreSQL 场景只加载 1 组 db reference；arc:dbqual 的自带测试在仓库内通过 |
| **P4 arc:audit** | 吸收 security；新增 review 模式；统一发现状态 | P1、P2（conventions 作为 rubric） | audit 与 security 的语料路由到 audit；在一个真实 diff 上试跑 review 模式 |
| **P5 arc:plan** | 合并 define、clarify、sdlc；pipeline 精简为 `_shared/finding-campaign.md` | P0（词表）、P4（发现格式） | 任务模板中没有 prewalk 段落；所有示例任务都有可复制的验收命令 |
| **P6 迁出与清理** | arc:docs 迁出为 lark-workflow-project-docs；删除 4 份路由文档；源包退出发现；更新校验器（标题集、门禁名单、新检查）、两份语料、`skills.index.json`、README、`Arc/README.md`、`docs/skill-catalog.md`、CHANGELOG（含旧名 → 新名对照）；重装并删除旧安装副本 | P1–P5 | 全量回归通过；宿主中只能发现 5 个 Arc 技能 |

P0 能单独带来收益：它消除 1.6 中 16 条矛盾里的 10 条，并减少约 5K 重复内容。即使后续阶段推迟，P0 也值得先做。P2 覆盖的任务最多，是收益最大的阶段。

---

## 10. 最终 SKILL 设计规范

### 10.1 什么时候新增一个 SKILL

以下 4 条必须同时满足：

1. 它的权限（只读 / 写文档 / 改源码 / 执行）或产出物类型，与所有现有技能都不同。
2. 它有至少 5 个步骤的独立工作流，不能表达为现有技能的一个模式。
3. 它的触发语料能和相邻技能分开：至少 8 个正例、3 个近似负例，路由准确率达到 100%。
4. 并入现有技能会让该技能的典型任务加载量增加 >1.5K，而这些内容对大多数任务没有用。

### 10.2 什么时候用模式、reference 或 conventions

- **模式（Mode）：** 权限与产出物相同，只是输入或步骤不同。例如 repair 与 implement。
- **reference：** 某个模式才需要的详细步骤或知识，归单个技能所有。
- **conventions（`_shared/`）：** "好代码长什么样"的团队规范，被 2 个以上技能读取。
- **contract（`_shared/`）：** 每个技能都适用的规则。只写一处，各处链接，禁止复制。

### 10.3 范围上限与最小独立价值

- 一个技能 = 一个权限等级 + 一类产出物，最多 6 个模式。
- 一个技能任意一次任务读取的 reference 不超过 3 个，共享文件除外。
- 某项内容并入其他技能后，加载量增加 <1.5K 且不造成触发混淆时，就不应成为独立技能。

### 10.4 Token 规则

- SKILL.md ≤3.5K tokens（约 300 行），单个 reference ≤3.5K 且只讲一个主题。
- description ≤120 个英文词，必须同时包含中英文触发词和 "Not for …（use X）" 子句。
- 只有一个禁令列表，放在 Tool Strategy 的"禁止"和 Decision Rules 中，不得在其他节复述。
- 运行时文件不写出处、"吸收自"、动机论证，这些写进 CHANGELOG 或 docs。
- 不解释模型本来就懂的概念，例如什么是 DTO。
- 删除测试：删掉某一句后，模型的行为不会改变，就删掉它。
- 示例只用于表达团队特有的风格，不用于演示通用写法。

### 10.5 SKILL.md 固定结构（10 个标题，按顺序）

| 标题 | 必须包含 | 不合格示例 |
|---|---|---|
| Trigger | 动词 + 对象的列表，中英文 | "需要帮助时使用" |
| Non-trigger | 每个相邻场景都写明去向技能 | 空，或只写"其他情况" |
| Goal | 一句话，含可观察的结果 | "高质量完成任务" |
| Inputs | 必填 / 可选、默认值、模式表 | 不写默认值 |
| Workflow | 编号步骤，每步都有可观察的产出 | "认真分析代码" |
| Tool Strategy | 首选、禁止、降级、查询顺序、查询失败时怎么办 | "合理使用工具" |
| Decision Rules | IF / THEN 加阈值 | "视情况决定" |
| Verification | 每条完成声明对应的证据类型 | "确保正确" |
| Failure Recovery | 每类失败的处理方式、重试预算、停止条件 | 不写 |
| Completion Criteria | 可勾选的清单 | "完成即可" |

### 10.6 触发、工作流、验证的原则

- **触发：**
  - 以用户会说的话为准，而不是技能的内部名称；
  - 相邻技能之间必须有近似负例；
  - 每条语料恰好路由到一个技能。
- **工作流：**
  - 每一步要么执行命令，要么产出一个具体工件；
  - 判断点写成 IF / THEN 加阈值；
  - 禁止"考虑""按需""适当""认真""仔细"这类词。
  
  可执行规则示例：
  - "修改函数前，按 code-intel 第 2 项定位全部调用方"；
  - "修改公共结构体前，执行第 3、4 项，覆盖全部构造和字段读写位置"；
  - "API 变化时，检查服务端、客户端、测试、文档、迁移"。
- **验证：**
  - 每条完成声明都要附工件：命令输出、`file:line` 或测试名；
  - 修复必须有"失败 → 通过"；
  - NOT_RUN 必须写原因；
  - 能用脚本检查的就用脚本（gate.sh），不用文字承诺。
- **低能力模型适配：**
  - 每个技能都要能在不读其他技能的情况下独立完成任务；
  - 失败时有明确的下一步，不会卡住；
  - 重试有上限，到上限后停下报告。

---

## 附录 A：能力保全清单

| 能力 | 原位置 | 新位置 |
|---|---|---|
| arc-idx 查询与索引生命周期 | arc:idx | `_shared/code-intel.md` + arc:explore（index-ops） |
| PRD | arc:define | arc:plan（prd） |
| 任务简报 | arc:clarify | arc:plan（brief） |
| docs/00–11 架构 | sdlc docs-architecture | `arc:plan/references/docs-layout.md` |
| 任务文档编写、生命周期、模板 | sdlc modules | `arc:plan/references/task-docs.md` |
| 安全发现 → 任务流水线 | sdlc security-audit-task-pipeline | `_shared/finding-campaign.md` |
| 功能实现 | arc:build | arc:code（implement） |
| 接口契约 | build api-contract | `_shared/api-contract.md` |
| 证据优先修复 | arc:fix + execution-truth §6 | `arc:code/references/repair.md`（§6 保留） |
| 前端栈约定、UI 状态、脚手架、色彩 token | arc:frontend | `_shared/conventions/frontend*.md` |
| 四项 UI 检查 | arc:frontend | `arc:verify/references/stacks.md` |
| Go 后端分层与 DIP | arch + backend-architecture + strict-type-safety | `_shared/conventions/go-backend.md` |
| 中文注释规范 | arc:comment + modules | `_shared/conventions/comments.md` + arc:code（comment） |
| 阶段计时探针 | arc:trace | `arc:code/references/perf-probes.md` |
| 用例设计、范围矩阵、技术栈工具 | arc:test references | arc:verify references |
| 业务不变量、事务与故障、发布证据、运维与事故学习 | arc:reliability references | arc:verify references |
| 工具获取、工业运营模型 | reliability-engineering-skill | `arc:verify/references/release-ops.md`（有行为价值的部分）；其余归档 |
| PostgreSQL 发布资格 G0–G31 | arc:dbqual（仓库外） | `arc:verify/references/db/` + `scripts/dbqual/` |
| 健康审计 36 项 | arc:audit + code-rot-taxonomy | `_shared/code-rot.md` + arc:audit（health） |
| AppSec Phase 0–6、数据价值排序 | audit playbook + security ranking | `arc:audit/references/appsec.md` |
| 扫描器、安装、DAST 授权 | arc:security | `arc:audit/references/scanners.md` + scripts |
| 飞书项目工作区 | arc:docs | `Lark/lark-workflow-project-docs` |
| 提交规范 | templates/git-commit-convention | `_shared/contract.md` |
| 学位论文证据清单 | arc:docs evidence-checklist | 删除（领域之外） |
| 中途换模型 | arc:prewalk | 删除（宿主不支持）；设计记录保留在 docs |
| 行业测试策略出处 | reliability industry-testing-strategies | 非运行时 docs |
| **新增：** 影响面 8 项清单 | — | `_shared/code-intel.md` |
| **新增：** diff / PR 评审 | — | arc:audit（review） |
| **新增：** 重构与迁移协议 | — | `arc:code/references/refactor.md` |
| **新增：** 基线感知完成门禁 | 孤立的 6 个脚本 | `arc:code/scripts/gate.sh` |
| **新增：** 失败恢复阶梯 | — | 每个 SKILL.md 的 Failure Recovery |
| **新增：** 数据层写时门禁 | 已经失效的指针 | arc:code Decision Rules → `_shared/code-rot.md` D、E 类 |

## 附录 B：执行本方案时与现有业务的冲突

本文只是方案，撰写时没有修改任何技能、脚本或校验器。执行时会产生以下冲突，需要在对应阶段处理：

1. **技能名消失。** 有 15 个名称会消失：arc:idx、define、clarify、sdlc、build、fix、frontend、arch、comment、trace、test、reliability、security、docs、prewalk。用户习惯以及调用 `/arc:build` 一类名称的自动化会失效。不提供别名，因为 T01 已经证明别名会造成重复发现。改为在 CHANGELOG 和 `Arc/README.md` 中给出旧名 → 新名对照，并用触发语料保证旧说法能路由到新技能。
2. **旧安装副本。** `~/.claude/skills/arc-*`（16 个，外加 `arc-dbqual` 和 `arc:dbqual`）和仓库内的 `.cursor/skills/` 都需要删除，否则宿主会同时发现新旧两套技能。
3. **仓库级合同。** Android、CNB、HarmonyOS 的 10 个技能链接 `docs/execution-truth.md`，所以它的路径和 §6 标题文字必须保持不变。
4. **校验器与测试。** `REQUIRED_HEADINGS`、`ROOT_CAUSE_GATED_SKILLS` 以及以下测试按旧名或旧标题断言，需要随阶段更新：
   - `tests/test_skill_validation.py`
   - `tests/test_triggers.py`
   - `tests/test_behavior_corpus.py`
   - T02 新增的 `tests/test_audit_regressions.py`
5. **语料。** `schemas/trigger_corpus.yaml` 和 `schemas/behavior_corpus.yaml` 以旧技能名为键。
6. **索引与说明文档。** `skills.index.json`、`README.md`、`Arc/README.md`、`docs/skill-catalog.md`、`CHANGELOG.md` 列出的都是 16 个技能。
7. **T02 基线。** T02 刚修订了全部 Arc 技能，并为 reliability-engineering-skill 新增了 3 份 references（domain-and-cadence、program-artifacts、program-workflow），这些文件尚未提交。本方案会吸收或归档这些新内容；T02 的 `remediation.csv` 中 Arc 部分将变为历史记录。
8. **用户项目里的旧任务文档。** 已生成的任务文档中带有 prewalk 段落。它们仍然有效，只是过时；arc:plan 需要兼容读取旧格式，不需要批量迁移。
9. **使用 `.lark.json` 的项目。** 索引格式不变，可以继续使用；但"arc:docs"这个触发名消失。
10. **arc:dbqual 的来源。** 它目前在仓库外，纳入仓库会改变它的真源位置，并需要一并带入 pyproject、requirements、测试与脚本。这一步需要所有者确认。
11. **门禁行为变化。** 范围检查从"失败"改为"告警"（越过范围锁时仍然失败）；`go fmt` 改为 `gofmt -l`，不再自动格式化。目前没有技能引用旧脚本，因此不影响现有流程。
12. **移除 ponytail。** arch 中"DIP 与 ponytail 冲突时如何取舍"的规则随之消失，简洁意图由 execution-truth §9 承接。如果团队仍希望遵循 ponytail，需要把它纳入仓库，并作为 conventions 的一部分。

## 附录 C：实施记录（2026-10-05）

本方案已按第 9 节执行。下表列出实施结果与正文设计不一致的地方，以实施为准。

| 正文设计 | 实施结果 | 原因 |
|---|---|---|
| SKILL.md 10 个固定标题（§10.5） | 11 个：在 Inputs 与 Workflow 之间加 Intent Router（表头 `Condition \| Mode \| Read`） | 多模式技能需要在读 reference 前确定模式；校验器强制顺序与非空 |
| arc:code 模式 implement / repair / refactor / comment / perf | implement / repair / refactor / frontend / comment / probe | 前端需要独立的状态清单；perf 只做计时探针，优化归 repair |
| arc:verify 模式 test / plan / reliability / db / release / perf | design / write / reliability / db / flaky / release | 负载与基准并入 write；flaky 分诊单列，有自己的重跑协议 |
| `conventions/go-backend.md` 一个文件 | 拆出 `conventions/go-type-safety.md` | 单文件超 3.5K token |
| arc:plan references 4 个 | 加 `task-templates.md` | 中文模板与规则同放超预算 |
| arc:verify `release-ops.md` 一个文件 | 拆出 `ops-learning.md` | 超预算 |
| `db/*.md` 按 5 组 | 8 组 `db/gates-*.md`（G0–G31 全覆盖）+ 入口 `db.md` | 每组控制在 2.5K 以内 |
| 审计交接包可写到 `docs/08-安全审计/` | arc:audit 只写 `.arc/artifacts/<slug>/security/handoff/`，arc:plan campaign 模式复制到 `docs/` | 与 arc:audit 只读权限一致 |
| 状态 `[-]` 为"放弃" | "放弃或延后"，必须写原因与重启条件 | 保留旧 sdlc 的延后语义 |
| arc:verify 不改生产源码 | 允许一次性 `git worktree` 做敏感性证明，用后删除 | 在坏版本上证明测试会失败，不碰工作区 |
| gate.sh 范围检查 | 超过 20 个文件告警；只有给出 `--scope` 时越界才失败 | 文件数不是缺陷；范围锁才是约束 |

实测结果：

- **数量：** 可发现入口从 18 个（16 个 Arc 技能 + 仓库外 arc:dbqual + reliability-engineering-skill）降为 5 个。
- **常驻成本：** Arc 描述合计约 1,100 token 降为 711 token（不含原 dbqual 与可靠性包的描述）。
- **入口正文：** SKILL.md 合计约 40.8K token（16 个 Arc 33.0K + dbqual 7.8K）降为 16.8K；单个均在 3.5K 以内（explore 3.1K、plan 3.3K、code 3.5K、verify 3.5K、audit 3.4K）。
- **总量：** Arc 相关文档约 127K token（Arc 94.8K + dbqual 入口 7.8K + 可靠性包 15.8K + 5 份路由/编排文档 9.1K）降为 95.3K，其中包含已吸收的 dbqual 资格门禁与可靠性规则。
- **校验：** 69 个运行入口校验通过；仓库测试 108 项全部通过（protobuf 测试使用 T02 的解释器）；69 个包 dry-run 与单包 dry-run 通过；安装后重新构建与实际安装逐文件比对一致。
- **未验证：** 宿主真实路由与 Agent 行为、DAST 与外部扫描器在真实目标上的运行、PostgreSQL 资格脚本连接真实数据库、shellcheck（未安装）。
