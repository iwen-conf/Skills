package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

type CoverageMeasure struct {
	Total    int     `json:"total"`
	Passed   int     `json:"passed"`
	Excluded int     `json:"excluded"`
	Percent  float64 `json:"percent"`
}
type BusinessCoverage struct {
	Obligations  CoverageMeasure `json:"obligations"`
	Capabilities CoverageMeasure `json:"capabilities"`
	Behaviors    CoverageMeasure `json:"behaviors"`
	Branches     CoverageMeasure `json:"branches"`
	Invariants   CoverageMeasure `json:"invariants"`
}
type BusinessDiagnostic struct {
	Code      string   `json:"code"`
	Severity  string   `json:"severity"`
	Subjects  []string `json:"subjects"`
	Reason    string   `json:"reason"`
	SourceRef string   `json:"source_ref,omitempty"`
}
type ObligationStatus struct {
	ModelObligation
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	Run             string `json:"run,omitempty"`
	HistoryFailures int    `json:"history_failures"`
	Planned         bool   `json:"planned"`
}
type NodeReadiness struct {
	Described            bool   `json:"described"`
	Implemented          bool   `json:"implemented"`
	Verified             bool   `json:"verified"`
	Live                 bool   `json:"live"`
	ImplementationStatus string `json:"implementation_status"`
	DeploymentStatus     string `json:"deployment_status"`
}
type BusinessNodeStatus struct {
	Status          string        `json:"status"`
	Reason          string        `json:"reason"`
	Required        int           `json:"required"`
	Passed          int           `json:"passed"`
	Excluded        int           `json:"excluded"`
	HistoryFailures int           `json:"history_failures"`
	Readiness       NodeReadiness `json:"readiness"`
}
type ModelAnalysis struct {
	Graph            string                        `json:"graph"`
	Environment      string                        `json:"environment,omitempty"`
	Round            string                        `json:"round,omitempty"`
	Verdict          string                        `json:"verdict"`
	Reason           string                        `json:"reason"`
	Coverage         BusinessCoverage              `json:"coverage"`
	Obligations      []ObligationStatus            `json:"obligations"`
	Nodes            map[string]BusinessNodeStatus `json:"node_status"`
	Diagnostics      []BusinessDiagnostic          `json:"diagnostics"`
	Context          ProofContext                  `json:"context"`
	CatalogApproved  bool                          `json:"catalog_approved"`
	UnconfirmedRules int                           `json:"unconfirmed_rules"`
	ActualTasks      int                           `json:"actual_tasks"`
	InvalidTasks     int                           `json:"invalid_tasks"`
	HistoryFailures  int                           `json:"history_failures"`
}

func aggregatedStatus(statuses []string) (string, string) {
	counts := map[string]int{}
	for _, status := range statuses {
		counts[status]++
	}
	if counts["FAIL"] > 0 {
		return "FAIL", "an independently observed required check failed"
	}
	if counts["FLAKY"] > 0 {
		return "FLAKY", "both failure and success occurred in this qualification round"
	}
	if counts["STALE"] > 0 {
		return "STALE", "evidence or qualification inputs changed"
	}
	if counts["BLOCKED"] > 0 {
		return "BLOCKED", "an applicable obligation or execution prerequisite is blocked"
	}
	applicable := len(statuses) - counts["EXCLUDED"]
	if applicable > 0 && counts["PASS"] == applicable {
		return "PASS", "all applicable required obligations have current independent evidence"
	}
	if counts["PASS"] > 0 {
		return "PARTIAL", "some required obligations have no passing evidence"
	}
	return "NOT_RUN", "no complete applicable business proof has been executed"
}

func coveragePercent(measure *CoverageMeasure) {
	denominator := measure.Total - measure.Excluded
	if denominator > 0 {
		measure.Percent = 100 * float64(measure.Passed) / float64(denominator)
	}
}

