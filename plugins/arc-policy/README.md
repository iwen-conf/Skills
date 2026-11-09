# Arc Policy Runtime

Arc workflows and their policy runtime, packaged for Claude Code, Grok Build, and Codex CLI.
Version 0.4.3 keeps one policy engine and one canonical `Arc/` skill source, with generated
host manifests, a complete bundled skill tree, and a strict Codex wire adapter.

| Piece | Job |
|---|---|
| Plugin | Distribute skills, hooks, and policies to Claude Code, Codex, Grok, Cursor Agent, and DeepSeek Harness |
| Skills | Tell the agent which Arc workflow to run and why |
| Hooks | Intercept host tools (`PreToolUse`, `PostToolUse`, `Stop`, `SessionStart`) |
| CLI (`arc-policy`) | Begin a task, verify, and write a receipt bound to the tree |
| Policy YAML | Decide what is required or denied without editing hook scripts |

Default mode: **the agent still writes code with host tools**. The CLI gates `task begin`, controlled schema migrations and verified delivery—not every commit. After explicit `.arc/arc.db` cutover, PostToolUse also calls `arc update --changed-only` to persist idempotent file changes and outbox events; this does not verify or close tasks. Putting every file write behind the CLI is a later, much more expensive mode.

## Arc Policy state/receipts in arc.db (opt-in)

Arc 0.4.0 + arc-policy 0.4.3 support a **separate, explicit Policy
state cutover** after the graph/resource database has been activated.
Install both updated runtimes and stop other agents or legacy Policy
writers before migration:

```bash
bash scripts/install_arc.sh
.venv/bin/python plugins/arc-policy/bin/arc-policy install
arc db status --repo .
arc policy state status --repo .           # legacy before cutover
arc policy state migrate --repo . --confirm-offline
arc policy state status --repo .           # mode = arc.db
arc-policy --cwd . task status             # reads live task from Turso
```

The operation imports the active task, historical tasks and every historical
receipt into the same Turso `.arc/arc.db`, under the Policy lock and Arc DB
lock. A SHA-256-verified JSON backup remains in `.arc/backups/`, and legacy
receipt files are archived there. A directory tombstone replaces
`.arc/policy/state.json` to prevent older Policy runtimes from silently
writing a second authoritative JSON state. The migration can be re-run
offline to finish archival after interruption; it never overwrites migrated
DB state.

Python uses the `arc policy state get/put` JSON interface with optimistic
version checks and **no fallback to state.json after cutover**. A missing
binary, version conflict or broken Turso file blocks operations. The
Policy task/receipt lock continues to protect the verification and
completion path; `arc task complete` reads the current Policy receipt
from Turso after migration. Git commits/pushes still do not run verify.

**Do not migrate while old agents are running.** Windows Policy migration
and completion remain fail-closed until compatible cross-language locking
is implemented. This is an opt-in storage cutover, not automatic verification
or authorization of any task.

## Local business-graph gate (opt-in)

The existing Python `arc-policy` runtime stays intact. Its `task verify` command
checks the independent **Go** business graph ledger when
`.arc/verify/config.jsonc` exists or `verification.business_graph` is configured.
The gate runner prefers `arc verify gate` when `arc` is installed, falling
back to the legacy backend only before an active unified-DB cutover. After
`.arc/arc.json` activation, the default graph DB is `.arc/arc.db`, and `arc`
is required for locked access. An explicitly pinned policy DB overrides the
default; neither historical data nor policy receipts are silently rewritten.

The graph ledger lives in the target repository's local
`.arc/verify/data.db` file, opened by the official in-process `tursogo` Go driver.
It never contacts Turso Cloud; the tested web/API application may be on a
separate authorized remote server. The Go CLI is short-lived: no daemon,
persistent browser process or background worker.

```yaml
verification:
  business_graph:
    graph: academic-v1
    db: .arc/verify/data.db
    risk_review: false   # true adds `--risk-review`: undecided risks then fail the gate
```

