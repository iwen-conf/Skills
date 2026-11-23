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
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	stepEvidencePrefix   = "[ARC_STEP_EVIDENCE]"
	softlockMarkerPrefix = "[ARC_SOFTLOCK]"
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
	RunID        string              `json:"run_id"`
	StoryID      string              `json:"story_id"`
	StorySHA256  string              `json:"story_sha256"`
	Verdict      string              `json:"verdict"`
	BlindMode    bool                `json:"blind_mode"`
	Parallel     bool                `json:"parallel"`
	Checkpoints  []CheckpointRecord  `json:"checkpoints"`
	RouteResults []RouteResultRecord `json:"route_results"`
	StepTraces   []StepTraceRecord   `json:"step_traces,omitempty"`
	Completion   *CompletionReport   `json:"completion"`
}

type routeExecutionOutcome struct {
	route  RouteResultRecord
	traces []StepTraceRecord
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
// executes routes sequentially or concurrently with 6-dimension step verification,
// persists immutable route + step receipts (preserving first FAIL), and returns summary.
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
	outcomes := make([]routeExecutionOutcome, len(selected))

	execOne := func(idx int, r Route) {
		outcomes[idx] = executeRoute(s, r, runID, absRoot, cpEngines[r.LoadSave], opts)
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
	results := make([]RouteResultRecord, 0, len(outcomes))
	var allTraces []StepTraceRecord
	for _, out := range outcomes {
		if err := ledger.AppendRouteResult(out.route); err != nil {
			return nil, err
		}
		for _, tr := range out.traces {
			if err := ledger.AppendStepTrace(tr); err != nil {
				return nil, err
			}
			allTraces = append(allTraces, tr)
		}
		results = append(results, out.route)
		if out.route.Verdict == "FAIL" {
			runVerdict = "FAIL"
		} else if out.route.Verdict != "PASS" && runVerdict == "PASS" {
			runVerdict = out.route.Verdict
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
		StepTraces:   allTraces,
		Completion:   comp,
	}, nil
}

func executeRoute(s *Storyline, r Route, runID, repoRoot, engine string, opts PlayOptions) routeExecutionOutcome {
	clonePath, workDir, err := cloneCheckpointWithDir(repoRoot, s.StoryID, r.LoadSave, r.RuntimeNS, engine, opts.PGURL)
	if err != nil {
		return routeExecutionOutcome{
			route: finalizeRouteRecord(s, r, runID, "BLOCKED", false, 0, 0, "", "clone checkpoint failed: "+err.Error()),
		}
	}

	varsFile := filepath.Join(workDir, "arc_vars.json")
	routeVars := map[string]any{}
	_ = os.WriteFile(varsFile, []byte("{}\n"), 0o644)

	baseEnv := append(os.Environ(),
		"SHARD_NS="+r.RuntimeNS,
		"ARC_STORY_ID="+s.StoryID,
		"ARC_ROUTE_ID="+r.RouteID,
		"ARC_LOAD_SAVE="+r.LoadSave,
		"ARC_CLONE_PATH="+clonePath,
		"ARC_VARS_FILE="+varsFile,
	)
	if opts.Blind {
		baseEnv = append(baseEnv, "ARC_BLIND_MODE=1", "ARC_BLIND_MASK="+strings.Join(s.BlindMask, ","))
	}

	probesTotal := 0
	for _, st := range r.Sequence {
		probesTotal += len(st.ButterflyProbes)
	}

	if opts.DryRun {
		return routeExecutionOutcome{
			route: finalizeRouteRecord(s, r, runID, "BLOCKED", false, probesTotal, 0, "", "dry-run only; no real commands executed"),
		}
	}

	hasStepCmds := false
	for _, st := range r.Sequence {
		if strings.TrimSpace(st.PrecheckCmd) != "" || strings.TrimSpace(st.Cmd) != "" || strings.TrimSpace(st.ProbeCmd) != "" {
			hasStepCmds = true
			break
		}
	}
	hasRouteVerifyCmd := strings.TrimSpace(r.VerifyCmd) != ""

	if !hasStepCmds && !hasRouteVerifyCmd {
		traces := make([]StepTraceRecord, 0, len(r.Sequence))
		for _, st := range r.Sequence {
			traces = append(traces, finalizeStepTrace(s, r, st, runID, "", 0, nil, "BLOCKED", "no executable command configured", ""))
		}
		return routeExecutionOutcome{
			route:  finalizeRouteRecord(s, r, runID, "BLOCKED", false, probesTotal, 0, "", "route has no executable verify_cmd or step cmd; real execution required for PASS"),
			traces: traces,
		}
	}

	var logBuf strings.Builder
	var traces []StepTraceRecord
	probesPassed := 0
	softlock := false

	// 1. Execute step-by-step commands when declared (never bypassed by route-level verify_cmd!).
	if hasStepCmds {
		for idx, st := range r.Sequence {
			stepNum := st.Step
			if stepNum == 0 {
				stepNum = idx + 1
			}
			reloadVarsFromFile(varsFile, routeVars)
			stepEnv := buildStepEnv(baseEnv, st, stepNum, routeVars)

			var stepOut strings.Builder
			var cmdsRun []string
			t0 := time.Now()

			// Dimension 1: Precondition check (failure -> BLOCKED, never false FAIL/PASS).
			if strings.TrimSpace(st.PrecheckCmd) != "" {
				cmdsRun = append(cmdsRun, "precheck: "+st.PrecheckCmd)
				out, pErr := runCommandCapture(repoRoot, st.PrecheckCmd, stepEnv)
				fmt.Fprintf(&logBuf, "[precheck %d:%s] %s\n%s\n", stepNum, st.Choice, st.PrecheckCmd, out)
				stepOut.WriteString(out + "\n")
				if pErr != nil {
					dur := time.Since(t0).Milliseconds()
					reason := fmt.Sprintf("precheck_cmd failed: %v", pErr)
					traces = append(traces, finalizeStepTrace(s, r, st, runID, strings.Join(cmdsRun, " && "), dur, nil, "BLOCKED", reason, strings.TrimSpace(stepOut.String())))
					return routeExecutionOutcome{
						route:  finalizeRouteRecord(s, r, runID, "BLOCKED", false, probesTotal, probesPassed, "", logBuf.String()),
						traces: traces,
					}
				}
			}

			// Dimension 4: Step execution command.
			if strings.TrimSpace(st.Cmd) != "" {
				cmdsRun = append(cmdsRun, st.Cmd)
				out, runErr := runCommandCapture(repoRoot, st.Cmd, stepEnv)
				fmt.Fprintf(&logBuf, "[step %d:%s] %s\n%s\n", stepNum, st.Choice, st.Cmd, out)
				stepOut.WriteString(out + "\n")
				if runErr != nil {
					dur := time.Since(t0).Milliseconds()
					ev := mergeAllStepEvidence(parseStepEvidence(stepOut.String()))
					if hasSoftlockSignal(out, true, ev) {
						softlock = true
					}
					traces = append(traces, finalizeStepTrace(s, r, st, runID, strings.Join(cmdsRun, " && "), dur, ev, "FAIL", fmt.Sprintf("step command exited non-zero: %v", runErr), strings.TrimSpace(stepOut.String())))
					return routeExecutionOutcome{
						route:  finalizeRouteRecord(s, r, runID, "FAIL", softlock, probesTotal, probesPassed, "", logBuf.String()),
						traces: traces,
					}
				}
			}

			// Dimension 6: Independent probe command.
			if strings.TrimSpace(st.ProbeCmd) != "" {
				cmdsRun = append(cmdsRun, "probe: "+st.ProbeCmd)
				reloadVarsFromFile(varsFile, routeVars)
				stepEnv = buildStepEnv(baseEnv, st, stepNum, routeVars)
				out, pErr := runCommandCapture(repoRoot, st.ProbeCmd, stepEnv)
				fmt.Fprintf(&logBuf, "[probe %d] %s\n%s\n", stepNum, st.ProbeCmd, out)
				stepOut.WriteString(out + "\n")
				if pErr != nil {
					dur := time.Since(t0).Milliseconds()
					ev := mergeAllStepEvidence(parseStepEvidence(stepOut.String()))
					if hasSoftlockSignal(out, true, ev) {
						softlock = true
					}
					traces = append(traces, finalizeStepTrace(s, r, st, runID, strings.Join(cmdsRun, " && "), dur, ev, "FAIL", fmt.Sprintf("probe_cmd exited non-zero: %v", pErr), strings.TrimSpace(stepOut.String())))
					return routeExecutionOutcome{
						route:  finalizeRouteRecord(s, r, runID, "FAIL", softlock, probesTotal, probesPassed, "", logBuf.String()),
						traces: traces,
					}
				}
			}

			dur := time.Since(t0).Milliseconds()
			rawStepOut := strings.TrimSpace(stepOut.String())
			mergedEv := mergeAllStepEvidence(parseStepEvidence(rawStepOut))
			if hasSoftlockSignal(rawStepOut, false, mergedEv) {
				softlock = true
			}
			if mergedEv != nil && len(mergedEv.ExtractedVars) > 0 {
				for k, v := range mergedEv.ExtractedVars {
					routeVars[k] = v
				}
				saveVarsToFile(varsFile, routeVars)
			}

			stepVerdict, diffReason := evaluateStepEvidence(st, mergedEv)
			if softlock && stepVerdict == "PASS" {
				stepVerdict = "FAIL"
				diffReason = "explicit softlock marker reported in step evidence/output"
			}
			traces = append(traces, finalizeStepTrace(s, r, st, runID, strings.Join(cmdsRun, " && "), dur, mergedEv, stepVerdict, diffReason, rawStepOut))

			if stepVerdict != "PASS" {
				return routeExecutionOutcome{
					route:  finalizeRouteRecord(s, r, runID, stepVerdict, softlock, probesTotal, probesPassed, "", logBuf.String()),
					traces: traces,
				}
			}
			probesPassed += len(st.ButterflyProbes)
		}
	}

	// 2. Execute route-level verify_cmd (either as standalone runner or post-sequence assertion).
	if hasRouteVerifyCmd {
		reloadVarsFromFile(varsFile, routeVars)
		varsJSON, _ := json.Marshal(routeVars)
		vEnv := append(append([]string{}, baseEnv...), "ARC_VARS_JSON="+string(varsJSON))
		t0 := time.Now()
		out, runErr := runCommandCapture(repoRoot, r.VerifyCmd, vEnv)
		dur := time.Since(t0).Milliseconds()
		logBuf.WriteString("[verify_cmd] " + r.VerifyCmd + "\n" + out + "\n")

		evByStep := map[int]*StepEvidencePayload{}
		evs := parseStepEvidence(out)
		mergedVerifyEv := mergeAllStepEvidence(evs)
		for i := range evs {
			sn := evs[i].Step
			if sn <= 0 {
				sn = i + 1
			}
			evByStep[sn] = mergeStepEvidence(evByStep[sn], &evs[i])
		}
		if hasSoftlockSignal(out, runErr != nil, mergedVerifyEv) {
			softlock = true
		}

		if runErr != nil {
			if !hasStepCmds {
				for idx, st := range r.Sequence {
					sn := st.Step
					if sn == 0 {
						sn = idx + 1
					}
					traces = append(traces, finalizeStepTrace(s, r, st, runID, r.VerifyCmd, dur, evByStep[sn], "FAIL", fmt.Sprintf("verify_cmd exited non-zero: %v", runErr), out))
				}
			} else if len(traces) > 0 {
				last := &traces[len(traces)-1]
				last.Verdict = "FAIL"
				last.DiffReason = fmt.Sprintf("verify_cmd exited non-zero: %v", runErr)
				if out != "" {
					if last.RawOutput != "" {
						last.RawOutput = last.RawOutput + "\n" + out
					} else {
						last.RawOutput = out
					}
				}
				last.TraceSHA256 = computeStepTraceSHA256(*last)
			}
			return routeExecutionOutcome{
				route:  finalizeRouteRecord(s, r, runID, "FAIL", softlock, probesTotal, 0, "", logBuf.String()),
				traces: traces,
			}
		}

		if !hasStepCmds {
			for idx, st := range r.Sequence {
				sn := st.Step
				if sn == 0 {
					sn = idx + 1
				}
				ev := evByStep[sn]
				stepVerdict, diffReason := evaluateStepEvidence(st, ev)
				if softlock && stepVerdict == "PASS" {
					stepVerdict = "FAIL"
					diffReason = "explicit softlock marker reported in verify_cmd output"
				}
				traces = append(traces, finalizeStepTrace(s, r, st, runID, r.VerifyCmd, dur, ev, stepVerdict, diffReason, out))
				if stepVerdict != "PASS" {
					return routeExecutionOutcome{
						route:  finalizeRouteRecord(s, r, runID, stepVerdict, softlock, probesTotal, probesPassed, "", logBuf.String()),
						traces: traces,
					}
				}
				probesPassed += len(st.ButterflyProbes)
			}
		}
	}

	if softlock {
		return routeExecutionOutcome{
			route:  finalizeRouteRecord(s, r, runID, "FAIL", true, probesTotal, probesPassed, "", logBuf.String()),
			traces: traces,
		}
	}

	return routeExecutionOutcome{
		route:  finalizeRouteRecord(s, r, runID, "PASS", false, probesTotal, probesPassed, r.UnlocksEnding, logBuf.String()),
		traces: traces,
	}
}

func buildStepEnv(baseEnv []string, st RouteStep, stepNum int, routeVars map[string]any) []string {
	preJSON, _ := json.Marshal(st.Preconditions)
	inJSON, _ := json.Marshal(st.effectiveExpectedInput())
	probesJSON, _ := json.Marshal(st.ButterflyProbes)
	varsJSON, _ := json.Marshal(routeVars)

	env := append([]string{}, baseEnv...)
	env = append(env,
		"ARC_STEP_NUM="+strconv.Itoa(stepNum),
		"ARC_STEP_CHOICE="+st.Choice,
		"ARC_EXPECTED_BEHAVIOR="+st.ExpectedBehavior,
		"ARC_PRECONDITIONS_JSON="+string(preJSON),
		"ARC_EXPECTED_INPUT_JSON="+string(inJSON),
		"ARC_EXPECTED_STATUS="+st.ExpectStatus,
		"ARC_EXPECTED_PROBES_JSON="+string(probesJSON),
		"ARC_VARS_JSON="+string(varsJSON),
	)
	for _, k := range sortedKeys(routeVars) {
		envKey := "ARC_VAR_" + strings.ToUpper(sanitizeNS(k))
		env = append(env, fmt.Sprintf("%s=%v", envKey, routeVars[k]))
	}
	return env
}

func reloadVarsFromFile(path string, dst map[string]any) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	if err := dec.Decode(&m); err == nil {
		for k, v := range m {
			dst[k] = v
		}
	}
}

func saveVarsToFile(path string, vars map[string]any) {
	b, err := json.MarshalIndent(vars, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, append(b, '\n'), 0o644)
	}
}

