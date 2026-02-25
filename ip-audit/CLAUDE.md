# ip-audit/CLAUDE.md

## 模块定位

`ip-audit/` 提供 `arc:ip-audit`，采用**多Agent协作模式**(oracle/deep/momus)负责软件专利/软著可行性审查，不负责最终申请文书写作。

## 核心产物

- `.arc/ip-audit/<project>/reports/ip-feasibility-report.md`
- `.arc/ip-audit/<project>/reports/filing-readiness-checklist.md`
- `.arc/ip-audit/<project>/handoff/ip-drafting-input.json`

## 多Agent架构

### Agent角色分工

| Agent | 角色定位 | 评估维度 | 输出文件 |
|-------|---------|---------|---------|
| **oracle** (subagent) | 架构与创新性专家 | 技术方案独创性、架构设计新颖性、现有技术差异度、专利申请可行性 | `agents/oracle/innovation-analysis.md` |
| **deep** (category) | 工程实现专家 | 代码完整性、实现充分性、技术效果可量化性、软著申请可行性 | `agents/deep/implementation-analysis.md` |
| **momus** (subagent) | 质量与合规专家 | 文档完备性、材料准备度、申请流程风险、知识产权合规性 | `agents/momus/compliance-analysis.md` |

### 协作流程

**Phase 1: 上下文采集** → **Phase 2: 并发独立评估** → **Phase 3: 交叉反驳** → **Phase 4: 综合可行性报告**

### 评分机制

- **专利可行性**: oracle 60% + deep 30% + momus 10%
- **软著可行性**: deep 60% + momus 30% + oracle 10%
- 若某Agent被强力反驳(2+条有力论据),降低其权重10%

## 脚本入口

- `scripts/scaffold_audit_case.py`
- `scripts/render_audit_report.py`

## 协作关系与调用方式

- 上游：`arc:init`、`arc:review`（提供上下文）
- 下游：`arc:ip-docs`（消费 handoff JSON）

### Agent调用示例

```typescript
// Phase 2: 并发启动三Agent
Task(
  subagent_type="oracle",
  load_skills=["arc:ip-audit"],
  run_in_background=true,
  description="Oracle评估技术创新性与专利可行性",
  prompt="..."
)
Task(
  category="deep",
  load_skills=["arc:ip-audit"],
  run_in_background=true,
  description="Deep评估代码完整性与软著可行性",
  prompt="..."
)
Task(
  subagent_type="momus",
  load_skills=["arc:ip-audit"],
  run_in_background=true,
  description="Momus评估文档完备性与申请准备度",
  prompt="..."
)

// Phase 3: 交叉反驳(复用session_id)
Task(
  session_id="<oracle_session_id>",
  load_skills=["arc:ip-audit"],
  run_in_background=false,
  description="Oracle交叉反驳Deep和Momus",
  prompt="..."
)
```
