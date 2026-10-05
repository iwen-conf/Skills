---
name: testing-setup
description: Analyze and create a testing strategy for native Android apps - install
  testing libraries, set up test infrastructure, create harnesses for unit tests,
  UI tests, screenshot tests, and end-to-end tests.
license: Complete terms in LICENSE.txt
metadata:
  author: Google LLC
  last-updated: '2026-06-25'
  keywords:
  - android
  - testing
  - ui tests
  - screenshot tests
  - coverage
---

## Scope

Select only the requested test layers and the risks of the changed behavior. Existing setup/one test does not require all 14 steps. Apply [Android execution](../../docs/android-execution.md); Maestro is the host default for Android E2E, and existing native frameworks remain valid for their layers.

## Step 1: analyze the current testing setup

To understand the testing setup of an existing project, look for these
dependencies in the libs.versions.toml file, or build files:

1. Dependency Injection framework used. Examples: Hilt, Koin, Anvil, vanilla Dagger...
2. Unit (local) testing framework this project uses, Example JUnit4, JUnit5...
3. Mocking framework (if any) used for unit tests, and for Instrumented and UI tests. Examples: Mockito, Mockk...
4. Robolectric. It can be used in 3 ways:
   1. Used in unit tests to have fakes for platform entities
   2. To run behavior UI tests without a device or emulator. For example, used to run Espresso or Compose tests.
   3. To do screenshot testing with Roborazzi
5. Is the app 100% Compose, Views or hybrid?
6. Behavior UI tests:
   1. Compose Tests (`androidx.compose.ui:ui-test-*`)
   2. Espresso Tests for Views. Might use wrappers like Kaspresso. Dependencies: `androidx.test.espresso:espresso-core`, `androidx.test:runner`, `androidx.test:rules`.
7. Screenshot tests can be:
   1. Instrumented (device-based). For example, using Dropshots.
   2. Based on Robolectric, so they run locally. For example, using Roborazzi.
   3. Based on LayoutLib, so they run locally. For example, Paparazzi, or the Compose Preview Screenshot Testing tool.
8. End to end tests (also known as Release Candidate tests) always run on device and use high-level frameworks such as UIAutomator, Appium, or Robotium.
9. Generate a Markdown report with the analysis

## Step 2: Set up Dependency Injection frameworks for testing

Reuse constructor/manual injection and current test seams. Do not install Hilt/Koin or migrate production architecture merely to add tests. Framework-specific setup below applies only when that framework is already used or its introduction is explicitly in scope.

Install the testing dependencies (for example `com.google.dagger:hilt-compiler`
that should be applied with a `kspAndroidTest` configuration).

