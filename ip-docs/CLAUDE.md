# ip-docs/CLAUDE.md

## 模块定位

`ip-docs/` 提供 `arc:ip-docs`，采用**多Agent协作模式**(oracle/deep/momus)专注于软著/专利申请文档草稿写作，不做可行性裁决。

## 核心产物

- `.arc/ip-docs/<project>/copyright/software-summary.md`
- `.arc/ip-docs/<project>/patent/disclosure-draft.md`
- `.arc/ip-docs/<project>/patent/claims-draft.md`
- `.arc/ip-docs/<project>/reports/doc-writing-log.md`

## 多Agent架构

### Agent角色分工

| Agent | 角色定位 | 起草文档类型 | 输出文件 |
|-------|---------|------------|---------|
| **oracle** (subagent) | 技术方案描述专家 | 架构设计、技术方案、系统流程、专利技术交底书 | `agents/oracle/technical-description.md` |
| **deep** (category) | 实现细节专家 | 代码实现、算法细节、性能优化、软著技术交底 | `agents/deep/implementation-details.md` |
| **momus** (subagent) | 用户文档专家 | 用户手册、操作说明、软著摘要、权利要求书 | `agents/momus/user-documentation.md` |

### 协作流程

**Phase 1: 读取交接与上下文** → **Phase 2: 并发独立起草** → **Phase 3: 交叉审阅** → **Phase 4: 定稿文档草稿**

### 文档分工矩阵

| 文档类型 | 主起草Agent | 审阅Agent | 输出路径 |
|---------|-----------|----------|---------|
| 软著摘要 | momus | oracle, deep | `copyright/software-summary.md` |
| 操作说明书 | momus | oracle, deep | `copyright/manual-outline.md` |
| 代码材料说明 | deep | oracle | `copyright/source-code-package-notes.md` |
| 技术交底书 | oracle | deep, momus | `patent/disclosure-draft.md` |
| 权利要求书 | momus | oracle, deep | `patent/claims-draft.md` |
| 附图说明 | oracle, momus | deep | `patent/drawings-description.md` |

## 脚本入口

- `scripts/scaffold_drafting_case.py`
- `scripts/render_ip_documents.py`

## 协作关系与调用方式

- 上游：`arc:ip-audit`（优先读取 `handoff/ip-drafting-input.json`）
- 平级：`arc:init`、`arc:deliberate`（用于上下文补全与术语校正）

### Agent调用示例

```typescript
// Phase 2: 并发启动三Agent起草
Task(
  subagent_type="oracle",
  load_skills=["arc:ip-docs"],
  run_in_background=true,
  description="Oracle起草技术方案描述",
  prompt="..."
)
Task(
  category="deep",
  load_skills=["arc:ip-docs"],
  run_in_background=true,
  description="Deep起草实现细节",
  prompt="..."
)
Task(
  subagent_type="momus",
  load_skills=["arc:ip-docs"],
  run_in_background=true,
  description="Momus起草用户文档",
  prompt="..."
)

// Phase 3: 交叉审阅(复用session_id)
Task(
  session_id="<oracle_session_id>",
  load_skills=["arc:ip-docs"],
  run_in_background=false,
  description="Oracle审阅Deep和Momus草稿",
  prompt="..."
)
```
