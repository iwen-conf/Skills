---
name: "arc:clarify"
description: "需求模糊或约束不全时使用：补齐上下文并生成可直接执行的高质量提示。"
---

# Question Refiner

## Overview

Systematically supplement user questions with contextual information. By scanning the CLAUDE.md multi-level index of the project, combined with interactive questioning, fuzzy requirements are transformed into structured, executable enhanced prompts.

## Quick Contract

- **Trigger**: User needs are vaguely expressed, context is insufficient, and acceptance criteria are missing.
- **Inputs**: Original question, item index context, clarification Q&A results.
- **Outputs**: Structured enhanced prompt (Context/Task/Constraints/Success Criteria).
- **Quality Gate**: The executability check of `## Quality Gates` must pass before downstream execution.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:clarify`, first complete the context and constraints, and then generate the executable prompt."

## The Iron Law

```
NO EXECUTION PROMPT WITHOUT CONTEXT, CONSTRAINTS, AND SUCCESS CRITERIA
```

The execution phase must not be entered before the context, constraints and acceptance criteria are formed.

## Workflow

1. Prioritize reads from shared indexes and CLAUDE/codemap contexts.
2. Compare the user's original problem with gap analysis.
3. Clarify uncertainties with key questions.
4. Generate a structured enhanced prompt and write to the shared directory.

## Quality Gates

- Enhanced prompt must contain Context/Task/Constraints/Success Criteria.
- Key terms must be unified and reusable downstream.
- Clarification questions should be minimal and of high value to avoid noisy questioning.
- Clearly label contextual sources and freshness.

## Red Flags

- Rewrite requirements directly without context scanning.
- It only repeats the user's original words without supplementing actionable information.
- Lack of success criteria but delivered to the implementation phase.
- Critical constraint violations are not explicitly prompted.

## When to Use

- **首选触发**: The requirements are vague or the context is insufficient and need to be converted into executable task descriptions.
- **典型场景**: Module boundaries are unclear, constraints are missing, and acceptance criteria are undefined.
- **边界提示**: Use `arc:decide` when multiple-solution disputes need to be demonstrated. If the requirements are clear, they will be implemented directly downstream.

## Core Pattern

### Step 1: Project index scan

Read the shared context index first, then downgrade according to priority:

1. **Read first** `.arc/context-hub/index.json`, find reusable products:
   - `CLAUDE.md` level index
   - `codemap.md`（arc:cartography）
   - Recent arc:audit/score/implementation handoff
2. **If the index is missing or invalid**, scan the project root directory and subdirectories to collect CLAUDE.md:

```bash
# Scan all CLAUDE.md of the current project
find . -name "CLAUDE.md" -type f
```

3. **If it is still not enough**, finally fall back to ace-tool source code semantic search.

### Step 2: Context extraction

First extract from the shared index product, and then add CLAUDE.md level information:

- **Root level CLAUDE.md**: project vision, technology stack overview, module relationship
- **Module-level CLAUDE.md**: The architecture, coding specifications, and dependencies of a specific module
- **codemap.md**: Directory responsibilities, module boundaries, cross-directory calling relationships
- **arc:audit/score/handoff**: Identified risk points, quality bottlenecks, and change context

Use Grep/Read to read relevant content and build a project context portrait.

### Step 3: Gap analysis

Compare the user's original question with the project context to identify missing dimensions:

| missing dimension | Example |
|---------|------|
| technology boundaries | Frontend/backend/full stack? Specific module? |
| Constraints | Performance requirements? Compatibility requirements? Time limit? |
| Dependency impact | Does the change involve other modules? Need to migrate? |
| Acceptance criteria | How is it completed? How to verify success? |

### Step 4: Interactive refinement

Use `AskUserQuestion` to ask the user 1-4 key questions. Each question should:

- Findings based on gap analysis
- Offers 2-4 options and allows users to customize supplements
- State the question clearly and avoid jargon

### Step 5: Prompt synthesis

Assemble a structured enhanced prompt, consisting of four parts:

```markdown
## Context
<Project context summary, including relevant modules, technology stack, specifications>

## Task
<User's original task description>

## Constraints
<Recognized constraints>

## Success Criteria
<How to verify task completion>
```

### Step 6: Output

Write the complete enhanced prompt to the shared directory:

```
<workdir>/.arc/arc:decide/<task-name>/context/enhanced-prompt.md
```

## Quick Reference

| stage | tool | output |
|------|------|------|
| scanning | Read + Glob/Find + ace-tool | Shared index/CLAUDE.md/source context |
| extract | Read | Project context summary |
| analyze | Artificial | gap list |
| Ask a question | AskUserQuestion | User answers |
| synthesis | Write | enhanced-prompt.md |

## Anti-Patterns

**CRITICAL: The following behaviors are FORBIDDEN in arc:clarify execution:**

### Scanning Anti-Patterns

- **Skip-Scan-Ask**: Jumping straight to questions without reading CLAUDE.md hierarchy first — questions lack context
- **Cache Ignorance**: Not checking `.arc/context-hub/index.json` when valid cache exists — wastes prior work
- **Stale Context Usage**: Using expired cache (24h+) without verification — causes incorrect assumptions

### Question Anti-Patterns

- **Question Overload**: Asking 5+ questions at once — overwhelming, prioritize to 1-3 key questions
- **Technical Jargon**: Using developer terms with non-technical users — use business language
- **Leading Questions**: Phrasing questions to bias toward a specific answer — neutral phrasing required
- **No Custom Option**: Forcing users to pick from predefined options only — always allow free-form input

### Analysis Anti-Patterns

- **Literal Interpretation**: Taking user's words at face value without identifying implicit assumptions
- **Scope Skipping**: Not clarifying module boundaries when project has multiple sub-systems
- **Constraint Blindness**: Not identifying performance, compatibility, or timeline constraints

### Output Anti-Patterns

- **Missing Success Criteria**: Enhanced prompt without verification checklist — how to know it's done?
- **Orphaned Output**: Not writing enhanced-prompt.md to `.arc/arc:decide/` — breaks arc:decide consumption
## Integration

After this Skill is completed, it is recommended to continue calling `arc:decide` Skill for multi-Agent deliberation.

If the shared product is found to be invalid during the refinement process:
- CLAUDE index invalidation → trigger `arc:init:update` (`arc:init:full` if necessary)
- codemap invalid → trigger `arc:cartography` update
- score/review data invalidation → trigger `score` module refresh (triggered by `arc:release` arrangement) / `arc:audit` update

```
Problem refinement completed. Enhanced prompt has been written:
.arc/arc:decide/<task-name>/context/enhanced-prompt.md

You can continue to call `arc decide` for multi-Agent deliberation.
```
