# HTTP, JSON & Upload Contract Rules

Applied when designing, implementing, or auditing APIs.

1. **Explicit Wire Schema**: Every endpoint defines method, path, auth scope, request DTO, response DTO, pagination cursor/bounds, and deterministic error codes before implementation.
2. **JSON Field Discipline**: Explicit `snake_case` or project-standard casing (never mixed); distinguish omitted vs `null`; bound array lengths and string sizes; format timestamps in RFC 3339 UTC and money in integer minor units or exact decimal strings (never float).
3. **Idempotency & Concurrency**: State-mutating retries (`POST`/`PATCH` on payments, exports, jobs) require `Idempotency-Key` or optimistic version CAS.
4. **Uploads**: Enforce max byte size, server-side MIME sniff (never trust client `Content-Type` or filename path), randomized storage keys, and pre-signed/quarantined scan flow.
