package main

import (
	"strings"
	"time"
)

const (
	trustVerified        = "VERIFIED"
	trustUnverified      = "UNVERIFIED"
	trustUnbound         = "UNBOUND"
	trustIncomplete      = "INCOMPLETE"
	trustTampered        = "TAMPERED"
	trustStale           = "STALE"
	trustSynthetic       = "SYNTHETIC_ONLY"
	trustLegacy          = "LEGACY_UNVERIFIED"
	trustMissingIdentity = "MISSING_IDENTITY"
	trustMissingOracle   = "MISSING_ORACLE"
)

type VerifiedEvidenceResult struct {
	CampaignID         string   `json:"campaign_id"`
	GraphID            string   `json:"graph_id"`
	GraphSHA           string   `json:"graph_sha"`
	RunID              string   `json:"run_id"`
	SourceCommit       string   `json:"source_commit"`
	EnvironmentID      string   `json:"environment_id"`
	Target             string   `json:"target"`
	DeploymentIdentity string   `json:"deployment_identity"`
	FixtureID          string   `json:"fixture_id"`
	Actor              string   `json:"actor"`
	Role               string   `json:"role"`
	OracleIdentity     string   `json:"oracle_identity"`
	EvidenceIDs        []string `json:"evidence_ids"`
	Integrity          string   `json:"integrity"`
	BindingResult      string   `json:"binding"`
	OracleTrust        string   `json:"oracle_trust"`
	BusinessVerdict    string   `json:"business_verdict"`
	EvidenceTrust      string   `json:"evidence_trust"`
	VerifiedAt         string   `json:"verified_at"`
	InvalidReason      string   `json:"invalid_reason,omitempty"`
	GateVerdict        string   `json:"gate_verdict"`
}

func (d db) verifyExploreEvidence(campaign ExploreCheckpoint) VerifiedEvidenceResult {
	report := campaign.Report
	if report.Campaign == "" {
		report.Campaign = campaign.Campaign
		report.Graph = firstNonEmpty(report.Graph, campaign.Graph)
	}
	if len(report.OracleFields) == 0 {
		for _, step := range report.Steps {
			report.OracleFields = append(report.OracleFields, step.OracleFields...)
		}
	}
	binding := campaign.Binding
	if loaded, err := d.loadExploreBinding(firstNonEmpty(campaign.Campaign, report.Campaign)); err == nil {
		binding = mergeExploreBinding(binding, loaded)
	}
	binding = mergeExploreBinding(binding, RunBinding{Identity: report.Identity, Oracle: report.Oracle})
	if binding.Identity.Graph == "" {
		binding.Identity.Graph = firstNonEmpty(campaign.Graph, report.Graph)
	}
	if binding.Identity.SourceCommit == "" {
		binding.Identity.SourceCommit = campaign.SourceCommit
	}
	if binding.Identity.GraphSHA256 == "" {
		binding.Identity.GraphSHA256 = campaign.ModelSHA
	}
	if binding.Identity.Fixture == "" {
		binding.Identity.Fixture = campaign.Fixture
	}
	if binding.Identity.Environment == "" {
		binding.Identity.Environment = campaign.Environment
	}
	result := VerifiedEvidenceResult{
		CampaignID:         firstNonEmpty(campaign.Campaign, report.Campaign),
		GraphID:            firstNonEmpty(binding.Identity.Graph, campaign.Graph, report.Graph),
		GraphSHA:           firstNonEmpty(binding.Identity.GraphSHA256, campaign.ModelSHA),
		RunID:              firstNonEmpty(campaign.Campaign, report.Campaign),
		SourceCommit:       firstNonEmpty(binding.Identity.SourceCommit, campaign.SourceCommit),
		EnvironmentID:      firstNonEmpty(binding.Identity.Environment, campaign.Environment, report.Environment),
		Target:             binding.Identity.Target,
		DeploymentIdentity: firstNonEmpty(binding.Identity.Target, binding.Identity.ConfigSHA256),
		FixtureID:          firstNonEmpty(binding.Identity.Fixture, campaign.Fixture),
		Actor:              firstNonEmpty(binding.Identity.Actor, binding.Identity.Principal),
		Role:               binding.Identity.Actor,
		OracleIdentity:     firstNonEmpty(binding.Oracle, report.Oracle),
		EvidenceIDs:        evidenceIDs(report),
		BusinessVerdict:    classifyBusinessVerdict(report),
		VerifiedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		Integrity:          "UNKNOWN",
		BindingResult:      "UNKNOWN",
		OracleTrust:        "UNKNOWN",
		EvidenceTrust:      trustUnverified,
	}
	verdict, reason := evaluateEvidenceGate(EvidenceGateInput{
		Report:           report,
		Binding:          binding,
		ExpectedCommit:   result.SourceCommit,
		ExpectedGraph:    result.GraphSHA,
		ExpectedTarget:   result.Target,
		ExpectedCampaign: result.CampaignID,
		ExpectedFixture:  result.FixtureID,
		ExpectedActor:    result.Actor,
		ExpectedOracle:   result.OracleIdentity,
		PlannerCalls:     report.Spent.PlannerCalls,
		RequirePlanner:   campaign.Mode == "ai" || report.Mode == "ai",
		OracleFields:     report.OracleFields,
	})
	result.GateVerdict = string(verdict)
	result.InvalidReason = reason
	result.EvidenceTrust = trustFromGate(report, verdict, reason)
	result.Integrity, result.BindingResult, result.OracleTrust = splitTrustParts(result.EvidenceTrust, reason)
	return result
}

