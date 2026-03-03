---
name: "arc:ip-audit"
description: "面向软件项目的知识产权可行性审查与风险评估。采用多Agent协作模式(oracle/deep/writing)深度分析项目,输出专利/软件著作权申请可行性报告、优先级矩阵与申请准备度清单。用于申请前评估、融资前尽调、立项前合规审查。"
---

# arc:ip-audit — 项目专利/软著审查报告

## Overview

`arc:ip-audit` 采用**多Agent协作模式**,基于项目真实代码与架构上下文,评估该项目进行**软件著作权**与**发明专利**申请的可行性、风险与优先级。

**核心能力**:
- 三Agent并发独立评估(oracle架构视角/deep工程视角/writing文档与合规视角)
- 交叉反驳机制消除盲点与过度乐观/悲观评估
- 证据驱动的可行性评分与风险矩阵
- 结构化交接文件输出给 `arc:ip-docs`

本技能不直接产出正式申请文书,审查结论将以结构化交接文件输出,供 `arc:ip-docs` 继续完成文档撰写。

## Mandatory Linkage(不可单打独斗)

必须按以下链路协作:

1. 优先读取 `arc:init` 产出的项目 `CLAUDE.md` 索引。
2. 若存在 `arc:review` 产物,复用其架构深度和风险结论。
3. 需求模糊时先调用 `arc:refine` 明确产品边界与业务目标。
4. 高争议技术路线可串联 `arc:deliberate` 做多 Agent 论证。
5. 审查输出交接给 `arc:ip-docs`,不得在本技能内混做文书写作。

## When to Use

- 需要判断项目是否适合申请软著/专利。
- 需要在融资、投标、上架前做知识产权可行性尽调。
- 需要评估项目中的创新点是否达到专利门槛。
- 需要明确申请顺序、风险点和费减可行性。

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `project_path` | string | 是 | 目标项目根目录绝对路径 |
| `project_name` | string | 否 | 项目名;默认从路径推导 |
| `software_name` | string | 否 | 申请软件名称;未提供时先使用临时名并在报告标注 |
| `applicant_type` | enum | 否 | `individual` / `enterprise` / `institution`,默认 `enterprise` |
| `business_goal` | string | 否 | 产品商业目标(用于评估申请优先级) |
| `output_dir` | string | 否 | 默认 `<project_path>/.arc/ip-audit/<project-name>/` |

## Dependencies

- **ace-tool MCP**(必须):搜索项目代码与实现证据。
- **Exa MCP**(推荐):补充现有技术/政策依据。
- **Task API**(必须):调度 `oracle` / `deep` / `writing` 三Agent协作。
- **arc:init**(强推荐):读取 `CLAUDE.md` 层级索引。
- **arc:review**(可选):复用现有评审报告。

## Context Priority(强制)

严格遵循 `.arc/context-priority-protocol.md`:

1. `.arc/ip-audit/<project>/context/project-ip-snapshot.md`(24h)
2. `.arc/review/<project>/` 已有评审报告
3. 项目 `CLAUDE.md` 层级索引(7天)
4. `ace-tool` 源码语义扫描
5. `Exa` 外部参考

若前三级信息不足,必须降级扫描并在报告写明"上下文来源与新鲜度"。

## Critical Rules

1. **只读审查**:不得修改用户源码。
2. **证据驱动**:每个结论必须绑定文件路径或模块证据。
3. **双轨独立评分**:软著可行性与专利可行性分别评分,禁止混为单结论。
4. **风险显式化**:必须列出可驳回风险、补正风险、时间风险和材料缺口。
5. **交接标准化**:必须生成 `handoff/ip-drafting-input.json` 给 `arc:ip-docs`。
6. **法律边界**:输出为工程与流程建议,不替代执业律师/专利代理人法律意见。
7. **多Agent协作**:必须使用oracle/deep/writing三Agent并发分析+交叉反驳。

## Multi-Agent Architecture

