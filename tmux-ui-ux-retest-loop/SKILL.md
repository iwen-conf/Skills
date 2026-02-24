---
name: tmux-ui-ux-retest-loop
description: 使用 tmux 启动/重启本地或测试环境服务（多服务多 pane、可 pipe-pane 落日志），并循环运行 ui-ux-simulation 做回归验证（失败→修复→重启→再测），直到 PASS 或达到迭代上限；每轮输出 run_id、git 版本、tmux 会话信息与报告/日志路径。
---

# tmux 启动 + ui-ux-simulation 回归闭环（工业化）

## Overview

用于“修复后必须重启服务才能生效”的项目：把服务启动/重启、日志取证、回归测试与失败再修复串成可重复闭环，并且每轮都有可交付证据。

## Context Budget（避免 Request too large）

- 服务日志以落盘为准（`logs/uxloop/<run_id>/...`）；不要把整段日志粘贴到对话里，必要时只摘录“失败时间点附近的少量行”并注明来源文件路径。
- 与测试报告统一使用同一个 `run_id`，让证据链只靠路径对齐（少说话、多给路径）。

## Definition of Done（每轮必须产出）

每轮迭代在 stdout 明确输出（便于粘贴进 ticket/PR/日报）：

- `iter`: 01..N
- `run_id` / `run_dir`（ui-ux-simulation 工件目录）
- `result`: `PASS|FAIL` + 一句话失败类型（selector/timing/auth/backend/env/data/unknown）
- `git`: `HEAD` + 是否 dirty（`git status --porcelain` 是否为空）
- `tmux`: `session_name` / `window_name` / 每个服务对应的 pane
- `logs`: `logs_dir` + 每个服务的 log_file

并且：

- FAIL → 必须交给 `ui-ux-defect-fix` 输出根因与修复证据（禁止“感觉修了”）
- PASS → 必须给出通过 run 的 `run_id` + `report.md` 路径

## Artifacts & Paths（文档/文件放哪）

推荐**每轮迭代使用同一个 `run_id`** 贯穿“服务日志 + 测试报告”，这样证据可直接对齐：

- **服务日志（本 Skill / tmux）**：默认 `logs/uxloop/<run_id>/<service>.log`（可用 `uxloop_tmux.py --logs-dir ...` 覆盖）。
- **测试报告（ui-ux-simulation）**：默认 `reports/<run_id>/`（可用 `scaffold_run.py --report-output-dir ...` 覆盖）。
- **账号管理（ui-ux-simulation）**：`reports/<run_id>/accounts.jsonc`（明文账号/密码/Token；不得提交入库）。

示例（同一个 `run_id` 打通 logs+reports）：

```bash
run_id="2026-02-01_14-00-00_abcd_iter01"
python tmux-ui-ux-retest-loop/scripts/uxloop_tmux.py --config uxloop.config.json --run-id "$run_id" --reset-window --wait-ready
# 然后用同一个 run_id 跑 ui-ux-simulation，产出 reports/$run_id/
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

### B) ui-ux-simulation 参数

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
python tmux-ui-ux-retest-loop/scripts/uxloop_tmux.py \
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
  1. `python ui-ux-simulation/scripts/check_artifacts.py --run-dir <run_dir> --strict`（推荐）
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

- `python tmux-ui-ux-retest-loop/scripts/uxloop_tmux.py --config <cfg> --reset-window --wait-ready`

等待服务就绪（ready_check/健康检查/打开首页/端口连通），把关键观测写成 `DEBUG:` 日志。

### 2) 执行一轮 ui-ux-simulation

- 生成新的 `run_id`（建议带上迭代号：`..._iter01`）
- 运行 ui-ux-simulation，并确保产出/编译报告：
  - `python ui-ux-simulation/scripts/scaffold_run.py ...`
  - `python ui-ux-simulation/scripts/check_artifacts.py --run-dir <run_dir> --strict`
  - `python ui-ux-simulation/scripts/compile_report.py --run-dir <run_dir> --in-place`（可选：若已在 venv 中安装 `mdformat` 再加 `--beautify-md`）

### 3) 判断结果

- PASS：停止循环，输出最终 run_id 与报告路径
- FAIL：进入修复
  - 把当前 `run_dir` 作为输入，按 `ui-ux-defect-fix` 的流程定位与修复
  - 修复后回到步骤 1（重启服务再测；否则很容易“修了但没生效”）

### 4) 兜底退出条件

- 达到 `max_iterations` 仍失败：停止，汇总每轮 run_id、失败类型分布、当前最可能根因与下一步建议

## Resource & Cleanup（资源控制与及时关闭）

- 不要无限开 session / 无限开服务：始终复用同一个 `session_name`，并使用 `max_iterations` 约束回归次数。
- 日志/报告会持续增长：建议每轮用独立 `run_id`，结束后按需清理 `logs/uxloop/<run_id>/` 与 `reports/<run_id>/`。
- 结束后及时关闭 tmux 与服务（避免后台进程长期占用 CPU/内存/端口）：

```bash
python tmux-ui-ux-retest-loop/scripts/uxloop_cleanup.py --session uxloop --window svc --kill-session
```
