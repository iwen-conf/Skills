---
name: "arc:init:update"
description: "日常迭代后使用：基于变更模块增量更新 CLAUDE.md，避免全量扫描。"
---

# arc:init:update — Incremental update

## Overview

This Skill is the incremental mode of the `arc:init` subsystem. It detects project changes through fingerprint comparison and git diff, and only analyzes and updates the affected CLAUDE.md file.

**Difference from `arc:init:full`**:
- `arc:init:full`: Full scan + Full multi-Agent analysis + Full generation
- `arc:init:update`: Change detection + selective analysis + incremental generation

**Core Advantages**:
- Skip scanning and analysis of unchanged modules
- Reduce Agent calls (only analyze change modules)
- Leave manual edits to CLAUDE.md unchanged
- Generate incremental reports to clearly demonstrate the scope of changes

## Quick Contract

- **Trigger**: The baseline has been initialized and only recently changed modules need to be synchronized.
- **Inputs**: Project path, baseline fingerprint, optional `force_modules`, `since` and `dry_run`.
- **Outputs**: Incremental change report, affected CLAUDE update results, refreshed fingerprint status.
- **Quality Gate**: The fingerprint difference and minimum modification check of `## Quality Gates` must be passed before writing.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I'm using `arc:init:update` to do change detection first and then update only the affected indexes."

## The Iron Law

```
NO INCREMENTAL WRITE WITHOUT FINGERPRINT DIFF
```

No incremental writes are allowed without evidence of fingerprint differences.

## Workflow

1. Read baseline fingerprints, snapshots and generation plans.
2. Combined with git diff to recalculate fingerprints and classify changes.
3. Only the changed modules and necessary ancestor indexes are updated.
4. Generate incremental reports and refresh baseline status.

## Quality Gates

- Each update action must correspond to the change classification result.
- Modules that have not been changed must remain intact and cannot be modified by mistake.
- User manual blocks must be retained according to rules.
- Increment reports must contain a summary of the baseline ref and this difference.

## Red Flags

- Skip fingerprint comparison and update the module based on guesswork.
- The scope of the change spreads to unrelated directories.
- Overrides user manual editing areas.
- No change report but claims the update is complete.

## Prerequisites

**The following conditions must be met to run**:

1. `.arc/init/context/module-fingerprints.json` exists
2. `.arc/init/context/project-snapshot.md` exists
3. `.arc/init/context/generation-plan.md` exists

> If not satisfied, the user is prompted to run `arc:init` or `arc:init:full` for full initialization.

## When to Use

- **首选触发**: `arc:init` baseline already exists, requiring low-cost synchronization of affected modules.
- **典型场景**: Update CLAUDE index after daily code additions, deletions and modifications.
- **边界提示**: Use `arc:init` / `arc:init:full` instead when the fingerprint baseline is missing or the index is severely out of date.

## Input Arguments

| parameter | type | Required | default value | illustrate |
|------|------|------|--------|------|
| `project_path` | string | yes | — | Absolute path to project root directory |
| `force_modules` | string[] | no | — | Force an update of the specified module (even if the fingerprint has not changed) |
| `skip_agents` | boolean | no | false | Skip Agent analysis and regenerate based on file changes only |
| `since` | string | no | auto | Git ref starting point (defaults from git_ref in fingerprints) |
| `dry_run` | boolean | no | false | Only change reports are output, no files are actually modified. |
| `language` | string | no | zh-CN | Output language |

## Dependencies

* **Organization Contract**: Required. Following `docs/orchestration-contract.md`, scheduling is implemented through the runtime adaptation layer.
* **ace-tool (MCP)**: Required. Semantic search project code structure.
* **Unified Dispatch API**: Required. Schedule oracle/deep/visual-engineering Agent.
* **Git**: required. Change detection relies on git commands.

## Critical Rules

1. **Fingerprint Driver**
 - All change judgments must be based on `module-fingerprints.json` comparison.
 - There should be no guessing about which modules need to be updated.

