---
name: arc:cartography
description: "仓库地图与分层 codemap 生成/刷新；当用户说“梳理代码结构/仓库导览/repo map/codebase overview”时触发。"
---

# arc:cartography Skill

## Overview

Generates an incrementally maintainable layer `codemap.md` that provides stable context for downstream skills such as `arc:clarify`, `arc:build`, `arc:audit`, etc.

## Quick Contract

- **Trigger**: Need to quickly understand an unfamiliar repository, or downstream skills require the latest structured context.
- **Inputs**: Repository root path; optional inclusion/exclusion rules and first-run initialization hint.
- **Outputs**: tier `codemap.md`, optional tier JSON, `.arc/context-hub/index.json` metadata.
- **Quality Gate**: Must pass `## Quality Gates`'s incremental accuracy and index consistency check before publishing.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Border Tip** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I'm using `arc:cartography` to update the codemap baseline first and then hand it over to downstream skills for reuse."

## Teaming Requirement

- Each implementation must first "draw a team together" and clearly define at least three roles and responsibilities of `Owner`, `Executor`, and `Reviewer`.
- If the operating environment only has a single Agent, the three-role perspective must be explicitly output during delivery to form a "decision-execution-review" closed loop before submitting the conclusion.

## The Iron Law

```
NO MAJOR CHANGE WITHOUT CURRENT CODEMAP
```

Before making key structural changes, the latest codemap must be available as a context baseline.

## Dependencies

- **Organization Contract**: Required. Following `docs/orchestration-contract.md`, catalog analysis tasks are dispatched concurrently through the runtime adaptation layer.
- **cartographer.py**: required. The path is `<skills_root>/arc:cartography/scripts/cartographer.py`.

## When to Use

- **Preferred trigger**: The repository structure map (`codemap.md`) needs to be built or refreshed.
- **Typical scenario**: Taking over an unfamiliar repository for the first time and synchronizing the context after a major change in the directory structure.
- **Boundary Tip**: Use `arc:clarify` for requirement clarification and `arc:build` for code implementation.

## Workflow

### Step 1: Check status

Prioritize checking the repository root directory `.slim/cartography.json`:
- Exists: Entering the change detection process
- Does not exist: execute initialization process

### Step 2: Initialization (first time only)

```bash
python3 <skills_root>/arc:cartography/scripts/cartographer.py init \
  --root ./ \
  --include "src/**/*.ts" \
  --exclude "**/*.test.ts" \
  --exclude ".git/**" \
  --exclude "dist/**" \
  --exclude "build/**" \
  --exclude ".next/**" \
  --exclude "coverage/**" \
  --exclude "node_modules/**" \
  --exclude "vendor/**" \
  --exclude ".venv/**" \
  --exclude "tmp/**" \
  --exclude "artifacts/**"
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

### Load-safety defaults

1. Run `arc:cartography` against the real git root, not against a parent workspace that only groups multiple repositories.
2. Prefer `changes` or a tightly scoped `update` before `init`; reserve full initialization for first run or damaged state.
3. Always exclude generated, vendored, cache, and package-manager directories unless the user explicitly asks to map them.
4. Treat `cartographer.py` as a batch command. Do not wrap it in a background watcher, polling loop, or persistent Python service.
5. When downstream work only needs a small structure answer, prefer `rg`, `fd`, `git`, and existing `.arc/context-hub` artifacts instead of rebuilding the whole codemap.

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

Tier schema and examples:
- Canonical schema: [`../../schemas/codemap.schema.json`](../../schemas/codemap.schema.json)
- Tier 1/2/3 examples: [`references/tiered-json-examples.md`](references/tiered-json-examples.md)

### Step 5: Directory concurrency analysis and root graph summary

1. For medium or large repositories, directory analysis tasks can be dispatched concurrently through `schedule_task(...)` (by `docs/orchestration-contract.md`).
2. Each directory outputs/updates this directory `codemap.md`.
3. Finally, the root level `codemap.md` is aggregated as the repository entry map.

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

## Expert Standards

- Map layers need to be aligned with the `C4` perspective (Context/Container/Component/Code) and map to each other.
- The structural view needs to meet the `ISO/IEC 42010` focus traceability: roles, perspectives, and evidence sources are clear.
- Change detection uses the `churn × dependency` hotspot method to output high-risk directories and key dependency nodes.
- Each update must output `incremental diff list` (add/delete/move/rename) for direct downstream consumption.
- The index product must have `reproducible signature`: input snapshot, hash, generation time, version number.

## Scripts & Commands

- Initialization codemap: `python3 Arc/arc:cartography/scripts/cartographer.py init --root <project_path>`
- Detect changes: `python3 Arc/arc:cartography/scripts/cartographer.py changes --root <project_path>`
- Incremental update: `python3 Arc/arc:cartography/scripts/cartographer.py update --root <project_path>`
- Export layered JSON: `python3 Arc/arc:cartography/scripts/cartographer.py export --root <project_path> --tier 2 --output codemap.tier2.json`
- Runtime main command: `arc cartography`

## Red Flags

- Skip change detection and directly rewrite all codemaps in full
- No distinction is made between affected directories and unchanged directories
- The root-level map is not updated, causing the downstream entry to expire.
- Publishing without shared index results in the inability of downstream products to be reused.
- Running full mapping on `node_modules`, build outputs, vendored trees, or cache directories by default.
- Leaving long-lived Python helper processes behind after codemap generation.

Example (directory level):

```markdown
# src/services/

## Responsibility
Encapsulate the business service layer and connect the repository layer and external gateways.

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
