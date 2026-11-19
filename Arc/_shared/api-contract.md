# Unified API, HTTP, JSON & Upload Contract

Single source of truth for adding, changing, documenting or auditing HTTP/RPC APIs. Server, client, tests and reviews align on one frozen contract before concurrent coding starts.

## 1. Ownership & Contract-First Freeze

1. Store contracts in the project's API docs root (default `docs/02-API记录/`) or frozen schema files (`api/openapi.yaml`, `*.proto`, `db/migrations/*.sql`).
2. Locate existing routes, DTOs, callers and schemas via `arc idx skeleton` and `arc idx impact` before authoring. Never invent roles, error codes, enum values, pagination or numeric limits; mark unknowns `待确认` (blocks dependent implementation).
3. In concurrent DAG work (`plan-dag.yaml`), freeze contract & migration files in `frozen_contracts` before fanning out workers. Any worker touching a frozen path fails `gate.sh scope`.

## 2. HTTP Semantics & Boundaries

- **Methods & status**: `GET` read-only/idempotent; `POST` create/action; `PUT`/`PATCH` update; `DELETE` idempotent delete. Separate `400/422` validation, `401` unauthenticated, `403` forbidden, `404` not found, `409` business/state conflict, `429` rate limit, `5xx` server fault.
- **Empty collections**: return success with `[]` (not `null` or `404`); include pagination metadata only when paginated.
- **Auth & ownership**: validate identity, role/permission and resource ownership server-side on every entry; never trust client-supplied prices, roles, states or SQL/sort fragments.
- **Writes & idempotency**: guard concurrent state transitions with transactions, optimistic version locks or state-encoded `WHERE`; record explicit idempotency rules for callbacks/retries.

## 3. JSON Field Contract

- Distinguish **missing**, **`null`**, and **empty (`""` / `[]` / `{}`)** explicitly for every request/response field.
- State exact **units and bounds** (bytes vs UTF-8 chars/codepoints, integer cents vs decimal strings, RFC3339 timestamps, precision limits like `int64` / IEEE-754 safe range `2^53-1`).
- Specify handling of **unknown keys** (reject vs ignore), **duplicate keys** (reject), and **nesting/array limits**; record the enforcement layer (gateway / decoder / handler / DB).

## 4. File Upload Contract (When Applicable)

- Declare transport (`multipart/form-data`, presigned direct upload, chunked), max file count, per-file/total byte caps, allowed MIME + magic-byte content sniffing (never trust extension or `Content-Type` alone).
- Enforce upload ownership, quarantine/malware or image decode bounds, atomic commit/finalize semantics, and orphan cleanup on abort/timeout.

## 5. Minimum Contract Sections

| Section | Required facts |
|---|---|
| Locator & Auth | Purpose, method/path, callers, auth/role/ownership rule |
| Request / Response | Path, type, required, nullable, unit/bounds, enum source, sample |
| Behavior & Errors | Success, empty, invalid, 401/403/404/409/5xx, idempotency |
| Resource Limits | Payload/field/upload/batch limits, authority source, enforcement layer |
| Verification | Bound storyline route or real-entry test command & expected observation |
