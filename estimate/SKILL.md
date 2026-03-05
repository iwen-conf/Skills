---
name: arc:estimate
description: "需求范围已基本明确时使用：输出工时区间、风险系数、并行波次排期与里程碑。"
---

# arc:estimate Skill

Work estimation for software tasks using atomic rounds, module aggregation, and risk calibration.

## Overview

arc:estimate provides structured work estimation for software development tasks:

- **Round-Based Estimation**: Atomic units of 2-4 minutes each
- **Module Aggregation**: Combine rounds into logical modules (2-15 rounds)
- **Wave Planning**: Group modules into parallel-executable batches
- **Risk Calibration**: Apply multipliers based on uncertainty (1.0–2.0x)

## Quick Contract

- **Trigger**：需求已相对明确，需要排期、资源分配或风险缓冲建议。
- **Inputs**：任务描述、模块边界、依赖关系、历史校准样本（如有）。
- **Outputs**：round/module/wave 估算、风险校准结果、关键路径与区间工时。
- **Quality Gate**：对外承诺前必须通过 `## Quality Gates` 的可解释性与可追溯性检查。
- **Decision Tree**：输入信号路由图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree)。

## Routing Matrix

- 统一路由对照见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md)。
- 阶段化上手视图见 [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view)。
- 单页速查见 [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md)。
- 若出现冲突，以本技能 `## When to Use` 的**边界提示**为准。

## Announce

开始时明确说明：  
“我正在使用 `arc:estimate`，先做范围拆解、风险校准，再给区间估算。”

## The Iron Law

```
NO ESTIMATE WITHOUT SCOPE, RISK, AND ASSUMPTION TRACEABILITY
```

没有范围、风险与假设链路，不得输出可承诺工期。

## Workflow

1. 拆解任务为模块与 round 原子单元。
2. 逐模块评估不确定性并应用风险系数。
3. 依据依赖关系生成并行 wave 与关键路径。
4. 输出区间估算、校准依据与缓冲建议。

## Quality Gates

- 每个模块都要有可解释的 round 组成。
- 风险系数必须对应明确触发条件。
- 波次划分必须有依赖依据，不可拍脑袋。
- 结果必须给出基线值与校准后值。

## Red Flags

- 直接给单点时长、无区间与假设。
- 把未知项按“低风险”默认处理。
- 忽略并行依赖导致排期失真。
- 无历史校准样本却给高置信承诺。

## When to Use

- **首选触发**：需求已明确，需要工时区间、资源排期与风险缓冲。
- **典型场景**：并行波次规划、关键路径识别、里程碑承诺校准。
- **边界提示**：需求不清先 `arc:refine`；需要编码落地用 `arc:implement`。

## Invocation

```bash
arc estimate [--calibrate] [--waves] [--json]
```

### Arguments

- `--calibrate`: Output calibration reference points (known tasks)
- `--waves`: Generate wave plan for parallel execution
- `--json`: Output as machine-readable JSON

## Estimation Framework

### Round (Atomic Unit)

A **round** is the smallest estimation unit, representing 2-4 minutes of focused work:

- Read and understand a function
- Write a small test case
- Fix a simple bug
- Refactor a single method

**Average: 3 minutes/round**

### Module (Logical Grouping)

A **module** is a collection of 2-15 rounds that form a logical work unit:

- Feature implementation: 8-12 rounds
- Bug fix: 2-5 rounds
- Refactoring: 5-10 rounds
- Documentation: 3-6 rounds

### Wave (Parallel Batch)

A **wave** is a set of modules that can be executed in parallel:

- Wave 1: Foundation modules (no dependencies)
- Wave 2: Core modules (depend on Wave 1)
- Wave 3: Integration modules (depend on Wave 2)

### Risk Calibration

Apply risk multipliers based on uncertainty level:

| Risk Level | Multiplier | Criteria |
|------------|------------|----------|
| Low | 1.0x | Well-understood, similar work done before |
| Medium | 1.3x | Some uncertainty, minor unknowns |
| High | 1.5x | Significant unknowns, new technology |
| Very High | 2.0x | Major unknowns, research required |

## Estimation Process

### Phase 1: Task Breakdown

1. Read the task description and CLAUDE.md context
2. Identify all modules involved
3. For each module, estimate rounds:
   - Analysis rounds (understanding existing code)
   - Implementation rounds (writing code)
   - Testing rounds (writing/running tests)
   - Review rounds (code review, revisions)

### Phase 2: Risk Assessment

