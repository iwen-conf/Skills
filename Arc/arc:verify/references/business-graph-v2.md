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
| `report --kind acceptance\|risk` | Acceptance excludes every risk; the risk report shows declared, reference, derived and UNSPECIFIED risks, refutations, the three-way comparison and the per-origin triage summary |
| `accept-risk` / `reject-risk` / `promote-risk` | Interactive user decisions with the user's own words |
| `risk refute --result --evidence` | Record a bounded refutation attempt bound to evidence by SHA-256; it never closes the risk |
| `reference-import` / `reference-match` | Peer-invariant catalogs (JSONC, sourced) matched by typed slots; output is only peer_reference RISK candidates |
| `mutation-report --file` | Detected/survived/inconclusive injected defects from independent run verdicts; no threshold |
| `snapshot --graph --assets [--out]` | One-off static HTML+JSON workbench export; the page never calls back, executes or approves |

Implementation evidence (`scan`, exploration), requirements (approved rules) and peer references stay
separate sources: none of them becomes a rule by itself, and only approved rules enter acceptance.

Read with [business-graph](business-graph.md), which keeps the confirmation, card and RAW rules these commands rely on.
