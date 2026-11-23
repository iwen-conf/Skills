package main

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
)

const snapshotCSP = `<meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data:;">`

// exportSnapshot writes a self-contained, read-only static HTML + snapshot.json
// Storyline Flowchart Workbench with UTF-8 charset in the first 1024 bytes.
func exportSnapshot(ledger *StoryLedger, s *Storyline, outDir string) error {
	if outDir == "" {
		return fmt.Errorf("--out directory is required")
	}
	if entries, err := os.ReadDir(outDir); err == nil && len(entries) > 0 {
		return fmt.Errorf("snapshot output directory %s must be absent or empty", outDir)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	checkpoints, err := ledger.ListCheckpoints(s.StoryID)
	if err != nil {
		return err
	}
	results, err := ledger.ListRouteResults(s.StoryID)
	if err != nil {
		return err
	}
	latestRun, _ := ledger.LatestRunID(s.StoryID)
	stepTraces, err := ledger.ListStepTraces(s.StoryID, latestRun, "")
	if err != nil {
		return err
	}
	completion, err := computeCompletion(ledger, s, nil)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"schema_version": 3,
		"kind":           "arc_storyline_snapshot",
		"storyline":      s,
		"latest_run_id":  latestRun,
		"checkpoints":    checkpoints,
		"route_results":  results,
		"step_traces":    stepTraces,
		"completion":     completion,
		"exported_at":    nowUTC(),
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "snapshot.json"), append(body, '\n'), 0o644); err != nil {
		return err
	}

	page := renderSnapshotHTML(s, completion, stepTraces, body)
	return os.WriteFile(filepath.Join(outDir, "index.html"), []byte(page), 0o644)
}

