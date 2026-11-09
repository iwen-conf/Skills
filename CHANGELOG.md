# Changelog

## Unreleased

### arc 0.4.3 / arc-policy 0.4.8 — `arc idx` command group

- Code-index commands are written `arc idx search|symbol|files|ast|stats|doctor|index|daemon|watch`, like `arc verify …`. The bare forms (`arc search …`) still work for running sessions but are no longer documented; `arc index flush` stays an arc.db command. The engine's usage, errors, hints and doctor header say `arc idx …` (`arc idx ast: ast-grep:`).
- `arc idx` queries count as read-only shell calls. Skills, docs, READMEs and the dsh host notes use the `arc idx` form.

### Stop no longer blames a session for other sessions' tasks

- A session started in a parent folder (several repositories side by side) was blocked at Stop for every child repository with an unverified task, including tasks and changes of other sessions it never touched. A tracked session (one with a session ID) now skips repositories where none of its tool calls ran; hosts without a session ID keep checking all of them.

### arc 0.4.2 / arc-policy 0.4.7 — agents never see the engine name

- The code-index engine prints `arc …` in usage, errors, hints and the `doctor` header (`arc ast: ast-grep:` replaces `arc-idx ast: ast-grep:`); its identity reads `arc index engine …`, which never matches `arc 0.4.x`, so an `arc` alias to the engine is still detected. `arc index flush` and the missing-engine error name the engine role instead of its file.
- `arc:explore`, `code-intel`, `arc-db`, the CNB/Android runtime docs, the arc-policy skill and the dsh host notes name only `arc`.

### arc 0.4.1 / arc-policy 0.4.6 — `arc` is the only callable entry

- `scripts/install_arc.sh` publishes only `arc` to the bin directory; the `arc-idx` and `arc-verify` engines move to the private `<prefix>/libexec/arc` and stale public copies are removed, so agents cannot call the backends by name. `arc` resolves engines from `libexec/arc`, then beside itself, then `PATH`.
- `arc-policy doctor` asks `arc capabilities --json` for engines and runs `arc verify selftest`. `arc_idx_on_path`/`arc_verify_on_path` are replaced by `arc_on_path`, `code_index_ready` and `verify_engine_ready`; `workflow_ready` uses `code_index_ready`.
- `arc search|symbol|files|ast|stats|doctor|version` count as read-only shell calls, like the same `arc-idx` queries.
- `code-intel` drops the "use `arc-idx` without `arc`" fallback: a missing `arc` is reported as a broken install.

### Non-Arc skills call the public `arc` entry

- CNB, Waza (`health`, `research-write`), HarmonyOS, Lark `lark-workflow-project-docs` (2.0.1) and the Android `perfetto-sql` / `play-billing-library-version-upgrade` skills now write `arc search|symbol|files|ast|doctor|index` instead of the `arc-idx` backend. HarmonyOS `arc-idx-integration.md` is renamed `arc-integration.md` and points at `arc:explore` instead of the retired `arc:idx` skill.

### arc-policy 0.4.5 — fewer false hook errors in Grok, Claude Code, Cursor and Codex

- Shell text the parser cannot read no longer turns into `arc-policy hook error: No closing quotation` and a denial. All 49 such Grok/Claude calls found in local logs (unclosed quotes that `bash -n` also rejects) are now left to the shell; text that may hide `--no-verify`/`-n` on commit/push or a protected migration is still denied, with the parse error and a fix hint.
- Stop skips touched repositories that were deleted before the turn ended (temporary worktrees), instead of reporting `[Errno 2] No such file or directory`.
- Cursor runs both its native `~/.cursor/hooks.json` hook and the hooks of imported Claude Code plugins, so every event was judged twice, sometimes by an older cached version (the retired per-commit gate came back that way). The Claude entry now returns nothing for Cursor payloads when a native `hosts/cursor/run.sh` hook is configured; `run.sh` marks itself with `ARC_POLICY_CURSOR_NATIVE=1`.
- `host_profiles.yaml` declares `plugins: [arc-policy]` for Claude, Codex and Grok. `link_hosts.py` no longer links the six Arc skills the plugin already ships, reports leftovers as `plugin-duplicate`, and `--apply` moves them to `~/.skills-backup/`. Each host now discovers one copy of each Arc skill instead of two.

### arc-policy 0.4.4 — one public `arc` entry across skills, docs and installs

- Skill references now call `arc search|symbol|ast|files|stats` instead of the `arc-idx` backend; `arc-idx` remains only as a backend name, a labelled fallback and literal engine error text. `code-intel` makes an `arc` alias or symlink to `arc-idx` an explicit broken-install signal.
- Graph docs (`business-graph`, `business-graph-start`, `workflow` §6) select the active `.arc/arc.db` after migration and the legacy `.arc/verify/data.db` otherwise; `arc-db.md`, READMEs and AGENTS.md report arc 0.4.0 with `arc policy state` implemented.
- Removed the `.github/workflows/arc-cli.yml` workflow that violated the no-GitHub-Actions rule; its Go tests and real-backend smoke moved into the AGENTS.md acceptance commands.
- Dropped the dead `unverified_commit` fallback category and updated hook tests that still expected plain `git commit` to be denied; `--no-verify` stays denied, including when the engine cannot load.
- `tests/conftest.py` clears the `ARC_POLICY_ROOT` exported by the installed launcher, so the suite passes under `arc-policy task verify` instead of resolving fixture entries to the installed runtime.
- `tools/arc` builds again: a corrupted `'\n'` rune in `writeManifest` had made every 0.4.0 commit uncompilable, and two test files did not compile. Fixed `retireLegacyPolicyFiles`, where a shadowed `err` returned before archiving `.arc/policy/receipts`; Policy migration now actually moves legacy receipts into `.arc/backups/`. Test fixtures resolve symlinked temp roots like `repositoryRoot` does.
- `arc-verify` checkpoints its WAL (`PRAGMA wal_checkpoint(TRUNCATE)`) on close. Every write used to leave an uncheckpointed `data.db-wal`, so `arc db migrate --confirm-offline` always refused a real legacy ledger; the real-backend smoke now passes end to end.
- `arc-idx` doctor advice and search errors now tell agents to run `arc index` / `arc daemon ...` instead of the backend command.
- Restored the `arc-idx` trigger term in `arc:explore` and brought every load scenario back under its token ceiling. Skill versions: explore 2.3.4, code 2.7.3, plan 2.6.2, audit 2.7.1, vet 1.1.1.

