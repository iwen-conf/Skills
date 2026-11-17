package main

import (
	"encoding/json"
	"os"
	"testing"
)

func preparationLineageGraph(t *testing.T) Graph {
	t.Helper()
	graph := runtimeBusinessGraph(t)
	graph.Nodes = append(graph.Nodes, Node{ID: "activate", Kind: "action", Name: "激活测试账户"})
	graph.Edges = append(graph.Edges, Edge{ID: "impl-activate", From: "enrollment.submit", To: "activate", Kind: "implements"})
	active := businessPredicate("eq", "account", "active", "boolean", "true")
	graph.BusinessContract.Preparation = []PreparationAction{{ID: "activate-account", Action: "activate", Requires: businessNot(active), Ensures: active,
		Effects: []PreparationEffect{{Subject: "account", Attribute: "active", Value: json.RawMessage(`true`)}}, Source: "document", SourceRef: "test-spec:activate"}}
	return graph
}

func preparationLineageFixture(t *testing.T, lying bool) (Graph, executionFixture, ExecuteResult) {
	t.Helper()
	target, server := newRuntimeBusinessTarget(t, false)
	target.lyingPreparation = lying
	graph := preparationLineageGraph(t)
	active := businessPredicate("eq", "account", "active", "boolean", "true")
	fixture := setupRuntimeBusiness(t, graph, target, server, "accept", func(m *RunnerManifest) {
		observer := m.Steps[0]
		observer.BusinessBranch = ""
		m.Preparation = &RunnerPreparation{Goal: active, MaxSteps: 1, MaxStates: 16, Observer: observer,
			Adapters: map[string]RunnerStep{"activate-account": {Node: "activate", Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: "POST", Path: "/activate", SuccessStatuses: []int{200}}}}}
	})
	result := executeRuntimeBusiness(t, fixture)
	want, calls := "PASS", 1
	if lying {
		want, calls = "BLOCKED", 0
	}
	if result.Verdict != want || target.requests != calls || target.preparations != 1 {
		t.Fatalf("real tool HTTP preparation did not exercise the intended scenario: %+v requests=%d preparation=%d", result, target.requests, target.preparations)
	}
	return graph, fixture, result
}

