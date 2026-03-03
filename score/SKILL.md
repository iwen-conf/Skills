---
name: "arc:score"
description: 量化评分与 Code Smell 检测，为 arc:review 和 arc:gate 提供量化数据支撑
version: "1.0.0"
---

# arc:score -- 量化评分与 Code Smell 检测

## Overview

arc:score 是量化评分子模块，通过静态分析检测 Code Smell、分析 Bugfix 历史分级、检查架构违规，输出量化评分数据供 arc:review 和 arc:gate 消费。

核心能力：
- **Code Smell 检测**：6 大类 19 项检测规则（重复代码、噪音代码、过度防御、错误处理、安全模式）
- **Bugfix 分级**：基于改动行数自动分级 (A/B/C) + 自动打标
- **架构检查**：模块依赖违规检测
- **评分聚合**：量化分数 (0-100) + 评级 (A-F)

## When to Use

- 项目质量评估需要量化数据支撑
- CI 门禁需要客观的阻断依据
- 代码审查需要预先扫描问题清单
- 技术债务需要量化追踪

## Core Pattern

### 输入参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `project_path` | string | 是 | 待评分项目根目录 |
| `output_dir` | string | 否 | 输出目录，默认 `.arc/score/<project-name>` |
| `focus` | array | 否 | 检测焦点：`smell`/`bugfix`/`architecture` |
| `severity_threshold` | string | 否 | 最低严重程度：`low`/`medium`/`high`/`critical` |
| `languages` | array | 否 | 目标语言，默认自动检测 |

### 目录结构

```
.arc/score/<project-name>/
├── context/
│   └── project-snapshot.md       # 项目快照
├── analysis/
│   ├── smell-report.json         # Code Smell 检测结果
│   ├── smell-report.md           # Markdown 格式报告
│   ├── bugfix-grades.json        # Bugfix 分级结果
│   └── architecture-check.json   # 架构检查结果
├── score/
│   ├── overall-score.json        # 综合评分
│   ├── dimension-scores.json     # 各维度评分
│   └── scorecard.md              # 评分卡（可视化）
└── handoff/
    └── review-input.json         # 供 arc:review 消费
```

## Phase 1: 项目扫描

### Step 1.1: 创建工作目录

```bash
python scripts/scaffold_score_case.py --project <project_path> --output <output_dir>
```

产出：
- `.arc/score/<project-name>/` 目录结构
- `context/project-snapshot.md` 项目快照

### Step 1.2: 语言检测

使用 ace-tool MCP 检测项目主要语言：

```
Ace-tool_search_context(
  project_root_path="<project_path>",
  query="项目主要编程语言和技术栈"
)
```

支持的语言：
- Python (`.py`)
- TypeScript (`.ts`, `.tsx`)
- JavaScript (`.js`, `.jsx`)
- Go (`.go`)
- Rust (`.rs`)
- Java (`.java`)

---

## Phase 2: Code Smell 检测

### Step 2.1: 执行检测

```bash
python scripts/detect_smell.py \
  --project <project_path> \
  --output <output_dir>/analysis/smell-report.json \
  --languages python,typescript \
  --severity-threshold low
```

### Step 2.2: 检测规则

参考 `references/smell-rules.yaml`，共 6 大类 19 项：

| 类别 | 规则数 | 典型问题 |
|------|--------|---------|
| **重复代码** | 2 | 结构重复、骨架重复 |
| **噪音代码** | 7 | 空函数体、注释掉的代码、死分支 |
| **过度防御** | 3 | 残留样板、冗余类型检查 |
| **错误处理** | 2 | 吞没异常、过宽捕获 |
| **安全模式** | 5 | 硬编码凭证、注入风险、弱加密 |

### Step 2.3: 输出格式

```json
{
  "summary": {
    "total_violations": 42,
    "by_severity": {
      "critical": 2,
      "high": 5,
      "medium": 15,
      "low": 20
    },
    "by_category": {
      "duplication": 3,
      "noise": 12,
      "defensive": 8,
      "error_handling": 7,
      "security": 12
    }
  },
  "violations": [
    {
      "id": "hardcoded_credential",
      "category": "security",
      "severity": "critical",
      "file": "src/auth/config.py",
      "line": 42,
      "message": "硬编码凭证：源码中直接写死 API_KEY",
      "suggestion": "使用环境变量或密钥管理服务"
    }
  ]
}
```

## Phase 3: Bugfix 分级

### Step 3.1: 执行分级

```bash
python scripts/grade_bugfix.py \
  --project <project_path> \
  --output <output_dir>/analysis/bugfix-grades.json \
  --branch main \
  --limit 100
```

### Step 3.2: 分级规则

