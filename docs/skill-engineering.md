# Skill Engineering Contract

Arc skills follow progressive disclosure: `SKILL.md` carries the whole decision procedure for one
kind of work, and references load only when a mode needs them. The Android, HarmonyOS, CNB, and
Lark directories are bundled vertical skills with their native contracts; they are not converted
into Arc's structure.

## Discovery / Activation / Execution

1. **Discovery**: hosts inject `name` + `description` only. Descriptions answer WHAT and WHEN in
   the third person, include English and Chinese trigger terms, and end with a
   `Not for … (use X)` clause naming the adjacent skills.
2. **Activation**: the matched `SKILL.md` loads; its first workflow step reads
   `Arc/_shared/contract.md`.
3. **Execution**: `## Intent Router` picks the mode; only that mode's reference loads.
4. **Repair/diagnosis**: any skill that diagnoses a suspected defect or proposes a behavior fix
   loads the evidence-first root-cause repair section of [`execution-truth.md`](execution-truth.md).

## Required Arc `SKILL.md` surface

- Frontmatter: `name`, `description` (80–1024 chars, contains "Not for"), `version` (semver).
- Exactly these H2 headings, in order, none empty: Trigger, Non-trigger, Goal, Inputs,
  Intent Router, Workflow, Tool Strategy, Decision Rules, Verification, Failure Recovery,
  Completion Criteria. Headings inside code fences do not count.
- `## Intent Router`: markdown table with at least two data rows.
- Budgets: `SKILL.md` ≤ 3,500 estimated tokens (non-CJK characters ÷ 4 + CJK characters) and
  ≤ 500 lines; every reference and `_shared/` file ≤ 3,500 estimated tokens and ≤ 500 lines.
  Documentation inside a bundled tool package under `scripts/` is held to the line budget only.
- No retired skill names, no nested `SKILL.md`, every relative link resolves.
- Vague wording (`as needed`, `when relevant`, `按需`, `视情况` …) is reported as a warning.

## Writing rules

- One prohibition list per skill, in Tool Strategy (Forbidden) and Decision Rules.
- Decision points are IF/THEN with thresholds; every workflow step yields an observable output;
  every failure class has an action, a retry budget and a stop condition.
- No provenance or motivation text in runtime files; that belongs in `CHANGELOG.md` or `docs/`.
- Cross-cutting rules live once in `Arc/_shared/` or `docs/execution-truth.md` and are linked.

## Scripts

Deterministic work stays in scripts: `scripts/validate_skills.py`, `Arc/arc:code/scripts/gate.sh`,
`Arc/arc:audit/scripts/security-scan.py`, and the dbqual package under
`Arc/arc:verify/scripts/dbqual/`. Do not ask the model to re-derive line counts, trigger terms, or
JSON writes.

## Test gates

```bash
.venv/bin/python scripts/validate_skills.py
.venv/bin/python -m pytest tests -q
```

Trigger corpus: `schemas/trigger_corpus.yaml`. Every Arc skill needs ≥ 3 `must_contain` terms (all
present in its description), ≥ 8 positive and ≥ 3 negative utterances. Positives must uniquely win;
negatives must not select that skill. Utterances of retired skills stay in the corpus under their
new owner as a regression set. This keyword scorer is a **static** vocabulary gate; runtime
scenarios (negation, multi-intent, scope lock, authorization) live in
`schemas/behavior_corpus.yaml` and must not be "solved" by hard-coding sentences into the scorer.

## Lifecycle coverage

The five Arc skills map to the phases understand (`arc:explore`), plan (`arc:plan`), change
(`arc:code`), prove (`arc:verify`) and evaluate (`arc:audit`). A phase has one main skill; work that
crosses phases hands off by name.