func parseStepEvidence(output string) []StepEvidencePayload {
	var out []StepEvidencePayload
	for _, line := range strings.Split(output, "\n") {
		idx := strings.Index(line, stepEvidencePrefix)
		if idx < 0 {
			continue
		}
		rawJSON := strings.TrimSpace(line[idx+len(stepEvidencePrefix):])
		var payload StepEvidencePayload
		dec := json.NewDecoder(strings.NewReader(rawJSON))
		dec.UseNumber()
		if err := dec.Decode(&payload); err == nil {
			out = append(out, payload)
		}
	}
	return out
}

func mergeAllStepEvidence(evs []StepEvidencePayload) *StepEvidencePayload {
	var merged *StepEvidencePayload
	for i := range evs {
		merged = mergeStepEvidence(merged, &evs[i])
	}
	return merged
}

// hasSoftlockSignal distinguishes real softlocks from benign passing log text
// (such as "cooling revert verified without softlock" or "PASS: TestSoftlockGuard").
// Free-text "SOFTLOCK" only flags a softlock when the command exited non-zero;
// exit-0 commands require an explicit [ARC_SOFTLOCK] marker or structured softlock evidence.
func hasSoftlockSignal(output string, cmdFailed bool, ev *StepEvidencePayload) bool {
	if ev != nil {
		if ev.Softlock {
			return true
		}
		for _, eff := range ev.ObservedEffects {
			upEff := strings.ToUpper(strings.TrimSpace(eff))
			if upEff == "SOFTLOCK" || upEff == "SOFTLOCK_DETECTED" {
				return true
			}
		}
	}
	upOut := strings.ToUpper(output)
	if strings.Contains(upOut, softlockMarkerPrefix) {
		return true
	}
	if cmdFailed && strings.Contains(upOut, "SOFTLOCK") {
		return true
	}
	return false
}

