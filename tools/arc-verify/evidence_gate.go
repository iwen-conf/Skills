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
	required := requiredExploreIdentities(in.Report)
	if missing := missingExploreIdentities(in, required); len(missing) > 0 {
		return gateBlocked, "MISSING_IDENTITY: " + strings.Join(missing, ",")
	}
	if in.ExpectedCommit != "" && in.Binding.Identity.SourceCommit != "" && in.Binding.Identity.SourceCommit != in.ExpectedCommit {
		return gateStale, "source commit does not match the current candidate"
	}
	if in.ExpectedGraph != "" && in.Binding.Identity.GraphSHA256 != "" && in.Binding.Identity.GraphSHA256 != in.ExpectedGraph {
		return gateStale, "graph digest does not match the bound evidence"
	}
	if required["target"] || in.ExpectedTarget != "" {
		if in.Binding.Identity.Target == "" || in.ExpectedTarget == "" && required["target"] {
			return gateBlocked, "MISSING_IDENTITY: target"
		}
		if in.ExpectedTarget != "" && in.Binding.Identity.Target != in.ExpectedTarget {
			return gateFail, "evidence was transplanted from another target"
		}
	}
	if in.ExpectedCampaign != "" && in.Report.Campaign != "" && in.ExpectedCampaign != in.Report.Campaign {
		return gateFail, "evidence belongs to another campaign"
	}
	if required["fixture"] && (in.Binding.Identity.Fixture == "" || in.ExpectedFixture != "" && in.Binding.Identity.Fixture != in.ExpectedFixture) {
		if in.Binding.Identity.Fixture == "" {
			return gateBlocked, "MISSING_IDENTITY: fixture"
		}
		return gateFail, "evidence was transplanted from another fixture"
	}
	if required["actor"] && in.Binding.Identity.Actor == "" && in.Binding.Identity.Principal == "" {
		return gateBlocked, "MISSING_IDENTITY: actor"
	}
	if required["oracle"] {
		if in.Binding.Oracle == "" && in.ExpectedOracle == "" && in.Report.Oracle == "" {
			return gateBlocked, "MISSING_ORACLE"
		}
		if len(in.OracleFields) == 0 {
			return gateBlocked, "MISSING_ORACLE"
		}
		if !oracleAllowsBusinessPass(in.OracleFields) {
			return gateBlocked, "SYNTHETIC_ONLY: synthetic or unknown oracle fields cannot be the sole PASS evidence"
		}
	} else if len(in.OracleFields) > 0 && !oracleAllowsBusinessPass(in.OracleFields) {
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
			if strings.Contains(err.Error(), "target") {
				return gateStale, "deployment identity changed"
			}
			return gateFail, err.Error()
		}
		if err := rawMatchesCampaign(body, in); err != nil {
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
	if in.Report.UnsafeResume {
		return gateBlocked, "unreplayable side effect remains"
	}
	if in.Report.Truncated || in.Report.StopReason == "blocked" {
		return gatePartial, firstNonEmpty(in.Report.StopReason, "truncated")
	}
	if len(in.Report.Unspecified) > 0 {
		return gateUnspec, "contract-out observations remain"
	}
	return gatePass, "bound RAW, oracle and identity match the current run"
}

func rawMatchesCampaign(body []byte, in EvidenceGateInput) error {
	var raw StepRAW
	if err := json.Unmarshal(body, &raw); err != nil {
		return errors.New("RAW is not a StepRAW document")
	}
	// Versioned exploration evidence must bind the bytes to their originating
	// campaign, actor, fixture and independent observer. Absent fields are not
	// wildcards: a recomputed SHA256 cannot legalize stripped identity data.
	if in.Report.EvidenceContract == exploreIdentityContract {
		if in.ExpectedCampaign != "" && raw.Run != in.ExpectedCampaign {
			return errors.New("RAW campaign identity is missing or foreign")
		}
		if in.Binding.Identity.Task != "" && raw.Identity.Task != in.Binding.Identity.Task {
			return errors.New("RAW step identity is missing or foreign")
		}
		if in.ExpectedFixture != "" && raw.Identity.Fixture != in.ExpectedFixture {
			return errors.New("RAW fixture identity is missing or foreign")
		}
		if in.ExpectedActor != "" && raw.Identity.Actor != in.ExpectedActor && raw.Identity.Principal != in.ExpectedActor {
			return errors.New("RAW actor identity is missing or foreign")
		}
		if oracle := firstNonEmpty(in.ExpectedOracle, in.Binding.Oracle); oracle != "" && raw.Oracle != oracle {
			return errors.New("RAW independent oracle identity is missing or foreign")
		}
	}
	if in.ExpectedCampaign != "" && raw.Run != "" && raw.Run != in.ExpectedCampaign {
		return errors.New("RAW belongs to a foreign campaign")
	}
	if in.Binding.Identity.Task != "" && raw.Identity.Task != "" && raw.Identity.Task != in.Binding.Identity.Task {
		return errors.New("RAW belongs to a foreign step")
	}
	if in.ExpectedFixture != "" && raw.Identity.Fixture != "" && raw.Identity.Fixture != in.ExpectedFixture {
		return errors.New("RAW fixture does not match the current binding")
	}
	if in.ExpectedActor != "" && raw.Identity.Actor != "" && raw.Identity.Actor != in.ExpectedActor && raw.Identity.Principal != in.ExpectedActor {
		return errors.New("RAW actor does not match the current binding")
	}
	return nil
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
		return trustUnverified
	}
	verdict, reason := evaluateEvidenceGate(EvidenceGateInput{
		Report:           report,
		Binding:          binding,
		ExpectedCommit:   binding.Identity.SourceCommit,
		ExpectedGraph:    binding.Identity.GraphSHA256,
		ExpectedTarget:   binding.Identity.Target,
		ExpectedCampaign: report.Campaign,
		ExpectedFixture:  binding.Identity.Fixture,
		ExpectedActor:    firstNonEmpty(binding.Identity.Actor, binding.Identity.Principal),
		ExpectedOracle:   firstNonEmpty(binding.Oracle, report.Oracle),
		OracleFields:     report.OracleFields,
		PlannerCalls:     report.Spent.PlannerCalls,
		RequirePlanner:   report.Mode == "ai" || report.Mode == "agent",
	})
	return trustFromGate(report, verdict, reason)
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
		result := d.verifyExploreEvidence(campaign)
		evaluations = append(evaluations, map[string]string{
			"campaign":         result.CampaignID,
			"verdict":          result.GateVerdict,
			"reason":           result.InvalidReason,
			"evidence_trust":   result.EvidenceTrust,
			"business_verdict": result.BusinessVerdict,
		})
		if result.EvidenceTrust == trustLegacy {
			continue
		}
		verdict := EvidenceGateVerdict(result.GateVerdict)
		if verdict == gateFail || verdict == gateStale || verdict == gateBlocked {
			if worst != gateFail {
				worst, worstReason = verdict, result.InvalidReason
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