func (d db) analyzeModel(g Graph, environment, roundID string) (ModelAnalysis, error) {
	result := ModelAnalysis{Graph: g.ID, Environment: environment, Round: roundID, Nodes: map[string]BusinessNodeStatus{}, Obligations: []ObligationStatus{}, Diagnostics: modelDiagnostics(g)}
	if g.SchemaVersion != 2 {
		return result, errors.New("multi-view business analysis requires a v2 model; retain legacy evidence as its original scope")
	}
	if roundID == "" {
		query := "SELECT id FROM verification_rounds WHERE graph=" + quote(g.ID)
		if environment != "" {
			query += " AND environment=" + quote(environment)
		}
		rows, err := d.query(query+" ORDER BY rowid DESC LIMIT 2", 1)
		if err != nil {
			return result, err
		}
		if len(rows) == 1 {
			roundID = rows[0][0]
		}
		if len(rows) > 1 {
			result.Diagnostics = append(result.Diagnostics, BusinessDiagnostic{Code: "ROUND_REQUIRED", Severity: "warning", Subjects: []string{g.ID}, Reason: "select one existing round; partial runs from separate rounds cannot be combined"})
		}
	}
	roundCurrent := false
	staleReason := "no verification round selected"
	if roundID != "" {
		round, err := d.verificationRound(roundID)
		if err != nil {
			return result, err
		}
		if round.Context.Graph != g.ID || environment != "" && round.Context.Environment != environment {
			return result, errors.New("selected round belongs to another graph or environment")
		}
		result.Round, result.Environment, result.Context = round.ID, round.Context.Environment, round.Context
		if err := d.validateRound(round); err != nil {
			staleReason = err.Error()
		} else {
			roundCurrent = true
		}
	}
	graphRows, err := d.query("SELECT approved FROM graphs WHERE id="+quote(g.ID), 1)
	if err != nil {
		return result, err
	}
	if len(graphRows) != 1 {
		return result, errGraphNotFound
	}
	result.CatalogApproved = graphRows[0][0] == "1"
	approvals := map[string]bool{}
	rows, err := d.query("SELECT id,approved FROM rules WHERE graph="+quote(g.ID), 2)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		approvals[row[0]] = row[1] == "1"
		if !approvals[row[0]] {
			result.UnconfirmedRules++
		}
	}
	tasks := map[string]taskSpec{}
	taskStates := map[string]string{}
	rows, err = d.query("SELECT id,rule,kind,condition,expected,status FROM tasks WHERE graph="+quote(g.ID)+" ORDER BY id LIMIT 100001", 6)
	if err != nil {
		return result, err
	}
	if len(rows) > 100000 {
		return result, errors.New("model task ledger exceeds 100000 tasks")
	}
	for _, row := range rows {
		if row[0] == g.ID+"/_schema/v2" && row[2] == "MODEL_SCHEMA_GUARD" && row[5] == "BLOCKED" {
			continue
		}
		result.ActualTasks++
		tasks[row[0]] = taskSpec{ID: row[0], Rule: row[1], Kind: row[2], Condition: row[3], Expected: row[4]}
		taskStates[row[0]] = row[5]
	}
	type attempt struct{ run, status, round string }
	latest := map[string]attempt{}
	history := map[string]int{}
	roundFailures := map[string]int{}
	rows, err = d.query("SELECT r.task,r.id,r.status,COALESCE(m.round,'') FROM runs r JOIN tasks t ON t.id=r.task LEFT JOIN model_runs m ON m.run=r.id WHERE t.graph="+quote(g.ID)+" ORDER BY r.rowid LIMIT 100001", 4)
	if err != nil {
		return result, err
	}
	if len(rows) > 100000 {
		return result, errors.New("history query exceeds 100000 runs")
	}
	for _, row := range rows {
		if row[2] == "FAIL" {
			history[row[0]]++
			result.HistoryFailures++
		}
		if row[3] == result.Round && result.Round != "" {
			latest[row[0]] = attempt{row[1], row[2], row[3]}
			if row[2] == "FAIL" {
				roundFailures[row[0]]++
			}
		}
	}
	scans, err := d.latestScans(g.ID)
	if err != nil {
		return result, err
	}
	x := indexModel(g)
	obligations := modelObligations(g)
	nodeObligations := map[string][]int{}
	statuses := []string{}
	for _, obligation := range obligations {
		status := ObligationStatus{ModelObligation: obligation, Status: "NOT_RUN", Reason: "required obligation has not been executed", HistoryFailures: history[obligation.Task.ID]}
		stored, planned := tasks[obligation.Task.ID]
		status.Planned = planned
		if planned && stored != obligation.Task {
			status.Status, status.Reason = "BLOCKED", "stored task contradicts the frozen obligation"
			result.InvalidTasks++
		}
		if !planned {
			status.Reason = "required obligation has not been planned"
		}
		switch {
		case obligation.Excluded:
			status.Status, status.Reason = "EXCLUDED", obligation.ExclusionReason
		case obligation.Definition.PendingReason != "":
			status.Status, status.Reason = "BLOCKED", obligation.Definition.PendingReason
		case !result.CatalogApproved || !rulesApproved(approvals, obligationRules(x, obligation)):
			status.Status, status.Reason = "BLOCKED", "business catalog or rule needs explicit user approval"
		case planned && stored != obligation.Task:
		case obligation.Definition.Kind == testExistence:
			scan, found := scans[x.rules[obligation.Task.Rule].Action]
			status.Status, status.Reason = existenceVerdict(scan, found, result.Context.SourceCommit, roundCurrent, staleReason)
		default:
			if current, found := latest[obligation.Task.ID]; found {
				status.Run = current.run
				switch current.status {
				case "FAIL":
					status.Status, status.Reason = "FAIL", "independent business state or property failed in the selected round"
				case "BLOCKED", "RUNNING":
					status.Status, status.Reason = "BLOCKED", "execution or recovery did not complete"
				case "PASS":
					if !roundCurrent {
						status.Status, status.Reason = "STALE", staleReason
					} else if err := d.qualifyModelRun(current.run); err != nil {
						status.Status, status.Reason = "STALE", err.Error()
					} else if roundFailures[obligation.Task.ID] > 0 {
						status.Status, status.Reason = "FLAKY", "selected round contains both a failed attempt and a passing retry"
					} else {
						status.Status, status.Reason = "PASS", "registered execution and current independent evidence verified"
					}
				default:
					status.Status, status.Reason = "BLOCKED", "unknown persisted run status"
				}
			} else if taskStates[obligation.Task.ID] == "PASS" {
				status.Status, status.Reason = "STALE", "historical pass has no membership in the selected round"
			}
		}
		result.Obligations = append(result.Obligations, status)
		index := len(result.Obligations) - 1
		for _, target := range obligation.Targets {
			nodeObligations[target] = append(nodeObligations[target], index)
		}
		nodeObligations[obligation.Definition.ID] = append(nodeObligations[obligation.Definition.ID], index)
		statuses = append(statuses, status.Status)
		result.Coverage.Obligations.Total++
		if status.Status == "PASS" {
			result.Coverage.Obligations.Passed++
		}
		if status.Status == "EXCLUDED" {
			result.Coverage.Obligations.Excluded++
		}
		delete(tasks, obligation.Task.ID)
	}
	derived, err := d.derivedObligationStatuses(g, result.Round, roundCurrent, staleReason, approvals, result.CatalogApproved)
	if err != nil {
		return result, err
	}
	for _, status := range derived {
		obligations = append(obligations, status.ModelObligation)
		result.Obligations = append(result.Obligations, status)
		index := len(result.Obligations) - 1
		for _, target := range status.Targets {
			nodeObligations[target] = append(nodeObligations[target], index)
		}
		nodeObligations[status.Definition.ID] = append(nodeObligations[status.Definition.ID], index)
		statuses = append(statuses, status.Status)
		result.Coverage.Obligations.Total++
		if status.Status == "PASS" {
			result.Coverage.Obligations.Passed++
		}
	}
	result.InvalidTasks += len(tasks)
	for _, node := range g.Nodes {
		if node.Kind != "domain" && node.Kind != "system" && node.Kind != "boundary" {
			continue
		}
		indices := []int{}
		for id, items := range nodeObligations {
			if x.kinds[id] != "capability" {
				continue
			}
			for parent := id; parent != ""; parent = x.parents[parent] {
				if parent == node.ID {
					indices = append(indices, items...)
					break
				}
			}
			if node.Kind == "boundary" {
				for _, edge := range g.Edges {
					if edge.Kind == "owns" && edge.From == node.ID && edge.To == id {
						indices = append(indices, items...)
					}
				}
			}
		}
		if len(indices) > 0 {
			nodeObligations[node.ID] = indices
		}
	}
	allKinds := modelKinds(g)
	for _, obligation := range obligations {
		allKinds[obligation.Definition.ID] = "test"
	}
	implementation := implementationReadiness(g, x, scans, result.Context.SourceCommit, roundCurrent)
	for id, kind := range allKinds {
		seen := map[int]bool{}
		nodeStatuses := []string{}
		state := BusinessNodeStatus{Readiness: NodeReadiness{Described: true, ImplementationStatus: "UNOBSERVED", DeploymentStatus: "UNOBSERVED"}}
		if observed, found := implementation[id]; found {
			state.Readiness.ImplementationStatus, state.Readiness.Implemented = observed.Status, observed.Implemented
		}
		for _, index := range nodeObligations[id] {
			if seen[index] {
				continue
			}
			seen[index] = true
			obligation := result.Obligations[index]
			nodeStatuses = append(nodeStatuses, obligation.Status)
			state.Required++
			if obligation.Status == "PASS" {
				state.Passed++
			}
			if obligation.Status == "EXCLUDED" {
				state.Excluded++
			}
			state.HistoryFailures += obligation.HistoryFailures
		}
		state.Status, state.Reason = aggregatedStatus(nodeStatuses)
		state.Readiness.Verified = state.Status == "PASS"
		if kind == "role" || kind == "data" || kind == "contract" || kind == "deployment" {
			state.Reason = "modeled entity; no independent implementation or deployment evidence recorded"
		}
		result.Nodes[id] = state
		var measure *CoverageMeasure
		switch kind {
		case "capability":
			measure = &result.Coverage.Capabilities
		case "action", "condition", "state":
			measure = &result.Coverage.Behaviors
		case "invariant":
			measure = &result.Coverage.Invariants
		}
		if measure != nil {
			measure.Total++
			if state.Status == "PASS" {
				measure.Passed++
			}
		}
	}
	for _, edge := range g.Edges {
		if edge.Kind == "transitions_to" {
			result.Coverage.Branches.Total++
			s := []string{}
			for _, index := range nodeObligations[edge.ID] {
				s = append(s, result.Obligations[index].Status)
			}
			status, _ := aggregatedStatus(s)
			if status == "PASS" {
				result.Coverage.Branches.Passed++
			}
		}
	}
	for _, obligation := range result.Obligations {
		if obligation.Definition.Kind == testPositive || obligation.Definition.Kind == testNegative || obligation.Definition.Kind == testForced {
			result.Coverage.Branches.Total++
			if obligation.Status == "PASS" {
				result.Coverage.Branches.Passed++
			}
		}
	}
	for _, measure := range []*CoverageMeasure{&result.Coverage.Obligations, &result.Coverage.Capabilities, &result.Coverage.Behaviors, &result.Coverage.Branches, &result.Coverage.Invariants} {
		coveragePercent(measure)
	}
	result.Verdict, result.Reason = aggregatedStatus(statuses)
	if result.Verdict == "PARTIAL" || result.Verdict == "STALE" || result.Verdict == "FLAKY" {
		result.Verdict = "BLOCKED"
	}
	if result.InvalidTasks > 0 {
		result.Verdict, result.Reason = "BLOCKED", "unexpected or conflicting tasks are present in the model ledger"
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Severity == "error" {
			result.Verdict, result.Reason = "BLOCKED", diagnostic.Reason
		}
	}
	return result, nil
}

