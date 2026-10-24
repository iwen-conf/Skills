---
name: android
description: >-
  Native Android work with Google's official guidance: the android CLI (create projects, run on devices, emulator
  screenshots, SDK, official docs), adaptive layouts for phones, tablets, foldables, desktop, TV, Auto and XR,
  edge-to-edge and insets, Navigation 3, Jetpack Compose Styles, migrating XML views to Compose, Wear OS Compose
  Material 3, Android XR display glasses (Compose Glimmer), CameraX, AppFunctions, Credential Manager verified
  email, Engage SDK, Play Billing upgrades, AGP 9 upgrades, R8 keep-rule analysis, Intent and component security
  audits, Perfetto traces and SQL for jank, latency and memory, and Android test setup (unit, UI, screenshot,
  E2E). 安卓, Compose, 原生 Android. Not for Kotlin Multiplatform AGP migration, HarmonyOS (use harmonyos), or iOS.
metadata:
  library:
    - android-cli
    - adaptive
    - edge-to-edge
    - navigation-3
    - styles
    - migrate-xml-views-to-jetpack-compose
    - wear-compose-m3
    - display-glasses-with-jetpack-compose-glimmer
    - camerax
    - appfunctions
    - verified-email
    - engage-sdk-integration
    - play-billing-library-version-upgrade
    - agp-9-upgrade
    - r8-analyzer
    - android-intent-security
    - perfetto-sql
    - perfetto-trace-analysis
    - testing-setup
---

# android

One entry for native Android work. Pick exactly one library entry per step from the table and follow it. New
clients follow [native platform direction](../../Arc/_shared/conventions/native-platforms.md); builds, devices
and visual checks follow [Android execution](../../docs/android-execution.md) (remote redroid by default, local
AVD only through avdslim). Diagnosis, crash, jank or failed-build work first applies the evidence-first
root-cause repair gate in [execution-truth.md](../../docs/execution-truth.md) §6.

## Route

| Request | Entry |
|---|---|
| Create a project, run on a device, emulator screenshots or UI dump, SDK packages, official docs lookup, install official skills | [android-cli](../android-cli/SKILL.md) |
| Window sizes, foldables, tablets, desktop, TV, Auto, XR; pointer and keyboard input; multi-pane layouts | [adaptive](../adaptive/SKILL.md) |
| Content under system bars, IME insets, system bar legibility | [edge-to-edge](../edge-to-edge/SKILL.md) |
| Navigation 3: back stacks, deep links, scenes, dialogs, list-detail | [navigation-3](../navigation-3/SKILL.md) |
| Compose Styles API, component themes, styleable components | [styles](../styles/SKILL.md) |
| Migrate XML layouts and views to Compose | [migrate-xml-views-to-jetpack-compose](../migrate-xml-views-to-jetpack-compose/SKILL.md) |
| Wear OS UI with Compose Material 3 | [wear-compose-m3](../wear-compose-m3/SKILL.md) |
| Android XR display glasses, Jetpack Compose Glimmer | [display-glasses-with-jetpack-compose-glimmer](../display-glasses-with-jetpack-compose-glimmer/SKILL.md) |
| Camera capture, recording, ML Kit or Media3 effects | [camerax](../camerax/SKILL.md) |
| Expose app workflows to agents and the system (AppFunctions) | [appfunctions](../appfunctions/SKILL.md) |
| OTP-less verified email via Credential Manager | [verified-email](../verified-email/SKILL.md) |
| Engage SDK integration or errors | [engage-sdk-integration](../engage-sdk-integration/SKILL.md) |
| Upgrade Play Billing Library | [play-billing-library-version-upgrade](../play-billing-library-version-upgrade/SKILL.md) |
| Upgrade to AGP 9 (not KMP projects) | [agp-9-upgrade](../agp-9-upgrade/SKILL.md) |
| R8 keep rules, app size, broad or redundant rules | [r8-analyzer](../r8-analyzer/SKILL.md) |
| Audit exported components, Intents, PendingIntents, providers | [android-intent-security](../android-intent-security/SKILL.md) |
| A Perfetto trace: root-cause jank, latency or memory | [perfetto-trace-analysis](../perfetto-trace-analysis/SKILL.md) (uses perfetto-sql) |
| Write a specific Perfetto SQL query against a trace | [perfetto-sql](../perfetto-sql/SKILL.md) |
| Testing strategy and harnesses for an Android app | [testing-setup](../testing-setup/SKILL.md) |

A request spanning two rows (for example AGP upgrade then R8 cleanup) runs the entries in order, one at a time.
General test evidence, verdicts and release gates stay with `arc:verify`; code changes follow `arc:code`.

## Completion

- [ ] One entry chosen per step and followed; target SDK/AGP versions stated when behavior depends on them.
- [ ] Builds, tests and device checks report the command and result, or `NOT_RUN` / `BLOCKED` with the reason.
