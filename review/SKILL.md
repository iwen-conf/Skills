---
name: "arc:review"
description: "按企业级七维度框架（ISO/IEC 25010 + TOGAF）深度评审软件项目，三模型对抗式分析，输出诊断报告与改进路线图"
---

# 企业级项目评审（三模型对抗式）

## Overview

赋予 Agent "企业级软件评审专家"的能力。通过 Claude、Codex、Gemini 三模型各自独立按七维度评估项目，再互相反驳对方的评分和发现，最终收敛出一份可交付的诊断报告与改进路线图。

七维度评估框架参考 ISO/IEC 25010 软件质量模型、TOGAF 企业架构框架及现代软件工程最佳实践（详见 `references/dimensions.md`）。

流程分为四个阶段：

1. **项目侦察**：扫描代码结构、依赖、技术栈 → 生成项目快照
2. **独立评估**：三模型并发按七维度独立评估 → 各出 7 份维度分析
3. **交叉反驳**：三模型互相反驳评分和发现 → 各出反驳报告
4. **收敛报告**：Claude 主进程聚合 → 最终诊断报告 + 评分卡 + 改进建议

## Context Budget（避免 Request too large）

- 项目侦察阶段不要把整个代码库粘贴到对话中；只提取关键文件路径、目录结构、依赖清单、配置摘要。
- 每个维度分析控制在 200-500 行内；过长时拆分到独立文件并引用路径。
- 代码证据只引用关键片段（5-20 行），不粘贴完整文件。

## When to Use

- 项目启动前的技术尽职调查（Tech Due Diligence）
- 迭代里程碑后的架构健康度复盘
- 架构升级/重构前的现状评估与风险分析
- 新团队接手遗留项目前的全面摸底
- 技术选型决策需要多视角验证

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `project_path` | string | 是 | 待评审项目的根目录绝对路径 |
| `project_name` | string | 否 | 项目名称（用于目录命名）；未提供则从 path 推导 |
| `scope_dimensions` | array | 否 | 评估维度范围，默认全 7 维度；可选 `["architecture", "security", "code-quality", "business", "devops", "team", "tech-debt"]` |
| `depth_level` | string | 否 | 评估深度：`"quick"` / `"standard"` / `"deep"`；默认 `"standard"` |
| `focus_areas` | array | 否 | 特别关注的领域（如 `["security", "tech-debt"]`），会在这些维度进行更深入的分析 |
| `output_dir` | string | 否 | 输出根目录；默认 `<project_path>/.arc/review/` |

## Dependencies

* **ace-tool (MCP)**: 必须。用于语义搜索项目代码结构、实现模式、CLAUDE.md 索引。
* **Exa MCP**: 推荐。用于搜索项目依赖的行业标准、最佳实践、安全漏洞信息。
* **codeagent-wrapper**: 必须。用于调用 Codex/Gemini 执行独立评估与反驳。

## 模型调用方式

同 arc:deliberate：

| 模型 | 调用方式 | 原因 |
|------|---------|------|
| **Claude** | **Task 工具（subagent）** | Claude Code 不能嵌套调用自身 |
| **Codex** | `codeagent-wrapper --backend codex` | 独立进程，CLI 调用 |
| **Gemini** | `codeagent-wrapper --backend gemini` | 独立进程，CLI 调用 |

## Critical Rules（核心铁律）

0. **Markdown 格式校验（最高优先级）**
   - 所有产出 Markdown 的表格必须列数对齐（表头、分隔行、数据行一致）。
   - 生成后必须校验，校验失败必须修复。

1. **只读不改代码（Read-Only）**
   - **严禁**修改、删除、新增被评审项目的任何源代码或配置文件。
   - **允许**在 `.arc/review/` 目录下创建评审产出文件。

2. **必须引用代码证据**
   - 每个评估结论必须附带具体的文件路径 + 行号或代码片段。
   - 格式：`file_path:line_number` 或引用 5-20 行关键代码块。
   - **禁止**无证据的主观断言。

3. **对标企业最佳实践**
   - 每个维度的评估必须参考 `references/dimensions.md` 中的检查项。
   - 安全维度必须对标 OWASP Top 10。
   - 架构维度必须考虑 ISO/IEC 25010 质量模型。

4. **区分"观察"与"建议"**
   - 观察（Observation）= 事实，必须有代码证据。
   - 建议（Recommendation）= 意见，必须说明理由和权衡。
   - 不得混淆二者。

5. **评分必须有依据**
   - 每个维度的 0-10 分评分必须列出扣分点和加分点。
   - 三模型反驳阶段可以挑战对方的评分并给出修正。

