package main

import (
	"fmt"
	"strings"
)

type RouteExplainEntry struct {
	RouteID          string            `json:"route_id"`
	LoadSave         string            `json:"load_save"`
	RuntimeNS        string            `json:"runtime_ns"`
	Verdict          string            `json:"verdict"`
	SoftlockDetected bool              `json:"softlock_detected"`
	ProbesPassed     int               `json:"probes_passed"`
	ProbesTotal      int               `json:"probes_total"`
	UnlockedEnding   string            `json:"unlocked_ending,omitempty"`
	ReceiptSHA256    string            `json:"receipt_sha256,omitempty"`
	Steps            []StepTraceRecord `json:"steps"`
}

type ExplainReport struct {
	StoryID     string              `json:"story_id"`
	StorySHA256 string              `json:"story_sha256"`
	FocalEntity string              `json:"focal_entity"`
	RunID       string              `json:"run_id"`
	Completion  *CompletionReport   `json:"completion"`
	Routes      []RouteExplainEntry `json:"routes"`
}

// explainStory assembles the 6-dimension audit ledger for a stored storyline run.
func explainStory(ledger *StoryLedger, storyID, runID, routeFilter string) (*ExplainReport, error) {
	s, err := ledger.LoadStoryDefinition(storyID)
	if err != nil {
		return nil, fmt.Errorf("story %q not found in ledger: %w", storyID, err)
	}
	if runID == "" {
		runID, err = ledger.LatestRunID(storyID)
		if err != nil {
			return nil, err
		}
	}
	comp, err := computeCompletion(ledger, s, nil)
	if err != nil {
		return nil, err
	}
	allRouteRecs, err := ledger.ListRouteResults(storyID)
	if err != nil {
		return nil, err
	}
	routeRecMap := map[string]RouteResultRecord{}
	for _, rr := range allRouteRecs {
		if runID == "" || rr.RunID == runID {
			routeRecMap[rr.RouteID] = rr
		}
	}

	traces, err := ledger.ListStepTraces(storyID, runID, routeFilter)
	if err != nil {
		return nil, err
	}
	traceByRoute := map[string][]StepTraceRecord{}
	for _, tr := range traces {
		traceByRoute[tr.RouteID] = append(traceByRoute[tr.RouteID], tr)
	}

	rep := &ExplainReport{
		StoryID:     s.StoryID,
		StorySHA256: s.SHA256,
		FocalEntity: s.FocalEntity,
		RunID:       runID,
		Completion:  comp,
	}

	for _, r := range s.Routes {
		if routeFilter != "" && r.RouteID != routeFilter {
			continue
		}
		entry := RouteExplainEntry{
			RouteID:   r.RouteID,
			LoadSave:  r.LoadSave,
			RuntimeNS: r.RuntimeNS,
			Verdict:   "NOT_RUN",
			Steps:     traceByRoute[r.RouteID],
		}
		if rr, ok := routeRecMap[r.RouteID]; ok {
			entry.Verdict = rr.Verdict
			entry.SoftlockDetected = rr.SoftlockDetected
			entry.ProbesPassed = rr.ProbesPassed
			entry.ProbesTotal = rr.ProbesTotal
			entry.UnlockedEnding = rr.UnlockedEnding
			entry.ReceiptSHA256 = rr.ReceiptSHA256
		}
		if len(entry.Steps) == 0 {
			// Populate declared skeleton steps when run has not executed yet.
			for idx, st := range r.Sequence {
				sn := st.Step
				if sn == 0 {
					sn = idx + 1
				}
				entry.Steps = append(entry.Steps, StepTraceRecord{
					RouteID:          r.RouteID,
					StepNum:          sn,
					Choice:           st.Choice,
					ExpectedBehavior: st.ExpectedBehavior,
					Preconditions:    st.Preconditions,
					ExpectedInput:    st.effectiveExpectedInput(),
					BeforeState:      st.BeforeState,
					ExpectStatus:     st.ExpectStatus,
					ExpectResult:     st.ExpectResult,
					ExpectedProbes:   st.ButterflyProbes,
					ForbiddenEffects: st.ForbiddenEffects,
					AntiSoftlock:     st.AntiSoftlock,
					Verdict:          "NOT_RUN",
				})
			}
		}
		rep.Routes = append(rep.Routes, entry)
	}
	return rep, nil
}

func renderExplainMarkdown(rep *ExplainReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## 6-Dimension Verification Ledger: `%s` (run `%s`, verdict **%s**)\n",
		rep.StoryID, nonEmpty(rep.RunID, "NONE"), rep.Completion.Verdict)
	fmt.Fprintf(&b, "- Focal Entity: `%s` · Historical FAILs Preserved: **%d** · Softlocks: **%d**\n\n",
		rep.FocalEntity, rep.Completion.HistoricalFailCount, rep.Completion.SoftlockCount)

	for _, r := range rep.Routes {
		fmt.Fprintf(&b, "### Route `%s` — **%s** (save `%s`, ns `%s`, ending `%s`)\n",
			r.RouteID, r.Verdict, r.LoadSave, r.RuntimeNS, nonEmpty(r.UnlockedEnding, "LOCKED"))
		for _, st := range r.Steps {
			fmt.Fprintf(&b, "- **Step %d · `%s`** [%s] (%dms)\n", st.StepNum, st.Choice, st.Verdict, st.DurationMS)
			fmt.Fprintf(&b, "  1. Preconditions: expected=%s | actual=%s\n",
				marshalMapJSON(st.Preconditions), marshalMapJSON(st.PreconditionsActual))
			fmt.Fprintf(&b, "  2. Input: expected=%s | actual=%s\n",
				marshalMapJSON(st.ExpectedInput), marshalMapJSON(st.ActualInput))
			fmt.Fprintf(&b, "  3. State: before=%s -> after=%s\n",
				marshalMapJSON(st.BeforeState), marshalMapJSON(st.AfterState))
			fmt.Fprintf(&b, "  4. Execution: `%s`\n", nonEmpty(st.ExecutedCmd, "(not executed)"))
			fmt.Fprintf(&b, "  5. Behavior: `%s` (anti_softlock: `%s`)\n",
				nonEmpty(st.ExpectedBehavior, "UNSPECIFIED"), nonEmpty(st.AntiSoftlock, "n/a"))
			fmt.Fprintf(&b, "  6. Outcome & Probes: status(`%s` vs `%s`) · probes(%s vs %s)\n",
				st.ExpectStatus, nonEmpty(st.ActualStatus, "?"),
				marshalMapJSON(st.ExpectedProbes), marshalMapJSON(st.ActualProbes))
			if st.DiffReason != "" {
				fmt.Fprintf(&b, "  - Diff / Block Reason: **%s**\n", st.DiffReason)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
