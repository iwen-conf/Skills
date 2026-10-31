# ARC Verify (local graph test ledger, Go)

Short-lived Go CLI using the **official in-process `tursogo` driver** as its database engine.
It opens `.arc/verify/data.db` inside the tested repository and exits after every command.
No Python runtime, no Turso Cloud URL/auth token, no daemon and no background processes.
Turso's `tursodb` is distinct from the cloud-management `turso` CLI.

Build from this Skills repository:

```bash
go -C tools/arc-verify test ./...
go -C tools/arc-verify build -o ~/.local/bin/arc-verify .
```
The same binary runs from the **target project** root. Keep the target project's `.arc/verify/` private and gitignored.
The official `tursodb` shell is optional and intended only for manually inspecting the local database.

Project lifecycle:

    arc-verify init
    arc-verify validate --file /path/to/Skills/tools/arc-verify/examples/academic-v1.json
    arc-verify import --file /path/to/Skills/tools/arc-verify/examples/academic-v1.json
    arc-verify approve --graph academic-v1           # USER terminal approval
    arc-verify approve --graph academic-v1 --rule R-ENROLL
    arc-verify plan --graph academic-v1
    arc-verify next --graph academic-v1
    arc-verify fixtures --file /path/to/Skills/tools/arc-verify/examples/fixtures.json
    arc-verify start --task academic-v1/R-ENROLL/POSITIVE --env test60 --commit HEAD_SHA --account student-01 --fixture course-open
    arc-verify step --run RUN_ID --expected ENROLLED --observed ENROLLED --evidence .arc/verify/raw/RUN_ID-step-1.json
    arc-verify finish --run RUN_ID
    # record remains a shorthand for a single step plus finish
    arc-verify trace --run RUN_ID
    arc-verify risk --graph academic-v1
    arc-verify gate --graph academic-v1

An unconfirmed rule cannot produce required test tasks; a risk never becomes a rule
implicitly. Import never overwrites a graph version. RAW must contain matching `identity`, `run`,
`observed`, nonempty `oracle`, nonempty `runner` and successful `exit_code`.
The CLI hashes RAW locally, and tests derive expected states from the frozen graph.

A minimal `.arc/verify/config.jsonc`:

    {
      // The tested system runs remotely; graph database stays local.
      "environments": {
        "test60": {"approved": true, "url": "https://YOUR-TEST-HOST"}
      },
      "accounts": {
        "student": {"role": "STUDENT", "secretRef": "env://TEST_STUDENT_PASSWORD"}
      }
    }

Do not store passwords/tokens in config, graph, committed evidence, or RAW.
Evidence hash alone is not proof of an independent observer; use a separate trusted
runner/CI for strong authenticity. This CLI does not implement browser automation
or automatic database snapshot restoration yet; those are separate guarded adapters.

Current implementation is a **core test ledger**, not a full E2E runner. It does not
discover hidden UI features, execute browsers or restore target-system state automatically;
those adapters are tracked in `docs/03-执行任务/arc-verify-graph-runtime.md`.
Native Go tests exercise the official embedded Turso driver as well as model/JSONC checks.
Remote-system E2E needs its own executed client journey and independent business evidence.

Fixture/implementation evidence commands (status is never inferred from missing UI alone):

    arc-verify fixtures --file /path/to/Skills/tools/arc-verify/examples/fixtures.json
    arc-verify fixture --id student-01
    arc-verify scan --file /path/to/Skills/tools/arc-verify/examples/implementation-scan.json
    arc-verify gaps --graph academic-v1

Fixtures retain logical IDs, intended states, role metadata and only secret references
(e.g. `env://TEST_STUDENT_PASSWORD`), not passwords. An imported fixture is a
recipe/reference; no automatic server-side restoration is claimed. Implementation
observations require an evidence file and are append-only; a `NOT_FOUND` observation
remains a **candidate missing-functionality risk**, never an automatic product FAIL.

## Step-level trace and freeze rules

Each RAW JSON object needs `run`, sequential `step`, `expected`, `observed`,
`runner`, `oracle`, and explicit integer `exit_code`. Duplicate JSON fields, including
case-fold aliases, and null/omitted observations are rejected. The CLI independently looks up
the frozen graph expectation, checks the identities and hashes the RAW. A
negative/forced outcome additionally needs independent `before` and `after`
readbacks of the contract's forbidden-effect scope to be identical plus explicit `denied: true`.
Permitted audit/failure records are observed separately. Missing evidence is rejected;
runner failure is BLOCKED/HALT and wrong setup is BLOCKED/RESET. A valid observation
of a wrong state, non-denial or forbidden write creates a permanent FAIL at
that step. Only an explicitly safe, known observed state can be marked
`REPLAN`; it never rewrites the original FAIL. `finish` requires at least
one PASS step, valid unchanged RAW files, and a final observation matching the
task's frozen expected state. The gate reconstructs the mandatory task set
and rechecks frozen task meaning and RAW content/hash; counts alone cannot grant PASS.
`requires` edges and RULE conditions must agree in both directions. Colliding task IDs
and conflicting stored obligations are rejected rather than silently ignored.


