# 项目快照: Skills

## 拓扑类型
单项目仓库（Claude Code Skills 集合）

## 目录树
```
Skills/
├── agent/           # arc:agent 智能调度
├── deliberate/      # arc:deliberate 三模型审议
├── init/            # arc:init CLAUDE.md 生成
│   └── references/  # schema + 扫描启发式
├── loop/            # arc:loop tmux 回归闭环
│   ├── assets/      # 配置示例
│   ├── references/  # tmux runbook
│   └── scripts/     # uxloop_tmux.py, uxloop_cleanup.py
├── refine/          # arc:refine 问题细化
├── review/          # arc:review 企业级评审
│   └── references/  # dimensions.md 七维度框架
├── simulate/        # arc:simulate E2E 测试
│   ├── examples/    # 脚手架示例
│   ├── reports/     # 测试报告输出
│   ├── scripts/     # 6 个 Python 脚本
│   └── templates/   # 报告模板
└── triage/          # arc:triage 缺陷修复
    ├── references/  # decision tree + fix-packet template
    └── scripts/     # triage_run.py
```

## 技术栈清单
| 目录 | 语言 | 框架 | 版本 |
|------|------|------|------|
| scripts/ | Python 3 | - | 无版本约束 |
| SKILL.md | Markdown | frontmatter YAML | - |
| 外部依赖 | CLI | codex, gemini, openspec | 需预装 |

## 代码规模
- Markdown 文件: 26 个
- Python 脚本: 9 个
- 总行数: ~6,819 行
- 主要内容: 技能定义文档 + 辅助脚本

## 已有 CLAUDE.md
- `/Users/iluwen/Documents/Code/Skills/CLAUDE.md`（根级，已存在）

## 项目成熟度
稳定生产使用中，为多项目工作空间提供 Claude Code 技能扩展。
