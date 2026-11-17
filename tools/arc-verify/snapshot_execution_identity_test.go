package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A snapshot may retain a historical PASS, but it must not qualify that run as
// current evidence after the program that actually ran changes. The command
// calls the real HTTP test target; no output-only stub is used as its oracle.
func TestSnapshotRevalidatesActualCommandProgramBeforeCountingBranchCoverage(t *testing.T) {
	for _, change := range []string{"unchanged", "actual-command-program", "candidate-source"} {
		t.Run(change, func(t *testing.T) {
			target, server := newRuntimeBusinessTarget(t, true)
			graph := runtimeBusinessGraph(t)
			graph.BusinessContract.Branches = graph.BusinessContract.Branches[:1]
			// A command captures its exit status; HTTP status belongs to the
			// command's own target call and is not fabricated by the adapter.
			graph.BusinessContract.Branches[0].Expected.Outcomes[0].Checks = []BusinessExpectedCheck{{
				ID: "command-status", Kind: "response", Oracle: "response-capture", SourceRef: "test-spec:command-exit",
				Predicate: businessPredicate("eq", "transport", "exit_code", "integer", "0"),
			}}
			fixture := prepareExecutionFixture(t, graph, server.URL)
			target.ledger = &fixture.d
			directory := t.TempDir() // Explicit runner directory outside the candidate tree.
			program, marker := filepath.Join(directory, "action.py"), filepath.Join(directory, "invocations.txt")
			if err := os.WriteFile(program, commandIdentityProgram(t, marker, "original"), 0700); err != nil {
				t.Fatal(err)
			}
			decoy := t.TempDir()
			if err := os.WriteFile(filepath.Join(decoy, "action.py"), []byte("#!/bin/sh\nprintf '{}\\n'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Chdir(decoy)
			body, err := os.ReadFile(fixtureManifest(t, "snapshot-command-identity", "R1/POSITIVE"))
			if err != nil {
				t.Fatal(err)
			}
			var manifest RunnerManifest
			if err := decodeStrictJSON(body, &manifest); err != nil {
				t.Fatal(err)
			}
			step := &manifest.Steps[0]
			step.Call = AdapterCall{Kind: adapterCommand, Args: []string{"./action.py"}, Directory: directory, SuccessExitCodes: []int{0}}
			step.BusinessBranch = "accept"
			step.SubjectBindings = map[string]string{"account": "actor.primary.principal"}
			step.Oracles[0].Measurements["account.active"] = "/active"
			step.Oracles[0].Measurements["order.stored"] = "/stored"
			commandIdentityRegisterAndPlan(t, &fixture, manifest)
			result, err := commandIdentityExecute(fixture, manifest)
			if err != nil || result.Verdict != "PASS" {
				t.Fatalf("control command did not perform a verified action: %+v %v", result, err)
			}
			request := SnapshotRequest{Graph: graph.ID, Environment: "independent", Round: fixture.round, Config: fixture.config, Runs: []string{result.Run}}
			before, err := fixture.d.buildSnapshot(request)
			if err != nil || len(before.Runs) != 1 || before.Runs[0].EvidenceStatus != "PASS" || before.ContractExecution == nil || before.ContractExecution.Status != "PASS" || before.ContractExecution.Passed != 1 {
				t.Fatalf("control snapshot has no verified branch execution: %+v %v", before.ContractExecution, err)
			}
			captures, err := fixture.d.query("SELECT CAST(idx AS TEXT),sha256 FROM model_captures WHERE run="+quote(result.Run)+" ORDER BY idx", 2)
			if err != nil || len(captures) != 3 {
				t.Fatalf("control did not persist setup/action/recovery RAW: %v %v", captures, err)
			}
			_, sourceBefore, err := sourceFingerprint(fixture.root)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "actual-command-program":
				if err := os.WriteFile(program, commandIdentityProgram(t, marker, "changed-after-pass"), 0700); err != nil {
					t.Fatal(err)
				}
			case "candidate-source":
				if err := os.WriteFile(filepath.Join(fixture.root, "business.txt"), []byte("changed candidate after the recorded PASS"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, sourceAfter, err := sourceFingerprint(fixture.root)
			if err != nil || (sourceBefore == sourceAfter) != (change != "candidate-source") {
				t.Fatalf("test did not isolate candidate vs command identity: %s %s %v", sourceBefore, sourceAfter, err)
			}
			after, err := fixture.d.buildSnapshot(request)
			if err != nil || len(after.Runs) != 1 || after.ContractExecution == nil {
				t.Fatalf("history must remain exportable: %+v %v", after.ContractExecution, err)
			}
			run := after.Runs[0]
			if run.Trace.Status != "PASS" || len(run.Trace.Steps) != 1 || run.Trace.Steps[0].Verdict != "PASS" {
				t.Fatalf("revalidation rewrote the historical execution verdict: %+v", run.Trace)
			}
			retained, err := fixture.d.query("SELECT CAST(idx AS TEXT),sha256 FROM model_captures WHERE run="+quote(result.Run)+" ORDER BY idx", 2)
			oldJSON, _ := json.Marshal(captures)
			newJSON, _ := json.Marshal(retained)
			if err != nil || string(oldJSON) != string(newJSON) {
				t.Fatalf("snapshot changed or discarded historical RAW receipts: %v %v", retained, err)
			}
			target.mu.Lock()
			requests := target.requests
			target.mu.Unlock()
			invocations, err := os.ReadFile(marker)
			if err != nil || requests != 1 || string(invocations) != "original\n" {
				t.Fatalf("read-only revalidation replayed the target: requests=%d marker=%q error=%v", requests, invocations, err)
			}
			if change == "unchanged" {
				if run.EvidenceStatus != "PASS" || after.ContractExecution.Status != "PASS" || after.ContractExecution.Passed != 1 {
					t.Fatalf("unchanged inputs lost their qualified branch evidence: %+v evidence=%s", after.ContractExecution, run.EvidenceStatus)
				}
			} else if run.EvidenceStatus != "BLOCKED" || after.ContractExecution.Status != "BLOCKED" || after.ContractExecution.Passed != 0 || len(after.ContractExecution.EvidenceErrors) == 0 {
				t.Fatalf("changed actual inputs still qualify old PASS: evidence=%s coverage=%+v", run.EvidenceStatus, after.ContractExecution)
			}
		})
	}
}