type implementationState struct {
	Status      string
	Implemented bool
}

// implementationReadiness projects the latest scans onto actions, their rules
// and every capability that implements them. A capability is implemented only
// when all of its actions are; the weakest observation wins otherwise.
func implementationReadiness(g Graph, x modelIndex, scans map[string]ScanObservation, candidate string, current bool) map[string]implementationState {
	result := map[string]implementationState{}
	rank := map[string]int{"NOT_FOUND": 0, "UNKNOWN": 1, "UNOBSERVED": 2, "PRESENT": 3}
	for _, node := range g.Nodes {
		if node.Kind != "action" {
			continue
		}
		state := implementationState{Status: "UNOBSERVED"}
		if scan, found := scans[node.ID]; found {
			verdict, _ := existenceVerdict(scan, true, candidate, current, "")
			state = implementationState{Status: scan.Status, Implemented: verdict == "PASS"}
		}
		result[node.ID] = state
		for _, capability := range x.capsByAction[node.ID] {
			aggregate, seen := result[capability]
			if !seen || rank[state.Status] < rank[aggregate.Status] {
				aggregate.Status = state.Status
			}
			aggregate.Implemented = (!seen || aggregate.Implemented) && state.Implemented
			result[capability] = aggregate
		}
	}
	for _, rule := range g.Rules {
		if state, found := result[rule.Action]; found {
			result[rule.ID] = state
		}
	}
	return result
}

