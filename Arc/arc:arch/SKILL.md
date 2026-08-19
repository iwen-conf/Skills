---
name: arc:arch
version: 1.0.0
description: >
  Applies the default backend architecture, DIP, layering, zap logging, Go
  constants, and helper limits. Use when writing backend or Go services, 架构,
  DIP, usecase, repository, 分层, or reviewing layering. Not for frontend stack
  choices.
---

# Project Architecture Conventions

## Overview

Use this skill before writing, changing, or reviewing backend, service, controller, repository, infrastructure, helper, logging, debugging, constants, enum-like states, or project skeleton code.

For new backend modules, use this architecture by default. For existing repositories, preserve established local patterns unless the task explicitly asks to migrate.

`SKILL.md` holds the router and always-on gates. Load [`references/backend-architecture.md`](references/backend-architecture.md) for file-level naming, DTO composition, usecase results, zap, constants, and review checks.

## When to Use

- Before implementing backend, service, controller, repository, integration, or helper code.
- Before creating a new module, package, business feature, command entrypoint, or project skeleton.
- When adding or reviewing zap logs, constants, enum-like state, or error reporting.
- When reviewing DIP, layer leaks, helper placement, or magic literals.
- Use with `arc:comment` when adding comments. Not for frontend stack selection (`arc:frontend`).

## Intent Router

| When | Load |
|---|---|
| Any backend edit | this SKILL.md Red Lines, DIP, Go file constraints, ponytail |
| Directory layout, DTO/entity/model, usecase results, wire, zap, constants | [`references/backend-architecture.md`](references/backend-architecture.md) |
| Identity keys, env/branch surface, paper-over risk | [`docs/execution-truth.md`](../../docs/execution-truth.md) |
| Frontend stack / UI | `arc:frontend` |
| Large tracked change | `arc:sdlc` then this skill |
| Comments | `arc:comment` |

## Context Search

- MUST load the **Evidence-first root-cause repair** section in [`docs/execution-truth.md`](../../docs/execution-truth.md) when reviewing or changing a suspected defect, regression, or performance issue.
- MUST inspect the host repository before proposing an architectural change: package layout, constructors, contracts, DTOs, tests, and dependency direction.

## Red Lines

```text
NO INFRASTRUCTURE INSIDE BUSINESS POLICY.
NO ARCHITECTURE CHANGE FOR AN UNCONFIRMED FAILURE.
TRACE OWNERSHIP AND INVARIANTS BEFORE PROPOSING A PATCH.
NO THIRD PRIVATE FUNC IN ONE GO FILE — SPLIT TO <name>_helpers.go.
NO DOMAIN/GATEWAYS PACKAGE.
NO ENTITY ToDTO / ToResponse.
NO TEMP PACKAGE-LEVEL VAR FLAGS TO BYPASS PRODUCT RULES.
NO INVENTED DOMAIN IDENTITY KEYS.
NO PONETAIL SILENCE — REPORT ABSENCE OR CONFLICT.
```

- MUST keep every Go file at no more than two unexported/private `func` declarations.
- MUST create a sibling `<original>_helpers.go` when an original file reaches two private functions.
- MUST NOT use `var _ Interface = (*Struct)(nil)`; return the interface from the constructor.
- MUST apply [`docs/execution-truth.md`](../../docs/execution-truth.md) when identity keys, env/branch surface, or paper-over risks appear.
- MUST separate an architectural hypothesis from the business contract: trace the owning boundary first, then confirm the proposed behavior against real domain rules and user-visible semantics before changing code.
- MUST identify analogous call sites governed by the same boundary before declaring a localized change complete.

## Architecture Preflight

1. Inspect the host repository first: package layout, constructors, contracts, DTOs, tests, and dependency direction.
2. For new modules, use the default architecture unless the repository already has a stronger local convention.
3. For migrations, change the smallest slice that preserves behavior.
4. Read `references/backend-architecture.md` when file-level or interface-level guidance is needed.
5. Do not invent extra layers, factories, interfaces, or helpers beyond the boundaries described here and in the reference.

## Common Drift Patterns

Reject these patterns even when they "make the build green":

1. **Workflow in `domain/services`**: application orchestration belongs in `usecase/<module>`.
2. **Temporary package-level `var` flags** that bypass product rules without an explicit task and reviewable config path.
3. **Type assertions** to grab optional infrastructure capabilities that should be constructor-injected contracts.
4. **Hard-coded epochs, auth schemes, magic principal IDs, or "default 1"** where typed constants, config, or domain values are required.
5. **Invented domain identity / routing keys** not present in project contracts or docs.
6. **Compatibility shims** kept "for history" when the user forbids compatibility and the product is not shipped.
7. **Compile-time `var _ Interface = (*T)(nil)` assertions**.

## Ponytail Preflight

- Before writing code, load and read the installed `ponytail` skill.
- If the environment exposes `$ponytail`, use that skill first.
- Known fallback locations: `~/.claude/plugins/marketplaces/ponytail/skills/ponytail/SKILL.md` and `~/.codex/.tmp/marketplaces/ponytail/skills/ponytail/SKILL.md`.
- If `ponytail` cannot be found, report that explicitly before editing; do not invent ponytail rules.
- If `ponytail` conflicts with this skill, stop and report the conflict.

`ponytail` rejects unrequested abstractions such as an interface with one implementation. This skill explicitly requests DIP boundary interfaces when they protect business logic from infrastructure, external capabilities, framework details, or transport boundaries.

- Required DIP boundary interfaces are not "unrequested abstraction".
- Do not create service interfaces, factories, or adapter interfaces solely because a folder exists.
- Do not add an interface for private helpers or same-layer calls unless a real boundary, test seam, or second implementation exists.
- If a ponytail simplification would remove a required DIP boundary, keep the boundary and shrink everything around it.

## DIP Rules

- High-level business policy depends on abstractions, not concrete storage, SDK, network, queue, cache, or framework implementations.
- `usecase/<module>` owns application behavior and consumes `domain/repositories` or explicit capability contracts.
- Concrete infrastructure is wired in `internal/wire`; it is not constructed inside business logic.
- Controllers translate transport input/output; they must not contain core business decisions.
- New dependencies are injected through constructors or explicit parameters, not pulled from globals.

## Review Checklist

- Host repository patterns were inspected before applying the default architecture.
- Drift patterns above are absent.
- Business logic is in `usecase/<module>`, not controllers, infrastructure, or `wire`.
- No `internal/domain/gateways` (or ports/adapters/clients) package exists.
- Injected repository fields use a `Repo` suffix.
- Usecase `Contract` methods return named result types, not raw entities.
- List/query contracts return success with empty collections for no-data results.
- Zap is initialized once, injected, structured, and free of secrets/payload dumps.
- Go constants use `MixedCaps` / `mixedCaps` and standard-library semantic constants.
- No Go file contains more than two private functions.
- `ponytail` was read before coding, or its absence was reported.
- Details in `references/backend-architecture.md` were loaded when DTO, logging, constants, or wire rules were in play.