func mergeExploreBinding(base, extra RunBinding) RunBinding {
	if base.Identity.Graph == "" {
		base.Identity.Graph = extra.Identity.Graph
	}
	if base.Identity.Task == "" {
		base.Identity.Task = extra.Identity.Task
	}
	if base.Identity.Environment == "" {
		base.Identity.Environment = extra.Identity.Environment
	}
	if base.Identity.Target == "" {
		base.Identity.Target = extra.Identity.Target
	}
	if base.Identity.SourceCommit == "" {
		base.Identity.SourceCommit = extra.Identity.SourceCommit
	}
	if base.Identity.SourceSHA256 == "" {
		base.Identity.SourceSHA256 = extra.Identity.SourceSHA256
	}
	if base.Identity.ConfigSHA256 == "" {
		base.Identity.ConfigSHA256 = extra.Identity.ConfigSHA256
	}
	if base.Identity.GraphSHA256 == "" {
		base.Identity.GraphSHA256 = extra.Identity.GraphSHA256
	}
	if base.Identity.Actor == "" {
		base.Identity.Actor = extra.Identity.Actor
	}
	if base.Identity.Account == "" {
		base.Identity.Account = extra.Identity.Account
	}
	if base.Identity.Principal == "" {
		base.Identity.Principal = extra.Identity.Principal
	}
	if base.Identity.Fixture == "" {
		base.Identity.Fixture = extra.Identity.Fixture
	}
	if base.Identity.FixtureSHA256 == "" {
		base.Identity.FixtureSHA256 = extra.Identity.FixtureSHA256
	}
	if base.Oracle == "" {
		base.Oracle = extra.Oracle
	}
	if base.SourceRoot == "" {
		base.SourceRoot = extra.SourceRoot
	}
	if base.ConfigPath == "" {
		base.ConfigPath = extra.ConfigPath
	}
	return base
}

func evidenceIDs(report ExploreReport) []string {
	out := []string{}
	for _, step := range report.Steps {
		if step.RAW != "" {
			out = appendUnique(out, step.RAW)
		}
		if step.SHA256 != "" {
			out = appendUnique(out, step.SHA256)
		}
	}
	return out
}

