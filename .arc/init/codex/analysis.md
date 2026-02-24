# Skills 仓库工程分析（Codex 视角）

> 输入来源：
> - `/Users/iluwen/Documents/Code/Skills/.arc/init/context/project-snapshot.md`
> - `/Users/iluwen/Documents/Code/Skills/.arc/init/context/generation-plan.md`
> - 仓库实际文件扫描（simulate/triage/loop/review/deliberate/init/agent/refine）
>
> 分析时间：2026-02-24

## 全局结论

- 该仓库本质是 **Skill 文档 + Python 辅助脚本** 的单项目仓库。
- 可执行代码主要集中在 `simulate/`、`triage/`、`loop/`；其余目录以 `SKILL.md` 规范编排为主。
- 全仓库未发现 `Makefile`、`package.json`、`go.mod`、`pyproject.toml`、`.github/workflows` 等标准构建/CI 配置。
- 唯一显式第三方依赖清单是 `simulate/requirements.txt`，且全部未锁版本（存在可复现性风险）。
- Python 脚本广泛使用 `X | None` 类型注解，语法下限为 **Python 3.10+**；`__pycache__` 出现 `cpython-314`，说明至少在 Python 3.14 环境运行过。

---

## 1) `simulate/`

### 1. 技术栈详情

- 语言与形态：Python 脚本（6 个）+ Markdown 模板（`templates/`）+ 示例 JSON/JSONL。
- Python 版本：代码语法要求 `>=3.10`（`|` 联合类型）；`requirements` 中 `mdformat` 最新要求也为 `>=3.10`。
- 框架：无 Web 框架；定位为 CLI 工具链（脚手架、报告编译、质量门禁、格式化）。
- 关键库（来自 `simulate/requirements.txt`，未锁版本）：`mdformat`、`tabulate`、`py-markdown-table`、`pandas`。

### 2. 依赖清单与版本健康度

- 依赖声明方式：纯名称、无版本约束，存在依赖漂移风险。
- 截至 2026-02-24 的上游状态（PyPI）：
  - `mdformat`：`1.0.0`（2025-10-16，Requires Python >=3.10）
  - `tabulate`：`0.9.0`（2022-10-06，Requires Python >=3.7）
  - `py-markdown-table`：`1.3.0`（2025-02-06，Requires Python >=3.8）
  - `pandas`：`3.0.1`（2026-02-17，Requires Python >=3.11）
- 健康度结论：
  - 未见已废弃包的直接证据。
  - `tabulate` 更新节奏较慢（最后发布 2022），建议关注维护活跃度。
  - 若运行环境是 Python 3.10，`pandas` 将无法安装最新主版本（会回退到旧版），建议显式锁定兼容区间。

### 3. 构建系统与启动命令

- 无 Makefile / npm scripts / go build。
- 主要运行命令（脚本化）：
  - `python simulate/scripts/scaffold_run.py ...`
  - `python simulate/scripts/compile_report.py --run-dir <run_dir> ...`
  - `python simulate/scripts/check_artifacts.py --run-dir <run_dir> --strict`
  - `python simulate/scripts/beautify_md.py --run-dir <run_dir>`
  - `python simulate/scripts/new_defect.py ...`
  - `python simulate/scripts/accounts_to_personas.py --accounts-file <...>`

### 4. 测试策略

- 重点是 **E2E 产物校验**，不是单元测试。
- 核心验证脚本：`check_artifacts.py`（必需工件、JSONL 解析、截图引用、Markdown 表格结构）。
- 覆盖率：无 `coverage` 配置、无 pytest/unittest 测试目录，无法提供代码覆盖率。
- 类型分布：集成/流程验证（simulate 运行）占主导；无自动化单测/契约测试框架。

### 5. 编码规范

- 显式规范：强制 Markdown 表格合法性、截图路径规范、报告结构模板。
- 格式化工具：`mdformat`（可选，但在流程中被多处推荐/要求）。
- Lint/类型检查：未发现 `ruff/flake8/black/mypy` 等配置。
- CI 强制：未发现 CI pipeline 文件，规范主要靠本地脚本与流程约束执行。

