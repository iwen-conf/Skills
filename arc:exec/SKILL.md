---
name: "arc:exec"
description: "统一任务编排入口：理解需求、路由到 arc:* 或多 Agent 协同执行；当用户说“帮我拉团队/不知道用哪个技能/split task/orchestrate this work”时触发。"
---

# arc:exec — unified orchestration entry

## Overview

`arc:exec` is a scheduling Meta-Skill that does not directly produce business solutions, but is responsible for:

1. Identify user goals and constraints;
2. Route between `arc:*` skills and general multi-Agent execution;
3. Organize concurrent scheduling, result collection, and conflict arbitration;
4. Output a trackable executive summary and next step recommendations.

## Quick Contract

- **Trigger**: The requirements are vague, span multiple skills, or the user only gives the target without specifying the execution path.
- **Inputs**: `task_description`, `workdir`; optional `task_name`, `preferred_skill`, `dry_run`, `confirm`, `snapshot`.
- **Outputs**: routing decisions, scheduling records, execution preview, aggregation conclusions and follow-up actions.
- **Quality Gate**: Passes the "Explainable Routing + Traceable Scheduling + Convergent Aggregation" check of `## Quality Gates`.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Note** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:  
"I am using `arc:exec` to complete requirement understanding, skill routing, and orchestration planning first."

## Teaming Requirement

- Every execution must first "draw a team together" and at least clarify the three roles and responsibilities of `Owner`, `Executor` and `Reviewer`.
- If the operating environment only has a single Agent, the three-role perspective must be explicitly output during delivery to form a "decision-execution-review" closed loop before submitting the conclusion.

## The Iron Law

```
ROUTE BEFORE EXECUTE, COLLECT BEFORE CONCLUDE
```

No scheduling is allowed before routing and capability verification is completed; no final conclusion is given before all concurrent results are collected.

## Workflow

1. Scan the context and understand the requirements (goals, scope, risks, dependencies).
2. Decision execution path: single skill, skill link, or general multi-agent concurrency.
3. Generate execution preview (optional snapshot, confirmation, dry-run).
4. Schedule and collect results according to waves, and perform conflict arbitration and downgrade processing.
5. Output aggregate summary, risk and next action.

## Quality Gates

- The routing conclusion must be explainable (why this Skill/profile was selected).
- Each schedule must contain `capability_profile`, `description`, `prompt`.
- Concurrent tasks must have collection points (`collect_task(...)`) and failure handling strategies.
- Aggregation conclusions must contain responsibility, risk, and tracking identifiers (`task_ref` or equivalent fields).

## Expert Standards

- The orchestration output must give `RACI`: clear decision-making, execution, review and notification roles.
- The scheduling plan must be marked with `critical path (CPM)` and concurrent waves to avoid false concurrency.
- The aggregation phase must follow `conflict arbitration rules` (security > correctness > consistency).
- The result summary needs to give the confidence level, degradation conditions and rollback path.
- Manual confirmation gates need to be clearly defined when high-risk changes are involved.

## Scripts & Commands

- Runtime main command: `arc exec`
- Initialize the orchestration workspace: `python3 arc:exec/scripts/scaffold_exec_case.py --workdir <workdir> --task-name <task-name>`
- Rendering scheduling and summary products: `python3 arc:exec/scripts/render_dispatch_report.py --case-dir <case_dir> --task-name <task-name> --dispatch "deep|[arc:build]|implement backend interface|done|execution/backend.md"`
- Scheduling example manual: `arc:exec/references/schedule-task-playbook.md`

## Red Flags

- Enter encoding directly without routing.
- There are no collection points and no failed branches for concurrent tasks.
- Ignore `context-hub` cache causing duplicate scans.
- Multi-Agent conflicts are directly spliced ​​and output without arbitration.
- Still perform real changes when `dry_run=true`.

## When to Use

- **Preferred Trigger**: User expresses "Help me arrange/pull the team to execute", or is not sure which `arc:*` should be called.
- **Typical scenario**: Requirements span multiple stages (clarification→decision→implementation→verification), writing/review chains such as `arc:clarify → arc:aigc → arc:audit`, or full-stack concurrent task decomposition.
- **Boundary Tip**: If the target skill has been identified and the boundary is clear, call the target skill directly without going through `arc:exec`.

## Input Arguments

| parameter | type | required | description |
|-----------|------|----------|-------------|
| `task_description` | string | yes | User task description |
| `task_name` | string | no | Task name used for orchestration workspace naming |
| `workdir` | string | yes | Absolute path to the working directory |
| `preferred_skill` | string | no | User-specified skill (skip routing if hit) |
| `dry_run` | boolean | no | Only output execution preview, not actual execution |
| `confirm` | boolean | no | Require secondary confirmation before high-impact operations |
| `snapshot` | boolean | no | Generate a rollback snapshot before execution |

