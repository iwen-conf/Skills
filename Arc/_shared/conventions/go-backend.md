# Go Backend Conventions

Applied by arc:code for Go backend changes; checked by arc:audit; arc:explore maps layers with it.

Also read [go-type-safety.md](go-type-safety.md) (typing, response DTOs) and
[coding-discipline.md](../coding-discipline.md) §4 (paper-overs). §2, §5, §6, §8, §10, §12 and §14
(layout, models and mappers, usecase module, naming and constructors, controllers, debug evidence,
file budgets) live in [go-backend-structure.md](go-backend-structure.md): read it when creating a
module, wiring the composition root, adding mappers or naming constructors.

## 1. When the default applies

1. IF the module is new THEN use the layout in [go-backend-structure.md](go-backend-structure.md) §2. IF the repository already has an established local
   convention THEN follow it unless the task explicitly asks to migrate.
2. IF migrating THEN change the smallest slice that preserves behavior.
3. Before proposing an architecture change, read the host's package layout, constructors, contracts,
   DTOs, tests and dependency direction.
4. Add no layer, factory, interface or helper beyond those named here; the §7 boundary interfaces are
   required, not unrequested abstraction.
5. Never write `var _ Interface = (*T)(nil)`; the constructor returns the interface.
6. Never keep a compatibility shim "for history" when the user forbids compatibility and the product is
   not shipped.

## 3. Dependency direction

```text
cmd -> internal/wire -> internal/interface/restful -> internal/usecase -> internal/domain
internal/infrastructure -> internal/domain
```

| Package | Must not import |
|---|---|
| `domain` | infrastructure, usecase, interface, REST DTOs, Gin, pgx, Redis, Casbin, any driver or framework |
| `usecase` | Gin, `net/http`, pgx, `database/sql`, interface-layer packages |
| `interface/restful/controllers` | `domain/repositories`, pgx, `database/sql`, `pgxpool` |
| `dto/requests`, `dto/responses` | `internal/domain`, `internal/usecase`, repositories, database models, Gin, drivers |

Only `wire` imports concrete infrastructure and assembles the graph; business code never constructs it.

## 4. Layer responsibilities

| Layer | Owns | Never |
|---|---|---|
| `domain/entities` | identity, lifecycle state, invariants, entity-local rules (`Cancel`, `Pay`, `Freeze`, `ChangeEmail`, status transitions) | DTO conversion (`ToDTO`, `ToResponse`), JSON/ORM tags, `TableName`, ORM hooks, repository calls, logging; serving as row struct or response DTO |
| `domain/repositories` | persistence ports in domain language (`FindByID`, `Save`, `ListByOwner`) over entities or value objects; `TxManager` | tables, collections, HTTP, driver types, repository models; payment, notification, storage, search, moderation or SDK contracts |
| `domain/services` | pure rules spanning several entities or value objects | persistence, transport, logging, orchestration |
| `usecase/<module>` | workflows, application-specific authorization, transactions, calls to repository ports and its own capability contracts | storage adapters, HTTP, REST DTOs |
| `controllers` | bind/validate input, transport auth, call `Contract`, map errors and results to DTOs, transport mappers | business decisions, transactions, SQL, repository calls |
| `dto/*` | named transport schema; harmless response constants | behavior, invariants, persistence tags, mapper constructors or factories |
| `postgres/models` | row shapes: DB tags, nullable and denormalized columns | business behavior |
| `postgres/repository` | `domain/repositories` implementations; model ↔ entity conversion | returning models |
| `gateways/<capability>` | external protocols (notification, storage, payment, recommendation) behind contracts | business rules |
| `support/<capability>` | authorization, cache, logger, security | business rules |
| `constants` | stable application-wide values with several consumers | module-specific limits, labels, states |
| `wire` | repository/usecase sets, controller construction, bootstrap, seeds, reset, lifecycle | business logic |
| `bootstrap` | startup work after migrations and repository construction | — |

Persistence paths sit under `infrastructure/gateways/persistence/postgres/`.

## 7. Interfaces and DIP

1. Business policy depends on abstractions, never on concrete storage, SDK, network, queue, cache or
   framework implementations. Inject dependencies through constructors or explicit parameters, never
   through globals.
2. Interfaces exist only at these boundaries: usecase `Contract`; repository
   `domain/repositories.<Entity>`; external capability used by a usecase — a narrow interface owned by the
   consuming usecase package (`usecase/shared` only after real reuse), adapter in `infrastructure/gateways/*`
   or `support/*`, wired in `internal/wire`; swappable low-level implementation — `Engine` in a support or
   gateway package.
3. IF a candidate interface covers private helpers, same-package pure functions or same-layer calls THEN do
   not create it, unless a layer boundary, external capability, test seam or second implementation exists.
   Never create an interface, factory or adapter only because a folder exists.
4. A capability interface lists only the methods the workflow calls; never mirror a third-party SDK client.
5. Entities and pure domain services never depend on capability contracts.
6. A capability with engines uses `contract.go`, `engine.go`, `service.go`, `main.go`. `NewService(...)`
   validates required engines and returns an error; methods do not re-check "engine not configured".

## 9. Transactions and persistence

1. Usecases orchestrate transactions through `TxManager` (`domain/repositories/transaction.go` defines
   `Tx`, `TxFunc`, `TxManager`; Postgres implements it in `postgres/repository/transaction.go`).
2. Repositories execute SQL and reuse the transaction context; they never start ad hoc transactions for
   business workflows. Controllers never see transactions.
3. One physical table concept maps to one repository interface and implementation.
4. No repository interface for a projection without a physical persistence boundary; query through the
   existing repositories or usecase logic.

## 11. Logging (zap)

1. Initialize zap once in `cmd`, `wire` or `infrastructure/support/logger`; inject it, or a narrow project
   logger contract, through constructors.
2. `domain` has no logging imports. `usecase` logs only business-significant state changes,
   security-sensitive decisions, idempotency conflicts and workflow failures that need traceability.
3. HTTP request summaries come from centralized middleware; controllers do not repeat success logs.
4. Repositories and gateways may log slow operations, retries, retry exhaustion and external dependency
   failures with sanitized structured fields.
5. Field names: `operation`, `request_id`, `tenant_id`, `user_id`, `resource`, `resource_id`, `status`,
   `duration_ms`, `error`.
6. Never log normal validation failures, expected empty or not-found results, every successful read,
   per-row loop details, health checks, secrets, tokens, raw authorization headers, full payloads, or
   personal data beyond minimal identifiers.
7. Log each failure once, at the owning boundary; lower layers wrap and return the error.
8. Compare errors only with `errors.Is` / `errors.As`; never `err == X` or `err != X`.

## 13. Constants and enums

1. Names use `MixedCaps` / `mixedCaps`; never `SNAKE_CASE`, `ALL_CAPS`, or a prefix the package name
   already gives.
2. Export a constant only when another package compiles against it; keep private constants next to the
   behavior they configure.
3. Placement: entity states in `domain/entities`, workflow limits in the owning `usecase/<module>`,
   transport literals in the REST package, integration constants in the gateway or support package.
   `internal/constants` only for stable values shared across several layers or modules; never a catch-all
   `constants.go`.
4. Replace repeated magic numbers and strings with a constant named for its business meaning
   (`MaxBatchSize`, `DefaultPageSize`, `TokenExpirySkew`).
5. Standard-library constants, typed vs untyped constants, enum-like states and cross-service values:
   [go-type-safety.md](go-type-safety.md) §5.
