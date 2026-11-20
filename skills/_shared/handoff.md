# State Ledger, Wave Compiler & Handoff Protocol

Connects `arc` (`0.5.x`), `arc idx`, and `arc-wave` so **Claude Code**, **Codex**, and **Grok Build** collaborate without static YAML ceremony.

## 1. Code Intelligence Preflight

```bash
arc capabilities --json
arc idx doctor --format json
```
Rebuild once via `arc idx index --quiet` only if `healthy: false` or `index_stale: true`. Query shallowest-first: `arc idx symbol` -> `arc idx search` / callers -> 8-surface impact list.

## 2. Real `arc` Task State Machine & CAS

`arc_tasks` transitions strictly follow optimistic concurrency (`--expected-version <N>`):

```text
draft -> ready -> running -> verifying -> done
            ^        |           |          |
            +--- blocked <-------+     reopened -> ready / running
```

- Claim / start: `arc task transition --id <ID> --to running --expected-version <N>`
- Post-edit sync: `arc update --changed-only` and `arc change collect --task <ID>`
- Ready for independent check: `arc task transition --id <ID> --to verifying --expected-version <N+1>`
- Finalize: `arc index flush --repo .` -> `arc task complete --id <ID> --expected-version <N+2>`

## 3. Code + DB Wave Compilation (`arc-wave`)

Never ask an LLM to hand-compute pairwise-disjoint `plan-dag.yaml`:
1. **Wave 0 (Contract Freeze)**: Lock `openapi.yaml`, `*.proto`, and SQL migrations single-threaded.
2. **Derive Waves**: Run `arc-wave plan <anchor1> <anchor2> ...` to expand symbol/import/test `write_scope` from the live codebase and `.arc/arc.db`:
   - `>= 40%` file overlap -> coalesced into one task unit.
   - Partial overlap -> serialized provider-before-caller across consecutive waves; hub registries (`router.go`, `main.go`, `package.json`, `go.mod`) deferred to a thin wiring step.
   - `0%` overlap -> scheduled in parallel `git worktree` directories.
3. **Mid-Task Agent Switch**: Run `arc-wave handoff --task <ID> --agent <runtime> --note "..."` on exit, and `arc-wave status` on entry in the next agent.
