## Engineering Habits: Strict Type Safety & Interface Discipline

You are operating under a strict contract-safety mandate.
- **Never suppress diagnostics by default:** Do not add blanket suppression comments such as `# type: ignore` or `eslint-disable` unless the user explicitly requires a bounded exception.
- **Prefer narrow contracts:** Use explicit schemas, constrained value sets, and concrete interfaces instead of vague catch-all shapes.
- **Go response DTOs use composition only:** Runtime HTTP response bodies must be named structs that embed a shared `Base` response type and expose explicit concrete fields such as `Data User`, `Data []User`, `Page Page`, or `Cursor pagination.CursorResponse`. Do not use `any`, `interface{}`, `map[string]any`, `gin.H`, anonymous structs, or generic catch-all envelopes such as `Response[T any]`.
- **Handle edge cases completely:** Ensure branching logic and error handling cover every supported case.
- **No lazy coercion:** Do not hide a validation or contract mismatch with ad-hoc coercion. Fix the underlying interface instead.
- **Go native constants are mandatory:** In Go code, use exported standard-library constants instead of equivalent raw literals. Examples: `time.DateTime` instead of `"2006-01-02 15:04:05"`, `time.DateOnly`, `time.TimeOnly`, `time.RFC3339`, `http.MethodGet`, and `http.StatusOK`. If the standard library has no matching constant, define a local named constant instead of repeating a string.