6. **Resource Control & Cleanup**
   - 所有 MCP 搜索和文件读取必须有上限，禁止无限递归扫描。
   - `depth_level="quick"` 时限制每维度最多分析 5 个关键文件。

## 七维度评估框架

详见 `references/dimensions.md`。每个维度包含检查项、评估要点和代码证据来源。

| # | 维度 | 英文标识 | 核心问题 |
|---|------|---------|---------|
| 1 | 架构设计与技术演进性 | architecture | 架构是否支持长期演进？ |
| 2 | 安全合规与权限管控 | security | 安全与权限是否可靠？ |
| 3 | 代码质量与工程规范 | code-quality | 代码质量与维护性如何？ |
| 4 | 业务价值与需求契合度 | business | 是否解决实际业务问题？ |
| 5 | 运维可观测性与部署交付 | devops | 运维是否稳健可观测？ |
| 6 | 团队效能与协作 | team | 团队是否可持续维护？ |
| 7 | 技术债务与演进阻力 | tech-debt | 系统"沉重程度"如何？ |

## Instructions（执行流程）

### Phase 1: 项目侦察（Reconnaissance）

**目标**：收集被评审项目的基础信息，构建项目全景快照。

#### Step 1.1: 扫描项目结构

1. **使用 ace-tool MCP** 搜索项目代码结构、技术栈、核心模块
2. **读取项目 CLAUDE.md 层级索引**（如存在），快速理解项目愿景、模块关系
3. **收集基础元数据**：
   - 目录结构（顶级 + 关键子目录）
   - 技术栈推导（从 go.mod / package.json / Cargo.toml / requirements.txt 等）
   - 依赖清单与版本
   - 代码规模（文件数、主要语言分布）
   - CI/CD 配置（.github/workflows、Makefile、Dockerfile 等）
   - 测试结构（test 目录、覆盖率配置）

#### Step 1.2: 搜索行业标准

使用 **Exa MCP** 搜索：
- 项目使用的核心框架/库的最新安全公告
- 行业同类项目的架构最佳实践
- 技术栈相关的已知漏洞或 deprecation 信息

#### Step 1.3: 生成项目快照

产出 `context/project-snapshot.md`，包含：

```markdown
# 项目快照: <project_name>

## 基本信息
* **路径**: <project_path>
* **主要语言**: <languages>
* **框架/库**: <frameworks>
* **评审时间**: <timestamp>
* **评审深度**: <depth_level>

## 目录结构
<tree output, top 3 levels>

## 技术栈
<dependency list with versions>

## 代码规模
<file counts by language>

## CI/CD
<pipeline summary>

## 测试结构
<test directory layout, coverage config>
```

---

### Phase 2: 三模型独立评估（Independent Assessment）

**目标**：三模型并发，各自按七维度独立评估项目。

#### Step 2.1: 三模型并发评估

**CRITICAL**: 三个模型必须在同一消息中并发发起（`run_in_background: true`）。

每个模型读取 `context/project-snapshot.md` 后，对全部 7 个维度各自独立评估。

**Claude 评估**（subagent）:
```
Task({
  description: "Claude 七维度评估",
  subagent_type: "general-purpose",
  run_in_background: true,
  mode: "bypassPermissions",
  prompt: "你是企业级软件评审专家。

读取 <output_dir>/context/project-snapshot.md 了解项目概况。
使用 ace-tool MCP 搜索项目代码，按七维度逐一评估。
评估框架参考 <skills_dir>/review/references/dimensions.md。

对每个维度，产出一个独立文件 <output_dir>/claude/dim-N-<name>.md，格式：

# 维度 N: <维度名>
## 评分: X/10
## 加分点
- <观察> — 证据: `file:line`
## 扣分点
- <观察> — 证据: `file:line`
## 关键发现
<详细分析，引用代码证据>
## 改进建议
<具体建议及理由>

项目路径: <project_path>
评估深度: <depth_level>"
})
```

**Codex 评估**（codeagent-wrapper）:
```bash
/Users/iluwen/.claude/bin/codeagent-wrapper --lite --backend codex - "<project_path>" <<'EOF'
ROLE_FILE: /Users/iluwen/.claude/.ccg/prompts/codex/architect.md
<TASK>
你是企业级软件评审专家，侧重后端架构、代码质量和安全。

读取 <output_dir>/context/project-snapshot.md 了解项目概况。
评估框架参考 <skills_dir>/review/references/dimensions.md。

对全部 7 个维度逐一评估，每个维度产出一个独立文件 <output_dir>/codex/dim-N-<name>.md。
格式同 Claude（评分 + 加分点 + 扣分点 + 关键发现 + 改进建议）。
必须引用代码证据（file:line）。
</TASK>
OUTPUT: 7 份维度分析文件
EOF
```

