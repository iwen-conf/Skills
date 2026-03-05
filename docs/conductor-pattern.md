# Conductor Pattern Design

## Overview

The Conductor Pattern enables chained skill orchestration, where one skill (the Conductor) plans and delegates work to other skills (the Workers) in a structured workflow.

**Inspired by**: claude-mem's `make-plan` → `do` pattern

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Conductor Skill                         │
│  (arc:exec, arc:decide, or custom orchestrator)        │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  1. PLAN    ──▶  2. DELEGATE  ──▶  3. VERIFY  ──▶  4. ITERATE │
│       │               │                  │                │
│       ▼               ▼                  ▼                │
│   Generate     schedule_task()        Collect            Loop
│   Workflow        Dispatch           Results            Back
│                                                       │
└─────────────────────────────────────────────────────────────┘
                    │
                    ▼ (delegates to)
    ┌───────────┬───────────┬───────────┬───────────┐
    │           │           │           │           │
    ▼           ▼           ▼           ▼           ▼
┌───────┐  ┌───────┐  ┌───────┐  ┌───────┐  ┌───────┐
│arc:   │  │arc:   │  │arc:   │  │arc:   │  │arc:   │
│refine │  │delib- │  │impl-  │  │review │  │simul- │
│       │  │erate  │  │ement  │  │       │  │ate    │
└───────┘  └───────┘  └───────┘  └───────┘  └───────┘
   Worker     Worker     Worker     Worker     Worker
```

## Core Concepts

### 1. Conductor Skill

The Conductor is responsible for:
- **Planning**: Generate a workflow with ordered steps
- **Delegation**: Dispatch tasks to Worker skills via unified Scheduling API
- **Verification**: Collect and validate results
- **Iteration**: Loop back on failure or continue on success

### 2. Worker Skills

Workers are regular arc: skills that:
- Accept a specific task with clear inputs
- Execute the task independently
- Return structured results
- Can be chained in sequence or parallel

### 3. Workflow Definition

A workflow is a declarative specification of the orchestration:

```yaml
workflow:
  name: feature-implementation
  description: Implement a new feature from request to deployment
  
  steps:
    - id: refine
      skill: arc:clarify
      input:
        prompt: "${user_request}"
      output:
        enhanced_prompt: ".arc/arc:clarify/enhanced-prompt.md"
        
    - id: deliberate
      skill: arc:decide
      depends_on: [refine]
      input:
        prompt_file: ".arc/arc:clarify/enhanced-prompt.md"
      output:
        plan: ".arc/arc:decide/consensus.md"
        
    - id: implement
      skill: arc:build
      depends_on: [deliberate]
      input:
        plan_file: ".arc/arc:decide/consensus.md"
      output:
        changes: ".arc/arc:build/changes/"
        
    - id: review
      skill: arc:audit
      depends_on: [implement]
      input:
        changes_dir: ".arc/arc:build/changes/"
      output:
        report: ".arc/arc:audit/report.md"
        
    - id: test
      skill: arc:e2e
      depends_on: [implement]
      input:
        changes_dir: ".arc/arc:build/changes/"
      output:
        results: "reports/"
        
  verification:
    - step: review
      condition: "report.contains('PASS')"
    - step: test
      condition: "results.all_passed == true"
      
  failure_handling:
    max_iterations: 3
    on_failure: arc:fix
```

### 4. Session Continuity

Workers maintain session continuity via `task_ref`:

```python
# Step 1: Initial delegation
result = schedule_task(
    skill="arc:clarify",
    prompt="...",
    execution_mode="background"
)
task_ref = result["task_ref"]

# Step 2: Continue session
result = schedule_task(
    task_ref=task_ref,
    prompt="Continue with the enhanced prompt..."
)

# Step 3: Pass to next worker with context
result = schedule_task(
    skill="arc:build",
    prompt=f"Based on refinement session {task_ref}..."
)
```

## Implementation Patterns

### Pattern 1: Sequential Chain

Simple linear workflow where each step depends on the previous:

```
refine → deliberate → implement → review → test
```

**Use case**: Feature implementation with quality gates

### Pattern 2: Parallel Fan-Out

Multiple independent workers execute in parallel:

```
          ┌─→ arc:gate ─┐
refine ──┼─→ arc:audit ─┼─→ aggregate
          └─→ arc:e2e ─┘