The graph check is an additional fail-closed receipt condition; it accepts a v1
`planned_rule_tasks` proof or a v2 `complete_business_model_obligations` proof; existing
unit/integration/E2E `verification.test_commands` continue to run.
For a required graph, `verification.fail_closed: false` cannot turn BLOCKED/NOT_RUN into PASS.
Historical PASS receipts with missing, duplicate or non-PASS mandatory graph checks are refused
by current-tree and delivered-task checks; regular commits and pushes need no receipt. `AUTHORIZED` remains a distinct
user waiver, never business verification evidence.
No graph ID, no local DB, unconfirmed business rules, unexecuted tasks,
or a failing observed state => no graph PASS. Risks discovered by AI
remain non-gating until the user confirms them as business rules, unless
the project sets `risk_review: true`.
See `tools/arc-verify/README.md` and
`Arc/arc:verify/references/business-graph.md`.

Switching Skills also requires switching the active policy task with `task switch --skill <next-skill>`.
An unfinished phase needs `--incomplete --reason '<why>'`; the old task and evidence remain in
history. Loading a Skill alone does not update the stored task identity or verification scope.

This is workflow enforcement, not an adversarial security boundary:
a same-user agent can alter local files or disable hooks. Use protected CI
and separately trusted execution for high-assurance evidence.

## Read-only business diagrams with arch-diagram

