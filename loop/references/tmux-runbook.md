# tmux Runbook（面向 UI/UX 回归闭环）

> 目标：把“启动/重启服务 + 观测日志 + 回归验证 + 失败后再修复”的操作收敛成可重复、可审计、可恢复的 runbook。

## Session / Window 基础

- 列出 session：`tmux ls`
- 检查 session 是否存在：`tmux has-session -t uxloop`
- 创建后台 session：`tmux new -d -s uxloop`
- 附加：`tmux attach -t uxloop`
- 杀掉 session（谨慎）：`tmux kill-session -t uxloop`
- 列出窗口：`tmux list-windows -t uxloop`
- 切窗口：`Ctrl-b w`

## Pane 管理

- 分屏（纵向）：`Ctrl-b %`
- 分屏（横向）：`Ctrl-b "`
- 关闭 pane：`Ctrl-b x`
- 列出 panes：`tmux list-panes -t uxloop:svc -F "#{pane_index} #{pane_id} #{pane_title}"`
- 平铺布局：`tmux select-layout -t uxloop:svc tiled`
- 设置 pane title：`tmux select-pane -t uxloop:svc.0 -T backend`

## 发送命令 / 重启策略

- 发送命令：`tmux send-keys -t uxloop:svc.0 "cd backend && npm run dev" C-m`
- 温和停止：`tmux send-keys -t uxloop:svc.0 C-c`
- 观察是否退出：`tmux capture-pane -t uxloop:svc.0 -p | tail -n 50`
- 若进程僵死：优先定位 PID 再 kill（比 kill-session 更安全）
  - `tmux list-panes -t uxloop:svc -F "#{pane_title} #{pane_pid}"`
  - `kill -TERM <pid>`（必要时 `kill -KILL <pid>`）

## 日志持久化（强烈推荐）

- 将某个 pane 的输出落盘：
  - `tmux pipe-pane -t uxloop:svc.0 "cat >> logs/uxloop/backend.log"`
- 更换落盘文件（覆盖 pipe 命令即可）：
  - `tmux pipe-pane -t uxloop:svc.0 "cat >> logs/uxloop/<run_id>/backend.log"`
- 关闭 pipe：
  - `tmux pipe-pane -t uxloop:svc.0`

## 快速环境检查（避免“测了半天其实服务没起来”）

- 端口占用：`lsof -i :5173 -sTCP:LISTEN`
- HTTP 健康检查：`curl -fsS http://localhost:3000/health | head`
- 前端可达：`curl -fsSI http://localhost:5173/ | head`

## 取证（回归失败时常用）

- 导出 pane 最后一段输出到文件：
  - `tmux capture-pane -t uxloop:svc.0 -p -S -5000 > logs/uxloop/backend.tail.txt`
- 记录 tmux 结构快照：
  - `tmux list-windows -t uxloop > logs/uxloop/tmux.windows.txt`
  - `tmux list-panes -t uxloop -a -F "#{session_name}:#{window_name}.#{pane_index} #{pane_title} #{pane_pid}" > logs/uxloop/tmux.panes.txt`

