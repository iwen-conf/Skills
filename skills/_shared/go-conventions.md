# Go Explicit Architecture, Composition & Engineering Standards (`iwen-example` Baseline)

Applied by `arc-plan`, `arc-code`, and `arc-audit` on all Go (`*.go`, `go.mod`) repositories. Anchored on **`iwen-example` Explicit Architecture**, **Go Composition Over Inheritance (`组合大于继承`)**, **Zero Dynamic Typing (`严禁 any / interface{}`)**, and mature Go standards (Effective Go, Google Go Style Decisions, Uber Go Style Guide).

## 1. Explicit Architecture & Project Structure (`iwen-example` Baseline)

### 1.1 One-Way Dependency Direction & Import Matrix

```text
cmd/server -> internal/wire -> internal/interface/restful -> internal/usecase -> internal/domain
                                                          \-> internal/infrastructure -> internal/domain
                                                          \-> pkg/<topic> & github.com/iwen-conf/utils-pkg
```

| Layer / Package | Strictly Must NOT Import |
|---|---|
| `internal/domain` | `infrastructure`, `usecase`, `interface`, REST DTOs, Gin, `pgx`/`database/sql`, Redis, `zap`, any SDK/driver |
| `internal/usecase` | Gin, `net/http`, `pgx`/`database/sql`, `interface/restful/*`, persistence `models` |
| `interface/restful/controllers` | `domain/repositories`, `pgx`/`database/sql`, `pgxpool`, persistence `models` |
| `dto/requests`, `dto/responses` | `internal/domain`, `internal/usecase`, `repositories`, persistence `models`, Gin, drivers |

Only `internal/wire` imports concrete infrastructure and assembles the dependency graph; business packages never construct concrete dependencies or read global singletons.

### 1.2 Standard Directory Layout & Plural Package Naming

```text
├── cmd/server/main.go                          # Thin bootstrap, signal handling, graceful drain only
├── configs/                                    # Typed config schemas
├── migrations/{up,down}/                       # Reversible SQL migrations
├── internal/
│   ├── bootstrap/                              # Post-migration & post-repo startup tasks
│   ├── constants/                              # Cross-layer stable constants ONLY (never a catch-all dump)
│   ├── domain/                                 # Plural package names ONLY; no gateways/ports/adapters/clients
│   │   ├── entities/                           # Identity, lifecycle transitions, entity invariants (no JSON/ORM tags)
│   │   ├── events/                             # Domain events
│   │   ├── filters/                            # Domain query filter value types
│   │   ├── repositories/                       # Persistence contracts in domain language + TxManager
│   │   └── services/                           # Pure cross-entity domain rules (zero I/O or logging)
│   ├── usecase/
│   │   ├── shared/                             # Shared pagination params & common application errors
│   │   └── <module>/
│   │       ├── contract.go                     # type Contract interface { ... } (controller-facing boundary)
│   │       ├── main.go                         # type Service struct + func New(...) Contract (<= 200 lines)
│   │       ├── params.go                       # Named business input structs (*Param)
│   │       ├── results.go                      # Named business output structs (*Result; never return raw entities)
│   │       ├── dependencies.go                 # Narrow consumer-owned external capability contracts
│   │       ├── service_<feature>.go            # Cohesive workflows, authz & TxManager orchestration (<= 400 lines)
│   │       └── <feature>_helpers.go            # Cohesive private helpers (validation/mapping)
│   ├── interface/restful/
│   │   ├── router/routes/                      # Route registration & middleware mounting
│   │   ├── middlewares/                        # Auth, trace, recovery, request summary middlewares
│   │   ├── dto/requests/                       # Named transport request DTOs + binding tags
│   │   ├── dto/responses/                      # base.go + meta.go + per-resource response DTOs embedding Base
│   │   └── controllers/                        # Bind DTO -> build Param -> call Contract -> struct-literal Resp (<= 600 lines)
│   ├── infrastructure/
│   │   ├── gateways/
│   │   │   ├── persistence/postgres/{models,repository}/ # DB row structs + Entity <-> Model conversion + TxManager impl
│   │   │   └── <capability>/                   # External protocols (payment, storage, notification) behind contracts
│   │   └── support/<capability>/               # Authz, cache, fulltext, logger, security: contract.go + engine.go + service.go + main.go
│   └── wire/                                   # providers.go, repositories.go, gateways.go, usecases.go, controllers.go, application.go, lifecycle.go
└── pkg/<topic>/                                # Pure zero-business-dep shared utilities + <topic>_test.go
```