### 6. 环境依赖

- 运行时：Python 3.10+（建议 3.11+ 以兼容 pandas 新版）。
- 外部服务/CLI：
  - `agent-browser`（浏览器自动化，核心依赖）
  - `docker`（可选，用于只读 DB 验证）
  - `ace-tool MCP`（前置上下文检索）
- 数据库：无内建数据库，仅读取目标系统数据库（经容器命令）。

---

## 2) `triage/`

### 1. 技术栈详情

- 语言与形态：Python 脚本（`triage_run.py`）+ triage 规则文档。
- Python 版本：语法要求 `>=3.10`（`Path | None` 等）。
- 框架：无框架，标准库实现（argparse/json/re/subprocess/dataclasses）。

### 2. 依赖清单与版本健康度

- 无独立 `requirements.txt`；第三方 Python 依赖为 0（仅标准库）。
- 依赖健康度较好（供应链面小），但功能依赖 `simulate` 产物格式稳定性。
- 外部工具依赖（非 Python 包）：`git`（脚本会读取 HEAD/status）、`simulate/scripts/*`（流程联动）。

### 3. 构建系统与启动命令

- 无构建系统。
- 核心命令：
  - `python triage/scripts/triage_run.py <run_dir>`
  - 输出扩展：`--md-out` / `--json-out`
- 流程内常联动：`simulate/scripts/check_artifacts.py`、`compile_report.py`、`new_defect.py`。

### 4. 测试策略

- 策略是“失败证据 -> 根因 -> 修复 -> 通过证据”的回归闭环。
- 无 triage 脚本自身单元测试/覆盖率配置。
- 测试类型偏 **故障分析与回归验证**，非传统自动化测试工程（pytest）。

### 5. 编码规范

- 强制 `DEBUG:` 日志输出、证据链完整、禁止绕过权限修复。
- Markdown 产物必须校验（推荐 `check_artifacts.py --strict` 或 `mdformat --check`）。
- 未发现静态检查与 CI 强制门禁配置。

### 6. 环境依赖

- 运行时：Python 3.10+。
- 外部依赖：`git` 命令、`simulate` 目录工件结构。
- 数据库/外部服务：不直接绑定数据库；若触发迁移需用户批准（流程约束层）。

---

## 3) `loop/`

### 1. 技术栈详情

- 语言与形态：Python 脚本 2 个（`uxloop_tmux.py` / `uxloop_cleanup.py`）+ JSON 配置 + runbook 文档。
- Python 版本：语法要求 `>=3.10`。
- 框架：无框架；使用 subprocess + tmux 命令编排。

### 2. 依赖清单与版本健康度

- 无 Python 第三方依赖（标准库实现）。
- 关键依赖转为系统工具：`tmux`（硬依赖）。
- 配置样例中服务启动命令是 `npm run dev`（意味着被测项目常见 Node.js 运行时依赖），但并非本目录自带依赖清单。

### 3. 构建系统与启动命令

- 本目录无 Makefile/package/go build。
- 启动命令：
  - `python loop/scripts/uxloop_tmux.py --config <cfg> --reset-window --wait-ready`
  - 清理命令：`python loop/scripts/uxloop_cleanup.py --session uxloop --window svc --kill-session`
- 服务构建/启动命令由配置文件 `assets/uxloop.config.example.json` 中 `services[].command` 注入（示例为 `npm run dev`）。

### 4. 测试策略

- 采用“重启服务 -> arc:simulate -> PASS/FAIL 判定 -> triage 修复”的迭代回归。
- 无 loop 脚本自身单测、无覆盖率文件。
- 测试类型偏系统联调与可运维性验证（ready_check: http/tcp/cmd）。

### 5. 编码规范

- 强制 tmux 使用约定、日志落盘、迭代上限、资源清理。
- Markdown 校验依赖 simulate 的检查脚本/`mdformat --check`。
- 无 lint/format 配置文件与 CI 约束。

