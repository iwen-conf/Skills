# Lean Arc Cheatsheet

Arc 现在处理五类判断：一个项目级（define），四个任务级（clarify/build/fix/audit）。任务编排、记忆和跨 Agent 协作走 `aitask`。

## 选择规则

| 输入信号 | 使用 |
|---|---|
| 项目想法待结构化、需要 PRD/Blueprint | `arc:define` |
| 需求不清、边界不明、验收标准缺失 | `arc:clarify` |
| 方案已明确，需要改代码并交付验证 | `arc:build` |
| 已有失败证据、线上故障或测试失败 | `arc:fix` |
| 需要只读体检、风险盘点、改进建议 | `arc:audit` |

## 默认顺序

1. 项目尚未定义先 `arc:define`。
2. 任务不清楚先 `arc:clarify`。
3. 要改代码用 `arc:build`。
4. 有失败证据用 `arc:fix`。
5. 要评估质量用 `arc:audit`。
