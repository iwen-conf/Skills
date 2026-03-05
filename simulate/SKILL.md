---
name: "arc:simulate"
description: "当需要按真实用户路径执行 E2E 验证并沉淀 UI 证据工件时使用。"
---
# **UI/UX Simulation & E2E Testing**

## Overview

本 Skill 赋予 Agent "高级自动化测试工程师" 的能力。Agent 将使用 agent-browser 二进制工具，模拟真实人类用户的思维逻辑与操作习惯，执行高保真的业务流闭环测试。

## Quick Contract

- **Trigger**：需要按真实用户路径验证端到端业务流并留存 UI 证据。
- **Inputs**：`test_objective`、`personas`、`target_url`、可选验证与输出参数。
- **Outputs**：`report.md`、`action-log`、`screenshot-manifest`、`screenshots/` 与账号文件。
- **Quality Gate**：结果宣告前必须通过 `## Quality Gates` 的工件完整性与格式检查。
- **Decision Tree**：输入信号路由图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree)。

## Routing Matrix

- 统一路由对照见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md)。
- 阶段化上手视图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view)。
- 单页速查见 [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md)。
- 若出现冲突，以本技能 `## When to Use` 的**边界提示**为准。

## Announce

开始时明确说明：  
“我正在使用 `arc:simulate`，将按真实用户路径执行 E2E 并沉淀可审计证据。”

## The Iron Law

```
NO E2E PASS CLAIM WITHOUT UI EVIDENCE AND ARTIFACT INTEGRITY
```

没有 UI 证据与工件完整性校验，不得宣称通过。

## Workflow

1. 获取测试目标、账号角色与运行参数并建档。
2. 按 UI 操作路径执行业务流并记录事件与截图。
3. 汇总报告、校验工件完整性与 Markdown 格式。
4. 输出 PASS/FAIL 证据，必要时回流 `arc:triage`。

## Quality Gates

- 每次运行必须产出 `run_id` 与标准化工件目录。
- 报告中的失败步骤必须能映射到截图/日志证据。
- 账号与会话切换必须符合会话隔离规则。
- 所有 Markdown 产物必须通过格式校验。

## Red Flags

- 通过 API/脚本绕过 UI 操作链路。
- 报告结论与截图/事件记录不一致。
- 缺少 `accounts.jsonc` 或关键工件仍继续交付。
- 在未授权情况下执行写操作数据库变更。

## When to Use

- **首选触发**：需要按真实用户路径做 E2E 验证并沉淀 UI 证据。
- **典型场景**：跨账号/跨角色流程、关键状态流转与截图取证。
- **边界提示**：失败根因定位与修复请转 `arc:triage`。

## Context Budget（避免 Request too large）

- 默认一次只跑/分析 **1 个** `run_id`（对应 **1 个** `run_dir`）。
- 不要把完整 `report.md` / `events.jsonl` / 大段日志粘贴到对话里；只提供：失败步骤表里 **FAIL 行** + 对应 `screenshots/...png` + 必要的少量关键信息。
- 如遇到 `Request too large`：把工作拆成多个小回合（例如：先确定要看的失败 step → 再补充该 step 的证据 → 再做修复/验证）。

## Helper Scripts Available

> 仿照 `webapp-testing/` 的模式：优先将 `scripts/` 当作黑盒直接调用；先跑 `--help` 再决定是否需要读源码。

- `scripts/scaffold_run.py`：一键生成 `<report_output_dir>/<run_id>/` 目录骨架 + 报告/工件模版（支持 pack：`e2e` / `full-process`）
- `scripts/compile_report.py`：从 `events.jsonl` 编译产出 `action-log.compiled.md` / `screenshot-manifest.compiled.md`；支持 `tabulate`/`py-markdown-table`/`pandas(df.to_markdown)` 等表格后端、列宽控制；可更新 `report.md` 的步骤表（auto markers）
- `scripts/new_defect.py`：生成 `failures/failure-XXXX.md`，并可追加到 `execution/defect-log.md`
- `scripts/check_artifacts.py`：质量门禁（必需文件/目录、截图引用存在性、accounts.jsonc 可解析、JSONL 可解析等）
- `scripts/beautify_md.py`：一键美化现有 Markdown（基于 `mdformat`，可对 run dir 全量格式化）
- `scripts/accounts_to_personas.py`：从 `accounts.jsonc` 生成 `personas` JSON（role/user/pass/token），便于重复跑回归

## Reference Files