> [!IMPORTANT]
> **Important:** Always consult the documentation of the applicable framework to learn about testing (for example: [Hilt testing guide](references/android/training/dependency-injection/hilt-testing.md), [Koin Instrumented tests](https://insert-koin.io/docs/reference/koin-android/instrumented-testing)).

For instrumented tests, create and configure (by adding
`testInstrumentationRunner` to the build gradle files) a new test runner and
apply the testing rules required by the framework (for example in Hilt, annotate
your test classes with `@HiltAndroidTest` and apply the `HiltAndroidRule`).
Other frameworks use other mechanisms, consult their documentation.

## Step 3: Install frameworks

Unless otherwise specified, respect the current stack of testing frameworks.

If a requested layer lacks a framework, add only what it needs:

- Unit: the project's JUnit generation; mocking only if a fake is impractical.
- UI: Compose Testing APIs or Espresso; Robolectric only for suitable local platform simulations.
- Screenshot: one suitable renderer (Preview/Paparazzi/Roborazzi) or device framework when system UI fidelity is required. See [Compose screenshot setup](references/android/studio/preview/compose-screenshot-testing.md).
- E2E: host-default Maestro; use UI Automator only when a required native/system interaction cannot be covered there.
- Coverage tooling only if a project gate or requested report needs it.

Do not install the entire list as a default.

## Step 4: Refactor and create fakes for testing

### **Refactor for unit tests**

In the next sections you'll be asked to create tests. If you have dependencies
on Android framework classes, or entities that are not part of the codebase:

- First, use a fake. If it doesn't exist, create an interface for the class
  and a "Default" implementation with the existing code. Add the Fake version
  to the test sourceset (test or androidTest).

- If not possible to use a fake (example: no access to the class or
  interface), mock the dependencies.

### **Refactor for UI tests**

If you need to fake components to make testing easier and faster and more
reliable, replace slow and problematic dependencies with fakes. Use runtime fakes through the existing injection seam to:

- **Simulate** different scenarios with the user (wrong credentials, reset password flow...), with a server (no connection, server down, bad JSON from server...) or with a platform component (insufficient permissions, no disk space, no front camera available)
- **Improve** speed and reliability (replacing a database with an in-memory database, replacing a repository with an in-memory fake to avoid hitting the network)

## Step 5: Unit testing

Cover changed business invariants and their meaningful failure paths. Reuse existing cases; do not create tasks per source file or tests that mirror implementation. Pure layout/DI wiring does not imply unit coverage.

## Step 6: UI testing

Use `test` for explicitly selected Robolectric/local cases; use `androidTest` for native instrumented behavior. Choose the layer by required platform fidelity.

## Step 7: Test databases

If the database is using SQLite (using Room, SQLDelight, etc.), create
instrumented tests using an in-memory database to make sure that they work with
the SQLite engine on device.

## Step 8: Screenshot tests

Irrespective of the framework used, screenshot tests focus on 2 types of tests:

- Screen-level cases at relevant compact/medium/expanded boundaries; choose dimensions from actual layout risks instead of requiring nine sizes for every screen.
- Screen-level variations. Add a mobile (400x500) screenshot of:
  - All the alternative themes, if used.
  - Font scale set to 1.5.
- Component-level screenshot tests, where each component is tested in different themes and font scales.

Behavior isn't tested with screenshots, but do test different common scenarios
if their UIs change a lot depending on the state. For example, test loading
screens by injecting a loading state to the UI or simulating it with a fake.

## Step 9: UI Behavior tests

Test the UI logic using behavior tests, which ensures that the UIs react as
expected when different states are passed, and when user actions are performed.

### **Compose UI behavior tests**

- Use the ComposeTestRule with a `ComponentActivity` to access resources such as strings.
- Always try to match with semantic matchers first. If the matcher is too complicated to write (using more than 3 matchers to find a single element), use `testTag`.
- Always verify state restoration

### **Views (XML) UI behavior tests**

Use Espresso to match views and interact with them.

## Step 10: Navigation tests

Create a test suite to verify navigation logic. Include:

- Back handling
- Deeplinks
- Special patterns like "exit through home" with multiple backstacks.

## Step 11: Simulate different window sizes and settings

For Compose layouts, use `DeviceConfigurationOverride` described in "[UI testing
common patterns](references/android/develop/ui/compose/testing/common-patterns.md)" to simulate different window sizes, font scales

## Step 12: End-to-end tests

Cover critical user journeys and relevant failure transitions using Maestro or the established project E2E runner. Test counts/percentages are not risk coverage. Native system interactions may need UI Automator.

If you need to take screenshots of the app running in a device, use
[Dropshots](https://raw.githubusercontent.com/dropbox/dropshots/refs/heads/main/README.md). You need a device for screenshot tests when verifying
interaction with the system UI (examples: edge-to-edge rendering, notifications,
picture-in-picture)

### Step 13: Instrumented Screenshot tests

Only when device screenshots are needed and no suitable framework exists, install the `com.dropbox.dropshots` plugin in the module and a `Dropshots()`
JUnit Rule. Create a new instrumented screenshot test for one of the app's
features.

### Step 14: Install jacoco

Install jacoco only if the requested coverage report or existing project gate requires it.

- Add the `jacoco` plugin to each module that contains tests.

## Final touches

When tooling or test commands change, update existing project documentation and relevant AGENTS facts. A one-test addition needs no new documentation hierarchy. Report passed, failed and NOT_RUN checks separately; stop when required risk evidence and project gates are satisfied.
