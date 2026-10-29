# ARC Verify (local graph test ledger, Go)

Short-lived Go CLI using the **official local `tursodb` SQL shell** as its database engine.
It opens `.arc/verify/data.db` inside the tested repository and exits after every command.
No Python runtime, no Turso Cloud URL/auth token, no daemon and no background processes.
Turso's `tursodb` is distinct from the cloud-management `turso` CLI.

Build: `cd tools/arc-verify && go test ./... && go build -o ~/.local/bin/arc-verify .`
Requires the official `tursodb` executable in PATH for DB commands.

Project lifecycle:

    arc-verify init
    arc-verify validate --file docs/verify/academic-v1.json
    arc-verify import --file docs/verify/academic-v1.json
    arc-verify approve --graph academic-v1           # USER terminal approval
    arc-verify approve --graph academic-v1 --rule R-ENROLL
    arc-verify plan --graph academic-v1
    arc-verify next --graph academic-v1
    arc-verify start --task academic-v1/R-ENROLL/POSITIVE --env test60 --commit HEAD_SHA
    arc-verify record --run RUN_ID --observed ENROLLED --evidence .arc/verify/raw/RUN_ID.json
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