- `templates/`：交付物模版与 packs（“测试公司级”全流程）
- `examples/`：脚手架、事件样例与用法示例
- `requirements.txt`：报告美化/表格工具依赖（mdformat/tabulate/py-markdown-table/pandas）
  - 默认不强制安装：不使用 `--beautify-md` 时可零依赖运行；建议只在需要时、并在 **venv** 里安装所需包，避免污染系统 Python。

## **Input Arguments (输入参数)**

当调用此 Skill 时，必须在上下文中明确以下参数：

1. **test_objective** (string, required)  
   * 描述：本次测试的宏观业务目标。  
   * 示例："验证从普通用户提交采购申请到经理审批通过的完整流程"  
2. **personas** (array, required)  
   * 描述：测试涉及的用户角色列表。必须包含**明文**的账号和密码，以便 Agent 模拟真实登录。  
   * 格式：[{"role": "buyer", "user": "u1", "pass": "p1"}, ...]  
3. **target_url** (string, required)  
   * 描述：测试环境的入口 URL。  
4. **validation_container** (string, optional)  
   * 描述：用于数据层验证的 Docker 容器名称（仅限只读操作）。
5. **run_id** (string, optional but recommended)  
   * 描述：本次执行的唯一标识，用于关联报告与工件目录。未提供时，Agent 必须自行生成。  
   * 推荐格式：`YYYY-MM-DD_HH-mm-ss_<short>`  
6. **report_output_dir** (string, optional)  
   * 描述：报告输出根目录（相对路径或绝对路径均可）。  
   * 默认：`reports/`  
7. **report_formats** (array, optional)  
   * 描述：报告输出格式。  
   * 默认：`["markdown"]`  
   * 可选：`"jsonl"` (每步一行 JSON，便于机器汇总)  
8. **capture_screenshots** (boolean, optional)  
   * 描述：是否强制截图作为证据工件。  
   * 默认：`true`
9. **accounts_file** (string, optional but recommended)  
   * 描述：统一账号管理文件路径（建议：`<report_output_dir>/<run_id>/accounts.jsonc`）。  
   * 约束：即使提供了 `personas`，也必须把最终使用的账号/密码同步写入该文件，并在 `report.md` 明文列出。
   * 辅助：可用 `python scripts/accounts_to_personas.py --accounts-file <...>` 生成 `personas` JSON 以便复跑。

## **Dependencies (环境依赖)**

* **ace-tool (MCP)**: 必须工具。用于在测试前读取项目源代码、API 定义和需求文档，以准确获取页面选择器(Selectors)和业务逻辑。  
* agent-browser: 用于浏览器自动化操作 (CLI)。  
* docker: 用于数据库只读验证 (CLI)。

## **Critical Rules (核心铁律)**

执行测试时必须严格遵守以下约束，违反即视为 Task Failed：

0. **Markdown Format Validation (Markdown 格式校验 - 最高优先级)**
   * **强制**: 生成任何 Markdown 文件（`report.md`、`screenshot-manifest.md`、`action-log.md`、`failures/*.md` 等）后，必须立即进行格式校验。
   * **表格列数对齐**: 表格的**表头行、分隔行、数据行**列数必须完全一致。例如表头 7 列，则分隔行和每行数据都必须是 7 列。
   * **分隔行格式**: 分隔行每列必须是 `---`、`:---`、`---:` 或 `:---:` 之一，不能为空或缺失。
   * **特殊字符转义**: 单元格内容若包含 `|` 字符，必须转义为 `\|`；若包含换行，必须用 `<br>` 替代或拆成多行。
   * **校验方法（任选其一）**:
     1. 使用 `scripts/check_artifacts.py --run-dir <run_dir> --strict`（推荐）
     2. 使用 `mdformat --check <file.md>`（需安装 mdformat）
     3. 手动检查：逐个表格数列数，确保表头、分隔行、所有数据行列数相同
   * **校验失败则修复后再继续**: 若发现格式错误，必须立即修复后重新校验，不得跳过。

1. **Human Simulation (拟人化)**  
   * **禁止**使用 curl/API 脚本绕过 UI。必须模拟点击 (click) 和输入 (type)。  
   * **思维链**: 操作前必须输出 "Observation (观察) -> Thought (思考) -> Action (行动)"。  
   * **会话隔离**: 切换账号前必须在 UI 上点击 "退出登录" (Logout)。  
