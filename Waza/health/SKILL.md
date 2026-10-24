---
name: health
description: >-
  Read-only health checks for agent instructions, runtime configuration, permissions, MCP tools,
  and installed skill discovery. Use for Codex/Claude/Pi configuration drift, ignored instructions,
  broken tool setup or missing skills. Code quality and release readiness belong to arc:audit.
---

# Health: Agent Environment

Explain which configuration or instruction layer causes the observed agent behavior, using current
evidence. This skill audits configuration and installed capabilities; code risks and maintainability
belong to `arc:audit`, while a requested repair continues through `arc:code` under the same task.

Adapted from [tw93/Waza](https://github.com/tw93/Waza), under the [MIT license](../LICENSE).

## Scope

- Inspect only the requested runtimes, project instruction entrypoints, permission settings,
  relevant MCP definitions and installed skill paths.
- Stay read-only: do not change settings, install packages or hooks, refresh fixtures, run project
  builds/tests, restart services, enable tools or repair links as part of the audit. A bundled
  explicit repair request is a separate authorized phase, not a reason to stop after the report.
- Do not collect raw conversations, session logs, memory stores, credentials or unrelated project
  history by default. A user-supplied symptom or sanitized excerpt can guide the checks. An explicit
  history-analysis request needs its own bounded scope and privacy handling.
- Report missing tools or inaccessible settings as `unverified` with the missing evidence.
  Neither an unavailable probe nor a large number of skills proves a defect.

## Collect the relevant facts

Read [code-intel](../../Arc/_shared/code-intel.md) before discovering project files. Repository
file/text discovery uses `arc-idx`; do not run a bundled collector, `find`, `os.walk`, `rglob` or
another scanner to inventory the project. Follow the user's stricter index identity/fallback rules.

For machine-global configuration outside repositories, use the active host's documented config
location and narrowly scoped discovery. Inspect only fields needed for the reported behavior.
Never print a full settings file or secret-bearing values. Report key names, whether a required
variable is set, redacted path roles and the affected source location; do not print token prefixes.

Record the current runtime version and the source of its loading rules. Consult current official
documentation when precedence, supported permission keys or instruction discovery is uncertain;
do not infer that different runtimes load the same directory names or support the same deny rules.

## Check by evidence layer

| Layer | What to establish |
|---|---|
| Instruction reachability | Which project/global entrypoints this runtime actually loads; whether a necessary domain rule is reachable when relevant; whether duplicated guides disagree |
| Configuration precedence | Global, project and local overrides; the effective setting behind the symptom; ignored/private overlays distinguished from distributed project guidance |
| Permissions | What the runtime can enforce, what instructions permit, and which layer produced a denial or unexpected prompt |
| MCP/tool state | Enabled state, required environment-variable presence and one relevant harmless read call; distinguish configuration failure from live tool failure |
| Installed skills | Actual installation path and source revision; discovery name, duplicate names, broken links, missing referenced resources and source-versus-installed drift |
| Discovery budget | Per host: entry count, entries listed by name only (description dropped from the skill listing), real-directory copies beside symlinks, the same name in two roots, nested `SKILL.md` below a package root, test fixtures, global instructions naming skills that are not installed |

Use only checks relevant to the requested scope:

- An allowlist entry for push and a rule requiring explicit push authorization can coexist:
  permissions control the runtime prompt; instructions control whether the action is requested.
  Do not weaken the user's chosen permissions or add repeated approval prompts as an audit fix.
- Recommend only controls the runtime supports. A shell prefix rule, a process sandbox and a
  pre-execution hook match different operations; explain the actual layer and its tradeoff.
- A pipe-to-shell detector may match quoted examples too. Do not install one or claim reliable
  blocking from a naive glob. Similarly, do not label task-scoped `.env` access defective when
  the user's policy permits it while forbidding disclosure.
- Probe only relevant, enabled MCP servers using documented harmless reads. Disabled servers are
  not failures. A tool error is evidence about that tool; a reviewer timeout is not an MCP failure.
- For skill installation issues, compare installed files and resolved reference targets with the
  intended pinned revision. A floating source is an update/reproducibility risk, not proof of an
  exploit. Never run an unfamiliar skill's script merely because it is installed.
- In the Skills repository, `scripts/link_hosts.py --check` compares every host root with
  `schemas/host_profiles.yaml` and `scripts/sync_skills.py --check` validates installed packages;
  `scripts/route_probe.py` measures description-only routing. Elsewhere, list each root's entries
  and resolved targets directly.
- Before declaring a hook absent, use its documented diagnostic state or a safe scoped probe.
  A prompt or diff panel can obscure hook output; visual absence alone does not prove nonexecution.

## Report and handoff

Give each finding a concrete symptom, source location, effective layer, supporting observation,
impact and exact next diagnostic or fix. Distinguish `confirmed`, `likely` and `unverified`;
use the project's severity scheme. Counts, file sizes and estimated token shares do not establish
critical severity. A clean result is valid when the checked scope and remaining gaps are explicit.

Label machine-global findings `global` and report them once. Before an authorized later repair,
re-read the file because another session may have changed it; never have concurrent agents write
the same global configuration. Carry the current findings, scope and authorization through
[Arc handoff](../../Arc/_shared/workflow.md). Configuration audit evidence is not a code-quality
verdict and does not qualify a release.
