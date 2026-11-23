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

## 2. Simplicity Ladder, Reuse & Surgical Edits

Stop at the first rung that solves the verified need:
1. **YAGNI**: Skip unrequested configurability or speculative abstractions.
2. **Anti-Wheel-Reinvention (`禁止重复造轮子`)**: Search `arc idx` + standard library (`slices`, `maps`, `cmp`, `strings`, `time`, `filepath`) -> existing shared package -> installed dependency -> minimal new code. Extract domain-agnostic, non-business helpers into topic-named shared packages (`pkg/<topic>` or `internal/pkg/<topic>`) instead of duplicating private helpers across business modules; never create catch-all `util`/`common` dumping grounds.
3. **File Modularity, Strong Typing & Comment Discipline**: Keep source files cohesive and strictly within stack hard limits (Go: `<= 1200` lines per `.go` file, enforced by `gate.sh check`); strictly prohibit `any` and `interface{}` dynamic types in Go (use explicit concrete `struct`s, `json.RawMessage`, or tagged union structs — enforced by `gate.sh check`); inline direct field reads and trivial 1-line expressions instead of inventing ceremonial getters/wrappers. Follow stack comment contracts verbatim (`../_shared/go-conventions.md`: field comments inside `struct` definitions only; concise `// 名称 含义` under `// 请求体` and `// 响应体` including DTO-level Request/Response on `controllers`; single-line `// 函数名 函数功能` on non-controller `c *gin.Context` middlewares/entrypoints and `TestXxx` test functions; `只补缺失，不动已有` on existing files).
4. **Surgical Diff**: Match local style; delete imports/functions orphaned by your diff; never reformat unrelated files. Mark deliberate throughput ceilings with `// debt: <ceiling & upgrade trigger>`.

## 3. Root-Cause & Anti-Rot Gates

- **Scope Tokens**: Honor `只读`, `只写文档`, `只做前端`, `禁止重启`, `明确不做` verbatim.
- **Evidence-First Repair**: Prove exact fault at `file:line` with a failing reproducer before editing; inspect sibling consumers of the same broken invariant; ban silent fallbacks or weakened assertions.
- **Data Hygiene**: Store/compare timestamps in UTC; keep one error-envelope shape boundary-wide; wrap multi-row writes in explicit transactions with `WHERE status = ?` CAS checks; leave zero `TODO`/`FIXME`/stub panics (`gate.sh check`).
- **Real Data & Verifiable Provenance (`真实结果与依据`)**: Every code output, API/UI field, metric, test result, doc claim, and audit finding MUST come from a real data source or real runtime/command/readback execution with traceable provenance (exact `file:line`, SQL/API query, command output, or receipt). Strictly ban fabricated values, hardcoded/mocked production data, silent fake fallbacks, invented line numbers/counts, or presenting estimates/historical logs as live verified results; when data or execution is unavailable, surface an explicit error, empty state, or `BLOCKED` / `UNKNOWN` status.