### arc:verify 3.9.0 — tester-army/e2e execution guidance

- Added an on-demand Web/mobile runner reference, reviewed against upstream `232ce75ab4b68ded93525162e1121bf973792ea8`: atomic agent goals plus exact assertions, readable-state negative checks, fixture/session boundaries and evidence-first debugging.
- Connected it to the existing E2E and graph-card workflow. Per-attempt output, independent readback and full report inspection preserve skipped/flaky cases and original FAILs; cache/healing, mobile providers and secret handling have explicit evidence limits.


### Arc 0.4.0 / arc-policy 0.4.3 — Policy task and receipt physical cutover

- Introduced `arc policy state get|put|status|migrate` using the existing
  local Turso driver and `.arc/arc.db`. The canonical document persists active
  task, task history and optimistic version, with immutable, indexed receipt
  records and audit events in the same database transaction.
- Explicit `--confirm-offline` migration imports old Policy JSON and archived
  PASS/AUTHORIZED receipts, makes a SHA-256-verified backup and replaces the
  old `state.json` with a directory tombstone to prevent legacy writers from
  silently re-creating a second authoritative state. Receipt files move to
  `.arc/backups/`. Offline migration can be rerun to finish interrupted
  archival; it never overwrites current DB state.
- Python Arc Policy Runtime now reads/writes through the unified `arc` CLI
  after cutover, using CAS for state changes. Backend failures fail closed;
  neither task mutations nor verification receipts fall back to JSON.
  Workspace child-task discovery includes migrated repositories.
- `arc task complete` validates the live Policy receipt from the Turso row
  under Policy and Arc locks, not stale `state.json`. Completed-task gates,
  business-graph verification and the rule against per-commit verification
  retain their existing semantics.
- Added Go migration/conflict/receipt-integrity tests and Python DB-routing,
  stale-writer, workspace and no-fallback regression tests. Cross-language
  Windows locking remains intentionally blocked until implemented.


### Arc task-completion integrity — policy lock and receipt recheck

- `arc task complete` now uses the same `.arc/policy/task.lock` coordination as
  `arc-policy`, taking that lock before `arc.db` to match graph-verification
  lock ordering. The live receipt ID and canonical contents are rechecked
  before committing a revision-scoped `done` state.
- Added regression tests for revoked, replaced and wrong-task receipts and a
  competing policy lock. Windows completion remains fail-closed until an
  equivalent `msvcrt` byte-range lock adapter is implemented.


### Arc CLI 0.3.0 — verified task/index closure

- Added `arc index flush`: bounded outbox watermark and retry counts, one
  real `arc-idx` full rebuild, doctor health check and worktree-fingerprint
  recheck before acknowledgement; a failed or concurrent build leaves source
  events pending. Generated Zoekt/ctags metadata is excluded from Git source
  observations. This is **batched full indexing**, not per-file indexing.
- Only file/ChangeSet events enqueue index work; task lifecycle events remain
  in the durable journal without causing redundant index rebuilds.
- Added `arc task complete` and `arc task transition --to done` guarded by
  expected-version CAS, an already existing current Arc Policy PASS (not
  `AUTHORIZED`), source snapshot and confirmed index Outbox. No per-commit
  verification is introduced.
- Persisted completion receipt fingerprint, source revision and check verdicts
  in append-only `task.completed` events. `arc task show` reports historical
  source freshness without claiming independent tests were rerun.
- Fixed a shared-store receipt feedback loop: Arc Policy now fingerprints an
  active graph's cutover manifest and config instead of the entire mutable
  `arc.db` file. Independent graph gates are still rechecked on receipt use;
  old standalone graph ledgers retain byte/WAL fingerprinting.
- Added mocked failure/retry/race tests, first-index and generated-metadata
  tests, receipt forgery/waiver rejection and guarded CLI completion tests.
  Real backend regression is still required before production cutover.


### Arc physical-store integration — task attribution and readback

- Connected activated `.arc/arc.db` ChangeSets to the **stable active Arc Policy task ID**:
  PostToolUse now idempotently imports the task using `arc task ensure`, then
  calls `arc sync reconcile --task <id>` (or the guarded legacy update path).
- Added `arc file show` and bounded `arc event list` so file fingerprints
  and append-only state transitions are inspectable without directly querying
  Turso tables.
- Avoided unnecessary subprocess/database synchronization for proven read-only
  shell commands; opaque or potentially mutating calls still reconcile.
- Added Go task identity/CLI readback tests, Plugin hook identity/readonly tests,
  and real-backend smoke steps. Task completion remains independently gated;
  code index outbox remains pending until an actual indexing acknowledgement.


### Arc CLI 0.2.0 — opt-in Turso arc.db state migration

- Added namespaced `arc_*` resource schema and explicit `arc db init|migrate|status`.
  Migration requires an offline assertion, SHA-256 backup and pre/post graph
  proof parity before activating `.arc/arc.json`; legacy graph data remains intact.
- Implemented versioned task transitions (CAS, no unverified `done`), file
  fingerprints and task-file edges, idempotent Git ChangeSets, transactional
  events and outbox records, plus bounded `arc update --changed-only`.
- Integrated active-project PostToolUse reconciliation and cutover-aware
  `arc-policy` graph gates. Missing shared-DB serialization now blocks
  verification instead of falling back to unsafe direct legacy DB access.
- Added Go state/migration regression tests, focused Python hook/gate tests
  and a dedicated GitHub Actions workflow. Live integrated results remain to
  be confirmed; index-outbox replay and evidence-backed task completion are
  not yet implemented.

### arc 0.2.x — migration safety, Git replay, and plugin reconciliation

- Implemented the opt-in Turso-backed `.arc/arc.db` resource lifecycle and
  unified `arc task/file/change/update` commands. The existing `arc-verify`
  graph ledger can be migrated only offline with backup and gate parity;
  `arc-policy` receipts remain independently enforced.
- Strengthened cutover validation: identical empty/invalid gate output, a
  contradictory exit status, or `PASS` without graph-scoped unchanged-input
  proof can no longer activate the new database.