func mergeStepEvidence(base, next *StepEvidencePayload) *StepEvidencePayload {
	if base == nil {
		if next == nil {
			return nil
		}
		cp := *next
		return &cp
	}
	if next == nil {
		return base
	}
	if next.Step != 0 {
		base.Step = next.Step
	}
	if len(next.PreconditionsActual) > 0 {
		base.PreconditionsActual = mergeMaps(base.PreconditionsActual, next.PreconditionsActual)
	}
	if len(next.ActualInput) > 0 {
		base.ActualInput = mergeMaps(base.ActualInput, next.ActualInput)
	}
	if len(next.BeforeState) > 0 {
		base.BeforeState = mergeMaps(base.BeforeState, next.BeforeState)
	}
	if len(next.AfterState) > 0 {
		base.AfterState = mergeMaps(base.AfterState, next.AfterState)
	}
	if next.ActualBehavior != "" {
		base.ActualBehavior = next.ActualBehavior
	}
	if next.ActualStatus != "" {
		base.ActualStatus = next.ActualStatus
	}
	if len(next.ActualResult) > 0 {
		base.ActualResult = mergeMaps(base.ActualResult, next.ActualResult)
	}
	if len(next.ActualProbes) > 0 {
		base.ActualProbes = mergeMaps(base.ActualProbes, next.ActualProbes)
	}
	if len(next.ObservedEffects) > 0 {
		base.ObservedEffects = append(base.ObservedEffects, next.ObservedEffects...)
	}
	if len(next.ExtractedVars) > 0 {
		base.ExtractedVars = mergeMaps(base.ExtractedVars, next.ExtractedVars)
	}
	if next.Softlock {
		base.Softlock = true
	}
	return base
}

