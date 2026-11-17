# Executable business contracts

Read [shared semantics](../../_shared/business-verification.md) first. Keep graph schema version
2 and add `business_contract.version: 1`; old model/RAW history stays readable with explicit
legacy scope. Contract validation is part of normal model loading, import and execution.

## Kernel and CLI

```bash
arc verify validate --file MODEL.json
arc verify contract check --file MODEL.json
arc verify contract match --file MODEL.json --action ACTION-ID --input observed.json
arc verify contract prepare --file MODEL.json --input preparation-request.json
arc verify contract compare --file MODEL.json --input frozen-and-actual.json
```

Use `--graph ID` with the active ledger instead of `--file` to analyze an imported model.
`check` returns finite-domain partition proof, not an execution verdict. `match` returns READY
only when it can freeze one confirmed expectation. `prepare` returns a bounded plan; predicted
effects do not establish real state. `compare` rechecks frozen integrity and never rematches.
Invalid/gapped/blocked results return nonzero. CLI analysis does not mint a run receipt.

The model declares typed `subjects[].attributes`, confirmed sourced `branches[]`, optional
`preparation[]`, and an explicit finite `coverage` scope/domain/budget. Each branch specifies
`action`, `rule`, `from_states`, a typed `when` predicate and `expected`. Conditions reference
`{subject, attribute}` or typed literals. Runtime `BusinessObservation` carries independently
read `values`, identity `bindings`, state, flow and oracle `sources`.

Expected mode `deterministic` has one outcome. `allowed_outcomes` requires a sourced reason;
common invariants apply to all permitted outcomes. Named checks explain response, persistence,
events, side effects, forbidden effects and invariants independently. Missing observations or
oracle provenance cannot satisfy a check. Explicit denied=false differs from missing denial data.

HTTP runners may declare `input_measurements` JSON pointers into the **resolved request body**.
These facts come from request capture, not fixture declarations or oracle constants. Requested
amount, entity and account selection therefore participate in the same pre-action freeze as the
independently observed state. `response_measurements` captures explicit response fields; named
persistence and event assertions still use independent oracles.

## Execution and history

Registered runners and rounds remain the execution entry. Before each target action the runner
collects its independent oracle observations, freezes the matched branch and records the decision.
No-match/ambiguity/wrong prestate leaves a blocked, unattempted action record. After execution,
the actual observation and comparison are retained beside the frozen object in
`business_expectations`; they are not folded into Expected. A later run must keep its own identity.

Use `arc verify explain --run ID` and `snapshot --graph ID --assets BUILD --run ID` to inspect
the validated facts. Snapshots include typed facts and evidence verification errors. Without
`--run`, the current graph/environment/round exports its latest 32 runs and explicit truncation
counts. The browser cannot repair or reclassify evidence.

`recovery.mode: "checkpoint"` retains an owned fixture after independent identity/state/checks
verification, with no reset operation. It is suitable for durable business records that should
remain available for continuation. Default `restore` keeps the prior restoration protocol.
Checkpoint evidence is labeled as retained, never restored; a subsequent run must independently
observe and bind its entities. Sessions/variables are not silently inherited across runs.

## Additive schema and real-project adapter

For an already active ledger, `arc verify init` applies additive tables transactionally. It does
not delete or rewrite old models, runs or RAW. `business_contracts`, `business_branches`,
`preparation_actions` and `business_expectation_captures` make the immutable contract and durable
pre-action captures queryable. The old ledger activation remains `arc db migrate --confirm-offline`.

The [pay_gateway adapter](../../../tools/arc-verify/examples/pay-gateway/README.md) binds the
reviewed source, owned PostgreSQL fixture, caller, actual requests and independent SQL observations.
Its controlled-institution scope is explicit. No target/credentials/SQL observation means BLOCKED,
and generated models or bundles do not count as product execution.

## Boundaries

Unknown/infinite domains are not exhaustively proved. A partition proof does not establish
reachability, asynchronous liveness, database durability, concurrency correctness or E2E success.
Those are separate executed obligations with named oracles and evidence. Read preparation plans
as plans until actual actions and independent postcondition observations are recorded.
The legacy `discover run/resume` scheduler lacks typed pre-action capture and rejects strict
models. Use declared runners through `execute`/`run-chain`; online typed exploration remains a gap.
