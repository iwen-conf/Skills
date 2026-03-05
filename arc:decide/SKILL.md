---
name: "arc:decide"
description: "方案有争议或风险高时使用：让多 Agent 独立提案并互相反驳，最终收敛到可执行决策。"
---

# Multi-Agent Deliberation

## Overview

By using the shared file system as a communication bus, multiple professional Agents (oracle/deep/visual-engineering) are coordinated for iterative collaborative deliberation. **In each stage, each Agent must refute each other and review each other's opinions**.

The process is divided into four stages:

1. **Ambiguity Checking Phase**: Multi-Agent analysis requirements → Identify ambiguities → Refute each other → User clarification → Until there is no ambiguity
2. **Deliberation stage**: Multi-Agent independent proposals → Cross-review → Mutual refutation → Iterative convergence → Synthetic consensus report
3. **Plan generation phase**: OpenSpec generates structured plan → Multi-Agent review and rebuttal → Finalize executable plan
4. **Execution phase**: Use schedule_task(workstream="deep") to execute code implementation

## Quick Contract

- **Trigger**: Technical decisions are high-risk, have obvious differences, or require multiple perspectives to be demonstrated before execution.
- **Inputs**: `task_name`, `workdir`, enhanced prompt, maximum rounds and ambiguous round limits.
- **Outputs**: ambiguity list, each Agent's proposal/rebuttal, convergence summary and executable plan.
- **Quality Gate**: The consensus and plan integrity check of `## Quality Gates` must be passed before entering implementation.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:decide` to do multi-perspective deliberation and convergence of differences before entering execution."

## The Iron Law

```
NO CONSENSUS CLAIM WITHOUT CROSS-CRITIQUE EVIDENCE
```

There should be no claim that "consensus has been reached" without cross-rebuttal evidence.

## Workflow

1. Do an ambiguity check first and go back to the user to clarify the issue if necessary.
2. Multiple agents independently propose and refute each other to form convergent evidence.
3. Generate structured plans and conduct plan-level review.
4. Only enter the execution phase after the dispute has subsided.

## Quality Gates

- Records of proposals and rebuttals must be kept for each round of review.
- Consensus conclusions must indicate reservations of disagreement and reasons for decision-making.
- There must be an executable plan and risk list before entering execution.
- Citations of external information must indicate the source and timeliness.

## Red Flags

- The "final solution" is synthesized directly without any refutation process.
- Treat preference opinions as evidence conclusions.
- Encoding execution begins without ambiguity being cleared.
- Only a single-agent perspective is adopted without explanation.

## Agent calling method

**CRITICAL**: All tasks are scheduled through the unified `schedule_task()` API:

| Agent role | Calling method | use |
|------|---------|------|
| **oracle** | `schedule_task(specialist="oracle", capabilities=["arc:decide"], ...)` | Architecture analysis, design review (read-only high-quality reasoning) |
| **deep** | `schedule_task(workstream="deep", capabilities=[...], ...)` | In-depth engineering analysis, solution proposal, code execution |
| **visual-engineering** | `schedule_task(workstream="visual-engineering", capabilities=["arc:decide", "frontend-ui-ux"], ...)` | Front-end and DX perspective, UI/UX design, interactive experience |
| **metis** | `schedule_task(specialist="metis", capabilities=["arc:clarify"], ...)` | Requirements pre-analysis, ambiguity detection |
| **explore** | `schedule_task(specialist="explore", capabilities=[], run_mode="background", ...)` | Code base search (cheap, backend) |
| **librarian** | `schedule_task(specialist="librarian", capabilities=[], run_mode="background", ...)` | External document search (cheap, backend) |

Generic `schedule_task` calling template:
```
schedule_task(
workstream: "<lane>", // or specialist: "<agent>"
capabilities: ["arc:decide", ...], // Equipment related skills
description: "<short description>",
prompt: "<Specific task instructions, including read and write file paths>",
run_mode: "background" // Concurrent execution
)
```

## MCP tool usage

### Search external information (Internet)
Use **Exa MCP** for web searches:
- Search the latest technical documentation and best practices
- Search use cases for open source projects and libraries
- Search industry standards and safety regulations

### Search project information
Use **ace-tool MCP** for semantic search:
- Search project code structure
- Search for existing implementation patterns
- Search project specifications (CLAUDE.md)

## When to Use

- **首选触发**: High-risk technology decisions require debate from multiple perspectives and convergence of consensus.
- **典型场景**: There is an obvious trade-off in architecture/performance/security/compatibility.
- **边界提示**: If the demand is not clear, go to `arc:clarify`; if the route is set, go to `arc:build`.

## Core Pattern

### input parameters

| parameter | type | Required | illustrate |
|------|------|------|------|
| `task_name` | string | yes | Task name, used for directory naming |
| `workdir` | string | yes | Working directory absolute path |
| `enhanced_prompt_path` | string | no | Enhance prompt path, read `.arc/deliberate/<task-name>/context/enhanced-prompt.md` by default |
| `max_rounds` | number | no | Maximum deliberation iteration rounds, default 3 |
| `max_ambiguity_rounds` | number | no | Maximum ambiguity checking rounds, default 3 |

### Directory structure

**Divided into directories by Agent role**, the output of all stages of each Agent is concentrated in its own directory:

```
<workdir>/.arc/deliberate/<task-name>/
├── context/
│ └── enhanced-prompt.md # arc:clarify output
├── agents/
│ ├── oracle/ # oracle Agent output in all stages (architectural perspective)
│ │ ├── ambiguity-round-1.md # Ambiguity analysis (Phase 1)
│ │ ├── proposal-round-1.md # Proposal (Phase 2)
│ │ ├── critique-round-1.md # Review rebuttal (Phase 2)
│ │ └── plan-review.md # Plan review (Phase 3)
│ ├── deep/ # deep Agent output in all stages (engineering perspective)
│   │   ├── ambiguity-round-1.md
│   │   ├── proposal-round-1.md
│   │   ├── critique-round-1.md
│   │   └── plan-review.md
│ └── visual-engineering/ # visual-engineering All stages of output (front-end and DX perspective)
│       ├── ambiguity-round-1.md
│       ├── proposal-round-1.md
│       ├── critique-round-1.md
│       └── plan-review.md
├── convergence/
│ └── round-N-summary.md # Summary of convergence determination
└── openspec/ # OpenSpec workspace (Phase 3)
    ├── changes/