2. **Read-Only Backend (后端只读)**  
   * **禁止**修改后端代码或配置文件。  
   * **禁止**通过 SQL INSERT/UPDATE/DELETE “手工伪造数据”来让测试通过。  
   * **允许**使用 SQL SELECT 验证 UI 操作结果。  
   * 若确需执行数据库迁移/DDL/DML（例如应用修复需要升级 schema 或执行官方 migration 脚本）：必须先获得用户明确同意，并把同意与执行证据写入 `<run_dir>/db/`；未获同意则视为阻塞，不得擅自执行。  
3. **Plain Text Logging (明文记录)**  
   * **强制**: 调试日志中的所有敏感信息（密码、Token、SessionID）**必须明文记录**，严禁脱敏，以便开发人员直接复现。

4. **Accounts File (统一账号管理文件必产出)**  
   * **强制**: 必须在 `<report_output_dir>/<run_id>/accounts.jsonc` 记录本次测试实际使用的账号/密码/Token。  
   * **强制**: 若为了验证修复必须创建新账号，必须在 `accounts.jsonc` 标记 `created_for_verification=true` 并写明原因与时间，同时在 `report.md` 的 `Account Changes` 章节说明。

5. **Report Artifacts (报告工件必产出)**  
   * **强制**: 每次业务流测试，无论 PASS/FAIL，必须生成一份可交付的测试结果文档（见 Phase 4）。  
   * **强制**: 报告中所有截图必须给出**准确路径 + 文件名 + 图片描述**（见 Screenshot Manifest）。

6. **Do Not Commit Secrets (凭证不可入库)**  
   * 由于本 Skill 规定明文记录账号/密码/Token，生成的 `reports/`（或你指定的输出目录）**不得提交到代码仓库**。
   * 已在本 Skill 目录的 `.gitignore` 默认忽略 `reports/` 等输出目录（但仍需团队约束与审查）。

7. **Resource Control & Cleanup (资源控制与及时关闭)**  
   * 所有等待/重试必须有上限（timeout / max attempts），禁止无限循环或无限等待。  
   * 禁止启动“跑完不退出”的后台进程（例如常驻 tail/监听器）而不在结束时关闭。  
   * 若使用了 tmux/容器/浏览器等外部工具：在本次 run 结束后及时退出/关闭（例如 detach + kill session），避免资源泄漏与长期占用。  

## **Instructions (执行流程)**

### **Phase 0: Context Acquisition (Requirement Analysis)**

**在开始任何测试之前，如果对页面结构、元素选择器(Selector)或具体业务流转逻辑不清晰，必须按以下优先级获取上下文：**

**优先级 0: 读取共享上下文索引（`.arc/context-hub/index.json`）**

1. 优先检索以下产物：
   * `codemap.md`（目录职责与关键路径）
   * `arc:review` 快照/诊断报告（已知风险点）
   * score 输出（由 `score/` 模块生成，高风险维度）
   * `arc:implement` handoff（本轮改动范围）
2. 验证产物新鲜度：`expires_at` + `content_hash`。
3. 产物可用则直接加载，不重复做全量扫描。
4. 产物失效则按 `refresh_skill` 回流更新（`arc:init:update` / `arc:cartography` / `score` 模块刷新（由 `arc:gate` 编排触发）/ `arc:review`）。

**优先级 1: 读取项目 CLAUDE.md 层级索引**

1. **扫描 CLAUDE.md**：使用 `find . -name "CLAUDE.md" -type f` 扫描项目的层级索引。
2. **提取关键信息**：
   * **根级 CLAUDE.md**：项目技术栈、运行命令、前端入口路径
   * **模块级 CLAUDE.md**（如 `frontend/CLAUDE.md`）：
     - "入口与启动" 章节：前端启动命令、开发服务器端口
     - "对外接口" 章节：页面路由、组件选择器模式、常用 data-testid 规范
     - "编码规范" 章节：选择器命名约定（如 `button[data-testid="{action}-{component}"]`）
     - "架构图" 章节：页面结构和组件层级
3. **验证索引新鲜度**：检查 CLAUDE.md 的 "变更记录" 章节，确认生成时间 < 7天。
4. **如索引缺失或过期**：触发 `arc:init:update`（必要时 `arc:init:full`）后再继续。

**优先级 2: 使用 ace-tool 补充细节**

当 CLAUDE.md 提供的信息不足时（如缺少具体选择器、页面逻辑不明确）：

1. **Call ace-tool**：读取项目代码库（特别是前端路由、组件定义）和需求分析文档。
2. **Identify Elements**：确认关键交互元素（按钮、输入框）的 ID、Class 或文本标识，避免盲目猜测选择器。
3. **Understand Logic**：理解业务的前后置条件（例如：订单状态流转规则），确保测试路径符合真实业务逻辑。

