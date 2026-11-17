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
	target, server := newRuntimeBusinessTarget(t, true)
	fixture := setupRuntimeBusiness(t, runtimeInputGraph(t, "100"), target, server, "accept", func(m *RunnerManifest) {
		m.Steps[0].Call.Body = json.RawMessage(`{"amount":100,"currency":"CNY","merchant_no":"{{run.id}}"}`)
		m.Steps[0].InputMeasurements = map[string]string{"request.amount": "/amount", "request.currency": "/currency", "request.merchant_no": "/merchant_no"}
		checkpointManifest(t, m, "ready")
	})
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "PASS" || target.requests != 1 || !target.preFrozen {
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
	t.Logf("observed tool run %s exported to %s; product E2E remains unproven", result.Run, path)
}