- Fixed Git rename collection so the old file path is invalidated along with
  the destination, and added tests for nested untracked files.
- Added `arc sync reconcile` with durable `arc_meta.sync_head`: reconcile
  committed changes missed by Agent hooks; cursor and change set commit together.
- `arc-policy` PostToolUse now prefers Git replay, with an older-CLI
  dirty-worktree fallback only for explicitly unsupported commands.
- Added regression cases for idempotent replay, CAS/state transitions, migration
  proof validity, and plugin synchronization. End-to-end execution against
  the actual target environment still requires an available Go/Turso toolchain.


### arc-policy 0.4.2 — remove verification per Git commit/push

- Removed the default unverified-commit/push receipt gate from PreToolUse policy,
  Git pre-commit/pre-push hooks, session guidance, and commit/push skill instructions.
- Normal `git commit` and `git push` no longer trigger or require
  `arc-policy task verify`. Starting or switching a task cleans up retired
  Arc-owned Git hooks and restores any preserved user hook scripts.
- Retained explicit `arc-policy task verify` for task completion/delivery,
  business graph gates for required verify phases, and schema-migration
  checks; `--no-verify` remains blocked to protect unrelated Git hooks.
- Updated policy and Git-hook integration tests to cover default commit
  permission, old hook cleanup, custom hook preservation, and explicit gates.


### Unified arc CLI 0.1.0 — executable compatibility stage

- Added the public `arc` Go CLI: code-index commands delegate to `arc-idx` and
  `arc verify ...` delegates to `arc-verify`. Added `arc capabilities --json`,
  exact argument/exit-code forwarding tests and sibling-binary resolution.
- Added `scripts/install_arc.sh` for building the three current executables
  under one installation directory; Skill workflows use the `arc` public CLI.
- Updated `arc-policy` to prefer `arc verify gate` and keep a fail-closed legacy
  fallback when `arc` is absent, with tests for both routes.
- Physical database consolidation into `.arc/arc.db` remains unimplemented;
  existing Turso graphs, evidence, task receipts and policy gates are preserved.


### arc 0.1.0 — unified public CLI (compatibility bridge)

- Added `tools/arc/` with a public `arc` binary routing existing search/index commands
  to `arc-idx` and `arc verify ...` to the `arc-verify` engine, retaining backend
  exit codes and reporting available capabilities.
- Updated Arc Skill sources to show `arc` / `arc verify` while leaving legacy
  executables and the `arc-policy` graph gate untouched. `arc.db` is now the
  explicit target for **both** resource state and business graph verification.
- Resource commands, engine fusion, database migration and hook cutover are **not**
  implemented by this bridge; its capabilities report that limitation.


### Arc skills — arc.db design protocol (documentation only)

- Named the future repository-local state database `.arc/arc.db` and specified typed CLI resources,
  guarded state transitions, idempotent change events, incremental plugin/Hook reconciliation,
  and crash recovery in `Arc/_shared/arc-db.md`.
- Linked the feature-gated design from `arc:explore`, shared code-intel, task documents and
  business graph references. Existing `arc-idx` CLI, `arc-policy` receipts, and `arc-verify`
  `.arc/verify/data.db` remain unchanged; these new commands are not shipped yet.


### Install — drop leftover typesafe-ai extra

- Removed missing-source `typesafe-ai` from `schemas/host_profiles.yaml`, the install catalog, and `route_corpus.yaml` externals. Classifier work routes to `arc-code`. `jev` and the TypeSafe skill package are not installed on this machine.

### Arc skills — graph-handoff integration with existing workflow

- Connected `arc:explore`, `arc:plan`, `arc:code`, `arc:verify`,
  `arc:audit` and `arc:vet` to the single existing handoff record and
  local short-lived `arc-verify handoff` CLI, preserving each phase's authority.
- Scoped the business graph gate to `arc:verify` by default, so a red graph
  cannot deadlock authorized repairs, while preserving fail-closed verify claims.
- The bridge never turns inferred risks into approved business rules,
  duplicates the task tracker or treats source discovery as real E2E evidence.


### arc:verify 3.3.0 / arc-policy 0.2.0 — local graph-based business chain evidence

- Introduced a lightweight Go `arc-verify` CLI using the official in-process `tursogo` Go driver, no cloud service or daemon.
- Added versioned business graph nodes/edges, user-reviewed contract RULE vs non-gating RISK, executable task cards with actor/entry/conditions/oracle and multi-step RAW trace with first-divergence semantics.
- Augmented `arc-policy task verify` with an opt-in project graph gate and automatic fail-closed checks when a project has `.arc/verify/config.jsonc`. Receipt revalidation checks changed local DB/WAL and referenced evidence.
- Added academic enrollment example, verification reference and separate incomplete integration work packages. Real Turso/database and browser E2E remain unverified.


### arc-policy — drop the OMP / PI host adapter

- Removed `hosts/omp/`, `.omp-plugin/`, and the OMP/PI `package.json` native-extension manifest. Remaining hosts are Claude Code, Codex, Grok, Cursor Agent, and DeepSeek Harness.
- Plugin-root discovery no longer reads `OMP_PLUGIN_ROOT`.

### arc-policy — tasks follow the edited file's repository

- Fix: `PreToolUse` looked up the task in the session directory's repository, not the edited file's. A Grok session in the non-Git workspace `ditie` was denied `search_replace` on `A133BMS/…` although `A133BMS` had an active task, then wrote the files through Python. The reverse leaked too: a task in the working repository allowed edits in any other repository.
- Edit gates now resolve every target (`file_path`, `path`, `notebook_path`, `target_file`, apply_patch headers) to its Git repository and need a task in each; paths outside every Git work tree are not gated, because `task begin` needs Git. Denials name the repository and give `arc-policy --cwd <repo> task begin …`.
- In a non-Git parent workspace, `SessionStart` lists the active tasks of child repositories and `Stop` checks them; Stop reminders name the repository and its `--cwd` command. The plugin skill, DSH and OMP prompts state that a denied edit must not be routed through the shell.

### arc-policy — closing the remaining bypasses

