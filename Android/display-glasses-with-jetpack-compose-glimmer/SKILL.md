---
name: display-glasses-with-jetpack-compose-glimmer
description: Provides guidelines for developing projected Android XR apps for display
  glasses using the Jetpack Compose Glimmer UI toolkit. This skill covers foundational
  Glimmer design principles, workflows for implementing Jetpack Compose Glimmer, and
  interaction models for the glasses form factor. Use this skill to build an Android
  XR Augmented Experience app with Jetpack Compose Glimmer that adheres to the Glimmer
  design system for optimized glasses styling.
license: Complete terms in LICENSE.txt
metadata:
  author: Google LLC
  last-updated: '2026-07-09'
  keywords:
  - Jetpack Compose Glimmer
  - audio glasses
  - display glasses
  - Projected Activity
  - GlimmerTheme
  - Additive Display
  - Android XR - Augmented Experiences
---

## Glossary

| Term | Definition |
|---|---|
| **Intelligent Eyewear** | All-day wear, hands-free devices that provide access to information. Equipped with speakers, a camera, and a microphone. Some are audio-only (audio glasses), and some also have a display (display glasses). |
| **Display Glasses** | Audio glasses with the addition of a small, private display for glanceable visuals that harmonize with audio output. |
| **Jetpack Compose Glimmer** | A Compose UI toolkit for building augmented Android XR experiences, optimized for display glasses. It provides components, theming, and behaviors for transparent displays. |
| **Projected Activity (Glasses Activity)** | An Android `Activity` that runs on a host device (phone) but its UI and interactions are projected to a connected, intelligent eyewear device (audio or display glasses). |
| **Projected Device** | An XR device connected to an Android-powered device (host). Host projects the application content to the Projected device and let users interact with it. |
| **GlimmerTheme** | The root provider for styling tokens, including GlimmerColors, GlimmerTypography, and GlimmerShapes. |
| **Additive Display** | A display technology where black (#000000) is rendered as 100% transparent. UI is built by adding light to the environment. Display glasses have an additive display. |
| **Augmented Experiences** | Android XR experiences that enhance a user's focus and presence in the real world. They are lightweight and additive, helping users while they are on-the-go |
| **Visual Angle** | A unit of measurement for perceived size in XR. The minimum readable text size is 0.6 degrees (approx. 18sp at 1 m). |

## Prerequisites

- Mobile project must target `compileSdk` 37 or higher. If the `compileSdk` is lower than 37, increase the `compileSdk` to 37.
- Ensure you are using the latest library dependencies from [Create your first
  activity for intelligent eyewear](https://developer.android.com/develop/xr/jetpack-xr-sdk/ai-glasses/first-activity).

## Core Constraints

- **Don't:** Use `MaterialTheme` or Material Components.
- **Do:** Use `GlimmerTheme` and Jetpack Compose Glimmer Components.
- **Do:** Use `createGoogleSansFlexTypography()` from `androidx.xr.glimmer:glimmer-google-fonts` as the `Typography` value for `GlimmerTheme` to ensure that consistent typography is applied throughout the components.

## Limitations

- Specific hardware device models or their unique capabilities are not detailed.
- **Elevation:** Standard Material 3 shadow or elevation modifiers are not supported. You must use `ShadowScope` or `Depth` tokens from Jetpack Compose Glimmer.
- **Minimum Size:** The absolute minimum size for readable text is 18sp (0.6°). Anything smaller will fail legibility checks.
- **Text Weight:** Avoid "Thin" or "Hairline" weights; they cause shimmering and aliasing on additive AR lenses.

## 1. Set up dependencies

- **Setup Projected Activity:** First, you need to create a new projected activity for your app. If the project doesn't already have one, see [Create
  your first activity for intelligent eyewear](https://developer.android.com/develop/xr/jetpack-xr-sdk/ai-glasses/first-activity). Use [references/projectedcontext-source.md](references/projectedcontext-source.md) to launch the Glasses Projected activity on the Projected Device. Ensure that you specify `xr_projected` for the `android:requiredDisplayCategory` attribute in app manifest to tell the system that this activity will use a projected context to access hardware from a connected device.
- **Mobile App Integration:** If the project contains an existing mobile app, you must create a new Glasses Activity dedicated to rendering Glimmer UI. For detailed configuration, heavily reference [Create your first activity
  for intelligent eyewear](https://developer.android.com/develop/xr/jetpack-xr-sdk/ai-glasses/first-activity). If there isn't already a method to launch the Glasses Activity, add a button to the existing mobile app UI labeled "Launch on Glasses" that uses `ProjectedContext` to launch the Glasses Activity on the glasses. Always keep this button in a highly visible location, such as an overlay Floating Action Button (FAB) or the top navigation bar, to ensure users discover the projection capability. If the glasses aren't connected, disable the button. Don't launch the Glasses Activity on the phone, only on the display glasses. If it makes sense to automatically launch the Glasses Activity without an explicit launch button, then do so.
- **UI Library:** Identify if the project has the `androidx.xr.glimmer:glimmer` library, if not it must be added to the project. See [Declaring Jetpack Compose Glimmer Dependencies](https://developer.android.com/jetpack/androidx/releases/xr-glimmer#declaring_dependencies) to fetch the latest dependency version.
- **Theming:** All Glimmer components must be wrapped within the `GlimmerTheme` composable to ensure correct token resolution.
- **Mandatory black background:** Display glasses use additive displays. Non-black backgrounds add light and reduce real-world contrast; they are not physically opaque pixels. **You must always** set a pure black background (`Modifier.background(Color.Black)`) on the root container of your Projected Activity.
- **Font:** The default font is Google Sans Flex. Use `androidx.xr.glimmer.googlefonts` library with the default type styles unless otherwise specified. Use `createGoogleSansFlexTypography` to create a `Typography` instance with the Google Sans Flex configuration. Provide this `Typography` instance as normal through `GlimmerTheme`. Use [references/glimmersansflextypography-source.md](references/glimmersansflextypography-source.md) for configuration.
- **Hardware Capabilities:** Different types of intelligent eyewear devices have different capabilities. To check for these at runtime, see the [Check
  device capabilities at runtime for intelligent eyewear](https://developer.android.com/develop/xr/jetpack-xr-sdk/ai-glasses/check-capabilities).
- **Hardware Permissions:** To request hardware permissions like the microphone and camera, see the [Request hardware permissions for
  intelligent eyewear](references/android/develop/xr/jetpack-xr-sdk/request-hardware-permissions.md).
- **Hardware Access:** To use the glasses camera, sensors, or access the phone's hardware, see the [Use a projected context to access hardware on
  intelligent eyewear](references/android/develop/xr/jetpack-xr-sdk/access-hardware-projected-context.md).

## 2. Minimize and translate the UI

- For display glasses, build UI using components from the Jetpack Compose Glimmer framework.
- Use depth to communicate element priority and hierarchy.
- Design from the bottom up, trying to minimize how much of the real world you cover. Always bottom align UI to the glasses display.
- **One Thing at a Time:** Prioritize the user's awareness of the real world. Show only one primary piece of information at a time (for example, using a `Stack`) to minimize obstruction of the user's field of view. Avoid competing primary cards; a focused list may show several related items within one task.
- **Color Contrast:** Ensure at least a 70% tone difference between foreground and background using the HCT color space. For calculation metrics, use [references/material-hct-source.md](references/material-hct-source.md).

## 3. Map input controls

- Map app interactions, such as tap and swipe, to the available hardware controls on the glasses, such as the touchpad.
- Inputs are more 1-dimensional; users typically make one control input at a time.
- Avoid nesting scrolling controls.
- Jetpack Compose Glimmer components are designed to work with standard input methods, such as a tap or swipe on the glasses' touchpad.
- Use System Back to dismiss temporary states or detailed views.
- To add input, focus, tap, swipe to your Glasses UI, follow [Focus in Jetpack
  Compose Glimmer](references/android/develop/xr/jetpack-xr-sdk/jetpack-compose-glimmer/focus.md).
  - For a detailed breakdown of hardware inputs, see [Hardware Controls for
    display glasses](references/android/design/ui/ai-glasses/guides/interaction/inputs.md)

## 4. Build with Jetpack Compose Glimmer

Jetpack Compose Glimmer is the UI toolkit for building augmented experiences on
display glasses.

### Key Features

- **Glimmer Theming:** A simplified glasses-specific theme for optimal visibility.
- **Pre-compatible Input:** Supports tap and swipe by default.
- **Pre-built Components:** Provides optimized composables like `Card`, `ListItem`, `Button`, etc.

### Focus Management in Glimmer

- Jetpack Compose Glimmer uses a one-dimensional focus search.
- Focus movement is continuous for scrollable containers and discrete for elements like a row of buttons.
- To enable the system to automatically set initial focus, you must set the `isInitialFocusOnFocusableAvailable` flag to `true` in your activity's `onCreate` method. For more information on how to implement, see [Focus in
  Jetpack Compose Glimmer](references/android/develop/xr/jetpack-xr-sdk/jetpack-compose-glimmer/focus.md).

### Implementing Glimmer Styles

Glimmer styles are accessed through the `GlimmerTheme` object. Use
[references/glimmertheme-source.md](references/glimmertheme-source.md) for reference.

| Category | Token | Value / Role |
|---|---|---|
| **Color** | primary | #9BBFFF (Focal color) |
| **Color** | secondary | #4C88E9 (Focal color) |
| **Color** | surface | #262626 (Dim emissive surface; not equivalent to transparent #000000) |
| **Color** | outline | #606460 (3.dp border color) |
| **Shape** | Standard | `RoundedCornerShape(36.dp)` |
| **Shape** | Small | `RoundedCornerShape(12.dp)` |

#### Typography Scale (Google Sans Flex)

**Strict default:** When creating Glimmer UI you must use Google Sans Flex
unless a custom brand typeface is explicitly specified.
**Variable Font Settings:** As Google Sans Flex is a variable font, you must
configure the following axes:

- **Roundness (`ROND`):** Always set to `100f` for the signature rounded appearance.
- **Width (`wdth`):** Set to `100f`.
- **Optical Size (`opsz`):** Set to `9f`.
- **Weight (`wght`):** Use specific values for different roles (Title: `725f`, Body: `520f`, Caption: `650f`).

| Style Name | Size / Line-Height | Weight Axis | Width | Roundness | Optical size |
|---|---|---|---|---|---|
| **Title Large** | 30.sp / 36.sp | 725 | 100 | 100 | 9 |
| **Title Medium** | 24.sp / 32.sp | 725 | 100 | 100 | 9 |
| **Title Small** | 20.sp / 28.sp | 725 | 100 | 100 | 9 |
| **Body Large** | 30.sp / 36.sp | 520 | 100 | 100 | 9 |
| **Body Medium** | 24.sp / 32.sp | 520 | 100 | 100 | 9 |
| **Body Small** | 20.sp / 28.sp | 520 | 100 | 100 | 9 |
| **Caption** | 18.sp / 28.sp | 650 | 100 | 100 | 9 |

#### Depth Levels

Simulate depth on display glasses using shadows to establish a sense of
hierarchy through varying levels of emphasis. The Jetpack Compose Glimmer
controls use `DepthEffect` with 5 preset `DepthEffectLevels`. Use
[references/deptheffect-source.md](references/deptheffect-source.md) and
[references/deptheffectlevels-source.md](references/deptheffectlevels-source.md) for reference.

Some examples:

| Level | Usage |
|---|---|
| **level1** | Standard rest state for cards and persistent background UI. |
| **level2** | Standard focus/pressed state for buttons and interactive cards. |
| **ExtraSmall** | 4.dp |

### Components

When implementing Cards, Buttons, Icons, Lists, Text or Surface, read the matching section of [component guidance](references/components.md); retain its focus, nesting and interaction constraints.

## 5. Integrate with system UI

- For a detailed breakdown of notifications on intelligent eyewear, see [Understand notification behavior for intelligent eyewear](https://developer.android.com/develop/xr/jetpack-xr-sdk/ai-glasses/notifications/behavior) and learn how to [Start a glasses activity on display glasses from a notification](https://developer.android.com/develop/xr/jetpack-xr-sdk/ai-glasses/notifications/start-activity).