func trustFromGate(report ExploreReport, verdict EvidenceGateVerdict, reason string) string {
	if len(report.Steps) == 0 {
		return trustUnverified
	}
	if report.EvidenceContract == "" && report.Identity.Target == "" && report.Identity.SourceCommit == "" {
		return trustLegacy
	}
	lower := strings.ToLower(reason)
	switch {
	case strings.Contains(reason, "MISSING_IDENTITY"):
		return trustMissingIdentity
	case strings.Contains(reason, "MISSING_ORACLE"):
		return trustMissingOracle
	case strings.Contains(lower, "digest"):
		return trustTampered
	case strings.Contains(lower, "tamper"):
		return trustTampered
	case strings.Contains(reason, "SYNTHETIC_ONLY") || strings.Contains(lower, "synthetic"):
		return trustSynthetic
	case strings.Contains(lower, "deployment identity") || strings.Contains(lower, "stale") || strings.Contains(lower, "commit") && strings.Contains(lower, "match"):
		return trustStale
	case strings.Contains(lower, "transplant") || strings.Contains(lower, "another campaign") || strings.Contains(lower, "another target") || strings.Contains(lower, "foreign"):
		return trustUnbound
	case strings.Contains(lower, "missing raw") || strings.Contains(lower, "missing RAW") || strings.Contains(lower, "a step is missing"):
		return trustIncomplete
	}
	switch verdict {
	case gatePass, gatePartial, gateUnspec, gateFail:
		if verdict == gateFail && (strings.Contains(lower, "digest") || strings.Contains(lower, "missing")) {
			return trustIncomplete
		}
		return trustVerified
	case gateStale:
		return trustStale
	case gateBlocked, gateNotRun:
		if strings.Contains(reason, "MISSING_IDENTITY") {
			return trustMissingIdentity
		}
		if strings.Contains(reason, "MISSING_ORACLE") || strings.Contains(reason, "SYNTHETIC") {
			if strings.Contains(reason, "SYNTHETIC") {
				return trustSynthetic
			}
			return trustMissingOracle
		}
		return trustIncomplete
	default:
		return trustUnverified
	}
}

func splitTrustParts(trust, reason string) (integrity, binding, oracle string) {
	integrity, binding, oracle = trust, trust, trust
	switch trust {
	case trustTampered:
		integrity = trustTampered
	case trustMissingIdentity, trustUnbound, trustStale:
		binding = trust
	case trustMissingOracle, trustSynthetic:
		oracle = trust
	case trustVerified:
		integrity, binding, oracle = "OK", "BOUND", "OBSERVED"
	}
	_ = reason
	return integrity, binding, oracle
}

func requiredExploreIdentities(report ExploreReport) map[string]bool {
	req := map[string]bool{}
	if report.EvidenceContract != exploreIdentityContract {
		return req
	}
	if len(report.Steps) == 0 && report.StopReason != "complete" {
		return req
	}
	if len(report.Steps) == 0 {
		return req
	}
	req["target"] = true
	req["environment"] = true
	req["commit"] = true
	req["graph"] = true
	req["campaign"] = true
	if exploreHasSideEffects(report) {
		req["fixture"] = true
		req["actor"] = true
	}
	req["oracle"] = true
	return req
}

func exploreHasSideEffects(report ExploreReport) bool {
	for _, step := range report.Steps {
		if sideEffecting(step.Candidate) {
			return true
		}
		for _, call := range step.Calls {
			if call.SideEffect {
				return true
			}
		}
	}
	return false
}

func missingExploreIdentities(in EvidenceGateInput, required map[string]bool) []string {
	missing := []string{}
	id := in.Binding.Identity
	if required["graph"] && id.Graph == "" {
		missing = append(missing, "graph")
	}
	if required["commit"] && (id.SourceCommit == "" || in.ExpectedCommit == "") {
		missing = append(missing, "source_commit")
	}
	if required["target"] && (id.Target == "" || in.ExpectedTarget == "") {
		missing = append(missing, "target")
	}
	if required["environment"] && id.Environment == "" {
		missing = append(missing, "environment")
	}
	if required["fixture"] && (id.Fixture == "" || in.ExpectedFixture == "") {
		missing = append(missing, "fixture")
	}
	if required["actor"] && (id.Actor == "" && id.Principal == "" || in.ExpectedActor == "") {
		missing = append(missing, "actor")
	}
	if required["campaign"] && in.ExpectedCampaign == "" && in.Report.Campaign == "" {
		missing = append(missing, "campaign")
	}
	return missing
}