- Git-level gate: `task begin` (or `arc-policy install-git-hooks`) registers config-based `pre-commit` / `pre-push` hooks (Git 2.54+) beside existing hook files. When an agent marker is set (`CLAUDECODE`, `GROK_AGENT`, `CODEX_THREAD_ID`, `CODEX_CI`, `CURSOR_AGENT`, `PI_CODING_AGENT`, `AI_AGENT`, `ARC_POLICY_AGENT`; each confirmed in host sessions or source), a commit or push by any route needs a matching receipt. Humans are not gated.
- Stop covers every repository the session touched: `PreToolUse` records edit targets and shell-reachable repositories (directory, `cd` / `-C` targets, absolute paths in commands and script bodies) with first-touch digests under `~/.cache/arc-policy/sessions/`. Changes written through Python or the shell, or before a late `task begin`, now block a delivery claim; repositories without a task are reported. Cursor `workspace_roots` are all checked; the strictest `stop.mode` wins.
- Shell parser: heredoc bodies are data. A body such as `Don't` used to deny the whole command with "No closing quotation". Command substitutions (`$(…)`, backticks, unquoted heredoc substitutions), `sh -c` / `bash -c`, `eval`, and prefix wrappers (`timeout`, `nohup`, `sudo`, `nice`, `time`, `exec`, `xargs`) are now parsed for commits and migrations. Grok input truncated past 128 KiB is denied.
- Grok `hashline_edit` and `EditNotebook` (`target_notebook`) edits are gated; before this they went through unchecked.
- Fail closed: the hook entry denies edits and controlled operations when the engine cannot load (for example, PyYAML missing) or crashes. The OMP and DSH adapters deny when the engine cannot answer at all; until now they allowed. They also send a per-process session id and export `ARC_POLICY_AGENT=1`. `PreToolUse` timeouts are raised from 10 s to 30 s, because Claude Code, Codex and Grok ignore a hook that times out. During a 15:00–15:14 system stall on 2026-10-24, Grok skipped seven calls this way.

### arc:audit 2.6.0 — learning from Cloudflare security-audit

- Compared `arc:audit` appsec with Cloudflare's `security-audit` skill (`cloudflare/security-audit-skill` at `c1c8a8c`, MIT) and adapted its method: asset rows name the trust boundary and strongest control; Phase 4 works from a concrete invariant with sibling-path equivalence, sad inputs, variant search and a candidate gate (principal, input, control, boundary, affected resource, concrete result; no claim stronger than the evidence); off-repository facts and unsandboxed reproduction become `manual-gap`; only `confirmed`/`likely` cards carry severity, with a High/Medium discriminator; cards gain a stable fingerprint and boundary line; fixes name the invariant at the last trusted decision point; `coverage.md` becomes per-row `surface × boundary × class` with prior-run carry-forward rules; anti-patterns listed.
- New `references/attack-classes.md`: nine ordinary classes and ten boundary-selected domains (HTTP and identity, browser, data isolation and lifecycle, supply chain and CI, cloud, RPC and messaging, resource exhaustion, desktop/mobile IPC, memory safety, AI/LLM agents). Upstream MIT license kept at `references/upstream/security-audit-LICENSE`.
- Description adds threat model, source-first pen test and the surface list; health mode now screens all 41 code-rot items (was 36). `appsec-campaign` load ceiling 15,000 → 19,000 for the new reference.
- `docs/01-系统设计/04-安全审计上游学习.md` records the comparison, what was absorbed or deliberately left out (JSON findings schema and Node validators, multi-agent budgets and artifact promotion, out-of-repo output) and the procedure to re-check upstream changes.

### Installed skill cleanup

- Removed `yab-browser` (app missing; MCP server removed from user config), upstream `lark-suite` and the standalone `lark-workflow-project-docs` copy (repository `lark` router installed instead), the `open-source-vetting-kit/` source (superseded by `arc:vet`), and the disabled `typesafe@typesafe-ai` and `ponytail@ponytail` Claude plugins with their marketplaces and caches.
- Cloudflare `security-audit` withdrawn from Claude, Codex, Grok, DSH and Cursor and its local copy deleted; upstream GitHub stays the learning source for `arc:audit`, and `~/.dsh/AGENTS.md` routes explicit security audits and pen tests to `arc:audit` appsec. Retired copies `~/.agents/skills-retired-arc-*` and `~/.agents/waza-retired-*` deleted.
- `host_profiles.yaml` drops the unused Gemini and Pi hosts; `link_hosts.py --check` is OK for Claude (19), Codex (21) and Grok (9). Skill backups (`~/.agents/skills-backup-*`, `~/.skills-backup/`) and the stale Claude `arc-policy` cache deleted; README states backups are cleaned manually.

### arc:vet 1.0.0 — open-source due diligence

- New sixth Arc skill `Arc/arc:vet/` (installed as `arc-vet`), rewritten from the standalone `open-source-vetting-kit` into the Arc structure: select · check · replace modes; local reuse first via `arc-idx`; at least three comparable candidates or a stated shortfall; dated, linked evidence per candidate; maintenance tiers (12 / 24 / 36 months) with vetoes for archived, deprecated, unlicensed and unfixed-advisory projects; an evidenced stale exception that never applies to cryptography, authentication, payment or network-facing parsers; a decision ladder (reuse → adopt → borrow the design → build); records at the project ADR location or `docs/10-复盘决策/`; handoff to `arc:code` in the same turn when implementation is authorized.
- `references/search-playbook.md` (keyword families, `gh api` queries, manual confirmation, successors, supply-chain hygiene) and `references/decision-template.md`.
- `scripts/github_health.py`: standard-library GitHub precheck with `--as-of`, `GITHUB_TOKEN`/`GH_TOKEN`, bot-commit exclusion, maintenance tier, deprecation hint, license and release warnings; a pure `assess()` covered offline by `tests/test_arc_vet.py`.
- Contract §1 authority row and §2 rule (a new third-party dependency needs an `arc:vet` record first); workflow owner row and `vet → code → verify` path; `LIFECYCLE_PHASES` gains `select`; new `vet-select` load ceiling (9,000); trigger, route and behavior corpora, host profiles (core, Grok) and registry updated.
- Remove `apifox-cli`: host profile external target and Codex extra, route corpus roster and case, skill catalog row; Arc API-contract, task-docs, docs-layout and task-template text no longer name Apifox (the rule stays: no API-platform sync unless the user asks; the Markdown contract is the authority).
- arc-policy: `arc-vet` accepted by `task begin` (alias `open-source-vetting`), listed in deny hints, the DSH prompt and the plugin skill, and exempt from the stack-skill architecture check like the other non-coding phases.

