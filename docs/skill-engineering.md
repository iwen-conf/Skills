# Skill Engineering Contract

Arc skills follow progressive disclosure. `SKILL.md` is the router; modules and references load only after a route hits.

## Discovery / Activation / Execution

1. **Discovery**: hosts inject `name` + `description` only. Descriptions must answer WHAT and WHEN, third person, with concrete trigger terms.
2. **Activation**: the matched skill's `SKILL.md` loads. It must stay ≤ 500 lines.
3. **Execution**: follow `## Intent Router` and load only the named module or reference.

## Required `SKILL.md` surface

- Frontmatter: `name`, `description` (80–1024 chars), `version` (semver).
- `## Intent Router`: markdown table, at least two data rows.
- `## Red Lines`: enforceable bans for this skill.
- `## When to Use`: include an exclusion boundary.

Retired: `enforce_arc_profile`, `expert_keywords`, required Announce / Expert Standards heading laundry.

## Scripts

Deterministic work stays in scripts (`scripts/validate_skills.py`, `Arc/scripts/*.sh`, skill-local scanners). Do not ask the model to re-derive line counts, trigger terms, or JSON writes.

## Test gates

```bash
.venv/bin/python scripts/validate_skills.py
.venv/bin/python -m pytest tests -q
```

Trigger corpus: `schemas/trigger_corpus.yaml`. Every skill needs ≥8 positive and ≥3 negative utterances. Positives must uniquely win; negatives must not select that skill.

## Lifecycle coverage

The 15 `arc:*` skills cover define, clarify, design, plan, implement, search, verify, secure, repair, operate, document, and same-session cost routing. Release/deploy notes live under `arc:sdlc` `docs/06-环境运维`; do not add a parallel release skill.
