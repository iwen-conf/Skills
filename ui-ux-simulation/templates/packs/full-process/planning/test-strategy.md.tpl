# Test Strategy

## 1. Background
- Product/Project: `<name>`
- Run ID / Reference: `{{RUN_ID}}`
- Objective: {{OBJECTIVE}}
- Target URL: `{{TARGET_URL}}`

## 2. Quality Goals
| Attribute | Goal | Notes |
|---|---|---|
| Functional | Core flows stable | E2E + data verification |
| UX | Major friction removed | Heuristic + usability |
| Accessibility | Meet WCAG 2.2 AA (target) | A11y audit |
| Compatibility | Target browsers/devices | Matrix execution |
| Performance | Meet budget | RUM/Lab budgets |

## 3. Scope
### In Scope
- `<in-scope feature 1>`
- `<in-scope feature 2>`

### Out of Scope
- `<out-of-scope feature 1>`

## 4. Test Approach
### UI/UX Simulation (E2E)
- Persona switching, session isolation, evidence screenshots
- Read-only backend verification (DB SELECT)

### Exploratory / Scenario-based
- Charters based on user journeys and risk

### UX Evaluation
- Heuristic evaluation (Nielsen/consistency/error prevention/etc.)
- Usability sessions (moderated/unmoderated) when applicable

### Accessibility Audit
- Keyboard-only navigation
- Screen reader spot checks
- Color contrast & focus visibility

### Compatibility
- Browser/OS/responsive checks across critical pages

### Visual Regression
- Baseline screenshots + diff triage process

### Performance
- Define budget for LCP/INP/CLS/TTFB and track per release

## 5. Environments & Tooling
| Item | Value |
|---|---|
| Env Name | {{ENV_NAME}} |
| Base URL | `{{TARGET_URL}}` |
| Validation Container | {{VALIDATION_CONTAINER}} |
| Browser | chromium (headless) |
| Automation | agent-browser + ui-ux-simulation |

## 6. Entry / Exit Criteria
### Entry Criteria
- Test env deployed and reachable
- Test accounts ready (personas)
- Critical dependencies healthy

### Exit Criteria
- P0/P1 defects closed or accepted
- Critical E2E flows PASS
- Deliverables complete (reports, manifests, defect log)

## 7. Risks & Mitigations
| Risk | Probability | Impact | Mitigation | Owner |
|---|---|---|---|---|
| Test data unstable | M | H | Data plan + reset scripts (read-only validation) | `<owner>` |
| Flaky UI due to async | M | M | Stabilize waits, network-idle, deterministic selectors | `<owner>` |

## 8. Deliverables
- `report.md` + evidence artifacts
- Defect log & failure reports
- Test summary / release recommendation
- UX/A11y/compat/perf add-ons (as applicable)

