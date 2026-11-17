package main

import (
	"fmt"
	"sort"
)

type BusinessBranchExecution struct {
	Branch   string   `json:"branch"`
	Status   string   `json:"status"`
	Reason   string   `json:"reason"`
	Runs     []string `json:"runs"`
	Steps    []string `json:"steps"`
	Outcomes []string `json:"outcomes,omitempty"`
}

type BusinessExecutionEvidenceError struct {
	Run    string `json:"run"`
	Reason string `json:"reason"`
}

type BusinessExecutionCoverage struct {
	Status         string                           `json:"status"`
	Round          string                           `json:"round,omitempty"`
	Total          int                              `json:"total"`
	Passed         int                              `json:"passed"`
	Failed         int                              `json:"failed"`
	Blocked        int                              `json:"blocked"`
	NotRun         int                              `json:"not_run"`
	Branches       []BusinessBranchExecution        `json:"branches"`
	EvidenceErrors []BusinessExecutionEvidenceError `json:"evidence_errors,omitempty"`
	Reason         string                           `json:"reason"`
}

func businessBranchExecutions(g Graph) map[string]*BusinessBranchExecution {
	branches := map[string]*BusinessBranchExecution{}
	if g.BusinessContract == nil {
		return branches
	}
	for _, branch := range g.BusinessContract.Branches {
		entry := &BusinessBranchExecution{Branch: branch.ID, Status: "NOT_RUN", Reason: "no validated independent RAW executed this exact frozen branch in the selected round", Runs: []string{}, Steps: []string{}}
		if !branch.Confirmed {
			entry.Status, entry.Reason = "BLOCKED", "branch has no explicit confirmation"
		}
		branches[branch.ID] = entry
	}
	return branches
}

// collectBusinessBranchFacts consumes only the verified projection returned by
// verifiedRunFacts. PASS belongs to the exact frozen branch, never every branch
// sharing a test, action, capability or obligation ID.
func collectBusinessBranchFacts(g Graph, run string, facts []UnifiedStepFact, branches map[string]*BusinessBranchExecution) error {
	for _, fact := range facts {
		if fact.SourceKind != "execute" && fact.SourceKind != "sequence" {
			continue
		}
		if fact.Graph != g.ID || fact.Run != run || fact.EvidenceStatus != "PASS" || fact.EvidenceVersion < runtimeEvidenceVersion {
			return fmt.Errorf("action fact lacks current graph/run identity and validated versioned RAW")
		}
		if len(fact.BusinessExpectations) == 0 {
			return fmt.Errorf("executed action has no frozen business expectation")
		}
		for _, item := range fact.BusinessExpectations {
			frozen := item.Decision.Frozen
			if item.Decision.Status != "READY" || frozen == nil || item.After == nil || item.Comparison == nil {
				return fmt.Errorf("action fact lacks a complete pre-action freeze and independent actual comparison")
			}
			branch := branches[frozen.Branch]
			if branch == nil {
				return fmt.Errorf("RAW names undeclared branch %s", frozen.Branch)
			}
			var action *ActionObservation
			for index := range fact.Actions {
				candidate := &fact.Actions[index]
				if candidate.Actor == item.Actor && candidate.ActionIndex == item.ActionIndex {
					if action != nil {
						return fmt.Errorf("ambiguous action identity in branch evidence")
					}
					action = candidate
				}
			}
			if action == nil || action.Node != frozen.Action {
				return fmt.Errorf("frozen branch is not attached to its captured action identity")
			}
			comparison := CompareBusinessExpectation(g, *frozen, *item.After, action.Denied)
			if comparison.Status != item.Comparison.Status {
				return fmt.Errorf("persisted comparison differs from deterministic RAW re-evaluation")
			}
			branch.Runs = append(branch.Runs, run)
			branch.Steps = append(branch.Steps, fmt.Sprintf("%s:%d:%s:%d", run, fact.Seq, item.Actor, item.ActionIndex))
			if comparison.Outcome != "" {
				branch.Outcomes = append(branch.Outcomes, comparison.Outcome)
			}
			switch {
			case comparison.Status == "FAIL" || fact.Verdict == "FAIL":
				branch.Status, branch.Reason = "FAIL", "this exact frozen branch has a verified failure in the selected round; later success does not erase it"
			case branch.Status == "FAIL":
			case comparison.Status != "PASS" || fact.Verdict != "PASS":
				branch.Status, branch.Reason = "BLOCKED", "branch action or independent comparison is incomplete or blocked"
			case branch.Status != "BLOCKED":
				branch.Status, branch.Reason = "PASS", "this exact frozen branch has current independently verified action evidence"
			}
		}
	}
	return nil
}

