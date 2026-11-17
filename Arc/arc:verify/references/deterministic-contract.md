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

An integer attribute may declare `encoding: "json_i64"` with its `source_ref` when the confirmed
transport contract requires a signed 64-bit JSON integer token. The captured token is checked
before numeric comparison: `100.0`, `1e2`, strings and overflow cannot be normalized into `100`.
Missing or invalid observed encodings required by branch selection block dispatch; a valid but uncovered value remains a
specification gap. The same encoding applies to finite domains and preparation effects. Ordinary
`integer` attributes retain mathematical semantics, so this stricter protocol is a versioned
model change rather than an implicit reinterpretation of historical observations. Declaring an
unused attribute is not a whole-request schema validator.

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

## Bounded exploration from observed state

`arc verify campaign` uses the strict execution protocol for an explicit set of registered
Runner input variants. Declare the model, flow, environment, round, source/config, account and
business fixture in a request file, with `variants: [{id, branch, runner}]` and finite
`budget: {max_actions, max_states, deadline_ms}`. A variant is a test obligation; multiple input
variants may use the same confirmed branch. Inputs are captured from the actual resolved request.

```bash
arc verify campaign run --file campaign.json
arc verify campaign status --id CAMPAIGN
arc verify campaign list --graph MODEL --env ENV --round ROUND
arc verify campaign resume --id CAMPAIGN
arc verify campaign abort --id CAMPAIGN --reason "named interruption"
```

Register fixtures and Runners before creating the bound round; see the
[complete request example](../../../tools/arc-verify/README.md#deterministic-business-verification).
The request neither prepares a fixture by declaration nor approves its rules.

The current bounds are 1–64 explicit variants, 1–1,024 registered action attempts,
1–10,000 observed contexts and a deadline of 1 ms–1 hour. A registered command adapter may
make multiple internal requests; its attempt count is not a per-network-request quota.

The scheduler independently probes current state, records each candidate's eligibility and
preparation plan, and dispatches through `execute`. Each dispatch reserves a unique child run
ID, frozen identity and action budget before work begins. Preparation and target execution
retain their own checked evidence. Follow-up eligibility is reobserved and respects the verified
outcome's explicit next-operation restriction. Expected poststates do not populate the frontier.

`COMPLETED_IN_SCOPE` means that all selected variants have verified passing visits. It does not
mean all model branches, parameter combinations or business paths were exercised. Gaps, conflicts,
observed deviations, unavailable conditions and budgets have distinct stop reasons. Static
setup actions, multi-action Runners and unsupported observation protocols are rejected by this
bounded scheduler; ordinary registered execution retains its existing supported protocols.

The total action budget includes target, preparation and restoration attempts. Before dispatch,
the scheduler conservatively reserves the registered worst-case action count. Unknown results
retain that reservation. Resume reconciles completed child runs against their original facts;
it never blindly reissues an uncertain dispatch. A failed visit and its first divergence remain
historical facts. Ordinary `retry` still cannot reopen a passing task; a campaign's next visit
requires its own durable dispatch authorization.

The current state budget counts observed candidate contexts and completed child contexts. Keys
retain actual input tokens, actors, entities, source/environment and history; the scheduler does
not merge different runs using an unproved equivalence. Automatic preparation feasibility uses
declared attribute effects. It does not invent a lifecycle-state prediction that those effects
do not declare. More general parameter generation, lifecycle preparation search and equivalence
reduction need additional explicit model contracts.

`last_actual` is the most recent **target operation's** observed poststate. A subsequent restore
may change the fixture; its independently observed recovery state stays separate, and the next
candidate must be freshly read. A restore is an isolation boundary. Continuous entity journeys
use checkpoint and new independent bindings, never a copied cookie/session/variable map.

## Additive schema and real-project adapter

For an already active ledger, `arc verify init` applies additive tables transactionally. It does
not delete or rewrite old models, runs or RAW. `business_contracts`, `business_branches`,
`preparation_actions` and `business_expectation_captures` make the immutable contract and durable
pre-action captures queryable. `business_campaigns`, `business_campaign_events` and
`business_campaign_dispatches` retain scoped exploration state, event history and child-run
dispatches. The old ledger activation remains `arc db migrate --confirm-offline`.

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
models. Use declared Runners through `execute`/`run-chain`, or the explicit bounded variants
supported by `campaign`; none of these operations discovers or approves new business rules.
