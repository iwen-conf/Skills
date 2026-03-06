---
name: "arc:init"
description: "项目索引与上下文中枢维护：初始化或更新共享索引产物；当用户说“初始化项目索引/bootstrap context/update index”时触发。"
---

# arc:init — index mode router

## Overview

This skill is the entrypoint of the `arc:init` subsystem. It chooses between full and incremental index maintenance based on the current project status:

- **Full Mode** (`arc:init --mode full`): First initialization or forced full refresh
- **Incremental Mode** (`arc:init --mode update`): Based on fingerprint detection, incremental refresh of changed index products

**Users don't need to care about selection**: For daily use, you only need to call this entrypoint, and the system will automatically decide.

Unified mode:
- Use `arc:init --mode full` for forced full rebuild.
- Use `arc:init --mode update` for explicit incremental refresh.

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
- If there is a conflict, the **Boundary Note** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:init` to determine full or incremental mode before performing index generation."

## Teaming Requirement

- Every execution must first "draw a team together" and at least clarify the three roles and responsibilities of `Owner`, `Executor` and `Reviewer`.
- If the operating environment only has a single Agent, the three-role perspective must be explicitly output during delivery to form a "decision-execution-review" closed loop before submitting the conclusion.

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

## Expert Standards

- Schema selection must be based on `fingerprint difference` and `schema version`, and the judgment evidence should be retained.
- Index writing uses `atomic update` (temporary file + replace) to avoid interruption artifacts.
- Incremental refresh needs to output a `hit/miss list` to prove that the principle of minimal impact has been implemented.
- Metadata must include `schema_version`, generator version, and hash algorithm to ensure compatibility with evolution.
- Failure recovery must support `last stable index rollback` and consistency self-check reconstruction.

## Scripts & Commands

- Automatic mode entry: `arc init`
- Force full mode: `arc init --mode full`
- Explicit incremental mode: `arc init --mode update`
- Note: `arc:init` currently does not have independent `scripts/`, and full/update is unified and arranged through runtime commands.

## Red Flags

- Forcing incremental updates without checking fingerprints.
- The reason for full/incremental judgment is missing.
- Sub-Skills are still marked initialized as complete after failure.
- The output is missing key index files but the downstream process continues.

## When to Use

- **Primary Trigger**: Need to automatically select full or incremental to maintain CLAUDE index.
- **Typical Scenario**: Daily index maintenance, but you don't want to manually judge `full/update`.
- **Boundary Note**: Use `arc:init --mode full` when it is clear that the full amount is required, and `arc:init --mode update` when it is clear that the increment is required.

## Working mode selection

```
User input → fingerprint detection → routing decision

┌─────────────────────────────────────────┐
│ arc:init (mode routing)                 │
└────────────────┬────────────────────────┘
                 │
                 ▼
        ┌────────────────┐
│ Check fingerprints │
        │ (.arc/arc:init/... ) │
        └────────┬────────┘
                 │
        ┌────────┴────────┐
        ▼                 ▼
Exist + Fresh Absent/Expired
        │                 │
        ▼                 ▼
   arc:init --mode update   arc:init --mode full
