---
name: "arc:gate"
description: CI 门禁子模块，基于 arc:score 量化数据执行可配置的质量门禁判定
version: "1.0.0"
---

# arc:gate -- CI 质量门禁

## Overview

arc:gate 是 CI 门禁子模块，基于 arc:score 的量化数据执行可配置的质量门禁判定。支持 warn/strict/strict_dangerous 三种模式，提供阈值配置、豁免清单和 CI 集成能力。

核心能力：
- **门禁模式**：warn（仅告警）/ strict（阻断）/ strict_dangerous（额外拦截危险级别）
- **阈值配置**：总分阈值、严重问题阈值、安全阈值
- **豁免清单**：支持对特定问题进行时间有限的豁免
- **CI 集成**：提供正确的 exit code，支持 GitHub Actions / GitLab CI

## When to Use

- CI/CD 流水线需要质量门禁
- 项目需要可配置的阻断阈值
- 需要对特定问题进行临时豁免
- 代码合并前需要自动化质量检查

## Core Pattern

### 门禁模式

| 模式 | 行为 | Exit Code |
|------|------|-----------|
| `warn` | 只告警不阻断 | 总是 0 |
| `strict` | 发现未豁免 BREAKING 时失败 | 0 或 1 |
| `strict_dangerous` | 额外拦截危险级别 | 0 或 1 |

### 输入参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `project_path` | string | 是 | 项目根目录 |
| `score_dir` | string | 否 | arc:score 输出目录，默认 `.arc/score/<project>` |
| `config` | string | 否 | 门禁配置文件路径 |
| `mode` | string | 否 | 门禁模式：warn/strict/strict_dangerous，默认 strict |

### 目录结构

```
.arc/gate-reports/
├── gate-result.json       # 门禁结果 (pass/fail) + violations/whitelist
└── summary.md             # 执行摘要
```

---

## Phase 1: 加载配置

### Step 1.0: 共享上下文预检（强制）

在加载配置和评分前，先读取 `.arc/context-hub/index.json`：

1. 检索 `arc:score` 产物（`overall-score.json`、`smell-report.json`）
2. 验证 `expires_at`、文件路径存在性（若提供 `content_hash` 且为 sha256，则校验哈希一致性）
3. 通过校验则直接复用索引中的 score 产物路径
4. 若缺失/过期/哈希不一致，先回流触发 `arc:score` 更新，再执行门禁

### Step 1.1: 加载门禁配置

优先级：
1. 命令行指定 `--config` 参数
2. 项目根目录 `.arc/gate-config.yaml`
3. 默认配置（参考 `references/gate-config.yaml`）

### Step 1.2: 加载评分数据

从 arc:score 输出目录加载：
- `overall-score.json`：综合评分
- `smell-report.json`：Code Smell 检测结果

如果以上文件无法加载或已过期，不直接失败，先触发 `arc:score` 重新产出后重试一次。

---

## Phase 2: 执行门禁检查

### Step 2.1: 总分阈值检查

```python
# 默认阈值
thresholds:
  overall_score:
    warn: 70   # <70 分告警
    fail: 60   # <60 分阻断
```

### Step 2.2: 严重问题阈值检查

```python
thresholds:
  critical_issues:
    warn: 0    # 有 critical 即告警
    fail: 1    # >=1 个 critical 阻断
```

### Step 2.3: 高优先级问题阈值检查

```python
thresholds:
  high_issues:
    warn: 5    # >5 个 high 告警
    fail: 10   # >=10 个 high 阻断
```

### Step 2.4: 安全问题阈值检查

```python
thresholds:
  security_issues:
    warn: 0
    fail: 1    # >=1 个安全问题阻断
```

### Step 2.5: 豁免检查

检查违规项是否在豁免清单中：

```yaml
whitelist:
  - id: "legacy-auth-bypass"
    rule: "hardcoded_credential"
    file: "src/auth/config.py"
    reason: "历史遗留，计划 Q2 重构"
    approved_by: "tech-lead"
    expires_at: "2026-06-30T00:00:00Z"
```

---

## Phase 3: 生成报告

### Step 3.1: 门禁结果

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

### Step 3.2: CI 集成

提供正确的 exit code：
- `exit_code: 0`：门禁通过
- `exit_code: 1`：门禁失败（仅 strict/strict_dangerous 模式）

---

## 与其他 Skill 的协作

### 上游

- `arc:score`: 提供量化评分数据
- `arc:review`: 提供定性评审数据（可选）

### 下游

- CI/CD 系统：消费门禁结果决定是否继续流水线

### 共享索引约束

- `arc:gate` 必须优先通过 `.arc/context-hub/index.json` 发现 score 产物
- 若发现 score 数据过期，必须回流触发 `arc:score`，而非直接使用旧数据判定

### 调用示例

```bash
# 基本用法
claude-code /arc:gate --project /path/to/project

# strict 模式
claude-code /arc:gate --project /path/to/project --mode strict

# 使用自定义配置
claude-code /arc:gate --project /path/to/project --config gate-config.yaml

# CI 集成
claude-code /arc:gate --project . --mode strict --exit-code
```

---

## 配置示例

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
    reason: "历史遗留，计划 Q2 重构"
    approved_by: "tech-lead"
    expires_at: "2026-06-30T00:00:00Z"

# CI 集成配置
ci:
  # 是否在失败时输出详细报告
  verbose: true
  # 报告输出格式
  output_format: json
  # 是否生成 Markdown 摘要
  markdown_summary: true
```

---

## CI 集成示例

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
      
      - name: Setup Claude Code
        run: |
          # 安装 Claude Code CLI
          npm install -g @anthropic/claude-code
          
      - name: Run arc:score
        run: |
          claude-code /arc:score --project . --output .arc/score
          
      - name: Run arc:gate
        env:
          GATE_MODE: strict
          GATE_FAIL_ON_DANGEROUS: true
        run: |
          claude-code /arc:gate --project . --mode strict --exit-code
          
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
    - claude-code /arc:score --project . --output .arc/score
    - claude-code /arc:gate --project . --mode strict --exit-code
  artifacts:
    when: always
    paths:
      - .arc/gate-reports/
    expire_in: 1 week
  allow_failure: false
```

---

## 超时与降级

| 情况 | 处理 |
|------|------|
| arc:score 数据不存在 | 运行 arc:score 或报错 |
| 配置文件解析失败 | 使用默认配置并警告 |
| 豁免清单过期 | 忽略过期豁免 |

## 质量约束

1. **确定性**：相同输入总是产生相同输出
2. **可审计**：所有判定都有明确的规则和数据依据
3. **可配置**：阈值和豁免均可配置
4. **CI 友好**：提供正确的 exit code 和结构化输出
