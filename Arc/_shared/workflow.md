# Cross-Skill Workflow: Write-Set DAG & Storyline Verification

Connects all six Arc skills with machine-checkable handoffs instead of serial Markdown prose chains.

## 1. End-to-End Pipeline

```text
arc-explore (skeleton + impact)
   ↓
arc-plan (freeze contracts + closure gate + plan-dag.yaml)
   ↓  [parallel git worktrees — disjoint write_scope per wave]
arc-code (implement inside .arc-task.json write_scope + atomic gate.sh)
   ↓
arc-verify (scan-world → *.story.yaml → lint-story → plan-shards → play --parallel --blind → completion 100%)
   ↓
arc-audit (read-only diff / AppSec / race & ledger invariant review)
```

## 2. Write-Set Isolated DAG Handoff (`arc-plan` → `arc-code`)

1. Single-threaded **Contract Freeze**: generate or update API contracts (`api/openapi.yaml`, `*.proto`, `docs/02-API记录/`) and SQL migrations first; list them under `frozen_contracts` in `plan-dag.yaml`.
2. Validate DAG: `python3 Arc/arc-plan/scripts/dag/dag_check.py plan-dag.yaml` enforces:
   - Acyclic dependencies & computed parallel waves
   - Zero overlap with `frozen_contracts`
   - **Pairwise disjoint `write_scope`** within every parallel wave
   - Non-empty `verify_cmd` per node
3. Dispatch wave: `python3 Arc/arc-plan/scripts/dag/dag_dispatch.py prepare plan-dag.yaml --wave 0` creates isolated `git worktree` directories and writes `.arc-task.json` into each worker's root.
4. Each `arc-code` worker runs `gate.sh scope --json` + `gate.sh hygiene --json` + `gate.sh test --json` (or `gate.sh check`) before merging back.

## 3. Closure Gate (`arc-plan` model mode → `arc-code`)

When modeling complex lifecycles in `03-closure-spec.yaml`, run:
```bash
python3 Arc/arc-plan/scripts/closure/check_closure.py check <spec> --json --max-errors 5
python3 Arc/arc-plan/scripts/closure/check_closure.py gate <spec>
```
Gate PASS binds both spec SHA-256 and checker SHA-256; it proves model consistency, not runtime behavior.

## 4. Storyline Verification Handoff (`arc-code` ↔ `arc-verify` ↔ `arc-audit`)

- `arc-verify` runs `arc verify scan-world <entity>` → authors `<feature>.story.yaml` → validates with `arc verify lint-story <file> --world-scan` → compiles parallel blind shards with `arc verify plan-shards` → executes with `arc verify play <file> --parallel --blind` → gates release on `arc verify completion <file>` (100% world/route/probe/ending, 0 softlocks).
- Failing routes hand a repair packet (failing route, checkpoint, probe diff, `file:line`) back to `arc-code` while keeping the original FAIL immutable in `.arc/arc.db`.
- `arc-audit` reviews diffs, permission boundaries, concurrency races and ledger conservation read-only without mutating code or ledger state.
