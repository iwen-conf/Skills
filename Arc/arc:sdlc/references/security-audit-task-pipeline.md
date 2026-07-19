# Security / Audit → Task Pipeline

Narrow linkage between `arc:audit` (appsec), `arc:security`, and `arc:sdlc`.

**Goal:** audit and scan produce a fixed handoff; the task skill turns that handoff into **very detailed** local tasks—not vague “修安全问题”.

Do **not** invent a new lifecycle skill. Do **not** expand into full red-team ops. Stay on: findings → constraints → subtasks → fix/verify.

## 1) Functional Roles (功能角色)

Exactly five roles. One human/agent may wear multiple hats, but **documents must name the active role**.

| Role ID | Name | Owns | Must produce | Must NOT do |
|---|---|---|---|---|
| `R-recon` | 侦察审计员 | `arc:audit` mode `appsec` | `assets`, `data-map`, finding cards, manual gaps | code edits; active DAST |
| `R-scan` | 扫描操作员 | `arc:security` | tool reports under `.arc/security/`, re-ranked priority list | claim AuthZ coverage from scanners alone |
| `R-task` | 任务作者 | `arc:sdlc` | `00-前置约束.md` (含定位/口径/角色), 任务组, 过细子任务, 进度表 | implement production code in this role |
| `R-fix` | 修复实施员 | `arc:fix` / `arc:build` | code + tests per **one** subtask | rewrite whole plan mid-fix without updating task docs |
| `R-verify` | 验证员 | fix/build + optional re-scan | verification notes in 进度跟踪表 | mark `[x]` without evidence |

### Role handoff rules

1. `R-recon` / `R-scan` finish → write **Handoff Package** (section 4) → only then `R-task`.
2. `R-task` finishes gate → only then `R-fix` starts first `[/]` subtask.
3. Multi-finding work: one task directory, multiple `TNN` groups by **permission class + data domain**, not by tool name.
4. If handoff lacks 项目定位 / 项目口径 / finding cards, `R-task` must pause (`[!]` blocker) or run a minimal discovery pass—not invent files.

## 2) Project Positioning (项目定位)

Fill from the **target repo** (README, AGENTS.md, PRD, architecture). Required in security-originated `00-前置约束.md`.

```markdown
## 项目定位

1. 产品形态：...（例：小说阅读 + 作家后台 + 管理端 多端应用）
2. 用户角色：...（例：读者 / 作者 / 管理员 / 支付回调服务）
3. 核心价值与不可丢数据：...（例：账号、订单/辰豆、章节正文、支付一致性）
4. 信任边界：...（例：公网 API、SSR Cookie、对象存储、Identity 网关、支付网关）
5. 本次审计/修复在产品中的位置：...（例：上线前 AppSec 修复战役 / 单点漏洞修复）
```

Rules:

- Positioning is **product truth**, not generic “web app”.
- If unknown, mark `待确认` and block P0 security tasks that depend on it.
- Downstream subtasks must restate the affected **user role** (读者/作者/管理员/匿名).

## 3) Project Caliber (项目口径)

Caliber = hard engineering constraints the task writer and fixer must obey. Collect from repo agent instructions and architecture conventions—**do not paste another project’s caliber**.

```markdown
## 项目口径

1. 部署形态：...（例：单节点；非分布式默认）
2. 数据命名/存储：...（例：小说侧表 `n_*`；PG + Mongo）
3. 保护区：...（例：`reference_projects/` 只读；`front/apple/` 默认不动）
4. 环境与部署顺序：...（例：先 `.26` 测试再 `.31`）
5. 架构约束：...（例：usecase 边界、禁止顺手大重构）
6. 安全测试边界：...（例：只读审计 / 仅授权测试服 DAST / 禁止生产主动攻击）
7. 密钥与证据：...（例：聊天脱敏；原始证据落本地）
8. 明确不做：...（例：不改 iOS、不引入 Redis Stream）
```

Rules:

- Caliber turns into **subtask “不要做”** lines automatically.
- Any subtask that would violate caliber is invalid until rewritten or caliber updated with user approval.
- Prefer citing file paths: `Agents.md`, `arc:project-architecture-conventions`, local deploy scripts.

## 4) Handoff Package (审计/扫描 → 任务作者)

`R-recon` and/or `R-scan` must leave a package the task skill can consume without re-auditing the world.

Minimum paths (project-local; adjust root):

```text
docs/…/security-handoff/   # or .arc/security/<ts>/handoff/
  00-项目定位与口径.md      # positioning + caliber (+ role matrix for this engagement)
  01-assets.md
  02-data-map.md
  03-findings.md            # finding cards
  04-manual-gaps.md
  05-re-ranked.md           # optional; from arc:security
  06-task-seed.md           # suggested T-groups only; not full subtasks
```

### Finding card fields required for tasking

Every finding used to create a subtask must have:

| Field | Task skill uses it for |
|---|---|
| `finding_id` | subtask title prefix / link |
| `sev` + data-value priority | 优先级 P0–P3 |
| `permission_class` | task group split + regression role |
| `data_yield` | acceptance “what must no longer leak” |
| `evidence` paths/symbols | 输入 / 影响文件 |
| `exploit_conditions` | 验证前置 |
| `fix_direction` | 执行清单骨架 |
| `status` | confirmed → implement; likely → verify-first subtask; assumption → research subtask |

### `06-task-seed.md` format (narrow)

```markdown
# Task Seed

## 建议任务组（仅种子，不是最终子任务）

| 组 | 名称 | 依据 finding_ids | 建议优先级 |
|---|---|---|---|
| T01 | 配置与密钥收敛 | F-001, F-003 | P0 |
| T02 | 鉴权与 IDOR | F-002, F-005 | P0 |
| T03 | 支付与积分完整性 | F-004 | P1 |

## 不做

1. ...
```

