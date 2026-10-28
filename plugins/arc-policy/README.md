# Arc Policy Runtime

A Coding Agent Policy Runtime, not a skill pack.

| Piece | Job |
|---|---|
| Plugin | Distribute skills, hooks, and policies to Claude Code, Codex, Grok, Cursor Agent, and DeepSeek Harness |
| Skills | Tell the agent which Arc workflow to run and why |
| Hooks | Intercept host tools (`PreToolUse`, `PostToolUse`, `Stop`, `SessionStart`) |
| CLI (`arc-policy`) | Begin a task, verify, and write a receipt bound to the tree |
| Policy YAML | Decide what is required or denied without editing hook scripts |

Default mode: **the agent still writes code with host tools**. The CLI is the gate for `task begin`, commits, schema migrations, and delivery. Putting every file write behind the CLI is a later, much more expensive mode.

## Install

CLI (once per machine):

```bash
ln -sf "$(pwd)/plugins/arc-policy/bin/arc-policy" ~/.local/bin/arc-policy
arc-policy doctor
```

The binary must stay `arc-policy`. Do not replace the existing `arc` → `arc-idx` symlink.

CLI and hook entrypoints use the plugin's `.venv/bin/python` first, then the linked source
repository's `.venv/bin/python`, otherwise the invoking Python. They never use the target
project's virtual environment or install dependencies during a hook. In this source checkout,
`uv sync --frozen` provisions the declared dependencies. For an independently installed plugin
copy, provision its runtime once (replace `<plugin-dir>` with its actual path):

```bash
uv venv --python 3.12 <plugin-dir>/.venv
uv pip install --python <plugin-dir>/.venv/bin/python 'PyYAML>=6.0.2'
```

Missing dependencies produce repair instructions; hook edits remain denied until repaired.

### Grok

```bash
grok plugin marketplace add /Users/iluwen/Documents/Code/Skills
grok plugin install arc-policy --trust
```

Or install the folder directly:

```bash
grok plugin install ./plugins/arc-policy --trust
```

Enable it in `~/.grok/config.toml`:

```toml
[plugins]
enabled = ["arc-policy"]
```

`~/.grok/plugins/` copies are auto-trusted.

### Claude Code

```bash
claude plugin marketplace add /Users/iluwen/Documents/Code/Skills
claude plugin install arc-policy
```

Or `--plugin-dir ./plugins/arc-policy`.

### Codex

Hooks bundled in a plugin need a **local** install and `/hooks` trust. They are not for the public plugin directory.

```bash
codex plugin marketplace add /Users/iluwen/Documents/Code/Skills
```

Repo enable:

```toml
# .codex/config.toml
[plugins."arc-policy@skills"]
enabled = true
```

Then `/hooks` and trust the Arc Policy handlers.

### Cursor Agent

User-level hooks (enforcement):

```bash
cp /Users/iluwen/Documents/Code/Skills/plugins/arc-policy/hosts/cursor/hooks.json ~/.cursor/hooks.json
```

Skill discovery (optional; `cursor-agent` also accepts `--plugin-dir`):

```bash
ln -sfn /Users/iluwen/Documents/Code/Skills/plugins/arc-policy ~/.cursor/plugins/local/arc-policy
```

`~/.cursor/hooks.json` is Cursor-native (`permission: deny`). Restart `cursor-agent`.

### DeepSeek Harness

```bash
ln -sfn /Users/iluwen/Documents/Code/Skills/plugins/arc-policy/hosts/dsh ~/.dsh/plugins/dsh-arc-policy
```

Add `"dsh-arc-policy": "link:../../plugins/dsh-arc-policy"` (a link; a `file:` copy cannot find the engine) to `~/.dsh/profiles/desktop/package.json`, insert in `cordis.patch.yml`, then `pnpm install` in that profile. Restart the Harness. See `hosts/dsh/README.md`. Does not replace `dsh-force-arc-idx`.

## Workflow

```bash
arc-policy task begin --skill arc-code
# agent writes code, follows the Arc skill
arc-policy task verify
# receipt matches task + HEAD + index and file contents → Stop unblocks delivery claims
# git commit is allowed
```

A task is delivered once HEAD has moved past its PASS receipt and the changes that receipt
verified are committed. A delivered task no longer covers new edits: `PreToolUse` denies source
edits until the next `task begin`, which archives the delivered task to history without
`--force`. `task end` also accepts a delivered task. If the verified changes are still
uncommitted (for example, another session committed first), the task stays in progress.

