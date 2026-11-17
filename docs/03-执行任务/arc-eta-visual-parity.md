## ETA visual parity — plan board
UI reference: [ETA WidePlan/PhonePlan](https://github.com/0xfe-project/eta/blob/aad7db8de46b8296fe35c3a0776e5e286836e417/apps/web/src/components/workspace/task-plan-view.tsx) (MIT).

Arc's dependency view now defaults to an ETA-style **wide plan**: compact flat task cards, horizontal levels (208–256px columns), 40px lanes, measured cubic dependency connectors, arrows, bold critical chain, card status icons, and cross-group links. On <=760px screens it uses ETA-style vertical steps with no misleading dependency wires. The previous detailed business genealogy directory is retained under an expandable section.

The visual plan is intentionally **read-only**. It never exposes ETA's drag-to-edit handles, because Arc snapshots are independently verified evidence. Alternative providers and OR branches are dashed and do not become mandatory blockers. State cycles remain in Arc's business-state view.

Acceptance still outstanding: side-by-side pixel comparison at matched desktop/mobile viewport, actual browser keyboard/hover/pan behavior, and an isolated pay_gateway business run. SSR and model tests prove DOM contracts, not screenshot equivalence.
