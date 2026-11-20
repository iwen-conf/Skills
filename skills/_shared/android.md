# Android Hard-Spec Cheatsheet

## 1. Execution & Device Surfaces

- **Default Surface (Remote Redroid)**: `ssh luwen@192.168.0.63` (`redroid/redroid:14.0.0_64only-latest` mounted with `/dev/binderfs`, helper `~/.cache/remote63-redroid-up.sh`, tunnel `ssh -f -N -L 15555:127.0.0.1:5555 luwen@192.168.0.63 && adb connect 127.0.0.1:15555`). Host lacks `/dev/kvm` and `64only` image has **no GMS**—mark Google Play / FCM / Credential Manager checks `BLOCKED` on redroid or run on an authorized GMS device/AVD.
- **Local AVD (`avdslim` Only)**: Manage solely via `~/.local/bin/avdslim` (`avdslim doctor`, `avdslim start [<n>] [--headless]`, `avdslim stop [--snap]`, `avdslim on`, `avdslim restore`). Never invoke raw SDK `emulator` or manual `-lowram`/`-gpu` flags.
- **Verification Discipline**: `--dry-run` or `help` checks task graphs only, never build/runtime PASS. Keep visual baselines immutable unless update is explicitly authorized.

## 2. AGP 9 Migration Floor

- Minimum versions when used: **KSP `>= 2.3.6`**, **Hilt `>= 2.59.2`**. Do **not** apply Android-app AGP 9 migration rules to KMP modules.
- Enable built-in Kotlin, migrate `kapt` -> `KSP`, set `buildFeatures.buildConfig = true` only in modules reading `BuildConfig`, and replace `applicationVariants.all` with public `androidComponents` Variant/Artifacts APIs (`android/gradle-recipes` branches `agp-9.0` / `9.1` / `9.2`).

## 3. Jetpack Navigation 3 (Never Regress to Nav 2 String Routes)

- Define `@Serializable` route keys (`data object Home`, `data class Detail(val id: String)`).
- Hold state in `rememberNavBackStack(...)` or `mutableStateListOf<Any>(Home)` and render with `NavDisplay(backStack = backStack, entryProvider = entryProvider { entry<Home> { ... } })` using entry-scoped ViewModels and Scenes (`Dialog`, `BottomSheet`, two-pane list-detail).

## 4. Edge-to-Edge, Adaptive & Security

- Call `enableEdgeToEdge()` in `ComponentActivity`; consume `WindowInsets.safeDrawing` / `ime` once without nested `Scaffold` double-padding; adapt across `WindowSizeClass` and foldable hinges.
- Set explicit `android:exported`, enforce `PendingIntent.FLAG_IMMUTABLE`, validate untrusted `Intent` extras/deep links, and verify R8 release shrink/keep rules.
