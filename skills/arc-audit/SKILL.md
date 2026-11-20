---
name: arc-audit
description: Read-only diff/PR review, AppSec & vulnerability scan (`security-scan.py`), code-rot audit, Linear/HIG UI anti-slop critique, and agent host health check. Triggers on 代码评审, 能不能合, 安全扫描, 技术债, 过度设计, 页面太乱, 环境体检.
---

# arc-audit

Read-only evaluation. Mutate zero source files, configs, or `.arc/arc.db` records.

## Protocol

1. **Diff, Code-Rot & Stack Audit**: Rank findings `CRITICAL > HIGH > MEDIUM` with exact `file:line`, failure/exploit scenario, and minimal remedy against `../_shared/contract.md` and matching stack specs (`ui-system.md`, `go-conventions.md`, `api-contract.md`, `android.md`, `harmonyos.md`, `apple-native.md`).
2. **Security Scan**: Run `python3 scripts/security-scan.py . --task-slug <slug>` for secret, SAST, lockfile, and OpenAPI checks.
3. **Agent Host Health**: Verify `~/.claude/skills`, `~/.codex/skills`, `~/.grok/skills` and `arc capabilities --json` without printing secret values.