### Agent角色分工

| Agent | 角色定位 | 评估维度 | 输出文件 |
|-------|---------|---------|---------|
| **oracle** (subagent) | 架构与创新性专家 | 技术方案独创性、架构设计新颖性、现有技术差异度、专利申请可行性 | `agents/oracle/innovation-analysis.md` |
| **deep** (category) | 工程实现专家 | 代码完整性、实现充分性、技术效果可量化性、软著申请可行性 | `agents/deep/implementation-analysis.md` |
| **writing** (category) | 文档与合规分析专家 | 文档完备性、材料准备度、申请流程风险、知识产权合规性 | `agents/writing/compliance-analysis.md` |

### 协作流程

**Phase 1: 上下文采集** → **Phase 2: 并发独立评估** → **Phase 3: 交叉反驳** → **Phase 4: 综合可行性报告**

### 文件系统通信

```
<project_path>/.arc/ip-audit/<project-name>/
├── context/
│   ├── project-ip-snapshot.md (共享输入)
│   └── external-references.md (Exa搜索结果)
├── agents/
│   ├── oracle/
│   │   ├── innovation-analysis.md (独立评估)
│   │   └── critique.md (反驳其他Agent)
│   ├── deep/
│   │   ├── implementation-analysis.md
│   │   └── critique.md
│   └── writing/
│       ├── compliance-analysis.md
│       └── critique.md
├── convergence/
│   ├── round-1-summary.md (首轮综合)
│   └── final-consensus.md (最终共识)
├── analysis/
│   └── ip-assets.md (资产清单)
├── reports/
│   ├── ip-feasibility-report.md (可行性总报告)
│   └── filing-readiness-checklist.md (准备度清单)
└── handoff/
    └── ip-drafting-input.json (交接给arc:ip-docs)
```

## Instructions

### Phase 1: 上下文采集

**Step 1.1: 检查缓存与索引**
1. 检查 `.arc/ip-audit/<project>/context/project-ip-snapshot.md` 是否存在且新鲜(<24h)。
2. 读取项目 `CLAUDE.md` 层级索引(根级+核心模块级)。
3. 若存在 `.arc/review/<project>/` 评审报告,提取架构/技术债务/安全结论。

**Step 1.2: 生成项目快照**
使用 `ace-tool` 搜索以下内容并生成 `context/project-ip-snapshot.md`:
- 核心算法与性能优化实现
- 关键模块边界与系统交互
- 可作为软著提交样本的代码区段
- 技术方案描述(架构图、流程图、数据流)
- 现有技术对比(若项目文档中有)
- 格式/命名一致性基线: 软件全称/简称/版本、页眉页脚示例、截图命名一致性检查入口(供 writing 使用)

**Step 1.3: 外部参考搜索**
使用 `Exa` 搜索并生成 `context/external-references.md`:
- 同类产品专利检索(关键词:项目核心技术+领域)
- 软著申请政策与审查标准
- 专利申请门槛与常见驳回理由
- 政策锚点(标注日期/来源): 无纸化实名规则、代码/说明格式要求、App 电子版权流程、2024 "计算机程序产品" 条款、2025 费减门槛与比例

**Step 1.4: 脚手架生成**
```bash
python ip-audit/scripts/scaffold_audit_case.py \
  --project-path <project_path> \
  --project-name <project_name>
```

### Phase 2: 多Agent并发独立评估

**并发启动三Agent**(在同一消息中):

```typescript
// Oracle: 架构与创新性评估
Task(
  subagent_type="oracle",
  load_skills=["arc:ip-audit"],
  run_in_background=true,
  description="Oracle评估技术创新性与专利可行性",
  prompt=`
[TASK]: 评估项目的技术创新性与专利申请可行性

