---
name: graduation-doc-support
description: 毕业设计支撑文档生成：当用户要求基于真实项目代码、数据库和现有论文/同学模板，生成“系统用例规约”“数据库物理设计三线表”“功能模块实现主要代码”“测试用例设计”等多个 docx 时触发；强调以运行时代码和 SQL 为准，模板只借格式不借内容。
---

# Graduation Doc Support

## Overview

`graduation-doc-support` 用于从本地项目真实实现出发，生成毕业设计/论文配套的支撑性 Word 文档。它适合这样的请求：用户给出项目目录，可能再给一个同学论文或旧 Word 模板，让你“按一样的方式”产出新的 `docx`，但内容必须严格对应当前系统实现。

这个 skill 解决的是“证据驱动文档交付”，不是单纯润色。源码、数据库脚本、已有测试和前端页面是真正的事实来源；同学论文或既有 Word 只允许提供版式、栏目和表格风格，不允许借用其业务结论。详细取证清单与交付矩阵见 [`references/evidence-checklist.md`](references/evidence-checklist.md)。

## Quick Contract

- **Trigger**: 用户要求查看某个项目的代码、数据库、同学论文或 Word 模板，并据此生成多个毕业设计支撑 `docx`。
- **Inputs**: `project_root`、可选 `peer_doc_path`、可选 `target_docs`、可选 `output_dir`、可选命名约定。
- **Outputs**: 基于真实实现生成的多个独立 `docx`，必要时附带 `tmp/docs/pdf/` 下的渲染校验文件与任务记录。
- **Quality Gate**: 每一项结论都必须能追溯到代码、SQL、路由、菜单、测试或配置；模板只能影响形式，不能替代事实来源。
- **Decision Tree**: 如果用户只是想润色已有论文正文，用 `arc:aigc`；如果用户要 UML 或 E-R 图，用 `arc:uml`；如果用户要的是从实现中生成毕业支撑文档，用本 skill。

## Announce

Begin by stating:

> "I am using `graduation-doc-support` to derive thesis support documents from real code and schema evidence, not from template copy."

## Input Arguments

| parameter | type | required | description |
|-----------|------|----------|-------------|
| `project_root` | string | yes | 目标项目根目录，必须是实际要取证的仓库路径 |
| `peer_doc_path` | string | no | 同学论文、旧版 Word 或模板路径，只用于提取栏目和格式线索 |
| `target_docs` | array | no | 需要生成的文档列表，默认包含系统用例规约、数据库物理设计三线表、功能模块实现主要代码、测试用例设计 |
| `output_dir` | string | no | 目标输出目录；若未指定，优先沿用项目内现有论文资料目录 |
| `naming_rule` | string | no | 文件命名规则，例如“项目名+文档名”或“学号姓名+文档名” |
| `verification_mode` | enum | no | `render-check` / `text-check`；默认优先 `render-check`，即转 PDF 检查版式 |

## The Iron Law

```text
NO THESIS SUPPORT DOCUMENT MAY CLAIM A FEATURE, TABLE, FLOW, OR TEST THAT THE PROJECT DOES NOT ACTUALLY IMPLEMENT.
RUNTIME CODE AND SQL BEAT TEMPLATES. TEMPLATES MAY GUIDE SHAPE, NEVER CONTENT.
```

## Workflow

1. 识别目标项目根目录，并先读取该仓库自己的 `AGENTS.md`、`CLAUDE.md` 或任务记录；如果用户提供了同学论文或旧 Word，只读取和本次需求相关的节，不批量照搬。
2. 建立事实优先级：先看运行时代码，再看数据库脚本，再看项目内设计文档，最后才看同学论文的栏目和表格样式。
3. 抽取证据矩阵：
   - 角色、登录注册、权限与会话；
   - 核心业务流和对应控制器/页面；
   - 物理表与字段；
   - 代表性代码片段；
   - 自动化测试现状与可设计的人工测试用例。
4. 对每一份目标文档先确定“必须写什么、不允许猜什么”：
   - `系统用例规约`：只写真实存在的角色和业务用例；
   - `数据库物理设计三线表`：直接从 SQL 解析表和字段，不用二手摘要；
   - `功能模块实现主要代码`：摘录最能说明系统实现的核心代码，给出真实路径和行号；
   - `测试用例设计`：若仓库自动化测试弱，则明确写人工测试设计并说明现状。
5. 使用 `python-docx` 生成多个独立 `docx`：
   - A4 页面；
   - 三线表；
   - 代码较宽时允许横向页面；
   - 文件名稳定且可直接交付。
6. 进行渲染校验：
   - `soffice --headless --convert-to pdf`；
   - `pdfinfo` 检查 A4；
   - `pdftotext -layout` 抽检标题、表格和代码片段是否断裂。
7. 收尾时明确说明验证结果与残余风险，例如：
   - `mvnw` 因换行符无法运行但系统 Maven 可运行；
   - 项目只有 `contextLoads()`，因此测试文档主要是手工用例设计。

## Quality Gates

- 所有业务描述都必须落到真实文件、类名、方法名、表名或页面路径。
- 不得把同学论文中的业务名词、模块名、第三方能力直接挪到目标项目。
- 数据库物理设计必须以实际 SQL 为准，不能只引用仓库快照、README 或人工总结。
- 如果项目没有完整自动化测试，要在测试文档里明确说明“现有自动化测试薄弱”，不能伪造已执行覆盖率。
- 输出必须是多个独立 `docx`，不能把四类内容混成一个总文档，除非用户明确要求。
- 完成后优先做 PDF 渲染校验；若环境缺依赖，则必须明确指出版式风险和缺失依赖。

## Red Flags

- 把“看一下同学论文”误解成“复制同学论文内容”。
- 未核对代码和数据库，就凭项目名称脑补角色、支付方式、推荐算法或模块边界。
- 直接沿用旧文档里的表数量、状态流转或测试结论，而没有回到 SQL 和源码复核。
- 把“测试用例设计”写成“测试结果报告”，或反过来把没有执行过的结果写成已通过。
- 未发现学号/姓名等命名证据，却擅自套用某个学生命名格式。

## When to Use

- **首选触发**：用户要求“根据我们的系统实现”“按这个论文/Word 的方式”生成毕业设计、论文支撑材料或多个 `docx`。
- **典型场景**：系统用例规约、数据库物理设计三线表、功能模块实现主要代码、测试用例设计、项目说明型论文附件。
- **边界提示**：如果任务只是润色已有论文段落，不要用本 skill；如果任务是建模出图，不要用本 skill；如果用户只给了模板却没给项目实现，也不能直接生成事实性文档。

## Outputs

```text
<project_root>/
├── docs/.../<project-name>系统用例规约.docx
├── docs/.../<project-name>数据库物理设计三线表.docx
├── docs/.../<project-name>功能模块实现主要代码.docx
├── docs/.../<project-name>测试用例设计.docx
└── tmp/docs/pdf/
    ├── <project-name>系统用例规约.pdf
    ├── <project-name>数据库物理设计三线表.pdf
    ├── <project-name>功能模块实现主要代码.pdf
    └── <project-name>测试用例设计.pdf
```