func mergeMaps(a, b map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// evaluateStepEvidence performs strict kernel-level Expected-vs-Actual comparison whenever
// structured [ARC_STEP_EVIDENCE] fields are emitted:
//   - Precondition mismatch -> BLOCKED
//   - Status / Result / ButterflyProbes mismatch or ForbiddenEffects hit -> FAIL
func evaluateStepEvidence(st RouteStep, ev *StepEvidencePayload) (verdict string, diffReason string) {
	if ev == nil {
		return "PASS", ""
	}
	if ev.Softlock {
		return "FAIL", "softlock reported in step evidence"
	}

	// 1. Preconditions: unmet preconditions classify as BLOCKED (env/fixture setup mismatch).
	if len(st.Preconditions) > 0 && len(ev.PreconditionsActual) > 0 {
		for _, k := range sortedKeys(st.Preconditions) {
			expVal := st.Preconditions[k]
			actVal, ok := ev.PreconditionsActual[k]
			if !ok || !valuesMatch(expVal, actVal) {
				return "BLOCKED", fmt.Sprintf("precondition %q mismatch: expected %v, actual %v", k, expVal, actVal)
			}
		}
	}

	var diffs []string

	// 2. Status check.
	if strings.TrimSpace(st.ExpectStatus) != "" && strings.TrimSpace(ev.ActualStatus) != "" {
		if strings.TrimSpace(st.ExpectStatus) != strings.TrimSpace(ev.ActualStatus) {
			diffs = append(diffs, fmt.Sprintf("status mismatch: expected %q, actual %q", st.ExpectStatus, ev.ActualStatus))
		}
	}

	// 3. Expected result fields check.
	if len(st.ExpectResult) > 0 && ev.ActualResult != nil {
		for _, k := range sortedKeys(st.ExpectResult) {
			expVal := st.ExpectResult[k]
			actVal, ok := ev.ActualResult[k]
			if !ok || !valuesMatch(expVal, actVal) {
				diffs = append(diffs, fmt.Sprintf("result[%s] mismatch: expected %v, actual %v", k, expVal, actVal))
			}
		}
	}

	// 4. Butterfly probes check (coupled side-effect verification).
	if len(st.ButterflyProbes) > 0 && ev.ActualProbes != nil {
		for _, k := range sortedKeys(st.ButterflyProbes) {
			expVal := st.ButterflyProbes[k]
			actVal, ok := ev.ActualProbes[k]
			if !ok || !valuesMatch(expVal, actVal) {
				diffs = append(diffs, fmt.Sprintf("butterfly_probe[%s] mismatch: expected %v, actual %v", k, expVal, actVal))
			}
		}
	}

	// 5. Forbidden effects check (zero-tolerance side effects).
	if len(st.ForbiddenEffects) > 0 && len(ev.ObservedEffects) > 0 {
		obsSet := map[string]bool{}
		for _, eff := range ev.ObservedEffects {
			obsSet[strings.TrimSpace(eff)] = true
		}
		for _, forb := range st.ForbiddenEffects {
			if obsSet[strings.TrimSpace(forb)] {
				diffs = append(diffs, fmt.Sprintf("forbidden_effect triggered: %q", forb))
			}
		}
	}

	if len(diffs) > 0 {
		return "FAIL", strings.Join(diffs, "; ")
	}
	return "PASS", ""
}

func valuesMatch(expected, actual any) bool {
	eb, err1 := json.Marshal(expected)
	ab, err2 := json.Marshal(actual)
	if err1 == nil && err2 == nil && string(eb) == string(ab) {
		return true
	}
	return fmt.Sprintf("%v", expected) == fmt.Sprintf("%v", actual)
}

func finalizeStepTrace(s *Storyline, r Route, st RouteStep, runID, executedCmd string, durationMS int64, ev *StepEvidencePayload, verdict, diffReason, rawOutput string) StepTraceRecord {
	stepNum := st.Step
	if stepNum == 0 {
		stepNum = 1
	}
	tr := StepTraceRecord{
		RunID:            runID,
		StoryID:          s.StoryID,
		StorySHA256:      s.SHA256,
		RouteID:          r.RouteID,
		StepNum:          stepNum,
		Choice:           st.Choice,
		ExpectedBehavior: st.ExpectedBehavior,
		Preconditions:    st.Preconditions,
		ExpectedInput:    st.effectiveExpectedInput(),
		BeforeState:      st.BeforeState,
		ExecutedCmd:      executedCmd,
		DurationMS:       durationMS,
		ExpectStatus:     st.ExpectStatus,
		ExpectResult:     st.ExpectResult,
		ExpectedProbes:   st.ButterflyProbes,
		ForbiddenEffects: st.ForbiddenEffects,
		AntiSoftlock:     st.AntiSoftlock,
		Verdict:          verdict,
		DiffReason:       diffReason,
		RawOutput:        rawOutput,
		CreatedAt:        nowUTC(),
	}
	if ev != nil {
		tr.PreconditionsActual = ev.PreconditionsActual
		tr.ActualInput = ev.ActualInput
		if len(ev.BeforeState) > 0 {
			tr.BeforeState = ev.BeforeState
		}
		tr.AfterState = ev.AfterState
		tr.ActualStatus = ev.ActualStatus
		tr.ActualResult = ev.ActualResult
		tr.ActualProbes = ev.ActualProbes
		tr.ObservedEffects = ev.ObservedEffects
	}
	if tr.ActualStatus == "" && verdict == "PASS" && st.ExpectStatus != "" {
		tr.ActualStatus = st.ExpectStatus
	}
	tr.TraceSHA256 = computeStepTraceSHA256(tr)
	return tr
}

func computeStepTraceSHA256(tr StepTraceRecord) string {
	tr.TraceSHA256 = ""
	payload, _ := json.Marshal(tr)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
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