func summarizeBusinessBranchExecutions(round string, branches map[string]*BusinessBranchExecution, evidenceErrors []BusinessExecutionEvidenceError) BusinessExecutionCoverage {
	result := BusinessExecutionCoverage{Status: "NOT_RUN", Round: round, Branches: []BusinessBranchExecution{}, EvidenceErrors: evidenceErrors}
	ids := []string{}
	for id := range branches {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		branch := *branches[id]
		branch.Runs, branch.Steps, branch.Outcomes = uniqueSortedBusiness(branch.Runs), uniqueSortedBusiness(branch.Steps), uniqueSortedBusiness(branch.Outcomes)
		result.Branches = append(result.Branches, branch)
		result.Total++
		switch branch.Status {
		case "PASS":
			result.Passed++
		case "FAIL":
			result.Failed++
		case "BLOCKED":
			result.Blocked++
		default:
			result.NotRun++
		}
	}
	switch {
	case result.Failed > 0:
		result.Status, result.Reason = "FAIL", "at least one exact branch has a verified failure in this round"
	case len(evidenceErrors) > 0 || result.Blocked > 0:
		result.Status, result.Reason = "BLOCKED", "branch coverage contains unconfirmed semantics, incomplete or damaged evidence"
	case result.Total > 0 && result.Passed == result.Total:
		result.Status, result.Reason = "PASS", "every declared branch has its own verified frozen expectation and actual execution in this round"
	default:
		result.Reason = "one or more declared branches have no independent execution evidence; shared tests cannot supply it"
	}
	return result
}

func (d db) businessExecutionCoverage(g Graph, round string) (BusinessExecutionCoverage, error) {
	if g.BusinessContract == nil {
		return BusinessExecutionCoverage{Status: "LEGACY_UNDECLARED", Round: round, Branches: []BusinessBranchExecution{}, Reason: "legacy obligations have no deterministic branch identity"}, nil
	}
	branches := businessBranchExecutions(g)
	if round == "" {
		return summarizeBusinessBranchExecutions(round, branches, nil), nil
	}
	rows, err := d.query("SELECT m.run FROM model_runs m JOIN runs r ON r.id=m.run JOIN tasks t ON t.id=r.task WHERE t.graph="+quote(g.ID)+" AND m.round="+quote(round)+" ORDER BY r.rowid LIMIT 100001", 1)
	if err != nil {
		return BusinessExecutionCoverage{}, err
	}
	if len(rows) > 100000 {
		return BusinessExecutionCoverage{}, fmt.Errorf("contract execution coverage exceeds 100000 runs")
	}
	evidenceErrors := []BusinessExecutionEvidenceError{}
	for _, row := range rows {
		verified, err := d.verifiedRunFacts(row[0])
		if err != nil {
			evidenceErrors = append(evidenceErrors, BusinessExecutionEvidenceError{Run: row[0], Reason: err.Error()})
			continue
		}
		if verified.EvidenceStatus != "PASS" {
			evidenceErrors = append(evidenceErrors, BusinessExecutionEvidenceError{Run: row[0], Reason: "run evidence is " + verified.EvidenceStatus})
			continue
		}
		if err := collectBusinessBranchFacts(g, row[0], verified.Facts, branches); err != nil {
			evidenceErrors = append(evidenceErrors, BusinessExecutionEvidenceError{Run: row[0], Reason: err.Error()})
		}
	}
	return summarizeBusinessBranchExecutions(round, branches, evidenceErrors), nil
}
