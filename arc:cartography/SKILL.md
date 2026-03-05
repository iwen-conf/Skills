---
name: arc:cartography
description: "仓库地图：生成或刷新分层 codemap。"
---

# arc:cartography Skill

## Overview

Generates an incrementally maintainable layer `codemap.md` that provides stable context for downstream skills such as `arc:clarify`, `arc:build`, `arc:audit`, etc.

## Quick Contract

- **Trigger**: Need to quickly understand an unfamiliar warehouse, or downstream skills require the latest structured context.
- **Inputs**: Warehouse root path, inclusion/exclusion rules, whether to initialize for the first time.
- **Outputs**: tier `codemap.md`, optional tier JSON, `.arc/context-hub/index.json` metadata.
- **Quality Gate**: Must pass `## Quality Gates`'s incremental accuracy and index consistency check before publishing.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I'm using `arc:cartography` to update the codemap baseline first and then hand it over to downstream skills for reuse."

## The Iron Law

```
NO MAJOR CHANGE WITHOUT CURRENT CODEMAP
```

Before making key structural changes, the latest codemap must be available as a context baseline.

## Dependencies

- **Organization Contract**: Required. Following `docs/orchestration-contract.md`, catalog analysis tasks are dispatched concurrently through the runtime adaptation layer.
- **cartographer.py**: required. The path is `<skills_root>/arc:cartography/scripts/cartographer.py`.

## When to Use

- **首选触发**: The warehouse structure map (`codemap.md`) needs to be built or refreshed.
- **典型场景**: Taking over an unfamiliar warehouse for the first time and synchronizing the context after a major change in the directory structure.
- **边界提示**: Use `arc:clarify` for requirement clarification and `arc:build` for code implementation.

## Workflow

### Step 1: Check status

Prioritize checking the warehouse root directory `.slim/cartography.json`:
- Exists: Entering the change detection process
- Does not exist: execute initialization process

### Step 2: Initialization (first time only)

```bash
python3 <skills_root>/arc:cartography/scripts/cartographer.py init \
  --root ./ \
  --include "src/**/*.ts" \
  --exclude "**/*.test.ts" --exclude "dist/**" --exclude "node_modules/**"
```

After initialization it will generate:
- `.slim/cartography.json` (change detection baseline)
- `codemap.md` placeholder file in the relevant directory

### Step 3: Incremental detection and update

```bash
python3 <skills_root>/arc:cartography/scripts/cartographer.py changes --root ./
python3 <skills_root>/arc:cartography/scripts/cartographer.py update  --root ./
```

rule:
- Only update `codemap.md` of affected directories
- Unaffected directories are not rewritten

### Step 4: Optional export of hierarchical JSON

```bash
python3 <skills_root>/arc:cartography/scripts/cartographer.py export --root ./ --tier 1 --output codemap/index.json
python3 <skills_root>/arc:cartography/scripts/cartographer.py export --root ./ --tier 2 --output codemap/context.json
python3 <skills_root>/arc:cartography/scripts/cartographer.py export --root ./ --tier 3 --output codemap/full.json
```

suggestion:
- Tier 1: Quick search entrance
- Tier 2: Module relationship analysis
- Tier 3: Deep Implementation Context

### Step 5: Directory concurrency analysis and root graph summary

1. Each directory analysis task is dispatched concurrently through `schedule_task(...)` (by `docs/orchestration-contract.md`).
2. Each directory outputs/updates this directory `codemap.md`.
3. Finally, the root level `codemap.md` is aggregated to form the warehouse Atlas entrance.

### Step 6: Publish shared context metadata

Write the root-level and directory-level `codemap` products into `.arc/context-hub/index.json`. Each record contains at least:
- `path`
- `content_hash`
- `generated_at`
- `ttl_seconds`
- `expires_at`
- `producer_skill=arc:cartography`
- `refresh_skill=arc:cartography`

## Codemap writing requirements

- **Responsibility**: Describe directory responsibilities and boundaries
- **Design Patterns**: Annotate key patterns and abstraction layers
- **Data & Control Flow**: Describes the main call chain/data flow
- **Integration Points**: List dependencies and dependent parties

## Quality Gates

- The incremental process must update only the affected directories.
- Root-level and directory-level codemaps must remain reachable.
- `.arc/context-hub/index.json` Metadata must be refreshed synchronously.
- The tier derived product must be consistent with the actual structure and consumable.

## Red Flags

- Skip change detection and directly rewrite all codemaps in full
- No distinction is made between affected directories and unchanged directories
- The root-level atlas is not updated, causing the downstream entry to expire.
- Publishing without shared index results in the inability of downstream products to be reused.

Example (directory level):

```markdown
# src/services/

## Responsibility
Encapsulate the business service layer and combine the warehousing layer and external gateway.

## Design
- Adopt Service + Repository layering
- Use strategy mode to select different payment channels for implementation

## Flow
1. API Handler calls Service
2. Service completes parameter verification and transaction orchestration
3. Repository persists and returns domain objects

## Integration
- Called by `src/api/`
- Depends on `src/repositories/` and `src/gateways/`
```
