---
name: arc:learn
description: "深度技术研究与知识内化：执行收集、消化、大纲、填充、提炼的六阶段工作流；当用户说“深入研究/deep research/learn this domain/整理资料并输出文章”时触发。"
version: 1.0.0
allowed_tools:
  - Bash
  - Read
  - Write
  - Edit
  - Grep
  - Glob
hooks:
  PreToolUse:
    - matcher: Bash
      hooks:
        - type: command
          command: "bash ${ARC_SKILL_DIR}/scripts/check-destructive.sh"
          statusMessage: "Checking for destructive commands..."
---

# arc:learn — deep research and knowledge synthesis

## Overview

`arc:learn` is designed for diving deep into unfamiliar technical domains, evaluating complex new libraries, or turning collected sources into structured, publishable output (like architecture whitepapers or RFCs).

It runs a strict six-phase workflow: Collect, Digest, Outline, Fill In, Refine, and Publish. It explicitly prevents "lazy AI summarization" by forcing the verification of sources and building a genuine mental model.

## Quick Contract

- **Trigger**: The user needs to deeply understand a new domain, research a technology, or synthesize raw materials into a comprehensive article.
- **Inputs**: Target domain/topic, raw source URLs or documents, `mode` (Deep Research / Quick Reference / Write to Learn).
- **Outputs**: Organized raw materials, verified outlines, and refined publishable documents.
- **Quality Gate**: Must pass the three-layer verification check (cross-domain recurrence, generative power, distinctiveness) in `## Quality Gates` before outlining.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **Boundary Note** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:learn` to run a deep research workflow. I will collect primary sources, verify concepts, and build a structured understanding before generating output."

## Teaming Requirement

- Every execution must first "draw a team together" and at least clarify the three roles and responsibilities of `Owner`, `Executor` and `Reviewer`.
- If the operating environment only has a single Agent, the three-role perspective must be explicitly output during delivery to form a "decision-execution-review" closed loop before submitting the conclusion.

## The Iron Law

```text
AI IS A TOOL FOR DIGESTION, NOT A REPLACEMENT FOR THINKING.
NO SUMMARIZATION WITHOUT SOURCE VERIFICATION.
```

The moment you outsource the thinking to generic summaries, the learning stops.

## Workflow

Before starting, confirm the mode:
- **Deep Research**: Understand a domain well enough to write about it (Phase 1 to 6).
- **Quick Reference**: Build a working mental model fast, no article planned (Phase 1 to 2).
- **Write to Learn**: Already have materials, want to force understanding through writing (Phase 3 to 6).

1. **Phase 1: Collect**: Gather only high-quality primary sources (papers, official blogs, canonical repos). Convert to Markdown and organize in a local repository.
2. **Phase 2: Digest**: Read fully. Use AI *only* to translate or explain specific dense concepts, NOT to summarize entire documents. Discard weak materials.
3. **Phase 3: Outline**: Build a structural outline mapping concepts to specific sources. Do not proceed until the outline is verified.
4. **Phase 4: Fill In**: Expand the outline section by section. Revisit materials to fill gaps.
5. **Phase 5: Refine**: Remove redundancy, flag logic gaps, and strip AI patterns.
6. **Phase 6: Publish/Review**: Read linearly for truth and flow. Fix abrupt transitions.

## Quality Gates

- Outlines must pass **Three-Layer Verification**:
  1. *Cross-domain recurrence*: Does the claim appear in multiple independent contexts?
  2. *Generative power*: Can the framework predict outcomes in unaddressed scenarios?
  3. *Distinctiveness*: Is it specific to the source, or generic wisdom?
- Unverified claims with zero passes must be cut.
- Every section in the outline must cite its origin source material.
- AI must not be used to write sections from scratch without source backing.

## Expert Standards

- Apply the `Three-Layer Verification` rigorously to all key claims.
- Construct the `Intellectual Genealogy` of the field: trace problems backward and forward to understand the evolution of the domain.
- Prefer `Primary Sources` over secondary explainers or listicles.
- Ensure `Contextual Fidelity` when translating or explaining dense concepts.

## Scripts & Commands

- Runtime main command: `arc learn`
- Setup research directory: `mkdir -p .arc/learn/<topic>/{raw-sources,digests,drafts}`

## Red Flags

- Asking AI to summarize a paper without reading it.
- Generating a template instead of building a structured mental model.
- Outlines lacking source citations.
- Letting AI write the final article in a generic voice.

## Gotchas

Real failures from prior sessions, in order of frequency:

- **Outsourced the thinking**: Asked the AI for a summary of 5 papers and just pasted the result. The learning was shallow.
- **Used weak sources**: Collected "Top 10" blog posts instead of official documentation or original papers.
- **Skipped the outline phase**: Started writing immediately, resulting in a rambling, incoherent draft.
- **Lost the authorial voice**: Failed to refine the draft, leaving it sounding like a generic AI response.

## When to Use

**首选触发**: Deep diving into a new architecture, preparing a tech radar report, or turning disorganized research into a whitepaper.
**典型场景**: Researching a new LLM framework, understanding a complex consensus algorithm, or writing an engineering blog post.
**边界提示**: Use `arc:context` for just recovering task state, `arc:decide` for making a concrete technical choice, and `arc:learn` for building the foundational knowledge required to make those choices.

## Sign-off

```text
files changed:    N (+X -Y)
scope:            on target / drift: [what]
hard stops:       N found, N fixed, N deferred
signals:          N noted
verification:     [command] → pass / fail
```