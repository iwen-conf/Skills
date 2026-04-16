---
name: arc:brand-brief
description: "提取项目信息生成设计简报：查看项目并提取项目名称、描述、技术栈、架构（前后端分离/CLI/服务端等）及前端主辅色调。禁止包含任何Logo设计建议。当用户要求生成Logo设计所需项目描述时触发。"
version: 1.0.0
---

# arc:brand-brief — Project Brand Brief Extractor

## Overview

This skill analyzes the current project and generates a pure, factual project description intended for professional designers to create a brand logo. It systematically extracts the name, summary, tech stack, and architectural/visual details (like colors) from the codebase.

## Quick Contract

- **Trigger**: The user requests a project description, brief, or summary specifically for the purpose of logo design.
- **Inputs**: None (automatically analyzes the workspace).
- **Outputs**: A structured Markdown report containing the project's factual details.
- **Quality Gate**: The output must exactly match the prescribed template, with NO design suggestions and NO redundant UI/color statements for non-frontend projects.

## Announce

Begin by stating clearly:
"I am using `arc:brand-brief` to extract project details for a design brief."

## The Iron Law

```
FACTS ONLY. NO DESIGN SUGGESTIONS.
```

The designers will decide the logo's appearance, forms, metaphors, and logo colors. Your ONLY job is to describe what the project *is* based on the code. 

## Workflow

1. **Information Gathering**:
   - **Name & Description**: Read `README.md`, `package.json`, `pyproject.toml`, or `Cargo.toml`.
   - **Tech Stack**: Analyze dependency files and source file extensions.
   - **Architecture**: Determine if it's Frontend, Backend, Frontend/Backend Separated, CLI, or Desktop.
2. **Color Extraction (Frontend Only)**:
   - If the project has a UI (Frontend, Fullstack, Desktop), search for theme configuration files:
     - `tailwind.config.js`, `tailwind.config.ts`
     - `.css` / `.scss` files with CSS variables (`:root { --primary: ... }`)
     - UI library theme files (e.g., `theme.ts`, `vuetify.js`)
   - Extract the logical colors (Primary/Main, Secondary, and Intermediate/Accent).
3. **Format & Sanitize**:
   - Ensure the output is strictly in Chinese.
   - Strip out any subjective descriptions, design advice, or suggestions.
   - If the project is CLI or Backend, completely omit any mention of colors or UI. Do not even say "No color scheme exists".
4. **Output**: Render the exact output template.

## Quality Gates

- **No Design Advice**: The output must not contain words like "建议", "Logo可以包含", "标志设计", "适合用作Logo", "推荐".
- **No Negative UI Statements**: For CLI/Backend projects, the output must not contain words like "没有前端", "不适用", "无配色". It should simply state "CLI项目" or "服务端项目" and end.
- **Format Consistency**: The output must exactly follow the provided Chinese markdown template.

## Red Flags

- Suggesting shapes or metaphors (e.g., "Because it's a cloud tool, the logo should have a cloud").
- Explaining *why* a color is used in the UI.
- Padding the output with conversational text like "Here is your design brief". (Just output the brief).

## Output Template

You MUST output EXACTLY this format, filling in the bracketed parts based on your findings:

```markdown
**项目描述（供设计师参考）**

1. **项目名称**：
   [Project Name]

2. **项目简介**：
   [1-2 sentences summarizing the project's core functionality]

3. **技术栈**：
   [Comma-separated list of main languages, frameworks, and core tools]

4. **项目架构与类型**：
   [Choose ONE of: 前后端分离项目 / 纯前端项目 / CLI项目 / 服务端项目 / 桌面端应用]
   
   [NOTE: ONLY IF it is a UI project (Frontend/Desktop), add the following lines:]
   - **配色方式**：
     - **主色调**：[Color name/hex, e.g., 深蓝色 (#0A2540)]
     - **辅色调**：[Color name/hex, e.g., 翠绿色 (#00D1B2)]
     - **中间偏色/其他颜色**：[Color name/hex, e.g., 浅灰色 (#F3F4F6), 警告红 (#FF3860)]
```

## Sign-off

```text
files read:       [list of key files analyzed]
scope:            on target
hard stops:       0
verification:     Ensured no design advice and strict template compliance.
```