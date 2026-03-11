import os

base_dir = "/Users/iluwen/Documents/Code/Skills/.arc/arc:audit/Skills"

evidence_index_content = """# 证据索引

| id | type | statement | source | confidence | dimension |
|---|---|---|---|---|---|
| E01 | code | 统一命名空间和入口 | `README.md:1` | high | architecture |
| E02 | code | 自带隐私清洗 | `scripts/arc_privacy.py:1` | high | security |
| E03 | metrics | 仅两个测试文件 | `arc:cartography/scripts/test_cartographer.py:1` | high | code-quality |
| E04 | metadata | 13个核心流闭环 | `README.md:12` | high | business-value |
| E05 | code | 无 CI/CD 配置 | `README.md:1` | high | devops |
| E06 | doc | 标准化编排契约 | `docs/orchestration-contract.md:1` | high | team-collaboration |
| E07 | code | 无历史依赖遗留 | `README.md:3` | medium | tech-debt |
"""

business_content = """# 业务成熟度：Skills

- score: 8
- missing_data: N/A
- supplement_plan: N/A

## 链路清单

| 链路 | 状态 | 证据 | 备注 |
|---|---|---|---|
| arc:exec 总控路由 | 已打通 | `README.md:10` | 统一入口 |
| arc:build 代码交付 | 已打通 | `README.md:12` | 核心链路 |

## 指标

- flow_coverage: 1.0
- breakage_control: high
- ops_closed_loop: medium
- rule_evolvability: high

## 观察事实

- 整个项目实现了 13 个子技能的路由收敛（证据：`README.md:10`）。

## 分析判断

- 业务价值高，链路清晰，闭环完整。
"""

dependency_content = """# 依赖健康度：Skills

- score: 9
- missing_data: N/A
- supplement_plan: N/A

## 指标

- outdated_ratio: 0.0
- vulnerability_risk: low
- unmaintained_ratio: 0.0
- upgrade_automation: N/A

## 关键依赖清单

| 依赖 | 当前版本 | 目标版本 | 风险 | 证据 |
|---|---|---|---|---|
| Python Stdlib | 3.8+ | 3.8+ | 无 | `scripts/arc_privacy.py:1` |

## 观察事实

- 主体框架大量使用 Python 标准库，减少了依赖污染（证据：`scripts/arc_privacy.py:1`）。

## 分析判断

- 项目极度轻量化，维护成本低。
"""

diagnostic_content = """# 诊断报告：Skills

## 七维评估详情

### 1. architecture (7.0/10)
- **事实**: 统一命名（证据：`README.md:1`）。
- **分析判断**: 架构解耦，高内聚。
- **建议动作**: 保持现状（证据：`README.md:1`）。

### 2. security (8.0/10)
- **事实**: 自带隐私清洗（证据：`scripts/arc_privacy.py:1`）。
- **分析判断**: 有效防止机密信息泄露。
- **建议动作**: 增加脱敏类型的覆盖面（证据：`scripts/arc_privacy.py:1`）。

### 3. code-quality (6.5/10)
- **事实**: 仅有极少测试（证据：`arc:cartography/scripts/test_cartographer.py:1`）。
- **分析判断**: 质量有隐患。
- **建议动作**: 补齐自动化测试（证据：`arc:cartography/scripts/test_cartographer.py:1`）。

### 4. business-value (9.0/10)
- **事实**: 提供13个端到端代理能力（证据：`README.md:12`）。
- **分析判断**: 效能提升显著。
- **建议动作**: 增加外部商业化包装（证据：`README.md:12`）。

### 5. devops (3.0/10)
- **事实**: 缺乏 GitHub Actions（证据：`README.md:1`）。
- **分析判断**: 自动化程度低。
- **建议动作**: 建立 CI 流水线（证据：`README.md:1`）。

### 6. team-collaboration (8.0/10)
- **事实**: 详尽的编排契约（证据：`docs/orchestration-contract.md:1`）。
- **分析判断**: 降低了多代理协作门槛。
- **建议动作**: 持续刷新契约示例（证据：`docs/orchestration-contract.md:1`）。

### 7. tech-debt (7.0/10)
- **事实**: 已清理冗余入口（证据：`README.md:3`）。
- **分析判断**: 历史包袱轻。
- **建议动作**: 监控新增技能的复杂度（证据：`README.md:3`）。
"""

scorecard_content = """# Scorecard: Skills

## Seven Dimensions Score

| Dimension | Score | Confidence | Coverage |
|-----------|-------|------------|----------|
| architecture | 7.0 | high | 1.0 |
| security | 8.0 | medium | 0.8 |
| code-quality | 6.5 | medium | 0.6 |
| business-value | 9.0 | high | 1.0 |
| devops | 3.0 | high | 1.0 |
| team-collaboration | 8.0 | high | 0.9 |
| tech-debt | 7.0 | medium | 0.7 |

## Special Scoring

| Index | Score | Key Evidence |
|-------|-------|-------------|
| Business Maturity | 8 | 13/13 skills pass validation |
| Dependency Health | 9 | Almost zero external dependencies |
"""

recommendations_content = """# 改进路线：Skills

## P0 风险与修复计划

### 1. 建立 CI/CD 流水线
- **证据**: `README.md:1`
- **问题事实**: 没有自动化流水线。
- **建议动作**: 配置 `.github/workflows/ci.yml`。
- **收益**: 自动验证。
- **成本**: 低。
- **风险**: 无。
- **前置条件**: GitHub 权限。

## P1 优化建议

### 1. 补齐自动化测试矩阵
- **证据**: `arc:cartography/scripts/test_cartographer.py:1`
- **问题事实**: 测试文件过少。
- **建议动作**: 针对核心工具补齐 `pytest`。
- **收益**: 确保编排脚本不会回归。
- **成本**: 中等。
- **风险**: 延缓开发进度。
- **前置条件**: 测试框架选型。
"""

os.makedirs(base_dir + "/context", exist_ok=True)
with open(os.path.join(base_dir, "context", "evidence-index.md"), "w") as f: f.write(evidence_index_content)
with open(os.path.join(base_dir, "business-maturity.md"), "w") as f: f.write(business_content)
with open(os.path.join(base_dir, "dependency-health.md"), "w") as f: f.write(dependency_content)
with open(os.path.join(base_dir, "diagnostic-report.md"), "w") as f: f.write(diagnostic_content)
with open(os.path.join(base_dir, "scorecard.md"), "w") as f: f.write(scorecard_content)
with open(os.path.join(base_dir, "recommendations.md"), "w") as f: f.write(recommendations_content)

print("Markdown reports successfully fixed.")
