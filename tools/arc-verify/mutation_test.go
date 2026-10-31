package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mutationFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mutations.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// mutationLedger has one failing and one passing R1/POSITIVE run, standing in
// for runs against two injected builds.
func mutationLedger(t *testing.T) (executionFixture, string, string) {
	t.Helper()
	target, server := newKindsServer(t)
	target.enrollBroken = true
	fixture := prepareExecutionFixture(t, kindsModel(t), server.URL)
	fixture.registerAndRound(t, positiveManifest(t, "positive-runner"))
	failed := executeKinds(t, fixture, "positive-runner", "R1/POSITIVE")
	if _, err := fixture.d.retry("course-v2/R1/POSITIVE", "fixture independently restored", failed.Recovery.Evidence, fixture.root, fixture.config, fixture.commit); err != nil {
		t.Fatal(err)
	}
	target.mu.Lock()
	target.enrollBroken = false
	target.mu.Unlock()
	passed := executeKinds(t, fixture, "positive-runner", "R1/POSITIVE")
	if failed.Verdict != "FAIL" || passed.Verdict != "PASS" {
		t.Fatalf("setup needs one failed and one passed run: %s %s", failed.Verdict, passed.Verdict)
	}
	return fixture, failed.Run, passed.Run
}

func mutationReport(t *testing.T, d db, body string) MutationReport {
	t.Helper()
	var report MutationReport
	if err := json.Unmarshal(cliJSON(t, "mutation-report", "--db", d.file, "--file", mutationFile(t, body)), &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func TestMutationReportDetectsExpectedFailure(t *testing.T) {
	fixture, failed, _ := mutationLedger(t)
	report := mutationReport(t, fixture.d, `{"graph": "course-v2", "mutations": [
	  {"id": "M-STATE", "category": "illegal_transition", "location": "enroll handler keeps pending", "expected_failing": ["R1/POSITIVE"], "runs": ["`+failed+`"]}]}`)
	if len(report.Mutations) != 1 || report.Mutations[0].Outcome != "detected" || strings.Join(report.Mutations[0].FailingRuns, ",") != failed {
		t.Fatalf("an expected obligation failing on the mutant must count as detected: %+v", report)
	}
	if report.Detected != 1 || report.Survived != 0 {
		t.Fatalf("totals: %+v", report)
	}
}

func TestMutationReportListsSurvivors(t *testing.T) {
	fixture, failed, passed := mutationLedger(t)
	report := mutationReport(t, fixture.d, `{"graph": "course-v2", "mutations": [
	  {"id": "M-RACE", "category": "race_oversell", "location": "seat check outside the transaction", "expected_failing": ["R1/POSITIVE"], "runs": ["`+passed+`"]},
	  {"id": "M-NORUN", "category": "idempotency", "location": "duplicate submit", "expected_failing": ["test.consistency.enroll"], "runs": []},
	  {"id": "M-AUTH", "category": "unauthorized_access", "location": "missing owner check", "expected_failing": ["R1/POSITIVE"], "runs": ["`+failed+`"]},
	  {"id": "M-OTHER", "category": "missing_server_validation", "location": "layer check", "expected_failing": ["test.consistency.enroll"], "runs": ["`+failed+`"]}]}`)
	got := map[string]MutationOutcome{}
	for _, item := range report.Mutations {
		got[item.ID] = item
	}
	if got["M-RACE"].Outcome != "survived" || !strings.Contains(got["M-RACE"].Reason, "passed") {
		t.Fatalf("a mutant every expected obligation passed on is a survivor: %+v", got["M-RACE"])
	}
	if got["M-NORUN"].Outcome != "inconclusive" {
		t.Fatalf("a mutant with no run is not a survivor: %+v", got["M-NORUN"])
	}
	if got["M-OTHER"].Outcome != "inconclusive" {
		t.Fatalf("a failure of an obligation the mutant was not expected to break is not a detection: %+v", got["M-OTHER"])
	}
	if strings.Join(report.Survivors, ",") != "M-RACE" || report.Detected != 1 || report.Inconclusive != 2 {
		t.Fatalf("survivors must be listed for new obligations: %+v", report)
	}
	if !report.InsufficientSample || report.ByCategory["race_oversell"].Survived != 1 {
		t.Fatalf("small samples must be flagged and categories kept apart: %+v", report)
	}
}

func TestMutationReportRejectsUnknownObligation(t *testing.T) {
	fixture, failed, _ := mutationLedger(t)
	for name, body := range map[string]string{
		"unknown obligation": `{"graph": "course-v2", "mutations": [{"id": "M1", "category": "illegal_transition", "location": "x", "expected_failing": ["R9/POSITIVE"], "runs": ["` + failed + `"]}]}`,
		"unknown category":   `{"graph": "course-v2", "mutations": [{"id": "M1", "category": "typo", "location": "x", "expected_failing": ["R1/POSITIVE"], "runs": ["` + failed + `"]}]}`,
		"foreign run":        `{"graph": "course-v2", "mutations": [{"id": "M1", "category": "illegal_transition", "location": "x", "expected_failing": ["R1/POSITIVE"], "runs": ["R-missing"]}]}`,
		"no expectation":     `{"graph": "course-v2", "mutations": [{"id": "M1", "category": "illegal_transition", "location": "x", "expected_failing": [], "runs": ["` + failed + `"]}]}`,
	} {
		if err := run([]string{"mutation-report", "--db", fixture.d.file, "--file", mutationFile(t, body)}); err == nil {
			t.Fatalf("%s: an invalid injection list was reported", name)
		}
	}
}
