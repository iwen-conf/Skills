package main

import (
	"encoding/json"
	"os"
	"testing"
)

// Optional integration artifact: real HTTP calls to the tool's explicit test
// target, exported by the production snapshot path. Never product E2E evidence.
func TestWorkbenchExportObservedToolRun(t *testing.T) {
	out, assets := os.Getenv("ARC_WORKBENCH_EXPORT_DIR"), os.Getenv("ARC_WORKBENCH_ASSETS")
	if out == "" || assets == "" {
		t.Skip("set ARC_WORKBENCH_EXPORT_DIR and ARC_WORKBENCH_ASSETS to export a tool-run integration artifact")
	}
	target, server := newRuntimeBusinessTarget(t, false)
	graph := runtimeInputGraph(t, "100")
	graph.Nodes = append(graph.Nodes, Node{ID: "activate", Kind: "action", Name: "激活测试账户"})
	graph.Edges = append(graph.Edges, Edge{ID: "impl-activate", From: "enrollment.submit", To: "activate", Kind: "implements"})
	active := businessPredicate("eq", "account", "active", "boolean", "true")
	graph.BusinessContract.Preparation = []PreparationAction{{ID: "activate-account", Action: "activate", Requires: businessNot(active), Ensures: active,
		Effects: []PreparationEffect{{Subject: "account", Attribute: "active", Value: json.RawMessage(`true`)}}, Source: "document", SourceRef: "test-spec:activate"}}
	fixture := setupRuntimeBusiness(t, graph, target, server, "accept", func(m *RunnerManifest) {
		m.Steps[0].Call.Body = json.RawMessage(`{"amount":100,"currency":"CNY","merchant_no":"{{run.id}}"}`)
		m.Steps[0].InputMeasurements = map[string]string{"request.amount": "/amount", "request.currency": "/currency", "request.merchant_no": "/merchant_no"}
		observer := m.Steps[0]
		observer.BusinessBranch, observer.InputMeasurements = "", nil
		m.Preparation = &RunnerPreparation{Goal: active, MaxSteps: 1, MaxStates: 16, Observer: observer,
			Adapters: map[string]RunnerStep{"activate-account": {Node: "activate", Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: "POST", Path: "/activate", SuccessStatuses: []int{200}}}}}
		checkpointManifest(t, m, "ready")
	})
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "PASS" || target.requests != 1 || target.preparations != 1 || !target.preFrozen {
		t.Fatal("tool fixture did not produce a real verified HTTP operation", result)
	}
	path, err := fixture.d.writeSnapshot(SnapshotRequest{Graph: "course-v2", Environment: "independent", Round: fixture.round,
		Config: fixture.config, Assets: assets, Out: out, Runs: []string{result.Run}, Argv: []string{"tool-integration-fixture", "not-pay-gateway-acceptance"}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := readSnapshot(t, path)
	if len(snapshot.Runs) != 1 || snapshot.Runs[0].EvidenceStatus != "PASS" {
		t.Fatal("native workbench output lost validated runtime evidence")
	}
	prepared := false
	for _, fact := range snapshot.Runs[0].Facts {
		if fact.Seq == -1 && fact.Preparation != nil {
			p := fact.Preparation
			prepared = p.TargetReady && len(p.Steps) == 1 && string(p.InitialObserved.Values["account.active"]) == "false" &&
				string(p.FinalObserved.Values["account.active"]) == "true" && p.Steps[0].Status == "PASS" && p.Steps[0].Receipt.Action.RequestSHA256 != ""
		}
	}
	if !prepared {
		t.Fatal("export lost the actual preparation action and independent false-to-true readbacks")
	}
	t.Logf("observed tool run %s exported to %s; product E2E remains unproven", result.Run, path)
}
