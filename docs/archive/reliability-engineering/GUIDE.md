---
name: industrial-business-reliability-engineer
description: >
  Provides the standalone industrial business verification framework for explicit framework adoption,
  maintenance or review. In this repository it is a source distribution; ordinary runtime business
  reliability tasks use arc:reliability and native execution uses arc:test.
---

# Industrial Business Reliability Engineer

## Distribution boundary

This directory is the retained standalone source package, not part of the default Arc/vertical runtime installation. It remains independently usable when selected for framework adoption. Do not load both full frameworks for ordinary reliability work; Arc owns its adapted runtime contract.

## Mode selection

| Mode | Work and references |
| --- | --- |
| Initial program adoption | Inspect current tooling, then [artifact model](references/program-artifacts.md), [workflow phases](references/program-workflow.md) and [project adoption](references/project-adoption.md). Establish only missing responsibilities. |
| One change / incident follow-up | Reuse durable specifications and bindings; select affected phases, claims and the [change plan](templates/change-verification-plan.md). No mandatory four new files or thirteen-phase replay. |
| Read-only assessment | Inspect existing claims, bindings and evidence; return gaps and status. Do not install tools, initialize artifacts, inject faults or mutate environments. |
| Concurrency / platform / CI policy | Read only the relevant [domain and cadence section](references/domain-and-cadence.md). |

## Execution paths and environment

`SKILL_ROOT` is this loaded directory; `PROJECT_ROOT` is the discovered target repository. Run scripts via `python3 "$SKILL_ROOT/scripts/<tool>.py"` while setting cwd to PROJECT_ROOT; plan/binding/evidence paths refer to the project or explicit temporary directory. Discover current paths before use. No missing tool, dependency, binding or oracle can become PASS.

Host rules take precedence: local PostgreSQL servers and local container runtimes are prohibited under the current user policy. Use existing authorized remote isolated instances. PostgreSQL qualification routes to arc-dbqual; no real experiment is implied by a read-only review.

## 1. Core operating principles

1. **Business first.** Understand what users/business/operators expect before choosing test techniques.
2. **Outcome over transport success.** `HTTP 200`, `rows_affected=1`, or “message published” are not sufficient business oracles unless the contract says they are the final outcome.
3. **Specification and execution are separate.** Business rules should remain stable when implementation changes; repository-specific test bindings may change freely.
4. **Applicability is explicit.** Declare whether state, permissions, concurrency, retries, side effects, time, compatibility, migration, async behavior, destructive behavior, or long-running effects apply. Do not require irrelevant ceremony.
5. **Real semantics require real boundaries.** If correctness depends on PostgreSQL isolation/locks, broker redelivery, filesystem durability, network timeout, protocol behavior, or hardware, include evidence against the real implementation or a faithful harness.
6. **Reuse the repository first.** Prefer existing runners, fixtures, Docker Compose/Testcontainers, test clients, CI and observability before introducing new frameworks.
7. **Tools follow failure models.** Do not install fuzzers, mutation tools, model checkers or chaos tooling merely because they exist.
8. **Tests need oracles.** Randomness without a correctness predicate is activity, not verification.
9. **Verification must be reproducible.** Persist seeds, schedules, corpora, exact tool versions and failure artifacts when relevant.
10. **The test system itself must be trustworthy.** Keep its unit tests hermetic; run real Docker/database/toolchain checks as explicit integration self-tests.
11. **Missing evidence is not PASS.** Use factual states: `PASS`, `FAIL`, `BLOCKED`, `NOT_RUN`, `FLAKY`.
12. **No “production-only” excuse without attempting testability.** Transaction, deadlock, retry, duplicate, cross-business race, timeout, partial failure and recovery behavior are test targets. If blocked, identify the exact missing prerequisite.

For large program work read [industrial operating model](references/industrial-operating-model.md).


## 7. Required output when reviewing or hardening a project

Return useful engineering artifacts, not a scorecard.

At minimum report:

### Business model
- changed capabilities/rules;
- applicable concerns;
- expected and forbidden outcomes;
- specification gaps.

### Highest-risk failure paths
- what can go wrong;
- business consequence;
- where in architecture it can occur.

### Verification plan
- concrete scenarios;
- oracle for each important scenario;
- chosen evidence layer and why;
- real dependencies/environment required.

### Implementation
- tests/harnesses/bindings added or changed;
- tools installed and repository changes;
- testability seams introduced.

### Execution evidence
- exactly what ran;
- PASS/FAIL/BLOCKED/NOT_RUN/FLAKY;
- failure reproduction data.

### Release decision inputs
- required evidence still missing;
- unresolved P0/P1 defects;
- rollback/containment readiness;
- residual risk stated plainly.

Do not emit an overall “85/100 reliability” number unless a user explicitly asks for a scoring exercise. Scores are not release evidence.


## 9. Stop rule

Testing is not complete in the mathematical sense. Stop/escalate based on evidence and residual risk.

A change is reasonably verified when:

- business behavior and applicable dimensions are explicit;
- critical rules have trustworthy oracles;
- high-risk scenarios are bound to credible evidence;
- real dependency semantics are tested where they matter;
- required scenarios actually ran and passed;
- failures are reproducible or preserved;
- release containment exists where escaped defects remain plausible;
- remaining uncertainty is named rather than hidden.

Do not keep generating low-value cases merely to increase counts.
