# First business graph: discover → persist → review → verify

Use for whole-system/all-business/all-interaction tests or DB business trees, even without a model.
The current Agent discovers business and writes JSON; the user confirms decisions.
[Business-graph](business-graph.md) owns execution/card/RAW rules.

## 1. Find the current project and scope

Read its AGENTS, requirements, contracts, flow catalog and existing Markdown progress. Discover
the existing `.arc/verify/data.db`, graph source/version and `.arc/policy.yaml` references before
creating anything. Use `model --db ... --graph ...` to verify a known ID; the CLI has no `model list`.
If an existing ledger's ID cannot be recovered with available project references/tools, preserve
it and report that lookup gap; never reset it or silently start a parallel graph.

Plan-only creates no ledger unless requested. Read-only does not init/import/approve/plan or change
ledger/target. CLI `--db` is not a read-only connection: use existing snapshots or a consistent
artifact copy; if neither is available, report the gap. Respect explicit no-file-writes scope.
Graph-only permits local modeling/export, not business operations. Complete tests continue when ready.

## 2. Discover two independent inventories

First draft expected capabilities from user intent, requirements and confirmed contracts. Record
IDs, actors, goals, triggers, success/denial and source; unresolved semantics stay candidates.
Independently trace implementation: UI/menu/action → API/RPC/consumer → usecase → rules → durable
effects, plus timers/callbacks. Record file:line/evidence and associate via traced facts or `implements`.
Do not treat routes, generic CRUD, HTTP 200 or existing behavior as the expected business contract.

Compare expected vs implemented; preserve expected-but-unfound capabilities as candidate gaps.
No requirements source means ask the business scope, not invent a full product from industry habit.
Keep the catalog in existing project docs, referenced by the model; no second tracker or giant YAML.

## 3. Build and show the candidate before execution setup

The Agent writes a bounded v2 import JSON under the project's `.arc/verify/`, using stable IDs.
Start with exactly one `system` root, `domain`/`capability` nodes and connected acyclic `contains`
edges. Add only supported boundaries/actions/conditions/states. A capability-only candidate needs
no RULE, flow, test, account, environment or runner. Do not fabricate normative sources to validate.
Uncertain capabilities retain sources/questions in the catalog; `Risk` requires an action.
Import retains a BLOCKED schema guard; missing rules/observations stay unverified.

Resolve these paths/IDs from the target project and tool installation, then run existing commands:

```bash
arc-verify init --db "$VERIFY_DB"
arc-verify validate --file "$VERIFY_GRAPH_FILE"
arc-verify import --db "$VERIFY_DB" --file "$VERIFY_GRAPH_FILE"
arc-verify model --db "$VERIFY_DB" --graph "$VERIFY_GRAPH"
arc-verify snapshot --db "$VERIFY_DB" --graph "$VERIFY_GRAPH" \
  --assets "$VERIFY_ASSETS" --out "$VERIFY_SNAPSHOT_DIR"
```

`VERIFY_DB` is target-local `.arc/verify/data.db`; record the generated model's ID/source in existing
progress. `VERIFY_ASSETS` is the tool's built static workbench, not a guessed target-project path.
Use the existing workbench build if needed; missing assets blocks visual export, not model import.
Snapshot output must be absent/empty. For initial review omit `--env`, `--round`, `--run`: do not
invent a test context. Return DB/model/snapshot paths, capability/source/gap summary and open decisions.
Draw [readback](business-diagram.md). Export failure needs error/readback; diagrams never replace DB export.

## 4. Confirm and expand without overwriting history

Ask the user to check business completeness and only unresolved normative choices; reuse prior
explicit decisions. Catalog confirmation is not approval of invented rules. Flesh out each accepted
action: entry/actor, AND prerequisites, expected state/effects, refusal, authorized forced attempt,
every known way to satisfy failed conditions, subsequent actions and an independent oracle.
Include relevant cross-role/concurrent/idempotent/async paths; expand bounded neighborhoods only.

AI/peer inference remains RISK, implementation-only behavior UNSPECIFIED; never promote either from
HTTP success or user acceptance of a risk. Undefined expected outcome/oracle is a SPECIFICATION GAP.
Imported models are immutable: when adding/changing meaning, use a new graph ID with JSON `previous`
pointing to the stored version, the same `system` and stable node IDs. Preserve old runs and FAILs.
Validate/import the revision, then obtain catalog/RULE approval through the existing human-terminal
flow. The Agent must not pipe/PTY the approval or set approval fields directly.

## 5. Continue the authorized test request

After confirmation, follow [v2 execution](business-graph-v2.md#minimal-v2-execution): bind the actual
authorized environment, roles/accounts, fixtures, action runner and independent readback/recovery.
Prefer the project's existing client/HTTP/UI runner; command adapters may integrate browser tools.
Missing adapters/authorization/oracles block dependent execution; name what is needed and continue
independent work. Never install a server, invent credentials or claim UI E2E from an HTTP-only test.

Register fixtures/runners before round, plan bounded cards, consume `next`, execute and record RAW
under the core protocol's positive/negative/forced/recovery and contracted chain/invariant rules.
Keep first FAIL; safe readback precedes linked continuation. New discoveries stay candidates/RISK
or a confirmed new version, never rewritten expectations. Export current trace/gate/report/snapshot.

Update existing progress with scope/version, command/exit, RAW/oracle, first deviation and next action.
Model/plan-only runtime claims are NOT_RUN; missing prerequisites BLOCKED; observed RULE violations FAIL.
Report coverage, truncation and unobserved paths: a tree or single card PASS is no full-system proof.
