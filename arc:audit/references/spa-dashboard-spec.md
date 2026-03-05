# SPA Dashboard Specification (9-Tab Standard)

> Condensed from the full SPA Project Evaluation Dashboard Specification V1.0.
> Referenced by `integrate_score.py` for HTML dashboard generation.

## Global Standards

- **Color Palette**: Success #52C41A, Warning #FAAD14, Danger #F5222D, Offline #BFBFBF
- **Grid System**: 24-column grid, card layout, 16px/24px spacing
- **Interaction**: Global time sandbox + environment filter; drill-down from macro to trace-level
- **Visualization**: ECharts-driven charts; WebGL acceleration for 10,000+ node topologies

## 9-Tab Structure

| # | Tab | Category | KPIs/SLIs | Primary Chart | Secondary Chart |
|---|-----|----------|-----------|---------------|-----------------|
| 1 | Executive Overview | Global | Apdex, 7-dim radar score (0-100) | Radar Chart | 3D Donut Gauge + Sparkline Cards |
| 2 | Business Completion | Business | Delivery Rate, Burn-down Rate | Sunburst Diagram | Interactive Gantt Chart |
| 3 | Business Connection | Business | Node Degree (In/Out), Dependency Depth | Force-Directed Graph | 3D Architecture Perspective |
| 4 | Business Connectivity | Business | Interface Success Rate, Message Delivery Rate | Sankey Diagram | Hexbin Map |
| 5 | Business Logic Fluency | Business | Conversion Rate, Fallback Rate | Process Mining DAG | Multi-dimensional Funnel |
| 6 | Architecture Health | Technical | Cyclomatic Complexity, Coupling Score | Dependency Matrix Heatmap | Polar Bar Chart |
| 7 | Performance & Stability | Technical | P95/P99 Latency, QPS/TPS | Time-Series Line Chart | Flame Graph |
| 8 | Security & Governance | Technical | Auth Interception Rate, Exposed API Rate | Threat Map | Nightingale Rose Chart |
| 9 | Resource & FinOps | Technical | Utilization Rate, Idle Cost | Treemap | Stacked Area Chart |

## Tab-to-Dimension Mapping

Maps the 7 assessment dimensions to the 9 dashboard tabs:

| Tab | Primary Dimensions | Secondary Dimensions |
|-----|-------------------|---------------------|
| 1 Executive Overview | ALL | - |
| 2 Business Completion | business | team |
| 3 Business Connection | architecture | tech-debt |
| 4 Business Connectivity | business | devops |
| 5 Business Logic Fluency | business | code-quality |
| 6 Architecture Health | architecture, code-quality | tech-debt |
| 7 Performance & Stability | devops | code-quality |
| 8 Security & Governance | security | architecture |
| 9 Resource & FinOps | devops, tech-debt | team |

## BFF Data Contract (Standard JSON Schema)

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "metricId": "string",
    "timestamp": 1718000000,
    "dimensions": ["dimension_key_1", "dimension_key_2"],
    "metrics": ["metric_key_1", "metric_key_2"],
    "dataset": [
      {"dimension_key_1": "value", "metric_key_1": 123}
    ]
  }
}
```

## Per-Tab Expert Review Card (Mandatory)

Every tab must include an expert review card with these fields:

| Field | Description |
|-------|-------------|
| Verdict | Pass / Concern / Fail |
| Gate Decision | Go / Conditional Go / No-Go |
| Risk Level | Low / Medium / High |
| SLA | Remediation timeline (P0/P1/P2) |
| Key Evidence | Top 3-5 issue references (file:line) |
