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
