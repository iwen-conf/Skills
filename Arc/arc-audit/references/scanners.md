# Scanners

Local-first tool scans for scan mode. Artifact paths, install permission and approvals:
[contract.md](../../_shared/contract.md) §5–§6. Ranking scanner signals: [appsec.md](appsec.md)
Phase 5.

## Tools

Verified 2026-10-05 against local `--help` output and official docs.

| Layer | Tool (verified) | Command run by `security-scan.py` | Runs when |
|---|---|---|---|
| Secrets | gitleaks 8.30 | `gitleaks git <root> --report-format json --report-path raw/gitleaks.json --redact --no-banner --exit-code 0` | always |
| SCA, secrets, IaC misconfig | trivy 0.74 | `trivy fs --format json --output raw/trivy.json --scanners vuln,secret,misconfig <root>` | always |
| SAST | semgrep 1.176 | `semgrep scan --config p/default --json --output raw/semgrep.json --metrics off <root>` | always |
| Go reachable vulns | govulncheck 1.8 | `govulncheck -json ./...` (`-json` is the legacy alias of `-format json`) | `go.mod` present |
| Go secure coding | gosec v2 | `gosec -fmt=json -out raw/gosec.json ./...` | `go.mod` present |
| JS dependencies | pnpm / npm audit | `pnpm audit --json` if `pnpm-lock.yaml`, else `npm audit --json` | `package.json` present |
| API fuzz (DAST) | schemathesis 4 | `schemathesis run <spec> --max-examples N --checks all [--url <target>]` | `--mode full` + OpenAPI |
| Template scan (DAST) | nuclei v3 | `nuclei -u <target> -jsonl -o raw/nuclei.jsonl -silent` | `--mode full` + `--target-url` |
| DAST baseline | ZAP `ghcr.io/zaproxy/zaproxy:stable` | `zap-baseline.py -t <target> -J -w -r` in Docker | `--mode full` + `--target-url` + Docker running |

Not run by the script (manual commands, same artifact directory):
- `gitleaks dir <root>`: working-tree files including uncommitted ones (`gitleaks git` reads history only).
- `osv-scanner scan source -r --format json <root> > raw/osv.json` (v2): SCA fallback when trivy is missing.

`p/default` downloads rules from the Semgrep registry without uploading code. IF network access
is not allowed THEN pass `--skip semgrep` and run semgrep with a local rules directory.

## Running

`SKILL_ROOT` = this skill's directory; `PROJECT_ROOT` = target root. Use absolute script paths
and keep the project as a separate argument.

```bash
python3 "$SKILL_ROOT/scripts/security-scan.py" "$PROJECT_ROOT" --task-slug <task-slug>
python3 "$SKILL_ROOT/scripts/security-scan.py" "$PROJECT_ROOT" --task-slug <task-slug> \
  --mode full --target-url http://localhost:8080 --openapi "$PROJECT_ROOT/openapi.yaml"
```

| Flag | Effect |
|---|---|
| `--task-slug S` | output to `<project>/.arc/artifacts/S/security/` (default slug `security-scan-<timestamp>`) |
| `--output-dir D` | override the output directory; use `…/security/rescan-<n>` to keep a pre-fix run for comparison |
| `--mode quick\|full` | `quick` = repository tools only; `full` adds schemathesis, nuclei, ZAP |
| `--skip TOOL` | repeatable; skipped tools are listed in the report |
| `--timeout SEC` | per tool, default 900 |
| `--max-examples N` | schemathesis, default 30 |
| `--install-missing` / `--pull-zap` | run the installer first; only with install permission |

Output: `security-report.md`, `security-report.html`, `security-summary.json`, `raw/`,
`logs/<tool>.stdout.txt|stderr.txt`, ZAP reports when ZAP ran. Re-running with the same slug
overwrites the previous files.

## Result contract

- Exit 0 means the report was generated, nothing more.
- Read each tool's `status` and `exit_code` in `security-summary.json`: `ok`,
  `findings_or_error`, `timeout`, `missing` and skipped are distinct outcomes.
- `findings_or_error` requires opening the raw artifact and the stderr log to tell findings
  from an execution error. A nonzero exit caused by findings is never ignored.
- The report is evidence, not a release gate. Gating needs an explicit project policy; absent
  or unknown evidence never passes a gate.
