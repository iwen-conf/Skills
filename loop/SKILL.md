---
name: "arc:loop"
description: "修复后必须重启服务时使用：执行失败-修复-回归的多轮闭环并沉淀证据。"
---

# tmux 启动 + arc:simulate 回归闭环（工业化）

## Overview

用于“修复后必须重启服务才能生效”的项目：把服务启动/重启、日志取证、回归测试与失败再修复串成可重复闭环，并且每轮都有可交付证据。

## Quick Contract

- **Trigger**：修复后需要重启服务并进行多轮回归闭环验证。
- **Inputs**：tmux 服务配置、`arc:simulate` 参数、`max_iterations`。
- **Outputs**：每轮 `run_id`、服务日志路径、测试结果与失败分流信息。
- **Quality Gate**：收敛前必须通过 `## Quality Gates` 的证据链与迭代控制检查。
- **Decision Tree**：输入信号路由图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree)。

## Routing Matrix

- 统一路由对照见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md)。
- 阶段化上手视图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view)。
- 单页速查见 [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md)。
- 若出现冲突，以本技能 `## When to Use` 的**边界提示**为准。

## Announce

开始时明确说明：  
“我正在使用 `arc:loop`，先拉起服务并建立回归闭环，再逐轮验证。”

## The Iron Law

```
NO RETEST LOOP WITHOUT SERVICE RESTART AND TRACEABLE EVIDENCE
```

没有重启确认与证据链，不得宣称回归结果有效。

## Workflow

1. 准备 `tmux` 启动配置并记录基线状态。
2. 启动/重启服务并落盘日志。
3. 执行 `arc:simulate` 产出本轮测试工件。
4. FAIL 则进入 `arc:triage` 修复，PASS 则收敛交付。

## Quality Gates

- 每轮必须输出 `run_id`、`run_dir`、日志路径与 git 状态。
- 服务日志与测试报告必须可按同一轮次关联。
- FAIL 必须附根因与修复证据，禁止“感觉修好”。
- 达到迭代上限必须停止并输出下一步建议。

## Red Flags

- 修复后不重启就直接复测。
- 报告通过但找不到对应日志与 run 证据。
- 无限循环回归无 `max_iterations` 上限。
- 通过绕过鉴权/权限逻辑来“制造 PASS”。

## When to Use

- **首选触发**：修复需要重启服务，并要执行多轮 fail→fix→retest 闭环。
- **典型场景**：需要统一沉淀 run_id、日志、报告三位一体证据链。
- **边界提示**：仅做单次失败根因定位可直接用 `arc:triage`。

## Context Budget（避免 Request too large）

- 服务日志以落盘为准（`logs/uxloop/<run_id>/...`）；不要把整段日志粘贴到对话里，必要时只摘录“失败时间点附近的少量行”并注明来源文件路径。
- 与测试报告统一使用同一个 `run_id`，让证据链只靠路径对齐（少说话、多给路径）。

## Definition of Done（每轮必须产出）

每轮迭代在 stdout 明确输出（便于粘贴进 ticket/PR/日报）：

- `iter`: 01..N
- `run_id` / `run_dir`（arc:simulate 工件目录）
- `result`: `PASS|FAIL` + 一句话失败类型（selector/timing/auth/backend/env/data/unknown）
- `git`: `HEAD` + 是否 dirty（`git status --porcelain` 是否为空）
- `tmux`: `session_name` / `window_name` / 每个服务对应的 pane
- `logs`: `logs_dir` + 每个服务的 log_file

并且：

- FAIL → 必须交给 `arc:triage` 输出根因与修复证据（禁止“感觉修了”）
- PASS → 必须给出通过 run 的 `run_id` + `report.md` 路径

## Artifacts & Paths（文档/文件放哪）

推荐**每轮迭代使用同一个 `run_id`** 贯穿“服务日志 + 测试报告”，这样证据可直接对齐：

- **服务日志（本 Skill / tmux）**：默认 `logs/uxloop/<run_id>/<service>.log`（可用 `uxloop_tmux.py --logs-dir ...` 覆盖）。
- **测试报告（arc:simulate）**：默认 `reports/<run_id>/`（可用 `scaffold_run.py --report-output-dir ...` 覆盖）。
- **账号管理（arc:simulate）**：`reports/<run_id>/accounts.jsonc`（明文账号/密码/Token；不得提交入库）。

示例（同一个 `run_id` 打通 logs+reports）：

```bash
run_id="2026-02-01_14-00-00_abcd_iter01"
python loop/scripts/uxloop_tmux.py --config uxloop.config.json --run-id "$run_id" --reset-window --wait-ready
# 然后用同一个 run_id 跑 arc:simulate，产出 reports/$run_id/
```

## Inputs（调用方需要提供）

### A) 服务启动配置（推荐）

