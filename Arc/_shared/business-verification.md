# Storyline Verification & Domain Invariants

Shared verification semantics across all six Arc skills. Replaces legacy multi-version business graphs and flat task cards with one machine-checkable **Branching Storyline Model** (`*.story.yaml`).

## 1. Core Model: World Flags → Checkpoints → Concurrent Routes → Multi-Endings

1. **World Flags (`world_flags`)**: Reverse-extracted cross-domain state variables referencing the focal entity (e.g. `users.id` → `wallet_accounts.available_cents`, `settlement_orders.pending_cents`, `fulfillment_orders.delivery_status`, `merchant_orders.buyer_display_state`, async topics).
   - Never guess preconditions from memory or read implementation `if` branches to pick cases.
   - Run `arc verify scan-world <focal_entity>` (or `arc idx impact <focal_entity>`) first; every coupled entity must bind into `world_flags` or fail `arc verify lint-story --world-scan`.
2. **Checkpoints (`checkpoints`)**: Ordered, read-only state snapshots at business crossroads (e.g. `save_01_clean_veteran` → `save_02_in_transit_crossroad`).
   - Built once in topological DAG order (`arc verify checkpoint build`), then cloned in milliseconds per route (`SHARD_NS` / isolated DB template / SQLite copy / dir copy) so sequential prerequisite history never blocks parallel execution.
3. **Branching Routes (`routes`)**: Parallel trajectories loaded from a checkpoint (`load_save`), executing real sequential steps (`sequence`), checking adjacent domain side effects via **`butterfly_probes`**, and unlocking a declared **`unlocks_ending`**.
   - Routes cover the 3 verification tracks + hard endings:
     - **Clean exit** (`END_CLEAN`)
     - **Hard block** (`BLOCKS` — nonzero balance, active goods, open disputes rejected with zero mutation)
     - **Cooling / async suspense + safe escape** (`MUTATES` — in-transit settlement enters cooling state and safely settles or reverts without trapping the user)
     - **Forced cascade / anonymization** (`CASCADES` — admin/compliance purge anonymizes counterparty snapshots, preserves non-null FKs, transfers residual funds to suspense ledger with double-entry conservation)
4. **Anti-Softlock Invariant (`anti_softlock`)**: Every intermediate suspended/cooling/frozen/pending status MUST define an escape transition or explicit `anti_softlock` invariant (e.g. `account.can_withdraw == true || account.status == CANCELLED`). A state where the user can neither progress nor recover is a fatal business softlock.
5. **Blind-Test Sharding & Namespace Isolation**:
   - `arc verify plan-shards` splits routes into `regular_unit`, `spiderweb_surface`, and `fullstack_e2e` shards with **disjoint `write_scope`**, unique **`runtime_ns` (`SHARD_NS`)**, and **`blind_mask`** (hiding private business implementation files while writing tests from contracts + schema + storyline).
   - E2E browser journeys reuse checkpoint seeders instead of clicking through 30-second UI prerequisites.

## 2. Hard Completion Gate & Immutable Failure History

- `arc verify play <story.yaml> --parallel --blind` executes routes and appends immutable receipts into `.arc/arc.db` (or `.arc/verify/story.db`).
- **First FAIL is never erased**: a later PASS appends a new receipt and preserves `historical_fail_count`.
- **Anti-theater**: dry-runs or routes without real executable commands stay `BLOCKED`, never `PASS`.
- `arc verify completion <story.yaml>` exits `0` **only** when all 4 metrics hit **100%** with `softlock_count == 0` and `unresolved_fail_count == 0`:
  1. `world_flag_binding_pct == 100`
  2. `route_coverage_pct == 100`
  3. `butterfly_probe_hit_pct == 100`
  4. `ending_unlock_pct == 100`
- Static read-only visualization: `arc verify snapshot --file <story.yaml> --out <dir>` exports a self-contained UTF-8 HTML+JSON Storyline Workbench.

## 3. Cross-Skill Responsibilities

| Skill | Role in Storyline & DAG Pipeline |
|---|---|
| `arc-explore` | `arc idx skeleton` + `arc idx impact` to locate symbols and extract cross-layer entity couplings |
| `arc-plan` | Freeze contracts, validate closure & compile write-set isolated `plan-dag.yaml` (`dag_check.py`) |
| `arc-code` | Implement inside `.arc-task.json` `write_scope` and pass `gate.sh scope/hygiene/test/check` |
| `arc-verify` | `scan-world` → author `*.story.yaml` → `lint-story` → `plan-shards` → `play --parallel --blind` → `completion` |
| `arc-audit` | Read-only review of diff, security, race windows, permission boundaries and ledger invariants |
| `arc-vet` | Evaluate third-party libraries/tools with dated evidence before adoption |
