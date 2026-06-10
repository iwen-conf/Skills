---
name: arc:security
description: "Security automation for SAST, SCA, secrets, Go, API fuzz, DAST scans, and readable Arc handoffs."
---

# arc:security

## Overview

`arc:security` runs local-first security automation for application repositories. It installs and orchestrates CLI scanners, produces human-readable reports, and routes remediation or durable project records through existing Arc skills.

Read [`references/security-tooling.md`](references/security-tooling.md) when choosing tools, interpreting scanner limits, or explaining coverage to a user.

## Quick Contract

- **Trigger**: The user asks for security scanning, vulnerability assessment, secure release checks, dependency/secrets review, API fuzzing, or a readable security report.
- **Inputs**: Project path, optional target URL, optional OpenAPI spec, scan mode, installation preference, and expected report format.
- **Outputs**: Local security reports, raw scanner artifacts, prioritized findings, remediation handoff, and optional Lark handoff.
- **Quality Gate**: Scans are local-first, reproducible, evidence-backed, readable, and explicit about skipped tools and manual test gaps.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../../docs/arc-routing-matrix.md).

## Routing Matrix

- Use `arc:clarify` first if target environment, scan scope, authorization, or destructive-test boundaries are unclear.
- Use `arc:audit` when the user wants a read-only security review without running active scanners.
- Use `arc:build` when the task is to add security tooling, scripts, checks, or project configuration.
- Use `arc:fix` when a concrete vulnerability, failing scanner result, or exploit path must be repaired.
- Use `arc:frontend` for frontend-specific XSS, CSP, auth UI, route guard, token handling, or browser verification work.
- Use `arc:docs` only when Lark is active for security reports, risk rows, remediation tasks, approval gates, artifacts, or `.lark.json.lifecycle[]`.

## Context Search

- MUST inspect project type, package managers, API specs, Docker files, auth boundaries, and existing security tooling before scanning.
- MUST use `.ai-code-index/search.sh` first for broad repository context when available.
- MUST use `.ai-code-index/struct-search.sh` for risky code shapes such as raw SQL, shell execution, file upload, SSRF fetches, JWT parsing, or auth bypasses.
- If `.lark.json` exists, MUST read it before security handoff and route durable reports through `arc:docs`.

## Announce

Begin by stating clearly:
"I am using `arc:security` to run local-first security automation and produce readable evidence-backed reports."

## The Iron Law

```text
NO SECURITY CLAIM WITHOUT EVIDENCE.
NO ACTIVE DAST AGAINST A TARGET WITHOUT AUTHORIZATION.
NO CLOUD SCAN OR PAID SERVICE WITHOUT USER CONFIRMATION.
NO LARK SECURITY UPDATE OUTSIDE arc:docs.
```

## Hard Constraints

- MUST keep scanning local-first: no paid SaaS, remote GraphRAG, external vector DB, or persistent remote memory by default.
- MUST disable telemetry where supported and avoid uploading source code or full reports to cloud services unless the user explicitly approves.
- MUST treat DAST, fuzzing, Nuclei, and ZAP as active testing; run them only against owned or authorized targets.
- MUST redact tokens, cookies, private keys, passwords, and internal secrets from final chat output.
- MUST preserve raw scanner artifacts locally and summarize findings in Markdown/HTML/JSON.
- MUST separate confirmed findings, tool warnings, skipped checks, and manual-test gaps.
- MUST route all Lark writes through `arc:docs`.
- MUST NOT create or request Lark resources when `.lark.json` is absent and the user did not explicitly trigger or confirm Lark.
- NEVER claim business-logic coverage from automated scanners alone.
- NEVER treat scanner severity as final severity without checking exploitability, reachability, auth context, and affected environment.

## Workflow

