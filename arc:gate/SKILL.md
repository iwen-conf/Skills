---
name: "arc:gate"
description: "合并/上线门禁决策：基于阈值、豁免与证据给出 Go/No-Go；当用户说“是否可合并/发布门禁/release gate”时触发。"
---

# arc:gate -- merge/release gate decision

## Overview

`arc:gate` focuses on quality gate decisions before merge/release. It consumes `score/` outputs, applies threshold and waiver policies, and returns explicit Go/No-Go with CI-ready exit code. It does not execute deployment actions.

Core competencies:
- **Gate mode**: `warn` / `strict` / `strict_dangerous`
- **Policy checks**: overall score, critical/high issues, security signals
- **Waiver policy**: rule-level temporary exemptions with approver and expiry
- **Release readiness**: rollback, monitoring, oncall, announcement checks
- **CI integration**: deterministic exit code for pipelines

## Quick Contract

- **Trigger**: Merge/release requires explicit Go/No-Go gate judgment.
- **Inputs**: `project_path`, score artifact path, gate config, mode, waiver list.
- **Outputs**: `gate-result.json`, `summary.md`, CI exit code.
- **Quality Gate**: Enforce threshold evidence and waiver validity in `## Quality Gates`.
- **Decision Tree**: See [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:gate`, loading score artifacts and policies, then outputting the Go/No-Go decision."

## The Iron Law

```
NO GREEN BUILD WITHOUT EXPLICIT GATE DECISION
```

No build/release is green without explicit gate evidence.

## Workflow

1. Read shared index and load score artifacts (refresh when stale).
2. Load gate config and run threshold checks.
3. Apply waiver rules and validate approval/expiry.
4. Output decision artifacts and CI exit code.

## Quality Gates

- Decision basis must map to explicit thresholds and evidence.
- Waivers must include `reason`, `approved_by`, `expires_at`.
- Missing/expired score artifacts must be refreshed before judging.
- Output must include pass/fail decision plus violation details.

## Expert Standards

- 门禁规则建议 `Policy-as-Code` 化（可映射 OPA/Rego 等规则引擎语义）。
- 安全门禁需覆盖 `OWASP` 高风险项与已知漏洞阈值，不得被总分掩盖。
- 供应链门禁需校验 `SBOM + 来源可追溯`，避免未知依赖进入发布链路。
- 例外项必须满足 `审批 + 到期 + 复盘` 三要素，超期自动失效。
- 结论应同时支持 `人审` 与 `机审`：结构化结果、证据链接、退出码一致。

## Scripts & Commands

- 脚本门禁（严格）：`python3 arc:gate/scripts/check_gate.py --project <project_path> --mode strict --exit-code`
- 脚本门禁（危险拦截）：`python3 arc:gate/scripts/check_gate.py --project <project_path> --mode strict_dangerous --exit-code`
- 自定义配置：`python3 arc:gate/scripts/check_gate.py --project <project_path> --config <gate-config.yaml> --output-dir <output_dir>`
- 运行时主命令：`arc gate`

## Red Flags

- Only checking total score while ignoring security-critical signals.
- Treating expired waivers as valid.
- Reusing stale score artifacts without refresh.
- Producing CI summary without explicit gate decision.

## When to Use

- **首选触发**: 合并或上线前需要明确的门禁判定（Go/No-Go）。
- **典型场景**: 依据评分阈值与豁免策略做阻断；在发布窗口统一校验回滚/监控/值班/公告准备度。
- **边界提示**: `arc:gate` 只负责门禁判定，不负责部署执行；根因分析与改进路线请先用 `arc:audit`。

## Dependencies

- **Organization Contract**: Recommended. Follow `docs/orchestration-contract.md` for cross-runtime call adaptation.
- **score product**: required. Depends on `overall-score.json` and `smell-report.json`.
- **Git (optional)**: Used to supplement change context in CI.

## Core Pattern

### Access control mode

| model | Behavior | Exit Code |
|------|------|-----------|
| `warn` | Only alert but not block | always 0 |
| `strict` | Failed when BREAKING was found not exempted | 0 or 1 |
| `strict_dangerous` | Extra interception risk level | 0 or 1 |

### input parameters

| parameter | type | Required | illustrate |
|------|------|------|------|
| `project_path` | string | yes | Project root directory |
| `score_dir` | string | no | score product directory, default `.arc/score/<project>` |
| `config` | string | no | Access control configuration file path |
| `mode` | string | no | Access control mode: warn/strict/strict_dangerous, default strict |

### Directory structure

```
.arc/gate-reports/
├── gate-result.json # Gate control result (pass/fail) + violations/whitelist
└── summary.md # Executive summary
```

---

## Phase 1: Load configuration

### Step 1.0: Shared context preflight (mandatory)

Before loading configuration and scoring, read `.arc/context-hub/index.json`:

1. Retrieve score products (`overall-score.json`, `smell-report.json`)
2. Verify the existence of `expires_at` and file path (if `content_hash` is provided and is sha256, verify hash consistency)
3. Through verification, the score product path in the index is directly reused.
4. If missing/expired/hash is inconsistent, first reflow triggers `score/` module refresh (triggered by `arc:gate` orchestration), and then executes access control

### Step 1.1: Load access control configuration

Priority:
1. Specify the `--config` parameter on the command line
2. Project root directory `.arc/gate-config.yaml`
3. Default configuration (reference `references/gate-config.yaml`)

### Step 1.2: Load rating data

Load from the score product directory:
- `overall-score.json`: Comprehensive score
- `smell-report.json`: Code Smell detection results

If the above file cannot be loaded or has expired, instead of failing directly, trigger the scoring stage to re-produce and then try again.

---

## Phase 2: Perform access control checks

### Step 2.1: Total score threshold check

```python
# Default threshold
thresholds:
  overall_score:
warn: 70 # <70 points warning
fail: 60 # <60 points block
```

### Step 2.2: Severe problem threshold check

```python
thresholds:
  critical_issues:
warn: 0 # If there is critical, it means a warning
fail: 1 # >=1 critical block
```

### Step 2.3: Threshold check for high priority issues

```python
thresholds:
  high_issues:
warn: 5 # >5 high warnings
fail: 10 # >=10 high blocks
```

### Step 2.4: Security issue threshold check

```python
thresholds:
  security_issues:
    warn: 0
fail: 1 # >=1 security question blocked
```

### Step 2.5: Exemption from inspection

Check if the offending item is in the exemption list:

```yaml
whitelist:
  - id: "legacy-auth-bypass"
    rule: "hardcoded_credential"
    file: "src/auth/config.py"
reason: "Historical legacy, planned Q2 reconstruction"
    approved_by: "tech-lead"
    expires_at: "2026-06-30T00:00:00Z"
```

---

## Phase 3: Generate report

### Step 3.1: Access control results

```json
{
  "status": "fail",
  "mode": "strict",
  "overall_score": 55,
  "violations": [
    {
      "rule": "overall_score_threshold",
      "severity": "fail",
      "actual": 55,
      "threshold": 60,
      "whitelisted": false
    }
  ],
  "whitelist_applied": 0,
  "exit_code": 1
}
```

### Step 3.2: CI integration

Provide the correct exit code:
- `exit_code: 0`: Access control passed
- `exit_code: 1`: Access control failed (strict/strict_dangerous mode only)

---

## Collaboration with other Skills

### upstream

- `built-in score stage`: provide quantitative scoring data
- `arc:audit`: Provide qualitative review data (optional)

### downstream

- CI/CD system: consumption access control results determine whether to continue the pipeline

### Shared index constraints

- `arc:gate` must first discover the score product through `.arc/context-hub/index.json`
- If it is found that the score data is expired, it must be reflowed to trigger a refresh of the scoring stage instead of directly using the old data for judgment.

### Call example

```bash
# Basic usage
arc gate --project /path/to/project

# strict mode
arc gate --project /path/to/project --mode strict

# Use custom configuration
arc gate --project /path/to/project --config gate-config.yaml

# CI integration
arc gate --project . --mode strict --exit-code
```

---

## Configuration example

```yaml
# gate-config.yaml

mode: strict

thresholds:
  overall_score:
    warn: 70
    fail: 60
    
  critical_issues:
    warn: 0
    fail: 1

  high_issues:
    warn: 5
    fail: 10
     
  security_issues:
    warn: 0
    fail: 1

whitelist:
  - id: "legacy-auth-bypass"
    rule: "hardcoded_credential"
    file: "src/auth/config.py"
reason: "Historical legacy, planned Q2 reconstruction"
    approved_by: "tech-lead"
    expires_at: "2026-06-30T00:00:00Z"

# CI integration configuration
ci:
  # Whether to output a detailed report on failure
  verbose: true
  # Report output format
  output_format: json
  # Whether to generate Markdown summary
  markdown_summary: true
```

---

## CI integration example

### GitHub Actions

```yaml
# .github/workflows/gate.yml

name: Quality Gate

on:
  push:
    branches: [main, develop]
  pull_request:

jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup ARC Adapter
        run: |
          # Install the runtime adaptation layer you use (example placeholder)
          echo "install your arc adapter here"
          
      - name: Run arc:gate
        env:
          GATE_MODE: strict
          GATE_FAIL_ON_DANGEROUS: true
        run: |
          arc gate --project . --mode strict --exit-code
          
      - name: Upload Reports
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: gate-reports
          path: .arc/gate-reports/
```

### GitLab CI

```yaml
# .gitlab-ci.yml

quality_gate:
  stage: test
  script:
    - arc gate --project . --mode strict --exit-code
  artifacts:
    when: always
    paths:
      - .arc/gate-reports/
    expire_in: 1 week
  allow_failure: false
```

---

## Timeouts and downgrades

| Condition | deal with |
|------|------|
| score product does not exist | First generate the score product and then run arc:gate |
| Configuration file parsing failed | Use default configuration with warning |
| Exemption list expired | Ignore expired exemptions |

## quality constraints

1. **Determinism**: The same input always produces the same output
2. **Auditable**: All decisions have clear rules and data basis
3. **Configurable**: Both thresholds and exemptions are configurable
4. **CI Friendly**: Provides correct exit code and structured output
