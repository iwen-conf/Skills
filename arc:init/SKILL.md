---
name: "arc:init"
description: "维护 CLAUDE.md 索引的默认入口：自动判断走全量重建还是增量更新。"
---

# arc:init — Intelligent scheduler

## Overview

This Skill is the **intelligent scheduling portal** of the `arc:init` subsystem, which automatically selects the optimal working mode based on the project status:

- **Full Mode** (`arc:init:full`): First initialization or forced full refresh
- **Incremental Mode** (`arc:init:update`): Based on fingerprint detection, intelligent incremental update

**Users don’t need to care about selection**: For daily use, you only need to call this entrance, and the system will automatically judge.

## Quick Contract

- **Trigger**: Need to create or refresh the project CLAUDE index system, but do not want to manually select full/update.
- **Inputs**: `project_path`, optional `mode`, scan depth and output parameters.
- **Outputs**: Mode decision results, sub-skill execution records and updated index products.
- **Quality Gate**: The routing and product integrity check of `## Quality Gates` must pass after execution.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:init` to determine full or incremental mode before performing index generation."

## The Iron Law

```
NO INDEX UPDATE WITHOUT MODE DECISION AND BASELINE CHECK
```

Index writing must not be started until mode decision and baseline verification are completed.

## Workflow

1. Check fingerprint and snapshot status to determine full/update routing.
2. Assemble the input parameters and deliver them to the corresponding sub-skill.
3. Track execution results and verify product integrity.
4. Output mode selection basis and subsequent maintenance recommendations.

## Quality Gates

- Routing decisions must be accompanied by conditional hit descriptions.
- Sub-Skill scheduling parameters must be complete and reproducible.
- Products must contain up-to-date metadata and timestamps.
- An executable fallback path must be given on failure.

## Red Flags

- Forcing incremental updates without checking fingerprints.
- The reason for full/incremental judgment is missing.
- Sub-Skills are still marked initialized as complete after failure.
- The output is missing key index files but the downstream process continues.

## When to Use

- **首选触发**: Need to automatically select full or incremental to maintain CLAUDE index.
- **典型场景**: Daily index maintenance, but you don’t want to manually judge `full/update`.
- **边界提示**: Use `arc:init:full` when it is clear that the full amount is required, and `arc:init:update` when it is clear that the increment is required.

## Working mode selection

```
User input → fingerprint detection → routing decision

┌─────────────────────────────────────────┐
│ arc:init (intelligent scheduling) │
└────────────────┬────────────────────────┘
                 │
                 ▼
        ┌────────────────┐
│ Check fingerprints │
        │ (.arc/init/... ) │
        └────────┬────────┘
                 │
        ┌────────┴────────┐
        ▼                 ▼
Exist + Fresh Absent/Expired
        │                 │
        ▼                 ▼
   arc:init:update   arc:init:full
(incremental mode) (full mode)
```

### automatic routing rules

| condition | route to | illustrate |
|------|--------|------|
| `module-fingerprints.json` does not exist | `arc:init:full` | First initialization |
| `module-fingerprints.json` exists but git_ref does not match | `arc:init:update` | git changes detected |
| `module-fingerprints.json` exists but expired (>7 days) | `arc:init:update` | Regularly updated |
| User explicitly specifies `mode=full` | `arc:init:full` | Force full amount |
| User explicitly specifies `mode=update` | `arc:init:update` | forced increment |

### Mandatory routing parameters

The user can specify the mode via prompt:

| parameter | Behavior |
|------|------|
| `force full` / `full scan` / `reinitialize` | Force routing to `arc:init:full` |
| `incremental` / `update` / `only changed modules` | Force routing to `arc:init:update` |

## Input Arguments

| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | Absolute path to project root directory |
| `project_name` | string | no | Project name |
| `depth_level` | string | no | Scan depth: `shallow` / `standard` / `deep` |
| `max_module_depth` | number | no | Maximum module depth, default 3 |
| `language` | string | no | Output language: `zh-CN` / `en` |
| `output_dir` | string | no | working directory |

> Note: This entry does not directly perform scanning and analysis, but schedules sub-Skills.

## Execution Flow

### Step 0: Pattern detection

1. **Check if `module-fingerprints.json` exists**
   - Path: `<project_path>/.arc/init/context/module-fingerprints.json`
   
2. **Check Snapshot Freshness**
   - Generation time < 7 days: considered as valid baseline
   - Generation time >= 7 days: prompt expired, it is recommended to update

3. **Check Git changes**
   - Compare `fingerprints.git_ref` to current HEAD
   - Inconsistency: There are unsynchronized changes

4. **Check for explicit user specification**
   - prompt contains the forced mode keyword

### Step 1: Routing decision

Make routing decisions based on detection results:

```python
if user_force_mode == "full":
    route_to("arc:init:full")
