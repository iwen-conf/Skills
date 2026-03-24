# arc-cartography Skill

Repository understanding and hierarchical codemap generation.

## Overview

arc-cartography helps orchestrators map and understand codebases by:

1. Selecting relevant code/config files using LLM judgment
2. Creating `.slim/cartography.json` for change tracking
3. Generating empty `codemap.md` templates for searchrs to fill in

## Commands

```bash
# Initialize mapping
python3 <skills_root>/arc-cartography/scripts/cartographer.py init --root /repo --include "src/**/*.ts" --exclude "node_modules/**"

# Check what changed
python3 <skills_root>/arc-cartography/scripts/cartographer.py changes --root /repo

# Update hashes
python3 <skills_root>/arc-cartography/scripts/cartographer.py update --root /repo
```

## Outputs

### .slim/cartography.json

```json
{
  "metadata": {
    "version": "1.0.0",
    "last_run": "2026-01-25T19:00:00Z",
    "include_patterns": ["src/**/*.ts"],
    "exclude_patterns": ["node_modules/**"]
  },
  "file_hashes": {
    "src/index.ts": "abc123..."
  },
  "folder_hashes": {
    "src": "def456..."
  }
}
```

### codemap.md (per folder)

Empty templates created in each folder for searchrs to fill with:
- Responsibility
- Design patterns
- Data/control flow
- Integration points

## Installation

将本技能目录放入你的技能根目录（例如 `<skills_root>/arc-cartography/`），并确保运行时可访问 `scripts/cartographer.py`。
