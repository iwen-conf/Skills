# 漏洞推演与验证缺口

> 对照 arc:plan 的 closure-loopholes.md 核查 11 类。模型覆盖和运行时验证分开登记；不另建任务 tracker。

| 编号 | 类别 | 问题 | 模型覆盖/缺口/不适用理由 | 来源及 REQ/INV/流转 ID | 严重度 | Finding 状态 | 处置与已有任务 | 运行时证据 |
|---|---|---|---|---|---|---|---|---|
| L-001 | <类别> | <问题> | <模型位置或具体理由> | <来源> | <Critical/High/Medium/Low/Info> | <confirmed/likely/needs-triage/false-positive/manual-gap> | <处置与链接> | <命令/结果或 NOT_RUN 原因> |

## 未决业务决策

- <编号、受影响流转、所需决策；已有确认写依据，不重复索取>

## 交接与关闭证据

- 模型缺口 → arc:plan model；原需求未定 → arc:plan prd；实现缺陷 → arc:code；运行时证明 → arc:verify。
- 模型 PASS 只关闭模型问题；产品缺陷关闭须引用修复后的真实验证证据和原任务进度表。
