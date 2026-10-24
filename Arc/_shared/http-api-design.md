# HTTP API design rules

Use for a new/changed HTTP contract or a requested design review. A valid document is not proof
of a sound API. Read only the relevant detail: [JSON fields](json-field-contract.md),
[uploads](file-upload-contract.md). Writing format stays in [api-contract.md](api-contract.md).

## Basis and freshness

Label every constraint: **standard** (specification + section + MUST/SHOULD/MAY), **guidance**
(security recommendation), **contract** (project decision), or **implementation** (verified
library/config/deployment limit). These labels describe the rule's authority, not severity.
Do not elevate a SHOULD, OWASP recommendation, draft, or style preference into a protocol MUST.

For a design audit, search the web and open primary sources relevant to its claims: RFC Editor,
JSON Schema, OpenAPI, OWASP, and the actual gateway/framework/storage vendor. Record exact URL,
section, version and check date; read beyond search snippets and check updates/errata. The sources
below were checked on 2026-10-06; they are starting points, not a permanently current catalog.
If network access fails, state which authority/version is unverified, use existing evidence for
independent checks, and do not invent a requirement. Do not send private schemas, source or data
to public search or online validators. Ordinary implementation may reuse applicable current research.

## Review surfaces

| Surface | Question and evidence |
|---|---|
| Purpose and boundary | Does the endpoint support the actual caller's operation without missing states, unnecessary round trips, leaking internal models or ambiguous ownership? Trace project requirements and consumers. RPC-style paths are not inherently defects. |
| Methods | Check safe/idempotent semantics, representation replacement vs partial update, and retry consequences. GET must not be the requested mechanism for destructive business actions. Idempotency does not require identical responses. GET content has no generally defined semantics; assess intermediary/client support. |
| Status and media | Distinguish 401/403, conflict/precondition failure, 413 size, 415 media and validation failures. 400 vs 422 follows applicable semantics and the chosen contract. 204 has no response content. 202 accepts work, not completion; define the observable terminal outcome. |
| Errors | Use the existing compatible machine-readable error contract. RFC 9457 is an available format, not mandatory for all APIs; if adopted, honor its media type and member semantics. Its generated `status` must match the HTTP response. Avoid leaking internals. |
| Updates and retries | Define duplicate-operation identity, replay/mismatch/expiry behavior and lost-response recovery when the business needs them. Check stale-write protection (conditional requests or a documented version mechanism) and the supported retry policy. A header's name alone proves nothing. |
| Reads and caching | Define freshness, authorization isolation and validators for cacheable representations. Check actual proxy/CDN rules and cache keys for tenant-specific responses; use RFC 9111 when caching is involved. |
| Lists and batches | Establish maximum work, stable ordering and tie-breaks, filter/sort allowlists, cursor scope/expiry, and partial failure or all-or-nothing semantics. Pagination style is a project decision; unbounded work needs a concrete resource analysis. |
| Authorization | Separate caller authentication, object/tenant access, writable properties and response visibility. Trace actual middleware and field projections; a client-supplied owner/role or schema annotation cannot grant authority. |
| Limits and lifetime | Define payload, parsing, processing, response and storage limits across the deployed path. Include timeout/cancellation and cleanup where work survives a request. Use applicable upload/JSON checks rather than a universal number. |
| Evolution | Check old/new readers and writers, required-field additions, narrowed ranges/enums, changed defaults/null semantics and error/status changes. Record actual clients and rollout path; an additive response field is not universally safe for strict clients. |

HTTP semantics: [RFC 9110 §§8–9, 13, 15](https://www.rfc-editor.org/rfc/rfc9110.html).
Errors: [RFC 9457 §§3, 5](https://www.rfc-editor.org/rfc/rfc9457.html).
Caching: [RFC 9111](https://www.rfc-editor.org/rfc/rfc9111.html).
Property authorization: [OWASP API3:2023](https://api-security.owasp.org/editions/2023/en/0xa3-broken-object-property-level-authorization/).
Resource costs: [OWASP API4:2023](https://api-security.owasp.org/editions/2023/en/0xa4-unrestricted-resource-consumption/).

The purpose, cost, rollout and evidence checks are engineering review methods; they are not
claimed as verbatim protocol requirements. Check a selected PATCH media type's own specification
before deciding omitted/null/array semantics; do not infer them from the method alone.

## Constraint ledger and handoff

For each relevant limit record **target + value + unit/counting rule + source + enforcement layer
+ rejection/result + boundary case**. Include unlimited/unknown settings explicitly; investigate
whether a shared layer covers them before reporting a missing control. HTTP/JSON do not define a
universal upload MB limit or field-length cap. Choose numbers from the business, consumers,
provider limits and resource budget; do not import tutorial defaults.

Review follows [api-design-review.md](../arc:audit/references/api-design-review.md). Planning and
implementation copy accepted rules into the existing endpoint contract; unresolved limits remain
`待确认`. Verification binds [API cases](../arc:verify/references/api-contract-tests.md) to the same
rule IDs. State/lifecycle gaps return to `arc:plan` model mode; defects to `arc:code`; evidence to
`arc:verify`. No phase gains write, upload, probe or deployment authority from reading this file.