func renderSnapshotHTML(s *Storyline, c *CompletionReport, traces []StepTraceRecord, snapshotJSON []byte) string {
	// Ensure <meta charset="UTF-8"> is inside the first 1024 bytes before any embedded JSON.
	safeJSON := strings.ReplaceAll(string(snapshotJSON), "</script>", `<\/script>`)
	traceByRoute := map[string][]StepTraceRecord{}
	for _, tr := range traces {
		traceByRoute[tr.RouteID] = append(traceByRoute[tr.RouteID], tr)
	}

	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"zh-CN\">\n<head>\n")
	b.WriteString("<meta charset=\"UTF-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n")
	b.WriteString(snapshotCSP + "\n")
	fmt.Fprintf(&b, "<title>Arc Storyline Workbench — %s</title>\n", html.EscapeString(s.StoryID))
	b.WriteString(`<style>
body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,sans-serif;margin:0;background:#0f172a;color:#e2e8f0}
header{padding:16px 24px;border-bottom:1px solid #334155;display:flex;justify-content:space-between;align-items:center}
.badge{padding:4px 10px;border-radius:999px;font-weight:700;font-size:12px}
.PASS{background:#14532d;color:#86efac}.FAIL{background:#7f1d1d;color:#fca5a5}.BLOCKED,.NOT_RUN{background:#78350f;color:#fde68a}
main{display:grid;grid-template-columns:280px 1fr 320px;gap:16px;padding:16px}
section{background:#1e293b;border:1px solid #334155;border-radius:10px;padding:14px}
h2{margin:0 0 10px;font-size:14px;color:#93c5fd;text-transform:uppercase;letter-spacing:.04em}
.card{background:#0f172a;border:1px solid #334155;border-radius:8px;padding:10px;margin-bottom:8px;font-size:13px}
.step{border-top:1px dashed #334155;margin-top:8px;padding-top:6px;font-size:12px;color:#cbd5e1}
code{color:#f8fafc;background:#334155;padding:1px 5px;border-radius:4px}
.metrics{display:grid;grid-template-columns:repeat(4,1fr);gap:8px;margin-bottom:12px}
.metric{background:#0f172a;border:1px solid #334155;border-radius:8px;padding:10px;text-align:center}
.metric b{font-size:18px;display:block;color:#38bdf8}
@media(max-width:960px){main{grid-template-columns:1fr}}
</style>`)
	b.WriteString("\n</head>\n<body>\n")
	fmt.Fprintf(&b, "<header><div><strong>%s</strong> · focal <code>%s</code></div><span class=\"badge %s\">%s</span></header>\n",
		html.EscapeString(s.StoryID), html.EscapeString(s.FocalEntity), html.EscapeString(c.Verdict), html.EscapeString(c.Verdict))
	b.WriteString("<main>\n")

	// Left: World Flags & Checkpoints
	b.WriteString("<section><h2>World Flags</h2>\n")
	for _, k := range sortedKeys(s.WorldFlags) {
		fmt.Fprintf(&b, "<div class=\"card\"><code>%s</code><br><small>%s</small></div>\n",
			html.EscapeString(k), html.EscapeString(s.WorldFlags[k]))
	}
	b.WriteString("<h2>Checkpoints</h2>\n")
	for _, cp := range s.Checkpoints {
		fmt.Fprintf(&b, "<div class=\"card\"><strong>%s</strong> <small>← %s</small></div>\n",
			html.EscapeString(cp.SaveID), html.EscapeString(cp.Parent))
	}
	b.WriteString("</section>\n")

	// Center: Metrics & Branching Routes with 6-Dimension Step Traces
	b.WriteString("<section><h2>Completion &amp; 6-Dimension Route Traces</h2>\n")
	fmt.Fprintf(&b, `<div class="metrics">
<div class="metric"><b>%.0f%%</b>World Flags</div>
<div class="metric"><b>%.0f%%</b>Routes</div>
<div class="metric"><b>%.0f%%</b>Probes</div>
<div class="metric"><b>%.0f%%</b>Endings</div>
</div>`, c.WorldFlagBindingPct, c.RouteCoveragePct, c.ButterflyProbeHitPct, c.EndingUnlockPct)
	for _, r := range s.Routes {
		st := c.RouteStatuses[r.RouteID]
		if st == "" {
			st = "NOT_RUN"
		}
		fmt.Fprintf(&b, "<div class=\"card\"><div style=\"display:flex;justify-content:space-between\"><strong>%s</strong><span class=\"badge %s\">%s</span></div>",
			html.EscapeString(r.RouteID), html.EscapeString(st), html.EscapeString(st))
		fmt.Fprintf(&b, "<div><small>save: <code>%s</code> · ns: <code>%s</code> → <code>%s</code></small></div>",
			html.EscapeString(r.LoadSave), html.EscapeString(r.RuntimeNS), html.EscapeString(r.UnlocksEnding))
		for _, tr := range traceByRoute[r.RouteID] {
			fmt.Fprintf(&b, "<div class=\"step\"><strong>#%d %s</strong> [%s] · behavior <code>%s</code> · status <code>%s → %s</code>",
				tr.StepNum, html.EscapeString(tr.Choice), html.EscapeString(tr.Verdict),
				html.EscapeString(nonEmpty(tr.ExpectedBehavior, "DEFAULT")),
				html.EscapeString(tr.ExpectStatus), html.EscapeString(nonEmpty(tr.ActualStatus, "?")))
			if tr.DiffReason != "" {
				fmt.Fprintf(&b, "<br><small>%s</small>", html.EscapeString(tr.DiffReason))
			}
			b.WriteString("</div>")
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("</section>\n")

	// Right: Ending Gallery & History
	b.WriteString("<section><h2>Ending Gallery</h2>\n")
	unlocked := map[string]bool{}
	for _, e := range c.UnlockedEndings {
		unlocked[e] = true
	}
	for _, req := range s.RequiredEndings {
		cls := "NOT_RUN"
		label := "LOCKED"
		if unlocked[req] {
			cls = "PASS"
			label = "UNLOCKED"
		}
		fmt.Fprintf(&b, "<div class=\"card\" style=\"display:flex;justify-content:space-between\"><code>%s</code><span class=\"badge %s\">%s</span></div>\n",
			html.EscapeString(req), cls, label)
	}
	fmt.Fprintf(&b, "<div class=\"card\">Softlocks: <strong>%d</strong> · Historical FAILs preserved: <strong>%d</strong></div>\n",
		c.SoftlockCount, c.HistoricalFailCount)
	b.WriteString("</section>\n</main>\n")
	b.WriteString(`<script type="application/json" id="arc-snapshot">` + safeJSON + "</script>\n")
	b.WriteString("</body>\n</html>\n")
	return b.String()
}
