# Native platform direction

User decision, 2026-10-06: client products target native platforms; cross-platform UI reuse is no
longer a goal. Native means the target SDK, UI framework, lifecycle, input/accessibility and system
integration. Machine-code compilation, native-looking controls or an installable web shell alone
do not satisfy this direction.

## Selection

| Target | Native direction for new work |
|---|---|
| Android | Kotlin + Jetpack Compose; Android SDK, lifecycle, navigation and platform services; retain Views interoperability where needed |
| iOS / iPadOS | Swift + SwiftUI; UIKit for platform capabilities or existing screens that need it |
| HarmonyOS | ArkTS + ArkUI and the target HarmonyOS SDK/Stage lifecycle; no default ArkUI-X route |
| macOS | Swift + SwiftUI/AppKit; native windows, menus, shortcuts and system integration |
| Windows | Windows SDK and platform UI such as WinUI/Win32, chosen for the actual deployment constraints |
| Linux desktop | First establish the target desktop/distribution and its native UI/toolkit requirements; do not choose a web wrapper merely to share a desktop codebase |
| WeChat Mini Program | Platform-native JavaScript/TypeScript + WXML/WXSS + native components/APIs and WeChat tooling |
| Other Mini Program | That platform's native runtime, components and official tooling |
| Web | Browser application; the Web conventions remain applicable and do not define a native app stack |

These are project selection defaults, not claims about the latest SDK versions. Check the actual
deployment target and installed SDK/official documentation before choosing version-sensitive APIs.
Do not add platforms the user did not request. Each requested platform gets its own acceptance
evidence; identical UI across platforms is not a goal.

## Boundaries and reuse

- Do not default to React Native/Expo, Flutter, Capacitor/Ionic, Tauri/Electron, Taro/uni-app,
  ArkUI-X or shared multiplatform UI to maximize reuse. Do not introduce KMP or another shared
  client layer solely for hypothetical future targets. An explicit later user choice is honored.
- Existing cross-platform maintenance stays scoped: fix the requested behavior in its current
  stack. This policy is not permission to rewrite a product, remove dependencies or abandon its
  shipped clients. A requested native migration needs a concrete platform, feature parity,
  data/integration boundaries and staged acceptance. Do not force a native screen into an existing
  runtime without establishing its integration scope.
- Share backend services, HTTP contracts, business meanings and suitable assets where useful.
  Existing justified native libraries or portable protocols are not automatically defects.
  Native client navigation, rendering, accessibility and lifecycle remain platform-owned.
- Use native state/observation, structured concurrency and networking/serialization conventions.
  Web libraries, DOM selectors, CSS classes, rem units and React directory layouts apply to Web,
  not to Kotlin, Swift or ArkTS. Native themes use semantic platform resources and system settings;
  translate brand intent rather than duplicating a web stylesheet.
- Platform-specific UX is part of acceptance: system back/navigation, safe areas/insets, keyboard,
  focus, permission flows, text scaling, accessibility, dark mode and process/state restoration,
  selected according to the feature. A browser preview does not prove native behavior.

Planning records the target SDK and required platform capabilities; implementation follows the
target stack; audit checks new work against this direction without calling existing legacy code a
new regression. Verification uses [platform tools](../../arc:verify/references/stacks.md) and the
real target environment. Missing SDK/device access is `NOT_RUN`/`BLOCKED`, never a reason to switch
to a cross-platform implementation or to claim a web simulation as native proof.
