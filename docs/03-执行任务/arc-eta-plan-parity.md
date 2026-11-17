# ETA Dependency Plan Parity — Arc Verify

Reference: [ETA task-plan](https://github.com/0xfe-project/eta/blob/22ea3beb815b5e0b5203fb8e0ce6c33e6c0ea435/apps/web/src/components/workspace/task-plan.ts) (MIT); Arc base: 2a49f090a1b3577fc2ddac6f8f98042a7ba35199.

## Contract

ETA manages mutable agent tasks. Arc reviews immutable business contracts and evidence. **1:1 is defined here as read-only plan navigation and presentation parity**, not permission to drag-edit historical state or run a business operation from an exported snapshot. Exact pixel parity is not established without side-by-side browser screenshots on the same viewport.

| ETA plan behavior | Arc implementation | Verification |
| --- | --- | --- |
| Longest dependency depth, regardless of shortcuts | `layout.ts` / `eta-read-plan.ts` | shortcut-vs-longest-path test |
| One displayed critical chain; unresolved chain wins ties | `etaReadPlan().critical / attention` | equal-length tie test |
| Parent/subtask groups and collapsible cards | `DependencyPlan.tsx` group `details` | existing grouped SSR test |
| Per-group progress and current priority | **Independent observation counts**, not task done | plan test + visible caveat |
| Waiting reasons and downstream release | Required predecessor/source checks and immediate impacted nodes | alternative-edges test |
| Search/filter and keyboard selectable nodes | `DependencyPlan.tsx` controls | SSR contract assertions; browser QA pending |
| Narrow viewport linear reading | `lineage.css` and `eta-plan.css` | CSS; real device QA pending |
| Dependency cycles | Fail closed for preparation DAG; state loops live in state view | cycle test |
| Live run history and editable dependency drag | **Not parity**: Arc is a signed-offline, read-only snapshot; changes/real execution use CLI | out of scope |
| Evidence trust | Go run/Oracle proof; the UI displays it without promoting observed prerequisites to PASS | existing suite, no new proof rules |

## Safety and correctness

Candidate providers and OR alternatives do not become mandatory precedence edges. Steps at the same level are **not** automatically safe to execute in parallel. `OBSERVED` means a qualified observation of the selected prerequisite; it never means that an ETA task is done, that the preparation action ran, or that the business under test passed.

The critical chain is a deterministic declaration projection. When the chain is larger than 48 nodes, the plan previews the beginning, current attention neighborhood, and end; all other nodes remain available in the full, untruncated group plan.

## Required acceptance before claiming 1:1

Run the workbench test/build pipeline; visually compare Arc and ETA at desktop, tablet and narrow mobile widths; verify group collapse, keyboard navigation, search/filter, deep-link selection and long-name clipping; and verify a real pay_gateway isolated run's Expected/Actual against its independent database Oracle. Until completed, label parity as **partial**, not 1:1 guaranteed.
