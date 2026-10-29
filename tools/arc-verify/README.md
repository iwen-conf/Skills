# ARC Verify (local graph test ledger, Go)

Short-lived Go CLI using the **official in-process `tursogo` driver** as its database engine.
It opens `.arc/verify/data.db` inside the tested repository and exits after every command.
No Python runtime, no Turso Cloud URL/auth token, no daemon and no background processes.
Turso's `tursodb` is distinct from the cloud-management `turso` CLI.

Build from this Skills repository: `mkdir -p ~/.local/bin && cd tools/arc-verify && go test ./... && go build -o ~/.local/bin/arc-verify .`
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
    arc-verify start --task academic-v1/R-ENROLL/POSITIVE --env test60 --commit HEAD_SHA
    arc-verify step --run RUN_ID --expected ENROLLED --observed ENROLLED --evidence .arc/verify/raw/RUN_ID-step-1.json
    arc-verify finish --run RUN_ID
    # record remains a shorthand for a single step plus finish
    arc-verify trace --run RUN_ID
    arc-verify risk --graph academic-v1
    arc-verify gate --graph academic-v1

An unconfirmed rule cannot produce required test tasks; a risk never becomes a rule
implicitly. Import never overwrites a graph version. RAW must contain matching `run`,
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
The offline Go tests cover model/JSONC checks. Real Turso SQL and remote-system E2E
must be executed before claiming end-to-end delivery.

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
`runner`, `oracle`, and numeric `exit_code`. The CLI independently looks up
the frozen graph expectation, checks the identities and hashes the RAW. A
`UNCHANGED` negative/forced outcome additionally needs `before` and `after`
to be identical plus `denied: true`. A mismatch creates a permanent FAIL at
that step. Only an explicitly safe, known observed state can be marked
`REPLAN`; it never rewrites the original FAIL. `finish` requires at least
one PASS step, valid unchanged RAW files, and a final observation matching the
task's frozen expected state. The gate reconstructs the mandatory task set
and rechecks evidence hashes; counts alone cannot grant PASS.


`arc-verify trace --run RUN_ID` returns an ordered execution trace with the **first diverging step**, latest observed state and next decision. A failed task cannot be overwritten by a successful diagnostic branch. `arc-verify recover --graph GRAPH --condition CONDITION` explores bounded AND prerequisites and OR recovery actions and marks unconfirmed actions unresolved rather than inventing rules.

## Resuming after failure or a lost runner

    arc-verify trace --run R123
    arc-verify abort --run R123 --reason "browser crashed; state unknown"
    # Restore an isolated fixture and independently check its state first.
    arc-verify retry --task "academic-v1/R-ENROLL/POSITIVE" --reason "recreated baseline"
    arc-verify next --graph academic-v1

An abort saves BLOCKED plus an immutable event. A retry creates a new
READY task attempt without deleting old FAIL/BLOCKED runs or RAW. This is not
database snapshot restoration, remote service rollback or permission to reuse
uncertain shared state.

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

The response is bounded JSON: `arc:code` receives only an observed FAILED
run, first divergent step, frozen rule, source SHA, environment, actor, entry,
oracle and RAW reference; `arc:plan` gets open rules and risks; `arc:explore`
gets unconfirmed implementation gap candidates; `arc:verify` gets READY
cards; `arc:audit` gets read-only mismatch observations; `arc:vet` gets
runtime constraints. A handoff does not approve a rule, prove an E2E run,
fix source code or authorize a new environment.


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
cd tools/arc-verify
go mod tidy
go test -run '^$' -bench 'Benchmark(TaskCardPagination|GateWithRAW)' -benchmem -count=5
```

Benchmark data is synthetic and isolated; it never accesses the remote test server
or passes a business E2E claim. Compare p50/p95 of **complete CLI commands** as well
as ns/op and allocs/op of the in-process components. Reject an optimization if it
breaks any contract/risk gate, orphan/absent-task detection or RAW revalidation.
Task materialization uses a transactional seek cursor with `plan --limit 128` (range 1..1024), and `recover`
reads only an indexed bounded graph slice (max depth 8, max 128 actions).
`gate` remains an intentionally exhaustive integrity check over *all* required
confirmed obligations; never substitute a fast status count for an E2E receipt.


The planner cursor is stored in `plan_cursor` and committed atomically with each task batch, so invoking the short-lived CLI again resumes without re-scanning earlier rules. Explicit approval of a previously unconfirmed rule resets the cursor safely; `INSERT OR IGNORE` keeps old task/run history untouched. Run `arc-verify init` after upgrading an existing local graph DB to add indexes and the cursor table. Each rule is capped at 64 named preconditions to bound the working set.
