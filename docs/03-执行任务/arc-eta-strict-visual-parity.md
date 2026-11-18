# Arc Verify — ETA strict visual comparison
Reference freeze: ETA [6b62da886b225391a157c7486e93b85aef73a17c](https://github.com/0xfe-project/eta/commit/6b62da886b225391a157c7486e93b85aef73a17c).
Reference files: `apps/web/src/components/workspace/task-plan-view.tsx`, `task-status-icon.tsx`, `use-mobile.ts`, `apps/web/src/styles.css`.

## UI implementation changes
- Local task-group steps start at 1, like ETA's `planOf(children)`, instead of carrying global dependency depth into group cards.
- Desktop plan expands to full canvas width by default. “显示事实面板” exits focus without losing selection.
- Exact ETA layout tokens: root page horizontal padding 21.6px, group/plan padding 12.6px, columns 208–256px, gap 36px, card padding 9px × 7.2px, row gap 9px, sharp 0px corners.
- Status icons use ETA's SVG ring/filled-disc geometry rather than Unicode symbols. State labels remain Arc's independent-observation states, not ETA's task-completion verdicts.
- Mobile breakpoint is 768px: 768 still uses WidePlan; 767 and below use vertical PhonePlan.
- Browser focus mode keeps old detailed proof context accessible via a clear toggle and preserves all underlying data.

## What was actually measured
A source-derived Chromium fixture (not a running copy of ETA) was inspected with matched synthetic tasks and a system-font fallback at 1440×900, 1024×768, 768×900, 767×900, and 390×844:
- Width 1440: first desktop plan card 256px wide, 55.67px tall; 36px column gap.
- At 768px: horizontal plan visible.
- At 767px and 390px: vertical phone steps visible; wide connectors hidden.

This fixture tests CSS geometry derived from ETA code. It **does not establish a 1:1 pixel-diff score between the actual ETA app and the actual Arc export**.

## Strict 1:1 acceptance gates
Must be passed and recorded before claiming 1:1:
1. Run identical task-dependency fixture in both actual applications with identical theme, viewport, deviceScaleFactor, operating-system font rendering and installed fonts.
2. Screenshot: 1440×900, 1024×768, 768×900, 767×900, 390×844.
3. Compare group header, task card and label bounding boxes (≤1px geometry error), link anchor positions (≤1px), border/shadow, hover/focus, keyboard, selection, scroll.
4. Pixel difference below a declared threshold with transparent masks limited to product-specific strings—not the whole board.
5. Independently validate Arc's business evidence and identity scoping; no stale run or missing observation may become PASS.

## Font-license limitation
ETA's `THIRD_PARTY_NOTICES.md` explicitly states the `Eta Sans` bundled font is **not** covered by MIT and merges CaskaydiaCove Nerd Font Mono (SIL OFL) with Alibaba PuHuiTi 3.0 (Alibaba terms). Arc intentionally does not redistribute the merged font. Therefore a guaranteed glyph-for-glyph visual match **cannot yet be signed off** until font usage/distribution is legally verified and the same font is available in the visual QA environment.

## Test status
Source-derived geometry fixture was rendered in Chromium (not the live ETA application). Component full-suite build, live browser comparison and exact pixel difference against the ETA application remain `NOT_EVALUATED` in this environment. No real pay_gateway E2E was performed.
