[根目录](../CLAUDE.md) > **uml**

# uml -- UML 图谱生成

## 变更记录 (Changelog)

| 时间 | 操作 |
|------|------|
| 2026-03-05T12:15:00 | 新增 `arc:model` 技能，支持 14 类 UML 图型产出 |

## 模块职责

`arc:model` 负责基于项目真实证据生成 UML 图谱，按适用性输出类图、对象图、组件图、部署图、包图、复合结构图、配置文件图、用例图、活动图、状态机图、序列图、通信图、交互概述图、时间图。所有图必须使用 Mermaid 语法；若输出 E-R 图，必须使用陈氏画法。

## 入口与启动

### 入口文件

| 文件 | 用途 |
|------|------|
| `SKILL.md` | 技能定义与执行流程 |
| `references/diagram-catalog.md` | 图型证据映射与判定建议 |
| `references/notation-standards.md` | UML 标准与 Chen E-R 记法规范 |
| `scripts/scaffold_uml_pack.py` | UML 图骨架初始化脚本 |

### 调用方式

通过 Claude Code 调用：`arc model`

## 输出产物

```text
.arc/uml/<project-name>/
├── context/project-snapshot.md
├── diagram-plan.md
├── diagram-index.md
├── validation-summary.md
└── diagrams/*.mmd
```