2. **Minimum modification principle**
 - Modify only the CLAUDE.md of the changed module and its ancestors.
 - CLAUDE.md for unchanged modules remains unchanged.
 - Preserve manually edited content by the user (regions marked via `<!-- user -->`).

3. **Cascading Updates**
 - Leaf module changes → update leaves → update index/mermaid parts of all ancestors.
 - Does not regenerate the entire content of the ancestor, only updates the affected chapters.

4. **Agent reduction**
 - ADDED Module: Complete 3 Agent Analysis
 - MODIFIED (key_files changes): 2 Agent (deep + visual-engineering)
 - MODIFIED (source changes only): 1 Agent (deep)
 - DELETED/RENAMED：0 Agent

## Instructions (execution process)

### Phase U1: Change Detection

**Goal**: Identify which modules have changed and classify the types of changes.

#### Step U1.1: Load baseline

1. Read `.arc/init/context/module-fingerprints.json`
2. Read `.arc/init/context/generation-plan.md`
3. Read `.arc/init/context/project-snapshot.md`
4. Record baseline git_ref

#### Step U1.2: Git change detection

```bash
# Get a list of changed files
git diff --name-only <baseline_git_ref>..HEAD

# Get rename detection
git diff --name-status --diff-filter=R <baseline_git_ref>..HEAD

# Get deleted files
git diff --name-status --diff-filter=D <baseline_git_ref>..HEAD
```

#### Step U1.3: Recalculate fingerprint

For all module directories involved in git diff:

1. Calculate the current `key_files` hash
2. Calculate current `source_tree_hash`
3. Calculate the current `claude_md_hash` (if CLAUDE.md exists)

#### Step U1.4: Change classification

Compare baseline fingerprint vs current fingerprint for each module:

| Change type | Detection conditions | priority |
|----------|----------|--------|
| `ADDED` | Directory is not in fingerprints, score >= 4 | high |
| `DELETED` | The directory is in fingerprints but the directory does not exist | high |
| `RENAMED` | git detect rename or path matching similar directories | middle |
| `MODIFIED_KEY` | `key_files` Hash changes (manifest/configuration changes) | high |
| `MODIFIED_SOURCE` | Only `source_tree_hash` changes | middle |
| `MODIFIED_CLAUDE_MD` | Only `claude_md_hash` changes (manual editing by user) | Low |
| `UNCHANGED` | All hashes are consistent | — |

#### Step U1.5: Add new module scan

For potential new directories:

1. Rated by `references/scan-heuristics.md`
2. Directories with score >= 4 are marked `ADDED`
3. Collect new module metadata

#### Step U1.6: Ancestor cascade tag

For all `ADDED` / `DELETED` / `RENAMED` / `MODIFIED_*` modules:

1. Find all ancestor directories (parent, grandparent...up to root)
2. Mark ancestor as `STALE_PARENT`

#### Step U1.7: Generate change report

Output `context/updates/update-<timestamp>.md`:

```markdown
# Incremental update change report

- **Updated**: <ISO 8601>
- **Baseline Git Ref**: <old_ref> → <new_ref>
- **Baseline Time**: <timestamp>

## Summary of changes

| module | Change type | Agent analysis | CLAUDE.md operations |
|------|----------|------------|----------------|
| src/new-feature/ | ADDED | oracle+deep+visual-engineering | New |
| src/auth/ | MODIFIED_KEY | deep+visual-engineering | Merge updates |
| src/utils/ | MODIFIED_SOURCE | deep | Merge updates |
| src/legacy/ | DELETED | — | delete |
| src/ | STALE_PARENT | — | Index update |

## File change statistics
- New files: N
- Modify file: M
- Delete file: D

## Detailed change list
<git diff --stat output>
```

> When `dry_run=true` is set, it will end immediately after outputting the report, and subsequent stages will not be executed.

---

### Phase U2: Selective Analysis

**Goal**: Start analysis only for modules that require Agent analysis.

#### Step U2.1: Determine the scope of analysis