1. Confirm authorization, scan scope, target URL, OpenAPI spec, and whether CLI installation is allowed.
2. Inspect the project with local index tools and detect stack markers such as `go.mod`, `package.json`, lockfiles, `Dockerfile`, and OpenAPI files.
3. Install missing local CLI tools with [`scripts/install-security-tools.sh`](scripts/install-security-tools.sh) when requested or necessary.
4. Run [`scripts/security-scan.py`](scripts/security-scan.py) with the project path and optional `--target-url` / `--openapi` arguments.
5. Review generated `security-report.md`, `security-report.html`, `security-summary.json`, and raw tool outputs under `.arc/security/<timestamp>/`.
6. Prioritize findings by confirmed exploitability, asset exposure, severity, reachability, and remediation cost.
7. Route fixes to `arc:fix` or `arc:build`; route read-only review to `arc:audit`; route active Lark project records to `arc:docs`.

## Quality Gates

- Report includes command status, skipped-tool reasons, raw artifact links, severity counts, and actionable next steps.
- SAST, SCA, secrets, Go vulnerability, Go secure coding, package audit, API fuzz, and DAST coverage are selected only when relevant to the project.
- AuthZ, ownership, payment amount, approval flow, tenant boundary, upload, SSRF, and role-bypass gaps are marked as manual checks.
- Scanner findings are deduplicated or grouped before remediation planning when practical.
- Security artifacts stay local unless `.lark.json` is active or the user explicitly asks for remote publication.

## Expert Standards

- Use `SAST` for source scanning, `SCA` for dependency and container risk, `DAST` for running services, and `OpenAPI Fuzz` for API robustness.
- Track `SBOM`, `SARIF`, `CWE`, `CVSS`, and `OWASP Top 10` metadata when tools provide it.
- Use `OWASP ASVS` thinking for authn, `AuthZ`, session, API, file upload, and business-logic checks.
- Treat secrets exposure, supply-chain compromise, auth bypass, RCE, SQL injection, SSRF, and payment tampering as priority risk classes.
- Validate scanner output against code reachability, deployment exposure, compensating controls, and exploit prerequisites.

## Scripts & Commands

Install local CLI tools:

```bash
Arc/arc:security/scripts/install-security-tools.sh --core
```

Run a local repository scan:

```bash
Arc/arc:security/scripts/security-scan.py /path/to/project --install-missing
```

Run full app scanning against an authorized local target:

```bash
Arc/arc:security/scripts/security-scan.py /path/to/project \
  --target-url http://localhost:8080 \
  --openapi /path/to/project/openapi.yaml \
  --mode full \
  --install-missing
```

The scan script writes reports to `.arc/security/<timestamp>/` by default:

- `security-report.md`
- `security-report.html`
- `security-summary.json`
- `raw/` scanner outputs
- `logs/` command stdout/stderr

## Red Flags

- Running ZAP, Nuclei, or fuzzing against a third-party system without explicit authorization.
- Uploading source, scanner results, secrets, or SARIF to a cloud service without confirmation.
- Reporting only scanner output with no manual authz/business-logic gap statement.
- Ignoring nonzero scanner exit codes caused by findings.
- Marking a Lark-active security task complete while `task_base` or risk records are stale.

## When to Use

- **Preferred Trigger**: security scan, vulnerability scan, dependency audit, secrets audit, Go security check, API fuzzing, ZAP/Nuclei scan, or security report.
- **Typical Scenario**: Go + React repository release check using Semgrep, Trivy, Gitleaks, Gosec, Govulncheck, package audit, Schemathesis, ZAP, and Nuclei.
- **Boundary Tip**: use `arc:audit` for read-only review only; use `arc:fix` for confirmed vulnerabilities; use `arc:docs` only when Lark is active.

## Input Arguments

| parameter | type | required | description |
|---|---|---|---|
| `project_path` | string | yes | Target repository root |
| `target_url` | string | no | Authorized running app URL for DAST tools |
| `openapi` | string | no | OpenAPI file or URL for API fuzzing |
| `mode` | enum | no | `quick` for local repository checks, `full` for local checks plus API/DAST |
| `install_missing` | boolean | no | Whether to install missing local CLI tools before scanning |
| `output_dir` | string | no | Directory for reports and raw artifacts |

## Outputs

```text
Security Handoff
- Report paths
- Tool coverage and skipped checks
- Severity summary
- Confirmed findings and evidence
- Manual authz/business-logic gaps
- Recommended arc:fix / arc:build / arc:audit / arc:docs next steps
- Lark / .lark.json / task_base handoff, if applicable
```