│ ├── <task-name>/ # openspec new change create
    │   │   ├── .openspec.yaml
│ │ ├── proposal.md #Proposal proposal
│ │ ├── specs/ # Detailed specifications
    │   │   │   └── <capability>/spec.md
│ │ ├── design.md # Architectural Design
│ │ └── tasks.md # Orderly executable tasks
    │   └── archive/
    └── specs/
```

---

## Phase 1: Ambiguity Check

### Step 1.1: Multi-Agent analysis + search

**CRITICAL**: Information must be searched using MCP tools before analysis.

1. **Search project information**: Search project code using `ace-tool` MCP's `search_context`
2. **Search external information**: Search the Internet using `Exa MCP`'s `web_search_exa` or `company_research_exa`
3. **Analysis**: Call multiple Agents concurrently, analyze the enhanced prompt, and identify potential ambiguities

**Multiple Agent concurrent calls** (initiated in the same message, `run_mode: "background"`):

**oracle analysis** (architectural perspective):
```
schedule_task(
  specialist: "oracle",
  capabilities: ["arc:decide"],
  run_mode: "background",
description: "oracle ambiguity analysis",
prompt: "You are an architect. Analyze the ambiguities of the following requirements.
Contextual information: <MCP search results>
Read <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md.
List all possible ambiguities, including: undefined boundary conditions, unclear constraints, possible different understandings of terminology, unstated assumptions, etc.
Write the analysis results to <workdir>/.arc/deliberate/<task-name>/agents/oracle/ambiguity-round-N.md. "
)
```

**deep analysis** (engineering perspective):
```
schedule_task(
  workstream: "deep",
  capabilities: ["arc:decide"],
  run_mode: "background",
description: "deep ambiguity analysis",
prompt: "You are a back-end architect. Analyze the ambiguities of the following requirements.
Contextual information (from MCP search): <MCP search results>
Read <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md.
List possible ambiguities from the perspectives of back-end architecture, technical constraints, performance requirements, etc.
Write to <workdir>/.arc/deliberate/<task-name>/agents/deep/ambiguity-round-N.md. "
)
```

**visual-engineering analysis** (front-end and DX perspective):
```
schedule_task(
  workstream: "visual-engineering",
  capabilities: ["arc:decide", "frontend-ui-ux"],
  run_mode: "background",
description: "visual-engineering ambiguity analysis",
prompt: "You are a front-end and DX engineer. Analyze the ambiguities of the following requirements.
Contextual information (from MCP search): <MCP search results>
Read <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md.
List possible ambiguities from the perspectives of user experience, completeness, maintainability, etc.
Write to <workdir>/.arc/deliberate/<task-name>/agents/visual-engineering/ambiguity-round-N.md. "
)
```

### Step 1.2: Mutual refutation of ambiguity analysis

**CRITICAL**: Each Agent must **refute each other's ambiguities** identified by the other party.

Each model must:
1. Read ambiguity analysis of other Agents (such as oracle reads `agents/deep/ambiguity-round-N.md` and `agents/visual-engineering/ambiguity-round-N.md`)
2. Refute the "ambiguity" perceived by the other party (it may not be an ambiguity)
3. Fill in the ambiguities missed by the other party
4. Append the rebuttal content to your own `ambiguity-round-N.md`

The calling method is the same as Step 1.1 (oracle uses `schedule_task(specialist="oracle")`, deep uses `schedule_task(workstream="deep")`, visual-engineering uses `schedule_task(workstream="visual-engineering")`).

### Step 1.3: Aggregation ambiguity

Read each analysis report, and the main process processes it directly (no additional role scheduling is required) to aggregate all ambiguity points:

```markdown
# Ambiguity Summary (Round N)