**优先级 3: 缓存验证与错误报告**

在测试执行过程中，如发现 CLAUDE.md 中的信息不准确（如选择器不存在、页面结构变更）：

1. **立即标记错误**：记录预期内容 vs 实际情况。
2. **回退到 ace-tool**：使用源码扫描获取正确信息，继续测试。
3. **生成错误报告**：在 `<run_dir>/context-errors/` 目录下生成缓存验证失败报告（见下方模板）。
4. **回流更新建议**：
   - CLAUDE 索引问题 → `arc:init:update`
   - codemap 问题 → `arc:cartography` 更新
   - 评分/评审产物问题 → `score` 模块刷新（由 `arc:gate` 编排触发）/ `arc:review` 更新

**缓存错误报告模板** (`<run_dir>/context-errors/cache-error-YYYYMMDD-HHMMSS.md`)：

```markdown
# 缓存验证失败报告

**生成时间**: <ISO 8601 timestamp>
**Run ID**: <run_id>
**测试步骤**: Step <step_number>

## 错误详情

- **缓存来源**: `<path-to-CLAUDE.md>`
- **章节**: <章节名称>
- **预期内容**:
  ```
  <从 CLAUDE.md 提取的选择器或信息>
  ```
- **实际情况**:
  - 选择器在页面中不存在 / 页面结构已变更
  - 实际选择器: `<actual-selector>`
  - 页面 URL: <current-url>
  - 发现时间: <ISO 8601 timestamp>

## 影响范围

- 当前测试：已回退到 ace-tool 扫描，测试继续执行
- 其他测试：可能影响所有依赖此选择器的测试用例

## 建议修复

1. **立即修复**（推荐）：
   ```bash
   arc init --project-path <project-path>
   ```

2. **手动修复**：
   编辑 `<path-to-CLAUDE.md>`，更新相关章节

## 临时补丁

已使用 ace-tool 获取正确信息：
```
<ace-tool 搜索结果摘要>
```
```

### **Phase 1: Strategy & Planning**

1. 基于 Phase 0 的分析，将 test_objective 拆解为有序的子任务队列 (Sub-tasks)。  
2. 规划完整的用户路径：Login A -> Action -> Logout -> Login B -> Verify。

3. 规划证据点 (Evidence Plan)：  
   * 必须截图的关键节点：登录后、关键提交按钮点击前后、出现 Toast/错误弹窗时、最终结果页。  
   * 必须落盘的验证：UI 验证文本/元素状态；必要时 DB SELECT 结果。

### **Phase 2: Execution Loop**

对每个子任务执行以下循环：

1. **Check**: 确认当前页面状态。  
2. **Action**: 使用 agent-browser 执行原子操作。  
   * 命令: agent-browser open|click|type|wait|screenshot ...  
3. **Wait**: 等待 UI 响应（Loading 结束、Toast 出现）。  
4. **Verify (UI)**: 检查页面文本或元素状态。  
5. **Capture Evidence (Mandatory if capture_screenshots=true)**:  
   * 关键节点必须截图，并且**立刻**记录：截图绝对/相对路径、文件名、图片描述、当前 URL、对应步骤号。

### **Phase 3: Deep Verification (Conditional)**

当 UI 反馈不明确或需要确认数据一致性时：

* 执行: docker exec -t <container> <db_cmd> -e "SELECT ..."  
* 验证数据库字段是否符合预期。

### **Phase 4: Report & Artifacts (Mandatory)**

每次业务流测试结束后（即使中途失败），必须生成测试报告与工件目录。

**输出根目录**：`<report_output_dir>/<run_id>/`  
若未提供 `run_id`，Agent 必须生成并在报告中显式打印。

**目录结构 (REQUIRED)**:

```text
<report_output_dir>/<run_id>/
  accounts.jsonc
  report.md
  action-log.md
  screenshot-manifest.md
  screenshots/
    s0001_login-page.png
    s0002_filled-form.png
  failures/
    failure-0001.md
  db/
    query-0001.txt
    result-0001.txt
  events.jsonl        (optional; when report_formats includes "jsonl")
```

> 备注：`db/` 目录建议始终存在（即使为空），以便 `scripts/check_artifacts.py --strict` 通过；当不做 DB 校验时可以保持空目录。

**账号管理（必需）**：