`R-task` expands seeds into full subtasks; it must not treat seeds as complete.

## 5) How `R-task` Writes Docs (非常详细)

### 5.1 Task directory naming

Prefer one domain entry:

```text
NN-安全修复-<短主题>/   # or NN-AppSec审计跟进-<短主题>/
  README.md
  00-前置约束.md
  进度跟踪表.md
  tasks/
    T01-.../
    T02-.../
```

### 5.2 `00-前置约束.md` must include (security source)

In addition to the skill’s base pre-constraint sections:

1. `## 项目定位` (section 2)
2. `## 项目口径` (section 3)
3. `## 功能角色` — who is R-recon/R-scan/R-task/R-fix/R-verify for this engagement
4. `## 上游 Handoff` — relative links to package files + scan timestamp if any
5. `## Finding 索引` — table: id, sev, priority, title, target subtask id (or `待拆`)
6. `## 修复战役边界` — which findings in scope this round; which deferred `[-]`

### 5.3 Task group split rules

Split groups by **risk domain**, not by scanner:

| Prefer T-group by | Example |
|---|---|
| Config/secrets/ops | committed secrets, pprof, default admin |
| AuthN/session | login, OTP, reset password |
| AuthZ/IDOR | object ownership |
| Payment/credits | callback, amount, points |
| Upload/storage | path traversal, signed URL |
| Injection/data query | SQL/NoSQL on high-yield tables |
| Dependency/supply-chain | reachable CVE only |
| Hardening | headers, cookie flags (usually P2+) |

### 5.4 One finding → one or more subtasks (detail contract)

Default: **one confirmed finding → one fix subtask** (+ optional verify subtask).

If a finding needs research first (`likely` / `assumption`):

1. `Txx-01-复核-<finding_id>` (read-only proof or false-positive close)
2. `Txx-02-修复-<finding_id>` only after 01 is `[x]` or promoted to confirmed

Each **fix** subtask MUST contain:

```markdown
# T0x-0y 修复 F-00N <短标题>

状态：`[ ]`

## 来源

1. finding_id：F-00N
2. handoff：`../security-handoff/03-findings.md#F-00N`（或等价路径）
3. 权限类型：...
4. 数据产量：...

## 目标

用项目语言写清：修完后谁不能再看到/改到什么数据。

## 项目定位对齐

1. 影响用户角色：...
2. 影响信任边界：...

## 项目口径对齐

1. 允许改动的层：...
2. 禁止改动：保护区、无关重构、未授权环境...

## 输入

1. 证据文件/符号：`path:symbol`
2. 相关路由/表/DTO：...
3. 上游任务：无 / T0x-0y

## 输出

1. 代码改动文件列表（预期）
2. 测试文件或验证记录

## 执行清单

- [ ] 阅读：`path` 中 `Symbol` 的当前行为（写出现状 1–3 句）
- [ ] 修改：具体改法（参数化 / 加 ownership 条件 / 移除密钥 / 校验权限码…）
- [ ] 同步：调用点 A、B（点名）
- [ ] 测试：单元/集成/手工步骤与期望结果
- [ ] 回归：原权限角色仍可完成合法操作；越权路径返回 401/403/404（写明期望）

## 下游编码注意

1. 只改本 finding 相关路径；禁止顺手重构。
2. 禁止在日志/响应中打印密钥或完整 token。
3. 禁止为通过编译而放宽鉴权。
4. 若发现 handoff 证据过期：停写代码，回写前置约束与进度为 `[!]`。

## 完成标准

1. 原利用路径不可再打到目标数据/能力（描述重现步骤与期望失败）。
2. 合法路径仍可用（描述角色与步骤）。
3. 进度跟踪表已更新验证命令与结果。
```

### 5.5 Priority mapping

| Finding / data-value | Task 优先级 |
|---|---|
| P0 / Critical bulk PII-PAY-AUTH / prod secret | P0 |
| P1 / High AuthZ or admin-as-user | P0 或 P1（写明理由） |
| P2 / limited or partial | P1 或 P2 |
| P3 / hardening | P2 或 P3 |
| manual gap only | research subtask P2，或 `[-]` 并写重启条件 |

## 6) Pipeline Sequences (not broad)

### A) Audit-only remediation planning

```text
arc:audit appsec
  → Handoff Package
  → arc:sdlc (R-task)
  → arc:fix / arc:build per subtask
  → R-verify
```

### B) Scan-assisted

```text
arc:audit appsec Phase1–2 (optional but preferred)
  → arc:security quick|full
  → merge re-ranked into handoff
  → arc:sdlc
  → fix / verify
  → optional arc:security re-scan of fixed paths
```

### C) Single confirmed vuln (small)

Skip full tree if user names one issue: still fill 定位+口径 briefly in pre-constraints; one task group; still use finding-card fields.

## 7) Quality Gate Before Coding

`R-task` may mark planning complete only if:

1. 项目定位 and 项目口径 are filled (or blockers listed).
2. 功能角色 named for this engagement.
3. Every in-scope confirmed finding maps to ≥1 subtask with files/symbols.
4. Every fix subtask has 执行清单 ≥3 concrete steps and 不要做 ≥2 items.
5. 进度跟踪表 contains all subtasks with P0–P3.
6. No subtask titled only “修复安全问题/加固/优化鉴权” without finding_id and paths.

## 8) Anti-Patterns

- Task docs that restate the playbook but name zero project files
- One mega-subtask “修复全部 High”
- Ignoring 口径 then editing protected trees
- Fixer starting before Handoff Package exists
- Scanner severity copied to priority without data yield
- Mixing recon narrative into implementation subtasks without acceptance tests
