---
name: harmonyos-development
description: >
  Use when developing, reviewing, debugging, or migrating HarmonyOS NEXT native apps
  with ArkTS, ArkUI, DevEco Studio, DevEco Code, DevEco CLI, Stage model, UIAbility, .ets,
  module.json5, oh-package.json5, HAP/HSP/HAR, API 22-24, API 26 Beta1, HarmonyOS 6.1,
  permissions, persistence, networking, media, testing, performance, or common 鸿蒙开发 workflows.
  Prefer arkts-syntax-assistant for syntax-only migration; do not use for Android, web, or Lark work.
---

# HarmonyOS (鸿蒙) Development

This entry routes HarmonyOS work to the smallest relevant reference. Load the full guide only
when a topic is not covered by the focused references; do not load every platform topic for a
single question.

Before opening the 4,000+ line full guide, enumerate headings with
`rg -n '^#{2,3} ' references/full-guide.md`, then locate the requested topic with
`rg -n -C 4 '<term>' references/full-guide.md` and load only the matching section.

## Intent Router

| User intent | Load |
|---|---|
| SDK, API baseline, DevEco versions, API 26 preview | [`references/platform-baseline.md`](references/platform-baseline.md), then [`references/api26-preview.md`](references/api26-preview.md) for preview work |
| ArkTS syntax, strictness, TypeScript migration | [`references/arkts-rules.md`](references/arkts-rules.md) |
| ArkUI components, layout, rendering | [`references/arkui-components.md`](references/arkui-components.md) |
| Stage model, Ability lifecycle, module.json5 | [`references/stage-model.md`](references/stage-model.md) |
| Navigation and page stacks | [`references/navigation.md`](references/navigation.md) |
| State decorators and data flow | [`references/state-management.md`](references/state-management.md) |
| Permissions and privacy | [`references/permissions.md`](references/permissions.md) |
| Build, CI, signing, packaging, release | [`references/build-sign-release.md`](references/build-sign-release.md) |
| Performance, memory, large lists, startup | [`references/performance.md`](references/performance.md) |
| Native C/C++ API compatibility | [`references/native-api-compatibility.md`](references/native-api-compatibility.md) |
| DevEco Code/CLI, Agent Framework, app Skills, A2A | [`references/ai-development-tools.md`](references/ai-development-tools.md) |
| Common examples or a topic not covered above | [`references/full-guide.md`](references/full-guide.md) |

## Red Lines

```text
DO NOT mix API 26 Beta guidance into API 24 production answers unless the user targets preview.
DO NOT invent DevEco CLI commands, API signatures, permissions, or SDK behavior.
DO NOT recommend FA model for new HarmonyOS NEXT applications; use it only for migration context.
DO NOT load the full guide when a focused reference answers the request.
```

- State the target API/SDK assumption when behavior depends on it.
- Separate stable production guidance from preview/adaptation guidance.
- Verify uncertain signatures against the installed SDK or official documentation before generating production code.
- For debugging or optimization, load the repository-wide evidence-first root-cause repair gate in [`docs/execution-truth.md`](../../docs/execution-truth.md) before editing; confirm the issue, trace architecture ownership, compare the business contract, and verify the complete fix.
