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
2. Bind an installed trace-capable browser observer: the project's CDP/Playwright runner,
   `agent-browser` when its trace exposes the required metric, or Chrome DevTools MCP. Record
   its version and actual metric source. A missing required observer makes that claim `BLOCKED`;
   continue codebase checks (§4). Tool acquisition needs existing authorization.
3. Fix the target: URL, device profile, network throttling, cold or warm cache. A verdict without
   these fields is not comparable.

## 2. Claims

Write EXPECTED only for a user/project-confirmed budget and target population, with its source.
The following dated industry examples are RISK/reference guidance until explicitly confirmed;
they never supply a missing SLO. Verify their current definitions against step 1 before citing:

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

A p75 field claim needs representative field observations and the project's sample/window
contract. A few cold-load lab traces and their median do not prove that population percentile.
INP needs actual named interactions in the claimed observation window; a load-only trace cannot
supply it. The following DevTools calls describe one adapter; use equivalent actual trace and
network evidence from the configured observer.

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

Use code-intel queries, not tree scans: separate `arc-idx files 'vite.config' --format json`,
`arc-idx files 'next.config' --format json` (and the project's other configured build file names),
`arc-idx search 'from "lodash"'`, `'moment'`, `'core-js'`, `'sideEffects'`, `'browserslist'`.
Each candidate (wholesale imports, barrel re-exports, polyfill breadth, production source maps,
missing compression) is a hypothesis until the trace shows its cost.

## 5. Verdicts

| Claim | Evidence |
|---|---|
| Lab metric meets threshold | valid traces, confirmed statistic/sample requirement, profile, cache state and budget source |
| Field percentile meets threshold | representative field population/window and sample contract; independent percentile and budget source |
| INP meets threshold | valid actual interactions, named journey/window, metric source and confirmed budget |
| Specific resource costs time | insight output naming the resource and estimated savings |
| Recommendation is worth doing | non-zero estimated savings; 0 ms items are reported but not recommended |

`FAIL` items go to `arc:code` as a repair packet: metric, trace evidence, the resource or code
location, and the expected improvement. Recommendations are concrete ("hero.png 450 KB → WebP"),
never "optimize images".