- 本次测试使用的账号/密码/Token 必须写入 `accounts.jsonc`（统一来源），并在 `report.md` 的 `Accounts & Secrets (PLAIN TEXT)` 章节明文列出。
- 若为了验证修复“必须创建新账号”（例如验证首登、权限边界、新租户隔离），必须：
  - 在 `accounts.jsonc` 对该账号标记 `created_for_verification=true` 并写明 `why/created_at`
  - 在 `report.md` 的 `Account Changes` 章节解释“为什么要产生新的账号”

**派生工件（由脚本生成，推荐保留）**：

- `scripts/scaffold_run.py`：创建 `<report_output_dir>/<run_id>/` 及 `screenshots/`、`failures/`、`db/`（可选创建 `events.jsonl`）
- `scripts/compile_report.py`：
  - 输出：`action-log.compiled.md`、`screenshot-manifest.compiled.md`
  - 报告：默认生成 `report.generated.md`；若使用 `--in-place` 则更新 `report.md` 的 auto blocks
- `scripts/beautify_md.py`：格式化 run_dir 下 Markdown（可选）

**路径规范（避免工具误判）**：

- 在 `report.md` / `screenshot-manifest.md` / `failures/*.md` 里引用截图时，路径必须使用相对路径 `screenshots/...` 且用反引号包裹，例如：`screenshots/s0007_after-submit.png`（`check_artifacts.py` 会解析并校验这些路径）。

**截图命名规则 (REQUIRED)**:

* 文件名必须可追溯到步骤号：`s<step_id>_<slug>.png`  
  - `step_id` 为 4 位数字，例如 `0007`  
  - `slug` 为简短英文/数字/连字符，例如 `after-submit`  
* 示例：`screenshots/s0007_after-submit.png`

**图片描述要求 (REQUIRED)**:

每张截图在 `screenshot-manifest.md` 中必须包含：  
`step_id` / `path` / `captured_at` / `url` / `description` / `expectation` / `result(PASS/FAIL)`。

## **Output Schema (日志规范)**

本 Skill 的输出分为两类：

1) **实时日志 (stdout)**：用于边跑边看。
2) **落盘报告 (artifacts)**：用于交付、回放、复现。**必须生成**。

以下给出标准化 Schema 与模板。

### **0. Run Report (Mandatory, report.md)**

`report.md` 必须包含以下章节（顺序建议固定，便于 diff 与机器解析）：

```markdown
# E2E UI/UX Simulation Report

## Run Metadata
* **Run ID**: `<run_id>`
* **Objective**: `<test_objective>`
* **Target URL**: `<target_url>`
* **Start Time**: `YYYY-MM-DD HH:MM:SS`
* **End Time**: `YYYY-MM-DD HH:MM:SS`
* **Result**: `PASS|FAIL`

## Personas & Secrets (PLAIN TEXT)
> 按铁律要求，以下信息必须明文记录，用于开发复现。
* **buyer**: user=`buyer_01` pass=`secret123` token=`<access_token>`
* **manager**: user=`manager_01` pass=`secret456` token=`<access_token>`

## Scenario Outline
1. Login buyer -> create request
2. Logout -> login manager -> approve
3. Verify final status

## Step-by-step Execution
> 每步必须具备：操作、预期、实际、结论、证据(截图路径)。

| Step | Role | Action | Expected | Actual | Evidence | Result |
|------|------|--------|----------|--------|----------|--------|
| 0001 | buyer | open `/login` | Login form visible | Login form visible | `screenshots/s0001_login-page.png` | PASS |
| 0002 | buyer | type username/password | Fields filled | Fields filled | `screenshots/s0002_filled-form.png` | PASS |
| 0003 | buyer | click `#btn-submit` | Redirect to dashboard | Redirected to dashboard | `screenshots/s0003_dashboard.png` | PASS |

## Screenshot Manifest
详见：`screenshot-manifest.md`

## DB Verification (Optional)
* Query: `db/query-0001.txt`
* Result: `db/result-0001.txt`