### arc-policy 0.1.0 — Coding Agent Policy Runtime

- New plugin at `plugins/arc-policy/`: portable `plugin.json`, Claude `.claude-plugin/plugin.json`, Codex `.codex-plugin/plugin.json`, Grok/Claude/Codex marketplaces.
- Policy engine (`lib/arc_policy`) plus CLI `bin/arc-policy`: `task begin|status|verify|end`, `detect`, `doctor`, `hook`.
- Default mode is free source edits after `task begin`; `git commit`/`push`, `--no-verify`, and schema-migration CLIs need a tree-bound receipt from `task verify`.
- Host hooks: `SessionStart` injects stack and skill, `PreToolUse` denies, `Stop` reminds or blocks delivery claims. Same-user hooks are documented as process control, not a security boundary.
- Host adapters for OMP (`hosts/omp/arc-policy.ts` + `.omp-plugin/`), Cursor Agent (`hosts/cursor/hooks.json`, Cursor `permission` replies), and DeepSeek Harness (`hosts/dsh` Cordis plugin). Hook JSON dual-emits Claude `permissionDecision` and Cursor `permission`.

### arc:verify 3.2.0 — properties and model conformance

- New `references/model-conformance.md` and Intent Router row (write mode): pick the cheapest disproving method (property, exhaustive enumeration, stateful model, conformance, linearizability, bounded proof); property shapes derived from contract and closure-spec invariants with their traps; generator, shrinking, seed and replay rules per cadence; property and state-machine APIs for rapid, fast-check, Hypothesis, proptest/proptest-state-machine and jqwik.
- Closure spec → tests: states, transitions, guards, `ignores`, `race_policy`, resources, invariants and timers each map to a command or assertion; the model gate must pass first, a transition hit ledger replaces iteration counts, and divergences split into `FAIL` (to `arc:code`), `SPECIFICATION GAP` (to `arc:plan`) or adapter defects. Existing TLA+ (trace validation), Quint (ITF traces) and Alloy (instances as fixtures) specs feed the same adapter; model-check and code-conformance verdicts stay separate.
- Concurrent conformance via linearizability checking (Porcupine `Unknown` is `BLOCKED`); Kani bounded proofs for Rust with unwinding failures `BLOCKED`; Verus/Dafny stay with `arc:code` because they change production source.
- `case-design.md` §4, `flow-model.md` §6 and `stacks.md` link the new reference; Go `testing/quick` is marked frozen and non-shrinking. Description gains 属性测试 and 模型一致性 (trigger corpus updated); new behavior scenario `model-divergence-is-not-a-model-edit`.
- SKILL.md keeps the cross-mode procedure and leaves mode detail to the mode references (3,496 → 3,345 tokens; `pg-reliability` load 15,498 → 15,347): the release waiver rule and GO/NO-GO, flaky and fuzz evidence rows defer to release-ops §2–§3 and fuzz-campaign §8, which already state them; the gap-terminal rule defers to flow-model §3; the ledger-failure recovery row and flow evidence move into the flow-model §1 Check row; the PostgreSQL routing rule folds into the db router row; the browser fallback defers to web-e2e and stacks. A "Mode evidence" Verification row points to each mode's verdict section.

### arc:verify 3.1.0 — business-flow-first testing

- New flow mode: T1/T2 test work starts from the business flows a change reaches. `references/flow-model.md` (catalog at `docs/07-测试质量/业务流/`, discovery from real entry points, Mermaid chart subset with `Nxx` decisions, `Bxx` branches and `Txx` terminals, closure kinds 正常/拒绝/失败/恢复/异步/缺口, flow card, mapping to closure-spec IDs) and `references/flow-matrix.md` (risk ranking, A–H dimensions, branch → case, pre/post state diff, branch ledger, layers, change impact, gap routing).
- `scripts/flow_ledger.py check <catalog>` fails on unlinked or missing flow files, charts with unreachable or dead-end nodes, decisions with fewer than two branches or bare yes/no conditions, ledger rows that differ from the chart, illegal verdicts, and verdicts without reason or evidence; warns on gap terminals and Critical/High branches without PASS. `tests/test_flow_ledger.py` (17 cases, 12 structural mutations).
- Workflow step 2 is "flows first"; generating T1/T2 cases from an endpoint, function or file list is forbidden; unimplemented recovery is charted as a gap terminal. Contract and workflow give verify ownership of the flow catalog; arc:plan closure-design and docs-layout point to it. New `verify-flow` load ceiling (14,000).
- ARC's own toolchain is the first catalog: 16 flows inventoried, 5 modeled (install, host links, closure gate, completion gate, database release gate) with 83 ledger branches. It records a confirmed defect: `dbqual/scripts/release_gate.py` returns PASS with exit 0 when the policy declares no required evidence or metric rule (empty or misspelled key), while `canary_evaluator.py` treats the same case as BLOCKED.
- Fix dbqual gates that check nothing: `release_gate.py` validates the policy against `schemas/gate-policy.schema.json` and fails (`invalid_policy`, `empty_policy`, exit 2) on unknown sections or selector fields, non-list sections and policies without any assertion; before, `{}` or a misspelled section passed with exit 0 and an object-shaped section crashed. `qualification_coverage_gate.py` fails a manifest without claims. db.md maps `empty_policy` to Arc `BLOCKED`.
- `sync_skills.py`: a rollback that fails itself now raises an error naming the backup that still holds the previous entries. `link_hosts.py --apply` stops at the first OSError and prints the entry and backup path instead of a traceback.
- New tests: `tests/test_link_hosts.py` (5), overlap/empty-selection/dry-run/argument cases in `tests/test_sync_skills.py`, red-without-baseline, vanished baseline head and exit-127 cases in `tests/test_gate_script.py`, unusable-input and render cases in the closure selftest. Each characterization test was mutation-checked in a disposable copy. All 83 ledger branches of the 5 modeled flows now PASS.

### arc:verify 3.0.0, arc:code 2.6.0 — business-expectation testing and fail-closed evidence

