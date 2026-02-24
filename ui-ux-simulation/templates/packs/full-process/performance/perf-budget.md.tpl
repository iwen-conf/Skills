# Web Performance Budget

## Scope & Method
- Reference Run ID: `{{RUN_ID}}`
- Pages/Flows: `<list>`
- Method: Lab (Lighthouse) / RUM / both
- Network/CPU profile: `<profile>`

## Budget (suggested)
| Metric | Target | Notes |
|---|---:|---|
| LCP | <= 2.5s | 75th percentile |
| INP | <= 200ms | 75th percentile |
| CLS | <= 0.1 | 75th percentile |
| TTFB | <= 0.8s | server-side |

## Results
| Page/Flow | LCP | INP | CLS | TTFB | Verdict | Evidence |
|---|---:|---:|---:|---:|---|---|
| `<page>` | `<n>` | `<n>` | `<n>` | `<n>` | PASS | `<link>` |

