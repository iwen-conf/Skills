---
name: "arc:decide"
description: "多视角方案决策与收敛：通过 architecture/deep/ui 辩论形成可执行计划；当用户说“技术选型/方案对比/架构决策/architecture trade-off”时触发。"
---

# arc:decide — multi-agent deliberation

## Overview

`arc:decide` uses a shared file system to organize multi-role debates (`architecture`, `deep`, `ui`) and turn ambiguity into a reviewable decision and executable plan.
The core goal is not to "give answers quickly", but to output traceable, reviewable, and executable decision-making basis before implementation.

## Quick Contract

- **Trigger**: Tasks involving technology selection, architectural disagreement, high-risk transformation, or multi-objective trade-offs.
- **Inputs**: `task_name`, `workdir`; optional `enhanced_prompt_path`, `max_rounds`, `max_ambiguity_rounds`.
- **Outputs**: ambiguity list, each role's solution/refutation, convergence report, executable plan (OpenSpec).
- **Quality Gate**: Pass the "Evidence, Refutation, Convergence, Plan Completeness" check of `## Quality Gates`.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Note** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:  
"I am using `arc:decide` to run multi-perspective deliberation and convergence before implementation."

## Teaming Requirement

- Every execution must first "draw a team together" and at least clarify the three roles and responsibilities of `Owner`, `Executor` and `Reviewer`.
- If the operating environment only has a single Agent, the three-role perspective must be explicitly output during delivery to form a "decision-execution-review" closed loop before submitting the conclusion.

## The Iron Law

```
NO CONSENSUS CLAIM WITHOUT CROSS-CRITIQUE EVIDENCE
```

"Consensus has been reached" must not be declared without cross-role rebuttal evidence.

## Workflow

1. Do an ambiguity check first, and ask the user to add boundaries if necessary.
2. Multiple roles propose solutions concurrently and conduct cross-refutations.
3. Form a convergence conclusion and solidify it into an executable plan (OpenSpec).
4. Only hand off to `arc:build` or delegated execution after the dispute has been resolved and execution is explicitly requested.

## Quality Gates

- Each round must have three types of evidence: "proposal + refutation + convergence judgment".
- The final consensus must give reasons for reservation of disagreements and decisions.
- The plan must include executable tasks, risk lists, and fallback strategies.
- References to external information need to indicate the source and timeliness.

## Expert Standards

- Decision products must conform to the `ADR` structure (Context / Decision / Consequences / Alternatives).
- For major multi-objective trade-offs, plan comparison should use `weighted scoring + sensitivity analysis` to avoid single-point preference.
- High-risk decisions must implement `Pre-Mortem` and clarify the failure triggering conditions.
- Key selections must define verifiable `Fitness Function` (performance, cost, stability thresholds).
- The conclusion needs to include failure conditions and undoable paths.

## Scripts & Commands

- Runtime main command: `arc decide`
- Estimate mode: `arc decide --mode estimate`
- Generate BDD seed: `python3 Arc/arc:decide/scripts/generate_bdd_seed.py --consensus-report <report.md> --output <bdd-seed.yaml>`
- Scheduling long example: `Arc/arc:decide/references/schedule-task-playbook.md`

## Red Flags

- Announced the "final plan" without any rebuttal.
- Just rely on preference without giving evidence chain and weighing basis.
-Encoding without clearing ambiguity.
- Only a single perspective is retained and trade-offs are not explained.

## MCP tool usage

### Search external information (Internet)

Use **Exa MCP** to fetch standards, official docs, and recent engineering practices.

### Search project information

Use **ace-tool MCP** for semantic retrieval of code structure, dependencies, and existing patterns.

## When to Use

- **Preferred Trigger**: User requirements for "technology selection, architectural solution comparison, and multiple solution choices."
- **Typical Scenarios**: Architecture upgrades, cross-team transformations, high-risk performance/security decisions.
- **Boundary Tip**: If the requirement itself is not clear, use `arc:clarify` first. The default output of `arc:decide` is a decision and plan; use `arc:build` when code implementation is needed.

## Core Pattern

### Input Arguments

| parameter | type | required | description |
|-----------|------|----------|-------------|
| `task_name` | string | yes | Task name (used for working directory naming) |
| `workdir` | string | yes | Absolute path to the working directory |
| `enhanced_prompt_path` | string | no | Default `.arc/decide/<task-name>/context/enhanced-prompt.md` |
| `max_rounds` | number | no | Maximum rounds of plan debate, default 3 |
| `max_ambiguity_rounds` | number | no | Maximum rounds for ambiguity checking, default 3 |

### Directory Structure

