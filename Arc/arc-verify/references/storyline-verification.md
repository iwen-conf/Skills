# Storyline Verification Guide (`*.story.yaml`)

Single canonical verification specification combining **domain trees (points)**, **spiderweb cross-entity couplings (surfaces)**, **branching checkpoints & multi-endings**, **blind testing**, and **concurrent shard execution**.

## 1. Five-Step Closed Loop

1. **Scan World Flags** (`arc verify scan-world <focal_entity> --format yaml`):
   - Deterministically extracts every SQL table/FK/status/amount column, ORM model field, and async MQ topic referencing `<focal_entity>` (e.g. `users.id`).
   - Every coupled entity must be mapped in `world_flags` — never guess preconditions from memory or copy `if` branches from implementation code.
2. **Author Branching Storyline** (`<feature>.story.yaml`):
   - `world_flags`: all coupled entity states from step 1.
   - `checkpoints`: ordered read-only savepoints at business crossroads (`parent: root` or earlier `save_id`), built once via `dir` / `sqlite` / `postgres` (`CREATE DATABASE ... TEMPLATE ...`) and cloned per route in milliseconds.
   - `routes`: parallel trajectories loaded from a checkpoint (`load_save`), with unique `runtime_ns` (`SHARD_NS`), disjoint `write_scope`, sequential `sequence` steps, adjacent-domain `butterfly_probes`, `anti_softlock` guards for every intermediate suspended/cooling/frozen status, and `unlocks_ending`.
   - `required_endings`: must enumerate all 4 outcome tracks when applicable:
     - Clean exit (`END_CLEAN`)
     - Hard block (`BLOCKS` — nonzero balance / in-flight goods / open dispute)
     - Cooling / async suspense + safe escape (`MUTATES` — zero softlock trap)
     - Forced cascade / anonymization (`CASCADES` — anonymized snapshots, zero null-FK crash, balanced suspense ledger)
   - `blind_mask`: private implementation files hidden from test-authoring workers so tests verify contracts & domain invariants instead of stubbing around implementation bugs.
3. **Static Storyline Lint** (`arc verify lint-story <file> --world-scan --max-errors 5`):
   - Rejects unbound world flags, checkpoint cycles, softlock dead-ends, unreachable endings, colliding `runtime_ns`, or overlapping `write_scope`.
4. **Compile Shards & Play Concurrently**:
   - `arc verify plan-shards <file> --out verify-shards.yaml` splits routes across `regular_unit`, `spiderweb_surface`, and `fullstack_e2e` shards.
   - `arc verify play <file> --parallel --blind` builds checkpoints, clones per-route namespaces (`SHARD_NS`), executes real commands, checks `butterfly_probes` and `anti_softlock`, and appends immutable receipts to `.arc/arc.db` (never erasing historical FAILs).
5. **100% Completion Gate & Static Workbench**:
   - `arc verify completion <file> --world-scan` exits `0` **only** when `world_flag_binding_pct == 100`, `route_coverage_pct == 100`, `butterfly_probe_hit_pct == 100`, `ending_unlock_pct == 100`, `softlock_count == 0`, and `unresolved_fail_count == 0`.
   - `arc verify snapshot --file <file> --out <dir>` exports a self-contained read-only UTF-8 HTML+JSON Storyline Flowchart Workbench.

## 2. Example Storyline

See [ecommerce-cancel.story.yaml](../../../tools/arc-verify/examples/ecommerce-cancel.story.yaml) for a complete runnable 4-ending e-commerce user cancellation storyline.
