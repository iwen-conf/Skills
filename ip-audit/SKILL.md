---
name: "arc:ip-audit"
description: "面向软件项目的知识产权可行性审查与风险评估。读取项目上下文，输出专利/软件著作权申请可行性报告、优先级矩阵与申请准备度清单。用于申请前评估、融资前尽调、立项前合规审查。"
---

# arc:ip-audit — 项目专利/软著审查报告

## Overview

`arc:ip-audit` 只做一件事：基于项目真实代码与架构上下文，评估该项目进行**软件著作权**与**发明专利**申请的可行性、风险与优先级。

本技能不直接产出正式申请文书，审查结论将以结构化交接文件输出，供 `arc:ip-docs` 继续完成文档撰写。

## Mandatory Linkage（不可单打独斗）

必须按以下链路协作：

1. 优先读取 `arc:init` 产出的项目 `CLAUDE.md` 索引。
2. 若存在 `arc:review` 产物，复用其架构深度和风险结论。
3. 需求模糊时先调用 `arc:refine` 明确产品边界与业务目标。
4. 高争议技术路线可串联 `arc:deliberate` 做多 Agent 论证。
5. 审查输出交接给 `arc:ip-docs`，不得在本技能内混做文书写作。

## When to Use

- 需要判断项目是否适合申请软著/专利。
- 需要在融资、投标、上架前做知识产权可行性尽调。
- 需要评估项目中的创新点是否达到专利门槛。
- 需要明确申请顺序、风险点和费减可行性。

## Input Arguments

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `project_path` | string | 是 | 目标项目根目录绝对路径 |
| `project_name` | string | 否 | 项目名；默认从路径推导 |
| `software_name` | string | 否 | 申请软件名称；未提供时先使用临时名并在报告标注 |
| `applicant_type` | enum | 否 | `individual` / `enterprise` / `institution`，默认 `enterprise` |
| `business_goal` | string | 否 | 产品商业目标（用于评估申请优先级） |
| `output_dir` | string | 否 | 默认 `<project_path>/.arc/ip-audit/<project-name>/` |

## Dependencies

- **ace-tool MCP**（必须）：搜索项目代码与实现证据。
- **Exa MCP**（推荐）：补充现有技术/政策依据。
- **Task API**（必须）：调度 `explore` / `oracle` / `librarian`。
- **arc:init**（强推荐）：读取 `CLAUDE.md` 层级索引。
- **arc:review**（可选）：复用现有评审报告。

## Context Priority（强制）

严格遵循 `.arc/context-priority-protocol.md`：

1. `.arc/ip-audit/<project>/context/project-ip-snapshot.md`（24h）
2. `.arc/review/<project>/` 已有评审报告
3. 项目 `CLAUDE.md` 层级索引（7天）
4. `ace-tool` 源码语义扫描
5. `Exa` 外部参考

若前三级信息不足，必须降级扫描并在报告写明“上下文来源与新鲜度”。

## Critical Rules

1. **只读审查**：不得修改用户源码。
2. **证据驱动**：每个结论必须绑定文件路径或模块证据。
3. **双轨独立评分**：软著可行性与专利可行性分别评分，禁止混为单结论。
4. **风险显式化**：必须列出可驳回风险、补正风险、时间风险和材料缺口。
5. **交接标准化**：必须生成 `handoff/ip-drafting-input.json` 给 `arc:ip-docs`。
6. **法律边界**：输出为工程与流程建议，不替代执业律师/专利代理人法律意见。

## Instructions

### Phase 1: 上下文采集

1. 读取缓存与 `CLAUDE.md`。
2. 检查是否存在 `arc:review` 结果并提取相关结论。
3. 使用 `ace-tool` 搜索以下内容：
- 核心算法与性能优化实现
- 关键模块边界与系统交互
- 可作为软著提交样本的代码区段

### Phase 2: 创新点与权属资产清单

生成 `analysis/ip-assets.md`，至少包含：

- 资产编号（IPA-001...）
- 资产类型（算法、系统流程、模块实现、文档）
- 证据路径
- 软著可行性（高/中/低）
- 专利可行性（高/中/低）
- 初步风险

### Phase 3: 可行性评分

按下表评分并输出理由：

- 软著：代码完整性、版本边界、文档准备度、命名一致性。
- 专利：技术问题清晰度、技术手段充分性、技术效果可量化、现有技术差异度。

### Phase 4: 输出审查报告

必须输出以下文件：

- `reports/ip-feasibility-report.md`
- `reports/filing-readiness-checklist.md`
- `handoff/ip-drafting-input.json`

并给出建议顺序：`软著先行 / 专利先行 / 并行推进`。

## Scripts

优先使用脚本生成标准化骨架：

```bash
python ip-audit/scripts/scaffold_audit_case.py --project-path <project_path>
python ip-audit/scripts/render_audit_report.py --case-dir <output_dir> --project-name <name>
```

## Artifacts

默认输出目录：`<project_path>/.arc/ip-audit/<project-name>/`

- `context/project-ip-snapshot.md`
- `analysis/ip-assets.md`
- `reports/ip-feasibility-report.md`
- `reports/filing-readiness-checklist.md`
- `handoff/ip-drafting-input.json`

## Quick Reference

| 输出物 | 用途 |
|------|------|
| `ip-feasibility-report.md` | 申请可行性总报告 |
| `filing-readiness-checklist.md` | 提交前材料缺口检查 |
| `ip-drafting-input.json` | 交接给 `arc:ip-docs` 的结构化输入 |