- Fix two false `PASS` paths in the bundled dbqual package (reproduced offline, before `PASS` → after `FAIL`/`BLOCKED`): `run_suite.py` archives earlier evidence and writes `run-manifest.json` (run id, HEAD, changed-path and config digests); every record carries `run_id`; `release_gate.py --root` re-checks the candidate fingerprint and rejects other-run, unreadable or duplicate evidence; `regression_compare.py` is `BLOCKED` on a missing baseline or manifest, missing environment snapshot, absent workload or zero effective comparisons. Siblings closed: coverage gate no longer skips unreadable files, baseline promotion only takes the gated run, canary with no metrics is `BLOCKED`, pgbench parses only the current invocation's logs. 61/61 package tests, selftest with negative controls.
- `arc:verify` states its purpose up front (tests prove business expectations; green is not the goal) and adds red mode (acceptance tests before code), test-value review (`references/test-value.md`), browser E2E with `scripts/with_server.py` and recon-before-act (`references/web-e2e.md`), and hostile-input fuzz campaigns (`references/fuzz-campaign.md`). Reliability minimums move to `txn-faults.md` §0.
- SQL verification: new `sql_static_audit.py` (evidence `sql.static`) parses and lints SQL with SQLFluff in the configured dialect and lints migrations with Squawk; parse errors, gated rules and unexcluded Squawk findings fail, nothing checked is `BLOCKED`. `gate.sh check` gains `lint(sql)` for changed `.sql` files under the project's sqlfluff config (and Squawk with `.squawk.toml`). New `references/sql-static.md` (what parse, lint, schema-bound EXPLAIN and migration lint each prove) and `references/txn-design.md` (transaction boundaries, forbidden anomaly → mechanism, per-engine isolation semantics for PostgreSQL, InnoDB, SQL Server, Oracle, CockroachDB, Spanner, SQLite; retry ownership). Verified against real sqlfluff 4.4.0 and squawk 2.68.0.
- `arc:code` adopts the red-before-code iron law (`Arc/_shared/test-first.md`): `gate.sh red -- <cmd>` refuses when production files changed before the first assertion-level red or when the command passes; `gate.sh check --require-red` re-runs every red and warns on oversized stubs and tests edited after their red.

### T08 full-catalog consolidation — Arc 2.5.0 (breaking)

- Collapse 162 installed skill directories (204 on-disk copies) into 22 entrypoints: Arc 5, check, health, research-write, routers `lark`, `android`, `ui-design`, `cloudflare`, `threejs`, merged `cnb`, `harmonyos`, `lazycat`, and six external tools. Design and probe data: `docs/01-系统设计/02-全量技能架构评审.md`.
- Merge `arc:closure` into `arc:plan` model mode; closure docs move to `docs/01-系统设计/业务闭环/` with legacy `docs/arc/` kept; checker under `arc:plan/scripts/closure/`.
- Remove the body Trigger section from Arc skills; demote `workflow.md` to handoff-only; split `coding-discipline.md` out of `execution-truth.md` (§1–§4, §6–§7, §11 keep their numbers) and `go-backend-structure.md` out of `go-backend.md`; add `ARC_LOAD_SCENARIOS` ceilings (Go repair 18.1K → 15.1K, full-stack API 22.3K → 17.5K estimated tokens).
- Add code-rot family G (over-engineering, from ponytail) and `arc:verify/references/web-perf.md`; arc:code consumes `DESIGN.md` and routes semantic-judgment features to TypeSafe.
- Waza: read, learn, write and AnySearch become `research-write` (fetch/search/research/edit); `check` covers branches and PRs on any host, absorbing cnb-code-commit's safe-branch steps.
- Packager: `metadata.library` routers bundle sources as `library/<name>/ENTRY.md`, refuse standalone installs of members, reject nested `SKILL.md`, skip `node_modules`, and tolerate directory links. Fix cnb-pipeline's validator and references that were never packaged.
- Add `scripts/link_hosts.py` + `schemas/host_profiles.yaml` (host install governance) and `scripts/route_probe.py` + `schemas/route_corpus.yaml` (63-case cross-family routing regression via TypeSafe Jev; 63/63 passing).
- Rewrite global instructions to current names (6.6K → 2.4K tokens); retire fixtures, stubs, ponytail, stale copies and drift copies to `~/.skills-backup/2026-10-08-T08/`.

### Earlier unreleased

### Arc 2.4.0 native client direction

- Replace Expo/mobile, Tauri/desktop and Taro/mini-program defaults with a single target-native policy shared by planning, implementation, audit and verification; scope Web state, theme, API and scaffold rules to Web.
- Preserve scoped maintenance of existing cross-platform products; require explicit migration scope. Add native Apple/desktop/mini-program verification guidance, retain native Android/HarmonyOS tools, and remove ArkUI-X as a default direction.

### Arc 2.3.0 API design review

- Extend arc:audit with api-design mode for contracts and implementations; verify primary standards online and distinguish design defects, implementation defects, specification gaps and evidence gaps.
- Share sourced HTTP, JSON field and upload rules with planning, implementation and verification. Record units, provenance, enforcement and failure behavior for limits; keep protocol requirements, security guidance and project choices distinct.
- Add API boundary evidence guidance and behavior scenarios without creating another skill, scanner or arbitrary global limits. Plan/code/audit/verify move to 2.3.0; explore/closure retain 2.1.0.

### Waza selection and Arc 2.2.0 references

- Review Waza at `d8fa34ff50a37043d40bbd3b9e622029a12dd37b`; manage adapted read/write/learn/check/health sources in `Waza/` with MIT license closure. Keep check for authorized delivery and health for agent configuration; omit duplicated source scanners and implicit conversation collection.
- Merge think/hunt/ui increments into Arc planning, regression evidence and screenshot iteration. Retain the existing Arc authority, root-cause and runtime evidence gates; retire the three competing installed entries with backups.
- Connect explicit read network proxy configuration to the local extractor without enabling third-party extraction. Preserve Gollum wikilinks and add an explicit Obsidian dialect to punctuation checks. Bundle read/write when installing learn.

### Arc 2.1.0 — business closure and shared handoffs

