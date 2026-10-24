# File upload design

Use for multipart, raw binary, encoded files in JSON, presigned direct upload, multipart/resumable
storage sessions and finalization endpoints. Choose only the actual transport/lifecycle in scope.

## Transport and resource contract

- Record accepted media/encoding, part names, whether a part repeats, metadata format, file count
  and missing/empty-part behavior. For `multipart/form-data`, verify boundary parsing and per-part
  disposition. Multiple files for one field use separate parts with the same name; do not reject
  repeated parts as though they were duplicate JSON keys. Never trust filename paths.
  [RFC 7578 §§4.1–4.4, 5.2, 7](https://www.rfc-editor.org/rfc/rfc7578.html).
- Specify total request bytes (including multipart overhead), each file's bytes, metadata bytes,
  part/header counts, decoded bytes, concurrency, time and temporary/persistent storage budgets.
  For archives or media processing, assess expanded bytes, file count, pixel/frame dimensions and
  processing cost as applicable. Rate limiting by request count alone can miss resource amplification.
- Trace gateway → streaming reader/parser → validation → processing/storage. A `Content-Length`
  precheck is not sufficient; enforce actual bytes consumed, including supported unknown-length
  streams. Check wire vs decompressed limits and cancellation/cleanup. Reading the entire body
  into memory before checking a limit does not prove bounded ingress memory.
- Direct-to-storage uploads bypass the application ingress limit. Inspect the provider's actual
  signature/policy capabilities, expiry, allowed object key, size/checksum conditions, completion
  checks and authorization. Do not assume a presigned URL can enforce every declared constraint.
- Keep schema length units explicit: JSON strings use character length. OAS 3.0.4 and 3.1.2 §4.4.2 allow
  `maxLength` to describe unencoded binary streaming length in octets; verify dialect and tooling.
  Base64-text length and decoded file length differ; bound both when needed. No schema declaration
  alone proves that the gateway, parser or object store enforces the limit.
  [OAS 3.0.4](https://spec.openapis.org/oas/v3.0.4.html#working-with-binary-data) /
  [OAS 3.1.2 Working with Binary Data](https://spec.openapis.org/oas/v3.1.2.html#working-with-binary-data).
  These are verified examples, not a claim that all other versions lack binary length support;
  read the corresponding section for the version declared by the input.

## Content, ownership and lifecycle

| Boundary | Inspect |
|---|---|
| Admission | Business-driven type allowlist; extension decoding; content/signature checks; trusted media processing. Client MIME and extensions are hints; signatures alone also do not establish safety. Apply antivirus/CDR where warranted by the content and threat model. |
| Naming/storage | Server-controlled object identity, collision policy, path isolation, non-executable/private storage, authorized retrieval and content disposition. Treat an original filename as display metadata, not a trusted filesystem path. |
| Actor/object | Permission to initiate, upload, finalize, attach, read and delete the actual object. Verify tenant/object binding at every relevant step; a valid object URL or client checksum does not establish ownership. |
| Publication | Define which state makes a file available. Check required validation/scan completion before publishing; provider/client "upload succeeded" alone is not business acceptance. Prevent post-validation replacement or validate the same immutable object/version at use. |
| Finalization | Where sessions exist, define expiry, resume/retry, repeated completion, concurrent attachment and incomplete parts. Recheck persisted object identity, actual size and required integrity metadata server-side. |
| Failure/cleanup | Specify aborted upload, timeout, rejected content, failed processing, abandoned sessions and unreferenced objects. Name cleanup ownership and observable terminal states; bounded storage cannot depend on the client returning. |

These are risk-based design checks, not a requirement to add every mechanism to every endpoint.
Identify actual guards before reporting a missing one; defer unknown limits to the project owner.
HTTP 413/415 and validation responses must be consistent with the endpoint's media/error contract.
Do not silently truncate or return success for rejected data.

Sources checked 2026-10-06: [OWASP File Upload Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/File_Upload_Cheat_Sheet.html)
(defense in depth), [OWASP API4:2023](https://api-security.owasp.org/editions/2023/en/0xa4-unrestricted-resource-consumption/)
(resource consumption). Lifecycle/finalization rows are engineering consequences to validate
against the selected storage provider and business contract, not invented RFC mandates.

Verification uses [API boundary cases](../arc:verify/references/api-contract-tests.md). During a
read-only audit inspect current evidence only; uploading files, fuzzing or stressing a shared
target requires authorization for those actions. Do not upload source or samples to public scanners.
