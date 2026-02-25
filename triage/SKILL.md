---
name: arc:triage
description: 读取 arc:simulate 的失败报告并定位/排查/修复缺陷（E2E/UI 自动化测试失败、回归失败、复现不稳定等），可用 triage_run.py 快速汇总 run_dir；强制输出大量 DEBUG 日志，禁止通过改代码跳过/短路权限(鉴权/授权)验证来让业务流”通过”，并在修复后给出可交付的 Fix Packet 与验证证据。
---

# UI/UX 缺陷排查与修复（基于 arc:simulate，工业化）

## Overview

将 arc:simulate 产出的失败工件（reports/run_id 下的 report、events、screenshots 等）转化为：最小复现 → 根因定位 → 代码修复 → 回归通过证据。

## Context Budget（必须拆分，避免上下文过长）

- 默认一次只 triage **1 个** `run_dir`；深入分析最多聚焦 **1~2 个** FAIL step。
- 证据优先落盘到 `<run_dir>/analysis/`，在对话中只引用路径与关键结论；不要粘贴整份报告/日志。
- 如遇到 `Request too large`：先生成 `analysis/triage.md`（概览）→ 再按 step 逐个处理（每次只看最小必要片段）。

## Definition of Done（最终必须交付）

最终输出必须包含（建议直接按 `references/fix-packet.template.md` 组织）：

- **Failing evidence**：至少 1 个失败 run 的 `run_id/run_dir` + 关键截图/日志路径
- **Passing evidence**：至少 1 个通过 run 的 `run_id/run_dir` + 关键截图路径
- **Root Cause**：具体到模块/条件/选择器/并发/权限策略
- **Fix**：涉及的文件/关键逻辑与为什么这么改
- **Verification**：如何复现失败 + 如何跑回归（含命令/参数）
- **Guardrails**：明确说明未通过短路/删除鉴权授权来“修复”

## Inputs（调用方需要提供/或由你生成）

优先从失败工件开始工作：

- `run_dir`：例如 `reports/2026-02-01_14-00-00_abcd/`
- 若没有 `run_dir`，则需要 arc:simulate 的核心参数：`test_objective`、`personas`、`target_url`（以及可选 `validation_container`）

## Quick Triage（推荐先跑）

先用脚本做一次 best-effort 汇总（不替代人工分析，但能加速定位）：

```bash
python triage/scripts/triage_run.py <run_dir>
```

分流决策树（用于快速判断“产品缺陷 vs 测试误报 vs 环境/数据/flake”）：见 `references/triage-decision-tree.md`。

## Artifacts & Paths（文档/文件放哪）

所有证据以 arc:simulate 的 `run_dir` 为根目录：

- **Failing run**：`reports/<fail_run_id>/`
- **Passing run**：`reports/<pass_run_id>/`
- **Accounts file（统一账号管理）**：`<run_dir>/accounts.jsonc`（明文账号/密码/Token；不得提交入库）

建议把“分析/修复交付件”放到 `run_dir/analysis/`（不影响 `check_artifacts.py --strict`）：

- `<run_dir>/analysis/triage.md` / `<run_dir>/analysis/triage.json`（由 `triage_run.py` 生成）
- `<run_dir>/analysis/fix-packet.md`（按 `references/fix-packet.template.md` 填写，引用 fail+pass 两个 run）

如涉及数据库迁移/DDL/DML（必须先获得用户同意），建议把变更控制证据放到 `<run_dir>/db/`：

- `<run_dir>/db/migration-approval.md`（粘贴用户同意原文/截图路径）
- `<run_dir>/db/migration-plan.md`（变更内容、影响范围、回滚策略）
- `<run_dir>/db/migration-execution.md`（实际执行命令与输出摘要）

示例（将 triage 落盘到 run_dir）：

```bash
mkdir -p <run_dir>/analysis
python triage/scripts/triage_run.py <run_dir> \
  --md-out <run_dir>/analysis/triage.md \
  --json-out <run_dir>/analysis/triage.json
```

## 核心铁律（必须遵守）

0. **Markdown 格式校验（最高优先级）**
   - **强制**: 生成或修改任何 Markdown 文件后，必须立即校验格式。
   - **表格列数对齐**: 表头行、分隔行、数据行列数必须完全一致（例如表头 7 列，则每行都是 7 列）。
   - **分隔行格式**: 每列必须是 `---`、`:---`、`---:` 或 `:---:`，不能为空或缺失。
   - **特殊字符转义**: 单元格内含 `|` 必须转义为 `\|`；含换行用 `<br>` 替代。
   - **校验方法**:
     1. `python simulate/scripts/check_artifacts.py --run-dir <run_dir> --strict`（推荐）
     2. `mdformat --check <file.md>`（需安装 mdformat）
     3. 手动逐表格数列数
   - **校验失败必须修复后再继续**，不得跳过。

