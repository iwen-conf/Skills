---
name: "arc:init:update"
description: "增量更新项目 CLAUDE.md 索引体系。基于指纹对比和 git diff 检测变更，仅分析和更新受影响的模块，大幅减少更新成本。"
---

# arc:init:update — 增量更新

## Overview

本 Skill 是 `arc:init` 子系统的增量模式，通过指纹对比和 git diff 检测项目变更，仅分析和更新受影响的 CLAUDE.md 文件。

**与 `arc:init:full` 的区别**：
- `arc:init:full`：全量扫描 + 全量多 Agent 分析 + 全量生成
- `arc:init:update`：变更检测 + 选择性分析 + 增量生成

**核心优势**：
- 跳过未变更模块的扫描和分析
- 减少 Agent 调用（仅分析变更模块）
- 保留未变更 CLAUDE.md 的手动编辑内容
- 生成增量报告，清晰展示变更范围

## Prerequisites

**必须满足以下条件才能运行**：

1. `.arc/init/context/module-fingerprints.json` 存在
2. `.arc/init/context/project-snapshot.md` 存在
3. `.arc/init/context/generation-plan.md` 存在

> 如不满足，提示用户先运行 `arc:init` 或 `arc:init:full` 进行全量初始化。

## When to Use

- 已运行过 `arc:init`，需要更新部分模块
- 新增/删除/修改了部分代码，需要同步 CLAUDE.md
- 定期维护项目索引，希望减少时间和成本
- 用户显式要求增量更新

## Input Arguments

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `project_path` | string | 是 | — | 项目根目录绝对路径 |
| `force_modules` | string[] | 否 | — | 强制更新指定模块（即使 fingerprint 未变） |
| `skip_agents` | boolean | 否 | false | 跳过 Agent 分析，仅基于文件变更重新生成 |
| `since` | string | 否 | auto | Git ref 起点（默认从 fingerprints 中的 git_ref） |
| `dry_run` | boolean | 否 | false | 仅输出变更报告，不实际修改文件 |
| `language` | string | 否 | zh-CN | 输出语言 |

## Dependencies

* **ace-tool (MCP)**: 必须。语义搜索项目代码结构。
* **oh-my-opencode Task API**: 必须。调度 oracle/deep/momus Agent。
* **Git**: 必须。变更检测依赖 git 命令。

## Critical Rules

1. **指纹驱动**
 - 所有变更判断必须基于 `module-fingerprints.json` 对比。
 - 不得猜测哪些模块需要更新。

2. **最小修改原则**
 - 仅修改变更模块及其祖先的 CLAUDE.md。
 - 未变更模块的 CLAUDE.md 保持不动。
 - 保留用户手动编辑的内容（通过 `<!-- user -->` 标记区域）。

3. **级联更新**
 - 叶子模块变更 → 更新叶子 → 更新所有祖先的索引/mermaid 部分。
 - 不重新生成祖先的全部内容，仅更新受影响章节。

4. **Agent 缩减**
 - ADDED 模块：完整 3 Agent 分析
 - MODIFIED（key_files 变化）：2 Agent（deep + momus）
 - MODIFIED（仅 source 变化）：1 Agent（deep）
 - DELETED/RENAMED：0 Agent

## Instructions（执行流程）

### Phase U1: 变更检测（Change Detection）

**目标**：识别哪些模块发生了变更，分类变更类型。

#### Step U1.1: 加载基线

1. 读取 `.arc/init/context/module-fingerprints.json`
2. 读取 `.arc/init/context/generation-plan.md`
3. 读取 `.arc/init/context/project-snapshot.md`
4. 记录基线 git_ref

#### Step U1.2: Git 变更检测

```bash
# 获取变更文件列表
git diff --name-only <baseline_git_ref>..HEAD

# 获取重命名检测
git diff --name-status --diff-filter=R <baseline_git_ref>..HEAD

# 获取删除文件
git diff --name-status --diff-filter=D <baseline_git_ref>..HEAD
```

#### Step U1.3: 重新计算指纹

对 git diff 涉及的所有模块目录：

1. 计算当前 `key_files` 哈希
2. 计算当前 `source_tree_hash`
3. 计算当前 `claude_md_hash`（如 CLAUDE.md 存在）

#### Step U1.4: 变更分类

对每个模块对比基线指纹 vs 当前指纹：

| 变更类型 | 检测条件 | 优先级 |
|----------|----------|--------|
| `ADDED` | 目录不在 fingerprints 中，score >= 4 | 高 |
| `DELETED` | 目录在 fingerprints 中，但目录不存在 | 高 |
| `RENAMED` | git detect rename 或路径匹配相似目录 | 中 |
| `MODIFIED_KEY` | `key_files` 哈希变化（manifest/配置变化） | 高 |
| `MODIFIED_SOURCE` | 仅 `source_tree_hash` 变化 | 中 |
| `MODIFIED_CLAUDE_MD` | 仅 `claude_md_hash` 变化（用户手动编辑） | 低 |
| `UNCHANGED` | 所有哈希一致 | — |

#### Step U1.5: 新增模块扫描

对潜在新增目录：

