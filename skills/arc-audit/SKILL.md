---
name: arc-audit
version: 4.2.0
description: Read-only diff/PR review, AppSec & vulnerability scan (`security-scan.py`), Go Explicit Architecture & composition-over-inheritance audit, code-rot & wheel-reinvention check, Linear/HIG UI anti-slop critique, and agent host health check. Triggers on 代码评审, 能不能合, 安全扫描, 技术债, 过度设计, 架构核查, 注释规范检查, 页面太乱, 环境体检.
---

# arc-audit

Read-only evaluation. Mutate zero source files, configs, or `.arc/arc.db` records.

## Protocol

1. **Diff, Code-Rot, Reuse & Stack Audit**: Rank findings `CRITICAL > HIGH > MEDIUM` with exact `file:line`, failure/exploit scenario, and minimal remedy against `../_shared/contract.md` and matching stack specs (`ui-system.md`, `go-conventions.md`, `api-contract.md`, `android.md`, `harmonyos.md`, `apple-native.md`). For Go diffs, explicitly audit:
   - **`iwen-example` Explicit Architecture**: Plural package layout (`domain/{entities,events,filters,repositories,services}`, `infrastructure/gateways/persistence/postgres/{models,repository}`, `support/*`, `wire/*`), strict import bans, `Contract`/`Params`/`Results`/`Service`/`Repository`/`Model`/`Engine` naming (zero `Port`/`Adapter` or `var _ Contract = (*Service)(nil)`).
   - **Composition Over Inheritance & Zero `any`/`interface{}` Strong Typing**: Ban `BaseService`/`BaseRepository`/`BaseEntity` pseudo-inheritance and embedded `*sql.DB`/`sync.Mutex`/SDK leaks; strictly ban `any` and `interface{}` dynamic types anywhere in Go (enforce concrete `struct`s, `dto/responses` `Base` struct-literal embedding, and narrow consumer interfaces at the 4 explicit seams only; zero `gin.H`, `map[string]any`, `[]any`, `Response[T any]`, `NewXxxResp` wrappers, or `entity.ToDTO()`).
   - **Budgets, Reuse & Go Standards**: `.go <= 1200` lines (`service_*.go <= 400`, `controllers/* <= 600`, `<= 2` private funcs per workflow file), stdlib/`utils-pkg`/`arc idx` reuse, `pkg/<topic>` extraction (ban `util`/`common` & ceremonial 1-line getters), `MixedCaps` typed enums, `errors.Is`/`errors.As` + log-once, `TxManager` CAS SQL, and `zap` hygiene.
   - **Comment Hygiene**: `struct` field comments inside `struct`, 3-part `请求体`/`响应体` headers (DTO-level on `controllers`), single-line `// 函数名 函数功能` on middlewares and `TestXxx`, internal `// 1. xxx` steps, and `只补缺失，不动已有`.
2. **Security Scan**: Run `python3 scripts/security-scan.py . --task-slug <slug>` for secret, SAST, lockfile, and OpenAPI checks.
3. **Agent Host Health**: Verify `~/.claude/skills`, `~/.codex/skills`, `~/.grok/skills` and `arc capabilities --json` without printing secret values.
4. **Data Provenance & Anti-Fabrication Audit (`真实数据与依据核查`)**: Flag any hardcoded fake data, static mock fallbacks, fabricated metrics, or unverified claims in code/UI as `CRITICAL`/`HIGH` defects. In the audit report itself, never invent line numbers, unread code behavior, coverage/perf metrics, or test outcomes—every finding and number must cite verified `file:line` or live command output, explicitly marking unexecuted checks as `NOT_RUN` / `BLOCKED`.
