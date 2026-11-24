# Web Frontend Engineering Standard (2026 Baseline)

Applied by `arc-plan`, `arc-code`, `arc-verify`, and `arc-audit` on Web frontends alongside `ui-system.md`, `contract.md`, and `api-contract.md`.

## 1. Scope & Legacy Boundary (`适用边界`)

- **New Web Projects & Independent Modules**: Adopt this 2026 baseline by default. Do **not** introduce Ant Design (`antd`) or competing UI, router, or state libraries.
- **Existing / Legacy Repositories**: Follow the repository's current stack unless a migration is explicitly authorized; never force-rewrite a legacy project unasked.

## 2. Universal Base Stack (`通用基础栈 — 每层唯一选择`)

| Layer | Locked Choice (2026 Default) | Hard Constraints & Pitfall Guards |
|---|---|---|
| Build / Framework | **Vite + React 19 + TypeScript (`strict`)** | Marketing/Official sites use **Astro** + React 19 Islands |
| Routing | **TanStack Router** | Marketing sites use Astro file-based routing |
| Server State | **TanStack Query** | Never duplicate API responses into Zustand |
| API / Codegen | **Orval** | Single `orval.config.ts` from OpenAPI -> TS types, TanStack Query hooks, Zod 4 (`override.zod.version: 4` or `'auto'`), MSW v2 |
| Client State | **Zustand** | Strictly UI-only state (theme, dialogs, sidebar, ephemeral selection) |
| URL State | **TanStack Router `validateSearch`** | Store filters, pagination, sorting, and tabs here; pass Zod 4 schemas directly via Standard Schema (ban `nuqs` and `@tanstack/zod-adapter`) |
| Forms & Validation | **React Hook Form + Zod 4** | Pin Zod 4-compatible `@hookform/resolvers >= 5.x/6.x` and `react-hook-form >= 7.70` |
| UI & Styling | **shadcn/ui + Tailwind CSS v4** | Use `@tailwindcss/vite` + CSS-first `@import "tailwindcss";`; ban `@astrojs/tailwind` and `tailwind.config.js` |
| Motion | **Motion** | Always import from `motion/react` (`>= 14.2` for GPU-accelerated `x`/`y`/`scale` shorthands, or full `transform` string; ban legacy `framer-motion`); follow `ui-system.md` curves & reduced-motion rules |
| Testing | **Vitest + Testing Library + Playwright** | Orval MSW v2 handlers are for tests only |
| Lint / Format / Monitor | **Vite+ (`vp`: Oxlint + Oxfmt + Vitest, MIT)** + **Sentry** | Default for Vite/React (`gate.sh` native); ESLint + Prettier fallback for Astro or legacy repos |
| Auth Boundary | **Backend-managed session** | Prefer `HttpOnly; Secure; SameSite` cookies (`credentials: 'include'`); frontend only sends credentials, guards routes in `beforeLoad`, and handles expiry. Ban tokens in `localStorage`; route/button permissions are UX guards only — backend RBAC is authoritative |

## 3. Three-Scenario Matrix (`三类场景定稿`)

| Scenario | Framework & Routing | UI & Styling | Table / Form | Charts | Motion | Scenario Specifics & Template |
|---|---|---|---|---|---|---|
| **后台管理 (Admin)** | Vite + React 19 + TanStack Router | shadcn/ui + Tailwind v4 (no Ant Design) | TanStack Table + shadcn Data Table + React Hook Form + Zod 4 | shadcn charts (Recharts) | Motion (`motion/react`, subtle functional feedback) | Route + button permissions (backend authoritative), dense data tables. Reference template: `satnaing/shadcn-admin` (MIT; when scaffolding, upgrade/pin Zod 4, add Orval + `vp`, replace `localStorage` token auth with backend cookie auth) |
| **官网 / 营销站 (Marketing)** | Astro (static HTML for SEO + file routing + React 19 Islands) | shadcn/ui + Tailwind v4 + Magic UI (MIT; ban custom-licensed Aceternity) | React Hook Form + Zod 4 (contact/lead forms) | None | Motion (`motion/react`) + Magic UI | Design tokens, perf budget, SEO; keep `.astro` static by default and hydrate interactive islands via `client:visible`/`client:idle` (avoid blanket `client:load`) |
| **C 端产品 (Consumer App)** | Vite + React 19 + TanStack Router | shadcn/ui + Tailwind v4 | React Hook Form + Zod 4 | shadcn charts (Recharts, when needed) | Motion (`motion/react`) | PostHog analytics, optimistic updates, non-layout-shifting skeletons |

## 4. Verified AI Toolchain (`AI 工具链`)

- **TanStack (Router / Query / Table)**: Official MCP (version-scoped docs) + `llms.txt` + community Router/Query agent skills.
- **shadcn/ui**: Official MCP Server (search & install registry components) + shadcn skills.
- **Astro**: Official Docs MCP (`https://mcp.docs.astro.build/mcp`) + `llms.txt`.
- **Motion**: `npx motion-ai` official skill & MCP (`motion/react` import convention).
- **Orval**: Single config generating TanStack Query hooks + Zod 4 schemas + MSW v2 handlers.

