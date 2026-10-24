# Web Page Performance

Claims about page load and interaction speed: Core Web Vitals, Lighthouse performance, render
blocking, payload and caching. Adapted from Cloudflare's `web-perf` skill. Fixing what this finds
belongs to `arc:code` (repair mode); this reference produces measured evidence and verdicts.

## 1. Preconditions

1. Thresholds and insight names drift. Before citing a number, fetch the current source:
   [web.dev Vitals](https://web.dev/articles/vitals),
   [Lighthouse scoring](https://developer.chrome.com/docs/lighthouse/performance/performance-scoring),
   [DevTools performance](https://developer.chrome.com/docs/devtools/performance).
   Record the URL and date with the verdict.
2. Probe the Chrome DevTools MCP tools (`navigate_page`, `performance_start_trace`). Missing →
   every runtime claim is `BLOCKED`; name the missing server
   (`"chrome-devtools": {"command": ["pnpm", "dlx", "chrome-devtools-mcp@latest"]}`) and continue
   with codebase-only checks (§4) labeled as such.
3. Fix the target: URL, device profile, network throttling, cold or warm cache. A verdict without
   these fields is not comparable.

## 2. Claims

Write each as EXPECTED with a threshold from step 1, for example `LCP ≤ 2.5 s at p75 on the
mobile profile, cold cache`. Defaults when the user gives none (verify against step 1 first):

| Metric | Good | Poor |
|---|---|---|
| TTFB | < 800 ms | > 1.8 s |
| FCP | < 1.8 s | > 3 s |
| LCP | < 2.5 s | > 4 s |
| INP | < 200 ms | > 500 ms |
| TBT | < 200 ms | > 600 ms |
| CLS | < 0.1 | > 0.25 |
| Speed Index | < 3.4 s | > 5.8 s |

## 3. Measure

1. `navigate_page(url)`, then `performance_start_trace(autoStop: true, reload: true)` for a cold
   load. Run ≥3 traces; report the median and the spread. One trace is not a verdict.
2. `performance_analyze_insight` with the `insightSetId` from the trace: `LCPBreakdown`,
   `CLSCulprits`, `RenderBlocking`, `DocumentLatency`, `NetworkRequestsDepGraph`. Names differ by
   version; list the available insights from the trace response when one fails.
3. `list_network_requests(resourceTypes: ["Document", "Script", "Stylesheet", "Font", "Image"])`
   and `get_network_request(reqid)` for: render-blocking head resources, late-discovered chains,
   missing preloads for hero image and fonts, weak `Cache-Control` or validators, uncompressed or
   oversized bundles, preconnects with zero requests to their origin.
4. `take_snapshot(verbose: true)` only when an accessibility claim is in scope.

## 4. Codebase evidence (own repositories only)

Use code-intel queries, not tree scans: `arc-idx files 'vite.config|next.config|webpack.config|astro.config'`,
`arc-idx search 'from "lodash"'`, `'moment'`, `'core-js'`, `'sideEffects'`, `'browserslist'`.
Each candidate (wholesale imports, barrel re-exports, polyfill breadth, production source maps,
missing compression) is a hypothesis until the trace shows its cost.

## 5. Verdicts

| Claim | Evidence |
|---|---|
| Metric meets threshold | median of ≥3 traces, profile, cache state, threshold source |
| Specific resource costs time | insight output naming the resource and estimated savings |
| Recommendation is worth doing | non-zero estimated savings; 0 ms items are reported but not recommended |

`FAIL` items go to `arc:code` as a repair packet: metric, trace evidence, the resource or code
location, and the expected improvement. Recommendations are concrete ("hero.png 450 KB → WebP"),
never "optimize images".