[EXPECTED OUTCOME]:
- 生成 agents/oracle/innovation-analysis.md,包含:
  1. 技术方案独创性评分(1-10分)
  2. 架构设计新颖性评分(1-10分)
  3. 现有技术差异度分析(引用context/external-references.md)
  4. 专利申请可行性评分(高/中/低)
  5. 每个评分必须附文件路径证据
  6. 技术三要素映射表(技术问题/技术手段/技术效果)
  7. 程序产品可专利化判断(是/否+依据)
  8. 权利要求组合建议(方法+系统/装置+计算机程序产品+存储介质)

[REQUIRED TOOLS]: ace-tool(代码搜索), Read(读取context/), Write(写入agents/oracle/)

[MUST DO]:
- 读取 context/project-ip-snapshot.md 和 context/external-references.md
- 使用 ace-tool 深入分析核心算法实现
- 对比现有技术,明确技术差异点
- 每个创新点必须引用具体文件路径(file:line)
- 评分必须给出量化依据
- 标注潜在 OA 风险(客体/创造性/超范围),引用政策锚点

[MUST NOT DO]:
- 不得修改项目源码
- 不得给出法律意见(仅工程评估)
- 不得混淆软著与专利评估标准
- 不得在本阶段读取其他Agent输出(Phase 3才交叉反驳)

[CONTEXT]: 项目路径 <project_path>, 工作目录 .arc/ip-audit/<project-name>/
`
)

// Deep: 工程实现评估
Task(
  category="deep",
  load_skills=["arc:ip-audit"],
  run_in_background=true,
  description="Deep评估代码完整性与软著可行性",
  prompt=`
[TASK]: 评估项目的代码完整性与软著申请可行性

[EXPECTED OUTCOME]:
- 生成 agents/deep/implementation-analysis.md,包含:
  1. 代码完整性评分(1-10分)
  2. 实现充分性评分(1-10分)
  3. 技术效果可量化性评分(1-10分)
  4. 软著申请可行性评分(高/中/低)
  5. 每个评分必须附代码证据
  6. 软著可提交代码样本清单(文件+起止行,估算满足≥50行/页的页数,脱敏/删注释建议)
  7. 技术效果量化数据表(含基准/对比,缺失则标记需补测指标)

[REQUIRED TOOLS]: ace-tool(代码搜索), Read(读取context/), Write(写入agents/deep/)

[MUST DO]:
- 读取 context/project-ip-snapshot.md
- 使用 ace-tool 分析代码结构完整性
- 评估核心功能实现充分性(是否有TODO/FIXME/未实现)
- 检查技术效果是否可量化(性能指标、测试覆盖率)
- 识别可作为软著提交的代码区段(建议3000-5000行)
- 每个结论必须引用具体文件路径
- 记录首尾样本页所在文件/行号,提示是否需脱敏/删注释
- 如缺乏性能/对比数据,输出“补测指标”列表

[MUST NOT DO]:
- 不得修改项目源码
- 不得给出法律意见
- 不得混淆软著与专利评估标准
- 不得在本阶段读取其他Agent输出

[CONTEXT]: 项目路径 <project_path>, 工作目录 .arc/ip-audit/<project-name>/
`
)

// Writing: 文档与合规分析
Task(
  category="writing",
  load_skills=["arc:ip-audit"],
  run_in_background=true,
  description="Writing评估文档完备性与申请准备度",
  prompt=`
[TASK]: 评估项目的文档完备性与知识产权申请准备度

[EXPECTED OUTCOME]:
- 生成 agents/writing/compliance-analysis.md,包含:
  1. 文档完备性评分(1-10分)
  2. 材料准备度评分(1-10分)
  3. 申请流程风险评估(高/中/低)
  4. 知识产权合规性检查(开源协议冲突、第三方依赖)
  5. 每个评分必须附证据
  6. 命名/实名一致性、页眉页脚格式检查结果
  7. 签章页、非职务开发保证书、开源声明的准备状态
  8. App 电子版权通道可行性(若目标为应用市场上架)
  9. 费减资格预判与所需证明文件清单

