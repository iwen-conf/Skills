---
name: "arc:ip-docs"
description: "面向软件项目的知识产权申请文档写作助手。采用多Agent协作模式(oracle/deep/momus)基于项目上下文与审查结论,辅助撰写软件著作权与发明专利申请文档草稿,包括说明书、技术交底书、权利要求草案与附图文字说明。"
---

# arc:ip-docs — 专利/软著文档写作

## Overview

`arc:ip-docs` 采用**多Agent协作模式**专注于文档写作,不做可行性裁决。它消费 `arc:ip-audit` 的审查结果,并结合项目代码上下文输出可编辑申请文档草稿。

**核心能力**:
- 三Agent并发独立起草(oracle技术方案/deep实现细节/momus用户文档)
- 交叉审阅机制确保术语一致性与技术准确性
- 证据驱动的文档写作(每个技术描述可回溯到代码)
- 结构化草稿输出(软著/专利分轨产出)

## Mandatory Linkage(不可单打独斗)

1. 默认先读取 `arc:ip-audit` 产物:`handoff/ip-drafting-input.json`。
2. 若交接文件缺失,优先提示先执行 `arc:ip-audit`;仅在用户明确要求时做最小化兜底草稿。
3. 文档内容必须回连项目上下文:`CLAUDE.md` + `ace-tool` 证据。
4. 对术语冲突或技术路线不明确,串联 `arc:deliberate` 做定稿前校正。

## When to Use

- 已完成审查评估,开始准备软著/专利申请文档。
- 需要撰写软著说明书、代码材料说明、专利技术交底书。
- 需要形成权利要求草案和附图文字说明供代理人加工。

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `project_path` | string | 是 | 目标项目根目录绝对路径 |
| `project_name` | string | 否 | 默认从路径推导 |
| `audit_case_dir` | string | 否 | `arc:ip-audit` 输出目录;默认 `<project_path>/.arc/ip-audit/<project-name>/` |
| `software_name` | string | 否 | 软件名称,未提供则从交接文件读取 |
| `applicant_name` | string | 否 | 申请主体名称 |
| `target_docs` | enum | 否 | `copyright` / `patent` / `both`,默认 `both` |
| `output_dir` | string | 否 | 默认 `<project_path>/.arc/ip-docs/<project-name>/` |

## Dependencies

- **arc:ip-audit**(强推荐):作为主输入。
- **ace-tool MCP**(必须):校正文档中的技术细节与代码证据。
- **arc:init**(推荐):复用模块索引,减少重复扫描。
- **Task API**(必须):调度 `oracle` / `deep` / `momus` 三Agent协作。

## Context Priority(强制)

1. `audit_case_dir/handoff/ip-drafting-input.json`
2. `.arc/ip-docs/<project>/context/doc-context.md`
3. 项目 `CLAUDE.md`
4. `ace-tool` 源码补扫

## Critical Rules

1. **不编造**:任何技术细节必须能回溯到代码或审查交接信息。
2. **术语一致**:同一对象全文统一称谓,不得自由替换同义词。
3. **结构完整**:交底书、权利要求、说明文档必须按模板章节完整输出。
4. **草稿定位**:输出为"可编辑申请草稿",不得声称最终法律文本。
5. **双轨拆分**:软著材料与专利材料分目录产出,不混写。
6. **多Agent协作**:必须使用oracle/deep/momus三Agent并发起草+交叉审阅。

## Multi-Agent Architecture

### Agent角色分工

| Agent | 角色定位 | 起草内容 | 输出文件 |
|-------|---------|---------|---------|
| **oracle** (subagent) | 技术方案描述专家 | 架构设计、技术方案、系统流程、专利技术交底书核心部分 | `agents/oracle/technical-description.md` |
| **deep** (category) | 实现细节专家 | 代码实现、算法细节、性能优化、技术效果量化、软著技术交底 | `agents/deep/implementation-details.md` |
| **momus** (subagent) | 用户文档专家 | 用户手册、操作说明、功能描述、软著申请摘要、权利要求书 | `agents/momus/user-documentation.md` |

### 协作流程

**Phase 1: 读取交接与上下文** → **Phase 2: 并发独立起草** → **Phase 3: 交叉审阅** → **Phase 4: 定稿文档草稿**

### 文件系统通信

