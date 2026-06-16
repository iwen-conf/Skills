---
name: project-architecture-conventions
description: Apply mandatory DIP and ONC-style architecture rules before writing or reviewing project code.
---

# Project Architecture Conventions

## Overview

Use this skill before writing, changing, or reviewing project code. All projects must follow the Dependency Inversion Principle (DIP) and the ONC-style architecture described here.

Do not read an external ONC project to discover the architecture while coding. Treat this SKILL.md as the source of truth for the ONC-style layer layout.

## When to Use

- Use before implementing backend, service, controller, repository, integration, or helper code.
- Use before creating a new module, package, business feature, command entrypoint, or project skeleton.
- Use when reviewing whether code violates DIP, leaks infrastructure into business logic, or places helpers in the wrong layer.
- Use together with `code-comment-conventions` when adding comments to functions, controllers, or implementation steps.

## Ponytail Preflight

- Before writing code, load and read the installed `ponytail` skill from the local environment.
- If the environment exposes it as `$ponytail`, use that skill first.
- If it is installed under a skill directory, read its `SKILL.md` before code edits. Known fallback locations include `~/.claude/plugins/marketplaces/ponytail/skills/ponytail/SKILL.md` and `~/.codex/.tmp/marketplaces/ponytail/skills/ponytail/SKILL.md`.
- If `ponytail` cannot be found or loaded, report that explicitly before editing code; do not invent or infer ponytail rules.
- If `ponytail` conflicts with this skill, stop and report the conflict instead of silently choosing one rule.

## Ponytail Conflict Resolution

`ponytail` rejects unrequested abstractions such as an interface with one implementation. This skill explicitly requests DIP and ONC-style boundaries, so boundary interfaces in `contract` are allowed only when they protect business logic from infrastructure, external capabilities, framework details, or cross-layer dependencies.

Resolution rules:

- Required DIP boundary interfaces are not "unrequested abstraction"; they are part of the project contract.
- Keep `contract` minimal: define ports that business logic consumes, not interfaces for every struct or helper.
- Do not create service interfaces, factories, config objects, or adapter interfaces solely because a folder exists.
- Do not add an interface for private pure business code, local helpers, or same-layer calls unless a real boundary, test seam, or second implementation exists.
- Prefer one small constructor-injection path over frameworks, registries, reflection, or generated wiring.
- If a ponytail simplification would remove a required DIP boundary, keep the boundary and shrink everything around it.

## DIP Rules

All project code must obey DIP:

- High-level business policy depends on abstractions, not concrete storage, SDK, network, queue, cache, or framework implementations.
- `services` owns core business behavior and consumes boundary interfaces from `contract`.
- Concrete adapters implement `contract` interfaces and are wired in `main`; they are not constructed inside business logic.
- Controllers/handlers translate transport input into service calls and transport output; they must not contain core business decisions.
- Domain/business code must not import infrastructure-only packages unless those packages are expressed through `contract` abstractions.
- New dependencies must be injected through constructors or explicit parameters, not pulled from globals or created ad hoc inside business functions.

## ONC Architecture

Use this ONC-style layout for business modules. Names may be adapted to the host language, but responsibilities must stay the same.

```text
project-root/
├── main/ or cmd/<app>/ or main.go
├── internal/<business>/
│   ├── contract/
│   ├── services/
│   ├── controllers/ or handlers/
│   ├── repositories/ or adapters/
│   ├── models/ or entities/
│   └── helpers/
└── pkg/
    └── utils/
        └── <common-utility>/
```

Layer responsibilities:

- `contract`: Business-layer interface definitions. Define minimal service ports, repository ports, external capability ports, DTO-facing contracts, and cross-layer abstractions that business logic depends on. Do not create interfaces for private helpers or same-layer code just to fill this folder.
- `services`: Business core logic. Implement use cases, orchestration, validation, state transitions, permission decisions, and transaction boundaries. Distinguish successful empty results from failures; zero matching rows for list/search/dashboard queries is a normal business state. Depend on `contract`, not concrete adapters.
- `controllers` / `handlers`: Control/transport layer. Parse HTTP/RPC/CLI/message input, call services, map errors and responses, and keep transport concerns out of business logic. Return successful empty response envelopes for no-data list/query results instead of mapping them to transport errors.
- `repositories` / `adapters`: Concrete implementations for persistence, external APIs, SDKs, queues, caches, file systems, and other infrastructure. Implement interfaces declared in `contract`. Return empty collections for successful zero-row queries; reserve errors for storage, network, parsing, validation, or authorization failures.
- `models` / `entities`: Business data structures, entities, value objects, constants, and state definitions used by the business module, including explicit states for empty/no-data, not-found, permission-denied, validation failure, and system failure when those states cross layer boundaries.
- `helpers`: Helper utilities that belong only to this business module. Keep them private to the module and do not use them as cross-business dumping grounds.
- `main` / `cmd/<app>` / `main.go`: Composition root. Create concrete implementations, inject dependencies, register routes/jobs/commands, start the process, and avoid business logic.
- `pkg/utils/<name>`: Project-wide common utilities. Use only for extracted helpers that are genuinely shared by multiple business modules.

## Dependency Direction

Allowed direction:

```text
controllers/handlers -> services -> contract
repositories/adapters -> contract
main -> controllers/handlers, services, repositories/adapters, contract
helpers -> local module code only when business-specific
pkg/utils -> no business-module dependency
```

Forbidden direction:

```text
services -> repositories/adapters concrete implementation
services -> HTTP framework, database client, SDK client, queue client, cache client
contract -> services concrete implementation
contract -> repositories/adapters concrete implementation
pkg/utils -> internal/<business>
helpers from one business module -> helpers of another business module
```

## Main And Injection

Keep object creation and wiring in `main` or the project composition root:

```go
func main() {
    repo := adapters.NewApprovalRequestRepository(db)
    service := services.NewApprovalRequestService(repo)
    handler := controllers.NewApprovalRequestController(service)

    router.RegisterApprovalRequestRoutes(handler)
    server.Start()
}
```

Rules:

- `main` may know concrete implementations because it is the injection point.
- `services.New...` must accept interfaces from `contract` when the dependency is infrastructure or an external capability.
- Business methods must not call `adapters.New...`, `sql.Open`, SDK constructors, HTTP client setup, or queue/cache constructors.
- If dependency construction requires configuration, parse configuration before injection and pass typed values into constructors.

## Helper Extraction

Use this lifecycle for helpers:

1. During feature work, put business-specific helpers under that module's `helpers`.
2. Keep helper names tied to the business need; avoid vague `common`, `misc`, `tools`, or `utils` buckets inside a module.
3. Near project completion or after a feature set stabilizes, review all business `helpers`.
4. If the same helper concept is used by multiple business modules and has no business-specific dependency, extract it to `pkg/utils/<specific-name>`.
5. After extraction, update imports and keep `pkg/utils/<specific-name>` independent from `internal/<business>` packages.

Do not extract prematurely. A helper becomes project-wide only after reuse is real and the API can be named without referencing one business module.

## Review Checklist

- Business logic is in `services`, not controllers, adapters, or `main`.
- Business code depends on `contract` boundary interfaces instead of concrete infrastructure.
- Each `contract` interface is justified by a boundary, external capability, test seam, or multiple implementations.
- Concrete adapters are wired in `main` and implement `contract`.
- `contract` contains interfaces and contracts, not concrete service or adapter logic.
- List/query contracts return success with empty collections for no-data results; single-resource missing cases are represented intentionally as `not found` only when the product flow needs a missing-resource error state.
- Controllers and frontend DTOs can distinguish empty, not-found, permission-denied, validation error, and system error without relying on generic error text.
- `helpers` are business-local unless proven reusable.
- Shared helpers extracted late live under `pkg/utils/<specific-name>` and do not import business modules.
- `ponytail` was read before coding, or its absence was reported before editing.
