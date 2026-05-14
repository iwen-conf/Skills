# Skills

这个仓库保留一组轻量 Skill 文档。代码库搜索默认走本地索引工具链：Zoekt、ast-grep、Universal Ctags；任务编排、Inbox 和跨 Agent 状态由 `aitask` 体系负责；`Arc/` 不再承担总控、E2E、门禁或本地服务托管。

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
├── frontend-stack-baseline/
├── .ai-code-index/
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
| `frontend-stack-baseline` | 为 React/Vite/Tailwind/shadcn 前端任务提供默认栈和配色基线 |

## 收敛原则

- 所有 Skill 需要搜索代码库或定位上下文时，优先使用 `.ai-code-index/search.sh` 查询本地 Zoekt 索引。
- 搜索代码结构、调用形态或重构目标时，优先使用 `.ai-code-index/struct-search.sh`。
- 查找符号定义或符号清单时，优先使用 `.ai-code-index/symbols.sh`。
- 索引缺失或过期时先运行 `.ai-code-index/reindex.sh`；`rg` 只用于小范围精确补充、新建文件或索引兜底。
- `aitask` 仅负责任务编排、协作、Inbox 和跨 Agent 状态。
- Arc 只保留工程交付过程中的四个稳定判断框架。
- Arc Skill 默认是纯 `SKILL.md` 契约，不维护专用 Python/Go 运行时。
- 图表、浏览器、飞书、Lazycat、前端设计等垂直能力由对应专门 Skill 负责，不再在 Arc 内重复建设。

## 校验

```bash
.ai-code-index/reindex.sh
uv run python scripts/validate_skills.py
uv run pytest tests/test_skill_validation.py tests/test_skill_registry.py tests/test_arc_privacy.py -q
uv run python scripts/build_skills_index.py
```