**Gemini 评估**（codeagent-wrapper）:
```bash
/Users/iluwen/.claude/bin/codeagent-wrapper --lite --backend gemini - "<project_path>" <<'EOF'
ROLE_FILE: /Users/iluwen/.claude/.ccg/prompts/gemini/architect.md
<TASK>
你是企业级软件评审专家，侧重前端交互、运维和用户体验。

读取 <output_dir>/context/project-snapshot.md 了解项目概况。
评估框架参考 <skills_dir>/review/references/dimensions.md。

对全部 7 个维度逐一评估，每个维度产出一个独立文件 <output_dir>/gemini/dim-N-<name>.md。
格式同 Claude（评分 + 加分点 + 扣分点 + 关键发现 + 改进建议）。
必须引用代码证据（file:line）。
</TASK>
OUTPUT: 7 份维度分析文件
EOF
```

#### Step 2.2: 等待完成

等待三个后台任务完成（subagent 用 TaskOutput，codeagent-wrapper 用 Bash 等待）。

---

### Phase 3: 交叉审阅与反驳（Cross-Review & Rebuttal）

**目标**：三模型互相反驳对方的评分和发现，消除盲点，校准评分。

#### Step 3.1: 三模型并发反驳

**CRITICAL**: 每个模型必须**反驳另外两个模型**的全部 7 维度评估。

每个模型必须：
1. 读取其他两个模型的全部 7 份维度分析
2. **反驳过于乐观或悲观的评分**（必须给出理由和代码证据）
3. **指出遗漏的问题或被忽略的优势**
4. **给出修正后的评分建议**

**Claude 反驳 Codex + Gemini**（subagent）:
```
Task({
  description: "Claude 交叉反驳",
  subagent_type: "general-purpose",
  run_in_background: true,
  mode: "bypassPermissions",
  prompt: "读取 Codex 和 Gemini 的全部 7 维度评估：
- <output_dir>/codex/dim-*.md
- <output_dir>/gemini/dim-*.md

逐维度反驳：
1. 指出评分过高或过低的地方（附代码证据）
2. 指出遗漏的重要问题或被忽略的优势
3. 给出你认为更合理的评分及理由

产出 <output_dir>/claude/critique.md。"
})
```

**Codex 反驳 Claude + Gemini**（codeagent-wrapper）:
- 读取 `claude/dim-*.md` 和 `gemini/dim-*.md`
- 从后端/代码质量/安全角度反驳
- 产出 `codex/critique.md`

**Gemini 反驳 Claude + Codex**（codeagent-wrapper）:
- 读取 `claude/dim-*.md` 和 `codex/dim-*.md`
- 从前端/运维/UX 角度反驳
- 产出 `gemini/critique.md`

---

### Phase 4: 收敛与报告生成（Convergence & Report）

**目标**：Claude 主进程聚合三方分析与反驳，生成最终交付物。

#### Step 4.1: 聚合评分

读取三方的维度分析 + 反驳报告，对每个维度：
1. 取三方评分的加权平均（Claude 40%、Codex 30%、Gemini 30%）
2. 根据反驳报告调整（如某方的评分被另外两方有力反驳，降低其权重）
3. 生成最终评分及依据

#### Step 4.2: 生成诊断报告

产出 `diagnostic-report.md`：

```markdown
# 项目诊断报告: <project_name>

## 执行摘要
* **综合评分**: X.X/10
* **评审时间**: <timestamp>
* **评审深度**: <depth_level>
* **关键风险**: <top 3 风险>

## 评分总览

| 维度 | 评分 | 评级 | 关键发现 |
|------|------|------|---------|
| 架构设计 | X/10 | 良好 | <一句话> |
| 安全合规 | X/10 | 警告 | <一句话> |
| ... | ... | ... | ... |

## 维度 1: 架构设计与技术演进性 (X/10)
### 观察
<事实 + 代码证据>
### 分析
<三方观点综合>
### 建议
<具体改进措施>

## 维度 2-7: ...
（结构同维度 1）

## 分歧记录
<三模型未达成共识的观点及各方理由>

## 方法论说明
<七维度框架来源、评分标准、三模型对抗机制>
```

#### Step 4.3: 生成评分卡

产出 `scorecard.md`：

