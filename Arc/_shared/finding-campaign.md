# Finding Campaign

Turns `arc:audit` findings into detailed task batches before any fix. `arc:audit` reads it to write
the handoff package; `arc:plan` reads it to write the tasks. Vocabulary and the severity→priority
mapping: [contract.md](contract.md) §4. Code queries: [code-intel.md](code-intel.md). Completion and
root-cause repair: [execution-truth.md](../../docs/execution-truth.md) §2, §6.

Scope: findings → constraints → leaves → fix → re-verification. No red-team operations.

## 1. Functional roles (功能角色)

| Role | Name | Owner | Produces | Never |
|---|---|---|---|---|
| `R-recon` | 侦察审计员 | `arc:audit` appsec | Asset table, data map, finding cards, manual gaps | Code edits; active DAST |
| `R-scan` | 扫描操作员 | `arc:audit` scan | Scanner reports under `.arc/artifacts/<task-slug>/security/`, triaged findings | Claiming AuthZ coverage from scanners alone |
| `R-task` | 任务作者 | `arc:plan` | `00-前置约束.md` with 定位/口径/角色, task groups, detailed leaves, tracker | Production code |
| `R-fix` | 修复实施员 | `arc:code` | Code and tests for one leaf at a time | Rewriting the plan mid-fix without updating the task docs |
| `R-verify` | 验证员 | `arc:verify`, plus `arc:audit` re-scan | Verification evidence in the tracker | `[x]` without evidence |

1. Documents name the active role; one agent may hold several.
2. `R-recon`/`R-scan` finish the handoff package (§2) before `R-task` starts. `R-task` passes the
   gate (§7) before `R-fix` starts the first leaf.
3. Handoff missing 项目定位, 项目口径 or finding cards → `R-task` runs one minimal discovery pass over
   README, AGENTS, PRD and architecture docs, or marks the dependent leaves `[!]`. It never invents
   files or facts.

## 2. Handoff package (written by arc:audit)

Path: `.arc/artifacts/<task-slug>/security/handoff/` (arc:audit writes nowhere else). When the
user wants it tracked in the repository, `arc:plan` campaign mode copies it unchanged to
`docs/08-安全审计/NN-<主题>/handoff/` and links the copy from the campaign README.

```text
handoff/
  00-项目定位与口径.md   # 项目定位 + 项目口径 + 功能角色 for this engagement
  01-assets.md
  02-data-map.md
  03-findings.md         # finding cards
  04-manual-gaps.md
  05-scan-triage.md      # optional, from scan mode
  06-task-seed.md        # suggested groups only, not leaves
```

### 项目定位

```markdown
## 项目定位

1. 产品形态：...（例：小说阅读 + 作家后台 + 管理端 多端应用）
2. 用户角色：...（例：读者 / 作者 / 管理员 / 支付回调服务）
3. 核心价值与不可丢数据：...（例：账号、订单/积分、章节正文、支付一致性）
4. 信任边界：...（例：公网 API、SSR Cookie、对象存储、身份网关、支付网关）
5. 本次审计/修复在产品中的位置：...（例：上线前 AppSec 修复战役 / 单点漏洞修复）
```

- Product truth from the target repository, never a generic "web app".
- Unknown → `待确认`; P0 leaves that depend on it are `[!]`.
- Every leaf restates the affected user role (读者/作者/管理员/匿名).

### 项目口径

```markdown
## 项目口径

1. 部署形态：...（例：单节点；非分布式默认）
2. 数据命名/存储：...（例：业务表前缀、PG + Mongo）
3. 保护区：...（例：参考项目目录只读；某客户端目录默认不动）
4. 环境与部署顺序：...（例：先测试环境再生产）
5. 架构约束：...（例：usecase 边界、禁止顺手大重构）
6. 安全测试边界：...（例：只读审计 / 仅授权测试服 DAST / 禁止生产主动攻击）
7. 密钥与证据：...（例：聊天脱敏；原始证据落本地）
8. 明确不做：...（例：不改 iOS、不引入新中间件）
```

