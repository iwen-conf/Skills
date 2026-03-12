# BypassAIGC Adaptation Notes

Source examined on 2026-03-12:
- Repository: <https://github.com/chi111i/BypassAIGC>
- README snapshot: <https://raw.githubusercontent.com/chi111i/BypassAIGC/master/README.md>

This reference distills the parts of that project that are useful for a compliant `arc:aigc` workflow.

## What to reuse

| Pattern | How `arc:aigc` uses it |
|---|---|
| Long-text segmentation | Break manuscripts into paragraph-aligned chunks so each pass stays reviewable and traceable. |
| Two-pass processing | First improve chunk-level clarity, then run a document-level cohesion pass. |
| Configurable model/provider workflow | Treat model choice as replaceable infrastructure, not part of the skill contract. |
| Context compression | Carry forward a short summary of prior chunks instead of replaying the entire history. |
| Reviewable logs | Record what changed and why, rather than returning only the final polished text. |

## What not to reuse

| Excluded behavior | Reason |
|---|---|
| Claims about "bypassing AI detection" | This creates dishonest-use pressure and is outside the allowed purpose of `arc:aigc`. |
| Academic cheating or authorship disguise | The skill is for honest editing, not misrepresentation. |
| Admin/backend implementation details | The skill needs the workflow, not the original product architecture. |
| Provider-specific marketing or pricing assumptions | Those are unstable and not core to the method. |

## Protected-span checklist

Before rewriting, mark these as protected or review-required:

- Citations and bibliography anchors
- Equations, formulas, and variable names
- Numbers, percentages, dates, and thresholds
- Named entities, dataset names, and method names
- Quotes, legal clauses, and compliance wording
- Code snippets, file paths, and command examples

## Suggested pass structure

### Pass 1: chunk rewrite

For each chunk:

1. State the local goal: clarity, concision, flow, or terminology cleanup.
2. Rewrite without changing evidence, citations, or quantitative claims.
3. Log what changed and any uncertainty that needs human review.

### Pass 2: document polish

Across the full draft:

1. Normalize terminology and section transitions.
2. Check for duplicated claims or inconsistent definitions.
3. Confirm citations, numbers, and equations still point to the same evidence.
4. Produce a short reviewer checklist for unresolved issues.

## Style heuristics that are safe to apply

- Vary sentence openings and clause rhythm.
- Replace generic filler with discipline-specific wording already supported by the source text.
- Merge or split sentences to improve readability.
- Tighten logical connectors so each paragraph has a clear function.

## Heuristics that require explicit caution

- Reframing claims of novelty or contribution
- Softening uncertainty language around results
- Reordering evidence-heavy paragraphs
- Collapsing literature review passages that contain multiple citations

When in doubt, prefer a visible note in `revision-log.md` over a silent rewrite.