`arc-verify trace --run RUN_ID` returns an ordered execution trace with the **first diverging step**, latest observed state and next decision. A failed task cannot be overwritten by a successful diagnostic branch. `arc-verify recover --graph GRAPH --condition CONDITION` explores bounded AND prerequisites and OR recovery actions and marks unconfirmed actions unresolved rather than inventing rules.

## Resuming after failure or a lost runner

    arc-verify trace --run R123
    arc-verify abort --run R123 --reason "browser crashed; state unknown"
    arc-verify context --run R123 --phase fixture --commit CURRENT_HEAD
    # Independently reconstruct and observe the authorized fixture, recording checkpoint RAW.
    arc-verify retry --task "academic-v1/R-ENROLL/POSITIVE" --reason "recreated baseline" --commit CURRENT_HEAD --evidence .arc/verify/raw/R123-checkpoint.json
    arc-verify next --graph academic-v1

An abort saves BLOCKED plus an immutable event. A retry creates a new
READY task attempt without deleting old FAIL/BLOCKED runs or RAW. This is not
database snapshot restoration, remote service rollback or permission to reuse
uncertain shared state.

Start binds current HEAD plus all nonignored staged/unstaged/untracked source changes, approved
environment URL/config hash, frozen graph, exact role/principal/account and fixture recipe hashes.
RAW `identity` must match all of them; the runner independently observes served build and actual
identity/state. Expected context alone is no proof that a server runs the candidate. A failed or
aborted run retains account/fixture locks until an independent `phase: fixture`, `step: 0`
checkpoint verifies the current recipe state and task conditions with exit 0 and safe continuation.
Retry atomically records that checkpoint, releases the old locks and reopens the task; start
revalidates the checkpoint. Changed recipes need a new fixture ID; repeated identical imports
return `fixtures_added: 0`.

For a new source/config/environment, run `prepare --graph academic-v1 --env test60 --commit
CURRENT_HEAD` from the target root, or pass its `--root`/`--config`. Stale PASS tasks become READY
while old PASS/FAIL runs remain history. RUNNING attempts block prepare; failed attempts still
require recovery. Prepare returns NOT_RUN because it performs no business actions. Upgrade
existing ledgers with `init` before using the new binding/checkpoint tables.

## Static snapshot (read-only workbench)

There is no local workbench service. `snapshot` exports one existing ledger context once and exits:

    arc-verify snapshot --graph course-v2 [--env independent] [--round ROUND_ID] [--run RUN_A,RUN_B] \
      --assets tools/arc-verify/workbench/dist [--out DIR]

It writes `snapshot.json` and an `index.html` that embeds the same JSON
(`<script type="application/json" id="arc-snapshot">`) behind a CSP of `default-src 'none'`, so the
page opens from `file://` without network requests and has no write actions. Rules:

- v2 models only; the graph, environment, round and runs must already exist in the ledger.
- Every RAW of a selected run is re-verified at export; one that fails is written as a failure entry
  (`phase`, `step`, `error`), never dropped or shown as valid.
- `--out` must not exist or be empty (default `.arc/verify/snapshots/<graph>/<UTC>/`); snapshots are
  never overwritten. Output is staged and renamed, so a refused export leaves nothing behind.
- A JSON body over 64 MiB fails and lists its parts by size. A build that still references `/api/`
  is refused.
- `workbench/src/types.ts` mirrors the Go DTOs; `TestSnapshotTypesMatchFrontend` fails on drift.

## v2 runtime commands

Details and semantics: `Arc/arc:verify/references/business-graph-v2.md`.

    arc-verify run-chain --graph G --env E --round Q --root . --config C --commit SHA \
      --account A --fixture F --max-steps 16 --deadline 10m      # exit 0 done, 10-15 = stop reason
    arc-verify derive --run RUN [--budget 16]                     # obligations from a failed run
    arc-verify discover record --file exploration.json            # UNSPECIFIED, never PASS/FAIL
    arc-verify discover triage --graph G --id ITEM --as missing_requirement   # USER terminal
    arc-verify report --graph G --kind acceptance|risk
    arc-verify reject-risk --graph G --risk ID                    # USER terminal
    arc-verify risk refute --graph G --risk ID --result refuted --evidence FILE
    arc-verify reference-import --file examples/references/enrollment.jsonc
    arc-verify reference-match --graph G
    arc-verify mutation-report --file mutations.json

