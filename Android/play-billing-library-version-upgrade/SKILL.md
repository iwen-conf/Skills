---
name: play-billing-library-version-upgrade
description: Use this skill when upgrading or migrating an Android project from any
  legacy Google Play Billing Library (PBL) version to the latest stable version of
  PBL.
license: Complete terms in LICENSE.txt
metadata:
  author: Google LLC
  last-updated: '2026-07-02'
  keywords:
  - android
  - play billing
  - play billing library
  - pbl
  - upgrade
  - migration
  - deprecation
  - google play
---

## Phase 0: Intent Message

**Reporting Action**: Before proceeding, immediately tell the user: "I will
upgrade Play Billing Library to the latest version."

## Phase 1: Discovery \& Situational Awareness

1. **Primary Check (Build Version)** : Locate the project's billing dependency (e.g., `com.android.billingclient:billing`) in `build.gradle`, `build.gradle.kts`, or `libs.versions.toml`.
2. **Initial Compilation Test**: Attempt to sync and build the project immediately.
3. **Resolved baseline**: Use target-variant dependency resolution and preserve the original build error. Deprecated APIs such as SkuDetails are migration clues, not proof of the installed version; mixed/dead code may remain.
4. **Target**: Use the explicitly requested version, otherwise the compatible stable upgrade target. Calculate the path from resolved dependencies and documented breaking changes.



- **Calculate Migration Path** :
  - If the **resolved baseline** is within 2 major versions of the target: Plan a **Direct Migration**.
  - If it is more than 2 major versions behind: Plan a **Stepped
    Migration**. Migrate by two major versions at a time (e.g., v4 -\> v6 -\> v8) until you are within two versions of the target.
- **Reporting Action**: Before proceeding, tell the user: "I've detected you are effectively on PBL \[Current\] and the latest is \[Target\]. I am planning a \[direct/stepped\] migration path."

## Phase 2: Contextual Document Mapping \& Planning

For every major version jump identified in your path, you **MUST** synthesize
instructions from:

- **[Migration Guide](https://developer.android.com/google/play/billing/migrate-gpblv%5BX%5D)** (where `[X]` is the target major version).
- **Release Highlights** : The "Deprecations" and "Breaking Changes" sections of the relevant [Release Notes](references/android/google/play/billing/release-notes.md).
- **Developer Documentation**: Consult your knowledge of the Google Play Billing documentation regarding the relevant features used in this app (e.g., Subscriptions, One-Time Products).
- **Develop the Plan**: Identify every specific code change required (API removals, class replacements, logic shifts) and print this out as a checklist.

## Phase 3: Instructions for Execution

*Reporting Action: For each of the following steps, give a brief explanation of
what you will be doing prior to execution, and a brief summary of what you
accomplished afterwards.*

### Step 1: SDK \& Environment Alignment

- **Action** : Update `build.gradle` to meet SDK requirements (e.g., "PBL 9 requires `compileSdk` 35").
- **Gradle Version**: Verify if the new library requires a newer Android Gradle Plugin (AGP) or Kotlin version.

### Step 2: Intent-based Refactoring

Analyze the intent of the existing code rather than performing purely textual
string replacement.

- **Action** : You **MUST** follow all deprecation instructions and refactor patterns from **both** the [references/migration-logic.md](references/migration-logic.md) section, the official migration guides, and the general documentation pages identified in Phase 2.
- **Verification** : Verify you are doing **all steps from all documentation** and then making sure you follow the specific directions from the checklist in the references.

### Step 3: Sequential Verification (Only applicable for Stepped Migrations)

1. **Upgrade** to the first major intermediate version in your path.
2. **Run `./gradlew assembleDebug`** to verify no intermediate breaking changes were missed.
3. **Repeat** until you reach the final target version.

### Step 4: Final Validation Checklist

1. **Smart Checklist Verification**:
2. Open [references/version-checklist.md](references/version-checklist.md) and locate the **Smart Version-Specific Checklist**.
3. **Action**: For every version between your \[Detected Effective Version\] and \[Detected New Version\], verify that every item has been addressed in the code using the host arc-idx search/structural interface.
4. **Tests** : Run all unit and implementation tests (`./gradlew test`).
5. **Build and business behavior**: Run the actual target variant assemble and relevant purchase/restore/acknowledgement tests once. Sync via the project-supported mechanism; do not assume a `sync` Gradle task exists or repeat clean/build without evidence. Apply [Android execution](../../docs/android-execution.md); unavailable billing/device evidence is NOT_RUN.

## Final Report

Explain the "Why" to the developer:

- "I updated your SDK to \[Version\] because PBL \[Version\] requires it for \[Reason from docs\]."
- "I removed your custom `retryConnection()` logic because it is now handled natively by the library using `enableAutoServiceReconnection()`."
- "Successfully upgraded from PBL \[Old\] to PBL \[New\] and verified with unit tests. Based on an analysis of features in the latest library and this application's current feature set, I suggest exploring \[New Feature\] (e.g., Prepaid Plans or Installments) from the latest release because it is now available but not yet implemented."
