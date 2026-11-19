package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type PlayOptions struct {
	Parallel    bool
	RouteFilter string
	Blind       bool
	DryRun      bool
	RepoRoot    string
	Engine      string
	PGURL       string
}

type PlaySummary struct {
	RunID         string              `json:"run_id"`
	StoryID       string              `json:"story_id"`
	StorySHA256   string              `json:"story_sha256"`
	Verdict       string              `json:"verdict"`
	BlindMode     bool                `json:"blind_mode"`
	Parallel      bool                `json:"parallel"`
	Checkpoints   []CheckpointRecord  `json:"checkpoints"`
	RouteResults  []RouteResultRecord `json:"route_results"`
	Completion    *CompletionReport   `json:"completion"`
}

func newRunID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "SR-" + hex.EncodeToString(b[:])
}

func currentGitHead(repoRoot string) string {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "NO_GIT"
	}
	return strings.TrimSpace(string(out))
}

// playStory builds checkpoints in topological order, clones isolated shard namespaces,
// executes routes sequentially or concurrently, persists immutable receipts (preserving
// first FAIL), and returns the completion summary.
func playStory(ledger *StoryLedger, s *Storyline, opts PlayOptions) (*PlaySummary, error) {
	lint := lintStory(s, nil, 10)
	if lint.Status != "PASS" {
		return nil, fmt.Errorf("storyline lint failed (%d errors): %s", lint.TotalErrors, lint.Errors[0].Message)
	}
	if opts.RepoRoot == "" {
		opts.RepoRoot = "."
	}
	absRoot, err := filepath.Abs(opts.RepoRoot)
	if err != nil {
		return nil, err
	}
	if err := ledger.SaveStoryDefinition(s); err != nil {
		return nil, err
	}

	cpRecords, err := buildCheckpoints(ledger, s, absRoot, opts.Engine, "", opts.PGURL)
	if err != nil {
		return nil, err
	}
	cpEngines := map[string]string{}
	for _, cp := range cpRecords {
		cpEngines[cp.SaveID] = cp.Engine
	}

	var selected []Route
	for _, r := range s.Routes {
		if opts.RouteFilter != "" && r.RouteID != opts.RouteFilter {
			continue
		}
		selected = append(selected, r)
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no matching routes to play (filter=%q)", opts.RouteFilter)
	}

	runID := newRunID()
	startedAt := nowUTC()
	results := make([]RouteResultRecord, len(selected))

	execOne := func(idx int, r Route) {
		results[idx] = executeRoute(s, r, runID, absRoot, cpEngines[r.LoadSave], opts)
	}

	if opts.Parallel && len(selected) > 1 {
		var wg sync.WaitGroup
		for i, r := range selected {
			wg.Add(1)
			go func(idx int, route Route) {
				defer wg.Done()
				execOne(idx, route)
			}(i, r)
		}
		wg.Wait()
	} else {
		for i, r := range selected {
			execOne(i, r)
		}
	}

	runVerdict := "PASS"
	for _, rec := range results {
		if err := ledger.AppendRouteResult(rec); err != nil {
			return nil, err
		}
		if rec.Verdict == "FAIL" {
			runVerdict = "FAIL"
		} else if rec.Verdict != "PASS" && runVerdict == "PASS" {
			runVerdict = rec.Verdict
		}
	}

	if err := ledger.RecordRun(runID, s.StoryID, s.SHA256, currentGitHead(absRoot), opts.Blind, runVerdict, startedAt); err != nil {
		return nil, err
	}

	comp, err := computeCompletion(ledger, s, nil)
	if err != nil {
		return nil, err
	}
	return &PlaySummary{
		RunID:        runID,
		StoryID:      s.StoryID,
		StorySHA256:  s.SHA256,
		Verdict:      runVerdict,
		BlindMode:    opts.Blind,
		Parallel:     opts.Parallel,
		Checkpoints:  cpRecords,
		RouteResults: results,
		Completion:   comp,
	}, nil
}

