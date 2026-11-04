# Business graph → task cards → real execution

For requested/configured graph work, follow this lifecycle instead of building a second branch
ledger. Reuse confirmed FLOW/RULE IDs and existing project progress. Ordinary tests do not need a
graph. v2 execution/commands: [business-graph-v2](business-graph-v2.md).

Whole-system/all-business/all-interaction tests select this route without requiring an existing
DB or user-supplied model. For first modeling or missing capabilities, use
[graph-start](business-graph-start.md); the existing Agent discovers semantics and writes the model.

The Go CLI opens target-local `.arc/verify/data.db` with in-process `tursogo`, then exits.
Remote servers are authorized test targets; no cloud ledger or persistent service. Nodes are
feature/action/condition/state; edges contains/requires/transitions/recovers/forbids.
Prerequisites are AND, recovery alternatives OR. Keep only the current bounded card in context.

## 1. Confirm expected functionality separately

Confirm the discovered/reused catalog's IDs, actors, entries, prerequisites, outcomes and recovery paths;
ask only unresolved decisions, citing sources. Keep implementation evidence separate. Preserve
expected-but-unfound features as candidate gaps until broader indexed discovery and real entry
evidence confirm absence. `NOT_FOUND` never means FAIL.

Import creates a candidate; `source: user|document` is not approval. Approve catalog and each RULE
before planning. Accepted AI/industry risks stay RISK; only normative decisions create RULEs.
Undefined outcome/oracle is a SPECIFICATION GAP. Approval needs a human terminal; agents must not
answer through pipes/PTYs. A prompt alone does not authenticate the person.

RULEs declare actor, UI/API entry, operation, AND conditions, outcomes and independent oracle;
missing fields block. Conditions must match `requires` both ways. Task-ID collisions/conflicting
obligations fail; identical planning is idempotent. Changed RULEs need a new graph version.

## 2. Bind fixtures and plan bounded cards

JSONC stores project environment/account aliases and credential references; no secrets in graph,
RAW, logs or git. Distinguish independent/synchronized environments. Accounts need exact role,
principal and environment; business fixtures belong to that environment. Conflicting IDs fail
atomically; identical imports add zero. `fixtures --file` / `fixture --id` manage recipes, not live data.

`scan --file` records implementation evidence (commit required for EXISTENCE); `gaps --graph`
compares it with expectations. Scan/configured adapters never prove execution.

`plan --limit` materializes bounded obligations; `next` returns ≤8 cards: IDs, actor/entry/action,
condition truths, outcome, oracle and setup/log references. Never expand the whole graph/YAML.
Cycles/truncation/unapproved actions/unreachable states stay gaps. `recover --graph --condition`
inspects bounded recovery prerequisites; it neither executes nor adds a full recovery suite.

| Obligation | Required observation |
|---|---|
| POSITIVE | All AND prerequisites independently true; client operation reaches confirmed state |
| NEGATIVE | Selected prerequisite false, all others true; client refusal and forbidden-effect readback |
| FORCED | Same unmet prerequisite; authorized legitimate API attempt proves server denial |
| Recovery | Each known path satisfying the condition, including its own prerequisites and actual state |
| Cross-role/concurrency | Only when contracted; actors, ordering/interleaving and independent invariants |
| EXISTENCE (v2) | PRESENT scan on round commit with unchanged evidence; NOT_FOUND remains BLOCKED/candidate |
| SEQUENCE (v2) | Declared ordered contiguous transitions from initial state; batch per step, own rule; stop at first divergence |
| CONSISTENCY (v2) | Two–four declared ui/api/database/async layers with separate final oracles; missing blocks, disagreement fails |
| DERIVED | After FAIL, `derive --run` adds approved outgoing transitions from equivalent observed state to denominator; undeclared states stay exploration risks |

## 3. Execute and retain independent RAW

