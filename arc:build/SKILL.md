---
name: "arc:build"
description: "代码交付与实施落地：按既定方案改代码并给出验证与交接；当用户说“实现这个方案/落地开发/implement this plan”时触发。"
---

# arc:build — implementation of the solution

## Overview

`arc:build` is responsible for transferring requirements and solutions to the code implementation layer, first freezing the interface contract and compatibility strategy, and then outputting deliverable engineering changes and execution reports.

This skill emphasizes "realizable, verifiable, and traceable":

- Produce the minimum executable plan before implementation
- Document key decisions and risks during implementation
- Produce verification evidence and handover summary after implementation

## Quick Contract

- **Trigger**: The requirements/solutions have been clarified and the plan needs to be implemented into submittable code changes.
- **Inputs**: `project_path`, `task_name`, achievement goal, change scope and verification level.
- **Outputs**: Implementation plan, code changes, verification records and handoff summary.
- **Quality Gate**: The plan advance and evidence verification check of `## Quality Gates` must be passed before handover.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Border Tip** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:build` to solidify the implementation plan and verification path first, and then implement the code."

## Teaming Requirement

- Each implementation must first "draw a team together" and clearly define at least three roles and responsibilities of `Owner`, `Executor`, and `Reviewer`.
- If the operating environment only has a single Agent, the three-role perspective must be explicitly output during delivery to form a "decision-execution-review" closed loop before submitting the conclusion.

## The Iron Law

```
NO CODE CHANGE WITHOUT PLAN, EVIDENCE, AND ROLLBACK
```

Code changes must not be implemented without clear plans, verification evidence, and rollback strategies.

## Workflow

1. Produce upstream requirements/plan products and create implementation briefs.
2. Generate a minimum implementation plan and split it into reviewable change units.
3. Implement in stages and simultaneously document key decisions and risks.
4. Perform verification, output handover materials, and mark impact surfaces.

## Quality Gates

- `implementation-plan.md` must be generated first and then encoded.
- Every critical change must have verification commands and results.
- The handover summary must cover impact modules and regression concerns.
- Failure scenarios must be reversible and irreversible operations are not allowed.

## Expert Standards

- Adopt `DoD` delivery baseline: Delivery can only be made after the build passes, key tests pass, documents are updated, and the rollback path is complete.
- Execute `SemVer + Contract Test` on interfaces and data contracts, prohibiting undeclared destructive changes.
- Changes to critical links must provide `RTO/RPO` impact assessment and rollback drill records.
- Delivery packages must contain `SBOM` with build provenance information (aligning supply chain traceability practices).
- Handoff must quantify the risk of change: impact area, regression scope, launch window, and observation indicators.

## Scripts & Commands

- Work area scaffolding: `python3 arc:build/scripts/scaffold_implement_case.py --project-path <project_path> --task-name <task_name>`
- Delivery report rendering: `python3 arc:build/scripts/render_implementation_report.py --case-dir <project_path>/.arc/arc:build/<task_name> --task-name <task_name> --result pass`
- Runtime main command: `arc build`

## Red Flags

- Skip planning and go directly to large-scale code changes.
- Only a "completed" conclusion is given, without verification evidence.
- The change scope is out of control and irrelevant directories are touched.
- There is no fallback plan but the critical path is changed.

## Mandatory Linkage (cannot be fought alone)

1. Preferably read the `arc:decide` product (if present) as implementation input.
2. When the requirements are vague, go to `arc:clarify` first, and do not modify it blindly directly.
3. After the implementation is completed, it is recommended to hand it over to `arc:audit` or `arc:e2e` for verification closed loop.
4. Output standardized handover documents for downstream skill consumption.

## When to Use

- **Preferred trigger**: The requirements or solutions have been clarified, and code changes can be submitted if output is required.
- **Typical scenarios**: Function development, interface evolution, database migration, behavior-invariant reconstruction, document synchronization, defect repair with verification evidence.
- **Boundary Tip**: `arc:clarify/arc:decide` is used for fuzzy requirements, and `arc:audit` is used for comprehensive evaluation.

## Input Arguments

| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | Absolute path to the target project root directory |
| `task_name` | string | yes | This implementation task name is used for directory naming. |
| `implementation_goal` | string | no | Achievement Goal Summary |
| `change_scope` | string | no | Change scope limit (module/directory) |
| `verification_level` | enum | no | `basic` / `standard` / `strict`, default `standard` |
| `output_dir` | string | no | Default `<project_path>/.arc/arc:build/<task-name>/` |

## Dependencies

- **Orchestration Contract** (recommended): Follow `docs/orchestration-contract.md` and distribute implementation and verification tasks through the runtime adaptation layer.
- **ace-tool MCP** (required): Locate the implementation entrance, influence surface, and related symbols.
- **Scheduling API** (required): schedule implementation and verification tasks.
- **arc:decide** (recommended): consume upstream solution documents.
- **arc:clarify** (optional): Supplement the context when the requirements are unclear.
- **Exa MCP** (optional): Check official documentation and implementation reference.

## Context Priority (mandatory)

0. `.arc/context-hub/index.json` (shared context index, reuse first)
1. `.arc/arc:build/<task>/context/implementation-brief.md`(24h)
2. `.arc/arc:decide/<task>/` The product of the next solution (if it exists)
3. `codemap.md` (arc:cartography) and `CLAUDE.md` hierarchical index (7 days)
4. score product (generated by `score/` module) / `arc:audit` / last `arc:build` handoff (if index available)
5. `ace-tool` Semantic scanning
6. `Exa` external documentation

Failure reflow rules:
- CLAUDE index invalid: trigger `arc:init --mode update`
- codemap invalid: trigger `arc:cartography` update
- score/review product failure: trigger `score` module refresh (triggered by `arc:gate` arrangement) / `arc:audit` update

## Critical Rules

1. **Plan first, then implement**: `plan/implementation-plan.md` must be produced first.
2. **Small step submission**: Prioritize splitting into small changes that can be reviewed, and do not make unbounded major changes.
3. **Evidence verification**: There must be verification records (commands/results/conclusions) after implementation.
4. **Impact Transparency**: Affected modules and regression concerns must be listed.
5. **Failure to rollback**: Retain the rollback idea and do not allow "non-recoverable" changes.
6. **No execution beyond authority**: Do not touch directories and files unrelated to the task.

## Instructions

### Phase 1: Contextual archiving

1. Read the upstream product and project index.
2. Identify implementation entries, affected modules, and constraints.
3. Generate `context/implementation-brief.md`.
4. Execute the scaffolding command:

```bash
python arc:build/scripts/scaffold_implement_case.py \
  --project-path <project_path> \
  --task-name <task_name>
