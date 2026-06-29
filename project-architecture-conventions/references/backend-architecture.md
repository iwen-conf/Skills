# Backend Architecture Reference

Use this file as the detailed reference for the default backend architecture. Keep `SKILL.md` as the short operating contract and load this file when a task needs file-level naming, interface boundaries, transaction placement, or review checks.

## Directory Layout

```text
backend/
├── cmd/server/
├── configs/
├── internal/
│   ├── bootstrap/
│   ├── constants/
│   ├── domain/
│   │   ├── entities/
│   │   ├── events/
│   │   ├── filters/
│   │   ├── repositories/
│   │   └── services/
│   ├── usecase/<module>/
│   ├── interface/restful/
│   │   ├── controllers/
│   │   ├── dto/requests/
│   │   ├── dto/responses/
│   │   ├── middlewares/
│   │   └── router/routes/
│   ├── infrastructure/
│   │   ├── gateways/
│   │   └── support/
│   └── wire/
└── migrations/{up,down}/
```

## Dependency Direction

```text
cmd -> internal/wire -> internal/interface/restful -> internal/usecase -> internal/domain
internal/infrastructure -> internal/domain
```

Rules:

- `domain` must not import infrastructure, usecase, interface, Gin, pgx, Redis, Casbin, or other driver/framework packages.
- `usecase` must not import Gin, `net/http`, pgx, `database/sql`, or interface-layer packages.
- `interface/restful/controllers` must not import `domain/repositories`, pgx, `database/sql`, or `pgxpool`.
- `wire` is allowed to import concrete infrastructure and assemble the object graph.
- `infrastructure` implements domain repository interfaces or capability contracts; business logic must not construct it directly.

## Layer Responsibilities

- `domain/entities`: Business entities, value objects, and enum-like state types.
- `domain/repositories`: Repository interfaces consumed by usecases. Example: `type Course interface { List(...); GetByID(...); Create(...); Update(...) }`.
- `domain/services`: Pure domain services when needed.
- `usecase/<module>`: Application workflows and transaction orchestration. Controllers depend on the module `Contract`.
- `interface/restful/controllers`: HTTP handlers. They bind/validate request input, authorize, call usecase contracts, map errors, map DTOs, and respond.
- `interface/restful/dto/requests`: Named request DTOs and reusable request fragments.
- `interface/restful/dto/responses`: Named response DTOs and response envelope helpers.
- `infrastructure/gateways/persistence/postgres/models`: Database row/table models and entity conversion.
- `infrastructure/gateways/persistence/postgres/repository`: Postgres implementations of `domain/repositories`.
- `infrastructure/gateways/<capability>`: External capability gateways such as notification, storage, and recommendation.
- `infrastructure/support/<capability>`: Cross-cutting infrastructure such as authorization, cache, logger, and security.
- `constants`: Truly application-wide constants with multiple legitimate consumers. Do not use this as a dumping ground for module-specific business limits, labels, or state.
- `wire`: Repository set, usecase set, controller construction, bootstrap, seeds, reset, and application lifecycle.
- `bootstrap`: Startup initialization that runs after migrations and repository construction.

## Usecase Module Shape

```text
internal/usecase/<module>/
├── contract.go
├── main.go
├── params.go
├── results.go
├── errors.go          # optional
├── service.go         # optional
├── services.go        # optional helper contracts/configs
└── service_<feature>.go
```

Conventions:

- `contract.go` exposes `type Contract interface { ... }` for controller-facing operations.
- `main.go` defines `type Service struct { ... }` and `New(...) Contract`.
- `params.go` contains input/request parameter structs for usecase methods.
- `results.go` contains application result/view structs returned to controllers.
- `service_<feature>.go`, `service.go`, or `services.go` hold focused workflow implementations or helper contracts when the module is too large for `main.go`.
- `internal/usecase/shared` holds shared application types such as pagination and common errors.
- Keep `main.go` small; split workflows into focused service files when needed.

## Interface Design

Use explicit contracts at real boundaries:

- Usecase boundary: `internal/usecase/<module>/Contract`.
- Repository boundary: `internal/domain/repositories.<Entity>`.
- External capability boundary: package-local `Contract` interfaces in `infrastructure/gateways/*` or `infrastructure/support/*`.
- Engine boundary for swappable low-level implementations: `Engine` interfaces in support/gateway packages such as cache or storage.

Avoid interfaces for private helpers or same-package pure functions. Add an interface only for a layer boundary, external capability, test seam, or multiple implementations.

## Constructor And Naming Patterns

Common constructor patterns:

- Usecase: `func New(...domain repositories..., options/config...) Contract`.
- Repository: `func New<Course>Repository(pool *pgxpool.Pool) repositories.Course` or `New<Course>DomainRepository`.
- Support/gateway service: `func NewService(engine Engine, ...) (Contract, error)`.
- Wire: `newRepositorySet`, `newUsecaseSet`, `newControllers`, `NewApplication`.

Common type names:

- `Contract` for the exported service boundary of a usecase or capability package.
- `Service` for the concrete implementation behind a `Contract`.
- `Engine` for low-level pluggable adapters inside infrastructure support/gateway packages.
- `<Entity>Repository` or `<Entity>DomainRepository` for concrete Postgres repository structs.
- `Params` for usecase inputs and `Result` for usecase outputs.

## Transaction Boundary

Put transaction orchestration in usecase logic through `domain/repositories.TxManager`.

- `domain/repositories/transaction.go` defines `Tx`, `TxFunc`, and `TxManager`.
- Postgres implements it in `infrastructure/gateways/persistence/postgres/repository/transaction.go`.
- Repositories execute SQL and can reuse transaction context; they should not start ad hoc transactions for business workflows.
- Controllers never see transactions.

