# ARC Skill Routing Matrix

统一 `arc:*` 技能路由参考，优先用于“该用谁 / 不该用谁”的快速判定。  
若与单个 Skill 细则冲突，以该 Skill 的 `## When to Use`（尤其是**边界提示**）为准。

| Skill | 首选触发 | 不建议使用时机 | 推荐下游 / 邻接技能 |
|---|---|---|---|
| `arc:exec` | 需求模糊或跨技能编排 | 已明确具体技能且边界清晰 | `arc:clarify` / `arc:decide` / `arc:build` |
| `arc:clarify` | 需求不清、上下文缺失 | 方案已清晰可直接执行 | `arc:decide` / `arc:estimate` / `arc:build` |
| `arc:decide` | 高风险决策需多视角论证 | 仅需单路径快速执行 | `arc:build` / `arc:audit` |
| `arc:estimate` | 需要工时区间与波次排期 | 问题仍未定 scope | `arc:build` / 项目管理流程 |
| `arc:build` | 方案已定，开始代码落地 | 需求与边界尚未澄清 | `arc:audit` / `arc:e2e` |
| `arc:audit` | 企业级多维诊断与路线图 | 仅需门禁阻断判定 | `arc:release` / `arc:build` |
| `arc:release` | CI/CD 质量门禁阻断 | 尚未生成可用评分产物 | `score 产物刷新（由 gate 编排）` |
| `arc:e2e` | 真实用户路径 E2E 验证 | 没有测试入口或账号上下文 | `arc:fix` / `arc:retest` |
| `arc:fix` | 基于 FAIL 工件做定位修复 | 尚无可复现失败证据 | `arc:e2e`（先产证据） |
| `arc:retest` | 需重启服务的多轮回归闭环 | 一次性单轮排查即可完成 | `arc:e2e` / `arc:fix` |
| `arc:cartography` | 需生成或刷新 `codemap` | 仅做需求澄清或编码落地 | `arc:clarify` / `arc:build` / `arc:audit` |
| `arc:model` | 需要按项目实际情况输出 UML 图谱 | 仅需仓库目录概览或单点评审结论 | `arc:cartography` / `arc:audit` / `arc:build` |
| `arc:init` | 自动选择 full/update 维护 CLAUDE 索引 | 已明确必须 full 或 update | `arc:init:full` / `arc:init:update` |
| `arc:init:full` | 首次初始化或全量重建 | 仅有局部增量变更 | `arc:init:update`（后续维护） |
| `arc:init:update` | 已有基线的增量同步 | 缺失基线或基线严重失效 | `arc:init` / `arc:init:full` |
| `arc:ip-check` | 申请前 IP 可行性与风险评估 | 已进入正式文书撰写阶段 | `arc:ip-draft` |
| `arc:ip-draft` | 基于审查交接起草申请材料 | 尚未完成可行性审查 | `arc:ip-check` |

## Signal-to-Skill Decision Tree

```mermaid
flowchart TD
    A["输入任务信号"] --> B{"技能边界是否明确?"}
    B -- "否" --> AG["arc:exec"]
    B -- "是" --> C{"需求是否清晰可执行?"}
    C -- "否" --> RF["arc:clarify"]
    C -- "是" --> D{"是否高风险/多方案争议?"}
    D -- "是" --> DL["arc:decide"]
    D -- "否" --> E{"是否进入代码落地?"}
    E -- "是" --> IM["arc:build"]
    E -- "否" --> F{"是质量治理链路?"}
    F -- "门禁阻断" --> GT["arc:release"]
    F -- "企业级诊断" --> RV["arc:audit"]
    F -- "系统建模/UML" --> UML["arc:model"]
    F -- "否" --> G{"是 E2E 验证链路?"}
    G -- "执行验证" --> SM["arc:e2e"]
    G -- "失败修复" --> TR["arc:fix"]
    G -- "多轮闭环" --> LP["arc:retest"]
    G -- "否" --> H{"是索引/地图链路?"}
    H -- "codemap" --> CT["arc:cartography"]
    H -- "CLAUDE 自动路由" --> IN["arc:init"]
    H -- "强制全量" --> IF["arc:init:full"]
    H -- "仅增量" --> IU["arc:init:update"]
    H -- "否" --> I{"是知识产权链路?"}
    I -- "可行性审查" --> IA["arc:ip-check"]
    I -- "文书起草" --> ID["arc:ip-draft"]
```

## Phase Routing View

| 阶段 | 目标 | 主技能（Primary） | 辅助技能（Support） | 典型交接 |
|---|---|---|---|---|
| 澄清（Clarify） | 从模糊输入转为可执行需求 | `arc:exec` / `arc:clarify` | `arc:cartography` | `refined prompt` → 决策/落地 |
| 决策（Decide） | 处理高风险方案分歧 | `arc:decide` | `arc:estimate` | `consensus plan` → 实施 |
| 落地（Build） | 产出可提交代码变更 | `arc:build` | `arc:init*` / `arc:cartography` | `change handoff` → 验证 |
| 建模（Modeling） | 输出结构/行为/部署 UML 图谱 | `arc:model` | `arc:cartography` / `arc:audit` | `uml pack` → 评审/交接 |
| 验证（Validate） | 验证行为、定位失败、闭环修复 | `arc:e2e` / `arc:fix` / `arc:retest` | `arc:build` | `pass/fail evidence` → 治理 |
| 治理（Govern） | 门禁阻断、改进路线与治理闭环 | `arc:release` / `arc:audit` | `arc:build` | `arc:release/review outputs` |
| 知识产权（IP） | 先审查可行性，再起草材料 | `arc:ip-check` / `arc:ip-draft` | `arc:audit` | `ip-drafting-input` → 申请材料草稿 |

## Fast Routing Rules

- 先判定“是否已明确技能边界”：不明确优先 `arc:exec`。
- 先判定“是否已明确需求边界”：未明确优先 `arc:clarify`。
- 先判定“是否争议高风险”：高风险优先 `arc:decide`。
- 先判定“是否进入落地阶段”：已明确直接 `arc:build`。
- 先判定“是否需要系统建模图”：需要 UML 图谱优先 `arc:model`。
- 质量链路默认：`arc:release`（先触发 `score/`），必要时并联 `arc:audit`。
- E2E 修复链路默认：`arc:e2e` → `arc:fix`（循环则 `arc:retest`）。