```
<project_path>/.arc/ip-docs/<project-name>/
├── context/
│   ├── doc-context.md (共享输入)
│   └── handoff-input.json (arc:ip-audit交接)
├── agents/
│   ├── oracle/
│   │   ├── technical-description.md (独立起草)
│   │   └── review.md (审阅其他Agent)
│   ├── deep/
│   │   ├── implementation-details.md
│   │   └── review.md
│   └── momus/
│       ├── user-documentation.md
│       └── review.md
├── convergence/
│   ├── terminology-alignment.md (术语统一)
│   └── final-review.md (最终审阅)
├── copyright/
│   ├── software-summary.md (软著摘要)
│   ├── manual-outline.md (操作说明书)
│   └── source-code-package-notes.md (代码材料说明)
├── patent/
│   ├── disclosure-draft.md (技术交底书)
│   ├── claims-draft.md (权利要求草案)
│   └── drawings-description.md (附图说明)
└── reports/
    └── doc-writing-log.md (写作日志)
```

## Instructions

### Phase 1: 读取审查交接与项目上下文

**Step 1.1: 读取交接文件**
1. 读取 `<audit_case_dir>/handoff/ip-drafting-input.json`。
2. 提取:
   - 目标资产清单(IPA-001, IPA-002...)
   - 软著/专利可行性评分
   - 优先级建议
   - 风险提示
   - 关键技术点与证据路径
   - 新字段: `format_compliance`、`program_product_recommended`、`fee_reduction`、`app_e_copyright`

**Step 1.2: 生成文档上下文**
使用 `ace-tool` 二次核对关键技术描述,生成 `context/doc-context.md`:
- 核心技术方案描述(架构图、流程图)
- 关键算法实现(代码片段+注释)
- 技术效果量化数据(性能指标、测试结果)
- 用户功能描述(UI截图、操作流程)
- 格式/命名基线: 软件全称/简称/版本、页眉页脚示例、截图命名要求、代码样本页数与行数要求

**Step 1.3: 脚手架生成**
```bash
python ip-docs/scripts/scaffold_drafting_case.py \
  --project-path <project_path> \
  --project-name <project_name>
```

### Phase 2: 多Agent并发独立起草

**并发启动三Agent**(在同一消息中):

```typescript
// Oracle: 技术方案描述
Task(
  subagent_type="oracle",
  load_skills=["arc:ip-docs"],
  run_in_background=true,
  description="Oracle起草技术方案描述",
  prompt=`
[TASK]: 起草技术方案描述与专利技术交底书核心部分

[EXPECTED OUTCOME]:
- 生成 agents/oracle/technical-description.md,包含:
  1. 技术背景与现有技术问题
  2. 技术方案总体架构(Mermaid图+文字描述)
  3. 系统流程与数据流(流程图+文字描述)
  4. 技术创新点描述(对比现有技术)
  5. 每个描述必须引用代码证据(file:line)
  6. 技术三要素表(技术问题/技术手段/技术效果)
  7. 程序产品权利要求骨架(方法+系统/装置+计算机程序产品+存储介质)
  8. OA 可能性清单(客体/创造性拼凑/超范围)

[REQUIRED TOOLS]: ace-tool(代码搜索), Read(读取context/), Write(写入agents/oracle/)

[MUST DO]:
- 读取 context/doc-context.md 和 context/handoff-input.json
- 使用 ace-tool 验证技术方案描述的准确性
- 技术术语必须统一(建立术语表)
- 架构图必须使用Mermaid格式
- 每个技术创新点必须对比现有技术(引用handoff中的外部参考)
- 技术描述必须面向专利审查员(清晰、完整、可实施)
- 标注程序产品可行性与权利要求组合建议

[MUST NOT DO]:
- 不得编造技术细节
- 不得使用模糊表述("可能"、"大概"、"一般")
- 不得在本阶段读取其他Agent输出(Phase 3才交叉审阅)
- 不得混淆软著与专利的描述风格

[CONTEXT]: 项目路径 <project_path>, 工作目录 .arc/ip-docs/<project-name>/
`
)

// Deep: 实现细节与技术效果
Task(
  category="deep",
  load_skills=["arc:ip-docs"],
  run_in_background=true,
  description="Deep起草实现细节与技术效果",
  prompt=`
[TASK]: 起草代码实现细节、算法描述与技术效果量化

[EXPECTED OUTCOME]:
- 生成 agents/deep/implementation-details.md,包含:
  1. 核心算法实现(伪代码+关键代码片段)
  2. 数据结构设计(类图+字段说明)
  3. 性能优化措施(具体实现+效果对比)
  4. 技术效果量化(性能指标、测试数据、对比表格)
  5. 软著代码材料说明(建议提交的代码区段)
  6. 可提交代码样本清单(文件+起止行、预计页数、脱敏要求)
  7. 性能/对比数据表模板,缺失则输出“需补测指标”列表

[REQUIRED TOOLS]: ace-tool(代码搜索), Read(读取context/), Write(写入agents/deep/)

