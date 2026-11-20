# Core Engineering Contract

Shared invariants for all 6 skills. Load stack-specific `_shared/*.md` files only when the task touches that surface.

## 1. Skill Ownership

| Skill | Allowed Writes | Strictly Forbidden |
|---|---|---|
| `arc-plan` | Index cache (`arc idx`), specs, frozen contracts, dependency records, `.arc/arc.db` tasks | Application or test source code |
| `arc-code` | Source, tests, and touched configs inside `write_scope` | Files outside `write_scope`, live deploys |
| `arc-verify` | Tests, fixtures, immutable `.arc/arc.db` receipts | Production source edits |
| `arc-audit` | Read-only reports under `.arc/artifacts/` | Any source, config, or DB mutation |
| `check` | Git commits, PRs, tags, release builds, authorized CNB actions | Unverified edits or unowned dirty files |
| `lazycat` | Wrapper manifests (`package.yml`, `lzc-*.yml`), `.lpk` artifacts | Unauthorized upstream business-code edits |

## 2. Simplicity Ladder & Surgical Edits

Stop at the first rung that solves the verified need:
1. **YAGNI**: Skip unrequested configurability or speculative abstractions.
2. **Reuse**: Existing repo helper -> standard library -> platform-native capability -> installed package -> minimal new code.
3. **Surgical Diff**: Match local style; delete imports/functions orphaned by your diff; never reformat unrelated files. Mark deliberate throughput ceilings with `// debt: <ceiling & upgrade trigger>`.

## 3. Root-Cause & Anti-Rot Gates

- **Scope Tokens**: Honor `只读`, `只写文档`, `只做前端`, `禁止重启`, `明确不做` verbatim.
- **Evidence-First Repair**: Prove exact fault at `file:line` with a failing reproducer before editing; inspect sibling consumers of the same broken invariant; ban silent fallbacks or weakened assertions.
- **Data Hygiene**: Store/compare timestamps in UTC; keep one error-envelope shape boundary-wide; wrap multi-row writes in explicit transactions with `WHERE status = ?` CAS checks; leave zero `TODO`/`FIXME`/stub panics (`gate.sh check`).
