# Interface System: Linear & Apple Notes Standard (UX x DX x AX)

Every interface decision must simultaneously improve **User Experience (UX)**, **Developer Experience (DX)**, and **Agent Experience (AX)** without breaking existing workflows.

## 1. Self-Evident Layout Over Crutch Copy

- **Squint & Strip Pass**: Delete marketing slogans, instructional banners, helper subtitles under section headers, empty-state essays, decorative icon circles on metric cards, and redundant status pills. If hierarchy feels unclear without prose, fix alignment, spacing rhythm (`4/8/12/16/24/32px`), or progressive disclosure—never add explanatory copy.
- **Single-Cue Framing**: Use **at most one** boundary cue per container (`1px` hairline border OR subtle surface elevation step—never stacked `border + drop-shadow + tinted card`).
- **Focused Canvas**: One primary action per screen; move secondary actions into context menus, sheets, or `Cmd/Ctrl+K` command palettes.

## 2. Native Toolkits & Viewing-Distance Ergonomics

- **Native Direction**: `Swift + SwiftUI` (`apple-native.md`), `Kotlin + Jetpack Compose` (`android.md`), `ArkTS + ArkUI` (`harmonyos.md`), `React 19 + TypeScript + Tailwind CSS + shadcn/ui` (Web). Never ship web wrappers as native apps or add competing UI/state libraries.
- **Touch / Mobile (20-40 cm)**: Minimum **44pt x 44pt** (`44px x 44px` / `48dp`) hit targets; thumb-zone primary controls; zero horizontal overflow at `320px`; foldable split-pane and hinge avoidance.
- **Pointer / Desktop (50-100 cm)**: Dense scannable rows, full keyboard traversal, crisp `:focus-visible` outlines, deterministic shortcuts.

## 3. Executable Visual, Motion & State Matrix

- **Typography**: System fonts (`SF Pro`/`NY` on Apple, native sans elsewhere). Floor **>= 11pt** native / **12px** metadata / **14px** body; **ban Ultralight/Thin weights**; enforce `tabular-nums` on counts, amounts, dates, and table columns; reflow horizontal groups into vertical stacks cleanly under Dynamic Type and `200%` zoom.
- **Color & Materials**: Single semantic token layer across Light/Dark modes (`>= 4.5:1` contrast, target `7:1`; never color-only status). Restrict translucent blur / Liquid Glass strictly to floating chrome over scrolling content. Concentric nested radii: `R_outer = R_inner + padding`.
- **Motion**: Animate only `transform` and `opacity` (`120-180ms`, `cubic-bezier(0.2, 0, 0, 1)`). **Ban `transition: all`** and layout-shifting skeletons. Honor `prefers-reduced-motion`.
- **AX & State Matrix**: Use semantic primitives (`<button>`, `<a href>`, `<input>`, native controls—**ban `<div onClick>`**) with explicit `aria-label` / `accessibilityLabel` on icon-only buttons. Verify `default`, `loading`, `empty`, `error` (`aria-invalid` + `aria-describedby` + focus first invalid field), and `disabled` states across Light and Dark themes.
