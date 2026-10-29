---
name: arc-policy
description: >
  Coding Agent Policy Runtime for Arc. Use when a session must follow Arc skills
  (arc-explore, arc-plan, arc-code, arc-verify, arc-audit, arc-vet) and the arc-policy CLI
  for task begin, controlled operations, and delivery verification. Triggers:
  工程规范插件, policy runtime, 强制使用 CLI, task begin, task verify, PreToolUse
  hook, Stop hook. Use when the user runs /arc-policy.
---

# Arc Policy Runtime

Plugin distributes capability. Skills guide behavior. Hooks intercept. CLI verifies.

This skill is the runtime contract. Domain work still belongs to one Arc skill per phase.

## Default mode

The agent writes source with host tools (`Write` / `Edit` / `search_replace` / `apply_patch` / `Shell` / DSH edit tools).
The CLI is mandatory for workflow start, commits, schema migrations, and delivery.

## Every coding session

1. `arc-policy doctor` if the CLI or policy might be missing.
2. `arc-policy detect` for stack → recommended skill.
3. Before the first source edit: `arc-policy task begin --skill <name>`
   (`arc-code`, `arc-explore`, `arc-plan`, `arc-verify`, `arc-audit`, `arc-vet`) in the repository
   that owns the files. From a parent folder that is not a repository, use
   `arc-policy --cwd <repo> task begin …` and `arc-policy --cwd <repo> task verify`.
   A new third-party dependency starts with `arc-vet`; its decision record feeds `arc-code`.
4. Load that Arc skill and follow it. Do not skip `arc-idx` for repo search.
5. Before claiming done or committing: stage exactly the files to commit (`git -C <repo> add …`),
   then `arc-policy task verify` in its own tool call. The receipt binds the index, so staging after
   verify invalidates it. Run verify when the work is finished, so a later "提交并推送" is a commit,
   not a test run.
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
arc-policy task begin --skill arc-code [--slug <slug>]
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

## Hook enforcement

- `SessionStart` injects stack, skill, and CLI instructions.
- `PreToolUse` denies source edits with no active task or only a delivered task in the repository that owns the edited path, `git commit`/`push` without a matching receipt, `--no-verify`, and schema-migration CLIs. Paths outside every Git work tree are not gated.
- A denied edit names the repository and an executable `task begin --skill arc-code` command without placeholders. Run it yourself, then retry the original edit; never route the edit through the shell or a script to get past the denial.
- `task begin` registers Git `pre-commit` / `pre-push` config hooks: an agent's commit or push,
  by any route, needs a receipt matching the tree; a push also passes when every pushed commit is
  exactly the receipt's tree. A command the user types into the host (Claude Code `!`) carries the
  agent marker too, so it passes the same way; pushing other commits needs a separate terminal.
- `Stop` checks every repository the session touched (edit targets, shell `cd` / `-C` targets,
  paths in commands). Uncommitted changes since first touch need a receipt, including ones written
  through the shell or before a late `task begin`. Once the host reports `PostToolUse`, only paths this
  session's edits and shell calls wrote are charged, so concurrent sessions do not owe each other's work.
- `Stop` uses TypeSafe (`jev ask`) for completion judgments in `block_on_claim` mode; when it cannot
  judge, `completion_markers` decide. A claim blocks the stop with the verify command; other turns stay
  silent, and each pending tree state is reported once per session. Commits stay gated either way.
- When Stop blocks, run the named `task verify`. If it fails on changes outside your task, report
  that instead of claiming delivery; if the work is unfinished, say so.
- The engine fails closed: when it cannot load or crashes, edits and controlled operations are denied.
- CLI and hooks prefer the plugin's `.venv`, then the linked source repository's `.venv`, before
  loading engine dependencies. They never install packages during a hook; missing PyYAML reports
  the runtime and exact setup commands from the plugin README.
- Recursive Stop callbacks produce a passive reminder instead of another continuation. Cursor
  requests verification once from task state.

Passing a hook is not proof of skill execution. Only CLI receipts count.

## Honesty

Same-user hooks are not a security boundary. An agent can still disable hooks or Git config, pass `--no-verify` from inside a script, or forge `.arc/policy` and session logs. Shell writes are not blocked when they happen, but Stop and the Git hook catch them before delivery. CI and protected branches must re-run tests independently. This runtime makes the compliant path the easy path and the bypass visible.