func modelDiagnostics(g Graph) []BusinessDiagnostic {
	result := []BusinessDiagnostic{}
	x := indexModel(g)
	owners := map[string][]string{}
	dependencies := map[string][]string{}
	shares := map[string]bool{}
	for _, shared := range g.SharedOwners {
		shares[shared.Subject] = true
	}
	for _, edge := range g.Edges {
		if edge.Kind == "owns" {
			owners[edge.To] = append(owners[edge.To], edge.From)
		}
		if edge.Kind == "depends_on" {
			dependencies[edge.From] = append(dependencies[edge.From], edge.To)
		}
	}
	for id, owned := range owners {
		if len(owned) > 1 && !shares[id] {
			result = append(result, BusinessDiagnostic{Code: "OWNERSHIP_CONFLICT", Severity: "error", Subjects: append([]string{id}, owned...), Reason: "multiple boundaries own the same business subject without a sourced sharing declaration"})
		}
	}
	if cycle := dependencyCycle(dependencies); len(cycle) > 0 {
		result = append(result, BusinessDiagnostic{Code: "DEPENDENCY_CYCLE", Severity: "warning", Subjects: cycle, Reason: "business dependencies are cyclic; containment and test ordering remain separate"})
	}
	actions := map[string][]string{}
	rules := map[string]int{}
	for _, edge := range g.Edges {
		if edge.Kind == "implements" {
			actions[edge.From] = append(actions[edge.From], edge.To)
		}
	}
	for _, rule := range g.Rules {
		rules[rule.Action]++
	}
	for _, node := range g.Nodes {
		if node.Kind == "capability" {
			count := 0
			for _, action := range actions[node.ID] {
				count += rules[action]
			}
			if count == 0 {
				result = append(result, BusinessDiagnostic{Code: "CAPABILITY_WITHOUT_RULE", Severity: "error", Subjects: []string{node.ID}, Reason: "capability has no independently verifiable action contract"})
			}
		}
	}
	for _, contract := range g.Contracts {
		if contract.Unconfirmed {
			result = append(result, BusinessDiagnostic{Code: "UNCONFIRMED_CONTRACT", Severity: "error", Subjects: []string{contract.Subject}, Reason: "unconfirmed business semantics cannot be qualified", SourceRef: contract.SourceRef})
		}
	}
	for _, invariant := range g.Invariants {
		connected := false
		for _, scope := range invariant.Scope {
			if x.kinds[scope] == "rule" || x.kinds[scope] == "action" || x.kinds[scope] == "capability" {
				connected = true
			}
		}
		if !connected {
			result = append(result, BusinessDiagnostic{Code: "INVARIANT_WITHOUT_ACTION", Severity: "error", Subjects: []string{invariant.ID}, Reason: "business invariant has no executable action scope"})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return fmt.Sprint(a.Subjects) < fmt.Sprint(b.Subjects)
	})
	return result
}

