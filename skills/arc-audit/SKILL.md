---
name: arc-audit
version: 4.1.0
description: Read-only diff/PR review, AppSec & vulnerability scan (`security-scan.py`), code-rot & wheel-reinvention audit, Go comment/file-size/architecture review, Linear/HIG UI anti-slop critique, and agent host health check. Triggers on 代码评审, 能不能合, 安全扫描, 技术债, 过度设计, 注释规范检查, 页面太乱, 环境体检.
---

# arc-audit

Read-only evaluation. Mutate zero source files, configs, or `.arc/arc.db` records.

## Protocol

1. **Diff, Code-Rot, Reuse & Stack Audit**: Rank findings `CRITICAL > HIGH > MEDIUM` with exact `file:line`, failure/exploit scenario, and minimal remedy against `../_shared/contract.md` and matching stack specs (`ui-system.md`, `go-conventions.md`, `api-contract.md`, `android.md`, `harmonyos.md`, `apple-native.md`). For Go diffs, explicitly audit:
   - `iwen-example` Explicit Architecture boundaries (`Contract`/`Params`/`Results`/`Service`/`Repository`/`Model`/`Engine`, zero `Port`/`Adapter` or layer leaks).
   - File length hard cap (`<= 1200` lines per `.go` file) and proactive workflow splitting.
   - Anti-wheel-reinvention (missed stdlib/`arc idx` reuse, duplicated non-business helpers not extracted to `pkg/<topic>` or `internal/pkg/<topic>`, or catch-all `util`/`common` dumps).
   - Comment hygiene: field-level comments inside `struct` definitions, matching `请求体`/`响应体` `// 名称 含义` lines (no duplicated struct fields in function headers), sequential internal `// 1. xxx` step comments, and `只补缺失，不动已有` on legacy files.
2. **Security Scan**: Run `python3 scripts/security-scan.py . --task-slug <slug>` for secret, SAST, lockfile, and OpenAPI checks.
3. **Agent Host Health**: Verify `~/.claude/skills`, `~/.codex/skills`, `~/.grok/skills` and `arc capabilities --json` without printing secret values.
4. **Data Provenance & Anti-Fabrication Audit (`真实数据与依据核查`)**: Flag any hardcoded fake data, static mock fallbacks, fabricated metrics, or unverified claims in code/UI as `CRITICAL`/`HIGH` defects. In the audit report itself, never invent line numbers, unread code behavior, coverage/perf metrics, or test outcomes—every finding and number must cite verified `file:line` or live command output, explicitly marking unexecuted checks as `NOT_RUN` / `BLOCKED`.
