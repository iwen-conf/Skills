# Business graph v2: model, equivalence and runtime commands

v2 models (`schema_version: 2`) add typed views (system, domain, capability, boundary, role, data,
contract, deployment) over the legacy action/condition/state core; v1 graphs reject every v2 field.
Optional v2 fields: `history[]` (path facts that change later behavior; `set_by`/`cleared_by`/`enables`
transitions, `measurement` for runtime), `equivalence.ignore` (measurements that must not split
states, each with a source) and `reference_bindings` (resolve, cover or rule out a peer reference).
States are merged only when flow, state, role, principal alias, relevant conditions, declared history
and runtime measurements all match and are observed; an unobserved part never merges.

| Command | Use |
|---|---|
| `explore --flow` | Bounded model exploration; equivalent states close as `merges[]`, budgets still truncate |
| `run-chain --round --max-steps --deadline` | Run the known path (READY cards with one current runner) through `execute`; stops with exit 10 divergence, 11 unknown state, 12 precondition, 13 new branch, 14 budget, 15 blocked, plus a minimal packet |
| `derive --run [--budget]` | Derive obligations from a failed run's observed state (see DERIVED) |
| `discover record/list/triage` | Register exploration observations; behavior outside the model becomes UNSPECIFIED (never PASS/FAIL); the user triages it in a real terminal |
| `discover run/resume/status/abort` | Bounded test execution with atomic campaign claim, fencing and independent Oracle; use deterministic/heuristic modes. Legacy `--mode ai --planner FILE` is a **scripted test fixture only**, never a model service or AI discovery proof. No model endpoint, settings or API key |
| `report --kind acceptance\|risk` | Acceptance excludes every risk; the risk report shows declared, reference, derived and UNSPECIFIED risks, refutations, the three-way comparison and the per-origin triage summary |
| `accept-risk` / `reject-risk` / `promote-risk` | Interactive user decisions with the user's own words |
| `risk refute --result --evidence` | Record a bounded refutation attempt bound to evidence by SHA-256; it never closes the risk |
| `reference-import` / `reference-match` | Peer-invariant catalogs (JSONC, sourced) matched by typed slots; output is only peer_reference RISK candidates |
| `mutation-report --file` | Detected/survived/inconclusive injected defects from independent run verdicts; no threshold |
| `snapshot --graph --assets [--out]` | One-off static HTML+JSON workbench export; the page never calls back, executes or approves |

Implementation evidence (`scan`, exploration), requirements (approved rules) and peer references stay
separate sources: none of them becomes a rule by itself, and only approved rules enter acceptance.

Read with [business-graph](business-graph.md), which keeps the confirmation, card and RAW rules these commands rely on.

Switching Grok/Codex/Claude Code requires only [当前状态与需求](../../../docs/03-执行任务/arc-verify-graph-runtime/当前状态与需求.md) and the target repository documents. There is no native Agent handoff, ACP protocol or new CLI command; CLI checkpoint is only for safe **test recovery**.

## Capability-first exploration

ARC Verify constrains the existing Grok, Codex, Claude Code or other coding Agent.
Do not create an Agent, ACP/native handoff, model service, HTTP model client,
API key, or extra state store.

1. Read requirements, confirmed RULEs, FLOWs, existing Markdown progress and HEAD.
   Record business goals before listing endpoints.
2. List capabilities as system → capability → meaningful boundary. Each has an
   actor, goal, trigger, success/denial state and source. Reuse capability IDs.
3. Trace implementation heuristically: UI/API/RPC/consumer → usecase → rule and
   persisted side effect. Go/OpenAPI scan is a candidate source; a route is not a
   capability. Link actions only via Graph `implements` or traced evidence.
4. Focus on justified boundaries (role, precondition, state, repeat/concurrency,
   timeout/recovery, cross-capability). Do not generate a matrix per HTTP path.
5. Verify with the existing runner and independent Oracle when authorized. Keep
   RISK/UNSPECIFIED, RULE FAIL and missing evidence separate. A scan is not proof.
6. Update the same Markdown: goal, capability, source location, command/exit/RAW/
   Oracle, unsolved problem, status and the next executable action.

An adapter entry proves a declared test entry only, not that the business action
was observed or passed. Do not treat HTTP 200 as business identity. Keep crash
recovery and Oracle; do not delete them to simplify. Legacy `--mode ai --planner FILE`
is a scripted fixture; normal exploration uses `heuristic` / `deterministic`.
