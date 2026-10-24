# Visual Iteration from Evidence

Use for improvements to an existing rendered surface, including repeated screenshot complaints.
The current product and user-provided reference define the direction. New-product design can use
an available design skill; this procedure does not restart branding or override the project's stack.

## 1. Lock the comparison

Before editing, name:

- **Target:** the specific surface, viewport/window dimensions and UI state to improve.
- **Preserve:** adjacent surfaces, content, interactions and business behavior outside the change.
- **Evidence:** the current render, any reference render or known-good version, and the exact
  visible difference (alignment, spacing, contrast, density, typography or clipped content).

Only ask when unresolved alternatives materially change the deliverable. Existing authorization
covers a concrete in-scope correction; silence is never authorization for a redesign.
Locate owning components/tokens with `arc-idx`, then inspect sibling consumers. A deterministic
rendering, data, font or timing regression uses arc:code repair with this visual evidence.

## 2. Fix the shared cause

Prefer the existing token, layout constraint or component boundary that governs the defect.
Do not tune one screenshot while leaving the same owning constraint broken on siblings.
Measure geometry when the claim is alignment or fit; preserve readable type instead of shrinking
text merely to hide overflow. Do not substitute a screenshot of a recreated mock for the product.

If subjective tuning remains unresolved after two concrete values, present a small set of labeled
variants on the actual product background, varying only that property. This is a taste decision,
not evidence that runtime behavior is repaired.

## 3. Verify the applicable visual states

Freeze the relevant matrix before claiming completion; omit irrelevant rows with a reason:

| Dimension | Evidence to select |
|---|---|
| Shared consumers | Affected sibling pages/components discovered with arc-idx |
| Size | Both sides of affected web breakpoints; native minimum width/height and normal size |
| State | Default, selected/focused, disabled, loading, empty/error and localized content that can occur |
| Timing | Before/during/after a changed transition; cold/warm entry for initialization defects |
| Input | Pointer, touch or keyboard paths supported by the affected control |

Use the available browser/device tools within the user's authorized environment; respect native
platform test and computer-use instructions. A final still cannot prove first paint, touch feedback
or transition stability. Record the measured before/after geometry and viewport for alignment claims,
or capture frames/a short recording for motion claims. Re-check the preserve boundaries.
If the environment cannot render, report that exact missing check and the shortest reproducible
view/action; do not call the visual outcome verified from compilation alone.

## 4. Motion remains meaningful

Use project/platform motion tokens named by interaction, not isolated duration literals.
High-frequency controls may use immediate feedback. Press feedback may be color, material,
native ripple, haptic or motion; neither universal scale animation nor a blanket keyboard-animation
ban is a design requirement.

Reduced-motion settings remove or simplify movement while preserving feedback: show a static
progress value, step or text status when a pulsing/sliding cue would otherwise freeze and resemble
a hang. Use native primitives according to their actual semantics; compositing/rasterization is
not a generic performance hint and requires measured justification.

Derived guidance is distributed under the [Waza MIT license](../../../Waza/LICENSE).
