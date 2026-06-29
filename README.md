# Skills

这个仓库保留一组 `arc:*` 软件工程生命周期 Skill 文档，以及少量显式白名单的通用工程约束 Skill。代码库搜索默认走本地索引工具链：Zoekt、ast-grep、Universal Ctags；任务编排、Inbox 和跨 Agent 状态由 `aitask` 体系负责；飞书项目空间只在用户明确要求创建/连接后由 `arc:docs` 创建/维护 `.lark.json`、项目资料、任务表、仪表盘、Wiki、画板和生命周期索引。

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
│   ├── arc:audit/
│   └── arc:security/
├── code-comment-conventions/
├── project-architecture-conventions/
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
| `arc:docs` | 飞书项目空间 | 在用户明确创建/连接飞书空间或项目已有 `.lark.json` 时，维护飞书项目文档、`.lark.json` 索引、进度、资料和生命周期资料 |
| `arc:build` | 代码交付 | 在方案明确时做代码交付、验证和变更说明 |
| `arc:frontend` | 前端工程 | 收敛 Web、移动、桌面和小程序默认技术栈并记录前端决策 |
| `arc:fix` | 故障修复 | 基于失败证据定位根因、修复并回归验证 |
| `arc:audit` | 项目体检 | 做只读项目体检，输出风险和改进建议 |
| `arc:security` | 安全自动化 | 本地安装和编排安全 CLI，生成可读安全报告，并把修复/审计/文档沉淀交回 Arc |

## 通用工程约束

| Skill ID | 用途 |
|---|---|
| `code-comment-conventions` | 统一 Controller、接口/契约、普通函数、结构体/字段和函数内部编号步骤注释规范 |
| `project-architecture-conventions` | 统一默认后端架构命名/分层/接口设计、DIP、依赖注入位置、Go 常量/枚举和 helper 抽取规范 |

## 共享参考

| 文档 | 用途 |
|---|---|
| [`docs/code-rot-taxonomy.md`](docs/code-rot-taxonomy.md) | AI 代码腐化 36 条权威清单(6 大家族),各 Arc 技能引用各自负责的切片作为可执行门禁 |

## 收敛原则

- 所有 Skill 需要搜索代码库或定位上下文时，优先使用 `.ai-code-index/search.sh` 查询本地 Zoekt 索引。
- 搜索代码结构、调用形态或重构目标时，优先使用 `.ai-code-index/struct-search.sh`。
- 查找符号定义或符号清单时，优先使用 `.ai-code-index/symbols.sh`。
- 索引缺失或过期时先运行 `.ai-code-index/reindex.sh`；`rg` 只用于小范围精确补充、新建文件或索引兜底。
- 不按项目大小自动创建飞书空间。只有用户明确说 `创建项目的飞书空间`、`创建飞书项目空间`、`初始化飞书项目空间`，或提供已有飞书项目空间链接时，才由 `arc:docs` 创建/连接项目空间。
- 项目如果接入飞书，必须在项目根目录维护 `.lark.json`，记录项目文件空间地址、项目主页、PRD、需求、架构、任务表、仪表盘、进度、审计、交付、故障、资料来源和本地索引状态。
- 创建/连接飞书项目空间后，必须把项目文件空间飞书地址写入 `.lark.json.resources.drive_folder.url`，后续启动 AI 或进入项目时先读取 `.lark.json`。
- 后续查找、更新、删除、创建项目相关飞书文档/表格/画板/资料时，都必须通过 `.lark.json` 里的飞书地址和资源 ID 定位；删除等破坏性操作仍需确认。
- 后续搜索到的资料、新增文档、外部链接、接口说明、架构事实、决策、截图、报告、会议纪要和交付证据，只要项目已有 `.lark.json`，都必须通过 `arc:docs` 写入对应飞书 Doc/Wiki/Base/Drive/Whiteboard/Slides，并在 `.lark.json.lifecycle[]` 保留来源和时间。
- Web 前端默认统一走 `arc:frontend`：除非用户明确指定其他技术，默认栈固定为 React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui + Zustand + TanStack Query + TanStack Router + React Hook Form + Zod。
- 所有前后端交付必须区分状态语义：无数据/空状态是成功业务状态，错误是失败状态，权限不足和单资源不存在也要独立建模；前端不得把空列表、空搜索、空仪表盘或首次使用页面展示成错误页。
- 移动端 iOS/Android 默认统一走 `arc:frontend`：React Native + Expo + TypeScript + NativeWind + Zustand + TanStack Query + Expo Router。
- 桌面端 Mac/Windows/Linux 默认统一走 `arc:frontend`：Tauri 2 + 现有 React Web 默认栈，优先复用 Web 代码。
- 小程序默认统一走 `arc:frontend`：Taro 4 + React + TypeScript + Zustand；`wxskills` 只提供微信 API、隐私、支付、组件、Skyline 和既有原生微信项目维护约束。
- 垂直业务 Skill 只声明“前端面按 `arc:frontend` 平台默认栈交付”，不得按 Lazycat、微信小程序、支付、后台、Dashboard 等业务差异另起一套默认前端栈。
- 用户说 `创建项目的飞书空间`、`创建飞书项目空间`、`创建完整飞书项目空间` 或 `一键创建飞书项目空间` 时，`arc:docs` 必须一次性创建标准项目空间：文件夹、文档、多维表格、仪表盘、项目流、日历、协作资源、画板、自动化等，并把全部真实链接/ID 写入 `.lark.json`。
- 用户说 `更新飞书项目空间`、`刷新飞书项目空间`、`补齐飞书项目空间` 或 `同步飞书项目空间` 时，`arc:docs` 必须更新既有空间：校验 `.lark.json`，修复断链/缺失索引，补齐标准资源，刷新任务表、仪表盘、项目流和自动化，不得重复创建项目空间。
- 飞书操作由 `arc:docs` 路由到 `lark-doc` / `lark-base` / `lark-shared` 等对应技能，遵守认证、API 版本和高风险写入规则。
- `aitask` 仅负责任务编排、协作、Inbox 和跨 Agent 状态。
- Arc 只保留软件工程生命周期中的稳定判断框架和文档索引契约。
- 普通工程约束 Skill 必须显式加入校验白名单，避免任意目录被误收录。
- 所有项目代码交付和修复必须遵守 `project-architecture-conventions`：先看 ponytail；保持 DIP；默认按 `domain`、`usecase`、`interface/restful`、`infrastructure`、`wire` 的分层、命名和接口设计组织后端代码；Go 常量必须使用 `MixedCaps` / `mixedCaps`、优先标准库语义常量、按最小作用域定义，枚举型业务状态必须用自定义类型建模，跨服务常量必须来自版本化契约或受治理的共享模块；DIP 边界接口是明确架构要求，但不得为私有 helper、同层代码或形式主义创建接口。
- Arc Skill 默认是纯 `SKILL.md` 契约；`arc:security` 这种需要可重复自动化的能力可以携带本地脚本。
- 图表、浏览器、Lazycat、纯设计等垂直能力由对应专门 Skill 负责，不再在 Arc 内重复建设。

## 校验

```bash
.ai-code-index/reindex.sh
uv run python scripts/validate_skills.py
uv run pytest tests/test_skill_validation.py tests/test_skill_registry.py tests/test_arc_privacy.py -q
uv run python scripts/build_skills_index.py
```
