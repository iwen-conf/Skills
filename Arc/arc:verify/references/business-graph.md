# Business graph → task cards → real execution

Use with flow-model/flow-matrix when business-chain work is configured. Keep expected functionality
independent of implementation observations. The route is confirmed RULE → graph → bounded card →
real execution → independent RAW → verdict, retaining every original FAIL.

Nodes are feature/action/condition/state; edges are contains/requires/transitions/recovers/forbids.
RULE prerequisites are AND; alternative recovery actions represent OR paths. The short-lived Go
CLI uses official in-process `tursogo` and the target project's local `.arc/verify/data.db`.
Remote servers are test targets; the ledger needs no cloud, PostgreSQL or object store.
Physical fixture reconstruction and external-effect reversal require authorized real runners.

## Confirm the catalog before planning

Reuse the existing project requirement, flow and task documents. Present a business catalog with
feature/action IDs, actor, entry, prerequisites, success, rejection and ways to make each failed
condition true again. Mark the source and unresolved question; ask the user only for decisions
not already explicitly confirmed. Separately record implementation entrances and evidence.
The difference reveals both possibly missing functionality and behavior contradicting a RULE.

`source: user|document` is provenance, not approval. Import creates a candidate; catalog approval
and each RULE approval are required before mandatory planning. AI proposals and industry practice
stay RISK even if the user accepts that risk. Only an explicit normative decision creates a RULE.
An unclear oracle or outcome is a SPECIFICATION GAP; implementation absence requires broader
indexed discovery and real entry evidence before confirmation. Never turn `NOT_FOUND` into FAIL
or silently remove a missing expected feature from the catalog.

## Obligations and bounded exploration