func executeRoute(s *Storyline, r Route, runID, repoRoot, engine string, opts PlayOptions) RouteResultRecord {
	clonePath, err := cloneCheckpoint(repoRoot, s.StoryID, r.LoadSave, r.RuntimeNS, engine, opts.PGURL)
	if err != nil {
		return finalizeRouteRecord(s, r, runID, "BLOCKED", false, 0, 0, "", "clone checkpoint failed: "+err.Error())
	}

	env := append(os.Environ(),
		"SHARD_NS="+r.RuntimeNS,
		"ARC_STORY_ID="+s.StoryID,
		"ARC_ROUTE_ID="+r.RouteID,
		"ARC_LOAD_SAVE="+r.LoadSave,
		"ARC_CLONE_PATH="+clonePath,
	)
	if opts.Blind {
		env = append(env, "ARC_BLIND_MODE=1", "ARC_BLIND_MASK="+strings.Join(s.BlindMask, ","))
	}

	var logBuf strings.Builder
	probesTotal := 0
	probesPassed := 0
	verdict := "PASS"
	softlock := false

	// Count total declared butterfly probes across steps.
	for _, st := range r.Sequence {
		probesTotal += len(st.ButterflyProbes)
	}

	if opts.DryRun {
		// Dry-run validates wiring without running external shell commands, never claims PASS.
		return finalizeRouteRecord(s, r, runID, "BLOCKED", false, probesTotal, 0, "", "dry-run only; no real commands executed")
	}

	// Execute route-level verify_cmd if declared, else step-by-step commands.
	executedAnyCmd := false
	if strings.TrimSpace(r.VerifyCmd) != "" {
		executedAnyCmd = true
		out, runErr := runCommandCapture(repoRoot, r.VerifyCmd, env)
		logBuf.WriteString("[verify_cmd] " + r.VerifyCmd + "\n" + out + "\n")
		if runErr != nil {
			verdict = "FAIL"
			if strings.Contains(strings.ToUpper(out), "SOFTLOCK") {
				softlock = true
			}
			return finalizeRouteRecord(s, r, runID, verdict, softlock, probesTotal, 0, "", logBuf.String())
		}
		probesPassed = probesTotal
	} else {
		for _, st := range r.Sequence {
			if strings.TrimSpace(st.Cmd) != "" {
				executedAnyCmd = true
				out, runErr := runCommandCapture(repoRoot, st.Cmd, env)
				fmt.Fprintf(&logBuf, "[step %d:%s] %s\n%s\n", st.Step, st.Choice, st.Cmd, out)
				if runErr != nil {
					verdict = "FAIL"
					if strings.Contains(strings.ToUpper(out), "SOFTLOCK") {
						softlock = true
					}
					return finalizeRouteRecord(s, r, runID, verdict, softlock, probesTotal, probesPassed, "", logBuf.String())
				}
			}
			if strings.TrimSpace(st.ProbeCmd) != "" {
				executedAnyCmd = true
				out, pErr := runCommandCapture(repoRoot, st.ProbeCmd, env)
				fmt.Fprintf(&logBuf, "[probe %d] %s\n%s\n", st.Step, st.ProbeCmd, out)
				if pErr != nil {
					verdict = "FAIL"
					return finalizeRouteRecord(s, r, runID, verdict, softlock, probesTotal, probesPassed, "", logBuf.String())
				}
				probesPassed += len(st.ButterflyProbes)
			} else if executedAnyCmd {
				probesPassed += len(st.ButterflyProbes)
			}
		}
	}

	if !executedAnyCmd {
		// Anti-theater rule: a route with zero executable commands cannot claim PASS.
		return finalizeRouteRecord(s, r, runID, "BLOCKED", false, probesTotal, 0, "",
			"route has no executable verify_cmd or step cmd; real execution required for PASS")
	}

	unlocked := ""
	if verdict == "PASS" && !softlock {
		unlocked = r.UnlocksEnding
	}
	return finalizeRouteRecord(s, r, runID, verdict, softlock, probesTotal, probesPassed, unlocked, logBuf.String())
}