1. 按 `references/scan-heuristics.md` 评分
2. score >= 4 的目录标记为 `ADDED`
3. 收集新模块元数据

#### Step U1.6: 祖先级联标记

对所有 `ADDED` / `DELETED` / `RENAMED` / `MODIFIED_*` 模块：

1. 找出所有祖先目录（父级、祖父级...直到根）
2. 标记祖先为 `STALE_PARENT`

#### Step U1.7: 生成变更报告

产出 `context/updates/update-<timestamp>.md`：

```markdown
# 增量更新变更报告

- **更新时间**: <ISO 8601>
- **基线 Git Ref**: <old_ref> → <new_ref>
- **基线时间**: <timestamp>

## 变更摘要

| 模块 | 变更类型 | Agent 分析 | CLAUDE.md 操作 |
|------|----------|------------|----------------|
| src/new-feature/ | ADDED | oracle+deep+momus | 新建 |
| src/auth/ | MODIFIED_KEY | deep+momus | 合并更新 |
| src/utils/ | MODIFIED_SOURCE | deep | 合并更新 |
| src/legacy/ | DELETED | — | 删除 |
| src/ | STALE_PARENT | — | 索引更新 |

## 文件变更统计
- 新增文件: N
- 修改文件: M
- 删除文件: D

## 详细变更列表
<git diff --stat output>
```

> `dry_run=true` 时，输出报告后立即结束，不执行后续阶段。

---

### Phase U2: 选择性分析（Selective Analysis）

**目标**：仅对需要 Agent 分析的模块启动分析。

#### Step U2.1: 确定分析范围

```
需要完整分析（3 Agent）:
 - 所有 ADDED 模块

需要部分分析（2 Agent）:
 - 所有 MODIFIED_KEY 模块（deep + momus，跳过 oracle）

需要最小分析（1 Agent）:
 - 所有 MODIFIED_SOURCE 模块（仅 deep）

无需分析:
 - DELETED / RENAMED / STALE_PARENT / MODIFIED_CLAUDE_MD
```

#### Step U2.2: 并发 Agent 分析

**ADDED 模块分析**（同 `arc:init:full`）：

```typescript
// oracle 分析
Task(
 subagent_type: "oracle",
 load_skills: ["arc:init:update"],
 run_in_background: true,
 description: "oracle 架构分析 - 新模块",
 prompt: `分析以下新增模块的架构...

 模块列表: <added_modules>
 项目路径: <project_path>
 输出到: <output_dir>/agents/oracle/analysis-update.md`
)

// deep 分析
Task(
 category: "deep",
 load_skills: ["arc:init:update"],
 run_in_background: true,
 description: "deep 工程分析",
 prompt: `...`
)

// momus 分析
Task(
 subagent_type: "momus",
 load_skills: ["arc:init:update"],
 run_in_background: true,
 description: "momus DX 分析",
 prompt: `...`
)
```

**MODIFIED_KEY 模块分析**（2 Agent）：

```typescript
// 仅 deep + momus，跳过 oracle
Task(category: "deep", ...),
Task(subagent_type: "momus", ...)
```

**MODIFIED_SOURCE 模块分析**（1 Agent）：

```typescript
// 仅 deep，shallow depth
Task(
 category: "deep",
 load_skills: ["arc:init:update"],
 run_in_background: true,
 description: "deep 浅层分析 - 源码变更",
 prompt: `以下模块仅源码变更，进行浅层分析...

 模块列表: <modified_source_modules>
 分析深度: shallow（仅分析变更文件）`
)
```

#### Step U2.3: 跳过条件

```
IF 无 ADDED 且无 MODIFIED_* 模块:
 - 跳过 Phase U2
 - 直接进入 Phase U3（仅做文件操作）

IF skip_agents=true:
 - 跳过 Phase U2
 - 使用现有快照数据直接生成
```

---

### Phase U3: 增量生成（Incremental Generation）

**目标**：更新/创建/删除受影响的 CLAUDE.md 文件。

#### Step U3.1: 处理 DELETED 模块

1. 删除模块目录下的 CLAUDE.md
2. 记录删除操作到变更日志

#### Step U3.2: 处理 RENAMED 模块

1. 移动 CLAUDE.md 到新路径
2. 更新 CLAUDE.md 内部的路径引用
3. 更新面包屑导航

#### Step U3.3: 处理 ADDED 模块

1. 使用 Agent 分析结果
2. 按 `references/claude-md-schema.md` 生成新 CLAUDE.md
3. 添加 Changelog 条目（`<timestamp> arc:init:update 新增模块索引`）

#### Step U3.4: 处理 MODIFIED 模块（合并更新）

**CLAUDE.md 合并策略**：