Use the [v2 execution sequence](business-graph-v2.md#minimal-v2-execution), or the
[v1 manual ledger example](_shared/tools/arc-verify/README.md#manual-v1-ledger-example).
Recording RAW is not execution.

The runner independently verifies served candidate, effective target, principal and live fixture
state. Context/start output is expected binding, never observation. Start locks account/fixture;
PASS releases them, FAIL/BLOCKED retain them. Same-user files/CLI are not a cryptographic boundary;
high assurance needs independently trusted execution.

RAW records complete start identity (graph/task/environment/target, commit/worktree/config/graph
hashes, actor/account/principal/account hash, fixture/hash), run, sequential step, independent
expected, observed, runner, oracle and integer exit_code. Final action observes every condition as
boolean. Duplicate/case-fold fields and null/omitted/blank observations fail. `finish`/`gate`
recheck frozen meaning and RAW identity/content/hash; empty/altered/deleted evidence cannot pass.

### Rejection scope

Negative/forced rejection needs explicit server denial and nonempty before/after readbacks equal
within the contract's forbidden durable/external-effect scope. Explicitly permitted denial/audit
effects are observed separately. `UNCHANGED` alone, HTTP 200 or a client message proves nothing.
This matches the [flow readback contract](flow-matrix.md#4-prepost-state-diff).

Missing/malformed RAW or runner failure is BLOCKED/HALT; wrong setup BLOCKED/RESET. Correct setup
and successful observation of wrong state, non-denial or forbidden mutation is FAIL. Observer errors
are not product defects. `trace` retains first divergence/latest state; later PASS never erases FAIL.

## 4. Recover without replaying uncertain effects

After FAIL, read state independently; `trace` locates safe approved next/recovery paths. Continue
in a linked new task/run; unknown state HALTs/BLOCKs. Abort retains history/locks; retry requires a
fresh independent checkpoint and atomically registers it, sets READY, releases old locks.
Neither restores data nor reverses effects.

Use `context --run <old-run> --phase fixture --commit <current-sha>` with current root/config.
Its EXPECTED_CONTEXT_NOT_EVIDENCE supplies binding/recipe/conditions. An independent observer writes
checkpoint RAW with current identity, old run, phase fixture, step 0, actual state/conditions,
safe_to_continue true, frozen oracle and exit 0. Retry preserves its hash; start rechecks checkpoint
and candidate before locking again. Unknown state, altered checkpoint or wrong principal/conditions
blocks continuation. Physical reconstruction/reversal requires an authorized real runner.

`prepare --graph --env --commit` reopens stale PASS cards for a changed candidate/config/environment,
preserving history. RUNNING blocks prepare; FAIL/BLOCKED still need checkpoint recovery. Preparation
is NOT_RUN. Recipe changes require a new fixture ID and live identity/state readback before reuse.

## 5. Gate, hand off and strengthen

Project `.arc/policy.yaml` enables graph verification:

```yaml
verification:
  business_graph:
    graph: <confirmed-graph-version>
    db: .arc/verify/data.db
```

Existing `.arc/verify/config.jsonc` requires graph identity for policy verification. Preserve
verification.test_commands. Required graphs only accept PASS, regardless of fail_closed;
current-tree receipts recheck graph/RAW, committed receipts retain original checks. AUTHORIZED is no proof.

For phase ownership/task identity use [shared workflow §6](_shared/Arc/_shared/workflow.md#6-graph-based-business-chain-verification-across-arc-skills).
`handoff --to <arc-skill>` returns bounded context, not approval/proof. Keep one progress Markdown;
verify owns graph/RAW, code fixes confirmed defects without waiting for the old red graph to pass.

Retain failure IDs, reproducer, recipe, candidate/environment, RAW and expected-vs-actual; rerun failed
and sibling obligations in new runs. New hypotheses stay RISK. Check missing/stale/duplicate/altered
evidence sensitivity. Core/planned-rule PASS does not prove real E2E, full recovery, cross-role,
concurrency or trusted provenance; each needs separate execution evidence.
