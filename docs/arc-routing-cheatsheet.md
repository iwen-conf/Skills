# ARC Routing Cheatsheet (One Screen)

一页速查：先看输入信号，再选主技能。  
详细规则见 [`docs/arc-routing-matrix.md`](./arc-routing-matrix.md)。

## 1) 先判信号

- 技能边界不清 → `arc:agent`
- 需求上下文不清 → `arc:refine`
- 方案争议/高风险 → `arc:deliberate`
- 已有方案要落地 → `arc:implement`

## 2) 常用链路

- 需求到落地：`arc:refine` → `arc:deliberate`（可选）→ `arc:implement`
- 质量治理：`arc:gate`（先触发 `score/`，并联 `arc:review`）
- E2E 修复：`arc:simulate` → `arc:triage`（多轮用 `arc:loop`）
- 索引维护：`arc:init`（自动）/ `arc:init:full` / `arc:init:update`
- 结构地图：`arc:cartography`
- IP 流程：`arc:ip-audit` → `arc:ip-docs`

## 3) 不确定时的默认顺序

1. 先 `arc:agent` 判断路由
2. 需求不清先 `arc:refine`
3. 争议大再 `arc:deliberate`
4. 最后 `arc:implement`
