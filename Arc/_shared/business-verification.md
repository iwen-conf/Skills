# Deterministic business verification

This is the shared semantic contract for all six Arc skills. Read it when modeling,
changing, executing or reviewing business behavior. Local unrelated tests do not require a graph.

## Three separate models

| Model | Meaning | Permitted structure |
|---|---|---|
| Capability tree | System → domain → capability → scenario | Acyclic ownership hierarchy; never a transition oracle |
| Preparation dependency genealogy | Which actions or independent observations establish each required condition | DAG with explicit dependencies, longest dependency depth, independent preparation layers, blockers and upstream/downstream tracing |
| Business state machine | Legal behavior of identified actors and entities under confirmed conditions | Cycles, retries, compensation, external events, concurrent interleavings and recovery |

ETA informs dependency organization, layers, focus and history. Its task completion status does
not prove business correctness. A state-machine retry loop is not a dependency cycle.

## Branch identity and meaning

`(complete relevant PreState, Guard, Actor, Action) -> ExpectedOutcome`.

Each branch binds model/version, flow, action, transition, RULE, source, capability and test
obligations. Conditions identify their subjects, typed attributes, operators and values; use
AND/OR/NOT, ranges, sets and relations when required. Do not collapse a multi-subject condition
into one unexplained boolean. Model aliases bind to actual account/entity identities at runtime.

For a complete input, confirmed guards must yield exactly one branch. Overlap, even when two
rules happen to produce the same output, is an explicit ambiguity; reject it as `MODEL_ERROR`.
No match is `SPECIFICATION_GAP`. Unknown observations, missing binding or unconfirmed semantics
are `BLOCKED`. None of these outcomes is product PASS. Priority is not an implicit tie breaker.

One branch freezes its response, resulting state, persistence, events, side effects, prohibited
effects, invariants and permitted next actions before the real operation. Source-backed
`allowed_outcomes` may express true contract-permitted nondeterminism, with common invariants;
ordinary exceptions and external events belong to their own conditioned transitions.

The runtime must never use an actual response to rewrite Expected or pick a different branch.
Unknown or out-of-model actual states remain observations, not newly approved expectations.

## Preparation and execution

1. Observe the actor, bound entities, state, relevant history and external conditions.
2. Compare `ObservedPreState` with the required `ExpectedPreState` at attribute level.
3. Plan missing conditions through declared preparation actions and dependency layers. Predicted
   effects are planning hints only. Existing satisfied conditions require fresh observations.
4. Execute authorized preparation through real entries, preserve its evidence, and independently
   read back each postcondition. Preparation failure blocks the target action.
5. Match exactly one branch from the observed prestate and freeze its contract/version/hash.
6. Execute the real action, retain request occurrence and actor/entity/session references.
7. Independently observe actual response, poststate, writes, messages and external effects.
8. Compare each frozen obligation, retain the first deviation and continue only from observed
   state. Missing/damaged evidence is `BLOCKED`; an observed contract violation is `FAIL`.

An async acceptance ends in its documented pending state. Success/failure events and timeout
observations are later inputs. Concurrent tests record actors, observed versions and controlled
or permitted interleavings; do not claim an order that the oracle cannot observe.

## Facts, coverage and workbench

Use one project ledger: `.arc/arc.db` after activation; preserve legacy history during migration.
The immutable model, frozen expectations, RAW, `step_facts`, `step_assertions` and
`run_entity_bindings` form the evidence chain. Identity includes run, step, actor, account/entity,
oracle and observation phase. Two accounts may use the same oracle name without sharing facts.
An archived RAW fallback is valid only after digest, identity and ledger projection checks.

Model analysis and real execution are separate. Finite-domain coverage states its actions,
states, relevant attribute domains, budget and checked scope. `PROVEN_IN_SCOPE` proves that
partition only. Unbounded values, omitted states, unconfirmed rules and exhausted budgets stay
visible. Endpoint coverage, node count and a green diagram never prove business completeness.
Old models without `business_contract` remain `LEGACY_UNDECLARED` for this stronger contract.

Bounded online exploration uses `arc verify campaign` with explicit registered input variants,
fresh scoped observations and the same prepare/freeze/execute/compare path. Its ledger retains
candidate eligibility, dispatch reservations, child runs, actual history and stop reasons.
`COMPLETED_IN_SCOPE` covers the selected variants only; it cannot approve missing rules or prove
full business reachability. Resume must reconcile original child-run evidence and retain unknown
dispatches without replay. The legacy `discover` executor remains unavailable to strict models.

The native snapshot workbench has capability, dependency/state/run, branch/facts and trace
regions, with one `model/environment/flow/branch/run/step/actor/entity` selection context.
Select a branch before showing its unique expected outcome; select a run before showing Actual.
Never merge account observations or attach a historical run to a branch by guessing its node.
Static exports are read-only; all writes use the existing CLI.

## Responsibilities and acceptance

| Skill | Required output |
|---|---|
| explore | Independently traced entries, state attributes, entities, writes, events and sources; observations do not approve rules |
| plan | Confirmed branch semantics, coverage scope and explicit missing decisions; reuse existing user confirmation |
| code | Kernel, adapters, persistence and UI implementing the frozen contract; preserve old evidence |
| verify | Independent pre/post observations, executed preparation, per-branch verdicts and counterexamples |
| audit | Check overlap, gaps, incorrect expected/actual coupling, isolation, freshness and evidence integrity |
| vet | Assess tools against dependency DAG versus cyclic state-machine needs; no mandatory rendering plugin |

D01–D10 cover conflicting rules, state-dependent branches, overlapping guards, uncovered
combinations, wrong actual prestate, frozen-expectation deviation, async event inputs, entity
continuity, multi-account isolation and separation of preparation DAGs from state cycles.
Tool tests or synthetic fixtures do not qualify pay_gateway; real project acceptance also needs
its isolated environment, authorized accounts, actual entries and independent persistence/
external-effect observations. Unavailable dependencies are named blockers, never simulated PASS.

## Make repeated corrections executable

Following [Lauren Tan's engineering argument](https://ai.engineer/code/2026/talks/lauren-tan),
prefer a better API, architectural boundary, type check or regression over another prose warning.
Arc enforces branch uniqueness, pre-action captures, scoped oracle identity and verified evidence
in code. `TestBusinessKernelHasNoAmbientIO` guards direct IO imports in the decision kernel;
`FuzzBusinessContractUniqueFreezeDoesNotFollowActual` exercises exact integer partitions,
branch-order independence and immutable Expected. Neither is a whole-program formal proof.

The capability tree is the shared user-facing feature vocabulary; registered CLI adapters are
reusable drivers. Reviewers inspect their scoped model, actual assertions and reproducible traces.
Keep documentation for decisions, sources and unresolved judgment. Do not globally ban comments
or React effects by copying another product's local framework policy. TLA+, Lean and UI property
testing can add separate evidence, but are not implemented by naming them in a skill. Expand
automated exploration only after its ordinary observe/freeze/execute/check loop is trustworthy.