`scan` input may carry `"commit"`; EXISTENCE obligations only accept scans bound to the round's
candidate commit. Decisions marked USER need an interactive terminal; agents must not pipe them.

## Existing Arc skill integration

No seventh top-level Arc skill is installed. The graph ledger is owned by
`arc:verify`; all phases reuse one existing handoff record and the graph
identifiers rather than keeping separate agent memory or a second task tree.

```sh
arc-verify handoff --to arc-explore --graph academic-v1
arc-verify handoff --to arc-plan --graph academic-v1
arc-verify handoff --to arc-verify --graph academic-v1
arc-verify handoff --to arc-code --run R-FAILED
arc-verify handoff --to arc-audit --graph academic-v1
arc-verify handoff --to arc-vet --graph academic-v1
```

The response is bounded JSON: `arc:code` receives only an independently observed FAIL
run, first divergent step, frozen rule, source SHA, environment, actor, entry,
oracle and RAW reference; `arc:plan` gets open rules and risks; `arc:explore`
gets unconfirmed implementation gap candidates; `arc:verify` gets READY
cards; `arc:audit` gets read-only mismatch observations; `arc:vet` gets
runtime constraints. A handoff does not approve a rule, prove an E2E run,
fix source code or authorize a new environment. Repair handoff revalidates failure RAW;
runner errors, missing observations and altered evidence cannot prove a product defect.


### Performance architecture (native Go)

The original SQL-shell implementation forked `tursodb` for every query. This version
uses official `turso.tech/database/tursogo@v0.8.3` directly through `database/sql` in
one process, one local-file connection and real Go-managed transactions. The CLI still
exits after each operation, without background service or Turso Cloud. The optional
`tursodb .arc/verify/data.db` shell is for manual inspection, not a runtime dependency.
Go 1.24+ is required by the official driver. `go mod tidy` downloads verified module
checksums during setup. Avoid unbounded connection pools or untrusted remote DSNs.


## Repeatable performance baseline

```bash
go -C tools/arc-verify mod tidy
go -C tools/arc-verify test -run '^$' -bench 'Benchmark(TaskCardPagination|GateWithRAW)' -benchmem -count=5
```

Benchmark data is synthetic and isolated; it never accesses the remote test server
or passes a business E2E claim. Compare p50/p95 of **complete CLI commands** as well
as ns/op and allocs/op of the in-process components. Reject an optimization if it
breaks any contract/risk gate, orphan/absent-task detection or RAW revalidation.
Task materialization uses a transactional seek cursor with `plan --limit 128` (range 1..1024), and `recover`
reads only an indexed bounded graph slice (max depth 8, max 128 actions).
`gate` remains an intentionally exhaustive integrity check over *all* required
confirmed obligations; never substitute a fast status count for an E2E receipt.


The planner cursor is stored in `plan_cursor` and committed atomically with each task batch, so invoking the short-lived CLI again resumes without re-scanning earlier rules. Explicit approval of a previously unconfirmed rule resets the cursor safely; an existing task is reused only when its meaning is identical. Run `arc-verify init` after upgrading an existing local graph DB to add indexes and the cursor table. Each rule is capped at 64 named preconditions to bound the working set.

Graph gate PASS covers planned positive, per-condition negative and forced obligations.
Condition recovery, cross-role sequences, concurrency and real E2E are separate coverage
claims. Keep expected business functionality user-confirmed and independently inventory
implementation; do not let current code or an AI proposal become the expected contract.


### Initialization and migrations

Run `arc-verify init` from the tested repository root before the first graph import
and after upgrading the Go CLI. Initialization creates the tables **before** their
indexes in a single transaction, is safe to repeat, and preserves existing graph,
task and run records. The integration tests include fresh, repeated and existing
ledger initialization. This does not install a browser or grant remote access.

### Real native-Turso readiness check

```bash
arc-verify selftest
arc-policy doctor
```

`selftest` creates and removes a temporary local database, checking the actual
embedded driver, schema, index creation, insert and independent SQL readback.
It does not access a remote system or reuse a test account. When graph
verification is enabled, `arc-policy doctor` now runs this self-test rather
than only checking that an `arc-verify` binary exists.