## Dependencies

- **Organization Contract**: Required. Follow `docs/orchestration-contract.md`.
- **ace-tool (MCP)**: Required. Used for repository semantic retrieval and influence surface positioning.
- **Exa MCP**: Optional. Used for external data supplement and standard verification.
- **All arc: skills**: Routing to the corresponding skill must follow its `SKILL.md`.

## Scheduling Playbook

The complete `schedule_task(...)` example of `arc:exec`, parameter combinations, and concurrent collection templates have been extracted to:

- `arc:exec/references/schedule-task-playbook.md`

SKILL The main text only retains the decision-making process to avoid being too long and repetitive.

## Instructions

### Phase 1: Understand

1. Read `.arc/context-hub/index.json` (if it exists) and reuse the generated products first.
2. Use ace-tool MCP to obtain the code structure, key modules and suspicious impact areas.
3. Use Exa MCP to supplement external constraints (standards, best practices, official documentation) when necessary.
4. Form a task portrait: type, technical domain, complexity, risk level, and whether it can be concurrent.

### Phase 2: Route

1. If `preferred_skill` is available, route the skill directly.
2. Otherwise, press `docs/arc-routing-matrix.md` for skill matching.
3. Writing tasks that are primarily about academic/professional prose polishing, citation-preserving rewrite, or multi-pass draft cleanup should route to `arc:aigc`.
4. When there is no hit skill, it will switch to "universal multi-agent scheduling" mode (Phase 3).
5. Write the routing record: `routing/dispatch-log.md` (reason, evidence, confidence, downgrade path).

### Phase 2.5: Preview & Safety

1. Output execution preview: planned actions, impact areas, and risk levels.
2. If `snapshot=true`, create a snapshot and record the rollback command.
3. If `confirm=true` or a high-impact operation is detected, obtain user confirmation first.
4. If `dry_run=true`, exit after this stage and return the preview results.

### Phase 3: Dispatch

1. Split the task into concurrent subtasks and dispatch them in waves.
2. Each subtask must be explicitly specified:
   - `capability_profile`
   - `capabilities` (if required)
   - Read and write paths and expected products
3. Collect `task_ref` for tasks of the same wave.
4. Use `collect_task(...)` to collect them all before entering the next wave or aggregation stage.

### Phase 4: Aggregate

1. Summarize the products of each subtask and detect conflicts.
2. When conflicts occur, they will be handled according to the arbitration rules; if necessary, a `review` schedule will be added.
3. Output `aggregation/final-summary.md`, including:
   - Actions performed;
   - Risks and residual issues;
   - Suggested next steps and owners.

## Outputs

```text
<workdir>/.arc/exec/<task-name>/
├── manifest.json
├── context/
│   └── task-brief.md
├── routing/
│   └── dispatch-log.md
├── preview/
│   └── execution-preview.md
├── dispatch/
│   └── task-board.md
├── aggregation/
│   └── final-summary.md
├── snapshots/
└── rollback/
    └── restore-notes.md
```

## Timeouts and downgrades

| condition | handling |
|-----------|----------|
| Single task timeout (>10min) | Mark timeout, report the blocker, and either wait for user direction or re-scope into a smaller subtask |
| Partial failure of concurrent tasks | Mark `partial` and trigger compensation scheduling |
| Uncertain routing | Downgrade to `arc:clarify` and reroute after obtaining constraints |
| Context invalidation/cache expiration | Trigger refresh first, then enter scheduling |

## Quick Reference

| stage | key actions | artifacts |
|-------|-------------|-----------|
| Understand | context preflight + MCP scan | `context/task-brief.md` |
| Route | skill match + confidence + fallback | `routing/dispatch-log.md` |
| Preview | operation list + snapshot + confirm | `preview/execution-preview.md` |
| Dispatch | split + schedule + collect | `dispatch/task-board.md` |
| Aggregate | conflict arbitration + summary | `aggregation/final-summary.md` |

## Anti-Patterns

- **Shotgun Dispatch**: Starting multiple Agents without boundaries leads to duplication of work.
- **Blocking Search**: Execute `search/research` serially in the foreground, slowing down the main link.
- **Orphaned Tasks**: Not tracking `task_ref` results in unrecyclable results.
- **Cache Blindness**: Repeating the same analysis even though there are reusable products.
- **Unsafe Execution**: No confirmation, no snapshot, and no rollback for high-risk operations.

## Context

The context is read with the following priorities before dispatching:

1. `.arc/context-hub/index.json` (shared index product)
2. `.arc/<skill>/` (skill cache product)
3. Project-level `CLAUDE.md` (project rules/architecture description)
4. ace-tool MCP scan results (latest code reality)
5. Exa MCP (external knowledge supplement)

See `docs/orchestration-contract.md` and `docs/arc-routing-matrix.md` for details.
