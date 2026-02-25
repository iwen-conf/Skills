# Arc Skills

Claude Code 技能（Skill）集合，统一使用 **`arc:`** 命名空间。每个技能是一个自包含的 Skill 插件，通过 `/arc:<name>` 在 Claude Code 中调用。

## 架构总览

```mermaid
graph TD
    subgraph "智能调度"
        agent["arc:agent<br/>需求分析 + Skill 路由 + 多Agent调度"]
    end
    subgraph "项目初始化"
        init["arc:init<br/>多Agent协作 CLAUDE.md 生成"]
    end
    subgraph "需求理解"
        refine["arc:refine<br/>问题细化"]
    end
    subgraph "多Agent审议"
        deliberate["arc:deliberate<br/>多Agent审议 + OpenSpec"]
    end
    subgraph "工程实现"
        implement["arc:implement<br/>方案落地与编码实现"]
    end
    subgraph "项目评审"
        review["arc:review<br/>企业级七维度评审"]
    end
    subgraph "知识产权"
        ip_audit["arc:ip-audit<br/>专利/软著可行性审查"]
        ip_docs["arc:ip-docs<br/>申请文档草稿写作"]
    end
    subgraph "E2E 测试闭环"
        simulate["arc:simulate<br/>E2E 浏览器测试"]
        triage["arc:triage<br/>缺陷定位与修复"]
        loop["arc:loop<br/>tmux 回归闭环"]
    end
    subgraph "模型层"
        claude["Claude<br/>Task subagent"]
        codex["Codex<br/>Bash: codex exec --full-auto"]
        gemini["Gemini<br/>Bash: gemini -p --yolo"]
    end

    agent -.->|"路由"| init
    agent -.->|"路由"| refine
    agent -.->|"路由"| deliberate
    agent -.->|"路由"| implement
    agent -.->|"路由"| review
    agent -.->|"路由"| ip_audit
    agent -.->|"路由"| ip_docs
    agent -.->|"路由"| simulate
    agent -->|"Task subagent"| claude
    agent -->|"Bash 调度"| codex
    agent -->|"Bash 调度"| gemini

    init -.->|"CLAUDE.md"| refine
    refine -->|"enhanced-prompt.md"| deliberate
    deliberate -->|"implementation plan"| implement
    implement -->|"change summary"| review
    review -.->|"技术评审结果"| ip_audit
    ip_audit -->|"handoff JSON"| ip_docs
    simulate -->|"run_dir 报告"| triage
    triage -->|"修复→重启"| loop
    loop -->|"重测"| simulate
```

## 技能一览

| 调用方式 | 目录 | 用途 |
|---------|------|------|
| `/arc:agent` | `agent/` | 智能调度 agent，分析用户需求后选择合适的 arc: skill，协调多Agent执行任务 |
| `/arc:simulate` | `simulate/` | 通过 agent-browser 模拟真实用户进行 E2E 浏览器测试，生成含截图的结构化报告 |
| `/arc:triage` | `triage/` | 分析 arc:simulate 的失败报告，定位根因、修复缺陷、执行回归验证 |
| `/arc:loop` | `loop/` | 管理 tmux 会话启动/重启服务，循环执行 arc:simulate 直到 PASS 或达到迭代上限 |
| `/arc:refine` | `refine/` | 扫描 CLAUDE.md 层级索引，为模糊的用户 prompt 补充项目上下文 |
| `/arc:deliberate` | `deliberate/` | 多Agent多视角审议，使用 OpenSpec 生成结构化计划 |
| `/arc:implement` | `implement/` | 消费上游方案并落地编码实现，输出实现计划、执行日志与交接摘要 |
| `/arc:review` | `review/` | 按企业级七维度框架深度评审软件项目，多Agent对抗式分析，输出诊断报告 |
| `/arc:init` | `init/` | 多Agent协作生成项目层级式 CLAUDE.md 索引体系，深度扫描后输出根级+模块级 CLAUDE.md |
| `/arc:ip-audit` | `ip-audit/` | 软件专利/软著可行性审查，输出评估报告、风险矩阵与文档交接 JSON |
| `/arc:ip-docs` | `ip-docs/` | 基于项目上下文与审查结论撰写软著/专利申请文档草稿 |

## 依赖链

```mermaid
graph LR
    agent["arc:agent"] -.->|"路由"| init
    agent -.->|"路由"| refine
    agent -.->|"路由"| deliberate
    agent -.->|"路由"| implement
    agent -.->|"路由"| review
    ip_audit["arc:ip-audit"]
    ip_docs["arc:ip-docs"]
    agent -.->|"路由"| ip_audit
    agent -.->|"路由"| ip_docs
    agent -->|"Task subagent"| claude["Claude"]
    agent -->|"Bash"| codex["Codex"]
    agent -->|"Bash"| gemini["Gemini"]

    init -.->|"CLAUDE.md"| refine
    refine --> deliberate
    deliberate --> implement --> review
    review -.-> ip_audit --> ip_docs
    simulate --> triage --> loop --> simulate
```

- `arc:agent`：统一入口，智能路由到合适的 skill 或直接调度模型
- `arc:init`：独立运行，输出的 CLAUDE.md 层级索引被 `arc:refine` 消费
- `arc:refine` → `arc:deliberate`：问题细化后进入多Agent审议
- `arc:deliberate` → `arc:implement`：方案进入工程实现阶段
- `arc:implement` → `arc:review`：实现后进入质量评审
- `arc:simulate` → `arc:triage` → `arc:loop` → `arc:simulate`：E2E 测试→缺陷修复→回归闭环
- `arc:review`：独立运行，不依赖其他 Skill
- `arc:ip-audit`：优先读取 `arc:init`/`arc:review` 产物，输出审查报告与 `handoff` 结构化交接
- `arc:ip-docs`：消费 `arc:ip-audit` 交接信息，生成专利/软著申请文档草稿

## 快速开始

```bash
# 在 Claude Code 中调用
/arc:agent       # 智能调度（推荐入口）
/arc:init        # 项目初始化（多Agent协作生成 CLAUDE.md）
/arc:simulate    # E2E 测试
/arc:triage      # 缺陷修复
/arc:loop        # 回归闭环
/arc:refine      # 问题细化
/arc:deliberate  # 多Agent审议
/arc:implement   # 方案落地实现
/arc:review      # 项目评审
/arc:ip-audit    # 知识产权可行性审查
/arc:ip-docs     # 知识产权申请文档写作
```

## 技术栈

- **技能定义**：Markdown（SKILL.md frontmatter）
- **辅助脚本**：Python 3（`scripts/` 目录，`--help` 查看用法）
- **外部模型**：`codex exec` (Codex CLI) + `gemini -p` (Gemini CLI)
- **计划生成**：[OpenSpec](https://github.com/Fission-AI/OpenSpec) CLI（`openspec`）
- **代码搜索**：ace-tool MCP（语义搜索）+ Exa MCP（互联网搜索）

## 约定

详见 [CLAUDE.md](./CLAUDE.md)。