1. **禁止绕过权限(鉴权/授权)校验来“修复”**
   - 这里的“权限”指**业务权限/鉴权/授权**（不是文件系统 chmod/chown）。
   - 禁止在代码里注释/删除/短路权限校验（例如把“无权限应失败”改成直接返回成功，或把必须角色/ACL 的判断移除）。
   - 若问题确为权限规则/角色映射/策略配置的缺陷：修到**符合业务预期**，并补回归覆盖“应允许/应拒绝”两侧边界。

2. **大量 DEBUG 输出（强制）**
   - 在排查过程中，必须在终端输出大量以 `DEBUG:` 开头的日志，覆盖：输入、关键分支、关键变量、外部请求/响应摘要、异常堆栈。
   - 在代码中优先添加 **可控** 的 DEBUG 日志（推荐用环境变量/配置开关，例如 `DEBUG_UI_UX_FIX=1`），避免永久污染正常日志。

3. **以证据驱动**
   - 不允许“感觉修了”：每次修复都要给出证据（至少一次复现失败 run + 一次回归通过 run 的 run_id/报告路径）。

4. **遵守 arc:simulate 的日志与工件规范**
   - 如需生成/更新报告，优先使用 `simulate/scripts/` 下的脚本，不要自创格式。

5. **DB Migration / DDL / DML 变更控制**
   - 任何数据库迁移/DDL/DML（包括但不限于 migrate、ALTER、INSERT/UPDATE/DELETE、回填数据）都必须先获得用户明确同意。
   - 同意前：只能做只读验证（SELECT）与代码排查；不得“先迁移再补票”。
   - 同意后：把同意与执行证据写入 `run_dir/db/`（见上方建议文件），并在最终 Fix Packet 里明确说明。

6. **新增账号（用于验证修复）必须可审计**
   - 若为了验证修复必须创建新账号（例如验证首登、权限边界、新租户隔离），必须写入：
     - `accounts.jsonc`：对该账号标记 `created_for_verification=true` 并写明 `why/created_at`
     - `report.md`：`Account Changes` 章节解释“为什么要产生新的账号”

7. **Resource Control & Cleanup（资源控制与及时关闭）**
   - 禁止跑“无上限”的重试/回归循环；必须有 `max_iterations` / timeout。
   - 若为了排查启动了额外工具（浏览器录制、长时间 tail、临时容器、tmux session 等），在拿到证据后及时关闭，避免资源泄漏与长期占用。

## Workflow（推荐按顺序执行）

### 0) 若缺少 run_dir：先跑一轮 arc:simulate 产出失败工件

- 用脚手架创建目录骨架（建议 `--pack full-process`，至少隐式包含 `e2e`）：
  - `python simulate/scripts/scaffold_run.py --help`
  - `python simulate/scripts/scaffold_run.py --pack full-process --objective "<objective>" --target-url "<url>" --personas "<json-or-path>"`
- 按 `simulate/SKILL.md` 执行测试，确保落盘：
  - `report.md`、`action-log.md`、`screenshot-manifest.md`、`screenshots/`
  -（可选）`events.jsonl`
- 失败后，把产出的 `run_dir` 作为本 Skill 的输入继续后续步骤。

### 1) 获取失败证据并编译报告

- 若已存在 `run_dir`：先校验与编译
  - `python simulate/scripts/check_artifacts.py --run-dir <run_dir> --strict`
  - `python simulate/scripts/compile_report.py --run-dir <run_dir> --in-place`（可选：若已在 venv 中安装 `mdformat` 再加 `--beautify-md`）
- 从以下文件提取“最小可复现信息”：
  - `report.md`（失败步骤表）
  - `failures/*.md`（缺陷描述）
  - `action-log.md` / `events.jsonl`（操作轨迹）
  - `screenshots/`（视觉证据）

### 2) 复现失败（缩小到最小步骤）

- 目标：把失败收敛成“从哪个页面、点了什么、期望什么、实际什么、哪里开始偏离”。
- 输出 `DEBUG:` 日志（至少包含）：
  - 复现入口 URL、账号角色、步骤号 step_id、页面 URL
  - 关键 DOM/接口请求的观测结果（例如 toast 文案、HTTP 状态码、关键字段）


**在开始诊断前，按以下优先级获取上下文：**

