# Go Backend & Type-Safety Conventions

Applied by `arc-code` and `arc-audit` on Go repositories.

1. **Layer Direction**: `transport/handler -> usecase/service -> domain <- adapter/repo`. Domain owns interfaces at consumer boundaries; never leak SQL rows, HTTP contexts, or third-party SDK structs across domain boundaries.
2. **Type Safety**: Ban `any` / `map[string]any` in domain and usecase signatures. Wrap IDs and statuses in distinct typed values; validate invariants at construction/unmarshal boundaries.
3. **Concurrency & Context**: Propagate `context.Context` as first parameter; bound every goroutine lifetime with cancellation and error propagation (`errgroup`); never fire-and-forget background writes without shutdown drain.
4. **Transactions & SQL**: Bind multi-row invariants inside explicit DB transactions; guard state updates with `WHERE status = expected` CAS checks and inspect `RowsAffected()`.
