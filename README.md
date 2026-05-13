# Skills

这个仓库保留一组轻量 Skill 文档。任务编排、Inbox、上下文记忆和 OpenViking 写入由 `aitask` 体系负责；`Arc/` 不再承担总控、索引、E2E、门禁或本地服务托管。

## 当前保留

```text
Skills/
├── Arc/
│   ├── arc:clarify/
│   ├── arc:build/
│   ├── arc:fix/
│   └── arc:audit/
├── graduation-doc-support/
├── terminal-table-output/
├── docs/
├── schemas/
├── scripts/
├── src/
└── tests/
```

## Arc 边界

| Skill ID | 用途 |
|---|---|
| `arc:clarify` | 把模糊需求收敛成可执行上下文、约束和验收标准 |
| `arc:build` | 在方案明确时做代码交付、验证和变更说明 |
| `arc:fix` | 基于失败证据定位根因、修复并回归验证 |
| `arc:audit` | 做只读项目体检，输出风险和改进建议 |

## 非 Arc 通用 Skill

| Skill ID | 用途 |
|---|---|
| `graduation-doc-support` | 基于真实项目生成毕业设计支撑文档 |
| `terminal-table-output` | 聊天中输出紧凑盒线表 |

## 收敛原则

- `aitask` 负责任务编排、协作、Inbox、OpenViking 记忆和跨 Agent 状态。
- Arc 只保留工程交付过程中的四个稳定判断框架。
- Arc Skill 默认是纯 `SKILL.md` 契约，不维护专用 Python/Go 运行时。
- 图表、浏览器、飞书、Lazycat、前端设计等垂直能力由对应专门 Skill 负责，不再在 Arc 内重复建设。

## 校验

```bash
uv run python scripts/validate_skills.py
uv run pytest tests/test_skill_validation.py tests/test_skill_registry.py tests/test_arc_privacy.py -q
uv run python scripts/build_skills_index.py
```
