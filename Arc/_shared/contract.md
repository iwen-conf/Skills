# Arc Contract

Every Arc skill reads this file first. Repository-wide gates (completion, scope lock, domain keys,
evidence-first root-cause repair, simplicity, surgical edits) live in
[execution-truth.md](../../docs/execution-truth.md); this file adds only Arc-specific rules.

## 1. Authority

| Skill | May change | Never changes |
|---|---|---|
| `arc:explore` | nothing (index files only) | source, tests, docs |
| `arc:audit` | nothing (reports under `.arc/artifacts/`) | source, tests, configs |
| `arc:plan` | `docs/`, task documents | source, tests, configs |
| `arc:code` | source, tests, configs, docs the change touches | deploy targets, remote data |
| `arc:verify` | tests, fixtures, test configs, evidence; a disposable `git worktree` for sensitivity proof, removed after use | production source in the working tree (report defects to `arc:code`) |

If the request needs a change outside the active skill's authority, finish the current phase,
report, and hand off. One phase has one main skill; skills never load each other.

## 2. Task tier

- **T0**: one file, no contract/schema/security change, no new dependency.
- **T1**: one module or layer, no external contract change.
- **T2**: any of: public API or DTO, database schema, auth, payment or money, concurrency,
  ≥3 packages, data migration, new dependency.

T2 needs a written brief (goal, scope, non-goals, acceptance commands) before the first edit.
Multi-session or multi-person T2 work needs task documents from `arc:plan`.

## 3. Scope lock

When the user or a task document names files, directories, symbols or a token from
execution-truth §3, edits outside that scope require stopping and reporting first.

## 4. Vocabulary (the only allowed values)

| Kind | Values |
|---|---|
| Verdict | `PASS` `FAIL` `BLOCKED` `NOT_RUN` `FLAKY` |
| Task status | `[ ]` todo · `[/]` in progress · `[?]` unverified · `[x]` done with evidence · `[!]` blocked · `[-]` dropped or deferred (reason and restart condition required) |
| Finding status | `confirmed` `likely` `needs-triage` `false-positive` `manual-gap` |
| Severity | `Critical` `High` `Medium` `Low` `Info` |
| Task priority | Critical→`P0`, High→`P1`, Medium→`P2`, Low/Info→`P3` (no other mapping) |

`NOT_RUN` and `BLOCKED` always carry a reason. Nothing unexecuted is `PASS`.

## 5. Evidence and artifacts

- Write logs, reports and scan output to `.arc/artifacts/<task-slug>/{logs,reports,security}/`.
- Do not commit artifacts unless asked. Redact secrets, tokens, cookies and personal data before
  quoting any output.
- Every completion claim cites a command and its result, a `file:line`, or a test name.

## 6. Environment

1. Destructive commands (data deletion, `DROP`, force push, hard reset, remote restart) need
   explicit approval in the current conversation.
2. Installing tools or dependencies needs permission; record what was installed.
3. Do not start PostgreSQL servers or container runtimes on the workstation. Use the project's
   test environment or client-side checks; when none exists, mark the claim `BLOCKED`.
4. Hosts with subagents: delegate read-heavy exploration, one focused task per subagent. The main
   agent keeps responsibility for verification; a failed, timed-out or partial subagent result is
   reported as such, never silently dropped or promoted to `PASS`.

## 7. Lark hook

If the project root has `.lark.json` and the task has tracked status, hand the status update to
`lark-workflow-project-docs` at the end of the phase. Without `.lark.json` or an explicit user
request, never create or edit Lark resources.

## 8. Commits

Commit only when the user asks. Format: `<type>: <description>` with type in
`feat` `fix` `refactor` `docs` `test` `chore`; one sentence naming what changed and where.

## 9. Report (end of every phase)

1. **Done**: what changed or was found.
2. **Evidence**: commands with results, `file:line`, tests.
3. **Files**: changed paths (or "none").
4. **Not verified**: each item with its `NOT_RUN`/`BLOCKED` reason.
5. **Next**: the handoff or the user decision needed.
