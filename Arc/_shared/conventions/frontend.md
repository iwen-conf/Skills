# Frontend Conventions

Applied by arc:code for frontend and UI changes; checked by arc:audit. Native selection:
[native-platforms.md](native-platforms.md). Web theme values: [frontend-tokens.md](frontend-tokens.md).
API docs: [api-contract.md](../api-contract.md). UI bug repair
follows [execution-truth.md](../../../docs/execution-truth.md) §6.

For existing-surface screenshot feedback, use [visual-iteration.md](visual-iteration.md): lock
target/preserve/evidence, fix the shared cause, then verify the applicable size/state/input matrix.

## 1. Before editing

1. Name the user workflow, pages, target devices/viewports and verification commands. No frontend change
   without a user workflow.
2. Read the existing routes, layouts, components, theme files, API clients, stores, forms and tests the
   change touches.
3. Name each requested platform and its SDK/runtime; mobile and desktop are not single shared UI targets.
4. Deliver the requested usable UI, never a decorative landing page in its place.

## 2. Stack

| Target | Default stack for new projects |
|---|---|
| Web | React 19 + TypeScript + Vite + Tailwind CSS + shadcn/ui + Zustand + TanStack Query + TanStack Router + React Hook Form + Zod |
| Native mobile, desktop and Mini Program | Target-specific stack in [native-platforms.md](native-platforms.md) |

1. Existing-product maintenance keeps its stack; a native migration requires that scope. New independent
   targets use the native direction; establish integration boundaries before mixing runtimes.
2. IF the user names another stack THEN use it and record the exception in the report.
3. For Web, never introduce a same-duty substitute (React Router, Redux, MobX, SWR, Formik, Yup, Ant Design, MUI,
   Chakra, a custom component library) unless the user requires it or the project already depends on it
   with a documented boundary.
4. Do not choose shared UI runtimes or web shells for native targets to maximize cross-platform reuse.
5. Install a capability library (forms, server cache, routing, cross-page state) only when the deliverable
   uses it.
6. Domain skills follow the actual platform: Web libraries do not override native Android, Apple or HarmonyOS guidance.
7. IF the task touches WeChat platform APIs, privacy, payment, native component constraints or Skyline, or
   maintains an existing native WeChat mini-program, THEN follow WeChat platform rules (the external
   `wxskills` skill when the host lists it, otherwise WeChat official docs).

## 3. State ownership

The library mapping below is Web-only. Native targets use platform lifecycle-aware state and
observation; keep the same ownership separation without installing JavaScript equivalents.

| State | Owner |
|---|---|
| URL and search params | TanStack Router |
| Server cache and mutations | TanStack Query |
| Lightweight cross-page client state | Zustand |
| Complex form state | React Hook Form + Zod |
| Component-only state | React local state |

1. Never copy server state into a global store; never mix URL, server, client-global, form and local state in
   one store.
2. Use the project's existing data, form and state patterns before adding a library.

## 4. UI states

1. Semantics: `loading` = request or transition pending; `empty` = request succeeded with no usable records
   or content; `error` = request or parsing failed; `permission-denied` = user may not see or perform the
   action; `disabled` = action unavailable in the current state.
2. Every state the data contract or permission model can produce has its own branch.
3. Derive `empty` from the successful contract (zero items/total or an explicit empty flag),
   never from thrown errors, rejected promises, HTTP 4xx/5xx branches or generic catch handlers.
4. `empty` and `error` are different surfaces. Empty uses neutral tone, a contextual next action and
   optional create or reset-filter guidance; never destructive/error styling, error pages, exception
   boundaries or failure toasts. Empty search results, first-use pages, zero-count dashboards and
   filtered-out lists are empty states.
5. IF roles or permissions exist THEN model RBAC at route and action level.

## 5. Styling, layout, accessibility

1. Style through semantic tokens: Web CSS variables or native theme/color resources; do not hardcode palettes in components.
2. Accessibility basics: semantic controls, labels, focus states, keyboard reachability, contrast.
3. Responsive layouts use explicit constraints; never viewport-scaled fonts as the layout solution; no
   overlapping or overflowing text in the target viewports.