## Failure Summary (Only if FAIL)
* Primary failure: `<one-line>`
* See: `failures/failure-0001.md`
```

### **1. Action Log (Standard, action-log.md and/or stdout)**

```markdown
[ANALYSIS] 使用 ace-tool 读取 src/pages/Login.tsx，确认登录按钮 ID 为 #btn-submit-v2  
[PLAN] 切换至审批经理账号  
[THOUGHT] 当前已登出，需输入经理账号密码。  
[EXEC] agent-browser type "#username" "manager_01"  
[EXEC] agent-browser type "#password" "secret123"  
[VERIFY] 登录成功，Dashboard 显示 "待审批: 1" -> PASS
```

为保证可追溯性，建议在 Action Log 中引入步骤号与截图记录：

```markdown
[STEP 0007][THOUGHT] 准备点击提交按钮，预期出现成功 Toast 并跳转至列表页。
[STEP 0007][EXEC] agent-browser click "#btn-submit"
[STEP 0007][EXEC] agent-browser wait "text=提交成功" 5000
[STEP 0007][EXEC] agent-browser screenshot "<report_output_dir>/<run_id>/screenshots/s0007_after-submit.png"
[STEP 0007][VERIFY] Toast 出现且页面跳转至 /requests -> PASS
```

### **2. Screenshot Manifest (Mandatory, screenshot-manifest.md)**

截图清单必须包含**准确路径 + 文件名 + 图片描述**（并且与 `report.md` 引用一致）。

```markdown
# Screenshot Manifest

| Step | Path | Captured At | URL | Description | Expectation | Result |
|------|------|-------------|-----|-------------|-------------|--------|
| 0001 | `screenshots/s0001_login-page.png` | YYYY-MM-DD HH:MM:SS | `<url>` | 登录页初始状态：用户名/密码输入框可见 | 显示登录表单 | PASS |
| 0002 | `screenshots/s0002_filled-form.png` | YYYY-MM-DD HH:MM:SS | `<url>` | 已填充用户名/密码，未点击登录 | 字段内容正确 | PASS |
| 0007 | `screenshots/s0007_after-submit.png` | YYYY-MM-DD HH:MM:SS | `<url>` | 点击提交后出现成功 Toast，列表页第一行显示新记录 | Toast + 列表更新 | PASS |
```
### **3. Failure Report (Mandatory on Error, failures/failure-XXXX.md)**

若测试失败，必须输出以下 Markdown 块：
```markdown
# 🛑 E2E Test Failure Report

## Context  
* **Task**: [当前子任务名称]  
* **Time**: [YYYY-MM-DD HH:MM:SS]

## Debug Artifacts (PLAIN TEXT)  
* **User**: `[明文账号]` / `[明文密码]`  
* **Token**: `[明文 AccessToken]`  
* **URL**: `[当前 URL]`

## Screenshot Evidence
* **Step**: `0007`
* **Path**: `screenshots/s0007_after-submit.png`
* **Description**: 点击提交后出现错误弹窗 "权限不足"，页面未跳转

## Reproduction Steps  
1. [操作步骤 1]  
2. [操作步骤 2]  
3. [导致错误的操作]

## Evidence  
* **UI State**: [报错信息或截图描述]  
* **DB State**:   
  > Query: `SELECT status FROM orders ...`  
  > Result: `[数据库返回结果]`
```

### **4. Optional: events.jsonl (Machine-readable)**

当 `report_formats` 包含 `"jsonl"` 时，每个步骤必须追加一行 JSON，至少包含这些字段：

```json
{"run_id":"<run_id>","step":1,"role":"buyer","kind":"exec","cmd":"agent-browser open \"<target_url>/login\"","ts":"YYYY-MM-DDTHH:MM:SS","result":"PASS"}
{"run_id":"<run_id>","step":1,"role":"buyer","kind":"screenshot","path":"screenshots/s0001_login-page.png","description":"登录页初始状态","ts":"YYYY-MM-DDTHH:MM:SS"}
```
## Anti-Patterns

**CRITICAL: The following behaviors are FORBIDDEN in arc:simulate execution:**

### Test Execution Anti-Patterns

- **Screenshot Skipping**: Failing to capture screenshots at each step — reports are incomplete without visual evidence
- **Selector Guessing**: Using guessed selectors without verification — causes flaky tests
- **Happy Path Only**: Testing only success scenarios — must include error cases and edge conditions
- **State Blindness**: Ignoring page state before actions — causes cascade failures

### Evidence Anti-Patterns

- **Missing Manifest**: Not updating `screenshot-manifest.compiled.md` — breaks report traceability
- **Broken References**: Screenshot filenames not matching manifest entries — orphaned evidence
- **JSONL Corruption**: Malformed `events.jsonl` entries — breaks compilation

### Report Anti-Patterns

- **Preliminary Conclusion**: Marking test PASS before `check_artifacts.py --strict` validation — premature success declaration
- **Failure Suppression**: Skipping failed steps instead of recording them — hides real issues
- **Context Ignorance**: Not reading CLAUDE.md for expected behavior — tests wrong things
