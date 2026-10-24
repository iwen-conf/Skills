# API contract and boundary evidence

Use with the active verify mode when HTTP semantics, JSON fields or upload constraints are in
scope. Expected results come from the accepted [API contract](../../_shared/api-contract.md) and
[design rules](../../_shared/http-api-design.md), not from the current implementation's output.

For each required claim record endpoint/field, rule/limit source, input, expected response and
side effects, actual enforcement layer, observer, candidate/config identity and result. An
undefined cap or error remains `SPECIFICATION GAP`; do not choose 10 MB/255 characters to fill it.

Select applicable cases; this is not a mandatory full-suite expansion for every endpoint:

| Claim surface | Discriminating cases and oracle |
|---|---|
| HTTP behavior | Correct method/media; relevant auth/conflict/precondition errors; 204 actually has no content; asynchronous acceptance reaches the defined status/result; retry does not produce forbidden extra effects. |
| Presence/type | Missing, null, empty, zero/false, wrong type, unknown and forbidden writable fields; assert response projection and no unauthorized side effect. |
| Strings/numbers | Just below/at/above the actual bound; Unicode/emoji/combining/escaped equivalents; large integer/exponent/precision edge in each supported consumer. Compare the contracted unit and exact value, not only status 200. |
| Parser/schema | Raw duplicate and escape-equivalent keys before parsing, supported depth/byte limits, invalid format and unanchored pattern counterexamples, composed schema branches. Exercise the actual parser/validator configuration. |
| Work amplification | Arrays, nested payloads, batch sizes, pagination/cursor/filter limits and expensive downstream calls. Observe count/cost with safe bounded fixtures; a small request can expand into large work. |
| Upload transport | Total vs per-file vs metadata limits, multipart overhead/counts, repeated file parts, missing/empty part, unsupported type, supported unknown-length stream, decoded/expanded limit. Confirm bounded handling and correct rejection. |
| Upload integrity/access | Mismatched extension/MIME/content, untrusted filename, cross-actor attachment/read/finalize, incomplete or expired direct-upload session, replaced object/version, publication before required checks. Inspect storage state and permissions. |
| Upload lifecycle | Interrupted/cancelled/rejected transfer, failed processing, retries/concurrent completion, abandoned session cleanup. Assert the promised state and absence of unauthorized/orphan side effects within the defined cleanup window. |
| Compatibility | Old/new clients, request/response field additions and changed null/default/enum/range behavior, supported rollout and error handling; generated client compilation alone is insufficient. |

A helper test proves its helper only. Use the real HTTP decoder/middleware for request claims;
gateway/CDN limits require that configured ingress, and direct-upload claims require the provider
path. Local handler tests do not qualify bypassed layers. Never use mocks to mark those claims PASS.

Use synthetic minimal fixtures and bounded resource consumption. Test-data writes on existing
shared/remote services, adversarial probes, fuzzing and load follow the active environment and
authorization boundaries; a design/audit request does not authorize them. A read-only verify phase
reviews existing evidence and marks unexecuted cases `NOT_RUN`; unavailable dependencies block
only their claims. No local PostgreSQL or container service is created for API verification.

Reuse existing runners and error contracts. For meaningful new checks prove sensitivity to the
targeted bad behavior, and preserve source/config identity. Report per-claim results and gaps;
schema validation, test counts and a well-formed OpenAPI file are not release evidence by themselves.
