# Changelog

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