- **Vocabulary & Constructor Rules**: Use `Contract`, `Params`, `Results`, `Service`, `Repository`, `Model`, and `Engine`. **Strictly ban** `Port` / `Adapter` in folder or type names. `New(...)` returns `Contract` directly (**never** write `var _ Contract = (*Service)(nil)`). Suffix injected repository fields with `Repo` (`noteRepo repositories.Note`). In `support/<capability>`, `NewService(engine Engine, ...)` validates required engines once at construction so methods never re-check nil engines.

## 2. Go Composition Over Inheritance & Strong Typing (`组合大于继承与全量强类型`)

1. **Explicit Field Delegation Over OOP Pseudo-Inheritance (`严禁 BaseXxx 匿名继承`)**:
   - Compose `Service`, `Controller`, and `Repository` capabilities via **named private struct fields** injected through constructors (`noteRepo repositories.Note`, `txManager repositories.TxManager`).
   - **Strictly forbid** `BaseService`, `BaseController`, `BaseRepository`, or `BaseEntity` anonymous struct embedding to mimic OOP class inheritance or hook overrides.
   - **Never** anonymously embed `*sql.DB`, `*pgxpool.Pool`, `sync.Mutex`, or third-party SDK clients inside exported structs (prevents leaking driver/lock methods onto public APIs).
2. **Response DTO Flat Value Embedding (`dto/responses` 结构体字面量组合)**:
   - `dto/responses/base.go` defines only `type Base struct { Success bool; Message string }` and `var SuccessBase = Base{Success: true, Message: "ok"}`; `meta.go` holds `Page` / cursor metadata; each resource file (`note.go`) defines concrete endpoint structs embedding `Base` (`type CreateNoteResp struct { Base; Data NoteDTO }`, `type ListNotesResp struct { Base; Data []NoteDTO; Page Page }`).
   - Construct responses at the HTTP boundary with **direct struct literals** (`c.JSON(http.StatusOK, responses.CreateNoteResp{Base: responses.SuccessBase, Data: ...})`). **Strictly forbid** `gin.H`, dynamic map envelopes, untyped `Data` fields, generic response wrappers, ceremonial `responses.NewXxx(...)` wrapper factories, and `entity.ToDTO()` methods on domain entities.
3. **Narrow Consumer Interface Composition (`小接口按消费方组合`)**:
   - Define named interfaces **only** at true architectural boundaries (`usecase` `Contract`, `domain/repositories`, consumer `dependencies.go`, `support` `Engine`).
   - Compose larger contracts from small focused interfaces (`io.Reader`-style); list only the exact methods the workflow calls — never mirror a bloated third-party SDK client or invent interfaces for same-package helpers.
4. **Zero `any` & `interface{}` Strong-Typing Law (`严禁 any / interface{} 弱类型与不明确行为`)**:
   - **Strictly prohibit** `any`, `interface{}`, and inline anonymous `interface{...}` anywhere in parameters, return values, struct fields, maps/slices, variadic helpers, generic constraints, or SQL parameter slices (`gate.sh` fails `suppressions` on every occurrence).
   - Replace dynamic bags with deterministic compile-time types:
     - **Fixed payloads & CLI/HTTP responses**: Define concrete named `struct` types.
     - **Polymorphic envelopes**: Pair a concrete header `struct` with `json.RawMessage` and decode into concrete target structs by version/kind.
     - **Heterogeneous DSL / assertion / telemetry values**: Define an explicit tagged-union `struct` (`Kind` closed enum + `BoolVal`, `IntVal`, `FloatVal`, `StrVal`, `ArrayVal`, `ObjectVal` with deterministic `MarshalJSON` / `UnmarshalJSON`).
     - **Conditional SQL filters**: Dispatch via explicit `switch` branches passing concrete typed parameters to `QueryContext` (or static positional SQL predicates), never untyped slice builders.

## 3. File Budgets, Reuse & Anti-Wrapper Rules (`禁止重复造轮子与无脑包装`)

### 3.1 File & Helper Budgets

- **Hard Ceiling**: Every authored `.go` file (production and `*_test.go`) **MUST NOT exceed `1200` lines** (`gate.sh check` fails `go-file-lines` at `1201+` lines).
- **Layer Budgets (`iwen-example`)**: `controllers/* <= 600` lines; `usecase/*/main.go <= 200` lines; `usecase/*/service_*.go <= 400` lines.
- **Private Helper Cap**: Keep **at most 2 private `func` declarations** inside a primary workflow file; move cohesive helpers into `<feature>_helpers.go` (or `<domain>_validation_helpers.go` / `<domain>_mapping_helpers.go`, never numbered `helpers_1.go`).

