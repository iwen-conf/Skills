# Skills

Six-skill architecture for **Claude Code**, **Codex**, and **Grok Build**.

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
│   │   ├── api-contract.md   # HTTP, JSON, idempotency & upload rules
│   │   ├── go-conventions.md # Go layering, concurrency & SQL CAS rules
│   │   ├── android.md        # Kotlin/Compose, AGP 9, Nav 3, redroid & avdslim
│   │   ├── harmonyos.md      # ArkTS/ArkUI Stage, API 24 baseline & TS migration
│   │   ├── apple-native.md   # Swift/SwiftUI (iOS, iPadOS, macOS, visionOS) & HIG
│   │   └── cnb.md            # CNB (.cnb.yml) pipelines, PR line review & CLI
│   ├── arc-plan/             # Explore code (arc idx) + vet deps + freeze contracts & waves
│   ├── arc-code/             # Implement, repair & build UI/mobile/backend + gate.sh
│   ├── arc-verify/           # Independent runtime E2E/contract proof & arc.db receipts
│   ├── arc-audit/            # Read-only diff, rot, UI anti-slop & AppSec scan
│   ├── check/                # Gate-verified Git, PR, release & CNB follow-through
│   └── lazycat/              # Lazycat intake -> boundary -> <=12MB LPK ship & update
├── hosts/                    # Host profiles & adapters (claude, codex, grok)
└── scripts/                  # sync.sh (installer & verifier) & arc-wave.py (wave/handoff CLI)
```

## Install & Verify

```bash
bash scripts/sync.sh           # Sync 6 skills to Claude / Codex / Grok + sweep extras to backup
bash scripts/sync.sh --check   # Verify frontmatter, links, scripts & the 3 host roots
```
