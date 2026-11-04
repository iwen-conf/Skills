---
name: arc-policy
description: >
  Start, verify, and finish Arc tasks with the arc-policy CLI. Use for task begin,
  task verify, receipt or hook failures, plugin setup, and Arc workflow enforcement
  (工程规范、验证回执、插件安装). Domain work uses the matching bundled Arc skill.
---

# Arc Policy Runtime

Plugin distributes capability. Skills guide behavior. Hooks intercept. CLI verifies.

This skill is the runtime contract. Domain work still belongs to one Arc skill per phase.

## Default mode

The agent writes source with host tools (`Write` / `Edit` / `search_replace` / `apply_patch` / `Shell` / DSH edit tools).
The CLI is mandatory for workflow start, commits, schema migrations, and delivery.

## Every coding session

1. `arc-policy doctor` if the CLI or policy might be missing.
2. `arc-policy detect` reports stack hints. Choose the owner by the requested result, then
   `arc-policy skills --name <skill>` resolves its actual installed entry and version.
   Read that entry and only the references needed now; discovery does not load every skill.
3. Before the first source edit: `arc-policy task begin --skill <name>`
   (`arc-code`, `arc-explore`, `arc-plan`, `arc-verify`, `arc-audit`, `arc-vet`) in the repository
   that owns the files. From a parent folder that is not a repository, use
   `arc-policy --cwd <repo> task begin …` and `arc-policy --cwd <repo> task verify`.
   A new third-party dependency starts with `arc-vet`; its decision record feeds `arc-code`.
4. Load that Arc skill and follow it. Do not skip `arc-idx` for repo search.
5. Before claiming verified code/runtime delivery: run `arc-policy task verify` in its own tool call,
   within the user's authorized execution scope. For a commit, first
   stage exactly its files (`git -C <repo> add …`), then verify. The receipt binds the index;
   staging after verify invalidates it. A delivery request uses a matching receipt or verifies
   the newly staged tree again.
6. `git -C <repo> commit` / `git -C <repo> push` only after a receipt that matches the current tree.
   Never `cd <repo> && git commit`; the hook refuses directory changes before a controlled command.
   A push needs no second verify when the pushed commit is exactly the receipt's tree.
7. Once the verified work is committed, the task is delivered and stops covering edits.
   New work starts with another `arc-policy task begin` (no `--force`; the delivered task is archived).
   Run this bookkeeping yourself and retry the edit without asking the user to run or approve
   the command. Keep the user's authorized scope; a custom `--slug` is optional.

## When verify cannot pass

- `tests: NOT_RUN — no test commands detected`: declare the project's real checks yourself in
  `.arc/policy.yaml` `verification.test_commands` (compile plus the touched module's tests:
  Gradle compile/unit tasks, `xcodebuild build`, `go test ./pkg/...`, `pnpm test`), then verify
  again. Rust compiles only on the remote build host the user's rules name: the command syncs
  the tree there and runs the tests over SSH. Never a no-op such as `true`. Do not hand the
  user a choice between configuring and bypassing.
- A real `FAIL`, or a `BLOCKED`/`NOT_RUN` that cannot be fixed now: report it in one line and
  keep the work uncommitted.
- The user explicitly confirms committing anyway ("直接提交", "不 verify 了", "我明确授权你提交",
  "不等测试，现在提交并推送"): stage the commit, run
  `arc-policy task authorize --quote '<their exact words>'`, then `git add`/`commit`/`push`
  yourself. A plain "提交并推送" is a delivery request, not a waiver: verify first. Never ask
  the user to run `git add` or `git commit`; they may push the commit themselves.
- `AUTHORIZED` proves the user's decision, not any check: the delivery report names every
  check that did not run.

## Commands

```bash
arc-policy doctor
arc-policy detect
arc-policy skills [--name arc-verify]
arc-policy task begin --skill arc-code [--slug <slug>]
arc-policy task switch --skill arc-verify
arc-policy task status
arc-policy task verify
arc-policy task authorize --quote '<the user's words>'
arc-policy task end [--incomplete]
arc-policy install-git-hooks
```

Receipts live in `.arc/policy/` and bind to the task instance, `git HEAD`, index and file contents.
A later edit or policy change invalidates the receipt. Re-run `task verify`; a failed attempt
revokes any previous PASS. Verification and task begin/end share a repository lock; concurrent
operations report `locked` and must be retried after the running operation finishes.
Legacy receipts without a content digest must be verified again.

