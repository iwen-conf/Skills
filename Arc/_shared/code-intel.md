# Code Intelligence Protocol

Owned by `arc:explore`; read by every Arc skill before reading or editing unfamiliar code.

## 1. Preflight (once per repository per session)

```bash
arc-idx version
arc-idx doctor --format json
```

- Repair when doctor shows `healthy: false`, `index_stale: true` or `tags_present: false`: run
  `arc-idx index --quiet` once, then re-run `doctor`. `index` does not fix a tool reported as
  `MISSING` under `tools` (`ctags`, `ast-grep`); installing needs permission, and until then
  `symbol` and `ast` are unavailable.
- Daemon: `daemon_alive: false` → `arc-idx daemon start` (exit 1 with `already running with pid N`
  is normal). `daemon_pid` present but not alive → `arc-idx daemon restart`.
- Version: accept any `arc-idx 1.x`; report the version, do not stop on a patch or minor change.
- Repository root: the nearest ancestor of the working directory (or of `--repo <dir>`) that holds
  an `.ai-code-index/` directory or a `.git` directory; config is `<root>/.ai-code-index/config.json`,
  else built-in defaults. Check `repo_root` in the doctor output. Use `--repo <dir>` for a child
  clone and `git -C <clone>` for git commands. A git worktree or submodule has a `.git` file, not a
  directory, and resolves to the parent: create `<clone>/.ai-code-index/` first.
- A `search` result with `"engine": "rg-fallback"` means no index shards were found: repair as
  above before relying on it; otherwise only §6 allows it.

## 2. Commands

| Need | Command |
|---|---|
| File names | `arc-idx files <pattern> --format json` (case-insensitive path substring or glob, not regex) |
| Text / regex | `arc-idx search '<zoekt query>' --format json` (`f:`, `lang:`, `sym:`, `case:yes`, `-<term>`, `-f:<path regex>`) |
| Definition | `arc-idx symbol <Name> --exact --format json` (optional `--kind`, `--lang`) |
| Structure | `arc-idx ast --lang <language> '<pattern>' [path ...] --format json` |
| Inventory | `arc-idx stats --format json` (counts by file extension) |
| Health | `arc-idx doctor --format json` |

Syntax facts (verified against the engine):