## 5. Hard Rules & New Project `AGENTS.md` Checklist (`硬红线与项目 AGENTS.md 必填规则`)

1. **Declare Stack & Versions in `AGENTS.md`**: Pin `React 19`, `TypeScript strict`, `Zod 4`, `Tailwind v4`, `shadcn/ui`, `TanStack Router/Query/Table`, `Orval`, `Vite+ (vp)` (plus `Astro + Magic UI` for marketing sites).
2. **Orval-Only Server Fetching**: Fetch and mutate server data solely through Orval-generated TanStack Query hooks; ban hand-written `fetch`/`axios` calls and `useEffect` data fetching.
3. **URL Search Params for View State**: Keep filters, pagination, sorting, and tabs in TanStack Router `validateSearch` (Zod 4 Standard Schema directly — no `nuqs`, no `@tanstack/zod-adapter`).
4. **UI-Only Zustand**: Store only client UI state in Zustand — never API/server data.
5. **Modern Motion & Astro Islands**: Import animations solely from `motion/react` (ban `framer-motion`); in Astro, keep components static `.astro` by default and hydrate interactive islands sparingly (`client:visible`/`client:idle`).
6. **Generated Code Is Read-Only**: Never hand-edit generated directories or files (`src/api/generated/`, `routeTree.gen.ts`); update OpenAPI specs or route files and regenerate.
7. **Test-Only MSW Boundary (`contract.md`)**: Orval MSW v2 handlers are strictly for Vitest, Playwright, or isolated contract tests — never bundle into production or use as silent fallback when real APIs fail.
8. **Canonical Reference Page First**: Build one reference page per new project (covering `beforeLoad` auth guard, `validateSearch`, Orval hook, form/table, and `loading`/`empty`/`error` states) before scaling sibling pages.
9. **Layout & Live Visual Gate**: Flow layouts use Grid/Flex + `gap` only (ban flow `position: absolute`, negative-margin stacking, fixed-height cards, and arbitrary `z-index`); enforce `min-h-*`, `min-w-0`, `>= 44px` hit targets with `>= 8px` spacing, full interactive states (`hover`, `:focus-visible`, `disabled`, `loading`), and 3-viewport (`375/768/1280`) browser screenshot verification before claiming UI work done.
10. **Web Stack Conflict Guards**: Ban dynamic Tailwind v4 `@theme` class interpolation (`bg-${color}-500` — use static class maps); never combine Tailwind + CSS Modules or Motion + CSS transitions on the same DOM element; stick to one icon set (`lucide-react` — never mix Heroicons or Font Awesome); virtualize lists `> 50` items via `@tanstack/react-virtual` (or `content-visibility: auto`).

## 6. Automated UI Quality Hard Gates (`不靠 AI 自觉的四道机器防线`)

- **Gate 1 — Static A11y Lint (`vp check` / Oxlint `jsx-a11y` + `gate.sh`)**: Enable `plugins: ["jsx-a11y"]` in `vite.config.ts` (or `oxlint.config.ts`; use `eslint-plugin-jsx-a11y` on Astro/ESLint repos) with `jsx-a11y/no-static-element-interactions`, `jsx-a11y/click-events-have-key-events`, `jsx-a11y/prefer-tag-over-role`, `jsx-a11y/anchor-is-valid`, and `jsx-a11y/label-has-associated-control` set to `"error"`. Ban `oxlint-disable` / `eslint-disable` bypasses (`gate.sh` blocks both and statically flags `<div|span onClick>`, `<div role="button">`, `<a href="#">`, `framer-motion` imports, `transition-all`/`transition: all`, `outline-none` without `focus-visible`/`focus:` replacement, `tabIndex > 0`, `user-scalable=no`/`maximum-scale=1`, and `onPaste` `preventDefault()`).
- **Gate 2 — Semantic Locators Only**: In Testing Library and Playwright, locate interactive controls exclusively with `getByRole` and `getByLabel` (never `data-testid` or CSS class selectors for clickable controls) so non-semantic `div` buttons fail automatically.
- **Gate 3 — Runtime WCAG Scan (`@axe-core/playwright`)**: Run `new AxeBuilder({ page }).analyze()` and assert `violations` is empty on key views and modal/drawer states, paired with manual/scripted keyboard Tab/Enter/Space checks.
- **Gate 4 — 3-Viewport Geometry & Overlap Assertions (`375 / 768 / 1280`)**: In Playwright, assert `document.documentElement.scrollWidth <= window.innerWidth`, verify pairwise `boundingBox()` non-intersection on sibling cards/controls, check `>= 44x44` hit targets on touch controls, and save before/after screenshots.
