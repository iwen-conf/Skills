package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

type EvidenceGateVerdict string

const (
	gatePass    EvidenceGateVerdict = "PASS"
	gateFail    EvidenceGateVerdict = "FAIL"
	gatePartial EvidenceGateVerdict = "PARTIAL"
	gateBlocked EvidenceGateVerdict = "BLOCKED"
	gateNotRun  EvidenceGateVerdict = "NOT_RUN"
	gateStale   EvidenceGateVerdict = "STALE"
	gateFlaky   EvidenceGateVerdict = "FLAKY"
	gateUnspec  EvidenceGateVerdict = "UNSPECIFIED"
	gateRisk    EvidenceGateVerdict = "RISK"
)

type EvidenceGateInput struct {
	Report          ExploreReport
	Binding         RunBinding
	ExpectedCommit  string
	ExpectedGraph   string
	ExpectedTarget  string
	ApprovedRules   map[string]bool
	PlannerCalls    int
	RequirePlanner  bool
	OracleFields    []OracleField
	HistoricalFails []string
}

func evaluateEvidenceGate(in EvidenceGateInput) (EvidenceGateVerdict, string) {
	if in.RequirePlanner && in.PlannerCalls < 1 {
		return gateBlocked, "AI acceptance requires an actual planner call"
	}
	if in.Binding.Identity.SourceCommit != "" && in.ExpectedCommit != "" && in.Binding.Identity.SourceCommit != in.ExpectedCommit {
		return gateStale, "source commit does not match the current candidate"
	}
	if in.ExpectedGraph != "" && in.Binding.Identity.GraphSHA256 != "" && in.ExpectedGraph != in.Binding.Identity.GraphSHA256 {
		return gateStale, "graph digest does not match the bound evidence"
	}
	if in.ExpectedTarget != "" && in.Binding.Identity.Target != "" && in.ExpectedTarget != in.Binding.Identity.Target {
		return gateFail, "evidence was transplanted from another target"
	}
	if !oracleAllowsBusinessPass(in.OracleFields) && len(in.OracleFields) > 0 {
		return gateBlocked, "synthetic or unknown oracle fields cannot be the sole PASS evidence"
	}
	if in.Report.RecoveryClass == RecoveryFailedLocked {
		return gateBlocked, "recovery failed; later side-effecting tasks stay blocked"
	}
	for _, step := range in.Report.Steps {
		if step.RAW == "" {
			return gateBlocked, "a step is missing RAW"
		}
		if _, err := os.Stat(step.RAW); err != nil {
			return gateFail, "RAW is missing: " + step.RAW
		}
		body, err := os.ReadFile(step.RAW)
		if err != nil {
			return gateBlocked, "RAW cannot be read"
		}
		if step.SHA256 != "" && digestBytes(body) != step.SHA256 {
			return gateFail, "RAW digest does not match the ledger"
		}
		if err := rawMatchesBinding(body, in.Binding); err != nil {
			return gateFail, err.Error()
		}
	}
	if len(in.Report.RuleFails) > 0 {
		return gateFail, "confirmed rule violations remain"
	}
	if len(in.HistoricalFails) > 0 && len(in.Report.RuleFails) == 0 && in.Report.StopReason == "complete" {
		return gateFail, "a later PASS must not overwrite the first historical FAIL"
	}
	unapproved := 0
	for id, ok := range in.ApprovedRules {
		if !ok && id != "" {
			unapproved++
		}
	}
	if unapproved > 0 && in.Report.StopReason == "complete" {
		return gatePartial, "unapproved rules stay out of the confirmed denominator"
	}
	if in.Report.Truncated || in.Report.StopReason == "blocked" {
		return gateBlocked, in.Report.StopReason
	}
	if len(in.Report.Unspecified) > 0 {
		return gateUnspec, "contract-out observations remain"
	}
	return gatePass, "bound RAW, oracle and identity match the current run"
}

func rawMatchesBinding(body []byte, binding RunBinding) error {
	var raw StepRAW
	if err := json.Unmarshal(body, &raw); err != nil {
		return errors.New("RAW is not a StepRAW document")
	}
	if raw.Identity.Graph != "" && binding.Identity.Graph != "" && raw.Identity.Graph != binding.Identity.Graph {
		return errors.New("RAW graph does not match the current binding")
	}
	if raw.Identity.SourceCommit != "" && binding.Identity.SourceCommit != "" && raw.Identity.SourceCommit != binding.Identity.SourceCommit {
		return errors.New("RAW commit does not match the current binding")
	}
	if raw.Identity.Target != "" && binding.Identity.Target != "" && raw.Identity.Target != binding.Identity.Target {
		return errors.New("RAW target does not match the current binding")
	}
	return nil
}

func gateRejectsTamper(reason string) error {
	return fmt.Errorf("evidence gate rejected: %s", reason)
}