(incremental mode) (full mode)
```

### automatic routing rules

| condition | route to | illustrate |
|------|--------|------|
| `module-fingerprints.json` does not exist | `arc:init --mode full` | First initialization |
| `module-fingerprints.json` exists but git_ref does not match | `arc:init --mode update` | git changes detected |
| `module-fingerprints.json` exists but expired | `arc:init --mode update` | Snapshot expired; refresh incrementally |
| User explicitly specifies `mode=full` | `arc:init --mode full` | Force full amount |
| User explicitly specifies `mode=update` | `arc:init --mode update` | forced increment |

### Mandatory routing parameters

The user can specify the mode via prompt:

| parameter | Behavior |
|------|------|
| `force full` / `full scan` / `reinitialize` | Force routing to `arc:init --mode full` |
| `incremental` / `update` / `only changed modules` | Force routing to `arc:init --mode update` |

## Input Arguments

| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | Absolute path to project root directory |
| `mode` | string | no | Routing override: `full` / `update` |
| `project_name` | string | no | Project name |
| `depth_level` | string | no | Scan depth: `shallow` / `standard` / `deep` |
| `max_module_depth` | number | no | Maximum module depth, default 3 |
| `language` | string | no | Output language: `zh-CN` / `en` |
| `output_dir` | string | no | working directory |

> Note: This entry does not directly perform scanning and analysis, but schedules sub-Skills.

## Execution Flow

### Step 0: Pattern detection

1. **Check if `module-fingerprints.json` exists**
   - Path: `<project_path>/.arc/arc:init/context/module-fingerprints.json`
   
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
    route_to("arc:init --mode full")
elif user_force_mode == "update":
    route_to("arc:init --mode update")
elif not fingerprints_exist:
    route_to("arc:init --mode full")  # first time
elif fingerprints_stale or git_changed:
    route_to("arc:init --mode update")  # incremental refresh
else:
    route_to("arc:init --mode update")  # default incremental refresh
```

### Step 2: Execute scheduling

Use **Scheduling API** to dispatch the corresponding sub-Skill:

```typescript
// Schedule arc:init --mode full
schedule_task(
  capability_profile: "unspecified-high",
  capabilities: ["arc:init --mode full"],
prompt: `Perform full initialization...

Project path: <project_path>
Depth level: <depth_level>
Language: <language>
  ...`,
  execution_mode: "foreground"
)

// Or schedule arc:init --mode update
schedule_task(
  capability_profile: "unspecified-high",
  capabilities: ["arc:init --mode update"],
prompt: `Perform incremental update...

Project path: <project_path>
Depth level: <depth_level>
Language: <language>
  ...`,
  execution_mode: "foreground"
)
```

### Step 3: Result aggregation

1. Collect sub-skill execution results
2. Summary change statistics
3. Output final report
4. Publish `CLAUDE.md` product metadata to `.arc/context-hub/index.json` (path, hash, expiration time, refresh entry)
5. If the shared index does not exist, initialize the minimum index structure first and then write this product

## Sub-Skills

### arc:init --mode full

Full initialization mode. Applies to:
- Initializing the project index for the first time
- Force full refresh
- The fingerprint file is damaged and needs to be reconstructed

**Execution content**:
- Deep scan project structure
- Multi-Agent collaborative analysis (architecture + deep + review)
- cross review
- Generate CLAUDE.md in full
- Generate fingerprint baseline

### arc:init --mode update

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
| Force full amount | `arc init --mode full` |
| forced increment | `arc init --mode update` |
| View help | `arc init --help` |

## Status feedback example

```
[Arc Init] Project: my-project
=== Pattern Detection ===
├── Fingerprint file: exists (generated on 2026-02-25)
├── Git changes: abc1234 → def5678 (15 file changes)
└── Routing decisions: incremental updates (arc:init --mode update)

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
arc:init (this skill - mode routing)
├── arc:init --mode full (full initialization)
└── arc:init --mode update (incremental update)

Downstream consumers:
arc:clarify / arc:decide / arc:build / arc:e2e / arc:audit
↑ Read CLAUDE index product through `.arc/context-hub/index.json`

Dependencies:
arc:init --mode update requires the fingerprint baseline generated by arc:init --mode full

Publishing relationship:
arc:init must publish this generated/updated CLAUDE.md metadata to the shared index
```

## troubleshooting

| Condition | solution |
|------|----------|
| Fingerprint file does not exist | Prompt to run `arc:init` or `arc:init --mode full` |
| Fingerprint file is damaged | Prompt to run `arc:init --mode full` Rebuild |
| Git repository does not exist | Only supports git repository |
| Sub-Skill execution failed | View `.arc/arc:init/` working directory log |

## Call example

```bash
# First initialization (automatically routed to full)
arc init /path/to/new-project

# Daily updates (automatically routed to update)
arc init /path/to/existing-project

# Force full amount
arc init --mode full /path/to/project

# forced increment
arc init --mode update /path/to/project
```
