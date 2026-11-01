# ARC Verify · Agent-Native / 跨现有 Agent 接力变更记录

日期：2026-11-01。来源：用户要求使用当前 Grok、Codex、Claude Code 等已有编程 Agent，而非重新开发 Agent，也无 HTTP 模型、API Key 或模型配置。

## 执行边界

- 当前宿主 Agent 负责业务推理、反事实假设和最小实验选择。
- Arc Skill 负责工作流程、交接与业务约束。
- Go CLI 负责明确授权内的一步执行、fencing/attempt/Oracle/RAW/恢复。
- 本地 Turso/Graph v2 负责跨会话事实、假设、Frontier 和 Checkpoint。
- 独立 Oracle 是业务事实来源，不是第二个 AI 模型。

## CLI 行为

`discover run --mode agent-native` 创建非执行 Campaign；
`discover context/handoff --id X` 读取可移交状态；
`discover propose --id X --proposal FILE` 提交带旧 Checkpoint 的候选；
`discover execute --id X` 只执行一次，再交还当前 Agent；
`discover complete --id X` 执行受约束的安全收尾。

原 CLI `discover run` 不带模式时仍为启发式，避免破坏现有脚本。
`--mode ai` 未提供 Planner 文件时自动使用当前 Agent 工作流。
不要求环境变量 `OPENAI_API_KEY` 或 `ARC_VERIFY_PLANNER_*`。

## 证据约束

Agent-Native 提案是带来源标记的结构化建议，不是模型身份的密码学证明；
它不能自己生成 PASS、批准 RULE 或替代独立 Oracle。
跨 Agent 的恢复必须检查模型/代码/环境/Adapter 身份与 Checkpoint，
不能重放未确认副作用，也不能覆盖旧 FAIL。

## 验收边界（未虚构成功）

1. 已实现 CLI/Skill/测试改动，并推送 main。
2. 因当前执行环境无法联网克隆 GitHub 仓库，本轮未运行 Go build/test/race/vet 或 gofmt。
3. `agent_native_test.go` 已增加跨进程读取原 Campaign 的测试，但尚待实际执行。
4. 三个真实 Agent 宿主之间的接力兼容性未独立验证；测试夹具不等于生产环境。
5. 真正业务 E2E 继续遵循既有授权范围与独立 Oracle；不回退不安全环境。

建议在有代码工作区的 Agent 中首先运行格式化与全量回归，如有失败按反例修复后才宣称验收完成。
