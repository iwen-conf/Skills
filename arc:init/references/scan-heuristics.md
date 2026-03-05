# 扫描策略与启发式规则

> arc:init Phase 1 深度扫描阶段的参考文档。定义项目拓扑识别、显著性评分、技术栈检测和架构模式识别的规则。

## 项目拓扑检测

### Monorepo 指标

以下任一文件/配置存在于根目录即判定为 Monorepo：

| 指标文件 | 生态 |
|---------|------|
| `package.json` 含 `workspaces` 字段 | Node.js (npm/yarn) |
| `pnpm-workspace.yaml` | Node.js (pnpm) |
| `lerna.json` | Node.js (Lerna) |
| `nx.json` | Node.js (Nx) |
| `go.work` | Go workspace |
| `Cargo.toml` 含 `[workspace]` | Rust workspace |
| `turbo.json` | Node.js (Turborepo) |

### 多仓库工作空间指标

同时满足：
- 多个子目录各自有 `.git` 目录
- 根目录**无** package manifest 或 workspace 配置
- 子目录间无 workspace 引用

### 单项目指标

- 根目录有且仅有一个 `.git`
- 根目录有一个 package manifest (go.mod, package.json, Cargo.toml 等)

---

## 显著性评分

对项目内的每个目录进行评分，决定是否为其生成独立的 CLAUDE.md。

### 评分规则（0-10 分）

| 指标 | 分值 | 检测方式 |
|------|------|---------|
| 有 package manifest (go.mod, package.json, Cargo.toml, pyproject.toml, setup.py, Gemfile, Package.swift, *.csproj) | +3 | Glob 扫描 |
| 有独立 `.git` 目录 | +2 | Glob `**/.git` |
| 源文件数 > 10 | +1 | Bash `find ... \| wc -l` |
| 有 README.md 或 docs/ 目录 | +1 | Glob |
| 有测试文件 (*_test.go, *.test.ts, test_*.py, *_spec.rb 等) | +1 | Glob |
| 有 CI 配置 (.github/workflows/, .gitlab-ci.yml, Jenkinsfile) | +1 | Glob |
| 被父级 workspace/manifest 显式引用 | +2 | Read 父级 manifest |

### 阈值

- **>= 4 分**：生成独立 CLAUDE.md
- **< 4 分**：合并到父级 CLAUDE.md 的模块索引中

### 特殊规则

- `node_modules/`, `vendor/`, `target/`, `dist/`, `build/`, `.cache/`, `__pycache__/` 永远排除
- `.arc/` 目录排除
- `max_module_depth` 参数限制最大扫描深度

---

## 技术栈检测

### Manifest → 语言/框架映射

| Manifest 文件 | 主语言 | 框架检测方式 |
|--------------|--------|------------|
| `go.mod` | Go | 读取 `require` 段：gin→Gin, echo→Echo, fiber→Fiber, hertz→Hertz |
| `package.json` | JavaScript/TypeScript | 读取 `dependencies`：react→React, vue→Vue, solid→SolidJS, next→Next.js, express→Express |
| `Cargo.toml` | Rust | 读取 `[dependencies]`：actix→Actix, axum→Axum, rocket→Rocket, tokio→Tokio |
| `pyproject.toml` / `requirements.txt` | Python | 搜索：django→Django, flask→Flask, fastapi→FastAPI |
| `Gemfile` | Ruby | 搜索：rails→Rails, sinatra→Sinatra |
| `Package.swift` | Swift | SwiftPM 项目 |
| `*.csproj` | C# | 搜索 SDK 类型：Microsoft.NET.Sdk.Web→ASP.NET |
| `pom.xml` / `build.gradle` | Java/Kotlin | 搜索：spring→Spring Boot |

### 版本提取

- Go：`go.mod` 第一行 `go 1.XX`
- Node.js：`package.json` → `engines.node` 或推导
- Rust：`Cargo.toml` → `edition`
- Python：`pyproject.toml` → `requires-python`

### 构建/测试命令检测

| 文件 | 提取内容 |
|------|---------|
| `Makefile` | make targets（build, test, lint, dev, run） |
| `package.json` → `scripts` | npm/pnpm 脚本 |
| `.github/workflows/*.yml` | CI steps 中的命令 |
| `Dockerfile` | 构建和入口命令 |
| `docker-compose.yml` | 服务启动方式 |

---

## 架构模式识别

根据目录结构推断项目采用的架构模式。

### DDD（领域驱动设计）

特征目录：`domain/` + `usecase/` (或 `application/`) + `interface/` (或 `adapter/`) + `infrastructure/`

### MVC

特征目录：`controllers/` (或 `handlers/`) + `models/` + `views/` (或 `templates/`)

### Standard Go Layout

特征目录：`cmd/` + `internal/` + `pkg/`

### 前端 SPA

特征目录：`src/components/` + `src/pages/` (或 `src/views/`) + `src/store/` (或 `src/stores/`)

### 微服务

特征：多个 `service-*/` 目录，各有独立 Dockerfile 或 manifest

### 库/工具

特征：根目录直接有源码文件 + `examples/` + 无 `src/` 分层

---

## CLAUDE.md 层级分类

根据拓扑和目录在树中的位置，确定每个 CLAUDE.md 的层级：

| 条件 | 层级 |
|------|------|
| 工作空间/仓库的最顶层 | 根级 (Root) |
| 包含多个显著子目录但自身不是独立项目 | 分组级 (Group) |
| 独立项目/包/服务 (有自己的 manifest 或 .git) | 模块级 (Module) |

---

## 生成顺序

**叶子优先（Leaf-First）**：

1. 先生成所有模块级 CLAUDE.md（最深层）
2. 再生成分组级 CLAUDE.md（中间层，此时可引用已生成的模块级）
3. 最后生成根级 CLAUDE.md（顶层，此时可引用所有下级）

这确保 mermaid `click` 链接和面包屑导航指向的文件在引用时已存在。
