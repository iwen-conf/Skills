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

The agent writes source with host tools (`Write` / `Edit` / `search_replace` / `apply_patch` / `Shell` / OMP `write` / DSH edit tools).
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
5. Before claiming done or committing: `arc-policy task verify`
6. `git commit` / `git push` only after a receipt that matches the current tree.
7. Once the verified work is committed, the task is delivered and stops covering edits.
   New work starts with another `arc-policy task begin` (no `--force`; the delivered task is archived).

## Commands

```bash
arc-policy doctor
arc-policy detect
arc-policy task begin --skill arc-code [--slug <slug>]
arc-policy task status
arc-policy task verify
arc-policy task end [--incomplete]
arc-policy install-git-hooks
```

Receipts live in `.arc/policy/` and bind to the task instance, `git HEAD`, index and file contents.
A later edit or policy change invalidates the receipt. Re-run `task verify`; a failed attempt
revokes any previous PASS. Verification and task begin/end share a repository lock; concurrent
operations report `locked` and must be retried after the running operation finishes.
Legacy receipts without a content digest must be verified again.

Verification runs every detected affected test project, including nested Go/Rust modules.
Missing tools or JavaScript `scripts.test` are `NOT_RUN`; automatic local Rust tests on macOS are `BLOCKED` because compilation
must use `cargo zigbuild`. Projects can declare their complete test workflow with
`verification.test_commands: [{argv: [...], cwd: .}]` in `.arc/policy.yaml`.

## Hook enforcement

- `SessionStart` injects stack, skill, and CLI instructions.
- `PreToolUse` denies source edits with no active task or only a delivered task in the repository that owns the edited path, `git commit`/`push` without a matching receipt, `--no-verify`, and schema-migration CLIs. Paths outside every Git work tree are not gated.
- A denied edit names the repository and the exact `task begin` command. Run it; never route the edit through the shell or a script to get past the denial.
- `task begin` registers Git `pre-commit` / `pre-push` config hooks: an agent's commit or push,
  by any route, needs a receipt matching the tree. Verify again after each commit before pushing.
- `Stop` checks every repository the session touched (edit targets, shell `cd` / `-C` targets,
  paths in commands). Uncommitted changes since first touch need a receipt, including ones written
  through the shell or before a late `task begin`.
- `Stop` uses TypeSafe (`jev ask`) for completion judgments in `block_on_claim` mode. Missing tools,
  credentials, service errors and uncertain answers remain explicitly unverified; commits stay gated.
- The engine fails closed: when it cannot load or crashes, edits and controlled operations are denied.
- Recursive Stop callbacks produce a passive reminder instead of another continuation. OMP uses
  `session_stop`; PI uses `agent_before_settle`. Cursor requests verification once from task state.

Passing a hook is not proof of skill execution. Only CLI receipts count.

## Honesty

Same-user hooks are not a security boundary. An agent can still disable hooks or Git config, pass `--no-verify` from inside a script, or forge `.arc/policy` and session logs. Shell writes are not blocked when they happen, but Stop and the Git hook catch them before delivery. CI and protected branches must re-run tests independently. This runtime makes the compliant path the easy path and the bypass visible.
