# Business graph → task cards → real execution

This reference extends (not replaces) flow-model, flow-matrix and the existing verification protocol.

1. Discover *expected* functionality from user intent, product requirements, UI/API inventory and domain questions; separately enumerate *implemented* entrances via code index and real UI observation.
2. Present the full business catalog to the user. The agent may propose a risk, never silently make it a rule. A user-accepted risk stays a risk; a confirmed rule is a versioned contract obligation. Review unclear semantics as SPECIFICATION GAP.
3. Create a directed property graph. Nodes: feature, action, condition, state. Edges: contains, requires, transitions, recovers, forbids. Each rule links action, AND prerequisites, positive outcome and rejection outcome; OR alternatives use graph recovery edges.
4. For every confirmed rule, generate a positive task, a negative task for each violated prerequisite, and a forced-request task. Explore condition-recovery and cross-role/multistep paths as separate tasks. Risks produce exploration reports but do not count as business FAIL or MISSING.
5. Keep task cards minimal and reference graph node/edge IDs. Expand only the selected task into an executable context. Never pre-materialize every path of a cyclic graph.
6. Before starting a run, check the allowed environment alias, existing fixture ID and current source identity. E2E means interacting with the real client. The remote test server is ONLY the target system; the graph ledger is local.
7. Save per-step RAW and observed business states. Verdict derives from an independently specified expected state, never from the implementation under test. On divergence: FAIL the original path; if the observed state is a known safe state, REPLAN as a *new* task; otherwise HALT/BLOCK. Passing a newly explored path does not rewrite the failed one.
8. The Go CLI is a short-lived client of the official `tursodb` executable, opening `.arc/verify/data.db` directly. No daemon, remote DB token, Python CLI, PostgreSQL or object store.
9. A reproducible fixture is a recipe plus code/environment identity, not necessarily a snapshot. Verify state before resuming. Do not reuse dirty shared test entities or modify a production database. A local log cannot prove undo of external side effects.
10. Gate only against user-confirmed rules, verified RAW and explicit coverage requirements. Unknown functionality and uncovered paths remain visible; never claim mathematical exhaustiveness.

Example command sequence:

    go build -o ~/.local/bin/arc-verify ./tools/arc-verify
    arc-verify init
    arc-verify validate --file /path/to/Skills/tools/arc-verify/examples/academic-v1.json
    arc-verify import --file /path/to/Skills/tools/arc-verify/examples/academic-v1.json
    arc-verify approve --graph academic-v1
    arc-verify approve --graph academic-v1 --rule R-ENROLL
    arc-verify plan --graph academic-v1
    arc-verify next --graph academic-v1
    arc-verify start --task academic-v1/R-ENROLL/POSITIVE --env test60 --commit <sha>
    arc-verify record --run <run> --observed ENROLLED --evidence .arc/verify/raw/<run>.json
    arc-verify trace --run <run>
    arc-verify gate --graph academic-v1

Approval requires an interactive human terminal; unattended agent confirmation is denied.
Evidence must bind the run ID, actual state, runner identity, oracle and exit code.
A user-owned CLI process and same-user file writes are not a cryptographic trust boundary;
high assurance requires an isolated runner or protected CI check.

Project `.arc/policy.yaml` may enable the graph gate:

    verification:
      business_graph:
        graph: academic-v1
        db: .arc/verify/data.db

Once `.arc/verify/config.jsonc` exists, the policy gate blocks unconfigured graph identity.
Preserve `verification.test_commands` and do not replace code tests with graph tests.

## Implementation inventory and reusable test data

The Go CLI also accepts `scan --file` (actual discovered action plus evidence)
and `gaps --graph` (contract action compared with most recent observation).
`NOT_FOUND` means a **possible absence** to investigate, not a confirmed product defect.
`fixtures --file` imports a fixture recipe and `fixture --id` retrieves it;
no real server data is mutated by these commands. User-approved test-data setup
and replay operations require explicit runner adapters and a state readback.