```
1. 读取现有 CLAUDE.md → 解析为 section map
2. 识别保留区域:
 - <!-- user-start --> ... <!-- user-end --> 标记区域
 - Changelog 章节（仅追加）
 - 自定义 section（不在 schema 定义中的）

3. 识别更新区域:
 - 根据变更类型决定更新哪些 section:

 MODIFIED_KEY:
 - 模块职责（如有新分析）
 - 关键依赖（manifest 变化）
 - 运行与开发（scripts 变化）

 MODIFIED_SOURCE:
 - 架构图（结构变化时）
 - 关联文件列表（总是更新）

 MODIFIED_CLAUDE_MD:
 - 跳过（用户手动编辑，不覆盖）

4. 执行合并:
 - 保留区域: 原样保留
 - 更新区域: 用 Agent 分析结果替换
 - Changelog: 追加新条目

5. 写入合并后的 CLAUDE.md
```

#### Step U3.5: 更新祖先 CLAUDE.md

对所有 `STALE_PARENT` 模块：

1. **模块索引表格**：
 - ADDED: 添加新条目
 - DELETED: 删除条目
 - RENAMED: 更新路径

2. **Mermaid 结构图**：
 - ADDED: 添加节点 + click 链接
 - DELETED: 删除节点
 - RENAMED: 更新节点 ID

3. **Changelog**：
 - 追加增量更新条目

> 注意：仅更新受影响的章节，不重新生成整个 CLAUDE.md。

#### Step U3.6: 更新指纹文件

生成完成后：

1. 重新计算所有变更模块的指纹
2. 更新 `module-fingerprints.json`
3. 更新 `git_ref` 为当前 HEAD

---

### Phase U4: 验证与报告（Validation & Report）

**目标**：验证更新结果，生成增量报告。

#### Step U4.1: 选择性验证

仅验证变更/新增的 CLAUDE.md 文件：

- 结构校验（必需章节齐全）
- 表格校验（列数对齐）
- 引用校验（mermaid click + breadcrumb）

#### Step U4.2: 祖先一致性验证

验证祖先 CLAUDE.md 的更新是否正确：

- 模块索引条目与实际目录对应
- Mermaid 节点与索引一致
- 所有 click 链接有效

#### Step U4.3: 更新快照

更新 `project-snapshot.md`：

```markdown
## 快照元数据
- **生成时间**：<更新后 timestamp>
- **项目路径**：<unchanged>
- **Git Ref**：<new HEAD>
- **更新类型**：incremental
- **变更模块数**：N
```

#### Step U4.4: 生成更新汇总

更新 `summary.md`：

```markdown
# 增量更新汇总

## 本次更新统计
- 更新时间: <timestamp>
- 基线版本: <old_git_ref>
- 当前版本: <new_git_ref>

## 变更模块
| 模块 | 变更类型 | Agent 调用 | CLAUDE.md 操作 |
|------|----------|------------|----------------|

## 资源节省
- 跳过扫描目录: N 个
- 跳过 Agent 分析: M 个模块
- 保留手动编辑: K 处

## 校验结果
- 结构校验: ✅
- 表格校验: ✅
- 引用校验: ✅
```

---

## Artifacts & Paths

```
<project_path>/.arc/init/
├── context/
│ ├── project-snapshot.md # 更新元数据
│ ├── generation-plan.md # 更新模块清单
│ ├── module-fingerprints.json # 更新指纹基线
│ └── updates/ # 新增目录
│ └── update-<timestamp>.md # 增量报告
├── agents/
│ ├── oracle/
│ │ └── analysis-update.md # 新增：本次分析
│ ├── deep/
│ │ └── analysis-update.md
│ └── momus/
│ └── analysis-update.md
└── summary.md # 更新
```

---

## 变更类型处理矩阵

| 变更类型 | Agent 分析 | CLAUDE.md 操作 | 祖先更新 |
|----------|------------|----------------|----------|
| `ADDED` | oracle + deep + momus | 新建完整文件 | 索引 + mermaid |
| `DELETED` | — | 删除文件 | 索引 + mermaid |
| `RENAMED` | — | 移动 + 路径更新 | 索引 + mermaid |
| `MODIFIED_KEY` | deep + momus | 合并更新 | 可能需要 |
| `MODIFIED_SOURCE` | deep (shallow) | 合并更新 | 通常不需要 |
| `MODIFIED_CLAUDE_MD` | — | 跳过 | 不需要 |
| `STALE_PARENT` | — | 索引 + mermaid 更新 | — |
| `UNCHANGED` | — | 跳过 | 不需要 |

---

## 错误处理

| 情况 | 处理 |
|------|------|
| fingerprints 不存在 | 提示先运行 `arc:init`，退出 |
| git 不是仓库 | 提示需要 git 仓库，退出 |
| 基线 git_ref 不存在 | 回退到文件时间戳对比 |
| Agent 超时 | 继续用已完成的分析，标记超时模块 |
| 合并冲突 | 保留用户编辑，标记需要人工检查 |

---

## 与其他 Skill 的关系

```
arc:init (调度器)
 ├── arc:init:full (全量初始化) → 生成 module-fingerprints.json
 └── arc:init:update (增量更新) → 消费 module-fingerprints.json
```

**调用链**：
1. 用户首次运行 `/arc:init` → 自动路由到 `arc:init:full`
2. 后续运行 `/arc:init` → 自动路由到 `arc:init:update`（如 fingerprints 存在）
3. 用户显式 `/arc:init:full` → 强制全量
4. 用户显式 `/arc:init:update` → 强制增量
