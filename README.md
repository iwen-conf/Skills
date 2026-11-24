# Skills

Six-skill architecture for **Claude Code**, **Codex**, **Grok Build**, and **Gemini CLI**.

Actions are skills; tech stacks are shared references loaded only when touched.

## Structure

```text
Skills/
├── AGENTS.md                 # Stateless cross-agent handoff & wave execution rules
├── skills/                   # Exactly 6 global skills (~280 tokens startup metadata)
│   ├── _shared/              # Stack & contract rules loaded on demand:
│   │   ├── contract.md       # Ownership, simplicity ladder, root-cause & anti-rot gates
│   │   ├── handoff.md        # .arc/arc.db CAS state machine & arc-wave rules
│   │   ├── ui-system.md      # Linear x Apple Notes UI standard & anti-slop matrix
│   │   ├── web-frontend.md   # 2026 Web frontend baseline (Vite/Astro, React 19, TanStack, shadcn/ui, Orval)
│   │   ├── debugging.md      # Root-cause debugging ladder, bisect, blast sweep & diff audit traps
│   │   ├── api-contract.md   # HTTP, JSON, idempotency & upload rules
│   │   ├── go-conventions.md # Go layering, concurrency & SQL CAS rules
│   │   ├── android.md        # Kotlin/Compose, AGP 9, Nav 3, redroid & avdslim
│   │   ├── harmonyos.md      # ArkTS/ArkUI Stage, API 24 baseline & TS migration
│   │   ├── apple-native.md   # Swift/SwiftUI (iOS, iPadOS, macOS, visionOS) & HIG
│   │   └── cnb.md            # CNB (.cnb.yml) pipelines, PR line review & CLI
│   ├── arc-plan/             # Explore code (arc idx) + vet deps + freeze contracts & waves
│   ├── arc-code/             # Implement, repair & build UI/mobile/backend + gate.sh
│   ├── arc-verify/           # 26-technique code testing + 6D business E2E proof & arc.db receipts
│   ├── arc-audit/            # Read-only diff, rot, UI anti-slop & AppSec scan
│   ├── check/                # Gate-verified Git, PR, release & CNB follow-through
│   └── lazycat/              # Lazycat intake -> boundary -> <=12MB LPK ship & update
├── hosts/                    # Host profiles & adapters (claude, codex, grok, gemini)
└── scripts/                  # sync.sh (installer & verifier) & arc-wave.py (wave/handoff CLI)
```

## Install & Verify

```bash
bash scripts/sync.sh                  # Sync 6 skills to Claude / Codex / Grok / Gemini (retains third-party skills)
bash scripts/sync.sh --prune-extra    # Also archive unlisted/extra skills to ~/.agents/skills-backup-<ts>
bash scripts/sync.sh --check          # Verify frontmatter, openai.yaml bounds, links, scripts & the 4 host roots
```
