# Lean Arc Cheatsheet

Arc 现在只处理四类工程判断。任务编排、记忆和跨 Agent 协作走 `aitask`。

## 选择规则

| 输入信号 | 使用 |
|---|---|
| 需求不清、边界不明、验收标准缺失 | `arc:clarify` |
| 方案已明确，需要改代码并交付验证 | `arc:build` |
| 已有失败证据、线上故障或测试失败 | `arc:fix` |
| 需要只读体检、风险盘点、改进建议 | `arc:audit` |

## 默认顺序

1. 不清楚先 `arc:clarify`。
2. 要改代码用 `arc:build`。
3. 有失败证据用 `arc:fix`。
4. 要评估质量用 `arc:audit`。
