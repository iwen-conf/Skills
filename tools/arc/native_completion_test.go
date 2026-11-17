package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func insertNativeCompletionLink(t *testing.T, db *sql.DB, task, obligation string) nativeTaskLink {
	t.Helper()
	link := nativeTaskLink{Graph: "native-self-test", ObligationID: obligation, VerifyTaskID: "native-self-test/" + obligation, Updated: "2026-11-17T00:00:00Z"}
	if _, err := db.Exec("INSERT INTO arc_task_obligations(arc_task_id,graph,obligation_id,verify_task_id,updated) VALUES(?,?,?,?,?)", task, link.Graph, link.ObligationID, link.VerifyTaskID, link.Updated); err != nil {
		t.Fatal(err)
	}
	return link
}

// Adapter fixture only: the verifier module separately tests real RAW revalidation.
func testNativeTaskProof(t *testing.T, root string, link nativeTaskLink) nativeTaskProof {
	t.Helper()
	head, err := gitHead(root)
	if err != nil {
		t.Fatal(err)
	}
	return nativeTaskProof{Version: 1, Graph: link.Graph, Task: link.VerifyTaskID, ObligationID: link.ObligationID, Run: "native-synthetic-run", Round: "native-synthetic-round", Verdict: "PASS", Reason: "adapter test fixture", ProofScope: "linked_task_evidence", InputsUnchanged: true, EvidenceStatus: "PASS", Context: nativeProofContext{Graph: link.Graph, Environment: "isolated", Target: "adapter-self-test", SourceCommit: head, SourceSHA256: strings.Repeat("a", 64), ConfigSHA256: strings.Repeat("b", 64), GraphSHA256: strings.Repeat("c", 64), SourceRoot: root, ConfigPath: filepath.Join(root, ".arc", "verify.jsonc")}, RAWSHA256: []string{strings.Repeat("d", 64)}, ReceiptSHA256: strings.Repeat("e", 64)}
}

func TestNativeProofRejectsWrongScopeIdentityAndMissingEvidence(t *testing.T) {
	root, db := testArcState(t)
	defer db.Close()
	initClosureRepo(t, root)
	link := nativeTaskLink{Graph: "G", ObligationID: "O", VerifyTaskID: "G/O"}
	valid := testNativeTaskProof(t, root, link)
	tests := []struct {
		name   string
		mutate func(*nativeTaskProof)
	}{
		{"wrong-graph", func(p *nativeTaskProof) { p.Graph = "other" }},
		{"wrong-task", func(p *nativeTaskProof) { p.Task = "G/another" }},
		{"wrong-obligation-override", func(p *nativeTaskProof) { p.ObligationID = "unrelated" }},
		{"bare-table-pass", func(p *nativeTaskProof) { p.EvidenceStatus = ""; p.RAWSHA256 = nil }},
		{"legacy", func(p *nativeTaskProof) { p.EvidenceStatus = "LEGACY" }},
		{"authorization", func(p *nativeTaskProof) { p.Verdict = "AUTHORIZED" }},
		{"stale", func(p *nativeTaskProof) { p.InputsUnchanged = false }},
		{"wrong-scope", func(p *nativeTaskProof) { p.ProofScope = "whole_graph" }},
		{"wrong-root", func(p *nativeTaskProof) { p.Context.SourceRoot = t.TempDir() }},
		{"old-head", func(p *nativeTaskProof) { p.Context.SourceCommit = "old" }},
		{"empty-round", func(p *nativeTaskProof) { p.Round = "" }},
		{"empty-environment", func(p *nativeTaskProof) { p.Context.Environment = "" }},
		{"invalid-raw-hash", func(p *nativeTaskProof) { p.RAWSHA256 = []string{"not-a-hash"} }},
		{"duplicate-raw-hash", func(p *nativeTaskProof) { p.RAWSHA256 = append(p.RAWSHA256, p.RAWSHA256[0]) }},
		{"missing-receipt", func(p *nativeTaskProof) { p.ReceiptSHA256 = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proof := valid
			proof.RAWSHA256 = append([]string{}, valid.RAWSHA256...)
			test.mutate(&proof)
			body, _ := json.Marshal(proof)
			if _, _, err := acceptNativeTaskProof(root, valid.Context.SourceCommit, link, body); err == nil {
				t.Fatalf("accepted %s", test.name)
			}
		})
	}
	body, _ := json.Marshal(valid)
	_, hash, err := acceptNativeTaskProof(root, valid.Context.SourceCommit, link, body)
	if err != nil || !validEvidenceSHA(hash) {
		t.Fatalf("valid adapter proof rejected: %s %v", hash, err)
	}
	pretty, _ := json.MarshalIndent(valid, "", "  ")
	_, sameHash, err := acceptNativeTaskProof(root, valid.Context.SourceCommit, link, pretty)
	if err != nil || sameHash != hash {
		t.Fatal("proof fingerprint depends on JSON formatting")
	}
}