- The schemathesis signal count is a count of failure markers in text output; read the raw file.

## Install policy

1. Ask for permission naming the tools; then preview: `bash "$SKILL_ROOT/scripts/install-security-tools.sh" --dry-run --core`.
2. Install: `--core` (semgrep, trivy, gitleaks, gosec, govulncheck, nuclei, schemathesis),
   `--tools a,b`, `--with-zap` (Docker image), `--all` (core + ZAP), `--update-nuclei-templates`.
3. Record installed tools and versions in the report. Without permission, missing tools stay
   skipped and their manual checks run instead.

## Local-first policy

- Telemetry off (the script sets `SEMGREP_SEND_METRICS=off`, `--metrics off`, `TRIVY_NO_PROGRESS`).
- Never upload source, SARIF, reports, secrets, cookies or tokens to cloud scanners or
  dashboards. A commercial or cloud scanner needs user confirmation that names the data leaving
  the machine.
- Raw artifacts stay local; quoted output is redacted.

## DAST authorization

ZAP, nuclei, schemathesis and any fuzzing are active testing. Run them only when all hold:
1. The user wrote, in the current conversation, an authorization naming the target URL or host
   and the environment.
2. The target is owned by the user or covered by that authorization; production also needs
   explicit production consent.
3. Repository scans (`quick`) and asset recon ran first.

IF any condition fails THEN do not pass `--target-url` or `--mode full`; list DAST as `BLOCKED`
with the missing condition. Never scan third-party systems.

## Order on large repositories

1. Inventory entrypoints, OpenAPI files, lockfiles, Dockerfiles and config paths.
2. `quick` mode (secrets + SCA + SAST) before any DAST or template volume.
3. Expand to `full` only after soft targets and high-yield stores are known
   ([appsec.md](appsec.md) Phases 1–3).
4. Re-scan after a fix: narrow `PROJECT_ROOT` to the fixed module and use a new output directory.

## Triage

Assign every signal (or signal group) a status from the contract:

| Status | Requires |
|---|---|
| `confirmed` | code trace from a reachable entry to the flagged sink, or a real secret verified by format, location and use (never print it) |
| `likely` | flagged code is reachable but one hop is unverified (dynamic dispatch, runtime config) |
| `needs-triage` | scanner-only signal not yet traced |
| `false-positive` | evidence: test fixture, documented example value, unreachable or dead code, input sanitized upstream at `file:line`, vulnerable function not called (govulncheck symbol level) |
| `manual-gap` | the class tools cannot cover (table below) |

- Group by rule + root cause before ranking; one group lists every location.
- Final severity comes from [appsec.md](appsec.md) Phase 5 re-rank rules; keep the tool severity
  beside it.
- A confirmed secret's fix direction is rotation plus removal from history and artifacts, not
  only deletion from the current file.

## Coverage limits

| Tools cover well | Manual only |
|---|---|
| secrets in git history, known-CVE dependencies, common injection patterns, weak crypto | object-level AuthZ, tenant isolation |
| container/IaC misconfiguration, missing security headers (DAST) | payment amount and callback trust, credit double spend |
| common XSS/SSRF patterns (partial) | approval workflow bypass, race conditions |
| Go vulnerability reachability (govulncheck) | half-implemented features reachable in production |

Never claim business-logic coverage from tool output.

## Manual checklist

Run for every `manual-gap` area and for every skipped or failed tool's layer:

- AuthZ: cross-user object access, tenant boundary, role/action matrix.
- AuthN/session: JWT validation, cookie flags, token storage, logout/revocation.
- Business flow: approval steps, payment amount, coupon/credit logic, idempotency, replay.
- Input: upload MIME/content validation, path traversal, SSRF allowlists.
- Data layer: ownership filters, soft-delete filters, pagination bounds, sort allowlists.
- Frontend: route guards, sensitive data in local storage, reflected/stored XSS surfaces.
- Operations: secrets in logs, debug endpoints, permissive CORS, missing security headers,
  rate limits, audit logging.
- Config/deploy: real-looking secrets in samples, default admin bootstrap, pprof/debug in
  release bundles.
- Half-finished auth: mounted forgot-password/reset/register routes without complete logic.
- Dependencies (when SCA tools are missing): read lockfiles for pinned versions of auth,
  crypto, parser and web-framework packages; check each against the vendor advisory page.