## Controller Rules

Controllers should:

- Use named request and response DTOs.
- Use mapper helpers for DTO conversion.
- Return successful empty list/query results as success responses with empty collections and pagination metadata.
- Keep HTTP status and business error mapping explicit at call sites when the flow is small.
- Avoid `gin.H` for runtime response bodies; named DTO structs are preferred.
- Keep business decisions, transaction logic, SQL, and repository calls out of controllers.

## Infrastructure Rules

Persistence:

- Put table models in `postgres/models`.
- Put SQL repository implementations in `postgres/repository`.
- Return domain entities from repositories, not database models.
- Keep one physical table concept aligned to one repository interface/implementation where possible.
- Do not create repository interfaces for projections that have no physical persistence boundary; query through existing real repositories or usecase logic.

Support/gateway wrappers:

- Use `contract.go`, `engine.go`, `service.go`, and `main.go` when a capability has a service boundary and one or more engines.
- Validate required engines in `NewService(...)` and return constructor errors; do not repeat "engine not configured" checks in every method.

## Logging Rules

Use zap as the default structured logger and keep logging as an explicit observability design choice.

- Initialize zap in `cmd`, `wire`, or `infrastructure/support/logger`; inject it or a narrow project logger contract through constructors.
- Keep `domain` free of logging imports. In `usecase`, log only business-significant state changes, security-sensitive decisions, idempotency conflicts, or workflow failures that need traceability.
- Prefer centralized request logging middleware for HTTP request summaries. Do not duplicate the same successful request log in every controller.
- Repositories and gateways may log slow operations, retries, retry exhaustion, and external dependency failures with sanitized structured fields.
- Use stable fields: `operation`, `request_id`, `tenant_id`, `user_id`, `resource`, `resource_id`, `status`, `duration_ms`, and `error`.
- Do not log normal validation failures, expected empty/not-found results, every successful read query, per-row loop details, health checks, secrets, tokens, raw authorization headers, full payloads, or personal data beyond minimal identifiers.
- Log each failure once at the owning boundary. Wrap and return errors in lower layers instead of producing duplicate caller/callee logs.

## Constant And Enum Rules

Use Go constants as compile-time semantic names. They should clarify business meaning, preserve type safety, and avoid repeated literals without becoming global macro buckets.

Naming and scope:

- Use Go `MixedCaps` / `mixedCaps`. Do not use `SNAKE_CASE`, `ALL_CAPS`, or package prefixes inside the name when the package already gives context.
- Export a constant only when another package should compile against it. Keep private constants lower-case and close to the behavior they configure.
- Prefer constants beside the code that uses them: entity state constants in `domain/entities`, workflow limits in the owning `usecase/<module>`, transport literals in the REST package, and integration-specific constants in the gateway/support package.
- Use `internal/constants` only for stable application-wide constants that are legitimately shared across several layers or modules. Do not create a catch-all `constants.go` for unrelated values.
- Replace repeated magic numbers and strings with a named constant that expresses business meaning, such as `MaxBatchSize`, `DefaultPageSize`, or `TokenExpirySkew`.

Typed and untyped constants:

- Prefer untyped constants for plain scalar literals when the value can safely adapt to context, such as numeric limits, ratios, string labels, and pure compile-time defaults.
- Add an explicit type when the type is part of the API contract, prevents invalid domain values, is required by a dependency signature, or models enum-like state.
- Use standard-library constants whenever they already represent the value: `time.DateTime`, `time.DateOnly`, `time.TimeOnly`, `time.RFC3339`, `http.MethodGet`, `http.StatusOK`, file modes, TLS versions, crypto hashes, and similar semantic constants.

```go
const (
    MaxBatchSize = 50
    defaultLimit = 20
)

func list(limit int32) {
    if limit > MaxBatchSize {
        limit = MaxBatchSize
    }
}
```

Enum-like domain states:

- Define a custom type for business states. Do not accept naked `int` or `string` parameters when the value represents a closed domain set.
- Reserve the zero value for `Unknown`, `Unspecified`, or a safe default that matches the product behavior.
- Use typed constants with `iota` when the underlying value is internal. Use explicit string values when they are persisted or part of a public wire contract.
- Implement `String()` for states used in logs, errors, metrics, CLI output, or operator-facing diagnostics.
- Add parse and validation helpers at boundaries where values enter from HTTP, queues, storage, or external APIs.

```go
type OrderStatus int

const (
    OrderStatusUnknown OrderStatus = iota
    OrderStatusCreated
    OrderStatusPaid
    OrderStatusShipped
    OrderStatusCompleted
)

func (s OrderStatus) String() string {
    switch s {
    case OrderStatusCreated:
        return "Created"
    case OrderStatusPaid:
        return "Paid"
    case OrderStatusShipped:
        return "Shipped"
    case OrderStatusCompleted:
        return "Completed"
    default:
        return "Unknown"
    }
}
```

Cross-service constants:

- Treat cross-service error codes, event types, and state values as API contracts, not local implementation details.
- Prefer Protobuf/OpenAPI definitions with generated code when the values cross service boundaries.
- In a monorepo, a governed shared module is acceptable only when it has version ownership and compatibility rules. Services must not copy-paste constants into private packages.
- Keep transport or wire-code enums separate from internal domain enums when their lifecycle differs. Map between them at the interface/gateway boundary.

## File Budgets And Naming

- `controllers/*` <= 600 lines.
- `usecase/*/main.go` <= 200 lines.
- `usecase/*/service_*.go` <= 400 lines.
- Use plural package names where applicable: `entities`, `repositories`, `services`, `events`, `filters`, `controllers`, `middlewares`, `models`, `requests`, `responses`.
