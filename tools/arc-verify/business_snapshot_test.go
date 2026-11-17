package main

import (
	"reflect"
	"testing"
)

func TestSnapshotRunSelectionPreservesHistoryAndExplicitReplayOrder(t *testing.T) {
	input := []string{"R-z-first", "R-a-second", "R-z-first", "R-b-third"}
	want := []string{"R-z-first", "R-a-second", "R-b-third"}
	if got := uniqueRunIDsInOrder(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("run IDs are opaque, not chronology: got %v; want %v", got, want)
	}
	if input[2] != "R-z-first" {
		t.Fatal("snapshot selection mutated the caller's replay request")
	}
}

func TestBusinessSnapshotIncludesValidatedBranchFactsWithoutManualRunSelection(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	fixture := setupRuntimeBusiness(t, runtimeBusinessGraph(t), target, server, "accept", nil)
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "PASS" {
		t.Fatal(result)
	}
	snapshot, err := fixture.d.buildSnapshot(SnapshotRequest{Graph: "course-v2", Environment: "independent", Round: fixture.round, Config: fixture.config})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Runs) != 1 || snapshot.AvailableRuns != 1 || snapshot.RunsTruncated {
		t.Fatalf("default history selection lost the actual run: %+v", snapshot.Runs)
	}
	run := snapshot.Runs[0]
	if run.Trace.Run != result.Run || run.Environment != "independent" || run.EvidenceStatus != "PASS" || run.BranchID != "accept" {
		t.Fatalf("branch/run/environment is not based on validated facts: %+v", run)
	}
	found := false
	for _, fact := range run.Facts {
		if fact.Seq == 1 && len(fact.BusinessExpectations) == 1 {
			decision := fact.BusinessExpectations[0]
			found = decision.Decision.Frozen != nil && decision.Decision.Frozen.Branch == "accept" && decision.After != nil && decision.Comparison.Status == "PASS"
		}
	}
	if !found {
		t.Fatal("workbench export lacks frozen expected/actual/assertion facts")
	}
	if snapshot.ContractExecution == nil || snapshot.ContractExecution.Status == "PASS" {
		t.Fatal("one passing branch incorrectly covered its unexecuted sibling")
	}
	if err := fixture.d.tx([]string{"UPDATE step_assertions SET actual_json='corrupt' WHERE run=" + quote(result.Run) + " AND seq=1"}); err != nil {
		t.Fatal(err)
	}
	damaged, err := fixture.d.snapshotRun("course-v2", result.Run)
	if err != nil {
		t.Fatal(err)
	}
	if damaged.EvidenceStatus != "BLOCKED" || len(damaged.EvidenceErrors) == 0 || len(damaged.Facts) != 0 || damaged.BranchID != "" {
		t.Fatalf("damaged facts became a green branch in the workbench: %+v", damaged)
	}
}

func TestBusinessAdditiveSchemaKeepsExistingExecutionHistory(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	run := latestRun(t, fixture)
	before, err := fixture.d.query("SELECT run,seq,evidence_sha256,raw_json FROM step_facts WHERE run="+quote(run)+" ORDER BY seq", 4)
	if err != nil || len(before) == 0 {
		t.Fatal(err)
	}
	// Simulate the preceding ledger schema; these tables contain no typed data
	// for this legacy model. The migration must only add the missing structures.
	if err := fixture.d.tx([]string{"DROP TABLE business_expectation_captures", "DROP TABLE business_branches", "DROP TABLE preparation_actions", "DROP TABLE business_contracts"}); err != nil {
		t.Fatal(err)
	}
	cliJSON(t, "init", "--db", fixture.d.file)
	after, err := fixture.d.query("SELECT run,seq,evidence_sha256,raw_json FROM step_facts WHERE run="+quote(run)+" ORDER BY seq", 4)
	if err != nil || len(before) != len(after) {
		t.Fatal("additive migration changed history cardinality", err)
	}
	for i := range before {
		for j := range before[i] {
			if before[i][j] != after[i][j] {
				t.Fatal("additive migration rewrote historical facts or RAW")
			}
		}
	}
	for _, table := range []string{"business_contracts", "business_branches", "preparation_actions", "business_expectation_captures"} {
		if _, err := fixture.d.query("SELECT COUNT(*) FROM "+table, 1); err != nil {
			t.Fatalf("new schema missing %s: %v", table, err)
		}
	}
}
