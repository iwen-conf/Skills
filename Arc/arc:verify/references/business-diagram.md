# Business verification workbench

The native static workbench is the main Arc Verify interface. It is exported from the local
ledger through `arc verify snapshot`, with versioned model, independently validated facts and
RAW. No rendering plugin, local API server or Agent lifecycle is required.

## Four synchronized regions

| Region | Content |
|---|---|
| Left | System/domain/capability/scenario tree; branches, missing rules and coverage |
| Center | Preparation dependency genealogy, cyclic state graph, or actual run overlay |
| Right | Selected node's branches; then one branch's prestate, typed guards and frozen Expected; then one run's actor/entity, Actual, assertions and evidence |
| Bottom | Actual steps, waiting, first deviation and linked history |

The single selection context is `model/environment/flow/branch/run/step/actor/entity`.
Changing a run clears account/entity selections that do not belong to it. A node can have
several branches; do not pick a branch because the actual result happens to fit its expectation.
Old runs without a frozen branch remain visibly unassociated legacy history.

## Graph meanings and layout

- Capability `contains` edges express ownership, not execution order or state transitions.
- Preparation edges express explicit `depends_on`/condition-producing actions. Use the longest
  dependency path for depth: A→D and A→B→C→D place D after C. Show upstream prerequisites,
  downstream impact, missing conditions and independent preparation groups.
- State edges express guarded transitions; retries/compensation may cycle. Condense strongly
  connected components for layout without deleting their internal transition edges or branch IDs.
- Run overlays use recorded step/action/transition identities, frozen branch and evidence status.
  Candidate edges, predicted preparation effects and model coverage must not look executed.
- Read `business_lineage` from a current Go snapshot: branch target, allowed prestate, full
  AND/OR/NOT tree, subject/attribute conditions and proven declared providers. Do not reconstruct
  genealogy from `depends_on` alone. Provider alternatives are not all-required execution steps.
- Select a single run/step/actor/action occurrence before showing observed conditions. A false
  NOT child or unused OR alternative is not automatically a blocker. Preparation readbacks use
  their own occurrence and setup RAW provenance; diagnostic failures stay BLOCKED.
- Begin with the all-model or current-flow declaration map when no branch is selected. Share
  conditions and preparation nodes only when complete semantics, prerequisites and provenance
  agree; retain every original branch/node/edge source. Shared declarations never share Actual.
- Use collapsible capability groups and a compact dependency plan, with common global levels,
  upstream/downstream navigation and a vertical narrow-screen layout. Keep the relation canvas
  available. Neither visual depth nor grouping establishes concurrency or an executable plan.
- Separate the structural longest required chain from the selected observation's blockers.
  Exclude provider alternatives and OR alternatives from required-chain calculations. A blocker
  explains the required truth value, recorded truth, path and relevant candidate providers;
  FALSE under NOT must not recommend an action merely because it establishes TRUE.
- Overlay Actual state transitions only from a scoped frozen before, captured request/action
  and independently observed after. Keep contract transitions unchanged, including retries and
  cycles. Model-external observations remain visible deviations, not new allowed contract exits.
- Clicking an Actual occurrence synchronizes run, step, actor, branch and action index together.
  Match complete entity identity and prefer explicit subject-to-entity provenance over aliases.
  Missing or contradictory evidence cannot draw a PASS edge; a blocked step cannot become green
  from an isolated comparison label. Step-wide assertions remain explicitly aggregate.
- Focus on a selected local subgraph, keep canonical IDs and expose any omitted/truncated scope.
  Group related nodes and show meaningful Chinese business names; do not flatten every domain
  into a giant task sequence. Actual package names remain source-backed implementation details.

An allowed outcome set is one explicitly declared contract, never random legal exits inferred
from repeated executions. A bad layout cannot be fixed by deleting state cycles or merging
account facts. The UI does not recompute business verdicts or rewrite evidence.

## Build, export and inspect

Build `tools/arc-verify/workbench` with the versions in its manifest/lockfile. Export using
`arc verify snapshot --graph GRAPH --assets BUILD --out DIRECTORY`, adding explicit environment,
round or run filters when needed. Without `--run`, the export includes the latest 32 runs in
that context, reports total/truncated history, and keeps the catalog for navigation.

The directory contains `index.html`, local assets and `snapshot.json`. It is read-only, has a
network-denying content security policy, and exposes no execute/approve/retry controls. A new
export never overwrites a previous nonempty snapshot. Evidence failures appear as BLOCKED
with the reason; raw-file presence or stored PASS text is not enough.
Without an execution environment, import the sourced model and export without run/env/round;
retain unapproved/NOT_RUN states. The [pay_gateway example](../../../tools/arc-verify/examples/pay-gateway/README.md)
provides this command sequence. It does not execute or approve payments.

Run frontend tests and the actual production build. Inspect desktop and narrow layouts plus
branch/run/step/actor/entity changes. Use a snapshot exported by the Go CLI for integration
checks. Synthetic tool fixtures must say synthetic; they are not pay_gateway acceptance.
No screenshot or geometry check proves the business system passed.

## Optional presentation export

The existing [arch-diagram plugin](https://github.com/bassnova42/arch-diagram-skill) can still
export a presentation graphic when requested. Read its installed skill for that substep and
use the exact imported model/snapshot. Preserve source IDs, graph hash, branch meanings,
missing evidence and original historical files. It is not the workbench's execution or layout
engine, and unavailable plugin tooling does not block native snapshots or independent tests.