4. Avoid unnecessary secondary labels and descriptions. Let the layout do the explaining.
5. Show each fact once per view; a count or status already in the list is not repeated in a header.
6. End, transfer and delete confirm in a modal dialog stating the consequence in one sentence.
   One feedback per action: no toast when the screen already shows the result.
7. A visible failure says what failed and why; unreachable environments add copyable detail
   ([repair.md](../../arc:code/references/repair.md) §1).

## 6. API client

1. Client calls and types follow the shared Markdown contract; same field names and empty-result shape.
2. Web: TanStack Query hooks live in `src/api/`, the fetcher in `src/lib/`, Zod schemas in `src/schemas/`.
   Native: use the platform's networking/serialization and existing module structure against the same contract.
3. Mock-only checks never complete acceptance that requires the real API.
4. When work is split into tasks, frontend and backend tasks are separate (own ID, dependencies,
   acceptance, status); the frontend task references the shared contract.

## 7. Debugging and probes

1. IF a UI bug is reproducible or observable THEN save console output, network failures, runtime errors,
   screenshots and reproduction notes to `.arc/artifacts/<task-slug>/logs/` before broad edits, and drive the
   fix from exact error strings or request IDs.
2. Trace the owning UI/data/state boundary and inspect sibling routes and components sharing it. Never ship a
   layout or state patch that hides a shared contract or architecture cause.
3. Remove `console.log`, `debugger`, alert probes and noisy instrumentation before completion, or convert
   them to the project's level-gated logger or telemetry.

## 8. Scaffold (new projects only)

Native targets use the platform IDE/official template for the confirmed SDK: Android Studio,
Xcode, DevEco Studio, Windows tooling or the Mini Program developer tools. Use the target's build,
signing and test setup; do not start with Web and wrap it. Check current template/API availability
for the selected target before producing commands.

The following commands and directory tree are **Web-only**:

```bash
pnpm dlx shadcn@latest init -t vite -b radix -n my-app   # Vite + React 19 + TS + Tailwind CSS v4 + shadcn/ui
cd my-app
pnpm add @tanstack/react-query @tanstack/react-router @tanstack/react-router-devtools \
  zustand react-hook-form @hookform/resolvers zod
pnpm dlx shadcn@latest add button card input label field dialog dropdown-menu select \
  table tabs sonner tooltip sheet avatar badge
# admin consoles add:
pnpm dlx shadcn@latest add pagination breadcrumb command popover calendar
pnpm add @tanstack/react-table   # data table = shadcn table + TanStack Table; date picker = popover + calendar
```

- Keep CSS variables on (never `--no-css-variables`). Pick the base color nearest the chosen palette as a
  placeholder, then replace the variables from [frontend-tokens.md](frontend-tokens.md).
- IF the project already uses a shadcn base (`radix`, `base`, `aria`) THEN keep it.

```text
src/
├── api/            # TanStack Query queries and mutations
├── components/     # business components
├── components/ui/  # shadcn primitives (generated by the CLI)
├── hooks/
├── lib/            # cn, fetcher
├── routes/         # TanStack Router routes
├── schemas/        # Zod schemas
└── stores/         # Zustand stores
```

`src/main.tsx` essentials:

```tsx
const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 30_000, retry: 1 } },
});
const router = createRouter({ routeTree, context: { queryClient } });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </React.StrictMode>
);
```

Check: `pnpm dev` shows background, card and primary button in the palette; `<html class="dark">` switches
to `.dark` when dual-theme; `pnpm build` passes.

## 9. Acceptance items (proved by arc:verify)

Commands and tools per platform: [stacks.md](../../arc:verify/references/stacks.md) "UI checks".

1. Interaction closed loop: each journey completes and the UI returns to a consistent state.
2. Visibility: key elements are present and visible, not hidden by system bars, overlap or overflow.
3. Clickability: controls are reachable and enabled, with correct `disabled`/loading states and hit area.
4. Readability: no overlapping or truncated text, sufficient contrast, stable layout at target viewports.
5. Empty and error render through different branches and visual treatments.
6. Build, typecheck, lint and tests pass, or the blocker is reported.
7. Native work proves relevant platform navigation, lifecycle, system integration and accessibility
   on its target; a Web preview or a successful package build is not equivalent evidence.
