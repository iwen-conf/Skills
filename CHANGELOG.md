# Changelog

## Unreleased

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
