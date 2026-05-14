# Lean Arc Routing Matrix

Arc 已收敛为四个轻量工程 Skill。若任务涉及代码库搜索或上下文定位，优先使用 `.ai-code-index/` 本地搜索脚本；若任务涉及编排、Inbox、项目状态同步或跨 Agent 协作，优先使用 `aitask`。

| Skill | 首选触发 | 不建议使用时机 | 常见后续 |
|---|---|---|---|
| `arc:clarify` | 需求不清、上下文缺失、验收标准缺失 | 需求已明确可直接执行 | `arc:build` / `arc:audit` |
| `arc:build` | 方案明确，需要代码交付和验证 | 根因未知的失败修复 | `arc:audit` |
| `arc:fix` | 有失败证据、线上故障、测试失败 | 只是新功能开发 | `arc:build` / `arc:audit` |
| `arc:audit` | 需要只读体检、风险盘点、改进建议 | 需要直接改代码 | `arc:clarify` / `arc:build` |

## Decision Tree

```mermaid
flowchart TD
    A[输入任务] --> B{需求是否清晰?}
    B -- 否 --> C[arc:clarify]
    B -- 是 --> D{是否已有失败证据?}
    D -- 是 --> F[arc:fix]
    D -- 否 --> E{是否需要改代码?}
    E -- 是 --> G[arc:build]
    E -- 否 --> H{是否需要只读评估?}
    H -- 是 --> I[arc:audit]
    H -- 否 --> J[本地搜索 / aitask / 直接处理]
```

## Fast Rules

- 需要多人/多 Agent/跨会话/记忆：用 `aitask`，不要用 Arc。
- 需要代码库搜索或上下文定位：用 `.ai-code-index/search.sh`、`struct-search.sh`、`symbols.sh`，不要在 Arc 内另建搜索机制。
- 需要浏览器自动化：用 `agent-browser` 或相关浏览器工具。
- 需要图表：用 `drawio`。
- 需要测试：直接按项目测试框架生成或运行，不再走 Arc 内部测试 Skill。
- 需要 E2E：直接使用项目 E2E 工具或 `agent-browser`，不再走 Arc 内部 E2E Skill。
