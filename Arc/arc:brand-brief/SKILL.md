---
name: arc:brand-brief
description: "提取项目信息生成简报：查看项目并提取项目名称、项目简介（详细描述）、技术栈以及功能特点（列举多个功能点说明特性和亮点优势）。当用户要求生成项目描述或简报时触发。"
version: 1.1.0
---

# arc:brand-brief — Project Brand Brief Extractor

## Overview

This skill analyzes the current project and generates a comprehensive, factual project description. It systematically extracts the project name, detailed summary, tech stack, and key features (highlighting characteristics and advantages) from the codebase and documentation.

## Quick Contract

- **Trigger**: The user requests a project description, brief, or summary.
- **Inputs**: None (automatically analyzes the workspace).
- **Outputs**: A structured Markdown report containing the project's factual details.
- **Quality Gate**: The output must exactly match the prescribed template, providing a detailed project introduction and multiple feature points with advantages. NO architectural details or design/color suggestions should be included.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Tip** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:brand-brief` to extract project details for a comprehensive brief."

## The Iron Law

```
FACTS ONLY. DETAILED DESCRIPTIONS AND FEATURES. NO ARCHITECTURE DETAILS.
```

Your ONLY job is to describe what the project *is*, its detailed introduction, its tech stack, and its key features based on the code. Do not output architectural modes or UI color schemes.

## Workflow

1. **Information Gathering**:
   - **Name & Description**: Read `README.md`, `package.json`, `pyproject.toml`, or `Cargo.toml`. Focus on extracting or synthesizing a detailed project introduction.
   - **Tech Stack**: Analyze dependency files and source file extensions.
   - **Features**: Identify core functionalities from the code, documentation, or structure. Detail multiple feature points explaining their characteristics, highlights, and advantages.
2. **Format & Sanitize**:
   - Ensure the output is strictly in Chinese.
   - Remove any mention of architectural styles (e.g., Frontend/Backend separation, CLI, Desktop).
   - Strip out any subjective descriptions or design advice.
3. **Output**: Render the exact output template.

## Quality Gates

- **Detailed Introduction**: The project introduction must be detailed, expanding beyond a single sentence.
- **Feature Highlights**: Must list multiple features and explicitly mention their characteristics and advantages.
- **No Architecture or Design Advice**: The output must not contain architectural classifications or color scheme extractions.
- **Format Consistency**: The output must exactly follow the provided Chinese markdown template.

## Expert Standards

- Apply strict factual extraction from code artifacts only — never infer or embellish beyond what the codebase supports.
- Identify the tech stack from dependency manifests (`package.json`, `pyproject.toml`, `Cargo.toml`, `go.mod`), not from file extensions alone.

## Scripts & Commands

- Runtime main command: `arc brand-brief`
- No external scripts required; this skill operates purely through file reading and analysis.

## Red Flags

- Outputting architectural types like "前后端分离" or "CLI".
- Extracting or outputting color schemes.
- Providing a very brief, one-sentence project introduction.
- Padding the output with conversational text like "Here is your brief". (Just output the brief).

## Output Template

You MUST output EXACTLY this format, filling in the bracketed parts based on your findings:

```markdown
**项目描述**

1. **项目名称**：
   [Project Name]

2. **项目简介**：
   [Detailed description summarizing the project's core functionality, purpose, and value proposition. This should be a comprehensive paragraph.]

3. **技术栈**：
   [Comma-separated list of main languages, frameworks, and core tools]

4. **功能特点**：
   - **[Feature 1 Name]**：[Detailed explanation of the feature, highlighting its characteristics and advantages]
   - **[Feature 2 Name]**：[Detailed explanation of the feature, highlighting its characteristics and advantages]
   - **[Feature 3 Name]**：[Detailed explanation of the feature, highlighting its characteristics and advantages]
   [Add more features as discovered]
```

## When to Use

**Preferred Trigger**: The user requests a project description, feature summary, or brief.
**Typical Scenario**: Preparing a project overview, generating a factual project summary for documentation, or assessing the project's capabilities.
**Boundary Tip**: Use `arc:cartography` for structural code maps, `arc:audit` for quality diagnostics, and `arc:brand-brief` for extracting factual project metadata and features.

## Sign-off

```text
files read:       [list of key files analyzed]
scope:            on target
hard stops:       0
verification:     Ensured detailed introduction, comprehensive feature list, and NO architecture/color info.
```