- Collected from this repository's agent instructions and architecture conventions, citing file
  paths; never pasted from another project.
- Every 口径 line becomes a 不要做 line in the leaves.
- A leaf that violates 口径 is invalid until rewritten, or until 口径 changes with user approval.

### Finding card fields used for tasking

| Field | Used for |
|---|---|
| ID (`F-NNN`) | Leaf title prefix and link |
| Severity | Priority via contract §4 |
| Status | Leaf type (§3) |
| `file:line` and evidence | 输入, affected files, `<Context-Queries>` |
| Permission class | Group split; regression role |
| Data yield and why it scores | Acceptance: what must no longer leak or change |
| Exploit conditions | Verification preconditions |
| Fix direction | 执行清单 skeleton |
| Verification suggestion | 完成标准 |

### 06-task-seed.md

```markdown
# Task Seed

## 建议任务组（仅种子，不是最终子任务）

| 组 | 名称 | 依据 finding_ids | 优先级 |
|---|---|---|---|
| T01 | 配置与密钥收敛 | F-001, F-003 | P0 |
| T02 | 鉴权与 IDOR | F-002, F-005 | P1 |

## 不做

1. ...
```

`R-task` expands seeds into full leaves and never treats a seed as complete. Seed priority does not
set difficulty; each leaf is rated from its own contracts, sides, data risk and verification cost.

## 3. Finding status → leaf type

| Status | Leaves |
|---|---|
| `confirmed` | One fix leaf per affected side (`修复-F-NNN`) |
| `likely` | `复核-F-NNN` first (read-only proof or false-positive close); `修复-F-NNN` depends on it with readiness "复核确认" |
| `needs-triage` | `分诊-F-NNN` research leaf with bounded questions and an exit condition; no fix leaf until the finding is `confirmed` |
| `false-positive` | No leaf; the Finding 索引 row cites the disproving evidence |
| `manual-gap` | Manual-check leaf naming who runs which steps on which surface, or `[-]` with a restart condition |

## 4. Priority, dedup, grouping

1. Priority is the contract §4 mapping of the card's severity, with no ranges and no re-ranking
   during planning. A card whose data yield contradicts its severity, or that has no severity or no
   data-yield justification, goes back to `arc:audit` for rating; its leaves stay `[!]` meanwhile.
2. One finding reported by several tools or passes → one Finding 索引 row carrying all evidence.
3. Findings sharing one root cause and owning boundary → one fix leaf per side listing every ID;
   the root-cause fix boundary (execution-truth §6) is written once and linked from each leaf.
4. Group (`TNN`) by risk domain + permission class + data domain, never by scanner name. Inside a
   group use the normal BE/FE package hierarchy and difficulty-based leaf sizing.

| Risk domain | Examples |
|---|---|
| Config, secrets, ops | Committed secrets, pprof, default admin |
| AuthN, session | Login, OTP, password reset |
| AuthZ, IDOR | Object ownership |
| Payment, credits | Callback, amount, points |
| Upload, storage | Path traversal, signed URL |
| Injection, data query | SQL/NoSQL on high-yield tables |
| Dependency, supply chain | Reachable CVEs only |
| Hardening | Headers, cookie flags |

## 5. Task directory and 前置约束 additions

```text
docs/03-执行任务/NN-安全修复-<短主题>/      # or NN-AppSec审计跟进-<短主题>/
  README.md
  00-前置约束.md
  进度跟踪表.md
  tasks/
    T01-<风险域>/
      README.md
      backend/
        README.md
        T01-BE-01-修复-F-001.md
      frontend/                # only when frontend work is in scope
```

`00-前置约束.md` adds, after the base sections:

1. `## 项目定位` and `## 项目口径` (§2).
2. `## 功能角色`: who holds each role for this engagement.
3. `## 上游 Handoff`: relative links to the package files and the scan time.
4. `## Finding 索引`:

   | finding_id | 严重度 | 状态 | 优先级 | 标题 | 子任务 |
   |---|---|---|---|---|---|
   | F-001 | High | confirmed | P1 | ... | T01-BE-01 / 待拆 |

