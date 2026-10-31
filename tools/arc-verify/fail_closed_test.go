package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func isolatedLedger(t *testing.T, g Graph) db {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	if err := run([]string{"init", "--db", path}); err != nil {
		t.Fatal(err)
	}
	d, err := openDB(path, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.close() })
	if err = d.importGraph(g); err != nil {
		t.Fatal(err)
	}
	// Approval here is an isolated test fixture, not approval of a user's RULE.
	if err = d.approve(g.ID, ""); err != nil {
		t.Fatal(err)
	}
	for _, rule := range g.Rules {
		if err = d.approve(g.ID, rule.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = d.plan(g.ID); err != nil {
		t.Fatal(err)
	}
	return d
}

func observedRAW(t testing.TB, d db, runID, expected string, satisfied bool) map[string]any {
	binding := observedBinding(t, d, runID)
	return map[string]any{
		"identity": binding.Identity,
		"run":      runID, "step": 1, "expected": expected, "observed": expected,
		"runner": "isolated-native-test", "oracle": binding.Oracle,
		"exit_code": 0, "preconditions": map[string]any{"login": satisfied},
		"before": "PENDING", "after": "PENDING", "denied": !satisfied,
	}
}

func writeRAW(t *testing.T, raw map[string]any) string {
	t.Helper()
	content, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "raw.json")
	if err = os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIncompleteRAWCannotPass(t *testing.T) {
	for _, name := range []string{"missing_exit", "null_exit", "null_precondition", "blank_oracle", "blank_runner"} {
		t.Run(name, func(t *testing.T) {
			d := isolatedLedger(t, sample())
			task, expected, satisfied := "academic-v1/R1/POSITIVE", "ready", true
			if name == "null_precondition" {
				task, expected, satisfied = "academic-v1/R1/NEGATIVE/login", "UNCHANGED", false
			}
			runID, err := startTestRun(t, d, task, "isolated-test", "0123456789012345678901234567890123456789")
			if err != nil {
				t.Fatal(err)
			}
			raw := observedRAW(t, d, runID, expected, satisfied)
			switch name {
			case "missing_exit":
				delete(raw, "exit_code")
			case "null_exit":
				raw["exit_code"] = nil
			case "null_precondition":
				raw["preconditions"].(map[string]any)["login"] = nil
			case "blank_oracle":
				raw["oracle"] = "  "
			case "blank_runner":
				raw["runner"] = "  "
			}
			step, err := d.step(runID, "", expected, writeRAW(t, raw))
			if err == nil && step["verdict"] == "PASS" {
				t.Fatalf("incomplete runner observation passed: %+v", step)
			}
			if _, err = d.finish(runID); err == nil {
				t.Fatal("incomplete observation completed a PASS run")
			}
		})
	}
}

func TestEveryRejectedOutcomeNeedsDenialAndReadback(t *testing.T) {
	g := sample()
	g.Nodes = append(g.Nodes, Node{ID: "rejected", Kind: "state", Name: "Rejected"})
	g.Rules[0].Rejected = "rejected"
	for _, kind := range []string{"NEGATIVE", "FORCED"} {
		t.Run(kind, func(t *testing.T) {
			d := isolatedLedger(t, g)
			runID, err := startTestRun(t, d, "academic-v1/R1/"+kind+"/login", "isolated-test", "0123456789012345678901234567890123456789")
			if err != nil {
				t.Fatal(err)
			}
			raw := observedRAW(t, d, runID, "rejected", false)
			delete(raw, "denied")
			delete(raw, "before")
			delete(raw, "after")
			step, err := d.step(runID, "", "rejected", writeRAW(t, raw))
			if err == nil && step["verdict"] == "PASS" {
				t.Fatal("named rejection passed without independent denial/readback")
			}
		})
	}
}

func TestRejectedOperationWithChangedStateKeepsFailure(t *testing.T) {
	d := isolatedLedger(t, sample())
	runID, err := startTestRun(t, d, "academic-v1/R1/FORCED/login", "isolated-test", "0123456789012345678901234567890123456789")
	if err != nil {
		t.Fatal(err)
	}
	raw := observedRAW(t, d, runID, "UNCHANGED", false)
	raw["after"] = "ENROLLED"
	step, err := d.step(runID, "", "UNCHANGED", writeRAW(t, raw))
	if err != nil || step["verdict"] != "FAIL" {
		t.Fatalf("observed forbidden write must remain FAIL, got %+v %v", step, err)
	}
	trace, err := d.trace(runID)
	if err != nil || trace.Status != "FAIL" || trace.FirstDivergence != "1" {
		t.Fatalf("original failure lost: %+v %v", trace, err)
	}
}

func TestRuleCannotOmitGraphPrerequisite(t *testing.T) {
	g := sample()
	g.Nodes = append(g.Nodes, Node{ID: "seats", Kind: "condition", Name: "Available seats"})
	g.Edges = append(g.Edges, Edge{ID: "requires_seats", From: "seats", To: "enroll", Kind: "requires"})
	if err := validateGraph(g); err == nil {
		t.Fatal("rule omitted an AND prerequisite from the frozen graph")
	}
}

func TestAmbiguousJSONCannotDefineEvidenceOrApproval(t *testing.T) {
	raw := []byte(`{"run":"R1","step":1,"expected":"ready","observed":"ready","runner":"native","oracle":"readback","exit_code":1,"exit_code":0}`)
	if _, err := parseStepRAW(raw, "R1", "ready", "ready", 1); err == nil {
		t.Error("duplicate RAW exit status hid a runner failure")
	}
	body, err := json.Marshal(sample())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "graph.json")
	body = []byte(strings.Replace(string(body), `"id":"academic-v1"`, `"id":"conflicting","id":"academic-v1"`, 1))
	if err = os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err = run([]string{"validate", "--file", path}); err == nil {
		t.Error("duplicate graph identity was silently overwritten")
	}
	config := filepath.Join(t.TempDir(), "config.jsonc")
	if err = os.WriteFile(config, []byte(`{"environments":{"isolated":{"approved":false,"approved":true,"url":"http://127.0.0.1"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err = approvedEnvironment(config, "isolated"); err == nil {
		t.Error("duplicate environment approval hid an explicit denial")
	}
}

func TestCaseAliasesCannotOverrideEvidenceOrApproval(t *testing.T) {
	for _, alias := range []string{`,"Exit_Code":0`, `,"Denied":true`, `,"obſerved":"ready"`} {
		t.Run(alias, func(t *testing.T) {
			d := isolatedLedger(t, sample())
			runID, err := startTestRun(t, d, "academic-v1/R1/POSITIVE", "isolated-test", "0123456789012345678901234567890123456789")
			if err != nil {
				t.Fatal(err)
			}
			raw := observedRAW(t, d, runID, "ready", true)
			raw["exit_code"], raw["denied"] = 1, false
			body, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "raw.json")
			if err = os.WriteFile(path, []byte(strings.TrimSuffix(string(body), "}")+alias+"}"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err = d.step(runID, "", "ready", path); err == nil {
				t.Fatal("case aliases must not overwrite independent observations")
			}
		})
	}
	config := filepath.Join(t.TempDir(), "config.jsonc")
	if err := os.WriteFile(config, []byte(`{"environments":{"isolated":{"approved":false,"Approved":true,"url":"http://127.0.0.1"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := approvedEnvironment(config, "isolated"); err == nil {
		t.Fatal("case alias overrode an explicit environment denial")
	}
}

func TestRunnerFailureCannotBecomeProductDefect(t *testing.T) {
	d := isolatedLedger(t, sample())
	runID, err := startTestRun(t, d, "academic-v1/R1/POSITIVE", "isolated-test", "0123456789012345678901234567890123456789")
	if err != nil {
		t.Fatal(err)
	}
	raw := observedRAW(t, d, runID, "ready", true)
	raw["exit_code"] = 7
	step, err := d.step(runID, "", "ready", writeRAW(t, raw))
	if err != nil {
		t.Fatal(err)
	}
	if step["verdict"] != "BLOCKED" || step["decision"] != "HALT" {
		t.Errorf("runner failure is missing verification, got %+v", step)
	}
	// A legacy FAIL must also be re-evaluated before a repair handoff.
	if err = d.tx([]string{"UPDATE runs SET status='FAIL' WHERE id=" + quote(runID), "UPDATE steps SET verdict='FAIL' WHERE run=" + quote(runID)}); err != nil {
		t.Fatal(err)
	}
	if _, err = d.handoff("arc-code", "", runID); err == nil {
		t.Fatal("runner failure without a business deviation became a confirmed product defect")
	}
}

func TestTaskIdentitiesCannotCollapseRuleObligations(t *testing.T) {
	g := sample()
	g.Nodes[1].ID, g.Edges[0].From = "login/POSITIVE", "login/POSITIVE"
	g.Rules[0].Conditions = []string{"login/POSITIVE"}
	g.Nodes = append(g.Nodes, Node{ID: "cancel", Kind: "action", Name: "Cancel"}, Node{ID: "cancelled", Kind: "state", Name: "Cancelled"})
	g.Edges = append(g.Edges, Edge{ID: "cancel_requires", From: "login/POSITIVE", To: "cancel", Kind: "requires"}, Edge{ID: "cancel_transition", From: "cancel", To: "cancelled", Kind: "transitions"})
	second := g.Rules[0]
	second.ID, second.Action, second.Expected = "R1/NEGATIVE/login", "cancel", "cancelled"
	second.Entry, second.Oracle, second.Description = "/cancel", "independent cancellation readback", "Authenticated students can cancel"
	g.Rules = append(g.Rules, second)
	if err := validateGraph(g); err == nil {
		t.Fatal("two distinct obligations generated the same task identity")
	}

	d := isolatedLedger(t, sample())
	if err := d.tx([]string{"UPDATE tasks SET expected='UNCHANGED' WHERE id='academic-v1/R1/POSITIVE'"}); err != nil {
		t.Fatal(err)
	}
	if err := d.approve("academic-v1", "R1"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.plan("academic-v1"); err == nil {
		t.Fatal("idempotent planning silently skipped a conflicting stored task")
	}
}

func completeIsolatedGraph(t *testing.T, d db) {
	t.Helper()
	for _, item := range []struct {
		id, expected string
		satisfied    bool
	}{
		{"academic-v1/R1/POSITIVE", "ready", true},
		{"academic-v1/R1/NEGATIVE/login", "UNCHANGED", false},
		{"academic-v1/R1/FORCED/login", "UNCHANGED", false},
	} {
		runID, err := startTestRun(t, d, item.id, "isolated-test", "0123456789012345678901234567890123456789")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = d.record(runID, item.expected, writeRAW(t, observedRAW(t, d, runID, item.expected, item.satisfied))); err != nil {
			t.Fatal(err)
		}
	}
	result, err := d.gate("academic-v1")
	if err != nil || result["verdict"] != "PASS" {
		t.Fatalf("complete native fixture must pass: %+v %v", result, err)
	}
}

func TestGateReconstructsFrozenTaskMeaning(t *testing.T) {
	for _, damage := range []string{"expected", "kind"} {
		t.Run(damage, func(t *testing.T) {
			g := sample()
			g.Nodes = append(g.Nodes, Node{ID: "unrelated", Kind: "state", Name: "Unrelated"})
			d := isolatedLedger(t, g)
			completeIsolatedGraph(t, d)
			rows, err := d.query("SELECT s.run,s.evidence_path FROM steps s JOIN runs r ON r.id=s.run WHERE r.task='academic-v1/R1/POSITIVE'", 2)
			if err != nil || len(rows) != 1 {
				t.Fatalf("fixture run: %v %+v", err, rows)
			}
			raw := observedRAW(t, d, rows[0][0], "ready", true)
			statement := "UPDATE tasks SET kind='NEGATIVE',condition='login' WHERE id='academic-v1/R1/POSITIVE'"
			if damage == "expected" {
				raw["expected"], raw["observed"] = "unrelated", "unrelated"
				statement = "UPDATE tasks SET expected='unrelated' WHERE id='academic-v1/R1/POSITIVE'"
			} else {
				raw["preconditions"] = map[string]bool{"login": false}
			}
			body, _ := json.Marshal(raw)
			if err = os.WriteFile(rows[0][1], body, 0600); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(body)
			if err = d.tx([]string{statement, "UPDATE steps SET expected=" + quote(raw["expected"].(string)) + ",observed=" + quote(raw["observed"].(string)) + ",evidence_sha256=" + quote(hex.EncodeToString(sum[:])) + " WHERE run=" + quote(rows[0][0])}); err != nil {
				t.Fatal(err)
			}
			result, err := d.gate(g.ID)
			if err == nil && result["verdict"] == "PASS" {
				t.Fatalf("derived task silently changed its frozen meaning: %+v", result)
			}
		})
	}
}

func TestFinishRevalidatesRecordedRAW(t *testing.T) {
	d := isolatedLedger(t, sample())
	runID, err := startTestRun(t, d, "academic-v1/R1/POSITIVE", "isolated-test", "0123456789012345678901234567890123456789")
	if err != nil {
		t.Fatal(err)
	}
	raw := observedRAW(t, d, runID, "ready", true)
	path := writeRAW(t, raw)
	if _, err = d.step(runID, "", "ready", path); err != nil {
		t.Fatal(err)
	}
	delete(raw, "exit_code")
	body, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	if err = d.tx([]string{"UPDATE steps SET evidence_sha256=" + quote(hex.EncodeToString(sum[:])) + " WHERE run=" + quote(runID)}); err != nil {
		t.Fatal(err)
	}
	if _, err = d.finish(runID); err == nil {
		t.Fatal("matching hash alone blessed a record without runner execution status")
	}
}

func TestRepairHandoffRequiresFailedRun(t *testing.T) {
	d := isolatedLedger(t, sample())
	if _, err := d.handoff("arc-code", "academic-v1", ""); err == nil {
		t.Fatal("repair handoff accepted a graph without a failed run")
	}
}

func TestRepairHandoffRequiresUnchangedFailedRAW(t *testing.T) {
	d := isolatedLedger(t, sample())
	runID, err := startTestRun(t, d, "academic-v1/R1/POSITIVE", "isolated-test", "0123456789012345678901234567890123456789")
	if err != nil {
		t.Fatal(err)
	}
	raw := observedRAW(t, d, runID, "ready", true)
	raw["observed"] = "BROKEN"
	path := writeRAW(t, raw)
	step, err := d.step(runID, "", "BROKEN", path)
	if err != nil || step["verdict"] != "FAIL" {
		t.Fatalf("fixture must have an observed failure: %+v %v", step, err)
	}
	if _, err = d.handoff("arc-code", "", runID); err != nil {
		t.Fatalf("unchanged observed failure must be transferable: %v", err)
	}
	if err = os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = d.handoff("arc-code", "", runID); err == nil {
		t.Fatal("repair handoff trusted a stored FAIL after RAW tampering")
	}
}