| Obligation | Conditions and observation |
|---|---|
| POSITIVE | All AND prerequisites independently true; normal client operation reaches the confirmed state |
| NEGATIVE | Exactly the selected prerequisite false, others true; normal client refusal and independent readback of the contract's forbidden-effect scope |
| FORCED | The same unmet prerequisite; authorized legitimate API attempt proves server denial and no forbidden write |
| Recovery | Each known path that can satisfy the condition; verify every action's own prerequisites and read back the actual state |
| Cross-role/concurrency | Only when required by the confirmed flow; explicit actors, ordering/interleaving and independent business invariants |
| EXISTENCE (v2) | A rule's entry must exist on the current candidate: satisfied only by a `scan` PRESENT observation bound to the round's commit with unchanged evidence; `NOT_FOUND` stays a candidate gap (BLOCKED), never FAIL |
| SEQUENCE (v2) | Declared `flow` and ordered `steps` (contiguous transitions from the flow's initial state); one runner batch per step, each judged by its own transition rule; the first divergent step stops the run |
| CONSISTENCY (v2) | Two to four `layers` (`ui`/`api`/`database`/`async`), each with its own oracle on the final step; a missing layer blocks, a disagreeing layer fails |
| DERIVED (runtime) | `derive --run` after a FAIL: each approved transition leaving the observed state must hold again from an equivalent start state; it joins the denominator. An undeclared observed state is only an exploration risk |

`plan --limit` materializes bounded basic obligations; `next` returns at most eight cards.
Use `recover` to inspect a condition's local neighborhood, then expand only the selected path.
Unapproved actions, cycles, truncation and unreachable states remain reported gaps. `recover`
does not execute, restore a fixture or add a complete recovery suite to the gate. Keep each card
to IDs, actor/entry/action, condition truth values, expected outcome and oracle; use references
for logs and setup. Do not feed the AI the whole graph or an expanded YAML for every possible path.

Example command sequence:

    go -C tools/arc-verify build -o ~/.local/bin/arc-verify .
    arc-verify init
    arc-verify validate --file /path/to/Skills/tools/arc-verify/examples/academic-v1.json
    arc-verify import --file /path/to/Skills/tools/arc-verify/examples/academic-v1.json
    arc-verify approve --graph academic-v1
    arc-verify approve --graph academic-v1 --rule R-ENROLL
    arc-verify plan --graph academic-v1
    arc-verify next --graph academic-v1
    arc-verify fixtures --file /path/to/Skills/tools/arc-verify/examples/fixtures.json
    arc-verify start --task academic-v1/R-ENROLL/POSITIVE --env test60 --commit <sha> --account student-01 --fixture course-open
    arc-verify step --run <run> --expected ENROLLED --observed ENROLLED --evidence .arc/verify/raw/<run>-01.json
    arc-verify finish --run <run>
    arc-verify trace --run <run>
    arc-verify gate --graph academic-v1

Approval requires an interactive human terminal; agents must not supply the user's confirmation
through a pipe or PTY. The prompt alone cannot prove who typed it.
Evidence must bind the complete identity returned by start, run ID, actual state, runner identity,
oracle and exit code. That start identity is expected context: a real runner must independently
verify the served candidate, effective target, principal and fixture state before recording it.
A user-owned CLI process and same-user file writes are not a cryptographic trust boundary;
high assurance requires an isolated runner or protected CI check.

Project `.arc/policy.yaml` may enable the graph gate:

    verification:
      business_graph:
        graph: academic-v1
        db: .arc/verify/data.db

Once `.arc/verify/config.jsonc` exists, the policy gate blocks unconfigured graph identity.
Preserve `verification.test_commands` and do not replace code tests with graph tests.

## v2 model and runtime commands

v2 fields, state equivalence and the `run-chain`, `derive`, `discover`, risk, reference, mutation and
`snapshot` commands: [business-graph-v2](business-graph-v2.md).

## Implementation inventory and reusable test data

The Go CLI also accepts `scan --file` (actual discovered action plus evidence; add `"commit"` so
EXISTENCE can bind the observation to a candidate) and `gaps --graph` (contract action compared with
most recent observation).
`NOT_FOUND` means a **possible absence** to investigate, not a confirmed product defect.
`fixtures --file` imports a fixture recipe and `fixture --id` retrieves it;
no real server data is mutated by these commands. User-approved test-data setup
and replay operations require explicit runner adapters and a state readback.

Each execution step has its own RAW and independent expected graph state.
The first diverging step is preserved as FAIL; a known safe observed state can
be a REPLAN candidate, never a silent correction. `finish` and `gate`
independently validate evidence content, file identity/hash and frozen task meaning; changed or deleted
evidence invalidates completion. Each `UNCHANGED` rejection requires
before/after equality in the contract's forbidden-effect readback and rejected-operation evidence,
not merely a string. Contracted failure/audit records outside that scope may legitimately change.

## Execution card semantics (frozen contract)

- Every RULE declares actor, UI/API entry, operation, AND conditions, positive and rejected outcomes and a separately sourced oracle. A missing declaration blocks model validation.
- `requires` edges and RULE conditions must match both ways. Distinct obligations cannot share a task ID; idempotent planning preserves an identical stored obligation and rejects a conflicting one.
- `next` expands only up to eight ready cards at a time, providing their actor, entry, expected condition truth values, action and oracle. The model is never copied wholesale into the AI context.
- `recover --graph <id> --condition <id>` searches known recovery actions, their prerequisites and cycles up to a bounded depth. Actions not backed by approved rules are marked unresolved, not executed.
- `step --run --expected --observed --evidence` records a sequential step. A final action step must include every observed precondition in RAW; a wrong prerequisite is BLOCKED/RESET, not PASS.
- A verified negative/forced rejection requires server-side denial and equal before/after readbacks of the forbidden business effects; retain separately permitted failure/audit effects. A real client success message by itself does not prove the business postcondition.
- `trace` identifies the first divergent step and the latest observed state; `finish` cannot pass an empty run or a run with altered evidence.
- `abort --run --reason` retains the stopped run and fixture locks. `retry --task --reason --commit --evidence` requires a fresh independent fixture checkpoint; checkpoint registration, READY and lock release commit atomically. These commands **do not restore data**.
- `prepare --graph --env --commit` reopens stale PASS tasks for a new candidate/config/environment, preserving old run verdicts. It refuses RUNNING graphs and leaves FAIL/BLOCKED tasks awaiting checkpoint recovery. Preparation is `NOT_RUN`, not execution proof.
- `arc-policy` receipts must be revalidated against the local graph and RAW before delivery, not just against the Git tree.

## RAW, verdict and resume boundary

The runner records `identity` (graph/task/environment/target, source commit/worktree/config/graph
hashes, actor/account/principal/account hash, fixture/hash), `run`, sequential `step`,
independently specified `expected`, actual `observed`,
`runner`, `oracle` and an explicit integer `exit_code`. Final action RAW includes every required
condition as an observed boolean. Rejections include explicit `denied` and nonempty before/after
readbacks. Duplicate JSON fields, including case-fold aliases in Go inputs, are invalid; null,
omitted or blank observations cannot acquire meaning from language zero values.

Missing/malformed evidence is rejected; the claim remains BLOCKED until recollected. A nonzero
runner exit is BLOCKED/HALT, even when a status string looks correct. Wrong setup conditions are
BLOCKED/RESET. With a successful observer and correct setup, wrong state, explicit non-denial or
forbidden mutation is FAIL. Preserve RAW and the first divergence before diagnosis. An observer's
failure cannot be handed to `arc:code` as a confirmed product defect.

After FAIL, use `trace` and independent readback to decide whether the actual state is safe and
has a confirmed next/recovery path. Continue in a new task/run; otherwise HALT/BLOCK. `abort`
and `retry` retain history but do not undo external effects. Revalidate fixture state and the
candidate/environment before resuming. Changed RULEs get a new graph version; changed code,
config or environment makes affected old runs stale without deleting their historical FAIL.

Store environment/account aliases and credential references in the project's JSONC config;
keep secrets out of graph, RAW, logs and git. Distinguish independent and synchronized test
environments using the project's actual aliases. Existing fixture recipes may be reused only
after live state and actor identity are verified. CLI fixture import/query records references;
account import requires the exact actor role, principal and environment; business fixtures belong
to that environment. Conflicting recipes under one ID are rejected atomically; identical imports
report zero additions. Start locks the account and fixture; PASS releases them, FAIL/BLOCKED retain
them. Physical reconstruction and live identity/state observations remain runner obligations.

For recovery, inspect `context --run <old-run> --phase fixture --commit <current-sha>` with the
current `--root`/`--config`. Its `EXPECTED_CONTEXT_NOT_EVIDENCE` output supplies expected binding,
recipe state and required conditions. An independent observer then writes RAW with that current
identity, the old run ID, `phase: fixture`, `step: 0`, actual state/conditions, `safe_to_continue:
true`, frozen oracle and exit 0. `retry` checks it, preserves its hash and associates it with the
old run; `start` rechecks that checkpoint and current candidate before acquiring fresh locks.
Unknown state, changed checkpoint or wrong principal/conditions blocks continuation.

## Continuous strengthening

For every confirmed failure, keep its RULE/FLOW/condition/task/run/step IDs, smallest reproducer,
fixture recipe, candidate/environment, RAW and expected-vs-actual difference. After root-cause
repair, re-run the failed and affected sibling obligations in a new run. New AI hypotheses remain
RISK until confirmed; do not expand business scope to make a diagnosis fit.

Prove the gate is sensitive to missing, stale, duplicate and altered evidence. Then benchmark
real local Turso under the same candidate and workload, recording command, versions, sample size,
latency, allocations and coverage limits. A parser/gate selftest proves that tool boundary;
it cannot stand in for real business E2E, complete path enumeration or protected runner provenance.

## Six-skill phase handoff

The canonical phase mapping and authority limits are
[Arc shared workflow §6](../../_shared/workflow.md).
The current `arc-verify handoff` command builds a bounded read-only packet:
`arc:explore` verifies implementation facts; `arc:plan` owns accepted
requirements; `arc:verify` maintains test graph/RAW; `arc:code` repairs
confirmed failures; `arc:audit` evaluates findings; `arc:vet` evaluates
new runner dependencies. One existing task handoff record remains the
single progress tracker. Graph FAIL must not deadlock code repairs;
the graph gate is compulsory on `arc:verify` completion claims.