[MUST DO]:
- 读取 context/doc-context.md 和 context/handoff-input.json
- 使用 ace-tool 提取关键代码片段(3-20行)
- 算法描述必须可实施(伪代码+注释)
- 技术效果必须量化(具体数值+对比基准)
- 代码片段必须脱敏(移除敏感信息)
- 每个实现细节必须引用具体文件路径
- 生成提交代码页数估算与样本首尾行标注
- 若性能/对比数据缺失,列出需补测的指标与基准

[MUST NOT DO]:
- 不得编造性能数据
- 不得泄露商业机密或敏感信息
- 不得在本阶段读取其他Agent输出
- 不得使用完整源码(仅关键片段)

[CONTEXT]: 项目路径 <project_path>, 工作目录 .arc/ip-docs/<project-name>/
`
)

// Momus: 用户文档与功能描述
Task(
  subagent_type="momus",
  load_skills=["arc:ip-docs"],
  run_in_background=true,
  description="Momus起草用户文档与功能描述",
  prompt=`
[TASK]: 起草用户手册、操作说明与软著申请摘要

[EXPECTED OUTCOME]:
- 生成 agents/momus/user-documentation.md,包含:
  1. 软件功能总览(面向用户的功能列表)
  2. 操作说明书提纲(安装、配置、使用、故障排除)
  3. 软著申请摘要(300-500字,面向版权局审查员)
  4. 专利权利要求草案(独立权利要求+从属权利要求)
  5. 用户界面描述(UI截图+操作流程)
  6. 页眉/页脚/命名一致性检查提示(引用 format 基线)
  7. 签章页、非职务开发保证书、开源声明占位提示
  8. 权利要求四件套(方法+系统/装置+计算机程序产品+存储介质)句式占位

[REQUIRED TOOLS]: ace-tool(搜索文档), Read(读取context/), Write(写入agents/momus/)

[MUST DO]:
- 读取 context/doc-context.md 和 context/handoff-input.json
- 使用 ace-tool 搜索项目文档(README/docs/)
- 功能描述必须面向非技术用户(清晰、易懂)
- 操作说明必须完整(覆盖安装到使用全流程)
- 软著摘要必须符合版权局格式要求
- 权利要求必须符合专利法格式(一句话、层次清晰)
- 校验软件名称与版本一致性并在文档中标注
- 插入截图占位说明(需与软件名称一致)
- 标记签章页/保证书/开源声明待补充项

[MUST NOT DO]:
- 不得使用技术黑话(面向用户)
- 不得编造功能(必须有代码支撑)
- 不得在本阶段读取其他Agent输出
- 不得混淆软著摘要与专利摘要

[CONTEXT]: 项目路径 <project_path>, 工作目录 .arc/ip-docs/<project-name>/
`
)
```

**等待三Agent完成**,使用 `background_output(task_id="...")` 收集结果。

### Phase 3: 交叉审阅

**强制审阅机制**(每个Agent必须审阅其他两个Agent):

```typescript
// Oracle审阅Deep和Momus
Task(
  session_id="<oracle_session_id>", // 复用Phase 2的session
  load_skills=["arc:ip-docs"],
  run_in_background=false,
  description="Oracle交叉审阅Deep和Momus",
  prompt=`
[TASK]: 审阅Deep和Momus的文档,确保技术准确性与术语一致性

[EXPECTED OUTCOME]:
- 生成 agents/oracle/review.md,包含:
  1. 对Deep实现细节的审阅(技术准确性、架构一致性)
  2. 对Momus用户文档的审阅(功能描述准确性、术语一致性)
  3. 术语冲突列表(同一对象的不同称谓)
  4. 技术描述错误列表(与代码不符的描述)
  5. 修正建议(具体修改方案)
  6. 程序产品权利要求完整性与技术三要素对应关系检查
  7. 格式合规(行数/页眉页脚)与命名统一性提示

[MUST DO]:
- 读取 agents/deep/implementation-details.md 和 agents/momus/user-documentation.md
- 使用 ace-tool 验证争议技术描述
- 建立术语对照表(统一称谓)
- 指出技术描述与代码不符之处
- 提出具体修正建议(不是简单指出问题)
- 审阅格式/命名合规与程序产品权利要求覆盖性

[MUST NOT DO]:
- 不得简单同意对方观点
- 不得无证据审阅

[CONTEXT]: 工作目录 .arc/ip-docs/<project-name>/
`
)

// Deep审阅Oracle和Momus (同理)
// Momus审阅Oracle和Deep (同理)
```

**收集审阅结果**,生成 `convergence/terminology-alignment.md` 和 `convergence/final-review.md`。

### Phase 4: 定稿文档草稿

**Step 4.1: 术语统一**

根据审阅报告,生成 `convergence/terminology-alignment.md`:

| 对象 | Oracle称谓 | Deep称谓 | Momus称谓 | 统一术语 |
|------|-----------|---------|----------|---------|
| 核心算法 | 智能调度算法 | 任务分配算法 | 自动调度功能 | 智能任务调度算法 |

**Step 4.2: 生成软著文档**(如 `target_docs` 包含 `copyright`)

使用脚本生成标准化文档:
```bash
python ip-docs/scripts/render_ip_documents.py \
  --case-dir .arc/ip-docs/<project-name>/ \
  --handoff-json <audit_case_dir>/handoff/ip-drafting-input.json \
  --target-docs copyright
```

生成文件:
- `copyright/software-summary.md` (软著申请摘要,整合Momus起草+Oracle/Deep审阅)
- `copyright/manual-outline.md` (操作说明书提纲,整合Momus起草+术语统一)
- `copyright/source-code-package-notes.md` (代码材料说明,整合Deep起草+Oracle审阅)
  - 需显式记录软件全称/简称/版本,在写作日志标记名称一致性已核对

**Step 4.3: 生成专利文档**(如 `target_docs` 包含 `patent`)

```bash
python ip-docs/scripts/render_ip_documents.py \
  --case-dir .arc/ip-docs/<project-name>/ \
  --handoff-json <audit_case_dir>/handoff/ip-drafting-input.json \
  --target-docs patent
```

生成文件:
- `patent/disclosure-draft.md` (技术交底书,整合Oracle技术方案+Deep实现细节+术语统一)
- `patent/claims-draft.md` (权利要求草案,整合Momus起草+Oracle/Deep审阅)
  - 默认包含: 1条独立方法 + 1条系统/装置 + 1条计算机程序产品 + 1条存储介质; 从属权利要求引用性能/数据流/模块参数差异点
- `patent/drawings-description.md` (附图说明,整合Oracle架构图+Deep数据流图+Momus UI截图)

**Step 4.4: 生成写作日志**

生成 `reports/doc-writing-log.md`:
- 输入来源(handoff文件路径、CLAUDE.md路径)
- 假设与推断(未在代码中找到但合理推断的内容)
- 待人工补充项(需要申请人提供的信息)
- Agent协作记录(各Agent起草内容、审阅意见、术语统一决策)
- 费减材料清单、电子版权选项、格式合规检查结果、缺失性能数据/截图列表

## Scripts

优先使用脚本生成标准化骨架:

```bash
# 生成工作目录骨架
python ip-docs/scripts/scaffold_drafting_case.py \
  --project-path <project_path> \
  --project-name <project_name>

# 渲染最终文档
python ip-docs/scripts/render_ip_documents.py \
  --case-dir <output_dir> \
  --handoff-json <audit_case_dir>/handoff/ip-drafting-input.json \
  --target-docs both
```

## Artifacts

默认输出目录:`<project_path>/.arc/ip-docs/<project-name>/`

- `context/doc-context.md` (文档上下文)
- `context/handoff-input.json` (arc:ip-audit交接)
- `agents/oracle/technical-description.md` (Oracle独立起草)
- `agents/oracle/review.md` (Oracle审阅)
- `agents/deep/implementation-details.md` (Deep独立起草)
- `agents/deep/review.md` (Deep审阅)
- `agents/momus/user-documentation.md` (Momus独立起草)
- `agents/momus/review.md` (Momus审阅)
- `convergence/terminology-alignment.md` (术语统一)
- `convergence/final-review.md` (最终审阅)
- `copyright/software-summary.md` (软著摘要)
- `copyright/manual-outline.md` (操作说明书)
- `copyright/source-code-package-notes.md` (代码材料说明)
- `patent/disclosure-draft.md` (技术交底书)
- `patent/claims-draft.md` (权利要求草案)
- `patent/drawings-description.md` (附图说明)
- `reports/doc-writing-log.md` (写作日志)

## Quick Reference

| 输出物 | 用途 |
|------|------|
| `disclosure-draft.md` | 专利技术交底书草稿 |
| `claims-draft.md` | 权利要求草案 |
| `software-summary.md` | 软著申请摘要 |
| `manual-outline.md` | 软著操作说明书提纲 |
| `source-code-package-notes.md` | 软著代码材料说明 |
| `doc-writing-log.md` | 写作日志(记录假设与待补充项) |

## Failure Recovery

- **Agent超时(>10min)**: 询问用户是否继续等待或降级为双Agent起草。
- **Agent起草缺失**: 用其他两个Agent填补,标注"双源起草"。
- **术语冲突无法解决**: 在文档中列出冲突术语,标注"需人工裁决"。
- **交接文件缺失**: 提示用户先执行 `arc:ip-audit`,或在用户明确要求时做最小化兜底草稿(标注"未经审查评估")。
