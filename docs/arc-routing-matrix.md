# ARC Skill Routing Matrix

统一 `arc:*` 技能路由参考，优先用于“该用谁 / 不该用谁”的快速判定。  
若与单个 Skill 细则冲突，以该 Skill 的 `## When to Use`（尤其是**边界提示**）为准。

| Skill | 首选触发 | 不建议使用时机 | 推荐下游 / 邻接技能 |
|---|---|---|---|
| `arc:exec` | 需求模糊或跨技能编排 | 已明确具体技能且边界清晰 | `arc:clarify` / `arc:decide` / `arc:build` |
| `arc:clarify` | 需求不清、上下文缺失 | 方案已清晰可直接执行 | `arc:decide` / `arc:build` |
| `arc:decide` | 高风险决策需多视角论证 | 仅需单路径快速执行 | `arc:build` / `arc:audit` |
| `arc:build` | 方案已定，开始代码落地 | 需求与边界尚未澄清 | `arc:audit` / `arc:e2e` |
| `arc:aigc` | 学术/专业文本去模板化润色、降低机器腔并保持引用/公式/数据不漂移 | 需要规避检测、冒充原创作者、捏造事实或补造引用 | `arc:clarify` / `arc:audit` |
| `arc:audit` | 企业级多维诊断与路线图 | 仅需门禁阻断判定 | `arc:gate` / `arc:build` |
| `arc:gate` | 合并/上线门禁判定（Go/No-Go） | 尚未生成可用评分产物 | `score 产物刷新（由 gate 编排）` |
| `arc:context` | 上下文过长、切会话、需要恢复任务状态或跨 agent 交接 | 只需要仓库索引，或已经可以直接进入编码/修复 | `arc:init` / `arc:build` / `arc:fix` |
| `arc:e2e` | 真实用户路径 E2E 验证 | 没有测试入口或账号上下文 | `arc:fix` |
| `arc:test` | 代码级测试生成（单测/边界/benchmark/fuzz） | 浏览器 E2E 验证或修复已有失败测试 | `arc:e2e` / `arc:fix` |
| `arc:fix` | 基于 FAIL 工件做定位修复 | 尚无可复现失败证据 | `arc:e2e`（先产证据） / `arc:fix --mode retest-loop` |
| `arc:cartography` | 需生成或刷新 `codemap` | 仅做需求澄清或编码落地 | `arc:clarify` / `arc:build` / `arc:audit` |
| `arc:uml` | 需要按项目实际情况输出 UML 图谱 | 仅需仓库目录概览或单点评审结论 | `arc:cartography` / `arc:audit` / `arc:build` |
| `arc:init` | 自动选择 full/update 维护索引 | 与索引无关的普通开发任务 | `arc:init --mode full` / `arc:init --mode update` |
| `arc:serve` | 启动/重启/停止本地前后端或 dev server，并避免重复 `tmux` 会话 | 生产部署、Docker/K8s 编排、一次性 build/test/lint 命令 | `arc:build` / `arc:fix` / `arc:context` |
| `arc:ip-check` | 申请前 IP 可行性与风险评估 | 已进入正式文书撰写阶段 | `arc:ip-draft` |
| `arc:ip-draft` | 基于审查交接起草申请材料 | 尚未完成可行性审查 | `arc:ip-check` |
| `arc:learn` | 深度学习特定技术/概念，需三层交叉验证 | 仅需快速查询或已有明确方案 | `arc:clarify` / `arc:build` |
| `arc:brand-brief` | 提取项目事实信息生成设计师简报 | 需要代码地图或质量诊断 | `arc:cartography` / `arc:audit` |

## Browser Automation Strategy

选择最适合当前任务的浏览器自动化工具：

| 场景类型 | 推荐工具 | 核心优势 | 典型任务 |
|---|---|---|---|
| **单点验证 / 证据沉淀** | `mcp_chrome-devtools_*` | 原生集成，极速，无需外部依赖 | 截图存证、简单元素状态检查、执行单行 JS |
| **复杂流程 / 多步交互** | `agent-browser` (Skill) | 自动等待、Ref 引用、Session 隔离 | 完整登录流程、多级表单提交、跨页面跳转 |
| **UI/UX 审计** | `arc:audit` + DevTools | 深度分析性能与无障碍 | 核心指标 (LCP/CLS) 审计、WCAG 语义检查 |
| **端到端回归** | `arc:e2e` | 完整报告、证据闭环、Persona 模拟 | 冒烟测试、关键业务路径回归 |

## Signal-to-Skill Decision Tree

