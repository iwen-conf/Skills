# ARC Routing Cheatsheet (One Screen)

一页速查：先看输入信号，再选主技能。  
详细规则见 [`docs/arc-routing-matrix.md`](./arc-routing-matrix.md)。

## 1) 先判信号

- 技能边界不清 → `arc:exec`
- 需求上下文不清 → `arc:clarify`
- 方案争议/高风险 → `arc:decide`
- 上下文过长/切新会话/需要恢复任务状态 → `arc:context`
- 要启动/重启/停止本地前后端或 dev server → `arc:serve`
- 已有方案要落地 → `arc:build`
- 需要系统建模图谱 → `arc:uml`
- 学术文本需要分段、双阶段润色 → `arc:aigc`

## 2) 常用链路

- 需求到落地：`arc:clarify` → `arc:decide`（可含 `--mode estimate`）→ `arc:build`
- 会话恢复：`arc:context` → `arc:build` / `arc:fix` / `arc:e2e`
- 本地服务管理：`arc:serve`（`tmux` 托管前后端 / worker，会先查重再启停）
- 需求澄清后进入写作链：`arc:clarify` → `arc:aigc` → `arc:audit`
- 学术润色：`arc:aigc`（分段重写 → 全文统稿 → 引用/公式保真复核）
- 质量治理：`arc:gate`（先触发 `score/`，并联 `arc:audit`）
- E2E 修复：`arc:e2e` → `arc:fix`（多轮用 `arc:fix --mode retest-loop`）
- 代码测试：`arc:test`（单测/边界/benchmark/fuzz 生成）
- 索引维护：`arc:init`（自动）/ `arc:init --mode full` / `arc:init --mode update`
- 结构地图：`arc:cartography`
- UML 图谱：`arc:uml`
- IP 流程：`arc:ip-check` → `arc:ip-draft`

## 3) 不确定时的默认顺序

1. 先 `arc:exec` 判断路由
2. 需求不清先 `arc:clarify`
3. 争议大再 `arc:decide`
4. 涉及本地长时服务先 `arc:serve`
5. 最后 `arc:build`