Verification runs every detected affected test project, including nested Go/Rust modules.
Missing tools or JavaScript `scripts.test` are `NOT_RUN`; automatic local Rust tests on macOS are `BLOCKED`
because Rust is not compiled on the Mac at all, not even with `cargo zigbuild`. Projects declare
their complete test workflow with `verification.test_commands: [{argv: [...], cwd: .}]` in
`.arc/policy.yaml`.

`task authorize` writes an `AUTHORIZED` receipt bound to the same tree, recording the quote. Where the
host sends a transcript (Claude Code, Codex), `PreToolUse` refuses a quote that is not in one of the
user's last 5 typed messages; host-injected text and pasted material do not count.

## Business graph and cross-Skill phase gates

For whole-system/all-business/all-interaction tests or DB business graphs, follow the installed
`arc-verify` Skill's graph-start and shared workflow §6, including first use without a ledger.
Ordinary local tests do not require a graph; plan-only/read-only scope still applies.
`arc-verify handoff --to <arc-skill> --graph
<graph-version>` gives a bounded read-only transfer; a confirmed failed
`--run` can be handed to `arc-code`. Do not treat the packet as a
user-approved rule or a verified test. `arc-plan` writes canonical requirement
documents; only `arc-verify` changes the local graph ledger.

The `business_graph` verification gate is **mandatory for `arc-verify`**
when enabled, even with `verification.fail_closed: false`. Missing, duplicate or non-PASS
mandatory graph checks invalidate a PASS receipt, including after commit. A red pre-fix graph must not block `arc-code` from committing
an authorized root-cause repair under its usual code/test gates. A new E2E
`arc-verify` run must independently verify the repair before declaring the
business claim PASS. Other skills may be explicitly opted into this gate.

Loading another Skill does not switch the task. Inspect `task status`, then run
`arc-policy task switch --skill <next-skill>`. This archives the old task/receipt and starts a
new phase under one lock, without inheriting PASS. A stale or absent receipt blocks the switch.
For an intentionally unfinished phase, preserve its evidence in the existing progress document
and use `task switch --skill <next-skill> --incomplete --reason '<remaining work>'`.
Do not use `--force` for handoffs. `arc:verify` and `arc-verify` identify the same phase, including
legacy tasks and explicit `require_for_skills`; unknown external skill names are not rewritten.
Modes within the same skill keep the current task; they do not call `task switch`.

Scope comes from the user, not the stack hint or the receipt command. Read-only/plan-only/graph-only
work must not run a project's test commands merely to close a policy task. Preserve its artifact
and use an explicit incomplete handoff when runtime verification is outside the authorized scope.
Candidate-model completion is not a graph-verification receipt. Incomplete handoff does not satisfy
or disable Stop/Git gates; report the artifact produced and runtime NOT_RUN without claiming verified
code delivery. Existing end/begin commands remain
available; never erase history or turn a blocked business claim into PASS during bookkeeping.

Before declaring graph completion, check the active task and receipt both name `arc-verify`
(or the explicitly opted-in skill), and the receipt contains a PASS graph check. `AUTHORIZED`
permits the quoted delivery decision; it never proves the business claim.

## Cooperation with other skills

Keep one Arc owner; activate a specialist only for its bounded output, then return that output
to the owner. Carry the user's restrictions, existing IDs, source revision and unresolved evidence.
For a missing skill, resolve the host's available inventory and report the affected step; do not
invent an installation path or recursively activate unrelated skills.

| Need | Specialist / returned artifact |
|---|---|
| Business graph review | Bundled `arch-diagram`: spec + read-only HTML + layout result; Arc verify owns the ledger and verdict |
| Visual direction | Available `ui-design`: existing `DESIGN.md` / design decisions feed Arc code |
| Platform implementation | Available `android` / `harmonyos`: platform conventions within the current code scope |
| Commit, push or PR | Available `check`: delivery uses the current matching receipt and existing user authorization |
| Project workspace synchronization | Available `lark`: existing project configuration and authorized destination; no implicit external publication |

For a v2 business snapshot:

```bash
arc-policy skills --name arch-diagram
arc-policy --cwd <repo> diagram --snapshot <snapshot.json> \
  --presentation <presentation.json> --view flows --out <new-artifact-directory>
```