Task bookkeeping belongs to the agent. When an edit is denied because a task is missing or
delivered, the agent runs the supplied `task begin --skill arc-code` command and retries the
edit without asking the user to run it or approve it. A custom `--slug` is optional. The hook
does not create tasks as a side effect of an attempted edit.

Tasks belong to a Git repository. `PreToolUse` checks the repository that owns every edited
path (`file_path`, `path`, `notebook_path`, `target_file`, or apply_patch file headers, relative
to the tool's working directory), not the session directory: a task in one repository never
covers edits in another, and a patch across repositories needs a task in each. Paths outside
every Git work tree have no task scope, since `task begin` needs Git. A session started in a
parent folder that is not a repository (several repositories side by side) runs the CLI with
`arc-policy --cwd <repo> task begin|verify`; `SessionStart` lists the active tasks of its direct
child repositories and `Stop` checks them. An edit tool whose input names no path is denied there.

`task begin` also registers the Git-level gate in that repository (`arc-policy install-git-hooks`
does the same by hand): config-based `pre-commit` and `pre-push` hooks (Git 2.54+), which run
beside any hook files or `core.hooksPath`. When an agent commits or pushes, however Git is invoked
(`bash -c`, a script, a subprocess), the hook requires a receipt matching the tree; push after a
commit therefore needs another `task verify`. Humans are not gated: the check applies only when an
agent marker is set (`CLAUDECODE`, `GROK_AGENT`, `CODEX_THREAD_ID`, `CODEX_CI`, `CURSOR_AGENT`,
`PI_CODING_AGENT`, `AI_AGENT`, or `ARC_POLICY_AGENT`, which the DSH adapter exports).

Project overlay: `.arc/policy.yaml` (merged on top of `policies/default.yaml`).

Verification checks every detected affected Go, Rust, Python and JavaScript test project,
including nested modules and untracked files. Rust defaults to all test targets, without
`--lib`. Missing runners, missing JavaScript `scripts.test`, and failed commands cannot produce a PASS. Python uses the existing
project virtual environment when available. On macOS, automatic Rust tests are `BLOCKED`:
local compilation must use `cargo zigbuild`; declare the project's actual test workflow.

For a project with its own test entry points, declare the complete required plan explicitly.
Every command runs without a shell; `cwd` must stay inside the repository. For this Skills
repository, root pytest covers the plugin and its host adapters; dbqual also has its own suite:

```yaml
verification:
  test_commands:
    - argv: [uv, run, --no-sync, python, -m, pytest, -q]
      cwd: .
    - argv: [../../../../.venv/bin/python, -m, pytest, -q]
      cwd: Arc/arc:verify/scripts/dbqual
```

Each verification attempt first revokes the old receipt. Tests must finish against the same
task instance and content snapshot they started with, including policy loading. Verification,
task begin and task end share a repository lock; overlapping operations fail immediately with
a `locked` diagnostic. State and receipts are saved atomically;
`.arc/policy/` and `.arc/artifacts/` are excluded from the snapshot, while project policy remains
included even when gitignored. Older receipts without a content digest must be verified again.

### Stop hooks

The default `block_on_claim` mode uses TypeSafe through the existing `jev ask` CLI to judge the
last assistant message. It needs `jev` on PATH and `TYPESAFE_API_KEY` or `~/.typesafe/api_key`.
Only the final assistant message and completion examples are sent, not session history.
`completion_markers` are examples for the semantic judgment.
Clear completion claims block the stop and send the agent to `task verify`; negative statements,
questions and progress notes stay silent, because commits remain gated by the receipt either way.
When TypeSafe cannot judge (no `jev`, missing credentials, invalid response, timeout, uncertain
answer), `completion_markers` decide as whole-word matches, so a claim is still gated. Neither path
creates a receipt or authorizes commits.

`stop.mode: block` uses task state directly; `stop.mode: remind` only reports pending verification.
Repeated Stop callbacks (`stop_hook_active` / Cursor `loop_count`) report a passive `systemMessage`
instead of starting another continuation. Cursor's completed event has no assistant message,
so it requests verification once based on task state. An unchanged task does not block merely
because the worktree was already dirty when it began. After delivery, Stop stays silent unless
there are new changes; those are reported as needing a new task, not as the delivered task's work.

With a host session id, `remind` and `block_on_claim` notify once per pending tree state: the same
unverified state is not repeated on later turns, and a new edit re-arms the check. A session that
saw no change in a repository since it first touched it is not held to that repository's task.

Stop checks every repository the session reached, not only the working one. With a host session
id (`session_id`, Grok `sessionId`, Cursor `conversation_id`; the DSH adapter sends one
per process), each
`PreToolUse` records the repositories an edit names or a shell command can reach (its directory,
`cd` and `-C` targets, absolute paths in the command or script body) with their dirty-file
digests at first touch, under `~/.cache/arc-policy/sessions/` (`ARC_POLICY_STATE_DIR`; logs older
than 14 days are pruned). At Stop, uncommitted changes made since then need a matching receipt,
whether an edit tool, the shell or a script wrote them, and even when `task begin` came later; a
repository without a task is reported as changed without one. Cursor's `workspace_roots` are all
checked. When several repositories report, the strictest `stop.mode` applies.

Once the host reports finished tool calls (`PostToolUse`; Cursor `postToolUse` /
`afterShellExecution`; the DSH adapter sends one after each call), Stop charges a session only for
the paths its own calls wrote: the target paths of an allowed edit, and for a shell call the
dirty files whose mode, size or mtime changed between its `PreToolUse` and `PostToolUse` (paired by
`tool_use_id`, else by tool input). Concurrent sessions in one repository are then not charged for
each other's work. A call whose finished event never arrives (a failed call on a host that reports
failures separately, a denied call has none) stays open until Stop, which charges everything that
changed in its repositories since it started. Writes during a call's window by another session, or
by a process the session left running after its call finished, are attributed the same imprecise
way. Without `PostToolUse` Stop falls back to first-touch state and cannot tell this session's
writes from another session's in the same repository; the reminder says so.