func TestNativeCompletionRequiresLinksAndNeverFallsBackAfterFailure(t *testing.T) {
	root, db := testArcState(t)
	defer db.Close()
	initClosureRepo(t, root)
	calls := 0
	runner := func(context.Context, string, nativeTaskLink) ([]byte, error) {
		calls++
		return nil, errors.New("native proof rejected")
	}
	if _, err := loadNativeCompletionProof(root, "T", runner); err == nil || !strings.Contains(err.Error(), "no linked") {
		t.Fatalf("missing-link result: %v", err)
	}
	if calls != 0 {
		t.Fatal("verification attempted without a declared scope")
	}
	insertNativeCompletionLink(t, db, "T", "O1")
	if _, err := loadNativeCompletionProof(root, "T", runner); err == nil || !strings.Contains(err.Error(), "native proof rejected") {
		t.Fatalf("failure did not remain closed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("unexpected retries or fallback: %d", calls)
	}
	legacy, err := requirePolicyProof("T", testPolicyReceipt("T", "PASS"))
	if err != nil {
		t.Fatal(err)
	}
	if err := commitTaskCompletion(db, root, "T", 1, legacy); err == nil {
		t.Fatal("historical policy receipt authorized new completion")
	}
}

func TestNativeCompletionSourceAndLinkedScopeRaces(t *testing.T) {
	root, db := testArcState(t)
	initClosureRepo(t, root)
	const id = "native-race"
	if err := executeTask(db, "ensure", id, "race test", "", 0); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		to string
		v  int
	}{{"ready", 1}, {"running", 2}, {"verifying", 3}} {
		if err := executeTask(db, "transition", id, "", step.to, step.v); err != nil {
			t.Fatal(err)
		}
	}
	insertNativeCompletionLink(t, db, id, "O1")
	mutating := func(_ context.Context, _ string, link nativeTaskLink) ([]byte, error) {
		body, err := json.Marshal(testNativeTaskProof(t, root, link))
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("source changed\n"), 0600); err != nil {
			return nil, err
		}
		return body, nil
	}
	if _, err := loadNativeCompletionProof(root, id, mutating); err == nil {
		t.Fatal("source race accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("initial\n"), 0600); err != nil {
		t.Fatal(err)
	}
	proof, err := loadNativeCompletionProof(root, id, func(_ context.Context, _ string, link nativeTaskLink) ([]byte, error) {
		return json.Marshal(testNativeTaskProof(t, root, link))
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE arc_task_obligations SET obligation_id='O2' WHERE arc_task_id=?", id); err != nil {
		t.Fatal(err)
	}
	if err := commitTaskCompletion(db, root, id, 4, proof); err == nil || !strings.Contains(err.Error(), "obligations changed") {
		t.Fatalf("relinked scope accepted: %v", err)
	}
	defer db.Close()
}