func runCommandCapture(dir, command string, env []string) (string, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func finalizeRouteRecord(s *Storyline, r Route, runID, verdict string, softlock bool, probesTotal, probesPassed int, unlocked, rawOutput string) RouteResultRecord {
	rec := RouteResultRecord{
		RunID:            runID,
		StoryID:          s.StoryID,
		StorySHA256:      s.SHA256,
		RouteID:          r.RouteID,
		LoadSave:         r.LoadSave,
		RuntimeNS:        r.RuntimeNS,
		Verdict:          verdict,
		SoftlockDetected: softlock,
		ProbesTotal:      probesTotal,
		ProbesPassed:     probesPassed,
		UnlockedEnding:   unlocked,
		RawOutput:        rawOutput,
		CreatedAt:        nowUTC(),
	}
	payload, _ := json.Marshal(rec)
	sum := sha256.Sum256(payload)
	rec.ReceiptSHA256 = hex.EncodeToString(sum[:])
	return rec
}

// computeCompletion evaluates the 4 hard completion gates from immutable ledger history.
func computeCompletion(ledger *StoryLedger, s *Storyline, worldScan *WorldScanResult) (*CompletionReport, error) {
	records, err := ledger.ListRouteResults(s.StoryID)
	if err != nil {
		return nil, err
	}

	// Latest verdict per route for the current story SHA + historical fail counter.
	latestByRoute := map[string]RouteResultRecord{}
	historicalFails := 0
	for _, rec := range records {
		if rec.Verdict == "FAIL" {
			historicalFails++
		}
		if rec.StorySHA256 == s.SHA256 {
			latestByRoute[rec.RouteID] = rec
		}
	}

	routeStatuses := map[string]string{}
	routesPassed := 0
	unresolvedFails := 0
	softlockCount := 0
	probesTotal := 0
	probesPassed := 0
	unlockedMap := map[string]bool{}

	for _, r := range s.Routes {
		declProbes := 0
		for _, st := range r.Sequence {
			declProbes += len(st.ButterflyProbes)
		}
		probesTotal += declProbes
		rec, ok := latestByRoute[r.RouteID]
		if !ok {
			routeStatuses[r.RouteID] = "NOT_RUN"
			continue
		}
		routeStatuses[r.RouteID] = rec.Verdict
		if rec.SoftlockDetected {
			softlockCount++
		}
		switch rec.Verdict {
		case "PASS":
			routesPassed++
			probesPassed += min(declProbes, rec.ProbesPassed)
			if rec.UnlockedEnding != "" {
				unlockedMap[rec.UnlockedEnding] = true
			}
		case "FAIL":
			unresolvedFails++
		}
	}

	// World flag binding %.
	worldPct := 100.0
	if len(s.WorldFlags) == 0 {
		worldPct = 0.0
	} else if worldScan != nil && len(worldScan.Candidates) > 0 {
		lint := lintStory(s, worldScan, 100)
		missing := 0
		for _, e := range lint.Errors {
			if e.Code == "UNBOUND_WORLD_FLAG" {
				missing++
			}
		}
		bound := len(worldScan.Candidates) - missing
		if bound < 0 {
			bound = 0
		}
		worldPct = float64(bound) * 100.0 / float64(len(worldScan.Candidates))
	}

	routePct := 0.0
	if len(s.Routes) > 0 {
		routePct = float64(routesPassed) * 100.0 / float64(len(s.Routes))
	}

	probePct := 100.0
	if probesTotal > 0 {
		probePct = float64(probesPassed) * 100.0 / float64(probesTotal)
	}

	var unlockedList, missingList []string
	for _, req := range s.RequiredEndings {
		if unlockedMap[req] {
			unlockedList = append(unlockedList, req)
		} else {
			missingList = append(missingList, req)
		}
	}
	sort.Strings(unlockedList)
	sort.Strings(missingList)

	endingPct := 0.0
	if len(s.RequiredEndings) > 0 {
		endingPct = float64(len(unlockedList)) * 100.0 / float64(len(s.RequiredEndings))
	}

	verdict := "PASS"
	reason := ""
	switch {
	case len(latestByRoute) == 0:
		verdict = "NOT_RUN"
		reason = "no route executions recorded for current storyline revision"
	case softlockCount > 0:
		verdict = "FAIL"
		reason = fmt.Sprintf("%d softlock violation(s) detected", softlockCount)
	case unresolvedFails > 0:
		verdict = "FAIL"
		reason = fmt.Sprintf("%d route(s) currently failing", unresolvedFails)
	case worldPct < 100 || routePct < 100 || probePct < 100 || endingPct < 100:
		verdict = "BLOCKED"
		reason = fmt.Sprintf("incomplete coverage (world=%.0f%% routes=%.0f%% probes=%.0f%% endings=%.0f%%)",
			worldPct, routePct, probePct, endingPct)
	}

	return &CompletionReport{
		StoryID:              s.StoryID,
		StorySHA256:          s.SHA256,
		Verdict:              verdict,
		WorldFlagBindingPct:  worldPct,
		RouteCoveragePct:     routePct,
		ButterflyProbeHitPct: probePct,
		EndingUnlockPct:      endingPct,
		SoftlockCount:        softlockCount,
		UnresolvedFailCount:  unresolvedFails,
		HistoricalFailCount:  historicalFails,
		UnlockedEndings:      unlockedList,
		MissingEndings:       missingList,
		RouteStatuses:        routeStatuses,
		Reason:               reason,
	}, nil
}