### 6. 环境依赖

- 运行时：Python 3.10+。
- 外部服务/工具：`tmux`（必需）、网络端口探活能力、shell 执行权限。
- 被测系统依赖：按 `services` 配置动态决定（可为 Node/Go/Java 等）。

---

## 4) `review/`

### 1. 技术栈详情

- 语言与形态：纯 Markdown Skill（评审流程编排），无本地可执行脚本。
- 框架：七维度评审框架（ISO/IEC 25010 + TOGAF）以文档约定实现。

### 2. 依赖清单与版本健康度

- 依赖类型为工具链：`ace-tool MCP`（必需）、`Exa MCP`（推荐）、`codex CLI`（必需）、`gemini CLI`（必需）。
- 版本未锁定；无法在仓库内判断是否过时/废弃。
- 健康度风险点：CLI/MCP 演进可能导致流程指令漂移，建议后续增加版本基线文档。

### 3. 构建系统与启动命令

- 无构建系统。
- 执行入口是 skill 调用 `/arc:review`，内部通过 `Task` + `codex exec` + `gemini -p` 并发。

### 4. 测试策略

- 无自动化测试代码。
- 通过“三模型独立评估 + 交叉反驳 + 收敛报告”实现方法学上的质量控制。
- 无覆盖率指标。

### 5. 编码规范

- 强制：Markdown 表格对齐、结论必须给出 `file:line` 证据、只读评审。
- 无 linter/formatter/CI 配置落地到仓库。

### 6. 环境依赖

- 运行依赖：可访问目标项目代码、MCP 服务可用、Codex/Gemini CLI 可执行。
- 外部服务：Exa 网络检索（用于最佳实践/漏洞信息）。

---

## 5) `deliberate/`

### 1. 技术栈详情

- 语言与形态：纯 Markdown Skill，负责三模型审议与 OpenSpec 计划生成。
- 框架/方法：多轮歧义检查 + 交叉反驳 + OpenSpec spec-driven 计划。

### 2. 依赖清单与版本健康度

- 关键依赖：`ace-tool MCP`、`Exa MCP`、`codex CLI`、`gemini CLI`、`openspec CLI`。
- 仓库内无版本锁定与兼容矩阵，无法精确判断过时/废弃状态。
- 风险：OpenSpec/CLI 升级可能影响命令与产物结构（`proposal/specs/design/tasks`）。

### 3. 构建系统与启动命令

- 无传统构建系统。
- 关键命令链：
  - `openspec init --tools none`
  - `openspec new change <task-name>`
  - `openspec instructions <artifact> --change <task-name>`
  - `openspec validate/status/archive ...`
  - `codex exec ...` / `gemini -p ...`

### 4. 测试策略

- 非代码级测试，主要是流程正确性验证：
  - 审议收敛判定
  - OpenSpec `validate/status`
  - 三模型计划审查与互相反驳
- 无测试覆盖率配置。

### 5. 编码规范

- 规范集中在流程一致性、目录结构一致性、Markdown 产物质量。
- 无 lint/formatter/CI 文件。

### 6. 环境依赖

- 运行依赖：共享文件系统（`.arc/deliberate/...`）、OpenSpec CLI、Codex/Gemini CLI、MCP 搜索能力。
- 外部服务：Exa 网络检索（可选但推荐）。

---

## 6) `init/`

### 1. 技术栈详情

- 语言与形态：纯 Markdown Skill（层级 CLAUDE.md 生成规范 + 扫描启发式）。
- 目标能力：深度扫描、三模型分析、交叉审阅、文档生成、校验。

### 2. 依赖清单与版本健康度

- 依赖工具：`ace-tool MCP`、`Exa MCP`、`codex CLI`、`gemini CLI`。
- 无版本锁定，健康度难量化；主要风险来自外部工具协议变动。

### 3. 构建系统与启动命令