### 3.2 Mandatory Reuse Ladder

1. **Go Standard Library & Constants First**: Use `slices`, `maps`, `cmp`, `strings`, `strconv`, `filepath`, `time`, `crypto/*`, `errors`, `errgroup`, and stdlib constants (`time.DateTime`, `time.DateOnly`, `time.RFC3339`, `http.StatusOK`, `http.MethodPost`) — never hand-roll `containsString` or `"2006-01-02 15:04:05"`.
2. **Existing Shared Modules (`utils-pkg` & `arc idx`)**: Check `github.com/iwen-conf/utils-pkg` (e.g., `utils-pkg/pagination` `OffsetRequest`, `CursorRequest`, `NewHMACCodec`) and run `arc idx search` / `arc idx symbol` before writing any helper.
3. **Topic-Named Shared Non-Business Packages**: Extract reusable domain-agnostic helpers into single-word noun packages (`pkg/paging`, `pkg/strx`, `pkg/timex`, `pkg/cryptox` or `internal/pkg/<topic>`) with zero business imports and table-driven `*_test.go`. **Strictly ban** `util`, `utils`, `common`, `misc`, or `helper` dumping grounds.

### 3.3 Ban Ceremonial Getters, One-Line Wrappers & Single-Line `func` Formatting (`禁止无脑拆单行 Getter 与单行函数体`)

- Read exported fields (`saved.ID`) and write simple 1-line checks (`order.Status == StatusPaid`, `len(items) == 0`) **directly inside the business function**.
- Before extracting any function/method, verify at least one holds: (1) multi-step logic reused by `>= 2` callers, (2) a genuine domain invariant on `entities`/`valueobjects`, or (3) required by an explicit `Contract` interface signature. Otherwise keep it inline.
- **Never** format any function or method on a single line (`func ... { return ... }`); always expand braces across multiple lines.

## 4. Mandatory Comment Conventions (`结构体字段注释 + 函数外部契约注释 + 函数内部逐行编号注释`)

### 4.1 Struct & Field-Level Comments (`struct` 字段级注释唯一事实来源)

Document every field **inside the `struct` definition itself** — never inline or repeat struct fields inside function header comments:

```go
// CreateNoteRequest 创建笔记请求DTO
type CreateNoteRequest struct {
	Title string `json:"title" binding:"required"` // 笔记标题
	Book  Book   `json:"book" binding:"required"`  // 关联书籍信息
}

// Book 书籍信息
type Book struct {
	ISBN   string `json:"isbn"`   // 国际标准书号
	Author string `json:"author"` // 作者姓名
}

// CreateNoteResp 创建笔记响应DTO
type CreateNoteResp struct {
	Base
	Data NoteDTO `json:"data"` // 笔记详情数据
}
```

### 4.2 External Function Header & Tier Matrix (`gate.sh` `go-comments` Enforced)

| Function Category | External Header Requirement | Internal `// 1. xxx` Requirement |
|---|---|---|
| `controllers` 层控制器方法 (`c *gin.Context`) | **必须写三段式**（按 **DTO 级别**写 Request / Response，字段仍在 DTO `struct` 内注释） | **必须写** |
| 普通业务 / 仓储 / 共享工具函数与方法 | **必须写三段式**（`// 函数名 函数功能` + `// 请求体` + `// 响应体`，无项写 `// 无`） | **必须写** |
| 非 `controllers` 层的 `c *gin.Context` / `gin.HandlerFunc` 中间件与路由入口 | **必须写单行函数功能注释**：`// 函数名 函数功能`（免去 `请求体`/`响应体`） | 多步逻辑按需编号 |
| 测试函数（`*_test.go` 中的 `TestXxx` / `BenchmarkXxx` / `FuzzXxx`） | **必须写单行函数名注释**：`// TestXxx 验证xxx场景`（免去 `请求体`/`响应体`） | 多阶段复杂用例按需分段 |
| 进程启动引导 `main()` / `init()` | **免去模板头** | 按需步骤注释 |

- **Concise Summary + Zero Struct Field Duplication**: First line `// 函数名 函数功能` states only the concise business capability (`// CreateNote 创建笔记`). `// 请求体` and `// 响应体` use identical `// <名称> <整体含义>` lines without repeating `struct` fields.
- **Internal Step Comments (`// 1. xxx`)**: Pair each atomic statement/call with `// N. <动作与业务意图>`, keep `if err != nil` attached to that step, limit nesting to `<= 3` via guard clauses, and renumber `1..N` whenever steps change.
- **Incremental Discipline (`只补缺失，不动已有`)**: Apply in full to new symbols and touched symbols missing comments; never rewrite untouched legacy comments just for formatting churn.