```markdown
# 评分卡: <project_name>

## 七维度评分

| 维度 | Claude | Codex | Gemini | 最终 | 评级 |
|------|--------|-------|--------|------|------|
| 架构设计 | 7 | 8 | 7 | 7.3 | 良好 |
| 安全合规 | 6 | 5 | 6 | 5.6 | 合格 |
| ... | ... | ... | ... | ... | ... |

## 评级标准
| 分数 | 评级 | 含义 |
|------|------|------|
| 9-10 | 卓越 | 行业标杆 |
| 7-8 | 良好 | 符合企业级标准 |
| 5-6 | 合格 | 基本可用，有明显短板 |
| 3-4 | 警告 | 较严重问题 |
| 0-2 | 危险 | 系统性缺陷 |
```

#### Step 4.4: 生成改进建议

产出 `recommendations.md`：

```markdown
# 改进建议: <project_name>

## P0 — 立即修复（阻塞性问题）
- [ ] <建议> — 维度: <维度名>, 证据: `file:line`

## P1 — 短期改进（1-2 个迭代内）
- [ ] <建议> — 维度: <维度名>, 证据: `file:line`

## P2 — 中期规划（季度级别）
- [ ] <建议> — 维度: <维度名>

## P3 — 长期演进（半年+）
- [ ] <建议> — 维度: <维度名>
```

---

## Artifacts & Paths

```
<workdir>/.arc/review/<project-name>/
├── context/
│   └── project-snapshot.md         # 项目基本信息快照
├── claude/
│   ├── dim-1-architecture.md       # 维度 1 分析
│   ├── dim-2-security.md
│   ├── dim-3-code-quality.md
│   ├── dim-4-business-value.md
│   ├── dim-5-devops.md
│   ├── dim-6-team.md
│   ├── dim-7-tech-debt.md
│   └── critique.md                 # 交叉反驳
├── codex/
│   ├── dim-1-architecture.md
│   ├── ...（同 claude）
│   └── critique.md
├── gemini/
│   ├── dim-1-architecture.md
│   ├── ...（同 claude）
│   └── critique.md
├── diagnostic-report.md            # 最终诊断报告
├── scorecard.md                    # 评分卡
└── recommendations.md              # 改进建议（按优先级）
```

## 超时与降级

| 情况 | 处理 |
|------|------|
| 单模型超时 > 10min | 使用 AskUserQuestion 询问用户是否继续用剩余模型 |
| 某模型维度分析缺失 | 用另外两个模型的分析填补，标注"单源评估" |
| ace-tool MCP 不可用 | 降级为 Grep + Read 手动扫描关键文件 |
| depth_level="quick" | 每维度限制 5 个关键文件，跳过 Phase 3 反驳 |

## 状态反馈

```
[Arc Review] 项目: <project_name>

=== 阶段 1: 项目侦察 ===
  ├── ace-tool 扫描... [完成]
  ├── Exa 搜索行业标准... [完成]
  └── 生成项目快照... [完成]

=== 阶段 2: 独立评估 ===
  ├── Claude(subagent) 7 维度... [完成]
  ├── Codex(CLI) 7 维度... [完成]
  └── Gemini(CLI) 7 维度... [完成]

=== 阶段 3: 交叉反驳 ===
  ├── Claude 反驳 Codex+Gemini... [完成]
  ├── Codex 反驳 Claude+Gemini... [完成]
  └── Gemini 反驳 Claude+Codex... [完成]

=== 阶段 4: 收敛报告 ===
  ├── 聚合评分... [完成]
  ├── 诊断报告... [完成]
  ├── 评分卡... [完成]
  └── 改进建议... [完成]
```

## Quick Reference

| 阶段 | 步骤 | 输出路径 |
|------|------|---------|
| 项目侦察 | MCP 扫描 → 快照 | `context/project-snapshot.md` |
| 独立评估 | 三模型×7 维度 | `(claude\|codex\|gemini)/dim-N-<name>.md` |
| 交叉反驳 | 三模型互相反驳 | `(claude\|codex\|gemini)/critique.md` |
| 收敛报告 | 聚合 → 诊断 + 评分 + 建议 | `diagnostic-report.md`, `scorecard.md`, `recommendations.md` |

## 调用方式速查

| 角色 | 调用方式 | 并发支持 |
|------|---------|---------|
| Claude | `Task({ subagent_type: "general-purpose", run_in_background: true })` | subagent 后台 |
| Codex | `Bash({ command: "codeagent-wrapper --lite --backend codex ...", run_in_background: true })` | Bash 后台 |
| Gemini | `Bash({ command: "codeagent-wrapper --lite --backend gemini ...", run_in_background: true })` | Bash 后台 |
| Claude（聚合/报告） | 主进程直接处理 | — |