```

**Use case**: Comprehensive project assessment

### Pattern 3: Conditional Branching

Workflow branches based on conditions:

```
                ┌─→ arc:build ─→ done
refine ─→ check ┤
                └─→ arc:decide ─→ arc:build ─→ done
```

**Use case**: Simple vs complex task routing

### Pattern 4: Loop with Triage

Iteration with automatic error recovery:

```
implement → test → [pass?] → done
                 ↓ [fail]
              triage → implement (loop, max 3)
```

**Use case**: Self-healing test loops

## Integration with Existing Skills

### arc:exec as Conductor

`arc:exec` already acts as a conductor. Enhance with:

1. **Workflow Templates**: Pre-defined workflows for common patterns
2. **Progress Tracking**: Track workflow state across steps
3. **Checkpoint/Resume**: Save state for long-running workflows

### arc:decide as Planner

`arc:decide` generates structured plans. Enhance with:

1. **Workflow Export**: Output workflow YAML from consensus
2. **Step Estimation**: Add time/resource estimates per step
3. **Dependency Graph**: Visual representation of dependencies

### arc:fix --mode retest-loop as Iterator

`arc:fix --mode retest-loop` already implements the loop pattern. Enhance with:

1. **Conductor Integration**: Accept workflow definition
2. **State Machine**: Track workflow state across iterations
3. **Escalation**: Escalate to human after max iterations

## File Structure

```
.arc/
├── conductor/
│   ├── workflows/
│   │   ├── feature-implementation.yaml
│   │   ├── bug-fix.yaml
│   │   └── code-review.yaml
│   ├── state/
│   │   └── <workflow-id>.json
│   └── templates/
│       └── workflow-template.yaml
```

## API Design

### Conductor Skill Interface

```python
# Start a workflow
arc conductor start --workflow feature-implementation --input "Add user authentication"

# Check workflow status
arc conductor status <workflow-id>

# Resume interrupted workflow
arc conductor resume <workflow-id>

# Cancel workflow
arc conductor cancel <workflow-id>
```

### Programmatic API

```python
from arc_conductor import Conductor

conductor = Conductor()

# Start workflow
run = conductor.start(
    workflow="feature-implementation",
    input={"user_request": "Add user authentication"}
)

# Wait for completion
result = conductor.wait(run.id)

# Or iterate step by step
for step in conductor.steps(run.id):
    print(f"Step {step.name}: {step.status}")
```

## Implementation Roadmap

### Phase 1: Core Infrastructure
- [ ] Workflow YAML schema definition
- [ ] Workflow parser and validator
- [ ] State management (save/resume)

### Phase 2: Execution Engine
- [ ] Sequential chain execution
- [ ] Parallel fan-out execution
- [ ] Conditional branching

### Phase 3: Integration
- [ ] arc:exec workflow templates
- [ ] arc:fix --mode retest-loop conductor mode
- [ ] arc:decide workflow export

### Phase 4: Monitoring
- [ ] Progress tracking
- [ ] Failure recovery
- [ ] Human escalation

## Anti-Patterns

**CRITICAL: The following behaviors are FORBIDDEN in Conductor Pattern:**

### Orchestration Anti-Patterns

- **Orphan Sessions**: Failing to pass `task_ref` between workers — breaks continuity
- **Infinite Loops**: No max_iterations limit — stuck forever on failures
- **Blind Delegation**: Dispatching without verifying worker completion — lost results
- **State Blindness**: Not persisting workflow state — can't resume after interruption

### Workflow Anti-Patterns

- **Circular Dependencies**: Step A depends on B, B depends on A — deadlock
- **Missing Outputs**: Step doesn't declare outputs — downstream can't consume
- **Oversized Steps**: One step does too much — defeats purpose of decomposition
- **Underspecified Inputs**: Vague input definitions — workers get wrong data

### Failure Handling Anti-Patterns

- **Silent Failures**: Not logging step failures — invisible problems
- **Retry Blindness**: Retrying without analyzing failure cause — repeated failures
- **No Escalation**: No human escalation path — stuck on unresolvable issues

## References

- claude-mem: `make-plan` → `do` pattern
- Workflow patterns: https://www.workflowpatterns.com/
- Temporal.io: Workflow orchestration best practices