The plugin bundles the official [arch-diagram skill](https://github.com/bassnova42/arch-diagram-skill)
at commit `879ca7fa503a64ab46b5bc77d2062ded32fadc84` (1.2.3), with its MIT license and original
renderer files. `skills/arch-diagram/source.json` records the source path, SHA-256 and Git blob SHA
for every payload file. The distribution builder and drawing command verify the reviewed manifest
and complete file set locally. Updating upstream requires reviewing a new commit and updating the
manifest integrity anchor; neither build nor hooks downloads a replacement renderer.

```bash
arc-policy skills --name arch-diagram
arc-policy --cwd /path/to/target diagram \
  --snapshot .arc/verify/snapshots/GRAPH/EXPORT/snapshot.json \
  --presentation .arc/verify/diagrams/presentation.json \
  --view flows --out .arc/verify/diagrams/GRAPH/new-review
```

`--view capabilities` shows the business tree. Inputs and relative output paths resolve against
`--cwd`. Open `<view>.review.html` for the business overview and expandable evidence. The output
also keeps `<view>.spec.json`, the unchanged official `<view>.html` and, when the official check
succeeds, `<view>.check.png`. Existing outputs are never overwritten. The command never opens the database,
changes approvals or runs a business target. Keep the canonical DB, CLI snapshot and flow ledger.

Version 0.4.1 represents valid capability containment with nested official groups, using balanced
sibling cards instead of repeated long `contains` arrows. Shared ownership and cycles keep explicit
relationships. Real participation/flow wires retain meaningful labels. Group and wire mappings
preserve all original node/edge IDs, including parallel relationships; parent failures stay visible.
Card faces prioritize the Chinese business name, full package name and relevant status. Full source
paths, IDs, hashes and verification records remain in the companion's read-only details. The
companion embeds the exact official HTML in an offline sandboxed frame; it does not replace or
modify the pinned renderer. Narrow screens scroll the diagram at readable size.

Presentation is a small source-bound attachment; it cannot supply verdicts or rewrite business
rules. Its graph/hash must match the snapshot, and package names need an actual repository-relative
source file and one-based line. Example shape (use the snapshot's real IDs and hash):

```json
{
  "graph": "graph-version",
  "graph_sha256": "<exact snapshot meta.graph_sha256>",
  "title_zh": "报名业务流程",
  "nodes": {
    "action-id": {
      "name_zh": "提交报名",
      "packages": [{"name": "actual.package", "file": "path/to/manifest", "line": 1}]
    }
  }
}
```

Absent or unverified package evidence displays `业务包待核实`; a business node ID, table or route
is never used as a fabricated package name. More than 16 nodes or 32 edges requires a narrower
`node_ids` selection; omitted source IDs and truncated coverage stay explicit. Diagram anchors
use safe aliases because canonical business IDs may contain dots; source IDs remain unchanged.
The view preserves candidate status, RULE confirmation, RISK, recorded verdicts, historical FAIL
and RAW errors. Animated execution traces are disabled. A snapshot's evidence-integrity PASS is
never treated as a business PASS, and offline rendering cannot refresh old evidence.

Node builds the self-contained HTML without external resources. Chrome/Chromium/Edge is required
for layout checks and screenshots; set `CHROME` to an existing browser when necessary. Missing
browser or failed checks return a nonzero exit while preserving HTML/spec for review. `--no-check`
builds a draft and explicitly records layout NOT_RUN. Even after the layout checker passes,
inspect the screenshot before claiming visual review. Node/browser availability does not block
independent Arc phases, and no long-lived service or browser is installed by the plugin.
The adapter asserts actual group containment and non-overlapping siblings because the upstream
checker treats group borders as decoration. A geometry PASS still needs a human/agent visual pass
for business hierarchy, balanced spacing and readable labels. An unavailable browser leaves that
work explicitly pending; HTML generation alone is not a finished visual review.

## Install

### Persistent CLI and dependencies

From this checkout, prepare the Python 3.12 environment and install the persistent CLI:

```bash
uv sync --frozen
.venv/bin/python plugins/arc-policy/bin/arc-policy install
```

For a standalone plugin folder, the equivalent bootstrap is:

```bash
uv run --no-project --python 3.12 --with 'PyYAML>=6.0.2' <plugin-dir>/bin/arc-policy install
```

`install` copies the plugin and the already provisioned pure-Python PyYAML dependency,
including its license, into `~/.local/share/arc-policy/versions/` (or
`$XDG_DATA_HOME/arc-policy`; `ARC_POLICY_HOME` explicitly overrides it). It checks the new
snapshot before switching `current`, then writes a launcher at `~/.local/bin/arc-policy`.
`--bin-dir <directory>` selects another PATH directory. Ensure that directory is on PATH.
Installation itself is offline; only the explicit environment bootstrap may download packages.

This stable runtime survives host plugin cache replacement. Re-run `install` from the updated
plugin after an upgrade. Old runtime snapshots remain available to running processes.
The binary stays `arc-policy`; the existing `arc` → `arc-idx` entry is separate.

Hook startup first uses an installed dependency copy, then the plugin/source `.venv`, then
the invoking Python. It never provisions packages inside a tool hook. A Python entrypoint
that can run but cannot load the engine emits an explicit refusal; a missing Python binary,
host timeout, disabled handler or untrusted hook cannot be treated as enforced policy.

`arc-policy doctor` reports engine, bundled skills, persistent CLI, external `arc-idx`,
optional graph runtime and Git capability separately. Its `workflow_ready` requires those
local prerequisites. `host_hooks_ready: null` means native host loading/trust and an actual
denied tool call still need verification. Python, `arc-idx` and optional `arc-verify` are
external dependencies; the plugin does not silently install them.

### Grok

```bash
grok plugin marketplace add iwen-conf/Skills
grok plugin install arc-policy --trust
grok plugin enable arc-policy
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

Check `grok plugin details arc-policy` and `grok inspect --json`, then `/hooks` in a trusted
project. Grok Build 1.0.64+ also applies folder trust to plugin hooks. `SessionStart` initializes
state and shows a user notification; it does not inject this workflow into the model. The
bundled skills carry the workflow. Current Stop coverage is the main agent; `SubagentStop`
is a separate contract. See the [host compatibility record](https://github.com/iwen-conf/Skills/blob/main/docs/plugin-host-compatibility.md).

### Claude Code

```bash
claude plugin marketplace add iwen-conf/Skills
claude plugin install arc-policy@skills
```

For development use `claude --plugin-dir ./plugins/arc-policy`; validate with
`claude plugin validate ./plugins/arc-policy`. Plugin skills use the plugin namespace:
the source `arc:code` is packaged as `arc-code` and invoked as `/arc-policy:arc-code`.
Keep all resources inside the plugin directory; an installed cache cannot follow `../../Arc/`.

### Codex

Use the local marketplace and review bundled hooks. The repository's native catalog is
`.agents/plugins/marketplace.json`; its `source.path` is relative to the repository root.

```bash
codex plugin marketplace add /absolute/path/to/Skills
codex plugin add arc-policy@skills
codex plugin list --json
```

Repo enable:

```toml
# .codex/config.toml
[plugins."arc-policy@skills"]
enabled = true
```

Then open `/hooks` and review/trust the Arc Policy handlers. Installation does not grant hook
trust; changed handlers require review again. Project config is read only for a trusted project.

Codex uses `hooks/codex.json` and invokes the shared entry with `--host codex`. Only supported
output fields reach Codex; the other adapters keep their compatible envelope. The pinned
contracts cover `SessionStart`, `PreToolUse`, `PostToolUse`, and `Stop` at `rust-v0.155.1` and
`rust-v0.166.0`. Tool matching covers `Bash` (including host-normalized `exec_command`) and
`apply_patch`. `write_stdin` does not run PreToolUse again; arbitrary MCP tools and hosted
tools are outside this matcher. Stop requests another verification turn, not a permission denial.

References: [OpenAI packaging](https://developers.openai.com/plugins/build/plugins),
[developer commands](https://learn.chatgpt.com/docs/developer-commands), and
[hook contracts](https://learn.chatgpt.com/docs/hooks).

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

Select the phase by its output, not the detected language: explore for facts, vet for a dependency
decision, plan for requirements/tasks, code for implementation, verify for execution evidence and
audit for read-only findings. `arc-policy skills` lists the actual bundled entry paths and versions;
`arc-policy skills --name arc:verify` resolves the same entry as `arc-verify`. Discovery reads entry
metadata; activate only the chosen skill and its relevant references. Optional specialists such as
`ui-design`, `check`, platform skills and `lark` come from the host's own available inventory.

```bash
arc-policy task begin --skill arc-code
# agent writes code and commits freely during implementation
git -C <repo> add <owned files>
git -C <repo> commit -m "incremental change"
# when ready to assert verified task completion:
arc-policy task verify
# a fresh PASS receipt is required for verified delivery claims, not for commits
```

If this repository explicitly activated `.arc/arc.db`, close the separate
resource task **once**, after verification—not on every commit. Set its
versioned state to `verifying` through legal `arc task transition` steps,
then run `arc sync reconcile --task <policy-task-id>` (if not already
reconciled by PostToolUse), `arc index flush`, and finally
`arc task complete --id <policy-task-id> --expected-version <N>`.
The completion command reads the existing Arc Policy PASS; it never launches
a second test run. An `AUTHORIZED` waiver does not count as PASS. A failed
index/changed tree/incorrect task version blocks completion, not normal Git
commit/push. The Python runtime remains authoritative for verification decisions, but
after `arc policy state migrate` its active task, history and receipts are
persisted only in `.arc/arc.db`. In an active shared-DB
business graph, the receipt fingerprint binds the activation manifest and
graph configuration, **not** unrelated task/Outbox bytes inside `arc.db`;
`receipt_matches_tree` still re-executes the independent graph gate so a
real business failure invalidates the claim. Legacy dedicated graph ledgers
continue to bind their raw DB and WAL fingerprints.

Move between existing Arc phases with an atomic handoff:

```bash
arc-policy task status
arc-policy task switch --skill arc-verify
# If verification was intentionally not completed in the old phase:
arc-policy task switch --skill arc-code --incomplete --reason 'Repair the recorded failing claim'
```

The old task and receipt stay in history, with the next task's ID and handoff status; the new task
gets no inherited receipt. An incomplete switch requires a reason. A normal switch needs a fresh
or delivered receipt, and the same canonical skill cannot silently restart itself. `task begin`
never replaces unfinished work by default; legacy explicit `--force` archives the replaced task.
Colon and dash forms of the six Arc names also match historical receipts and business graph gates.
Read-only, plan-only or graph-only work retains its user scope: do not execute project tests just
to satisfy bookkeeping. Preserve its artifact and use a reasoned incomplete handoff when runtime
verification is outside the authorized scope; this does not claim business PASS.

When verification cannot pass, normal Git commits and pushes can still proceed.
Do not assert verified delivery. If the user explicitly authorizes **closing or
delivering** the task without verification, use
`arc-policy task authorize --quote '<the user's words>'`; its `AUTHORIZED`
receipt records that decision without claiming that tests passed. On hosts that
send a transcript (Claude Code, Codex), `PreToolUse` refuses a quote that is
not in one of the user's last 5 typed messages. This authorization is
**not** required for routine commits or pushes.

A task is delivered once HEAD has moved past its PASS or AUTHORIZED receipt and the changes that receipt
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

Starting a task now removes any **Arc-owned** legacy `pre-commit` and
`pre-push` verification gates, without touching another tool's hooks or custom
`core.hooksPath`. The old `arc-policy install-git-hooks` command is a
backward-compatible cleanup operation. Updating the persistent plugin runtime
and running `arc-policy task begin` on a repository removes the old gate.
If hook cleanup fails, it reports the configuration error rather than
modifying an unrelated hook. There is no Arc verification hook on Git commit
or push. Other repository Git hooks continue to run normally.

Project overlay: `.arc/policy.yaml` (merged on top of `policies/default.yaml`).

Verification checks every detected affected Go, Rust, Python and JavaScript test project,
including nested modules and untracked files. Rust defaults to all test targets, without
`--lib`. Missing runners, missing JavaScript `scripts.test`, and failed commands cannot produce a PASS. Python uses the existing
project virtual environment when available. On macOS, automatic Rust tests are `BLOCKED`:
Rust is not compiled on the Mac, not even with `cargo zigbuild`; declare test commands that run
on the remote build host.

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
a `locked` diagnostic. State and receipts are saved atomically (to Turso after Policy cutover);
`.arc/policy/`, `.arc/artifacts/` and the generated arc-idx files
`.ai-code-index/tags.json`, `.ai-code-index/daemon.pid`, `.ai-code-index/daemon.log` are excluded
from snapshots and session write attribution. Index refreshes therefore do not invalidate a receipt
or repeat a Stop notice. Other files in `.ai-code-index/`, including `config.json`, and arc-idx
source remain included. Project policy remains included even when gitignored. Older receipts without
a content digest, or whose digest predates this exclusion, must be verified again.

### Stop hooks

The default `block_on_claim` mode uses TypeSafe through the existing `jev ask` CLI to judge the
last assistant message. It needs `jev` on PATH and `TYPESAFE_API_KEY` or `~/.typesafe/api_key`.
Only the final assistant message and completion examples are sent, not session history.
`completion_markers` are examples for the semantic judgment.
Clear completion claims block the stop and send the agent to `task verify`; negative statements,
questions and progress notes stay silent; Git commits do not trigger this task-completion gate.
When TypeSafe cannot judge (no `jev`, missing credentials, invalid response, timeout, uncertain
answer), `completion_markers` check positive occurrences outside fenced code, inline code and
blockquotes. A negated occurrence is ignored without hiding a separate positive claim. Neither path
creates a verification receipt.

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
than 14 days are pruned). At Stop, changes made since then need a matching receipt **only for a verified completion claim**,
whether an edit tool, the shell or a script wrote them, and even when `task begin` came later; a
repository without a task is reported as changed without one. Cursor's `workspace_roots` are all
checked. When several repositories report, the strictest `stop.mode` applies.

Allowed edit tools record their explicit target paths even when `PostToolUse` is missing.
Known inspection-only shell calls (such as `git status`, `arc-policy task status`, `arc-idx`
queries and plain file reads) open an empty attribution window: they do not acquire another
session's concurrent writes. Commands with output redirection, write options, prefix wrappers
or opaque scripts keep conservative write windows.

For a possible writing shell call, Stop charges dirty files whose mode, size or mtime changed
between `PreToolUse` and `PostToolUse` (paired by `tool_use_id`, else tool input). A missing
finished event leaves its window open until Stop. Writes by another session during such a
window cannot be distinguished, and a background write after it closes may lack attribution.
Legacy sessions with neither explicit targets nor tracked windows retain first-touch fallback;
their old attribution cannot be reconstructed by an upgrade. No session logs or task receipts
are cleared to silence a notice. Verify actual owned changes and report unrelated failures.

Shell guards check every command segment, including command substitutions, unquoted heredoc
substitutions, `sh -c` / `bash -c` scripts, `eval`, and prefix wrappers (`timeout`, `nohup`,
`sudo`, `nice`, `time`, `exec`, `xargs`, `env`, `command`). Heredoc bodies are data, so prose with
apostrophes no longer trips the parser. Shell input the host truncated (Grok, over 128 KiB) is
denied because its hidden part cannot be checked. `workdir` and Git `-C` resolve against the
target repository's policy. Run verification separately before claiming task completion or performing protected migrations, not before each commit.
Controlled commands using shell groups, `cd`, `env -C` / `--chdir` or `sudo -D` are refused
because their directory scope is not interpreted; run them separately with `git -C` instead.

The engine fails closed: if it cannot load or crashes, edits and controlled operations are denied
with the error and other tool calls proceed with it as a message. The DSH adapter denies a
tool call when the engine cannot answer at all. Claude Code, Codex and Grok ignore a hook that
times out; `PreToolUse` therefore allows 30 seconds (it normally takes about 50 ms), and the Git
explicit verification still guards completed-task claims; no Git-commit verification hook is installed.

If Stop reports an unverified task, run `arc-policy task status`, then `arc-policy task verify`
in that task's repository. Fix any `FAIL`, `BLOCKED` or `NOT_RUN` evidence before claiming delivery.

## What is actually forced

| Constraint | Mechanism | Strength |
|---|---|---|
| Discover the skill | Plugin + SKILL.md | Guidance only |
| Enter the workflow | `PreToolUse` denies writes in a repository with no `task begin` there | Host-enforced, bypassable |
| Controlled ops | Guard protected schema migrations; deny `--no-verify` / commit `-n` to avoid skipping unrelated Git hooks | Host-enforced, bypassable |
| Agent commit / push by any route | No Arc verification receipt required; retired Arc Git hooks are removed on task begin | No per-commit gate |
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
  skills/arc-{explore,plan,code,verify,audit,vet}/  generated from Arc/
  skills/arch-diagram/         pinned official renderer, LICENSE and source.json
  hooks/hooks.json            shared Claude / Grok contract
  hooks/codex.json            explicit strict Codex contract
  hooks/hook.py
  hosts/cursor/hooks.json     Cursor Agent user hooks
  hosts/dsh/                  DeepSeek Harness Cordis plugin
  policies/{default,go,rust}.yaml
  lib/arc_policy/             policy engine
  bin/arc-policy
```

The six Arc skills (including `arc-vet`) are authored only in `Arc/`. Their generated copies
and reachable references/scripts ship inside this plugin. Do not hand-edit the generated
copies. `scripts/sync_skills.py` remains the standalone skill publisher; plugin users should
avoid simultaneously enabling independent copies of the same six skills in other discovery roots.

```bash
.venv/bin/python scripts/build_arc_plugin.py
.venv/bin/python scripts/build_arc_plugin.py --check
```

The root `plugin.json` is the version source. The builder synchronizes host descriptors,
marketplaces and runtime version, validates resource closure, and rejects source drift,
missing files, extra generated files, escaping symlinks and executable-mode changes.
The generated artifacts are checked in so installing directly from Git is complete.

After upgrading to 0.3.0, finish or restart existing CLI sessions: session-log filenames now
hash the full session ID instead of collapsing different IDs by replacement/truncation.
Old session logs are not reused; repository task/receipt data keeps its existing format.

Release checks distinguish package/contract tests from native host tests. This change was
verified with isolated copies, pinned Codex schemas, Python entrypoint subprocesses and actual
Git commits. No Claude/Grok/Codex executable was available in the build environment, so native
installation, trust UI and end-to-end model turns remain `NOT_RUN` rather than a claimed PASS.

### Phase-aware graph gate

When graph verification is configured, `arc:verify` MUST satisfy the local
`arc-verify gate` before a PASS receipt. This does **not** block
`arc:code` repair work solely because the pre-fix E2E graph is red:
its normal compile/test/quality gates remain mandatory, and the old graph
failure remains FAIL until a subsequent `arc:verify` run proves the fix.

Other skills do not receive a false "business graph PASS"; they are simply
not authorized to assert the graph's runtime coverage. A project may
explicitly opt additional skill phases into this gate:

```yaml
verification:
  business_graph:
    graph: academic-v1
    db: .arc/verify/data.db
    require_for_skills: [arc-code]
```

`arc-verify` can never be removed from the required phases. This avoids a
verification ↔ repair deadlock without weakening `arc:verify` claims.