优先用 JSON config（可复用、可 review）：

- 参考：`assets/uxloop.config.example.json`
- 字段：
  - `session_name`（默认 `uxloop`）
  - `window_name`（默认 `svc`）
  - `services[]`：
    - `name`（用于 pane title 与日志文件名）
    - `cwd`（可选）
    - `command`
    - `env`（可选，键值均为 string）
    - `ready_check`（可选）：`http|tcp|cmd`

### B) arc:simulate 参数

- `target_url`
- `test_objective`
- `personas`
-（可选）`validation_container`

### C) 迭代控制

- `max_iterations`（默认 5）

## Quick Start（推荐脚本化）

1) 准备配置：复制 `assets/uxloop.config.example.json` 到项目里（例如 `uxloop.config.json`）并按项目改 `cwd/command/ready_check`。

2) 启动/重启服务 + 落日志（每次修复后都重复执行一次，确保服务与前端 bundle 都是最新的）：

```bash
python loop/scripts/uxloop_tmux.py \
  --config uxloop.config.json \
  --reset-window \
  --wait-ready
```

3) 进入迭代回归（见下方 Loop Workflow）。

> 高级 tmux 操作与取证：见 `references/tmux-runbook.md`。

## Markdown 格式校验（强制）

生成或修改任何 Markdown 文件后，必须立即校验格式：

- **表格列数对齐**: 表头行、分隔行、数据行列数必须完全一致（例如表头 7 列，则每行都是 7 列）。
- **分隔行格式**: 每列必须是 `---`、`:---`、`---:` 或 `:---:`，不能为空或缺失。
- **特殊字符转义**: 单元格内含 `|` 必须转义为 `\|`；含换行用 `<br>` 替代。
- **校验方法**:
  1. `python simulate/scripts/check_artifacts.py --run-dir <run_dir> --strict`（推荐）
  2. `mdformat --check <file.md>`（需安装 mdformat）
  3. 手动逐表格数列数
- **校验失败必须修复后再继续**，不得跳过。

## tmux 约定（强制）

- 同一个闭环使用同一个 session（默认 `uxloop`），避免散落多个 session 难以排查。
- 每个服务一个 pane，且必须可见 stdout/stderr。
- 必须落盘日志（推荐 `logs/uxloop/<loop_run_id>/...`），否则 FAIL 时证据链断裂。
- 重启策略：
  - 优先在 pane 内 `Ctrl-C` 停止，再重新执行启动命令
  - 若进程僵死：定位 PID 精准 kill；最后手段才 `tmux kill-session`

## Loop Workflow（直到 PASS）

### 0) 预检（每次开始前）

- 确认 `tmux` 可用：`tmux -V`
- 若不确定启动方式：先读项目启动脚本（package scripts / Makefile / compose）
- 清理冲突端口（只停止相关进程；不要通过改代码绕过权限/鉴权逻辑来“解决”）
- 记录 git 状态：
  - `git rev-parse HEAD`
  - `git status --porcelain`

### 1) 启动/重启服务（tmux）

优先使用脚本（可重复、可落盘、可复用）：

- `python loop/scripts/uxloop_tmux.py --config <cfg> --reset-window --wait-ready`

等待服务就绪（ready_check/健康检查/打开首页/端口连通），把关键观测写成 `DEBUG:` 日志。

### 2) 执行一轮 arc:simulate

- 生成新的 `run_id`（建议带上迭代号：`..._iter01`）
- 运行 arc:simulate，并确保产出/编译报告：
  - `python simulate/scripts/scaffold_run.py ...`
  - `python simulate/scripts/check_artifacts.py --run-dir <run_dir> --strict`
  - `python simulate/scripts/compile_report.py --run-dir <run_dir> --in-place`（可选：若已在 venv 中安装 `mdformat` 再加 `--beautify-md`）

### 3) 判断结果

- PASS：停止循环，输出最终 run_id 与报告路径
- FAIL：进入修复
  - 把当前 `run_dir` 作为输入，按 `arc:triage` 的流程定位与修复
  - 修复后回到步骤 1（重启服务再测；否则很容易“修了但没生效”）

### 4) 兜底退出条件

- 达到 `max_iterations` 仍失败：停止，汇总每轮 run_id、失败类型分布、当前最可能根因与下一步建议

## Resource & Cleanup（资源控制与及时关闭）

- 不要无限开 session / 无限开服务：始终复用同一个 `session_name`，并使用 `max_iterations` 约束回归次数。
- 日志/报告会持续增长：建议每轮用独立 `run_id`，结束后按需清理 `logs/uxloop/<run_id>/` 与 `reports/<run_id>/`。
- 结束后及时关闭 tmux 与服务（避免后台进程长期占用 CPU/内存/端口）：

```bash
python loop/scripts/uxloop_cleanup.py --session uxloop --window svc --kill-session
```