```
Full analysis required (3 Agents):
 - All ADDED modules

Partial analysis required (2 Agent):
 - All MODIFIED_KEY modules (deep + visual-engineering, skip oracle)

Minimum analysis required (1 Agent):
 - All MODIFIED_SOURCE modules (deep only)

No analysis required:
 - DELETED / RENAMED / STALE_PARENT / MODIFIED_CLAUDE_MD
```

#### Step U2.2: Concurrent Agent Analysis

**ADDED module analysis** (same as `arc:init:full`):

```typescript
// oracle analysis
dispatch_job(
 role: "oracle",
 capabilities: ["arc:init:update"],
 execution_mode: "background",
description: "oracle architecture analysis - new module",
prompt: `Analyze the architecture of the following new modules...

Module list: <added_modules>
Project path: <project_path>
Output to: <output_dir>/agents/oracle/analysis-update.md`
)

// deep analysis
dispatch_job(
 lane: "deep",
 capabilities: ["arc:init:update"],
 execution_mode: "background",
description: "deep engineering analysis",
 prompt: `...`
)

// visual-engineering analysis
dispatch_job(
 lane: "visual-engineering",
 capabilities: ["arc:init:update", "frontend-ui-ux"],
 execution_mode: "background",
description: "visual-engineering DX analysis",
 prompt: `...`
)
```

**MODIFIED_KEY module analysis** (2 Agent):

```typescript
// only deep + visual-engineering, skip oracle
dispatch_job(lane: "deep", ...),
dispatch_job(lane: "visual-engineering", ...)
```

**MODIFIED_SOURCE module analysis** (1 Agent):

```typescript
// only deep, shallow depth
dispatch_job(
 lane: "deep",
 capabilities: ["arc:init:update"],
 execution_mode: "background",
description: "deep shallow analysis - source code changes",
prompt: `The following modules only have source code changes, for shallow analysis...

Module list: <modified_source_modules>
Analysis depth: shallow (only changed files are analyzed)`
)
```

#### Step U2.3: Skip condition

```
IF no ADDED and no MODIFIED_* modules:
 - Skip Phase U2
 - Enter Phase U3 directly (only do file operations)

IF skip_agents=true:
 - Skip Phase U2
 - Generate directly using existing snapshot data
```

---

### Phase U3: Incremental Generation

**Goal**: Update/create/delete affected CLAUDE.md files.

#### Step U3.1: Handle DELETED module

1. Delete CLAUDE.md in the module directory
2. Record deletion operations to the change log

#### Step U3.2: Process the RENAMED module

1. Move CLAUDE.md to a new path
2. Update path references inside CLAUDE.md
3. Update breadcrumbs

#### Step U3.3: Handle ADDED module

1. Use Agent to analyze results
2. Generate new CLAUDE.md by `references/claude-md-schema.md`
3. Add Changelog entry (`<timestamp> arc:init:update add module index`)

#### Step U3.4: Process MODIFIED modules (merge updates)

**CLAUDE.md merge strategy**:

```
1. Read existing CLAUDE.md → parse into section map
2. Identify reserved areas:
 - <!-- user-start --> ... <!-- user-end --> mark area
 - Changelog chapter (append only)
 - Custom section (not in schema definition)

3. Identify the update area:
 - Decide which sections to update based on the type of change:

 MODIFIED_KEY:
 - Module responsibilities (if new analysis)
 - Key dependencies (manifest changes)
 - Operation and development (scripts changes)

 MODIFIED_SOURCE:
 - Architecture diagram (when the structure changes)
 - List of associated files (always updated)

 MODIFIED_CLAUDE_MD:
 - Skip (user manual editing, no overwriting)

4. To perform the merge:
 - Reserved area: Leave as is
 - Update area: Replace with Agent analysis results
 - Changelog: Append new entries

5. Write merged CLAUDE.md
```

#### Step U3.5: Update ancestor CLAUDE.md

For all `STALE_PARENT` modules:

1. **Module Index Table**:
 - ADDED: Add new item
 - DELETED: Delete an entry
 - RENAMED: update path

