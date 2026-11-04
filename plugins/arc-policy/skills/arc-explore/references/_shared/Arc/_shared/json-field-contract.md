# JSON field constraints

Apply to actual JSON requests/responses. A schema describes parsed data; a conforming document,
generated SDK or validator exit code does not establish server enforcement or authorization.

## Wire and parser boundary

Under [RFC 8259 §§4, 6, 8–9](https://www.rfc-editor.org/rfc/rfc8259.html), interoperable exchange
uses UTF-8; NaN and Infinity are not JSON numbers. Names SHOULD be unique, not MUST be rejected.
Set a project duplicate-key policy and compare parser behavior, including escape-equivalent names;
a schema cannot detect duplicates already discarded by decoding. Numbers outside ±(2^53−1) are
not syntactically forbidden, but binary64 clients may lose integer identity. Specify exact
representations for IDs/decimals from real consumer requirements. Parsers may impose size/depth/
number/string limits; the RFC supplies no universal cap. Verify rejection before expensive work.

## Field ledger

| Dimension | Review and counterexample |
|---|---|
| Presence and emptiness | Separate absent, null, empty string/container, zero and false. `required` tests presence; null needs an allowed type. Define create vs partial-update meaning and who applies defaults; don't mistake `default` annotations for inserted values. |
| String | State min/max length and counting unit, normalization policy and allowed syntax. JSON Schema string length counts Unicode characters, not UTF-8 bytes, UTF-16 units or displayed grapheme clusters. Test emoji, combining characters and escaped equivalents; layer an independent byte budget where needed. |
| Number | State integer/decimal domain, inclusive/exclusive bounds, units and precision across clients. Check large integers, overflow/exponents, negative/zero and fractional values; a type/format name does not ensure exact storage or arithmetic. |
| Array | Bound count and per-item cost; define uniqueness/order and empty behavior. An item count alone does not bound nested content or fan-out. |
| Object | Define required and allowed keys, key-name constraints and property count. `properties` does not make keys required. Unknown-property rejection is a compatibility/security choice, not a universal requirement. |
| Semantic constraints | Specify enum sources, cross-field dependencies, allowed combinations, date/time offsets and ranges. Distinguish syntax validation from entity existence, ownership and state rules. |
| Read/write exposure | Document request-writable vs response-visible fields per permitted actor/state. `readOnly`, `writeOnly` or rejecting unknown keys does not implement authorization or safe response projection. |

For size-bearing fields add enforcement location, failure response, evidence and consumers to the
ledger. Test normalization before/after validation where it changes length, equality or signing.
Do not silently coerce invalid types or truncate data unless the contract explicitly defines it.

## Schema and tool behavior

- Lock the OpenAPI version, JSON Schema dialect, parser/validator configuration and generator.
  OAS 3.0's Schema Object is not full JSON Schema 2020-12; its `nullable` requires an explicit
  same-level `type`, and other constraints can still reject null. OAS 3.1 uses 2020-12 and permits
  a null type union. Check the exact version in use, including newer versions; no forced upgrade.
- In JSON Schema 2020-12, `format` is normally annotation; format assertion depends on the
  vocabulary/validator configuration. Demonstrate rejection with a concrete invalid instance.
  `pattern` is not implicitly anchored. Check regular-expression cost for untrusted input.
- Omitted `additionalProperties` allows extras. Its interaction with `properties` and
  `patternProperties` is local to the same schema object; composed schemas need instance checks.
  Use `unevaluatedProperties` only in a supported dialect/toolchain when its semantics are wanted.
- Validate both positive and negative instances through the deployed decoder/validator, including
  composed schemas and their consumers. A documented restriction not exercised by the selected
  runtime remains unverified. A default allowed by one branch may fail another.

Sources checked 2026-10-06:

- [JSON Schema Validation 2020-12 §§6–9](https://json-schema.org/draft/2020-12/json-schema-validation): field, format, content and annotation semantics.
- [JSON Schema Core §§4.2.1, 10.3.2.3, 11.3](https://json-schema.org/draft/2020-12/json-schema-core): parsed model and property evaluation.
- [OAS 3.0.4 Schema Object](https://spec.openapis.org/oas/v3.0.4.html#schema-object), [OAS 3.1.2 Schema Object](https://spec.openapis.org/oas/v3.1.2.html#schema-object): version-specific rules.
- [OWASP API3:2023](https://api-security.owasp.org/editions/2023/en/0xa3-broken-object-property-level-authorization/): property-level authorization.

JSON string length is distinct from binary payload length. For uploads, including OpenAPI's
version-specific binary `maxLength` semantics, read [file-upload-contract.md](file-upload-contract.md).