[REQUIRED TOOLS]: ace-tool(搜索文档), Read(读取context/), Write(写入agents/writing/)

[MUST DO]:
- 读取 context/project-ip-snapshot.md
- 使用 ace-tool 搜索项目文档(README/docs/注释)
- 检查开源依赖的许可证兼容性(package.json/requirements.txt/go.mod)
- 评估申请材料缺口(用户手册、技术文档、测试报告)
- 识别申请流程风险(命名冲突、相似软件、审查周期)
- 每个风险必须给出缓解建议
- 依据政策锚点检查无纸化实名、格式合规、费减条件、电子版权可替代性

[MUST NOT DO]:
- 不得修改项目源码
- 不得给出法律意见
- 不得在本阶段读取其他Agent输出

[CONTEXT]: 项目路径 <project_path>, 工作目录 .arc/ip-audit/<project-name>/
`
)
```

**等待三Agent完成**,使用 `background_output(task_id="...")` 收集结果。

### Phase 3: 交叉反驳

**强制反驳机制**(每个Agent必须挑战其他两个Agent):

```typescript
// Oracle反驳Deep和Writing
Task(
  session_id="<oracle_session_id>", // 复用Phase 2的session
  load_skills=["arc:ip-audit"],
  run_in_background=false,
  description="Oracle交叉反驳Deep和Writing",
  prompt=`
[TASK]: 反驳Deep和Writing的评估,指出过度乐观/悲观之处

[EXPECTED OUTCOME]:
- 生成 agents/oracle/critique.md,包含:
  1. 对Deep实现评估的反驳(引用具体评分点)
  2. 对Writing合规评估的反驳
  3. 每个反驳必须附论据(文件路径/代码片段/外部参考)

[MUST DO]:
- 读取 agents/deep/implementation-analysis.md 和 agents/writing/compliance-analysis.md
- 挑战过高的评分(指出被忽略的风险)
- 挑战过低的评分(指出被低估的创新点)
- 使用 ace-tool 验证争议点
- 提出修正建议
- 反驳论据需包含技术三要素/程序产品视角,并可引用政策锚点

[MUST NOT DO]:
- 不得简单同意对方观点
- 不得无证据反驳

[CONTEXT]: 工作目录 .arc/ip-audit/<project-name>/
`
)

// Deep反驳Oracle和Writing (同理)
// Writing反驳Oracle和Deep (同理)
```

**收集反驳结果**,生成 `convergence/round-1-summary.md`。

### Phase 4: 综合可行性报告

**Step 4.1: 加权综合评分**

根据反驳报告动态调整权重:
- **专利可行性**: oracle 60% + deep 30% + writing 10%
- **软著可行性**: deep 60% + writing 30% + oracle 10%

若某Agent被强力反驳(2+条有力论据),降低其权重10%。

**Step 4.2: 生成资产清单**

生成 `analysis/ip-assets.md`:

| 资产编号 | 资产类型 | 证据路径 | 软著可行性 | 专利可行性 | 初步风险 |
|---------|---------|---------|-----------|-----------|---------|
| IPA-001 | 核心算法 | src/core/algorithm.ts:45-120 | 高 | 中 | 现有技术相似度待确认 |
| IPA-002 | 系统架构 | docs/architecture.md + src/server/ | 中 | 高 | 架构新颖但实现常规 |

**Step 4.3: 输出最终报告**

使用脚本生成标准化报告:
```bash
python ip-audit/scripts/render_audit_report.py \
  --case-dir .arc/ip-audit/<project-name>/ \
  --project-name <project_name>
```

生成文件:
- `reports/ip-feasibility-report.md` (可行性总报告)
- `reports/filing-readiness-checklist.md` (准备度清单)
- `handoff/ip-drafting-input.json` (交接给arc:ip-docs)