Shell guards check every command segment, including command substitutions, unquoted heredoc
substitutions, `sh -c` / `bash -c` scripts, `eval`, and prefix wrappers (`timeout`, `nohup`,
`sudo`, `nice`, `time`, `exec`, `xargs`, `env`, `command`). Heredoc bodies are data, so prose with
apostrophes no longer trips the parser. Shell input the host truncated (Grok, over 128 KiB) is
denied because its hidden part cannot be checked. `workdir` and Git `-C` resolve against the
target repository's receipt. Run verification in a separate tool call before committing.
Controlled commands using shell groups, `cd`, `env -C` / `--chdir` or `sudo -D` are refused
because their directory scope is not interpreted; run them separately with `git -C` instead.

The engine fails closed: if it cannot load or crashes, edits and controlled operations are denied
with the error and other tool calls proceed with it as a message. The DSH adapter denies a
tool call when the engine cannot answer at all. Claude Code, Codex and Grok ignore a hook that
times out; `PreToolUse` therefore allows 30 seconds (it normally takes about 50 ms), and the Git
hook still gates commits and pushes when a host skipped a call.

If Stop reports an unverified task, run `arc-policy task status`, then `arc-policy task verify`
in that task's repository. Fix any `FAIL`, `BLOCKED` or `NOT_RUN` evidence before claiming delivery.

## What is actually forced

| Constraint | Mechanism | Strength |
|---|---|---|
| Discover the skill | Plugin + SKILL.md | Guidance only |
| Enter the workflow | `PreToolUse` denies writes in a repository with no `task begin` there | Host-enforced, bypassable |
| Controlled ops | Deny `git commit` / `push` without a matching receipt; deny `--no-verify` / commit `-n` | Host-enforced, bypassable |
| Agent commit / push by any route | Git `pre-commit` / `pre-push` config hooks require a receipt when an agent marker is set | Git-enforced, bypassable with `--no-verify` or by unsetting markers |
| Delivery | `Stop` checks every repository the session touched, including shell-written changes; `task verify` fail-closed | Host-enforced, bypassable |
| Merge | CI re-runs tests on a protected branch | Real gate |

Same-user hooks are **not** a security boundary. An agent sharing the user's uid can still forge `.arc/policy` or session logs, disable hooks or Git config, pass `--no-verify` from inside a script, run a migration through `make` / `npm run` / a script, or write to a repository no command names. Treat this runtime as process control plus an audit trail. Irreversible enforcement needs a different principal (CI, protected branch, or a CLI/MCP with its own credentials).

## Layout

```
plugins/arc-policy/
  plugin.json                 portable / Grok
  .claude-plugin/plugin.json
  .codex-plugin/plugin.json
  skills/arc-policy/SKILL.md
  hooks/hooks.json
  hooks/hook.py
  hosts/cursor/hooks.json     Cursor Agent user hooks
  hosts/dsh/                  DeepSeek Harness Cordis plugin
  policies/{default,go,rust}.yaml
  lib/arc_policy/             policy engine
  bin/arc-policy
```

The six Arc skills (including `arc-vet`) stay in `Arc/` and continue to install through `scripts/sync_skills.py`. This plugin adds the enforcement plane; it does not fork those skills.
