---
name: arc-aigc
description: "学术/专业文本去模板化润色：当用户提到“AIGC 味太重/论文像机器写的/需要按段改写长文稿/保留引用公式重写表达”时触发；用于分段重写、双阶段统稿、作者声线统一与引用保真检查，不用于规避检测或学术作弊。"
---

# arc-aigc — evidence-based academic polish

## Overview
`arc-aigc` absorbs the reusable mechanics of `BypassAIGC` into a compliance-first writing workflow: split long text into bounded chunks, protect citations/equations/numbers/named entities, perform a chunked rewrite first, then run a two-stage polish for document-level cohesion. The goal is to reduce template-like phrasing and improve readability while keeping claims, evidence, and attribution stable.

For the repo-derived method and what is intentionally excluded, read [`references/bypassaigc-adaptation.md`](./references/bypassaigc-adaptation.md) only when you need the detailed pass structure or protected-span checklist.

## Quick Contract
- **Trigger**: The user asks to polish, restructure, or clarify academic prose (paper, thesis chapter, technical report) with explicit requests for chunked handling, dual-phase review, or citation consistency.
- **Inputs**: source manuscript (`target_text` or file path), academic context (research question, venue, tone), optional policies (citation style, forbidden edits), and polishing depth controls.
- **Outputs**: `polished-text.md`, `revision-log.md`, `session-plan.md`, `stage2-summary.md` (see Outputs).
- **Quality Gate**: Each chunk must attest to maintained claims, citation integrity, and explicit reviewer notes; double-check stage-2 summary before finalizing.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix
- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Tip** of this skill `## When to Use` shall prevail.

## Announce
Begin by stating:
> "I am using `arc-aigc` to drive a chunked, two-stage academic polish with explicit notes for each rewrite pass."

## Teaming Requirement

- Every execution should clarify `Owner`, `Executor`, and `Reviewer` responsibilities before rewriting starts.
- In a single-agent environment, keep an explicit decision-execution-review perspective so the final text is not delivered without an internal review pass.

## The Iron Law
```
NO POLISH IS WORTH FACT DRIFT, CITATION LOSS, OR MISREPRESENTED AUTHORSHIP.
ALL CHANGES MUST BE TRACEABLE BACK TO ORIGINAL CLAIMS AND CITATIONS.
```

## Workflow
1. **Set the integrity boundary**: confirm the text type, target audience, citation style, and disclosure constraints. If the request is framed as detector evasion, authorship disguise, or academic cheating, reframe it as honest clarity/style editing or refuse.
2. **Protect high-risk spans**: mark citations, equations, tables, numbers, named entities, code snippets, legal clauses, and quoted passages as protected or review-required before any rewriting starts.
3. **Plan chunks**: split the manuscript into paragraph-aligned chunks large enough to preserve local logic but small enough to review carefully. Save `<session>/session-plan.md` with chunk id, boundaries, protected spans, and stage-1 goals.
4. **Pass 1 — chunked rewrite**: perform a `chunked rewrite` on each chunk to reduce stock phrasing, improve transitions inside the chunk, and clarify argument flow. Log what changed and why in `revision-log.md`.
5. **Pass 2 — document polish**: run a `two-stage polish` across the full structure to normalize terminology, transitions, headings, and narrative cadence. Use prior chunk notes to keep the whole text aligned to the requested `authorial voice`.
6. **Drift check**: compare the rewritten text against the source for numbers, claims, citations, and definitions. Flag anything that may indicate `semantic drift` instead of silently smoothing it away.
7. **Finalize with review**: output the polished draft, unresolved questions, and a mandatory `human review` checklist for the user or downstream editor.

## Quality Gates
- Track every chunk rewrite in `revision-log.md` with chunk id, original excerpt, rewrite excerpt, intent, and cite-handling metadata.
- Preserve `citation fidelity`: references, equations, numbers, and named entities must remain intact unless the user explicitly approves a change.
- Do not invent new data, claims, citations, or literature support. If evidence is missing, flag it as a gap.
- The stage-2 review must explicitly state whether terminology, section logic, and transitions are now consistent across the document.
- Before delivery, state whether any residual risk of `semantic drift` remains and what the reviewer should inspect manually.

