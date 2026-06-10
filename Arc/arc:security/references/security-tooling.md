# Security Tooling Reference

Use this reference when `arc:security` needs to choose tools, explain coverage, or interpret scanner limits.

## Tool Matrix

| Layer | Tool | Default Use | Notes |
|---|---|---|---|
| Secrets | Gitleaks | Always for repositories | Redact findings in chat; raw report stays local. |
| SCA | Trivy | Repositories, lockfiles, Dockerfiles, IaC | Also reports secrets and misconfigurations when enabled. |
| SAST | Semgrep | Source code rules | Run locally with metrics off; do not use cloud upload by default. |
| Go vulnerabilities | Govulncheck | Go modules | Official Go reachability-aware vulnerability check. |
| Go secure coding | Gosec | Go source | Flags crypto, file permission, command execution, and injection risks. |
| JS dependencies | npm/pnpm/yarn audit | React/Node projects | Use the detected package manager when possible. |
| API fuzz | Schemathesis | OpenAPI specs | Requires an authorized target or spec with usable server URLs. |
| DAST baseline | OWASP ZAP | Running web apps | Active testing; only run against owned or authorized URLs. |
| Template scan | Nuclei | Running services | High template volume and false positives; verify before reporting as confirmed. |

## Coverage Limits

Automated tools are strong at common technical classes: secrets, vulnerable dependencies, SQL injection patterns, XSS patterns, SSRF patterns, weak crypto, risky file permissions, Docker/IaC misconfigurations, and missing web headers.

Automated tools are weak at business logic: user A reading user B data, normal users calling admin actions, workflow approval bypass, payment amount tampering, tenant boundary failures, rate-limit abuse, and contextual authorization.

## Local-First Policy

- Prefer local CLI tools and local reports.
- Disable telemetry when the tool supports it.
- Do not upload source code, SARIF, reports, secrets, cookies, or tokens to cloud scanning services unless the user explicitly approves.
- Before using commercial scanners or cloud dashboards, ask for confirmation and name what data will leave the machine.

## Report Interpretation

- Treat scanner severity as an input, not the final risk.
- Re-rank by reachability, authentication requirements, exposed environment, exploit maturity, business impact, and compensating controls.
- Separate `confirmed`, `likely`, `needs triage`, `false positive`, and `manual gap`.
- Keep raw scanner files linked from the Markdown/HTML report so humans can inspect evidence.

## Manual Security Checklist

Use this checklist after automation:

- AuthZ: cross-user object access, tenant boundary, role/action matrix.
- AuthN/session: JWT validation, cookie flags, token storage, logout/revocation behavior.
- Business flow: approval steps, payment amount, coupon/credit logic, idempotency.
- Input handling: upload MIME/content validation, path traversal, SSRF allowlists.
- Data layer: ownership filters, soft-delete filters, pagination bounds, sort allowlists.
- Frontend: route guards, sensitive data in local storage, reflected/stored XSS surfaces.
- Operations: secrets in logs, debug endpoints, permissive CORS, missing security headers.
