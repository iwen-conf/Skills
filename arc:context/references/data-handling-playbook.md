# Data Handling Playbook

Use this reference only when the task needs a concrete source-by-source handling path.

## Source routing matrix

| Source type | Preferred path | Good fallback | Return to conversation |
|---|---|---|---|
| Build/test/log output | Save to file, extract errors/warnings/counts, group by code or path | Pipe to temp file and summarize with shell/Python | Exit code, error families, hottest files, last failing lines |
| Large JSON/CSV/XML/YAML | Parse in sandbox or local script, compute stats first | Read file locally and print only queried fields | Counts, schema anomalies, suspicious records, top categories |
| Markdown docs / fetched pages | Index once, query many times | Chunk manually and grep/ripgrep targeted sections | Exact relevant snippet, heading path, next search terms |
| Browser snapshot / console / network | Save snapshot/trace first, then inspect the file | Export to file and grep or script over it | Present controls, missing elements, failing requests, error messages |
| Git log / diff / blame / repo scans | Narrow by path, author, symbol, or time window | Save command output and summarize | Commit IDs, changed paths, regression clusters |
| API responses | Parse the response body and summarize fields/anomalies | Save JSON to file then inspect locally | Status, contract mismatches, broken IDs, null fields |
| Mixed multi-command research | Batch collection first, then split into follow-up queries | Store each source under `sources/` and write a query list | Consolidated shortlist, unresolved questions, next commands |

## Output budget heuristic

| Budget | Use when | Expected response shape |
|---|---|---|
| `tiny` | Need a go/no-go signal or one concrete answer | 1-5 lines, one key metric or verdict |
| `compact` | Default for debugging and exploration | 5-20 lines, grouped findings with anchors |
| `standard` | Need a handoff note for a downstream skill | Short summary + artifact path + next-step queries |

## Minimal artifact set

When a task is likely to revisit the same data, create:

1. `plan/context-plan.md` — source list, chosen path, fallback.
2. `sources/` — raw artifacts kept out of the main thread.
3. `retrieval/search-queries.md` — reusable grep/index/search terms.
4. `findings/compact-findings.md` — only the narrowed result.

## Anti-drift checklist

Before replying, verify:

- Did I avoid dumping the full raw payload?
- Can the user reproduce the result from a saved artifact or query?
- Did I return identifiers, counts, paths, or exact failure anchors?
- If I had to degrade, did I say so explicitly?
- If the session is long-running, did I leave resumable notes behind?