5. `## 修复战役边界`: findings in scope this round; deferred ones `[-]` with a restart condition.

## 6. Fix leaf card

Default: one confirmed finding → one independently verifiable fix leaf per affected side, plus a
verify leaf only when it has a separate outcome. Split D3 work by risk and acceptance boundary;
isolate D4 unknowns first. A finding is resolved only when every required side is fixed.

The card keeps the base metadata table (上级, 所属端, 优先级, 难度与依据, 目标面, 前置依赖, 下游影响),
`<Context-Queries>`, 输出 and 执行注意, and adds:

```markdown
# T0x-BE-0y 修复 F-00N <短标题>

状态：`[ ]`

## 来源

1. finding_id：F-00N
2. handoff：`<handoff>/03-findings.md#F-00N`
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
3. 上游任务：无 / T0x-BE-0y，注明所需证据与就绪条件。

## 执行清单

- [ ] 阅读：`path` 中 `Symbol` 的当前行为（写出现状 1–3 句）
- [ ] 修改：具体改法（参数化 / 加 ownership 条件 / 移除密钥 / 校验权限码…）
- [ ] 同步：调用点 A、B（点名）
- [ ] 测试：单元/集成/手工步骤与期望结果
- [ ] 回归：原权限角色仍可完成合法操作；越权路径返回 401/403/404（写明期望）

## 不要做

1. 只改本 finding 相关路径；禁止顺手重构。
2. 禁止在日志/响应中打印密钥或完整 token。
3. 禁止为通过编译或测试而放宽鉴权。
4. 发现 handoff 证据过期：停止改代码，把前置约束与进度改为 `[!]`。

## 完成标准

1. 原利用路径不再能取得目标数据/能力（写重现步骤与期望失败）。
2. 合法路径仍可用（写角色与步骤）。
3. 进度跟踪表已记录验证命令与结果。
```

## 7. Gate before coding

`R-task` marks planning complete only when:

1. 项目定位 and 项目口径 are filled, or their blockers are listed.
2. 功能角色 are named for this engagement.
3. Every in-scope `confirmed` finding maps to ≥1 leaf naming files and symbols; every `likely` and
   `needs-triage` finding maps to a 复核 or 分诊 leaf.
4. Every fix leaf has ≥3 concrete 执行清单 steps and ≥2 不要做 items.
5. The tracker lists every leaf with owning side, priority, difficulty, parent and dependency
   readiness; the cards explain the difficulty.
6. No leaf is titled only "修复安全问题", "加固" or "优化鉴权" without a finding ID and paths.

## 8. Re-verification on close

A finding-sourced leaf becomes `[x]` only when all hold on the intended surface:

1. The original exploit path, re-run as a reproducer or test, now fails as stated (401/403/404 or
   the named rejection).
2. The legitimate path for the original role still succeeds.
3. Sibling paths sharing the boundary are checked, or listed as residual risk.
4. A scanner-sourced finding shows no recurrence in an `arc:audit` re-scan of the fixed paths.
5. The tracker records commands and results; the Finding 索引 row shows the closing evidence.

Evidence from before the fix or from another surface does not count. A `false-positive` close needs
the disproving evidence in the same form.

## 9. Sequences

- **Audit only:** `arc:audit` appsec → handoff → `arc:plan` (`R-task`) → `arc:code` per leaf →
  `arc:verify` (`R-verify`).
- **Scan assisted:** `arc:audit` appsec Phase 1–2 → `arc:audit` scan → triage merged into the
  handoff → `arc:plan` → `arc:code` → `arc:verify` → `arc:audit` re-scan of the fixed paths.
- **Single confirmed finding named by the user:** short 定位 and 口径 in 前置约束, one group, finding
  card fields still required.

## 10. Forbidden

- Task docs that restate methodology but name no project files.
- One mega-leaf such as "修复全部 High".
- Leaves that touch protected paths listed in 项目口径.
- `R-fix` starting before the handoff package exists and the §7 gate passes.
- Priority copied from scanner severity without the card's data-yield justification.
- Recon narrative inside implementation leaves without acceptance checks.
