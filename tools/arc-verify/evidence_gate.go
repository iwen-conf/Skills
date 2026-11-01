package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
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
	Report           ExploreReport
	Binding          RunBinding
	ExpectedCommit   string
	ExpectedGraph    string
	ExpectedTarget   string
	ExpectedCampaign string
	ExpectedFixture  string
	ExpectedActor    string
	ExpectedOracle   string
	ApprovedRules    map[string]bool
	PlannerCalls     int
	RequirePlanner   bool
	OracleFields     []OracleField
	HistoricalFails  []string
}

func evaluateEvidenceGate(in EvidenceGateInput) (EvidenceGateVerdict, string) {
	if in.RequirePlanner && in.PlannerCalls < 1 {
		return gateBlocked, "AI acceptance requires an actual planner call"
	}
	if len(in.Report.Steps) == 0 {
		if in.Report.StopReason == "complete" {
			return gateFail, "report has no actual steps"
		}
		return gateNotRun, "report has no actual steps"
	}
	if in.ExpectedCommit != "" && in.Binding.Identity.SourceCommit != in.ExpectedCommit {
		return gateStale, "source commit does not match the current candidate"
	}
	if in.ExpectedGraph != "" && in.Binding.Identity.GraphSHA256 != in.ExpectedGraph {
		return gateStale, "graph digest does not match the bound evidence"
	}
	if in.ExpectedTarget != "" && in.Binding.Identity.Target != in.ExpectedTarget {
		return gateFail, "evidence was transplanted from another target"
	}
	if in.ExpectedCampaign != "" && in.Report.Campaign != "" && in.ExpectedCampaign != in.Report.Campaign {
		return gateFail, "evidence belongs to another campaign"
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
		if step.SHA256 == "" {
			return gateFail, "RAW has no SHA256"
		}
		if _, err := os.Stat(step.RAW); err != nil {
			return gateFail, "RAW is missing: " + step.RAW
		}
		body, err := os.ReadFile(step.RAW)
		if err != nil {
			return gateBlocked, "RAW cannot be read"
		}
		if digestBytes(body) != step.SHA256 {
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
	if binding.Identity.Graph != "" && raw.Identity.Graph != binding.Identity.Graph {
		return errors.New("RAW graph does not match the current binding")
	}
	if binding.Identity.SourceCommit != "" && raw.Identity.SourceCommit != binding.Identity.SourceCommit {
		return errors.New("RAW commit does not match the current binding")
	}
	if binding.Identity.Target != "" && raw.Identity.Target != binding.Identity.Target {
		return errors.New("RAW target does not match the current binding")
	}
	if binding.Identity.Task != "" && raw.Identity.Task != "" && raw.Identity.Task != binding.Identity.Task {
		return errors.New("RAW task does not match the current binding")
	}
	if binding.Oracle != "" && raw.Oracle != "" && raw.Oracle != binding.Oracle {
		return errors.New("RAW oracle does not match the current binding")
	}
	return nil
}

func classifyEvidenceTrust(report ExploreReport, binding RunBinding) string {
	if len(report.Steps) == 0 {
		return "UNVERIFIED"
	}
	if binding.Identity.Graph == "" || binding.Identity.SourceCommit == "" || binding.Identity.Target == "" {
		return "UNBOUND"
	}
	verdict, reason := evaluateEvidenceGate(EvidenceGateInput{Report: report, Binding: binding, ExpectedCommit: binding.Identity.SourceCommit, ExpectedTarget: binding.Identity.Target, ExpectedCampaign: report.Campaign})
	switch verdict {
	case gatePass, gatePartial, gateUnspec, gateFail:
		if strings.Contains(reason, "digest") || strings.Contains(reason, "missing") {
			if strings.Contains(reason, "digest") {
				return "TAMPERED"
			}
			return "INCOMPLETE"
		}
		if verdict == gateFail && (strings.Contains(reason, "transplant") || strings.Contains(reason, "another")) {
			return "UNBOUND"
		}
		synthetic := true
		for _, step := range report.Steps {
			if step.After.Principal != "" && !strings.HasPrefix(step.After.Principal, "synthetic-") {
				synthetic = false
			}
		}
		if synthetic && verdict != gateFail {
			return "SYNTHETIC_ONLY"
		}
		return "VERIFIED"
	case gateStale:
		return "STALE"
	case gateBlocked, gateNotRun:
		return "INCOMPLETE"
	default:
		return "UNVERIFIED"
	}
}

func classifyBusinessVerdict(report ExploreReport) string {
	if len(report.RuleFails) > 0 {
		return "FAIL"
	}
	if report.RecoveryClass == RecoveryFailedLocked {
		return "BLOCKED"
	}
	if len(report.Steps) == 0 {
		return "NOT_RUN"
	}
	if report.Truncated || report.StopReason == "blocked" || report.StopReason == "classify_failed" {
		return "PARTIAL"
	}
	if report.StopReason == "complete" {
		return "PASS"
	}
	return "PARTIAL"
}

func gateRejectsTamper(reason string) error {
	return fmt.Errorf("evidence gate rejected: %s", reason)
}

func (d db) attachExploreEvidence(graph string, out map[string]any) (map[string]any, error) {
	if out == nil {
		out = map[string]any{}
	}
	campaigns, err := d.exploreEvidenceReports(graph)
	if err != nil {
		return out, err
	}
	if len(campaigns) == 0 {
		return out, nil
	}
	evaluations := []map[string]string{}
	worst := EvidenceGateVerdict("")
	worstReason := ""
	for _, campaign := range campaigns {
		report := campaign.Report
		if report.Campaign == "" {
			report.Campaign = campaign.Campaign
			report.Graph = campaign.Graph
			report.StopReason = firstNonEmpty(report.StopReason, campaign.Status)
		}
		verdict, reason := evaluateEvidenceGate(EvidenceGateInput{
			Report:           report,
			Binding:          RunBinding{Identity: ExecutionIdentity{Graph: firstNonEmpty(campaign.Graph, graph), SourceCommit: campaign.SourceCommit, GraphSHA256: campaign.ModelSHA}},
			ExpectedCommit:   campaign.SourceCommit,
			ExpectedGraph:    campaign.ModelSHA,
			ExpectedCampaign: campaign.Campaign,
			PlannerCalls:     report.Spent.PlannerCalls,
			RequirePlanner:   campaign.Mode == "ai" || report.Mode == "ai",
		})
		evaluations = append(evaluations, map[string]string{"campaign": campaign.Campaign, "verdict": string(verdict), "reason": reason})
		if verdict == gateFail || verdict == gateStale || verdict == gateBlocked {
			if worst != gateFail {
				worst, worstReason = verdict, reason
			}
		}
	}
	out["evidence_gate"] = evaluations
	if worst != "" {
		out["evidence_verdict"] = string(worst)
		if fmt.Sprint(out["verdict"]) == "PASS" {
			mapped := string(worst)
			if worst == gateStale {
				mapped = "STALE"
			}
			if worst == gateBlocked {
				mapped = "BLOCKED"
			}
			if worst == gateFail {
				mapped = "FAIL"
			}
			out["verdict"] = mapped
			out["reason"] = worstReason
		}
	}
	return out, nil
}

func (d db) exploreEvidenceReports(graph string) ([]ExploreCheckpoint, error) {
	states, err := d.listExploreCheckpoints(graph)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []ExploreCheckpoint{}
	for _, state := range states {
		out = append(out, state)
		seen[state.Campaign] = true
	}
	rows, err := d.query("SELECT body FROM exploration_campaigns WHERE graph="+quote(graph), 1)
	if err != nil {
		return out, err
	}
	for _, row := range rows {
		var report ExploreReport
		if json.Unmarshal([]byte(row[0]), &report) != nil || seen[report.Campaign] {
			continue
		}
		out = append(out, ExploreCheckpoint{Campaign: report.Campaign, Graph: graph, Report: report, Status: report.StopReason, Mode: report.Mode})
	}
	return out, nil
}
