package main

import (
	"encoding/json"
	"testing"
)

func TestBusinessContractGateSeparatesFiniteExecutionFromBusinessE2E(t *testing.T) {
	g := runtimeBusinessGraph(t)
	g.Scope = "One active-account input in one pending-state operation; no complete business journey"
	g.Rules[0].Conditions = nil
	g.Tests, g.Invariants, g.Transitions = nil, nil, nil
	edges := []Edge{}
	for _, edge := range g.Edges {
		if edge.Kind != "requires" && edge.Kind != "verifies" && edge.Kind != "transitions_to" {
			edges = append(edges, edge)
		}
	}
	g.Edges = edges
	for _, kind := range []TestKind{testBoundary, testConcurrency, testIdempotency, testRecovery, testFault} {
		g.Applicability = append(g.Applicability, ScenarioApplicability{Capability: "enrollment.submit", Kind: kind,
			Applicable: businessBool(false), Reason: "Outside the declared single-input tool acceptance scope", Source: "document", SourceRef: "test-spec:single-operation-scope"})
	}
	g.BusinessContract.Branches = g.BusinessContract.Branches[:1]
	g.BusinessContract.Coverage = &BusinessCoverageDomain{Scope: "Only active=true and pending; no path or full-system claim",
		Domains:  []BusinessFiniteDomain{{Subject: "account", Attribute: "active", Values: []json.RawMessage{json.RawMessage("true")}}},
		MaxCases: 8, Scopes: []BusinessCoverageScope{{Action: "enroll", States: []string{"pending"}}}}
	if err := validateGraph(g); err != nil {
		t.Fatal(err)
	}
	target, server := newRuntimeBusinessTarget(t, true)
	fixture := setupRuntimeBusiness(t, g, target, server, "accept", nil)
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "PASS" || target.requests != 1 {
		t.Fatalf("single target did not actually execute: result=%+v requests=%d", result, target.requests)
	}
	for _, view := range []struct {
		name string
		load func() (map[string]any, error)
	}{
		{"gate", func() (map[string]any, error) { return fixture.d.gate(g.ID) }},
		{"acceptance", func() (map[string]any, error) { return fixture.d.acceptanceReport(g.ID) }},
	} {
		t.Run(view.name, func(t *testing.T) {
			gate, err := view.load()
			if err != nil {
				t.Fatal(err)
			}
			proof := gate["contract_proof"].(*BusinessCoverageProof)
			if gate["verdict"] != "PASS" || proof.Status != "PROVEN_IN_SCOPE" || proof.CheckedCases != 1 {
				t.Fatalf("declared finite scope did not pass: %+v", gate)
			}
			complete := gate["completeness"].(map[string]any)
			if complete["complete_business_e2e"] != false {
				t.Fatalf("one operation and one finite input claimed complete business E2E: %+v", complete)
			}
			if complete["complete_in_declared_scope"] != true || complete["business_e2e_status"] != "NOT_EVALUATED" {
				t.Fatalf("finite-scope success and unevaluated journey must remain distinct: %+v", complete)
			}
			t.Logf("target_requests=%d checked_cases=%d completeness=%+v", target.requests, proof.CheckedCases, complete)
		})
	}
}
