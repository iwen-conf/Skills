# Business graph v2: model, equivalence and runtime commands

Follow [deterministic contracts](deterministic-contract.md): match observed prestate/actor/guard
before execution, freeze one Expected, never select it from Actual. The legacy boolean examples
below do not establish deterministic business coverage.


v2 models (`schema_version: 2`) add typed views (system, domain, capability, boundary, role, data,
contract, deployment) over the legacy action/condition/state core; v1 graphs reject every v2 field.
Optional v2 fields: `history[]` (path facts that change later behavior; `set_by`/`cleared_by`/`enables`
transitions, `measurement` for runtime), `equivalence.ignore` (measurements that must not split
states, each with a source) and `reference_bindings` (resolve, cover or rule out a peer reference).
States are merged only when flow, state, role, principal alias, relevant conditions, declared history
and runtime measurements all match and are observed; an unobserved part never merges.
These merge rules apply to model/legacy exploration. Strict `business_contract` execution
rejects `discover run/resume`; `campaign` retains distinct child-run observations.

| Command | Use |
|---|---|
| `explore --flow` | Bounded model exploration; equivalent states close as `merges[]`, budgets still truncate |
| `run-chain --round --max-steps --deadline` | Run the known path (READY cards with one current runner) through `execute`; stops with exit 10 divergence, 11 unknown state, 12 precondition, 13 new branch, 14 budget, 15 blocked, plus a minimal packet |
| `derive --run [--budget]` | Derive obligations from a failed run's observed state (see DERIVED) |
| `discover record/list/triage` | Register exploration observations; behavior outside the model becomes UNSPECIFIED (never PASS/FAIL); the user triages it in a real terminal |
| `discover run/resume/status/abort` | Legacy models without `business_contract`: bounded execution, atomic claim/fencing and independent Oracle. `--mode ai --planner FILE` is a **scripted fixture only**, without an Agent handshake or AI service |
| `campaign run/resume/status/list/abort` | Strict registered variants: observe, prepare, freeze, execute, verify. Selected-scope completion is not full coverage; see [contracts](deterministic-contract.md) |
| `report --kind acceptance\|risk` | Acceptance excludes every risk; the risk report shows declared, reference, derived and UNSPECIFIED risks, refutations, the three-way comparison and the per-origin triage summary |
| `accept-risk` / `reject-risk` / `promote-risk` | Interactive user decisions with the user's own words |
| `risk refute --result --evidence` | Record a bounded refutation attempt bound to evidence by SHA-256; it never closes the risk |
| `reference-import` / `reference-match` | Peer-invariant catalogs (JSONC, sourced) matched by typed slots; output is only peer_reference RISK candidates |
| `mutation-report --file` | Detected/survived/inconclusive injected defects from independent run verdicts; no threshold |
| `snapshot --graph --assets [--out]` | One-off static HTML+JSON workbench export; the page never calls back, executes or approves |

Implementation evidence (`scan`, exploration), requirements (approved rules) and peer references stay
separate sources: none of them becomes a rule by itself, and only approved rules enter acceptance.

Read with [business-graph](business-graph.md), which keeps the confirmation, card and RAW rules these commands rely on.

## Minimal v2 execution

Use [graph-start](business-graph-start.md) for unapproved model drafts; no environment or Runner is
needed. Execution below requires an approved JSONC environment, imported v2 graph, bound account/
fixture and authorized Runner with an independent Oracle. Run from the target root. The user
confirms the catalog and each RULE in a real terminal before planning. `examples/runner-raw.json`
is RAW, not a manifest. Missing setup/runner is BLOCKED.

Set `VERIFY_ROOT`, `VERIFY_CONFIG`, `VERIFY_GRAPH`, `VERIFY_ENV`, `VERIFY_COMMIT`,
`VERIFY_RUNNER_FILE`, `VERIFY_TASK`, `VERIFY_ACCOUNT` and `VERIFY_FIXTURE` from that project.
Register **all** fixtures/runners before round: it freezes their hashes. Register and execute with
the same CLI binary. Then use these commands, copying IDs from each JSON response:

```bash
arc verify runner --file "$VERIFY_RUNNER_FILE"
# Set VERIFY_RUNNER to response manifest.id; repeat registration for other required runners.
arc verify round --graph "$VERIFY_GRAPH" --env "$VERIFY_ENV" --root "$VERIFY_ROOT" \
  --config "$VERIFY_CONFIG" --commit "$VERIFY_COMMIT"
# Set VERIFY_ROUND to response id.
arc verify plan --graph "$VERIFY_GRAPH"
arc verify next --graph "$VERIFY_GRAPH"
# Select VERIFY_TASK from a ready card; bounded plan may need further batches.
arc verify execute --task "$VERIFY_TASK" --runner "$VERIFY_RUNNER" --round "$VERIFY_ROUND" \
  --env "$VERIFY_ENV" --root "$VERIFY_ROOT" --config "$VERIFY_CONFIG" \
  --commit "$VERIFY_COMMIT" --account "$VERIFY_ACCOUNT" --fixture "$VERIFY_FIXTURE"
# Set VERIFY_RUN to response run; inspect verdict and recovery.verdict, even with exit 0.
arc verify trace --run "$VERIFY_RUN"
arc verify explain --run "$VERIFY_RUN" # or --graph "$VERIFY_GRAPH" --node "$VERIFY_NODE"
arc verify gate --graph "$VERIFY_GRAPH"
```

Multi-actor cards add `--accounts`. Models support cross-capability `flows[].capabilities` and `condition_subjects[]`; runners support `setup` sessions, `extract` (`{{var.<name>}}`), and bounded `async_wait`. Offline facts (`step_facts`, `step_assertions`, `run_entity_bindings`) persist across execute/setup/recovery/explore; link tasks via `arc task link-verify` / `verify-status`.

Single-card PASS and partial `run-chain` subsets are not whole-graph PASS (`gate` / `completeness` check full obligations). New fixtures/runners require a new round; export static workbench with `snapshot --graph "$VERIFY_GRAPH" --round "$VERIFY_ROUND" --assets ...`.

Tool HTTP check: `go -C tools/arc-verify test -count=1 -run '^TestExecuteUsesIndependentOracleAndRestoresFixture$' .`; not product E2E.
