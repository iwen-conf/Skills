# Write-Set Isolated DAG Planning (`plan-dag.yaml`)

Replaces serial Markdown waterfalls for multi-module or multi-agent work with a deterministic DAG whose parallel waves have mathematically disjoint `write_scope`.

## 1. Three Hard Rules

1. **Contract Freeze First**: Single-threadedly author/update API contracts (`api/openapi.yaml`, `*.proto`, `docs/02-API记录/`) and SQL migrations first, then list them under `frozen_contracts`. Any worker editing a frozen file fails `gate.sh scope`.
2. **Disjoint `write_scope` Per Parallel Wave**: Two nodes in the same topological wave must have zero overlapping `write_scope` globs. If two tasks touch the same file, add an explicit `depends_on` edge or extract the shared interface into `frozen_contracts`.
3. **Minimal Context + Physical Worktree Isolation**: Dispatch waves via `git worktree` with `.arc-task.json` containing only `frozen_contracts`, `read_scope`, `write_scope`, and `verify_cmd`.

## 2. Manifest Format (`plan-dag.yaml`)

```yaml
frozen_contracts:
  - api/openapi.yaml
  - db/migrations/004_account_cancel.sql

nodes:
  - id: wallet-settlement-guard
    runner: codex
    depends_on: []
    read_scope: [api/openapi.yaml, db/migrations/004_account_cancel.sql, internal/wallet/**]
    write_scope: [internal/wallet/cancel_guard.go, internal/wallet/cancel_guard_test.go]
    verify_cmd: "go test ./internal/wallet/... -run TestCancelPrecheck -race"

  - id: order-fulfillment-guard
    runner: claude
    depends_on: []
    read_scope: [api/openapi.yaml, db/migrations/004_account_cancel.sql, internal/order/**]
    write_scope: [internal/order/cancel_guard.go, internal/order/cancel_guard_test.go]
    verify_cmd: "go test ./internal/order/... -run TestInFlightGoodsBlock -race"

  - id: cancellation-orchestrator
    runner: grok
    depends_on: [wallet-settlement-guard, order-fulfillment-guard]
    read_scope: [api/openapi.yaml, internal/wallet/cancel_guard.go, internal/order/cancel_guard.go]
    write_scope: [internal/account/cancel/**]
    verify_cmd: "go test ./internal/account/cancel/... -race"
```

## 3. Commands

```bash
# Validate cycles, frozen contract locks, disjoint write_scope per wave, and verify_cmd (top 5 errors JSON)
python3 <skill-dir>/scripts/dag/dag_check.py plan-dag.yaml --max-errors 5

# Prepare isolated git worktrees + .arc-task.json for wave 0
python3 <skill-dir>/scripts/dag/dag_dispatch.py prepare plan-dag.yaml --wave 0

# Verify a worktree touched only its write_scope and zero frozen contracts
python3 <skill-dir>/scripts/dag/dag_dispatch.py verify-scope --worktree .arc/worktrees/wallet-settlement-guard
```
