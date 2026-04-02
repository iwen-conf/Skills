# Arc Skills 评价报告

## 总览评分

| 技能 | 行数 | 结构 | 资源完备 | 可执行性 | 综合 |
|------|------|------|---------|---------|------|
| `arc:audit` | 600 | 5/5 | 5/5 | 高 | **A** |
| `arc:build` | 234 | 5/5 | 4/5 | 中 | **B+** |
| `arc:cartography` | 170 | 4/5 | 4/5 | 中 | **B** |
| `arc:clarify` | 208 | 4/5 | 2/5 | 低（纯交互） | **B-** |
| `arc:decide` | 740 | 5/5 | 3/5 | 中 | **B+** |
| `arc:e2e` | 523 | 5/5 | 5/5 | 高 | **A** |
| `arc:exec` | 722 | 5/5 | 2/5 | 低（无脚本） | **B** |
| `arc:fix` | 345 | 4/5 | 4/5 | 中 | **B+** |
| `arc:gate` | 389 | 5/5 | 4/5 | 高 | **A-** |
| `arc:init` | 329 | 4/5 | 3/5 | 中 | **B** |
| `arc:ip-check` | 510 | 5/5 | 5/5 | 高 | **A** |
| `arc:ip-draft` | 492 | 5/5 | 5/5 | 高 | **A** |
| `arc:uml` | 208 | 4/5 | 3/5 | 中 | **B** |

---

## 整体优势

1. **统一的架构范式** — 所有技能都遵循相同的骨架：Iron Law -> Quick Contract -> Routing Matrix -> Workflow -> Quality Gates -> Anti-Patterns，可读性和可维护性很强
2. **铁律约束明确** — 每个技能都有不可逾越的底线声明，这比堆砌 MUST 有效得多
3. **反模式教学充分** — 每个技能都列出禁止行为和错误示范，有效防止模型走偏
4. **跨技能衔接设计** — clarify -> decide -> build -> e2e -> fix 形成了完整的工程闭环；ip-check -> ip-draft 有清晰的移交接口
5. **标准对齐** — 引入了 IEEE 29148、ISTQB、OWASP、WCAG、ADR、SEV 分级等国际标准，不是闭门造车
6. **头部技能非常出色** — `arc:audit`、`arc:e2e`、`arc:ip-check`、`arc:ip-draft` 四个技能从文档到脚本到模板都非常完备

---

## 核心问题

### 问题 1：`schedule_task()` 伪代码悬空

`arc:exec`、`arc:decide`、`arc:ip-check`、`arc:ip-draft`、`arc:init` 都大量引用 `schedule_task()` API 和 `docs/orchestration-contract.md`，但 Skills 仓库中没有这些文件的实现。如果模型在运行时找不到这些依赖，会导致流程卡死。

### 问题 2：资源完备度参差不齐

| 资源丰富 | 资源贫乏 |
|---------|---------|
| `arc:e2e`（6 脚本 + 15 类模板） | `arc:exec`（0 脚本、0 模板） |
| `arc:ip-check`（4 脚本 + 5 参考 + 2 模板） | `arc:clarify`（纯文本，无任何资源） |
| `arc:ip-draft`（2 脚本 + 3 参考 + 8 模板） | `arc:init`（0 脚本） |

`arc:exec` 作为调度层核心，却没有任何可执行脚本，完全依赖运行时基础设施。

### 问题 3：部分技能行数过长

`arc:decide`（740 行）和 `arc:exec`（722 行）超出了推荐的 500 行上限。大量篇幅花在 `schedule_task()` 调用示例上，但这些其实可以抽取到 `references/` 中。

### 问题 4：description 触发词偏窄

所有技能的 description 都是中文短语，如"项目体检：七维评审并输出 HTML 可视化报告"。这对中文用户友好，但：
- 缺少英文关键词覆盖
- 缺少"推动性"触发描述（skill-creator 推荐的 "pushy" 风格）
- 边界场景（如用户说"帮我看看代码质量"）可能不会触发 `arc:audit`

### 问题 5：具体示例不足

- `arc:uml` 提到使用 Chen 记法，但没有 Mermaid 语法示例
- `arc:build` 的 `execution-standards.md` 仅 379 字节，过于单薄
- `arc:cartography` 的 Tier 1/2/3 JSON 导出缺少 schema 示例

---

## 建议优先级

| 优先级 | 行动 | 影响范围 |
|--------|------|---------|
| **P0** | 补全 `docs/orchestration-contract.md` 或将调度逻辑内联到技能中 | 5 个技能 |
| **P1** | 为 `arc:exec` 补充调度脚本 | 核心调度层 |
| **P1** | 将 `arc:decide` 和 `arc:exec` 的 `schedule_task()` 示例抽到 `references/` 降低行数 | 2 个技能 |
| **P2** | 优化所有技能的 description 触发词（可用 skill-creator 的描述优化流程） | 全部 13 个 |
| **P2** | 为 `arc:uml` 补充 Mermaid Chen 记法示例 | 1 个技能 |
| **P3** | 充实 `arc:build` 的 `execution-standards.md` | 1 个技能 |

---

## 总结

这套技能体系的架构设计是企业级水准，统一范式、铁律约束、反模式防护都做得很好。主要短板在于调度基础设施的文档/实现缺失和部分技能资源不够充实。