| 等级 | 改动行数 | 特征 | 优先级 |
|------|---------|------|--------|
| **A** | ≤10 行 | 小修复，高价值回归测试 | 高 |
| **B** | ≤50 行 | 中等修复 | 中 |
| **C** | >50 行 | 大修复，复杂度高 | 低 |

### Step 3.3: 自动打标

基于 commit message 关键词自动打标：

```yaml
- keywords: ["xss", "漏洞", "注入", "安全", "越权"]
  tag: security
- keywords: ["性能", "慢", "超时", "timeout"]
  tag: performance
- keywords: ["并发", "锁", "重复", "concurrent"]
  tag: concurrency
```

## Phase 4: 评分聚合

### Step 4.1: 执行聚合

```bash
python scripts/aggregate_score.py \
  --smell-report <output_dir>/analysis/smell-report.json \
  --bugfix-report <output_dir>/analysis/bugfix-grades.json \
  --output <output_dir>/score/overall-score.json \
  --scorecard <output_dir>/score/scorecard.md \
  --dimension-scores <output_dir>/score/dimension-scores.json
```

### Step 4.2: 评分算法

```python
SEVERITY_PENALTY = {
    "critical": 20,
    "high": 10,
    "medium": 5,
    "low": 2
}

# 基础分 100，按违规扣分
final_score = max(0, 100 - total_penalty)
```

### Step 4.3: 评级映射

| 分数 | 评级 | 含义 |
|------|------|------|
| 90-100 | A | 优秀 |
| 80-89 | B | 良好 |
| 70-79 | C | 合格 |
| 60-69 | D | 需改进 |
| 0-59 | F | 不合格 |

---

## Phase 5: 生成报告

### Step 5.1: 评分卡

评分卡由 `scripts/aggregate_score.py` 在聚合阶段通过 `--scorecard` 输出。

### Step 5.2: 交接产物

生成 `handoff/review-input.json` 供 arc:review 消费：

```bash
python scripts/generate_review_handoff.py \
  --score-dir <output_dir> \
  --output <output_dir>/handoff/review-input.json
```

```json
{
  "schema_version": "1.0.0",
  "producer_skill": "arc:score",
  "generated_at": "2026-03-03T00:00:00Z",
  "project_path": "/path/to/project",
  "artifacts": {
    "overall_score": {"path": "score/overall-score.json", "sha256": "..."},
    "smell_report": {"path": "analysis/smell-report.json", "sha256": "..."},
    "bugfix_grades": {"path": "analysis/bugfix-grades.json", "sha256": "..."}
  },
  "overall": {"score": 75, "weighted_score": 72.5, "grade": "C", "dimension_scores": {...}},
  "smell_summary": {...},
  "bugfix_summary": {...},
  "top_violations": [...]
}
```

可选：校验 score 产物契约（CI 友好，非 0 退出码表示失败）：

```bash
python scripts/validate_score_artifacts.py \
  --score-dir <output_dir> \
  --require-bugfix \
  --require-review-handoff
```

### Step 5.3: 发布共享上下文元数据（强制）

将本次评分产物写入 `.arc/context-hub/index.json`：

- `score/overall-score.json`
- `analysis/smell-report.json`
- `analysis/bugfix-grades.json`
- `handoff/review-input.json`

每条记录必须包含：`path`、`content_hash`、`generated_at`、`ttl_seconds`、`expires_at`、`producer_skill=arc:score`、`refresh_skill=arc:score`。

---

## 与其他 Skill 的协作

### 上游

- `arc:init`: 项目索引，提供 CLAUDE.md 层级结构

### 下游

- `arc:review`: 消费 `handoff/review-input.json` 作为量化数据输入
- `arc:gate`: 消费评分结果执行 CI 门禁判定

### 共享索引约束

- 下游 skill 应优先通过 `.arc/context-hub/index.json` 发现并加载 score 产物
- 若发现 score 产物过期/缺失/哈希不一致，必须回流触发 `arc:score` 更新

### 调用示例

```
# 完整流程
arc:init → arc:score → arc:review → arc:gate

# 单独运行
/arc:score --project /path/to/project

# 指定检测焦点
/arc:score --project /path/to/project --focus security
```

---

## 超时与降级

| 情况 | 处理 |
|------|------|
| AST 解析失败 | 跳过该文件，记录警告 |
| 项目过大 (>10000 文件) | 增量扫描，使用缓存 |
| 不支持的语言 | 跳过 Code Smell 检测，仅运行 Bugfix 分级 |

## 质量约束

1. **本地运行**：无需外部服务依赖
2. **增量支持**：支持增量扫描，只检测变更文件
3. **可配置**：规则、阈值、权重均可配置
4. **双格式输出**：JSON (可消费) + Markdown (可读)
