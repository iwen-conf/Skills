# ETA visual reference — MIT attribution

This Arc verification workbench's **read-only dependency plan layout** adapts the
published appearance of ETA's WidePlan and PhonePlan, including horizontal
dependency steps, compact cards, cubic link geometry, emphasized critical edges,
and vertically stacked mobile steps.

- Upstream: [0xfe-project/eta](https://github.com/0xfe-project/eta)
- Source version reviewed: `aad7db8de46b8296fe35c3a0776e5e286836e417`
- Reference: `apps/web/src/components/workspace/task-plan-view.tsx`
- Original copyright: Copyright (c) 2026 eta contributors
- License: MIT

The MIT license grants permission to use, modify, and distribute the code and
design references, provided the original copyright and license text are retained.

```text
MIT License

Copyright (c) 2026 eta contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is furnished
to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Visual acceptance matrix (not yet signed off)

The ETA-style default view in Arc should be compared side by side with the
upstream ETA plan using equivalent dependency structures at 1440×900 desktop,
1024×768 tablet, and 390×844 mobile dimensions:

- Desktop: 208–256 px horizontal columns, 40 px gaps, 10 px vertical card
  spacing, cubic dependency arrows with stroke 1.2 and critical stroke 1.7,
  compact rectangular cards with a task state icon and one metadata line.
- Mobile: vertical step groups, minimum 35 px button height (44 px advised for
  accessibility); edges intentionally hidden to avoid impossible graph
  compression.
- Interaction: click and keyboard Enter/Space on nodes and edges; selected node
  updates the Arc evidence panel rather than changing model topology.
- Branch group edges: cross-group links are explicit, not silently discarded.
- Behavior: never imply that parallel layout means safe concurrent execution
  or that an independent prerequisite observation equals completed business
  execution.

The fixture SSR suite validates the DOM structure and data semantics, **not**
pixel-level browser parity. GitHub Actions, screenshot comparison and a
real isolated pay_gateway browser walkthrough are separate acceptance gates.