```

### Phase 2: Implementing the plan

1. Output `plan/implementation-plan.md`:
- Goals and Scope
- Change steps
- Risks and Fallback Plans
- Verification plan
2. If there are major technical differences, return to the `arc:decide` argument first.

### Phase 3: Coding implementation

1. Implemented step by step as planned.
2. Each step is logged to `execution/execution-log.md`:
- Modification point
- reason
- risk
- Verification status

### Phase 4: Verification and Handover

1. Perform minimum necessary verification (compile/test/static checks).
2. Generate delivery documents:
- `reports/execution-report.md`
- `handoff/change-summary.md`
3. Use a script to render the final report:

```bash
python arc:build/scripts/render_implementation_report.py \
  --case-dir <output_dir> \
  --task-name <task_name> \
  --result pass
```

## Scripts

```bash
python arc:build/scripts/scaffold_implement_case.py --project-path <project_path> --task-name <task_name>
python arc:build/scripts/render_implementation_report.py --case-dir <output_dir> --task-name <task_name> --result pass
```

## Artifacts

Default directory: `<project_path>/.arc/arc:build/<task-name>/`

- `context/implementation-brief.md`
- `plan/implementation-plan.md`
- `execution/execution-log.md`
- `reports/execution-report.md`
- `handoff/change-summary.md`

## Quick Reference

| output | use |
|------|------|
| `implementation-plan.md` | Implementation steps and risk control |
| `execution-log.md` | Execution process tracking |
| `execution-report.md` | Implementation results and verification conclusions |
| `change-summary.md` | Handover summary for downstream skills/reviews |

## Anti-Patterns

**CRITICAL: The following behaviors are FORBIDDEN in arc:build execution:**

### Planning Anti-Patterns

- **Speculation Implementation**: Writing code without reading arc:decide or arc:clarify output first — must have approved plan
- **Scope Creep**: Adding features not in the approved plan — strict scope adherence required
- **Pattern Ignorance**: Not reading existing code patterns before implementing — causes inconsistency

### Implementation Anti-Patterns

- **Type Safety Violation**: Using `as any`, `@ts-ignore`, `@ts-expect-error` — forbidden without explicit exception approval
- **Empty Catch Blocks**: `catch(e) {}` — must handle or re-throw errors
- **Magic Values**: Hardcoding configuration values — use constants/config files
- **Commented Code**: Leaving commented-out code blocks — delete or implement

### Process Anti-Patterns

- **Skip Verification**: Not running LSP diagnostics after edits — must verify clean before commit
- **Batch Completion**: Marking multiple todos complete at once — one at a time, verify each
- **Cache Stale Usage**: Using expired codemap.md (7d+) without refresh — triggers arc:cartography
- **Handoff Skip**: Not generating `handoff/` artifacts — breaks downstream skills
