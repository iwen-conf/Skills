---
name: "arc:release"
description: "发布门禁：执行质量阈值检查并给出 Go/No-Go。"
---

# arc:release -- CI quality gate control

## Overview

arc:release is a unified access control module that consumes the quantitative data produced by the `score/` module to perform configurable quality judgments and completes release readiness checks (rollback, monitoring, duty and announcements) before going online. Supports three modes: warn/strict/strict_dangerous, and provides threshold configuration, exemption list and CI integration capabilities.

Core competencies:
- **Access Control Mode**: warn (warning only)/strict (blocking)/strict_dangerous (additional interception dangerous level)
- **Threshold configuration**: total score threshold, serious problem threshold, safety threshold
- **Waiver List**: Supports time-limited waivers for specific issues
- **Release readiness**: four types of online inspection items: rollback, monitoring, duty, and announcement
- **CI integration**: Provide correct exit code, support GitHub Actions / GitLab CI

## Quick Contract

- **Trigger**: Automatic access control and Go/No-Go determination are required before merging or going online.
- **Inputs**: `project_path`, score product path, access control configuration, mode and exemption list.
- **Outputs**: `gate-result.json`, `summary.md`, explicit exit code available for CI.
- **Quality Gate**: Must first pass threshold evidence and exemption validity checks of `## Quality Gates`.
- **Decision Tree**: For the input signal routing diagram, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#signal-to-skill-decision-tree).

## Routing Matrix

- For unified routing comparison, see [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md).
- A phased getting started view is available at [`docs/arc-routing-matrix.md`](../docs/arc-routing-matrix.md#phase-routing-view).
- For a quick cheat sheet, see [`docs/arc-routing-cheatsheet.md`](../docs/arc-routing-cheatsheet.md).
- If there is a conflict, the **边界提示** of this skill `## When to Use` shall prevail.

## Announce

Begin by stating clearly:
"I am using `arc:release`, first load the score and strategy, and then output the access control decision."

## The Iron Law

```
NO GREEN BUILD WITHOUT EXPLICIT GATE DECISION
```

Without clear judgment and basis for access control, no passage shall be marked.

## Workflow

1. Read the shared index and score products, triggering reflow updates if necessary.
2. Load the access control configuration and perform threshold checks.
3. Apply exemption rules and verify validity and approval information.
4. Output `gate-result.json`, summary, and CI exit code.

## Quality Gates

- The basis for determination must be traceable to specific thresholds and evidence documentation.
- Exemptions must have `reason`, `approved_by`, `expires_at`.
- Missing or expired scoring data must be refreshed before judging.
- The resulting output must contain both pass/fail and violation details.

## Red Flags

- Only look at the overall score and ignore high-risk safety issues.
- Expired exemptions are still considered valid.
- The score product expires but the old result continues to be used.
- Ungated digests at the CI stage result in non-auditability.

## When to Use

- **首选触发**: Automated pass/fail and Go/No-Go determination is required before merging or going online.
- **典型场景**: Execution blocking based on score threshold and exemption; unified check of rollback/monitoring/duty/announcement before release window.
- **边界提示**: For root cause location and systematic improvement, first use `arc:audit`, `arc:release` to focus on blocking judgment and release suggestions.

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
4. If missing/expired/hash is inconsistent, first reflow triggers `score/` module refresh (triggered by `arc:release` orchestration), and then executes access control

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

- `arc:release` must first discover the score product through `.arc/context-hub/index.json`
- If it is found that the score data is expired, it must be reflowed to trigger a refresh of the scoring stage instead of directly using the old data for judgment.

### Call example

```bash
# Basic usage
arc release --project /path/to/project

# strict mode
arc release --project /path/to/project --mode strict

# Use custom configuration
arc release --project /path/to/project --config gate-config.yaml

# CI integration
arc release --project . --mode strict --exit-code
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
          
      - name: Run arc:release
        env:
          GATE_MODE: strict
          GATE_FAIL_ON_DANGEROUS: true
        run: |
          arc release --project . --mode strict --exit-code
          
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
    - arc release --project . --mode strict --exit-code
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
| score product does not exist | First generate the score product and then run arc:release |
| Configuration file parsing failed | Use default configuration with warning |
| Exemption list expired | Ignore expired exemptions |

## quality constraints

1. **Determinism**: The same input always produces the same output
2. **Auditable**: All decisions have clear rules and data basis
3. **Configurable**: Both thresholds and exemptions are configurable
4. **CI Friendly**: Provides correct exit code and structured output