- Add `arc:closure` from `Arc/arc-closure`, with design/change/audit/gate modes and complete packaged assets and scripts.
- Connect all six skills through one handoff contract: preserve requirement/model IDs, source facts, task status, evidence freshness and existing authorization; route gaps back to their owner without recursive skill loading.
- Keep read-only closure audits in artifacts; distinguish finite-model checks, business decisions, implementation and runtime qualification. Generic planning and cosmetic edits do not require a closure model.
- Close warning/waiver gate bypasses; bind model evidence to current spec and checker fingerprints and require complete enumeration. Provide explicit checker-copy refresh without overwriting existing project documents.
- Extend lifecycle/trigger/behavior corpora and test real installed closure initialization, checking and gating under dash and colon names. Install the six-skill family through the existing atomic backup/rollback packager.

### Arc 2.0.0 — architecture consolidation (breaking)

Design record: [ARC 技能架构重构评审](docs/01-系统设计/01-ARC技能架构重构评审.md).

- Replace the 16 Arc skills with 5, one per kind of work: `arc:explore` (from `arc:idx`), `arc:plan` (`arc:define` + `arc:clarify` + `arc:sdlc`), `arc:code` (`arc:build` + `arc:fix` + `arc:frontend` + `arc:arch` + `arc:comment` + `arc:trace`), `arc:verify` (`arc:test` + `arc:reliability` + `reliability-engineering-skill` + the external `arc:dbqual` package), and `arc:audit` (`arc:audit` + `arc:security`, plus a new diff/PR review mode). No aliases; the old → new map is in `Arc/README.md`, and the retired skills' trigger utterances stay in the corpus under their new owner.
- Move `arc:docs` to the Lark family as `lark-workflow-project-docs`; the `.lark.json` format is unchanged. Drop the thesis evidence checklist.
- Remove `arc:prewalk` (requires mid-session model swap that hosts do not provide); its design record moves to `docs/01-系统设计/prewalk.md`. Remove the external `ponytail` preflight; the simplicity intent is covered by `execution-truth.md` §9.
- Add the undiscoverable shared layer `Arc/_shared/`: `contract.md` (authority, T0–T2 tiers, scope lock, one vocabulary for verdicts/status/severity with a single severity → P0–P3 mapping, artifacts, environment, subagent delegation, Lark hook, commit format, 5-line report), `code-intel.md` (arc-idx preflight, query ladder, new 8-surface impact checklist, zero-hit widening, major-version-only check), `code-rot.md`, `api-contract.md`, `finding-campaign.md`, and `conventions/` (Go backend, frontend, color tokens, comments).
- Import the PostgreSQL release-qualification package as `arc:verify` db mode (`scripts/dbqual/`, gates G0–G31). Its 1,109-line guide is split into `references/db.md` and `references/db/*.md`; its Chinese README moves to `docs/dbqual/`. The package's unit suite runs in the repository test suite.
- Replace the six unreferenced `Arc/scripts/*.sh` with `arc:code/scripts/gate.sh`: baseline-aware completion gate that scans only lines changed since `gate.sh start`, runs non-mutating format checks and lint/typecheck for every detected stack, affected-package tests, and a scope check.
- Every Arc `SKILL.md` uses the fixed headings Trigger → Non-trigger → Goal → Inputs → Intent Router → Workflow → Tool Strategy → Decision Rules → Verification → Failure Recovery → Completion Criteria, stays within 3,500 estimated tokens, and names adjacent skills in a "Not for" clause. The validator enforces order, non-empty sections, token budgets for references and `_shared/`, retired-name references, nested `SKILL.md`, and Arc directory layout; vague wording is a warning. The root-cause gate applies to `arc:code`, `arc:verify` and `arc:audit`.
- Delete `Arc/templates/`, `docs/arc-routing-matrix.md`, `docs/arc-routing-cheatsheet.md`, `docs/conductor-pattern.md` and `docs/orchestration-contract.md`; `Arc/README.md` holds the single routing table. Move `docs/code-rot-taxonomy.md` to `Arc/_shared/code-rot.md` and archive `reliability-engineering-skill/` under `docs/archive/`.
- `gate.sh` behavior versus the old scripts: the file-count check warns above 20 files (was FAIL above 5) and fails only when `--scope` is given; formatters run in check mode only (`gofmt -l`, never `go fmt ./...`); whole-project lint (`npm run lint`, `ruff check .`, `staticcheck`) is replaced by typecheck of changed packages; a missing test command is `NOT_RUN`, not a failure; the destructive-command blocklist moves to contract §6 approval.
- `arc:audit` scan script: semgrep uses `--config p/default` (`auto` with `--metrics off` always exited 2, so semgrep never ran); schemathesis 4 uses `--url`; gitleaks uses `gitleaks git`. Default output moves from `.arc/security/<timestamp>/` to `.arc/artifacts/<task-slug>/security/` (`--task-slug`; a rerun with the same slug overwrites).
- `code-intel.md` query templates corrected against `arc-idx` 1.0.0: Go method callers use a text query (`ast` cannot match Go selector calls), JSON-tag and field-write regexes fixed, `--max` truncation and per-command zero-hit output documented. `arc-idx --help` now shows the real `ast` argument order and `--format` for `files`, `stats` and `doctor`.
- Frontend conventions updated to current toolchains: Tailwind v4 with `shadcn@latest init -t vite -b radix`, `sonner`/`field` components, NativeWind pinned to Tailwind 3, Taro CLI pinned to `@4`.

- Apply the 2026-10-05 full-audit disposition to all 80 entries: correct CNB base/partial-review contracts, R8 execution and missing-data failure, disabled Compose clicks, server-verified authentication, JSON stage logs, nested calendar conflicts and event completion semantics. Preserve the eight entries for which the audit recommends no substantive change.
- Consolidate default stack, workspace, Android execution, CNB runtime and Lark auth/authorization contracts; use scoped workflows, finite failure exits and references for optional detail. `arc:docs`, `arc:frontend`, `arc:prewalk` and `arc:trace` receive minor contract versions; other changed Arc entries receive patch versions. No runtime installation or publication is performed by this revision.
- Validate the standalone reliability source distribution separately: 80 checked entries, 79 default runtime packages. Add executable regression cases for the audit counterexamples and repair transitive packaging links; static checks and local examples do not claim remote/API or agent end-to-end verification.