`ip-feasibility-report.md` 需新增版块:
- 软著材料合规度(页格式≥50行/页、命名一致性、代码样本覆盖度、说明文档截图一致性、签章页/保证书状态)
- 专利客体合规度(技术三要素完整性、程序产品可行性、附图/伪代码充分性)
- 费减资格与经济性(资格判断+原始/费减费用对比)
- App 电子版权可替代性(是否推荐、所需材料)

`filing-readiness-checklist.md` 增加: 格式核对、命名一致性、费减备案材料、签章页/非职务保证书、电子版权选项。

**Step 4.4: handoff JSON 扩展字段**

`handoff/ip-drafting-input.json` 新增/扩展字段(保持向后兼容,缺失时标记待补充):
- `format_compliance`: {`code_pages_ok`, `doc_lines_ok`, `name_consistency`, `signature_page_ready`}
- `program_product_recommended`: boolean
- `fee_reduction`: {`eligible`: boolean, `basis`: string, `required_proofs`: []}
- `app_e_copyright`: {`recommended`: boolean, `materials`: []}

**Step 4.5: 给出申请建议**

在报告末尾明确建议:
- **软著先行**: 代码完整、文档齐全、专利门槛不足
- **专利先行**: 技术创新显著、现有技术差异大、软著价值有限
- **并行推进**: 双轨可行性均高、时间紧迫、预算充足

## Scripts

优先使用脚本生成标准化骨架:

```bash
# 生成工作目录骨架
python ip-audit/scripts/scaffold_audit_case.py \
  --project-path <project_path> \
  --project-name <project_name>

# 渲染最终报告
python ip-audit/scripts/render_audit_report.py \
  --case-dir <output_dir> \
  --project-name <name>

# 费减资格检查(输出到 analysis/fee-reduction-assessment.md)
python ip-audit/scripts/fee_reduction_check.py \
  --applicant-type <individual|enterprise|institution> \
  --annual-income <number> \
  --output <path-to-analysis/fee-reduction-assessment.md>

# 格式合规检查(输出到 convergence/format-compliance.md)
python ip-audit/scripts/format_compliance_checker.py \
  --project-path <project_path> \
  --output <path-to-convergence/format-compliance.md>
```

## Artifacts

默认输出目录:`<project_path>/.arc/ip-audit/<project-name>/`

- `context/project-ip-snapshot.md` (项目快照)
- `context/external-references.md` (外部参考)
- `agents/oracle/innovation-analysis.md` (Oracle独立评估)
- `agents/oracle/critique.md` (Oracle反驳)
- `agents/deep/implementation-analysis.md` (Deep独立评估)
- `agents/deep/critique.md` (Deep反驳)
- `agents/writing/compliance-analysis.md` (Writing独立评估)
- `agents/writing/critique.md` (Writing反驳)
- `convergence/round-1-summary.md` (首轮综合)
- `convergence/final-consensus.md` (最终共识)
- `convergence/format-compliance.md` (格式/命名合规评估)
- `convergence/tech-elements-map.md` (技术三要素/程序产品映射)
- `analysis/ip-assets.md` (资产清单)
- `analysis/fee-reduction-assessment.md` (费减资格评估)
- `reports/ip-feasibility-report.md` (可行性总报告)
- `reports/filing-readiness-checklist.md` (准备度清单)
- `handoff/ip-drafting-input.json` (交接给arc:ip-docs)

## Quick Reference

| 输出物 | 用途 |
|------|------|
| `ip-feasibility-report.md` | 申请可行性总报告 |
| `filing-readiness-checklist.md` | 提交前材料缺口检查 |
| `ip-drafting-input.json` | 交接给 `arc:ip-docs` 的结构化输入 |
| `ip-assets.md` | 知识产权资产清单 |

## Failure Recovery

- **Agent超时(>10min)**: 询问用户是否继续等待或降级为双Agent分析。
- **Agent分析缺失**: 用其他两个Agent填补,标注"双源分析"。
- **MCP不可用**: 降级为 Grep + Read 直接扫描。
- **冲突无法解决**: 在报告中列出争议点,标注"需人工裁决"。
