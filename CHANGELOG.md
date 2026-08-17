# Changelog

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