func preparationLineageSnapshot(t *testing.T, fixture executionFixture, result ExecuteResult) Snapshot {
	t.Helper()
	snapshot, err := fixture.d.buildSnapshot(SnapshotRequest{Graph: "course-v2", Environment: "independent", Round: fixture.round,
		Config: fixture.config, Runs: []string{result.Run}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Runs) != 1 {
		t.Fatalf("the actual run was not exported: %+v", snapshot.Runs)
	}
	return snapshot
}

func TestBusinessLineagePreparationSnapshotShowsActualReadbackHistory(t *testing.T) {
	_, fixture, result := preparationLineageFixture(t, false)
	snapshot := preparationLineageSnapshot(t, fixture, result)
	_, setupHash, err := fixture.d.workbenchEvidence(result.Run, "setup", 0)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, overlay := range snapshot.Runs[0].BusinessLineage {
		if overlay.Phase != "preparation" {
			continue
		}
		count++
		if overlay.Branch != "accept" || overlay.Run != result.Run || overlay.Step != -1 || overlay.ActionIndex != 0 || overlay.Actor != "primary" || overlay.ReadbackIndex == nil || overlay.EvidenceSHA256 != setupHash || overlay.EvidenceStatus != "PASS" || overlay.Diagnostic {
			t.Fatalf("preparation history lost its own manifest/RAW/readback scope: %+v", overlay)
		}
		want := "false"
		if overlay.ReadbackPhase == "post_action" {
			want = "true"
		}
		found := false
		for _, node := range overlay.Nodes {
			if node.Status == "READY" {
				t.Fatalf("a historical preparation readback must not claim target execution readiness: %+v", node)
			}
			for _, value := range node.Values {
				if value.Subject == "account" && value.Attribute == "active" {
					found = true
					if string(value.Actual) != want || value.Binding != "synthetic-STUDENT" || len(value.Sources) != 1 || value.Sources[0].Kind != "preparation_readback" || value.Sources[0].SHA256 != setupHash || value.Sources[0].Oracle == "" {
						t.Fatalf("preparation readback borrowed final state or fabricated an Oracle response hash: %+v", value)
					}
				}
			}
		}
		if !found {
			t.Fatal("actual account state vanished from preparation genealogy")
		}
	}
	if count != 3 {
		t.Fatalf("need initial false, pre-action false and post-action true; got %d preparation overlays", count)
	}
}

func TestBusinessLineagePreparationFailureRemainsDiagnosticWithoutTargetFreeze(t *testing.T) {
	_, fixture, result := preparationLineageFixture(t, true)
	snapshot := preparationLineageSnapshot(t, fixture, result)
	count, falsified := 0, false
	for _, overlay := range snapshot.Runs[0].BusinessLineage {
		if overlay.Phase != "preparation" {
			t.Fatalf("blocked preparation produced a target branch occurrence: %+v", overlay)
		}
		count++
		if overlay.Branch != "accept" || overlay.Actor != "primary" || !overlay.Diagnostic || overlay.EvidenceStatus != "BLOCKED" || overlay.Reason == "" {
			t.Fatalf("failed preparation was promoted or lost its declared target: %+v", overlay)
		}
		for _, node := range overlay.Nodes {
			if node.Status != "BLOCKED" {
				t.Fatalf("diagnostic observations cannot grant readiness: %+v", node)
			}
			if node.Node == "branch:accept:when" && overlay.ReadbackPhase == "post_action" {
				falsified = node.Evaluation != nil && node.Evaluation.Value == businessFalse && len(node.Values) == 1 && string(node.Values[0].Actual) == "false"
			}
		}
	}
	if count != 3 || !falsified {
		t.Fatalf("the user cannot locate the actual failed prerequisite: readbacks=%d false-poststate=%v", count, falsified)
	}
	if out, assets := os.Getenv("ARC_PREPARATION_BLOCKED_EXPORT_DIR"), os.Getenv("ARC_WORKBENCH_ASSETS"); out != "" && assets != "" {
		path, err := fixture.d.writeSnapshot(SnapshotRequest{Graph: "course-v2", Environment: "independent", Round: fixture.round,
			Config: fixture.config, Assets: assets, Out: out, Runs: []string{result.Run}, Argv: []string{"blocked-preparation-tool-integration", "not-pay-gateway-acceptance"}})
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("observed failed tool preparation exported to %s; no pay_gateway acceptance", path)
	}
}

func TestBusinessLineagePreparationDamagedOriginalRAWCannotBecomeSatisfied(t *testing.T) {
	_, fixture, result := preparationLineageFixture(t, false)
	record, err := fixture.d.modelRun(result.Run)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(record.SetupEvidence)
	if err != nil {
		t.Fatal(err)
	}
	var raw StepRAW
	if err := decodeStrictJSON(body, &raw); err != nil {
		t.Fatal(err)
	}
	raw.Model.Preparation.Readbacks[0].Observation.Values["account.active"] = json.RawMessage(`true`)
	body, err = json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record.SetupEvidence, body, 0600); err != nil {
		t.Fatal(err)
	}
	snapshot := preparationLineageSnapshot(t, fixture, result)
	if snapshot.Runs[0].EvidenceStatus != "BLOCKED" {
		t.Fatal("damaged setup evidence retained a passing run evidence status")
	}
	for _, overlay := range snapshot.Runs[0].BusinessLineage {
		for _, node := range overlay.Nodes {
			if node.Status == "SATISFIED" || node.Status == "READY" {
				t.Fatalf("tampered setup RAW still created a green dependency: %+v", node)
			}
		}
	}
}

func TestBusinessLineagePreparationDoesNotClaimLaterUnfrozenBranch(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, false)
	graph := preparationLineageGraph(t)
	active := businessPredicate("eq", "account", "active", "boolean", "true")
	fixture := setupRuntimeBusiness(t, graph, target, server, "accept", func(m *RunnerManifest) {
		observer := m.Steps[0]
		observer.BusinessBranch = ""
		m.Preparation = &RunnerPreparation{Goal: active, MaxSteps: 1, MaxStates: 16, Observer: observer,
			Adapters: map[string]RunnerStep{"activate-account": {Node: "activate", Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: "POST", Path: "/activate", SuccessStatuses: []int{200}}}}}
		later := m.Steps[0]
		later.BusinessBranch = "reject"
		m.Steps = append(m.Steps, later)
	})
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "BLOCKED" || target.requests != 1 || target.preparations != 1 {
		t.Fatalf("the later branch should not match the real state after the first target: %+v requests=%d", result, target.requests)
	}
	snapshot := preparationLineageSnapshot(t, fixture, result)
	preparations := 0
	for _, overlay := range snapshot.Runs[0].BusinessLineage {
		if overlay.Phase == "preparation" {
			preparations++
			if overlay.Branch != "accept" {
				t.Fatalf("initial preparation was attached to a later independent branch: %+v", overlay)
			}
		}
	}
	if preparations != 3 {
		t.Fatalf("the actually prepared initial target lost its own readbacks: %d", preparations)
	}
}