elif user_force_mode == "update":
    route_to("arc:init:update")
elif not fingerprints_exist:
route_to("arc:init:full") # first time
elif fingerprints_stale or git_changed:
route_to("arc:init:update") # increment
else:
route_to("arc:init:update") #Default increment
```

### Step 2: Execute scheduling

Use **Scheduling API** to dispatch the corresponding sub-Skill:

```typescript
// Schedule arc:init:full
schedule_task(
  workstream: "unspecified-high",
  capabilities: ["arc:init:full"],
prompt: `Perform full initialization...

Project path: <project_path>
Depth level: <depth_level>
Language: <language>
  ...`,
  run_mode: "foreground"
)

// Or schedule arc:init:update
schedule_task(
  workstream: "unspecified-high",
  capabilities: ["arc:init:update"],
prompt: `Perform incremental update...

Project path: <project_path>
Depth level: <depth_level>
Language: <language>
  ...`,
  run_mode: "foreground"
)
```

### Step 3: Result aggregation

1. Collect sub-skill execution results
2. Summary change statistics
3. Output final report
4. Publish `CLAUDE.md` product metadata to `.arc/context-hub/index.json` (path, hash, expiration time, refresh entry)
5. If the shared index does not exist, initialize the minimum index structure first and then write this product

## Sub-Skills

### arc:init:full

Full initialization mode. Applies to:
- Initializing the project index for the first time
- Force full refresh
- The fingerprint file is damaged and needs to be reconstructed

**Execution content**:
- Deep scan project structure
- Multi-Agent collaborative analysis (oracle + deep + momus)
- cross review
- Generate CLAUDE.md in full
- Generate fingerprint baseline

### arc:init:update

Incremental update mode. Applies to:
- Routine maintenance updates
- Synchronize after some module changes
- Periodic index refresh

**Execution content**:
- Change detection (fingerprint comparison + git diff)
- Selective Agent Analysis
- Incremental generation of CLAUDE.md
- Update fingerprint baseline

## Quick Usage

| scene | Calling method |
|------|----------|
| First initialization | `arc init` → automatically route to full |
| daily updates | `arc init` → automatically route to update |
| Force full amount | `arc init full` |
| forced increment | `arc init update` |
| View help | `arc init --help` |

## Status feedback example

```
[Arc Init] Project: my-project
=== Pattern Detection ===
├── Fingerprint file: exists (generated on 2026-02-25)
├── Git changes: abc1234 → def5678 (15 file changes)
└── Routing decisions: incremental updates (arc:init:update)

=== Incremental update ===
├── Change detection... [Complete]
│ ├── New modules: 2
│ ├── Modification module: 5
│ └── Delete module: 1
├── Agent analysis... [Completed]
│ ├── Complete analysis of new modules: 2 modules
│ └── Shallow analysis of modified modules: 5 modules
├── Incremental generation... [Complete]
│ ├── New CLAUDE.md: 2
│ ├── Update CLAUDE.md: 7
│ └── Delete CLAUDE.md: 1
└── Verification... [Passed]

A total of 10 CLAUDE.md files were updated.
```

## Relationship to other Skills

```
arc:init (this Skill - Intelligent Scheduling)
├── arc:init:full (full initialization)
└── arc:init:update (incremental update)

Downstream consumers:
arc:clarify / arc:decide / arc:build / arc:e2e / arc:audit
↑ Read CLAUDE index product through `.arc/context-hub/index.json`

Dependencies:
arc:init:update requires the fingerprint baseline generated by arc:init:full

Publishing relationship:
arc:init must publish this generated/updated CLAUDE.md metadata to the shared index
```

## troubleshooting

| Condition | solution |
|------|----------|
| Fingerprint file does not exist | Prompt to run `arc:init` or `arc:init:full` |
| Fingerprint file is damaged | Prompt to run `arc:init:full` Rebuild |
| Git repository does not exist | Only supports git repository |
| Sub-Skill execution failed | View `.arc/init/` working directory log |

## Call example

```bash
# First initialization (automatically routed to full)
arc init /path/to/new-project

# Daily updates (automatically routed to update)
arc init /path/to/existing-project

# Force full amount
arc init full /path/to/project

# forced increment
arc init update /path/to/project
```