```text
<workdir>/.arc/decide/<task-name>/
├── context/
│   └── enhanced-prompt.md
├── agents/
│   ├── architecture/
│   ├── deep/
│   └── ui/
├── convergence/
│   ├── round-N-summary.md
│   └── final-consensus.md
├── openspec/
│   └── changes/<task-name>/
│       ├── proposal.md
│       ├── specs/<capability>/spec.md
│       ├── design.md
│       └── tasks.md
└── execution/
    └── implementation-log.md
```

## Dependencies

- **Organization Contract**: Required. Follow `docs/orchestration-contract.md`.
- **ace-tool MCP**: Required. Responsible for repository semantic retrieval and evidence location.
- **Exa MCP**: Recommended. Supplementary standards, specifications and external implementation basis.
- **OpenSpec**: Required in planning phase (`proposal → specs → design → tasks`).
- **arc:build**: Optional in execution phase when implementation is delegated.

## Agent Orchestration Profiles

| role | capability_profile | capabilities | output baseline |
|------|--------------------|--------------|-----------------|
| architecture | `architecture` | `["arc:decide"]` | Architectural trade-offs and global consistency |
| deep | `deep` | `["arc:decide"]` | Engineering feasibility, performance and safety |
| ui | `ui` | `["arc:decide", "frontend-ui-ux"]` | Experience, interaction and maintainability |

For detailed `schedule_task(...)` examples, see `Arc/arc:decide/references/schedule-task-playbook.md`.

## Instructions

### Phase 1: Ambiguity Check

1. Use ace-tool MCP and Exa MCP to complete the evidence first.
2. Three roles concurrently output ambiguity lists (boundaries, terms, constraints, dependencies).
3. After cross-rebuttal, summarize it into `convergence/round-N-summary.md`.
4. Clarify key ambiguities to users until the debate stage can be entered.

### Phase 2: Deliberation

1. Three characters propose independent plans concurrently (same round).
2. Each character must review and refute the other two characters' scenarios.
3. The main process summarizes the dispute points and determines whether there is convergence.
4. Generate `convergence/final-consensus.md` after convergence.

### Phase 3: Plan Generation (OpenSpec)

1. Initialize the OpenSpec change directory.
2. Generate `proposal → specs → design → tasks` in order.
3. The three roles concurrently review the plan document and propose modifications.
4. The main process incorporates review opinions to complete the final task sequence.
5. Run `openspec validate --change <task-name>` to ensure the structure is complete.

### Phase 4: Optional Execution Handoff

1. If implementation is explicitly requested, hand off `tasks.md` to `arc:build` or a delegated execution role.
2. If `arc:decide` also coordinates that execution, log the handoff or implementation evidence to `execution/implementation-log.md`.
3. Archive the change only after the decision product has been executed and verified (`openspec archive <task-name>`).

## Timeouts and downgrades

| condition | handling |
|-----------|----------|
| Role task timeout (>10min) | Try again; if still fails, downgrade to single role. Supplementary argument |
| Reached `max_ambiguity_rounds` with still ambiguity | Mark open questions and ask user for decision |
| Reaching `max_rounds` and still not converging | Produce a "preserve divergence" version and give recommended paths |
| OpenSpec command failed | Downgrade to manual generation of `tasks.md` and mark risk |

## status feedback

```text
[arc:decide] <task-name>
Phase 1: ambiguity check      [running/completed]
Phase 2: deliberation         [running/completed]
Phase 3: openspec planning    [running/completed]
Phase 4: execution            [running/completed]
```

## Quick Reference

| stage | key outputs |
|-------|-------------|
| Ambiguity checking | `agents/*/ambiguity-round-N.md` |
| Proposal debate | `agents/*/proposal-round-N.md`, `agents/*/critique-round-N.md` |
| Convergence | `convergence/final-consensus.md` |
| Plan generation | `openspec/changes/<task-name>/{proposal,design,tasks}.md` |
| Execution record | `execution/implementation-log.md` |

## Anti-Patterns

- **Premature Consensus**: Convergence is declared before cross-rebuttal is completed.
- **Echo Chamber**: Character output is highly homogeneous, with no real conflicts or trade-offs.
- **Spec Skip**: Enter implementation directly without OpenSpec verification.
- **Risk Blindness**: Ignore failed paths, fallback conditions and observation indicators.
- **No Decision Trail**: There is no ADR evidence chain, and subsequent review is impossible.

## Quick call method

| role | call pattern | mode |
|------|--------------|------|
| architecture | `schedule_task(capability_profile="architecture", capabilities=["arc:decide"], ...)` | background |
| deep | `schedule_task(capability_profile="deep", capabilities=["arc:decide"], ...)` | background |
| ui | `schedule_task(capability_profile="ui", capabilities=["arc:decide", "frontend-ui-ux"], ...)` | background |
| aggregation/finalization | processed directly by the main process | foreground |