## technical ambiguity
- <ambiguity 1 description>
- <ambiguity 2 description>

## Boundary condition ambiguity
- <ambiguous description>

## Assumption ambiguity
- <ambiguous description>

## Questions that need clarification
1. <Question 1>
2. <Question 2>
```

### Step 1.4: User clarification

Use `AskUserQuestion` to present a list of ambiguities to the user and ask them to clarify them one by one:

```markdown
## ambiguity clarification

Before proceeding, please clarify the following:

### Question 1
<ambiguous description>
- [Option A]
- [Option B]
- [User supplement]

### Question 2
...
```

### Step 1.5: Convergence determination

- **Unclarified ambiguity**: Return to Step 1.1 and enter the next round of ambiguity checking
- **No ambiguity or all clarifications**: Entering Phase 2 (deliberation stage)

---

## Phase 2: Deliberation

### Step 2.1: Directory scaffolding

Make sure the `agents/oracle/`, `agents/deep/`, `agents/visual-engineering/`, `convergence/` directories are created.

### Step 2.2: Distribute proposals concurrently (each round)

**CRITICAL**: Each Agent must initiate concurrently in the same message (`run_mode: "background"`).

**oracle proposal** (architectural perspective):
```
schedule_task(
  specialist: "oracle",
  capabilities: ["arc:decide"],
  run_mode: "background",
description: "oracle proposal Round N",
prompt: "You are the architect (overall perspective, architectural design, technology selection).
Read <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md.
Give a complete solution proposal, in plain text Markdown format only, code blocks are prohibited.
Write the proposal to <workdir>/.arc/deliberate/<task-name>/agents/oracle/proposal-round-N.md. "
)
```

**deep proposal** (engineering perspective):
```
schedule_task(
  workstream: "deep",
  capabilities: ["arc:decide"],
  run_mode: "background",
description: "deep proposal Round N",
prompt: "You are a backend architect (backend architecture, performance optimization, database, security).
Read <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md.
Give a complete solution proposal, in plain text Markdown format only, code blocks are prohibited.
Write to <workdir>/.arc/deliberate/<task-name>/agents/deep/proposal-round-N.md. "
)
```

**visual-engineering proposal** (front-end and DX perspective):
```
schedule_task(
  workstream: "visual-engineering",
  capabilities: ["arc:decide", "frontend-ui-ux"],
  run_mode: "background",
description: "visual-engineering proposal Round N",
prompt: "You are a front-end and DX engineer (UI/UX, user experience, responsive design, maintainability).
Read <workdir>/.arc/deliberate/<task-name>/context/enhanced-prompt.md.
Give a complete solution proposal, in plain text Markdown format only, code blocks are prohibited.
Write to <workdir>/.arc/deliberate/<task-name>/agents/visual-engineering/proposal-round-N.md. "
)
```

### Step 2.3: Wait for completion

Wait for each Agent background task to complete (use `collect_task_output(task_id="...")` to collect results).

### Step 2.4: Cross-review + mutual refutation

**CRITICAL**: Each Agent must **refute each other's opinions** and cannot simply review.

Each Agent must:
1. Read the proposals of the other two Agents
2. **Find out the problems and loopholes in the other party’s point of view**
3. **Use arguments to refute the opponent’s technical choices**
4. Come up with your own alternatives

**oracle review deep + visual-engineering** (using `schedule_task(specialist="oracle", session_ref="<reuse previous session>", ...)`):
- Read `agents/deep/proposal-round-N.md` and `agents/visual-engineering/proposal-round-N.md`
- Refute deep's engineering choices and visual-engineering's front-end design from an architectural perspective
- Output: `agents/oracle/critique-round-N.md`

**deep review oracle + visual-engineering** (using `schedule_task(workstream="deep", session_ref="<reuse previous session>", ...)`):
- Read `agents/oracle/proposal-round-N.md` and `agents/visual-engineering/proposal-round-N.md`
- Refute Oracle's architecture design and visual-engineering experience requirements from an engineering perspective
- Output: `agents/deep/critique-round-N.md`

**visual-engineering review oracle + deep** (using `schedule_task(workstream="visual-engineering", session_ref="<reuse previous session>", ...)`):
- Read `agents/oracle/proposal-round-N.md` and `agents/deep/proposal-round-N.md`
- Refute Oracle's abstract design and deep engineering implementation from a quality perspective
- Output:`agents/visual-engineering/critique-round-N.md`

### Step 2.5: Convergence determination

The main process directly reads the critique of each Agent:
- **No disagreement**: convergence, synthesis of consensus report, written to `convergence/final-consensus.md`
- **Disagreement**: Generate `convergence/round-N-summary.md` and enter the next round

### Step 2.6: Consensus Report

After convergence or reaching the maximum number of rounds, synthesize `convergence/final-consensus.md`:

```markdown
# Consensus report: <task name>