- 本目录无构建脚本。
- 流程中提取/引用外部项目的构建命令（Makefile/npm/go build），但该仓库自身不提供这些命令。
- 当前产物目录约定：`.arc/init/{context,claude,codex,gemini,summary.md}`。

### 4. 测试策略

- 无自动化测试框架。
- 质量控制依赖 Phase 5 校验：结构/表格/链接/内容一致性。
- 无 coverage 指标。

### 5. 编码规范

- 强制：只写 CLAUDE.md、证据驱动（版本来自 manifest）、面包屑和 mermaid 链接一致性。
- 无 lint/format/CI 配置文件。

### 6. 环境依赖

- 运行依赖：可读项目文件系统、MCP 搜索能力、Codex/Gemini CLI。
- 外部服务：Exa（standard/deep 模式推荐）。

---

## 7) `agent/`

### 1. 技术栈详情

- 语言与形态：纯 Markdown 元技能（调度层），不直接含可执行脚本。
- 功能：需求分析、skill 路由、模型分派、结果整合。

### 2. 依赖清单与版本健康度

- 依赖：`ace-tool MCP`、`Exa MCP`、`codex CLI`、`gemini CLI`、各 `arc:*` skill。
- 版本未锁，健康度依赖外部工具生态。

### 3. 构建系统与启动命令

- 无构建系统。
- 运行命令模板：`codex exec ...`、`gemini -p ...`、Claude `Task(...)`。
- 可选快照命令：`git stash` / `tar`（由文档流程定义）。

### 4. 测试策略

- 无代码测试框架。
- 通过流程控制保障可靠性：dry-run、confirm、snapshot、失败回滚、冲突裁决。
- 无覆盖率指标。

### 5. 编码规范

- 强制记录 `.arc/agent/dispatch-log.md`。
- 强调安全执行边界、批量变更确认、prompt 注入防护。
- 无 lint/CI 落地配置。

### 6. 环境依赖

- 运行依赖：MCP 服务、Codex/Gemini CLI、文件系统写权限。
- 可选依赖：Git 仓库环境（快照/回滚更完整）。

---

## 8) `refine/`

### 1. 技术栈详情

- 语言与形态：纯 Markdown Skill（问题细化器）。
- 主要机制：扫描 CLAUDE.md 索引 + AskUserQuestion 交互式澄清。

### 2. 依赖清单与版本健康度

- 无代码级第三方依赖。
- 工具依赖：`find`/`grep`/`read` 类操作能力与 AskUserQuestion。
- 不存在包版本过时问题，但依赖项目 CLAUDE.md 索引完整性。

### 3. 构建系统与启动命令

- 无构建系统。
- 示例命令：`find . -name "CLAUDE.md" -type f`。

### 4. 测试策略

- 无自动化测试框架。
- 结果质量靠“差距分析 + 1~4 个关键澄清问题 + 结构化 prompt 输出”保证。
- 无覆盖率指标。

### 5. 编码规范

- 规范重点在提问数量控制与上下文提取质量，非代码 lint。
- 无格式化/CI 配置。

### 6. 环境依赖

- 运行依赖：项目内已有 CLAUDE.md 层级索引；输出路径 `.arc/deliberate/<task-name>/context/enhanced-prompt.md` 可写。
- 无数据库或外部服务强依赖。

---

## 风险与改进建议（跨目录）

1. **依赖可复现性不足**：`simulate/requirements.txt` 未锁版本，建议引入最小/最大版本区间或 lock 文件。
2. **Python 版本基线不一致风险**：脚本最低看起来是 3.10+，但 `pandas` 最新要求 3.11+；建议统一声明 `Python >=3.11` 或给出双轨兼容策略。
3. **缺少自动化 CI 门禁**：建议补充最小 CI（脚本语法检查 + Markdown 表格检查 + 样例 run_dir 验证）。
4. **缺少静态规范工具**：可引入 `ruff` + `mdformat --check` + pre-commit，降低手工规范成本。
5. **外部 CLI 版本漂移**：为 `codex/gemini/openspec/tmux` 建议增加已验证版本基线文档。
