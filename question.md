# Skills 中 oh-my-opencode Agent 角色定位描述审查

## 审查结论

整体框架写得很详细，但存在几个明显的不一致和描述缺失问题。

---

## 问题 1: deliberate/CLAUDE.md 与 SKILL.md 的 Agent 命名冲突

**严重程度**: 高

- `deliberate/CLAUDE.md` 第13行写的是 "oracle、deep、**momus**"
- `deliberate/SKILL.md` 实际用的是 "oracle、deep、**visual-engineering**"
- CLAUDE.md 的产物目录结构里写了 `agents/momus/`，但 SKILL.md 写的是 `agents/visual-engineering/`

CLAUDE.md 还停留在旧版本的 Agent 配置，需要同步更新。

---

## 问题 2: review/CLAUDE.md 与 SKILL.md 的 Agent 命名冲突

**严重程度**: 高

- `review/CLAUDE.md` 多处提到 "oracle/deep/**momus**"
- `review/SKILL.md` 实际用的是 "oracle/deep/**deep(业务)**"，完全没有 momus

同样是 CLAUDE.md 没跟上 SKILL.md 的更新。

---

## 问题 3: agent/SKILL.md 的 Subagent 列表不完整

**严重程度**: 中

可用 Subagent 表格（第109-115行）只列了 5 个：
- explore、librarian、oracle、metis、momus

但根 CLAUDE.md 的 Agent 矩阵定义了 10 个角色，缺失了：
- **Hephaestus**（核心编程）
- **Prometheus**（宏观规划）— 虽然在决策树第400行和速查表第586行被引用
- **Atlas**（大规模重构）
- **Multimodal-looker**（视觉前端）

决策树里用了 prometheus，但 Subagent 表格里没列出来，读者会困惑"这个 agent 从哪来的"。

---

## 问题 4: agent/SKILL.md 决策树与根 CLAUDE.md 的角色定义矛盾

**严重程度**: 高

- `agent/SKILL.md` 第170行：需求模糊需要澄清 → `Task(subagent_type="metis")`
- 根 `CLAUDE.md` 明确写了：**"Metis = 计划审计，不是需求澄清"**，需求澄清应该用 **Prometheus**

这是一个直接的角色职责冲突。

---

## 问题 5: Sisyphus 角色在 Skill 层面完全缺失

**严重程度**: 低

根 CLAUDE.md 定义了 Sisyphus 作为"主控调度员"，但所有 Skill 文件里都用"主进程"来指代这个角色。

如果 Sisyphus 就是 Claude Code 本身（即执行 Skill 的主进程），应该在某处明确说明这个映射关系。

---

## 问题 6: Category 与 Subagent 的边界不够清晰

**严重程度**: 中

根 CLAUDE.md 的 Agent 矩阵把所有角色都列为"Agent 代号"，但实际调用时：
- 有的是 `category`（deep、visual-engineering、quick 等）
- 有的是 `subagent_type`（oracle、explore、metis 等）

这两种调用方式的区别和选择逻辑没有在 Agent 矩阵中体现，读者需要跳到 agent/SKILL.md 才能搞清楚。

---

## 修复建议

| 优先级 | 修复项 | 涉及文件 |
|--------|--------|---------|
| P0 | deliberate/CLAUDE.md Agent 命名同步为 visual-engineering | `deliberate/CLAUDE.md` |
| P0 | review/CLAUDE.md Agent 命名同步为 deep(业务) | `review/CLAUDE.md` |
| P0 | agent/SKILL.md 决策树中 metis→prometheus（需求澄清场景） | `agent/SKILL.md` |
| P1 | agent/SKILL.md Subagent 表格补全 prometheus/hephaestus/atlas/multimodal-looker | `agent/SKILL.md` |
| P1 | 根 CLAUDE.md Agent 矩阵标注 category vs subagent_type 调用方式 | `CLAUDE.md` |
| P2 | 明确 Sisyphus = 主进程/Claude Code 本身的映射关系 | `CLAUDE.md` |