### 4.3 Canonical End-to-End Example

```go
// CreateNote 创建笔记
// 请求体
// req CreateNoteRequest 创建笔记请求DTO
// 响应体
// resp CreateNoteResp 创建笔记响应DTO
func (ctrl *NoteController) CreateNote(c *gin.Context) {
	// 1. 绑定并校验HTTP JSON请求DTO
	var req requests.CreateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ctrl.writeBindError(c, err)
		return
	}
	// 2. 调用笔记用例契约执行创建逻辑
	res, err := ctrl.noteUC.CreateNote(c.Request.Context(), req.Title, Book{ISBN: req.Book.ISBN, Author: req.Book.Author})
	if err != nil {
		ctrl.writeDomainError(c, err)
		return
	}
	// 3. 通过嵌入 Base 的结构体字面量直接返回响应DTO
	c.JSON(http.StatusOK, responses.CreateNoteResp{
		Base: responses.SuccessBase,
		Data: responses.NoteDTO{ID: res.ID, CreatedAt: res.CreatedAt.Format(time.RFC3339)},
	})
}

// TraceMiddleware 注入链路追踪ID并记录请求耗时
func TraceMiddleware(c *gin.Context) {
	c.Next()
}

// CreateNote 创建笔记
// 请求体
// ctx 请求上下文
// title 标题
// book 书籍
// 响应体
// result 创建笔记结果
// err 错误信息
func (s *Service) CreateNote(ctx context.Context, title string, book Book) (result NoteResult, err error) {
	// 1. 清洗标题首尾空白并校验必填约束
	cleanTitle := strx.TrimCollapseSpace(title)
	if cleanTitle == "" {
		return NoteResult{}, errs.ErrInvalidNoteTitle
	}
	// 2. 校验书籍标准书号格式合法性
	if !s.isbnValidator.Valid(book.ISBN) {
		return NoteResult{}, errs.ErrInvalidBookISBN
	}
	// 3. 构造纯领域笔记实体并注入 UTC 创建时间
	note := entities.NewNote(cleanTitle, book.ISBN, s.clock.NowUTC())
	// 4. 持久化领域实体到仓储层
	saved, err := s.noteRepo.Create(ctx, note)
	if err != nil {
		return NoteResult{}, fmt.Errorf("create note: %w", err)
	}
	// 5. 直接读取实体字段组装标准响应结果返回
	return NoteResult{ID: saved.ID, CreatedAt: saved.CreatedAt}, nil
}

// TestCreateNoteRejectsBlankTitle 验证空白标题时拒绝创建笔记
func TestCreateNoteRejectsBlankTitle(t *testing.T) {
	// ...
}
```

## 5. Idiomatic Go Engineering Standards (`开发标准规范`)

1. **Constants & Closed-Set Enums**: Name constants in `MixedCaps` / `mixedCaps` (ban `ALL_CAPS` / `SNAKE_CASE`). Give every closed domain state set a custom type whose zero value is `Unknown` / `Unspecified` (`iota` internally; explicit strings for DB/wire), implement `String()`, validate at entry boundaries, and cover all cases in `switch` with explicit `default`.
2. **Error Wrapping & Log-Once**: Wrap lower-layer errors with `fmt.Errorf("<op>: %w", err)`. Compare errors **exclusively** via `errors.Is` / `errors.As` (**strictly ban `err == target`**; use plain structs with `Code int`, `Message string`, `Err error` without `...any` variadic constructors). Log each failure **once** at the owning boundary — never log-and-return across multiple layers.
3. **Concurrency & Context**: Pass `ctx context.Context` as first parameter (never store `Context` inside a struct); bound every goroutine lifetime with cancellation and error propagation (`errgroup`) plus graceful shutdown drain. Never call `defer` inside a `for` loop body; never `panic` in business logic or constructors.
4. **Transactions & CAS SQL**: Orchestrate multi-row invariants in `usecase` via `TxManager` (repositories reuse tx from `ctx` and never open ad-hoc workflow transactions; controllers never see transactions). Guard state transitions with `WHERE status = expected` CAS checks and inspect `RowsAffected()`.
5. **Structured Logging (`zap`)**: Initialize `zap` once in `wire` / `support/logger` and inject via constructors. Keep `domain` free of logger imports; emit HTTP request summaries in middleware (controllers never duplicate routine `200` logs); never log validation failures, `ErrNotFound`, per-row loops, tokens, or raw secrets (`operation`, `request_id`, `tenant_id`, `user_id`, `resource`, `resource_id`, `status`, `duration_ms`, `error`).