- `search`: every bare term is an RE2 regex; escape `. ( ) [ ] ? + * ^ $ \` for literals.
  Space-separated terms are ANDed per file, not adjacent: quote a phrase (`"func New"`) or use `\s+`.
  Write a literal double quote as `\"`; a bare `"` fails with `unterminated quoted string`.
  `lang:` takes the `language` value shown in search JSON, case-insensitive (`go`; `javascript`
  covers `.jsx`); when unsure, use `f:\.<ext>$`.
- `symbol`: without `--exact`, a case-insensitive substring match. `--kind` takes ctags kind names:
  Go `func` (methods too; the receiver is in `scope`, e.g. `order.Service`), `struct`,
  `interface`, `member` (fields); Python methods are `member`; JavaScript/TypeScript `method`.
- `ast`: paths are trailing arguments; there is no `--paths`, `--max` or `--context`.
- `--profile <name>` (`search`, `files`, `stats`): only names listed in doctor `profiles`; `all` is
  implicit; an unknown name is an error.
- Reading hits in context: `search … --format ai-md --context 3 --max 20`.

Counts:

- `search --max` caps returned files (default 50), and `match_count` counts only the returned files.
  When `file_count` equals `--max` the list is truncated: re-run with a larger `--max` before reporting
  a count or a complete list.
- Zero hits: `search` → `file_count: 0`, `files: null`; `symbol` → `count: 0`; `ast` exits 1 with
  `arc-idx ast: ast-grep:` and nothing after it. Text after `ast-grep:` is a real error.

## 3. Query ladder (stop at the first rung that answers the question)

1. Exact symbol (`symbol --exact`).
2. Exact string (`search 'case:yes <literal>'`, regex-escaped per §2): identifiers, route paths,
   error text, JSON keys.
3. AST pattern (`ast`): calls, literals, method sets.
4. File name (`files`).
5. Broadened text: case-insensitive, partial, snake_case/camelCase/PascalCase variants, receiver
   forms, string-literal forms.
6. Host-native search (`rg`, IDE search): only under the fallback conditions in §6.

Never query with natural-language sentences. Read the file at every hit before concluding;
snippets are not evidence.

## 4. Impact checklist

Run before changing the signature or meaning of any function, type, field, route, table column,
config key, error code or event. Record each row as `query → hit count → files`. Write
`none (query …)` for an empty row; never skip a row.

| # | Surface | Query templates (verify syntax once per repository) |
|---|---|---|
| 1 | Definitions and implementations | `symbol <Name> --exact`; interface implementations: `ast --lang go 'func ($_ $_) <Method>($$$) $$$'` |
| 2 | Callers and references | unqualified calls `ast --lang go '<Name>($$$)'`; method and package-qualified calls `search 'lang:go case:yes \.<Name>\('`; text pass `search 'case:yes \b<Name>\b'` for function values and name-based reflection |
| 3 | Construction and writes | `ast --lang go '<Type>{$$$}'` and `'<pkg>.<Type>{$$$}'`; `symbol New<Type> --exact`; `search 'case:yes \.<Field>\s*=[^=]'` |
| 4 | Reads and serialization | `search 'case:yes json:\"<key>\b'`; templates; SQL column names |
| 5 | Contract boundaries | `search '<route-path>'` across server registration, client API modules and Markdown API docs; same-named client types; message schemas; migrations |
| 6 | Config and wiring | `search 'case:yes <CONFIG_KEY>'` across code, env samples, deploy manifests; DI registration; cron/consumer registration |
| 7 | Tests | `search 'f:_test\.go$ <Name>'`; `search 'f:\.test\.tsx?$ <Name>'` and `search 'f:\.spec\.tsx?$ <Name>'`; fixtures, golden files, generated mocks |
| 8 | Docs and generated code | references in `docs/`; trace generated files to their generator input (proto, OpenAPI, sqlc) |

For TypeScript, Python or other languages, use the same eight surfaces with `lang:<language>` and
the language's AST patterns; there `ast '$R.<Name>($$$)'` matches method calls.

Cross-check: for rows 2–4 use one structural and one text query. When they disagree, re-read the
files and trust the AST result. Exception: Go selector calls (`x.Name(…)`, `pkg.Name(…)`) never
match through `arc-idx ast` (the pattern parses as a type conversion), so an AST zero for them is
not evidence; decide each text hit from the source.

## 5. When a query fails

- **Zero hits**, widen in order: (1) spelling, case and naming variants; (2) literal forms (route
  strings, JSON tags, error text); (3) AST pattern; (4) `doctor` for index freshness; (5) host
  search over unindexed paths. After rung 5, report `not found` with every query run. Never guess
  a path.
- **More than 200 hits**: narrow by directory, language and word boundary; if still too many,
  sample per directory and label the result as sampled.
- **Dynamic dispatch** (interfaces, reflection, DI containers, string-keyed routes or handlers):
  search the method name and the registration string. Mark anything still unresolved as
  `dynamic — confirm at runtime`.
- **Generated or vendored code**: report the hit, follow generated code to its source, and do not
  expand into vendored code.

## 6. Fallback conditions

Host-native search is allowed only when `arc-idx` is missing, stays unhealthy after one repair,
or the target lies outside every indexed repository. Label such evidence `fallback:<tool>` and
state the reason.

Never indexed: dot-directories except `.github`. Not indexed by default (`ignore_dirs`,
`ignore_files`, `index_exts`, `size_max` in `.ai-code-index/config.json` replace these): directories
such as `vendor`, `node_modules`, `dist`, `build`, `target`, `out`, `outputs`, `release`, `tmp`,
`logs`, `coverage`, `uploads`; files over 1 MiB; extensions outside the code/config/docs allowlist
(`.txt`, `.csv`, `.log` among them); lock files, `go.sum`, `*.min.js`, `*.map`, `*.snap`. A target
there counts as outside the indexed repository.

## 7. Evidence format

```text
Q1 symbol OrderService --exact → 1 → internal/order/service.go:24
Q2 search 'lang:go case:yes \.CreateOrder\(' → 3 → api/order_handler.go:51, jobs/retry.go:88, service_test.go:40
Q3 search 'case:yes json:\"order_id\b' → 0 → none (query Q3)
```
