# API design review

Review a Markdown contract, existing OpenAPI/schema, source or their combination. A document-only
review is valid without a running server. Keep the input read-only and put reports in the task's
artifact directory. Do not create or rewrite contracts during this mode.

1. Record scope, inputs/revisions, consumers, actual API/schema versions and available runtime
   evidence. Hash the reviewed inputs before and after, including untracked documents and inputs
   inside the otherwise excluded artifact directory. In a repository also compare worktree/index
   diffs; unchanged `git status` alone cannot prove unchanged bytes. Discover local routes,
   DTOs, validators, auth middleware, gateway limits, upload/storage
   handlers and clients with `arc-idx` when source is provided. Missing source is a coverage gap,
   not grounds to assume a missing control. Without a repository, cite document sections/lines.
2. Apply [HTTP design](../../_shared/http-api-design.md), loading its JSON/upload references only
   where relevant. Perform its primary-source web verification. Record source version/section,
   rule class and applicability; project/vendor limits need their own evidence.
3. Build an endpoint × surface coverage table: method/status/media, fields/parser, authorization,
   limits, upload lifecycle, retries/state, compatibility. Mark checked, N/A with reason, or
   unverified. For each relevant limit produce the shared constraint ledger.
4. Try to disprove each suspected defect: read inherited middleware/schema/config, check the
   correct dialect, distinguish bytes from characters and transport from decoded size, trace
   caller expectations and intended rollout. Record a minimal counterexample and expected vs
   specified/observed result. Do not execute probes in a read-only review.
5. Classify the evidence target separately from the normal severity/status:
   - **design defect**: a contract contradicts an applicable rule or cannot satisfy its own
     stated behavior. May be `confirmed` from exact sections and a failed disproof, with no claim
     that the runtime is exploitable. Example: the same response is specified as 204 with a body.
   - **implementation defect**: source/runtime contradicts an accepted constraint. Confirmation
     follows the main skill's entry-to-sink/reproduction rule; missing gateway evidence stays open.
   - **specification gap**: a necessary choice/limit/error is absent or ambiguous; cite scope and
     consequence, identify the decision owner, and do not fabricate a value or runtime failure.
   - **evidence gap**: contract exists but enforcement/reachability has not been checked. An
     unexecuted probe, unavailable validator or absent server cannot establish PASS or a defect.
6. Produce cards with ID, target kind, severity/status, endpoint/field, input location, normative
   source+section/version or project rule, counterexample, disproof/result, consequence, enforcement
   layer, affected consumers/siblings, minimal repair direction, owner and acceptance case.
   Separate confirmed defects, unresolved decisions, evidence gaps and optional improvements.
7. Summarize coverage and current limits. Do not call naming preferences or a different error
   envelope an RFC violation. No findings in checked scope is valid. A document-only conclusion
   is design evidence, not runtime or release qualification.

Handoff within the same user-authorized task: `arc:plan` clarifies/updates the contract or tasks;
`arc:plan` model mode handles business lifecycle gaps; `arc:code` implements accepted repairs;
`arc:verify` binds [boundary cases](../../arc:verify/references/api-contract-tests.md). Preserve IDs
and accepted authorization. Read-only-only requests stop after the report.