func (d db) modelGate(g Graph) (map[string]any, error) {
	analysis, err := d.analyzeModel(g, "", "")
	if err != nil {
		return nil, err
	}
	passed := analysis.Coverage.Obligations.Passed
	missing := 0
	invalid := analysis.InvalidTasks
	for _, obligation := range analysis.Obligations {
		if obligation.Excluded {
			continue
		}
		if !obligation.Planned {
			missing++
		}
		if obligation.Status != "PASS" {
			invalid++
		}
	}
	openRisks, err := d.openRisks(g.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"verdict": analysis.Verdict, "reason": analysis.Reason, "catalog_approved": analysis.CatalogApproved, "confirmed_rules": len(g.Rules) - analysis.UnconfirmedRules, "unconfirmed_rules": analysis.UnconfirmedRules, "required_tasks": analysis.Coverage.Obligations.Total, "actual_tasks": analysis.ActualTasks, "passed_with_valid_raw": passed, "missing_tasks": missing, "invalid_tasks_or_evidence": invalid, "open_risks": openRisks, "proof_scope": "complete_business_model_obligations", "execution_contexts": func() int {
		if analysis.Round != "" {
			return 1
		}
		return 0
	}(), "inputs_unchanged": analysis.Round != "" && analysis.Verdict == "PASS", "context": analysis.Context, "coverage": analysis.Coverage, "history_failures": analysis.HistoryFailures}, nil
}

func (d db) analysisJSON(graph, environment, round string) ([]byte, error) {
	g, err := d.model(graph)
	if err != nil {
		return nil, err
	}
	result, err := d.analyzeModel(g, environment, round)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}