**优先级 1: 检查 `.arc/review/` 架构分析**
1. 查找评审产物：检查 `.arc/review/<project-name>/` 是否存在架构分析文档
2. 验证新鲜度：检查 `project-snapshot.md` 或 `diagnosis-report.md` 的生成时间 < 7天
3. 提取关键信息：模块依赖关系、常见缺陷模式、修复策略建议
4. 如评审产物不存在或过期：提示用户运行 `/arc:review` 生成项目评审后再继续

**优先级 2: 检查 `.arc/triage/known-patterns.md`**
1. 查找历史缺陷模式：读取 `.arc/triage/known-patterns.md`（如存在）
2. 匹配当前失败：对比当前失败症状与已知模式
3. 复用修复策略：如匹配成功，直接应用已验证的修复方案

**优先级 3: 使用 ace-tool 搜索源码**
当缓存信息不足时，使用 ace-tool 搜索项目源码定位问题根因。

**优先级 4: 缓存验证与错误报告**
在诊断过程中，如发现 `.arc/review/` 或 CLAUDE.md 中的信息不准确：
1. 标记缓存错误：记录预期内容 vs 实际情况
2. 回退到源码搜索：使用 ace-tool 获取正确信息
3. 生成错误报告：在 `<run_dir>/analysis/context-errors/` 生成缓存验证失败报告
4. 提示用户：建议运行 `/arc:review` 或 `/arc:init` 更新项目索引

**优先级 5: 更新已知缺陷模式库**
在成功修复缺陷后，将新发现的缺陷模式追加到 `.arc/triage/known-patterns.md`。

---

### 3) 分流诊断（优先排除“测试误报”）

按优先级快速判断：

- **选择器/文案变更**：元素不存在/role 变化/文本改变
- **时序问题**：加载慢、等待条件不对、race condition
- **权限/鉴权**：登录态丢失、token 过期、跨账号未 logout
- **后端异常**：500/超时/数据不一致
- **环境问题**：服务未启动、端口冲突、数据库不可达

若判断为测试脚本/选择器误报：先修正 arc:simulate 的步骤与选择器，再回归一次，确认真实缺陷是否仍存在。

### 4) 加强可观测性（在代码里加 DEBUG）

- 先加“观测点”，再动“业务逻辑”。
- 建议统一打点格式，便于 grep：
  - 后端：`DEBUG_UI_UX_FIX` + `run_id` + `step_id` + `request_id`
  - 前端：`console.debug('[DEBUG_UI_UX_FIX]', { runId, stepId, ... })`
- 记录“输入→中间态→输出”，避免只打“到了这里”。

### 5) 修复（小步提交、避免破坏面）

- 优先修根因：状态机、校验、边界条件、并发、选择器稳定性等。
- 禁止用权限修改掩盖问题（见铁律 1）。
- 每次改动后都跑最小复现路径；确认失败消失后再跑完整回归。

### 5.1) UI 相关修复的工程化建议（常见工业级做法）

- 优先引入/使用稳定选择器：`data-testid`、ARIA role + label；避免依赖易变文案与复杂 CSS。
- 避免 `sleep` 式等待：使用“元素出现/可点击/请求完成/Toast 出现”等确定性条件。
- 对 Flake 要求证据：同 commit 下至少重复跑 2~3 次仍稳定通过，才判定“已修复”。

### 6) 回归验证并产出可交付修复方案

- 重新执行 arc:simulate，生成新的 `run_dir`，并同样：
  - `python simulate/scripts/check_artifacts.py --run-dir <run_dir> --strict`
  - `python simulate/scripts/compile_report.py --run-dir <run_dir> --in-place`（可选：若已在 venv 中安装 `mdformat` 再加 `--beautify-md`）
- 在最终输出中给出：
  - **Root Cause（根因）**
  - **Fix（修复点）**：涉及文件/模块/关键逻辑
  - **Verification（验证证据）**：失败 run_id + 通过 run_id + 报告路径
  - **Risk & Follow-ups（风险与后续）**：可能的回归点、需要补的单测/监控

> 最终交付模板：见 `references/fix-packet.template.md`。

## 缺陷文件（可选但推荐）

当定位到明确失败步骤时，生成缺陷文件，便于跟踪：

```bash
python simulate/scripts/new_defect.py \
  --run-dir <run_dir> \
  --step 0007 \
  --title "点击提交后出现 500" \
  --role buyer \
  --url "http://localhost:5173/order/new" \
  --user "buyer_01" \
  --password "secret123" \
  --screenshot "screenshots/s0007_after-submit.png" \
  --severity S1
```

注意：arc:simulate 允许明文记录账号/密码；因此 `reports/` 不要提交到仓库。