## Expert Standards
- Use `chunked rewrite` rather than one-shot global paraphrase when the source text is long or citation-dense.
- Run a `two-stage polish`: first improve local sentences and paragraph flow, then reconcile terminology and transitions across the whole document.
- Preserve `citation fidelity` by reusing exact citation anchors, equation identifiers, and quantitative claims unless the user explicitly approves a correction.
- Calibrate `authorial voice` deliberately: adjust cadence, stance markers, and sentence openings without erasing discipline-specific precision.
- Check for `semantic drift` after every major pass; do not trade precision for surface variety.
- End with a `human review` checklist so the final authority remains with the author, editor, or reviewer.

## Scripts & Commands
- Runtime main command: `arc aigc`
- Method reference: `Arc/arc-aigc/references/bypassaigc-adaptation.md`
- Session plan: create `<project>/.arc/aigc/<session>/session-plan.md` with chunk bounds, protected spans, and pass goals.
- Stage passes: write chunk-level changes to `revision-log.md`, then perform the document-level cohesion pass and summarize it in `stage2-summary.md`.
- Hand-off: deliver the assembled directory with `polished-text.md`, `revision-log.md`, and explicit follow-up questions for the reviewer.

## Red Flags
- Requests to remove citations, erase plagiarism markers, or explicitly bypass AI-detection or integrity review systems.
- Asking to invent or fabricate new experimental results, numbers, references, or authorial experience.
- Treating stylistic variation as more important than factual precision, citation preservation, or reviewer transparency.
- Ignoring the chunk plan and stage-2 summary, which leads to untraceable "musical rewrite" across the draft.

## Mandatory Linkage (cannot be fought alone)

1. If the writing goal, target venue, tone, or integrity constraints are still vague, call `arc-clarify` first instead of guessing rewrite objectives.
2. If the user is really asking for multi-stage orchestration across writing, review, and delivery, route through `arc-exec` rather than keeping all coordination inside `arc-aigc`.
3. If the manuscript contains disputed claims, compliance-sensitive wording, or evidence-risk questions that exceed style editing, hand off to `arc-audit` for bounded review.
4. If the request turns into implementation work, document generation from code, or repository changes, stop escalating inside prose editing and route to `arc-build`.
5. If repository context or prior project artifacts are stale and the writing task depends on them, refresh via `arc-init` and read `arc-cartography` products before continuing.
6. The default downstream close-out is `arc-audit` or explicit human review when the user asks for a second-pass quality gate after polishing.

## When to Use
- **Preferred Trigger**: The user wants to polish or restructure academic or professional prose with explicit requirements around chunked processing, citation preservation, or multi-stage review.
- **Typical Scenario**: Thesis chapters, conference papers, white papers, regulatory reports, and longform technical prose that feel too templated or too "AI-written" but must retain evidence and attribution.
- **Boundary Tip**: Use `arc-audit` for repository/system audits, `arc-build` for implementation work, and do not use `arc-aigc` to help the user evade detectors or misrepresent authorship.

## Input Arguments
| Parameter | Type | Required | Description |
|---|---|---|---|
| `target_text` | string | yes | Full manuscript text or path to the document needing polish. |
| `academic_context` | string | no | Research goal, venue, audience, tone target, or author voice guidance. |
| `polish_depth` | string | no | `light` / `balanced` / `deep`; controls rewrite intensity and review strictness. |
| `citation_style` | string | no | Citation system to preserve, such as APA, IEEE, Nature, or GB/T 7714. |
| `integrity_policy` | string | no | Disclosure rules, forbidden edits, or institutional constraints that the rewrite must respect. |
| `preserve_sections` | array | no | Section headers or span types that must remain untouched, such as equations or legal clauses. |

## Outputs
```text
<project>/.arc/aigc/<session>/
├── session-plan.md       # chunk breakdown, protected spans, and pass goals
├── revision-log.md       # chunk id, original text, rewrite, rationale, citation comments
├── stage2-summary.md     # cohesion review, transition fixes, terminology and citation gaps
├── polished-text.md      # final draft for author/editor review
└── compliance-notes.md   # optional notes for disclosure, unresolved evidence gaps, or manual follow-up
```