1. For each module, assess risk level:
   - Code familiarity (known/unknown)
   - Technology familiarity (known/unknown)
   - Dependency complexity (simple/complex)
   - Integration risk (isolated/connected)
2. Apply risk multiplier to base rounds

### Phase 3: Wave Planning

1. Identify module dependencies
2. Group independent modules into waves
3. Calculate total duration per wave
4. Identify critical path

### Phase 4: Output

Generate structured estimate:

```markdown
## Estimation Report

### Summary
- Total Rounds: X (base) × Y (risk) = Z
- Total Duration: X minutes / 60 = Y hours
- Waves: N
- Critical Path: Module A → Module B → Module C

### Modules

| Module | Base Rounds | Risk | Adjusted | Duration |
|--------|-------------|------|----------|----------|
| Feature X | 10 | Medium (1.3x) | 13 | 39 min |
| Bug Fix Y | 3 | Low (1.0x) | 3 | 9 min |

### Wave Plan

**Wave 1** (45 min): Module A, Module B
**Wave 2** (30 min): Module C (depends on A)
**Wave 3** (15 min): Module D (depends on B, C)

### Calibration Notes
- Similar to previous task: FEATURE-123 (10 rounds, took 12)
- Adjustment: +2 rounds for additional edge cases
```

## Calibration Reference

Use known tasks to calibrate estimates:

```markdown
### Known Task Library

| Task Type | Typical Rounds | Notes |
|-----------|----------------|-------|
| Add CLI flag | 2-3 | Low risk if similar flags exist |
| Fix type error | 1-2 | May need type research |
| Add API endpoint | 8-12 | Depends on validation complexity |
| Write unit test | 2-4 | Per test case |
| Refactor module | 5-10 | Depends on coupling |
| Add database migration | 4-6 | Plus testing rounds |
| Implement auth flow | 15-25 | Security-sensitive, test heavily |
```

## Anti-Patterns

**CRITICAL: The following behaviors are FORBIDDEN in arc:estimate execution:**

### Estimation Anti-Patterns

- **Human-Time Anchoring**: Estimating based on how long it would take a human — use round-based units
- **Padding by Vibes**: Adding buffer "just in case" without specific risk factors — use risk calibration
- **Single-Point Estimates**: Only providing one number — always give range (optimistic/pessimistic)
- **Ignoring Unknowns**: Pretending all unknowns are known — explicitly list unknowns and their impact

### Process Anti-Patterns

- **Skipping Calibration**: Not referencing similar past tasks — calibrate against known work
- **Forgetting Testing**: Only counting implementation rounds — testing is 30-50% of work
- **Ignoring Integration**: Estimating modules in isolation — account for integration effort
- **Wave Blindness**: Not planning parallel execution — identify wave opportunities

### Output Anti-Patterns

- **False Precision**: "This will take exactly 47 minutes" — use ranges and confidence levels
- **Missing Assumptions**: Not documenting what the estimate assumes — list all assumptions
- **No Risk Factors**: Flat estimate without risk analysis — always include risk assessment

## Context

### Reads From

- `.arc/deliberate/` — Deliberation results with task breakdown
- `.arc/refine/` — Refined prompts with context
- `CLAUDE.md` — Project structure and patterns
- `codemap.md` — Code organization for understanding

### Writes To

- `.arc/estimate/` — Estimation reports and wave plans

### Dependencies

- `arc:refine` — For task context (optional)
- `arc:deliberate` — For task breakdown (optional)

## Examples

### Example 1: Simple Feature

```bash
arc estimate

# Task: Add --verbose flag to CLI

## Modules
1. Add flag definition (2 rounds, Low risk)
2. Update help text (1 round, Low risk)
3. Wire to logger (3 rounds, Low risk)
4. Add test (2 rounds, Low risk)

Total: 8 rounds × 1.0 = 8 rounds (24 min)
```

### Example 2: Complex Feature

```bash
arc estimate --waves

# Task: Implement user authentication with OAuth

## Modules
1. OAuth client setup (8 rounds, High risk) = 12 adjusted
2. Session management (6 rounds, Medium risk) = 8 adjusted
3. Login UI (5 rounds, Low risk) = 5 adjusted
4. Token refresh (4 rounds, High risk) = 6 adjusted
5. Integration tests (8 rounds, Medium risk) = 10 adjusted

Total: 41 adjusted rounds (123 min / 2.05 hours)

## Wave Plan
Wave 1: OAuth client setup (parallel with Login UI)
Wave 2: Session management, Token refresh
Wave 3: Integration tests
```

## References

- Calibration examples: `references/calibration-examples.md`
- Estimation templates: `templates/estimate-template.md`