Follow the installed Arc verify `business-diagram.md` and upstream spec. The renderer ships at
a reviewed commit with its MIT license; builds verify the recorded files and never download code.
Node builds the standalone HTML; Chrome/Chromium/Edge (`CHROME`) is required for the layout check
and screenshot. Missing browser keeps HTML/spec available with visualization BLOCKED. `--no-check`
explicitly records NOT_RUN. Inspect the screenshot before claiming visual review.
Use Chinese business names with evidenced actual package names, otherwise `业务包待核实`.
The bridge preserves canonical IDs, candidate/RISK, recorded status, RAW errors and historical FAIL;
it never opens the DB, approves a rule, executes a target or refreshes a business PASS.

## Hook enforcement

- `SessionStart` initializes session state. Claude/Codex can receive its workflow context;
  Grok displays a user notification and does not inject that context into the model. Load the
  appropriate bundled Arc skill rather than treating a successful hook as loaded instructions.
- `PreToolUse` denies source edits with no active task or only a delivered task in the repository that owns the edited path, `git commit`/`push` without a matching receipt, `--no-verify`, and schema-migration CLIs. Paths outside every Git work tree are not gated.
- A denied edit names the repository and an executable `task begin --skill arc-code` command without placeholders. Run it yourself, then retry the original edit; never route the edit through the shell or a script to get past the denial.
- `task begin` probes Git's hook execution support before creating a task and installs
  `pre-commit` / `pre-push` gates through config hooks or a compatible default-directory
  file hook. Existing hooks are preserved; unsupported custom `core.hooksPath` fails explicitly.
  An agent's commit or push,
  by any route, needs a receipt matching the tree; a push also passes when every pushed commit is
  exactly the receipt's tree. A command the user types into the host (Claude Code `!`) carries the
  agent marker too, so it passes the same way; pushing other commits needs a separate terminal.
- `Stop` checks every repository the session touched (edit targets, shell `cd` / `-C` targets,
  paths in commands). Explicit edit targets are charged even without `PostToolUse`. Known read-only
  shell calls charge no concurrent source writes; possible writing calls charge changes within their
  tool window, closed at `PostToolUse` or Stop. Opaque scripts still cannot distinguish concurrent
  writes within that window. Legacy sessions without target/window records retain first-touch fallback.
- `Stop` uses TypeSafe (`jev ask`) for completion judgments in `block_on_claim` mode; when it cannot
  judge, `completion_markers` check positive occurrences outside Markdown examples, ignoring negated
  occurrences while retaining separate positive claims. A claim blocks the stop with the verify command; other turns stay
  silent, and each pending tree state is reported once per session. Commits stay gated either way.
- When Stop blocks code/runtime delivery, run the named `task verify` only within authorized scope.
  For read-only/plan-only/graph-only requests that exclude execution, retain the unverified task;
  report the artifact and runtime NOT_RUN instead of running tests to silence the notice. If verify
  fails on changes outside your task, report that. Never delete session logs,
  clear task state or manufacture a receipt to remove a notice.
- When the Python entrypoint runs but the engine cannot load or crashes, it emits an explicit
  refusal for edits and controlled operations in the host's supported output format. Disabled,
  untrusted, unlaunchable or timed-out hooks do not prove enforcement.
- Run `arc-policy install` from the updated plugin to keep the CLI and its provisioned
  dependencies outside disposable host caches. CLI and hooks use that dependency copy or an
  existing plugin/source `.venv`; they never install packages during a hook. `doctor` separates
  local prerequisites from native host loading/trust, which needs an actual host check.
- Current `Stop` registration covers the main agent. `SubagentStop` requires separate
  subagent identity and ownership handling and is not enabled by this plugin.
- Recursive Stop callbacks produce a passive reminder instead of another continuation. Cursor
  requests verification once from task state.

Passing a hook is not proof of skill execution. Only CLI receipts count.

## Honesty

Same-user hooks are not a security boundary. An agent can still disable hooks or Git config, pass `--no-verify` from inside a script, or forge `.arc/policy` and session logs. Shell writes are not blocked when they happen, but Stop and the Git hook catch them before delivery. CI and protected branches must re-run tests independently. This runtime makes the compliant path the easy path and the bypass visible.
