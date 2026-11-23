---
name: arc-code
version: 4.1.0
description: Implement features, evidence-first bug repairs, refactors, APIs, and clean Web or native mobile/desktop UI (Android, HarmonyOS, iOS/macOS, Web) inside `write_scope`, gated by `gate.sh`. Triggers on 写代码, 实现功能, 修复报错, 根因修复, 重构, 前端页面, UI设计, 界面重构, 去AI味, Android开发, 安卓, 鸿蒙开发, HarmonyOS, ArkTS, SwiftUI, iOS开发.
---

# arc-code

Ship the smallest complete diff wired to the real entry point.

## Protocol

1. Read `../_shared/contract.md` and load **only** the `_shared` specs matching touched files:
   - Any Web, mobile, or desktop UI -> `../_shared/ui-system.md`
   - Go (`*.go`, `go.mod`) -> `../_shared/go-conventions.md` (`iwen-example` Explicit Architecture, external `请求体`/`响应体` + internal `// 1. xxx` comments, `<= 1200` lines per `.go` file)
   - HTTP / JSON / Upload API -> `../_shared/api-contract.md`
   - Android (`*.kt`, `build.gradle*`, `AndroidManifest.xml`) -> `../_shared/android.md`
   - HarmonyOS (`*.ets`, `module.json5`, `hvigorfile.ts`) -> `../_shared/harmonyos.md`
   - Apple (`*.swift`, `Package.swift`, `*.xcodeproj`) -> `../_shared/apple-native.md`
2. **Reuse Lookup Before Writing (`禁止重复造轮子`)**: Query stdlib (`slices`, `maps`, `cmp`, `strings`, `time`) and `arc idx search` / `arc idx symbol` before adding helpers. Extract generic non-business functions into topic-named shared packages (`pkg/<topic>` or `internal/pkg/<topic>`) with unit tests rather than duplicating private helpers across business packages.
3. **Baseline & Root-Cause Repair**: Run `bash scripts/gate.sh start --task <slug>`. Prove bug root cause at `file:line` with a failing test (`gate.sh red`) before editing source; fix sibling consumers sharing the broken invariant.
4. **Structure, Comments & Scope Discipline**: Edit strictly inside `write_scope`; enforce stack file-size limits (`.go <= 1200` lines) and mandatory external/internal comment conventions; request explicit contract update if cross-boundary types change.

## Hard Gate

Run `bash scripts/gate.sh check --task <slug>`, `arc update --changed-only`, and `arc index flush --repo .` before handoff or completion.
