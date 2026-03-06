# SPA Dashboard Engineering Specification (Optional 9-Tab Delivery Reference)

> This document is a reference specification for cases where `arc:audit` explicitly delivers an HTML dashboard.
> It does **not** define the default baseline contract of `arc:audit`; the default baseline remains the report, scorecard, recommendations, and evidence registry.
> It may be referenced by `integrate_score.py` or by the LLM when dashboard delivery is requested.

## Global Design Ontology & UX Standards

- **Semantic Color Palette**:
  - Success/Nominal: `#52C41A`
  - Warning/Degraded: `#FAAD14`
  - Critical/Danger: `#F5222D`
  - Deprecated/Archived/Offline: `#BFBFBF`
- **Volumetric Grid Matrix**: 24-column macroscopic grid system, modular card encapsulation, rigid 16px/24px spacing topology.
- **Dynamic Interaction Model**: Global temporal sandbox capabilities fused with environment state filters. Supports frictionless drill-down mechanics from macro-level observability down to micro-trace metrics.
- **High-Performance Visualization**: ECharts-driven DOM rendering, reinforced with hardware-accelerated WebGL pipelines capable of resolving 10,000+ node dependency topologies smoothly.

## 9-Tab Architectural Hierarchy

| Index | UI Tab | Domain Category | Cardinal KPIs/SLIs | Primary Visualization Matrix | Secondary Visualization Matrix |
|---|---|---|---|---|---|
| 1 | Executive Overview | Global Telemetry | Normalized Apdex, Aggregated 7-Dim Radar (0-100 scale) | Radar Chart | 3D Donut Gauges + Micro-Sparkline Cards |
| 2 | Business Completion | Value Engineering | Deployment Delivery Rate, Feature Burn-down Velocity | Sunburst Hierarchical Diagram | Interactive Timeline Gantt Chart |
| 3 | Business Connection | Topology Integration | Structural Node Degree (In/Out), Dependency Depth Mapping | Force-Directed Graph | 3D Perspective Architecture Render |
| 4 | Business Connectivity | Process Flow | E2E Interface Success SLA, Event Message Delivery Ratios | Funnel/Sankey Event Flow Diagram | Dense Hexbin Heatmap |
| 5 | Business Logic Fluency | Algorithmic Process | Conversion Efficacy, Fallback/Exception Trigger Rates | Process Mining Directed Acyclic Graph (DAG) | Multi-Dimensional Drop-Off Funnel |
| 6 | Architecture Health | Infrastructure Entropy | Cyclomatic Complexity Extremes, Macro-Coupling Coefficients | Dependency Matrix Correlation Heatmap | Polar Distribution Bar Chart |
| 7 | Performance & Stability | Reliability Metrics | P95/P99 Latency Tails, Max Sustainable QPS/TPS | High-Fidelity Time-Series Line Graph | Bottleneck Profiling Flame Graph |
| 8 | Security & Governance | Threat Posture | Unauthorized Transact Interception Rate, Exposed Critical API Ratios | Multi-Vector Threat Distribution Map | Florence Nightingale Rose Chart |
| 9 | Resource & FinOps | Economic Efficiency | Absolute Utilization vs. Idle Spin Rate, Cloud Waste Trajectory | Storage/Compute Treemap Matrix | Stacked Cost/Resource Area Chart |

## Dimensional Mapping Matrix

Algorithmically binds the 7 empirical assessment dimensions directly to the 9 dashboard DOM tabs:

| Dashboard Tab | Primary Dominant Dimension | Correlated Secondary Dimension |
|---|---|---|
| **1 Executive Overview** | **ALL** | N/A |
| **2 Business Completion** | business | team |
| **3 Business Connection** | architecture | tech-debt |
| **4 Business Connectivity** | business | devops |
| **5 Business Logic Fluency** | business | code-quality |
| **6 Architecture Health** | architecture, code-quality | tech-debt |
| **7 Performance & Stability** | devops | code-quality |
| **8 Security & Governance** | security | architecture |
| **9 Resource & FinOps** | devops, tech-debt | team |

## Backend-for-Frontend (BFF) Protocol Contract

Rigid schema enforcement for JSON payloads interfacing between the analytical engine and the SPA charting core:

```json
{
  "code": 200,
  "msg": "SYN_ACK_SUCCESS",
  "data": {
    "metricId": "uuid-v4-string",
    "timestamp": 1718000000,
    "dimensions": [
      "dimension_schema_key_1",
      "dimension_schema_key_2"
    ],
    "metrics": [
      "scalar_key_1",
      "scalar_key_2"
    ],
    "dataset": [
      {
        "dimension_schema_key_1": "nominal_state",
        "scalar_key_1": 123.45
      }
    ]
  }
}
```

## Immutable Expert Review Card Constraints

To guarantee accountability, **EVERY SINGLE TAB** must inject an immovable Expert Review Card block detailing qualitative, advisory outcomes.
These fields are dashboard summaries only and must not be interpreted as formal merge/release gate decisions; use `arc:gate` for explicit Go/No-Go decisions.

| Contract Field | Evaluation State Description |
|---|---|
| **Architectural Verdict** | Pass (Nominal) / Concern (Degraded) / Fail (Critical) |
| **Delivery Advisory** | Proceed Advisory / Conditional Advisory / Escalate to `arc:gate` for formal Go/No-Go |
| **Threat/Risk Posture** | Low Priority / Medium Priority / High Priority |
| **SLA Remediation Window** | Rigid remediation compliance timelines mapped to priority (P0: Immediate / P1: Next Sprint / P2: End of Quarter) |
| **Traceable Telemetry Evidence** | Top 3-5 structurally verifiable code issue references (Crucially formatted as `file:line`) |
