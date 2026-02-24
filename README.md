# Arc Skills

Claude Code 技能（Skill）集合，统一使用 **`arc:`** 命名空间。每个技能是一个自包含的 Skill 插件，通过 `/arc:<name>` 在 Claude Code 中调用。

## 架构总览

```mermaid
graph TD
    subgraph "需求理解"
        refine["arc:refine<br/>问题细化"]
    end
    subgraph "多模型审议"
        deliberate["arc:deliberate<br/>三模型审议 + OpenSpec"]
    end
    subgraph "项目评审"
        review["arc:review<br/>企业级七维度评审"]
    end
    subgraph "E2E 测试闭环"
        simulate["arc:simulate<br/>E2E 浏览器测试"]
        triage["arc:triage<br/>缺陷定位与修复"]
        loop["arc:loop<br/>tmux 回归闭环"]
    end

    refine -->|"enhanced-prompt.md"| deliberate
    simulate -->|"run_dir 报告"| triage
    triage -->|"修复→重启"| loop
    loop -->|"重测"| simulate
```

## 技能一览

| 调用方式 | 目录 | 用途 |
|---------|------|------|
| `/arc:simulate` | `simulate/` | 通过 agent-browser 模拟真实用户进行 E2E 浏览器测试，生成含截图的结构化报告 |
| `/arc:triage` | `triage/` | 分析 arc:simulate 的失败报告，定位根因、修复缺陷、执行回归验证 |
| `/arc:loop` | `loop/` | 管理 tmux 会话启动/重启服务，循环执行 arc:simulate 直到 PASS 或达到迭代上限 |
| `/arc:refine` | `refine/` | 扫描 CLAUDE.md 层级索引，为模糊的用户 prompt 补充项目上下文 |
| `/arc:deliberate` | `deliberate/` | 三模型（Claude/Codex/Gemini）多视角审议，使用 OpenSpec 生成结构化计划 |
| `/arc:review` | `review/` | 按企业级七维度框架深度评审软件项目，三模型对抗式分析，输出诊断报告 |

## 依赖链

```mermaid
graph LR
    refine --> deliberate
    simulate --> triage --> loop --> simulate
    review["arc:review（独立）"]
```

- `arc:refine` → `arc:deliberate`：问题细化后进入三模型审议
- `arc:simulate` → `arc:triage` → `arc:loop` → `arc:simulate`：E2E 测试→缺陷修复→回归闭环
- `arc:review`：独立运行，不依赖其他 Skill

## 快速开始

```bash
# 在 Claude Code 中调用
/arc:simulate    # E2E 测试
/arc:triage      # 缺陷修复
/arc:loop        # 回归闭环
/arc:refine      # 问题细化
/arc:deliberate  # 三模型审议
/arc:review      # 项目评审
```

## 技术栈

- **技能定义**：Markdown（SKILL.md frontmatter）
- **辅助脚本**：Python 3（`scripts/` 目录，`--help` 查看用法）
- **外部模型**：通过 `codeagent-wrapper` CLI 调用 Codex/Gemini
- **计划生成**：[OpenSpec](https://github.com/Fission-AI/OpenSpec) CLI（`openspec`）
- **代码搜索**：ace-tool MCP（语义搜索）+ Exa MCP（互联网搜索）

## 约定

详见 [CLAUDE.md](./CLAUDE.md)。