2. **Mermaid Structure Diagram**:
 - ADDED: add node + click link
 - DELETED: delete node
 - RENAMED: Update node ID

3. **Changelog**：
 - Append incremental update entries

> Note: Only the affected chapters are updated, the entire CLAUDE.md is not regenerated.

#### Step U3.6: Update fingerprint file

After the generation is complete:

1. Recalculate fingerprints for all changed modules
2. Update `module-fingerprints.json`
3. Update `git_ref` to the current HEAD

---

### Phase U4: Validation & Report

**Goal**: Verify the update results and generate incremental reports.

#### Step U4.1: Selective verification

Only verify changed/new CLAUDE.md files:

- Structure verification (complete required chapters)
- Table validation (column number alignment)
- Reference verification (mermaid click + breadcrumb)

#### Step U4.2: Ancestor consistency verification

Verify that the ancestor CLAUDE.md has been updated correctly:

- Module index entries correspond to actual directories
- Mermaid node is consistent with index
- All click links are valid

#### Step U4.3: Update snapshot

Update `project-snapshot.md`:

```markdown
## Snapshot metadata
- **Generation time**: <timestamp after update>
- **Project Path**:<unchanged>
- **Git Ref**：<new HEAD>
- **Update type**: incremental
- **Number of change modules**: N
```

#### Step U4.4: Generate update summary

Update `summary.md`:

```markdown
# Incremental update summary

## Statistics for this update
- Update time: <timestamp>
- Baseline version: <old_git_ref>
- Current version: <new_git_ref>

## change module
| module | Change type | Agent call | CLAUDE.md operations |
|------|----------|------------|----------------|

## Resource saving
- Skip scanning directories: N
- Skip Agent analysis: M modules
- Preserve manual editing: K

## Verification result
- Structure check: ✅
- Table validation: ✅
- Reference verification: ✅
```

---

## Artifacts & Paths

```
<project_path>/.arc/init/
├── context/
│ ├── project-snapshot.md # Update metadata
│ ├── generation-plan.md # Update module list
│ ├── module-fingerprints.json # Update fingerprint baseline
│ └── updates/ # Add new directory
│ └── update-<timestamp>.md # Incremental report
├── agents/
│ ├── oracle/
│ │ └── analysis-update.md # New: this analysis
│ ├── deep/
│ │ └── analysis-update.md
│ └── visual-engineering/
│ └── analysis-update.md
└── summary.md # Update
```

---

## Change type processing matrix

| Change type | Agent analysis | CLAUDE.md operations | Ancestry update |
|----------|------------|----------------|----------|
| `ADDED` | oracle + deep + visual-engineering | Create new complete file | index + mermaid |
| `DELETED` | — | Delete files | index + mermaid |
| `RENAMED` | — | Move + path update | index + mermaid |
| `MODIFIED_KEY` | deep + visual-engineering | Merge updates | may be needed |
| `MODIFIED_SOURCE` | deep (shallow) | Merge updates | Usually not required |
| `MODIFIED_CLAUDE_MD` | — | jump over | unnecessary |
| `STALE_PARENT` | — | index + mermaid update | — |
| `UNCHANGED` | — | jump over | unnecessary |

---

## Error handling

| Condition | deal with |
|------|------|
| fingerprints does not exist | Prompt to run `arc:init` first and exit. |
| git is not a warehouse | Prompt that git repository is required, exit |
| Baseline git_ref does not exist | Fallback to file timestamp comparison |
| Agent timeout | Continue with the completed analysis and mark the timeout module |
| merge conflicts | User editing reserved, tags require manual review |

---

## Relationship to other Skills

```
arc:init (scheduler)
├── arc:init:full (full initialization) → generate module-fingerprints.json
└── arc:init:update (incremental update) → consume module-fingerprints.json
```

**Call chain**:
1. User first runs `arc init` → automatically routed to `arc:init:full`
2. Subsequent runs `arc init` → automatically route to `arc:init:update` (if fingerprints exist)
3. User explicit `arc init full` → force full
4. User explicit `arc init update` → force increment