```mermaid
flowchart TD
    A["输入任务信号"] --> B{"技能边界是否明确?"}
    B -- "否" --> AG["arc:exec"]
    B -- "是" --> C{"需求是否清晰可执行?"}
    C -- "否" --> RF["arc:clarify"]
    C -- "是" --> D{"是否高风险/多方案争议?"}
    D -- "是" --> DL["arc:decide"]
    D -- "需要估算" --> EST["arc:decide --mode estimate"]
    D -- "否" --> E{"是否进入代码落地?"}
    E -- "是" --> IM["arc:build"]
    E -- "否" --> F{"是质量治理链路?"}
    F -- "门禁阻断" --> GT["arc:gate"]
    F -- "企业级诊断" --> RV["arc:audit"]
    F -- "系统建模/UML" --> UML["arc:uml"]
    F -- "否" --> G{"是 E2E 验证链路?"}
    G -- "执行验证" --> SM["arc:e2e"]
    G -- "代码级测试" --> UT["arc:test"]
    G -- "失败修复" --> TR["arc:fix"]
    G -- "多轮闭环" --> LP["arc:fix --mode retest-loop"]
    G -- "否" --> H{"是索引/地图/上下文链路?"}
    H -- "codemap" --> CT["arc:cartography"]
    H -- "索引自动路由" --> IN["arc:init"]
    H -- "强制全量" --> IF["arc:init --mode full"]
    H -- "仅增量" --> IU["arc:init --mode update"]
    H -- "本地服务启停" --> SV["arc:serve"]
    H -- "上下文恢复/切会话" --> CX["arc:context"]
    H -- "否" --> I{"是知识产权链路?"}
    I -- "可行性审查" --> IA["arc:ip-check"]
    I -- "文书起草" --> ID["arc:ip-draft"]
    I -- "否" --> J{"是学术/专业文本润色链路?"}
    J -- "是" --> AI["arc:aigc"]
    J -- "否" --> K{"是深度学习/品牌简报链路?"}
    K -- "深度学习" --> LN["arc:learn"]
    K -- "品牌简报" --> BB["arc:brand-brief"]
    K -- "否" --> AG2["arc:exec"]
```

## Phase Routing View

| 阶段 | 目标 | 主技能（Primary） | 辅助技能（Support） | 典型交接 |
|---|---|---|---|---|
| 澄清（Clarify） | 从模糊输入转为可执行需求 | `arc:exec` / `arc:clarify` | `arc:cartography` | `refined prompt` → 决策/落地 |
| 上下文（Context） | 生成/恢复任务工作集、恢复清单与交接包 | `arc:context` | `arc:init` / `arc:cartography` / `arc:build` | `restore packet` → 落地/修复/验证 |
| 决策（Decide） | 处理高风险方案分歧 | `arc:decide` | `arc:decide --mode estimate` | `consensus plan` → 实施 |
| 落地（Build） | 产出可提交代码变更 | `arc:build` | `arc:init` / `arc:cartography` | `change handoff` → 验证 |
| 运行（Serve） | 启停本地长时前后端服务并维持单实例 `tmux` 会话 | `arc:serve` | `arc:build` / `arc:fix` / `arc:context` | `tmux session handoff` → 调试/回归 |
| 写作（Writing） | 学术/专业文本去模板化润色并统一作者声线 | `arc:aigc` | `arc:clarify` / `arc:audit` | `rewritten draft` → 人工复核/交付 |
| 建模（Modeling） | 输出结构/行为/部署 UML 图谱 | `arc:uml` | `arc:cartography` / `arc:audit` | `uml pack` → 评审/交接 |
| 验证（Validate） | 验证行为、定位失败、闭环修复 | `arc:e2e` / `arc:test` / `arc:fix` | `arc:fix --mode retest-loop` / `arc:build` | `pass/fail evidence` → 治理 |
| 治理（Govern） | 门禁阻断、改进路线与治理闭环 | `arc:gate` / `arc:audit` | `arc:build` | `arc:gate/review outputs` |
| 知识产权（IP） | 先审查可行性，再起草材料 | `arc:ip-check` / `arc:ip-draft` | `arc:audit` | `ip-drafting-input` → 申请材料草稿 |
| 学习（Learn） | 深度学习特定技术概念并交叉验证 | `arc:learn` | `arc:clarify` / `arc:build` | `verified knowledge` → 落地/决策 |
| 品牌（Brand） | 提取项目事实信息供设计师使用 | `arc:brand-brief` | `arc:cartography` / `arc:audit` | `brand brief` → 设计交接 |

## Fast Routing Rules

- 先判定“是否已明确技能边界”：不明确优先 `arc:exec`。
- 先判定“是否已明确需求边界”：未明确优先 `arc:clarify`。
- 先判定“是否争议高风险”：高风险优先 `arc:decide`。
- 先判定“是否需要压缩或恢复任务上下文”：需要则优先 `arc:context`。
- 先判定“是否需要启停本地长时服务”：需要则优先 `arc:serve`。
- 先判定“是否进入落地阶段”：已明确直接 `arc:build`。
- 先判定“是否需要学术/专业文本去模板化润色”：需要则优先 `arc:aigc`。
- 先判定“是否需要系统建模图”：需要 UML 图谱优先 `arc:uml`。
- 质量链路默认：`arc:gate`（先触发 `score/`），必要时并联 `arc:audit`。
- E2E 修复链路默认：`arc:e2e` → `arc:fix`（循环则 `arc:fix --mode retest-loop`）。
