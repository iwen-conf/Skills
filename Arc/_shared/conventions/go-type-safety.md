# Go Strict Type Safety

Applied by arc:code when it writes or changes Go code; checked by arc:audit. Layering and naming:
[go-backend.md](go-backend.md).

## 1. Contracts and diagnostics

1. Never add diagnostic suppression (`//nolint`, `# type: ignore`, `eslint-disable`, `@ts-ignore`). IF the
   user explicitly requires an exception THEN bound it to the named line and rule.
2. Use narrow contracts: explicit schemas, constrained value sets, concrete interfaces. Never a catch-all
   shape (`any`, `interface{}`, `map[string]any`) where the fields are known.
3. Branching and error handling cover every supported case: a `switch` over an enum-like type lists each
   constant and handles `default` explicitly.
4. Never hide a validation or contract mismatch with ad hoc coercion; fix the interface that disagrees.

## 2. Response DTO composition

Every Go REST response body is a named struct that embeds the shared base envelope.

```go
// dto/responses/base.go — only the envelope and its constants/helpers
package responses

type Base struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

var SuccessBase = Base{
    Success: true,
    Message: "ok",
}
```

```go
// dto/responses/meta.go — shared metadata (Meta, Pagination, Page, cursor structs)
type Page struct {
    Page     int   `json:"page"`
    PageSize int   `json:"page_size"`
    Total    int64 `json:"total"`
}
```

```go
// dto/responses/user.go — one resource's DTOs and endpoint responses
import "github.com/iwen-conf/utils-pkg/pagination"

type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type UserResp struct {
    Base
    Data User `json:"data"`
}

type UserListResp struct {
    Base
    Data []User `json:"data"`
}

type UserPageResp struct {
    Base
    Data []User `json:"data"`
    Page Page   `json:"page"`
}

type UserCursorResp struct {
    Base
    Data   []User                    `json:"data"`
    Cursor pagination.CursorResponse `json:"cursor"`
}
```

At the HTTP boundary:

```go
c.JSON(http.StatusOK, responses.UserListResp{
    Base: responses.SuccessBase,
    Data: users,
})
```

## 3. Response rules

1. IF the repository already uses singular `dto/response` or another base name (`Base`, `BaseResponse`,
   `ResponseBase`) THEN keep the established name.
2. Each endpoint has its own named response struct (`GetCourseResponse`, `ListCoursesResponse`,
   `DeleteCourseResponse`) that embeds `Base`; never repeat `Success` and `Message` fields.
3. `Data` is a concrete DTO or a slice of one (`Data User`, `Data []User`). Never `Data any`,
   `Data interface{}`, `Data map[string]any` or an inline `struct{...}`.
4. Never use `gin.H`, `map[string]any`, anonymous structs or generic envelopes such as `Response[T any]`
   for runtime response bodies.
5. File split: `base.go` holds only the envelope; `meta.go` holds shared metadata; each feature file
   (`activity_category.go`) holds only that resource's DTOs and endpoint responses. Never put `BaseResponse`,
   `Meta`, `Pagination` and feature DTOs in one feature file.
6. Page and cursor metadata are separate top-level fields, never inside `Data`.
7. Never add constructors, factories or mappers (`responses.NewUser(result)`, `responses.NewUserList(...)`)
   only to satisfy these rules; use struct literals. IF the result-to-DTO conversion is nontrivial THEN
   place it at the HTTP boundary (controllers package or a focused mapper file there).
8. Entities never import `dto/responses` or expose conversion methods such as
   `func (a ActivityCategory) ToDTO() responses.ActivityCategoryDTO`.

## 4. Pagination contracts

| Contract | Request | Response | Use when |
|---|---|---|---|
| Page number | `page`, `page_size` | `page`, `page_size`, `total` | the UI needs a total count or jump-to-page |
| Offset | `offset`, `limit` via `pagination.OffsetRequest` | `offset`, `limit`, `total`, `has_more` via `pagination.OffsetResponse` | the API exposes offset/limit |
| Cursor | `cursor`, `limit` via `pagination.CursorRequest` | `next_cursor`, `prev_cursor`, `has_more` via `pagination.CursorResponse`; sign opaque cursors with `pagination.NewHMACCodec` | the API exposes cursor/limit; never add `page` or `total` |

`pagination` is `github.com/iwen-conf/utils-pkg/pagination`.

## 5. Constants, enum-like states and cross-service values

Naming, export and placement of constants: [go-backend.md](go-backend.md) §13.

1. Use the standard-library constant whenever one exists: `time.DateTime` (not `"2006-01-02 15:04:05"`),
   `time.DateOnly`, `time.TimeOnly`, `time.RFC3339`, `http.MethodGet`, `http.StatusOK`, file modes, TLS
   versions, crypto hashes. IF none exists THEN define a local named constant; never repeat the literal.
2. Use untyped constants for plain scalars that adapt to context (limits, ratios, labels, compile-time
   defaults). Add an explicit type when the type is part of the API contract, prevents invalid domain
   values, is required by a dependency signature, or models enum-like state.
3. A closed domain set gets a custom type; never a naked `int` or `string` parameter for it.
4. The zero value is `Unknown`, `Unspecified`, or a safe default that matches product behavior.
5. Use `iota` when the value is internal; explicit string values when it is persisted or on a public wire
   contract.
6. Implement `String()` when the state appears in logs, errors, metrics, CLI output or operator diagnostics.
7. Add parse and validate helpers where values enter from HTTP, queues, storage or external APIs.
8. Cross-service error codes, event types and state values are API contracts: prefer Protobuf/OpenAPI with
   generated code; a monorepo shared module only with version ownership and compatibility rules; never copy
   them into private packages.
9. IF wire enums and domain enums have different lifecycles THEN keep them separate and map at the
   interface or gateway boundary.