## Problem background
<original problem description>

## Solution overview
<One sentence plan summary>

## Plan details
<Synthesis of viewpoints from all parties>

## Risks and Mitigation
<Identified risks and responses>

## Unresolved differences (if any)
<Consensus reached>
<Unresolved issues and reasons>

## in conclusion
<Final recommendations>
```

---

## Phase 3: Plan Generation

**CRITICAL**: Use [OpenSpec](https://github.com/Fission-AI/OpenSpec) (CLI: `openspec`) to convert the consensus report into a structured executable plan, which is then reviewed and refuted by multiple Agents to finalize it.

OpenSpec uses a spec-driven workflow to generate artifacts in `proposal → specs → design → tasks` order. Each artifact has dependencies and structured templates.

### Step 3.0: OpenSpec initialization

Initialize OpenSpec and create changes within the review workspace:

```bash
cd <workdir>/.arc/deliberate/<task-name>
openspec init --tools none
openspec new change <task-name>
```

Output directory structure:
```
openspec/
├── changes/
│   ├── <task-name>/
│   │   └── .openspec.yaml   # schema: spec-driven
│   └── archive/
└── specs/
```

### Step 3.1: Generate 4 artifacts in sequence

For each artifact (proposal → specs → design → tasks), execute the following process:

1. **Get OpenSpec structured directives**:
```bash
cd <workdir>/.arc/deliberate/<task-name>
openspec instructions <artifact> --change <task-name>
```
The `openspec instructions` output contains `<instruction>` (writing guide), `<template>` (structural template), and `<output>` (target write path).

2. **Execution sub-Agent writing**: Send the content of the OpenSpec instruction + `convergence/final-consensus.md` to the execution sub-Agent (`specialist: "general-purpose"`), which will fill in the template and write it to the specified path.

**Execute sub-Agent to generate proposal** (role scheduling, each artifact is called separately):
```
schedule_task({
description: "OpenSpec proposal generation",
  specialist: "general-purpose",
  run_mode: "background",
  mode: "bypassPermissions",
prompt: "Generate OpenSpec proposal based on consensus report.
Read the following files:
- <workdir>/.arc/deliberate/<task-name>/convergence/final-consensus.md

Fill out the proposal according to the following OpenSpec instructions:
<Paste full output of openspec instructions proposal here>

Write the results to the path specified by the <output> tag. "
})
```

Repeat the above process for `specs`, `design`, and `tasks`. Each artifact must be generated after the prerequisite dependencies are completed (`openspec instructions` will prompt for missing dependencies through the `<warning>` tag).

**Additional requirements when generating `tasks` artifact**: The execution sub-Agent must mark the estimated AI execution time `[~Xmin]` for each task. Please refer to the following benchmark:

| complexity | Estimated time | Example |
|--------|---------|------|
| Simple (single file modification, configuration adjustment, CRUD) | ~1-2min | Modify a configuration item and add a field |
| Medium (new modules/interfaces, spanning 2-3 files) | ~3-5min | Add new API endpoint and add middleware |
| High (architectural changes, cross-module refactoring) | ~5-15min | Reconstruct the authentication system and migrate the data model |
| Very large (new system, large number of files linkage) | ~15-30min | Added new microservices and comprehensive reconstruction |

**Output files** (all under `openspec/changes/<task-name>/`):
- `proposal.md` — Solution proposal (Why / What Changes / Capabilities / Impact)
- `specs/<capability>/spec.md` — Detailed specifications (ADDED Requirements + Scenarios)
- `design.md` — Architecture design (Context / Goals / Decisions / Risks)
- `tasks.md` — Orderly executable tasks (grouped checkbox format + AI time-consuming annotation, can be tracked by `openspec archive`)

### Step 3.2: OpenSpec verification

After the generation is completed, verify the artifact structural integrity and change status:

```bash
cd <workdir>/.arc/deliberate/<task-name>
openspec validate --change <task-name>
openspec status --change <task-name>
```

- `validate` Verifies whether the artifact structure meets the schema requirements
- `status` displays the completion status (missing/present) of each artifact

### Step 3.3: Multi-Agent concurrent review plan

After OpenSpec generates the plan, **Multi-Agent concurrent independent review** (same message, `run_mode: "background"`).

> The following path abbreviation `$CHANGE` stands for `<workdir>/.arc/deliberate/<task-name>/openspec/changes/<task-name>`.

**oracle review plan** (architectural perspective):
```
schedule_task(
  specialist: "oracle",
  capabilities: ["arc:decide"],
  run_mode: "background",
description: "oracle review plan",
prompt: "You are the architect. Review the following OpenSpec plan document and review and refute it from the perspective of global architecture, overall consistency, and reasonable task sequencing.
Read the following files:
- $CHANGE/proposal.md
- All spec.md under $CHANGE/specs/
- $CHANGE/design.md
- $CHANGE/tasks.md
Review requirements:
1. Point out logistical problems and risks in the plan
2. Refute unreasonable task ordering or dependencies
3. Check plan for consistency with consensus reporting
4. Give suggestions for modifications
Write the review results to <workdir>/.arc/deliberate/<task-name>/agents/oracle/plan-review.md. "
)
```

**deep review plan** (engineering perspective):
```
schedule_task(
  workstream: "deep",
  capabilities: ["arc:decide"],
  run_mode: "background",
description: "deep review plan",
prompt: "You are a backend architect. Review the following OpenSpec plan document and review and refute it from the perspectives of backend architecture, performance, security, and feasibility.
Read the following files:
- $CHANGE/proposal.md
- All spec.md under $CHANGE/specs/
- $CHANGE/design.md
- $CHANGE/tasks.md
Review requirements:
1. Point out technical issues and risks in the plan
2. Refute unreasonable task ordering or dependencies
3. Supplement missing backend related tasks
4. Give suggestions for modifications
Write to <workdir>/.arc/deliberate/<task-name>/agents/deep/plan-review.md. "
)
```

**visual-engineering review plan** (front-end and DX perspective):
```
schedule_task(
  workstream: "visual-engineering",
  capabilities: ["arc:decide", "frontend-ui-ux"],
  run_mode: "background",
description: "visual-engineering review plan",
prompt: "You are a front-end and interaction designer. Review the following OpenSpec plan documents and review and refute them from the perspectives of front-end interaction, UI/UX, component architecture, and user experience.
Read the following files:
- $CHANGE/proposal.md
- All spec.md under $CHANGE/specs/
- $CHANGE/design.md
- $CHANGE/tasks.md
Review requirements:
1. Point out front-end/interaction issues in the plan
2. Refute unreasonable design choices
3. Supplement missing front-end related tasks
4. Give suggestions for modifications
Write to <workdir>/.arc/deliberate/<task-name>/agents/visual-engineering/plan-review.md. "
)
```

### Step 3.4: Multi-Agent cross-rebuttal plan review

**CRITICAL**: Each Agent refutes each other's plan review opinions. Each Agent reads the `plan-review.md` of the other two, refutes the unreasonable points, and makes up for the omissions.

Calling method: oracle uses `schedule_task(specialist="oracle", session_ref="<reuse>")`, deep uses `schedule_task(workstream="deep", session_ref="<reuse>")`, visual-engineering uses `schedule_task(workstream="visual-engineering", session_ref="<reuse>")`, and the three run concurrently.

Each Agent output overwrites (updates) its own `plan-review.md` and adds a rebuttal paragraph.

### Step 3.5: Finalize plan

The main process handles it directly, integrates various review reports, and revise the OpenSpec artifact file:

1. Read `agents/oracle/plan-review.md`, `agents/deep/plan-review.md`, `agents/visual-engineering/plan-review.md`
2. Revise `openspec/changes/<task-name>/tasks.md` based on review by all parties (to ensure that tasks are orderly, dependent, and executable)
3. Synchronously update the spec files under `design.md` and `specs/` (if necessary)

**`tasks.md` format requirements** (OpenSpec standard checkbox format + AI time-consuming annotation, progress can be tracked by `openspec archive`):

```markdown
## 1. <Task group name>

