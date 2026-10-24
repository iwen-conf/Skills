---
name: ui-design
description: >-
  Visual design direction only, before or instead of page code: choose a style (editorial minimal, high-end soft, brutalist,
  motion-rich editorial, premium SaaS), strip generic AI-looking patterns, audit or critique an existing UI's look,
  generate per-section reference images for web or mobile, build brand-kit boards, and write DESIGN.md (tokens,
  type scale, layout, motion, banned patterns) for arc:code to implement. 视觉方向, 设计规范, 风格, 去AI味审美,
  改版审计, 品牌视觉, 设计参考图. Not for building or changing pages, even ones that must look polished
  and not AI-generic (use arc:code, which follows DESIGN.md), or reading Lanhu drafts (use lanhu-design).
metadata:
  library:
    - design-taste-frontend
    - minimalist-ui
    - high-end-visual-design
    - industrial-brutalist-ui
    - gpt-taste
    - redesign-existing-projects
    - stitch-design-taste
    - impeccable
    - better-ui
    - brandkit
    - image-to-code
    - imagegen-frontend-web
    - imagegen-frontend-mobile
---

# ui-design

Decide how the interface should look and hand `arc:code` a `DESIGN.md` it can implement without making further
taste decisions. The library entries are third-party style guides; this file overrides them.

## Overrides (apply to every entry)

1. **No code from this skill.** Ignore entry sections that build pages, scaffold projects, install packages or
   list component code; implementation is `arc:code` frontend mode.
2. **Stack belongs to the project.** Ignore entry rules that pick a framework, renderer, animation or icon library
   (Next.js/RSC defaults, Phosphor over lucide, GSAP, Python-driven randomization). The stack comes from the
   project, else [frontend.md](../../Arc/_shared/conventions/frontend.md); DESIGN.md names visual rules only.
3. **Images only when asked.** Entries that make image generation mandatory apply only in reference-images or
   brand mode.
4. **Existing design systems win.** A project with tokens or a component library gets increments, not a reset.

## Modes

| Condition | Mode | Read |
|---|---|---|
| New look for a product or page; user names or implies a style | direction | one preset below, then [stitch-design-taste](../../Library/design/stitch-skill/SKILL.md) for the DESIGN.md format |
| Make an existing UI look less generic; review its look | audit | [redesign-existing-projects](../../Library/design/redesign-skill/SKILL.md); critique and polish lists in [impeccable](../../Library/design/impeccable/SKILL.md) |
| Small polish: radii, alignment, depth, motion restraint | audit (polish) | [better-ui](../../Library/design/better-ui/SKILL.md) |
| Section-by-section reference images (web / mobile) | reference-images | [imagegen-frontend-web](../../Library/design/imagegen-frontend-web/SKILL.md) / [imagegen-frontend-mobile](../../Library/design/imagegen-frontend-mobile/SKILL.md); image-first build analysis in [image-to-code](../../Library/design/image-to-code-skill/SKILL.md) |
| Logo system, identity boards, brand guidelines | brand | [brandkit](../../Library/design/brandkit/SKILL.md) |

| Preset | Read |
|---|---|
| Premium SaaS / landing default, anti-slop dials | [design-taste-frontend](../../Library/design/taste-skill/SKILL.md) |
| Editorial minimal, warm monochrome | [minimalist-ui](../../Library/design/minimalist-skill/SKILL.md) |
| High-end soft, agency feel | [high-end-visual-design](../../Library/design/soft-skill/SKILL.md) |
| Brutalist, Swiss print plus terminal | [industrial-brutalist-ui](../../Library/design/brutalist-skill/SKILL.md) |
| Motion-rich editorial, AIDA pages | [gpt-taste](../../Library/design/gpt-tasteskill/SKILL.md) |

Read one preset and at most two mode entries per task; large entries (design-taste-frontend, image-to-code,
imagegen-*) are read by section, starting from their banned-pattern and pre-flight sections.

## Workflow

1. Write three lines: product, audience, intended feel. Audience unknown and not inferable → ask one question with
   a default.
2. Pick the mode and preset from the tables; record why.
3. Audit mode: list concrete problems with location (`page/section/element`), each tied to a rule.
4. Write `DESIGN.md` at the project root (or the path the project uses) with seven sections: theme and
   atmosphere; color palette with roles and token names; typography scale; component styling; layout and
   spacing; motion; banned patterns.
5. Every rule is either a value (token, size, duration, ratio) or a checkable ban (`no h-screen hero; use
   min-h-[100dvh]`); rewrite anything vaguer.
6. Hand off: one line naming `arc:code` frontend mode, the DESIGN.md path and any reference images.

## Completion

- [ ] DESIGN.md has all seven sections, values and checkable bans only, no stack choices.
- [ ] Audit findings located and tied to rules; reference images listed with paths when generated.
- [ ] No page code written; handoff line to `arc:code` present.
