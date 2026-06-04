# Skills

这个仓库保留一组 `arc:*` 软件工程生命周期 Skill 文档。代码库搜索默认走本地索引工具链：Zoekt、ast-grep、Universal Ctags；任务编排、Inbox 和跨 Agent 状态由 `aitask` 体系负责；飞书资源是可选能力，只有已有 `.lark.json`、用户提供飞书链接、或用户明确触发/确认时才由 `arc:docs` 创建和索引。

## 当前保留

```text
Skills/
├── Arc/
│   ├── arc:define/
│   ├── arc:clarify/
│   ├── arc:docs/
│   ├── arc:build/
│   ├── arc:frontend/
│   ├── arc:fix/
│   └── arc:audit/
├── .ai-code-index/
├── docs/
├── schemas/
├── scripts/
├── src/
└── tests/
```

## Arc 边界

| Skill ID | 生命周期阶段 | 用途 |
|---|---|
| `arc:define` | 项目定义 | 把模糊想法整理为 PRD / Blueprint，并可在用户启用飞书后交给 `arc:docs` 建立项目文档中心 |
| `arc:clarify` | 需求澄清 | 把模糊需求收敛成可执行上下文、约束和验收标准 |
| `arc:docs` | 可选飞书工作区 | 在飞书已启用或用户触发时创建/维护飞书项目文档、`.lark.json` 索引、进度和生命周期资料 |
| `arc:build` | 代码交付 | 在方案明确时做代码交付、验证和变更说明 |
| `arc:frontend` | 前端工程 | 收敛 React/Vite/Tailwind/shadcn 基线并记录前端决策 |
| `arc:fix` | 故障修复 | 基于失败证据定位根因、修复并回归验证 |
| `arc:audit` | 项目体检 | 做只读项目体检，输出风险和改进建议 |

## 共享参考

| 文档 | 用途 |
|---|---|
| [`docs/code-rot-taxonomy.md`](docs/code-rot-taxonomy.md) | AI 代码腐化 36 条权威清单(6 大家族),各 Arc 技能引用各自负责的切片作为可执行门禁 |

## 收敛原则

- 所有 Skill 需要搜索代码库或定位上下文时，优先使用 `.ai-code-index/search.sh` 查询本地 Zoekt 索引。
- 搜索代码结构、调用形态或重构目标时，优先使用 `.ai-code-index/struct-search.sh`。
- 查找符号定义或符号清单时，优先使用 `.ai-code-index/symbols.sh`。
- 索引缺失或过期时先运行 `.ai-code-index/reindex.sh`；`rg` 只用于小范围精确补充、新建文件或索引兜底。
- 项目如果接入飞书，必须在项目根目录维护 `.lark.json`，记录项目主页、PRD、需求、架构、任务表、仪表盘、进度、审计、交付、故障和本地索引状态。
- 项目没有 `.lark.json` 时默认本地处理，不创建飞书；用户可通过 `启用飞书项目空间`、`接入飞书`、`创建飞书文档`、`创建飞书任务表`、`创建飞书仪表盘`、`同步到飞书`、`索引飞书资源` 等触发词启用。
- 用户说 `创建飞书项目空间`、`创建完整飞书项目空间` 或 `一键创建飞书项目空间` 时，`arc:docs` 必须一次性创建标准项目空间：文件夹、文档、多维表格、仪表盘、项目流、日历、协作资源、画板、自动化等，并把全部真实链接/ID 写入 `.lark.json`。
- 用户说 `更新飞书项目空间`、`刷新飞书项目空间`、`补齐飞书项目空间` 或 `同步飞书项目空间` 时，`arc:docs` 必须更新既有空间：校验 `.lark.json`，修复断链/缺失索引，补齐标准资源，刷新任务表、仪表盘、项目流和自动化，不得重复创建项目空间。
- 飞书操作由 `arc:docs` 路由到 `lark-doc` / `lark-base` / `lark-shared` 等对应技能，遵守认证、API 版本和高风险写入规则。
- `aitask` 仅负责任务编排、协作、Inbox 和跨 Agent 状态。
- Arc 只保留软件工程生命周期中的稳定判断框架和文档索引契约。
- Arc Skill 默认是纯 `SKILL.md` 契约，不维护专用 Python/Go 运行时。
- 图表、浏览器、Lazycat、纯设计等垂直能力由对应专门 Skill 负责，不再在 Arc 内重复建设。

## 校验

```bash
.ai-code-index/reindex.sh
uv run python scripts/validate_skills.py
uv run pytest tests/test_skill_validation.py tests/test_skill_registry.py tests/test_arc_privacy.py -q
uv run python scripts/build_skills_index.py
```