- Add `arc:reliability` 1.1.0, adapted from the supplied reliability-engineering skill and 13 primary industry/expert sources: risk-based scope, independent business oracles, real transaction/fault/recovery evidence, mixed-version and rollback validation, and incident controls. Include source limits and a worked inventory/payment-notification example; reuse native execution and remote infrastructure without a source-runner dependency. Connect build/fix/audit/sdlc routing and lifecycle/root-cause registration.
- Upgrade `arc:test` to 1.2.0 with risk-based case design, explicit untested scope, generator/oracle review and bounded defect-sensitivity checks. Remove generic coverage/E2E percentages and test-type priority defaults; retain project-owned gates, independent functional/performance verdicts and required claims when flaky tests are quarantined. Add trigger and runtime scenarios for these decisions.
- `scripts/sync_skills.py` now stages a complete package (file links plus declared `metadata.requires.skills`), validates it, then replaces one host name (`--naming dash|colon`) and archives the other alias. Missing resources or publish failures leave the destination unchanged and restore from the backup. Shared docs are generated into `references/_shared/`.
- Keyword trigger tests are labeled as a static vocabulary gate. Runtime negation, multi-intent, scope, and authorization scenarios live in `schemas/behavior_corpus.yaml` and are not solved by hard-coding sentences into the scorer.
- Task tiering has a single authority in `arc:sdlc`: ordinary multi-step work stays on an in-session plan; only large / cross-session / tracked work creates persistent docs. Search fallback is `arc-idx` first; `rg`/`fd` only when the index cannot cover the target.
- Import CLI-embedded Lark meeting skills with `scripts/import_lark_skills.py`. Compatibility entries `lark-minutes` / `lark-note` / `lark-vc` / `lark-vc-agent` route to `lark-meeting`.
- Make repository Markdown API contracts the implementation and acceptance baseline; replace the mandatory Apifox source rule with explicit-request-only downstream sync. Add field/behavior/verification templates and connect `arc:build` 1.2.0, `arc:arch` 1.2.0, and `arc:frontend` 1.1.0 to the shared contract.
- `arc:sdlc` 1.2.0 requires Markdown task docs with business-group → separate frontend/backend work-package → executable-leaf hierarchy. Assess D1–D4 difficulty separately from P0–P4 priority, record reasons and dependency readiness, split complex work by acceptance boundaries, and isolate high-uncertainty exploration. Update task/progress/audit templates and status rules, including mandatory `<Context-Queries>` and independent frontend/backend verification.
- `arc-idx version` now prints the full identity line (`arc-idx 1.0.0 — local AI context engine (zoekt + ctags + ast-grep)`) instead of the bare semver, so AGENTS/CLAUDE identity checks can compare one command's output; `arc:idx` 1.1.0 makes the identity check, per-session `daemon status`/`start`, and the no-tree-walk red line explicit, and syncs the repo source with the stricter installed copy.
- Add cross-skill coding-discipline gates to `docs/execution-truth.md`: think-before-coding (state assumptions, present competing interpretations and simpler alternatives, ask/autonomy boundary, verifiable success criteria first), the simplicity ladder (skip > reuse > stdlib > platform-native > installed dependency > one line > minimal code, with `debt:` ceiling comments and never-simplify boundaries), surgical edits (no adjacent "improvements", orphan-only cleanup, per-line traceability, bounded elegance pass), and the self-improvement lessons loop (`tasks/lessons.md` + persistent memory).
- Wire the new gates into `arc:build`, `arc:fix` (autonomous evidence-driven repair, no user hand-holding), `arc:clarify` (present interpretations, ask/autonomy boundary), `arc:arch` (ladder vs required DIP boundaries), and add explicit task tiering (trivial / in-session bounded todo / persistent docs) plus re-plan-on-drift to `arc:sdlc`; bump the five skills to 1.1.0.
- Add a subagent usage strategy to `docs/orchestration-contract.md`: delegate read-heavy research/exploration to keep the main context clean, one subagent per task, no silent swallowing of failed/partial results.
- Add one evidence-first root-cause repair gate: confirm the issue before changing production behavior, allow only bounded evidence instrumentation while unconfirmed, trace the owning architecture and shared invariants, compare the real business contract, implement the smallest complete fix, and verify analogous paths.
- Require repair-capable Arc and vertical entrypoints to load that gate; preserve multi-inline-code router values during index generation.
- Enforce the 500-line progressive-disclosure budget and relative-link checks for bundled Android, HarmonyOS, CNB, and Lark entrypoints without forcing Arc frontmatter on them.
- Split the oversized HarmonyOS and Android Intent Security entrypoints into concise routers while preserving their complete guides under `references/`.
- Keep `skills.index.json.generated_at` stable when generated content has not changed, avoiding timestamp-only worktree and commit noise.

## 1.0.0 — 2026-08-17

Breaking Skill engineering upgrade. No historical compatibility.

- Treat every `SKILL.md` as a progressive-disclosure router: required `## Intent Router`, `## Red Lines`, `## When to Use`; body ≤ 500 lines.
- Descriptions are WHAT+WHEN, third person, 80–1024 characters, with mandatory trigger terms from `schemas/trigger_corpus.yaml`.
- Drop `enforce_arc_profile`, `expert_keywords`, and heading-laundry validation. Split oversized skills (`arc:sdlc`, `arc:arch`, `arc:docs`, `arc:comment`) into modules.
- Discovery index no longer embeds full section bodies. Automated trigger tests cover ≥8 positive and ≥3 negative utterances per skill.
- Authoring contract: [`docs/skill-engineering.md`](docs/skill-engineering.md).

## 0.2.0 — 2026-08-10

### Publish channel

- Add `scripts/sync_skills.py` to install SSOT `Arc/arc:*` into `~/.agents/skills` as both colon and dash aliases.
- Set `version: 0.2.0` on all Arc `SKILL.md` frontmatter.
- Document validate → index → sync flow in `README.md`.

### Content (anti-drift)

- Add [`docs/execution-truth.md`](docs/execution-truth.md): environment/branch/deploy truth, completion definition, scope lock, domain keys, paper-over bans, cheap-executor task authoring.
- `arc:sdlc`: completion definition, explicit non-goals, downstream task template gates.
- `arc:build` / `arc:fix`: environment surface preflight, scope lock, no matrix-only done claims.
- `arc:arch`: common drift patterns (domain services dump, temp var flags, type asserts, invented identity keys).