- [ ] 1.1 <Task Description> [~2min]
- [ ] 1.2 <Task Description> [~5min]

## 2. <Task group name>

- [ ] 2.1 <Task description> [~3min]
- [ ] 2.2 <Task description> [~10min]
```

> Time annotation is based on the AI ​​agent execution speed (non-manual), refer to the time-consuming benchmark table in Step 3.1.

### Step 3.6: Final verification and archiving

```bash
cd <workdir>/.arc/deliberate/<task-name>
# final verification
openspec validate --change <task-name>
openspec status --change <task-name>
# Archive after execution (optional, executed after Phase 4 is completed)
openspec archive <task-name>
```

---

## Phase 4: Execution

**CRITICAL**: After the plan is finalized, use `schedule_task(workstream="deep")` to execute the code implementation.

### Step 4.1: Agent execution plan

According to the final plan, use deep Agent to execute step by step according to `tasks.md`:

```
schedule_task(
  workstream: "deep",
  capabilities: ["arc:decide"],
description: "Execution Review Plan",
prompt: "According to the task list in .arc/deliberate/<task-name>/openspec/changes/<task-name>/tasks.md, execute the code implementation in sequence.
Also refer to:
- .arc/deliberate/<task-name>/openspec/changes/<task-name>/design.md (Architecture Design)
- .arc/deliberate/<task-name>/openspec/changes/<task-name>/specs/ (specification constraints)
Working directory: <workdir>
Only output the implementation results, do not ask for confirmation.
EOF
```

### Step 4.2: Verification

After execution completes, verify that the code meets the output requirements described in `tasks.md`.

### Step 4.3: Archive changes

After all tasks are completed, archive the OpenSpec changes:

```bash
cd <workdir>/.arc/deliberate/<task-name>
openspec archive <task-name>
```

---

## Timeouts and downgrades

| Condition | deal with |
|------|------|
| Agent timeout > 10min | Use AskUserQuestion to ask the user whether to continue waiting or switch to another Agent |
| Reaching max_ambiguity_rounds still has ambiguity | Mark unresolved ambiguity and enter review stage |
| Reached max_rounds without convergence | Forced synthesis of consensus report, marking unresolved differences |
| openspec command failed | Downgrade to manually writing openspec/changes/<task-name>/tasks.md |

## status feedback

```
[Multi-Agent Deliberation] Task: <task-name>

=== Phase 1: Ambiguity Checking ===
Round 1/3:
├── oracle(role) analysis... [Complete]
├── deep(lane) analysis... [Complete]
├── visual-engineering(lane) analysis... [Complete]
├── Aggregate ambiguities... [N ambiguities]
└── User clarification... [In Progress]

=== Phase 2: Deliberation ===
Round 1/3:
├── oracle proposal... [Completed]
├── deep proposal... [Complete]
├── visual-engineering proposal... [Completed]
├── Cross-review... [Complete]
└── Convergence judgment... [Convergence]

=== Phase 3: Plan Generation (OpenSpec) ===
├── openspec init + new change... [Complete]
├── Generate artifact (proposal→specs→design→tasks)... [Complete]
├── openspec validate... [pass]
├── oracle review... [Completed]
├── deep review... [Completed]
├── visual-engineering review... [Complete]
├── Multi-Agent cross-rebuttal... [Complete]
└── Plan finalized... [Complete]

=== Phase 4: Execution ===
├── deep Agent execution... [In progress]
└── Verification... [TBD]
```

## Quick Reference

| stage | step | Output path |
|------|------|---------|
| Ambiguity checking | Multi-Agent analysis → aggregation → user clarification → judgment | `agents/(oracle\|deep\|visual-engineering)/ambiguity-round-N.md` |
| review | Proposal → Review → Convergence Determination → Consensus Report | `agents/(oracle\|deep\|visual-engineering)/proposal-round-N.md`, `convergence/final-consensus.md` |
| Plan generation | OpenSpec init → generate artifact → verification → multi-agent review → cross-refutation → finalization | `openspec/changes/<task-name>/(proposal\|design\|tasks).md`, `openspec/changes/<task-name>/specs/` |
| implement | deep Agent implementation code according to tasks.md → Archive | Project code + `openspec archive` |

## Anti-Patterns

**CRITICAL: The following behaviors are FORBIDDEN in arc:decide execution:**

### Consensus Anti-Patterns

- **Premature Consensus**: Declaring consensus before all agents report — must collect all perspectives
- **Agent Skip**: Not launching counter-arguments — multi-perspective analysis is mandatory
- **Echo Chamber**: All agents agreeing without debate — indicates insufficient diversity

### Planning Anti-Patterns

- **Vague Deliverables**: "Improve the system" without specific outcomes — must define measurable goals
- **Missing Alternatives**: Only considering one approach — must document at least 2 alternatives
- **Risk Ignorance**: Not documenting failure modes and mitigations — incomplete plan

### OpenSpec Anti-Patterns

- **Validation Skip**: Not running `openspec validate` before archive — broken plans pass through
- **Archive Without Instructions**: Skipping `openspec instructions` phase — implementation guidance missing
- **Session Break**: Starting fresh OpenSpec session instead of continuing — loses context

## Quick call method

| Role | Calling method | Concurrency support |
|------|---------|---------|
| oracle | `schedule_task(specialist="oracle", capabilities=["arc:decide"], run_mode="background", ...)` | Background asynchronous |
| deep | `schedule_task(workstream="deep", capabilities=["arc:decide"], run_mode="background", ...)` | Background asynchronous |
| visual-engineering | `schedule_task(workstream="visual-engineering", capabilities=["arc:decide", "frontend-ui-ux"], run_mode="background", ...)` | Background asynchronous |
| Aggregation/finalization | The main process handles it directly | — |
