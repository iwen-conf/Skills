package main

import (
	"fmt"
	"reflect"
	"strings"
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

func TestBusinessCampaignSnapshotTypesMatchFrontend(t *testing.T) {
	interfaces := tsInterfaces(t, "workbench/src/campaign.ts")
	for name, value := range map[string]any{
		"BusinessCampaign":          BusinessCampaign{},
		"BusinessCampaignRequest":   BusinessCampaignRequest{},
		"BusinessCampaignBudget":    BusinessCampaignBudget{},
		"BusinessCampaignSpent":     BusinessCampaignSpent{},
		"BusinessCampaignVariant":   BusinessCampaignVariant{},
		"BusinessCampaignCandidate": BusinessCampaignCandidate{},
		"BusinessCampaignCoverage":  BusinessCampaignCoverage{},
	} {
		want := uniqueIDs(goJSONFields(reflect.TypeOf(value)))
		got, exists := interfaces[name]
		if !exists || strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s frontend fields %v differ from campaign export %v", name, got, want)
		}
	}
}

func TestBusinessCampaignSnapshotKeepsDamagedHistoryVisibleAndScopesItsBound(t *testing.T) {
	d := modelTestDB(t, referenceModel(t))
	statements := []string{}
	for index := 0; index < 34; index++ {
		environment, round := "independent", "Q-selected"
		if index == 33 {
			environment, round = "other-account-environment", "Q-other"
		}
		statements = append(statements, fmt.Sprintf("INSERT INTO business_campaigns(id,graph,flow,environment,round,status,revision,body_json,sha256,created,updated) VALUES(%s,'course-v2','enrollment',%s,%s,'COMPLETED_IN_SCOPE',1,'{}','damaged','2026-11-17T00:00:00Z','2026-11-17T00:00:00Z')", quote(fmt.Sprintf("BC-corrupted-%02d", index)), quote(environment), quote(round)))
	}
	if err := d.tx(statements); err != nil {
		t.Fatal(err)
	}
	items, available, err := d.snapshotBusinessCampaigns("course-v2", "independent", "Q-selected")
	if err != nil || available != 33 || len(items) != 32 {
		t.Fatalf("bounded campaign history lost its true denominator: count=%d exported=%d %v", available, len(items), err)
	}
	for index, campaign := range items {
		if campaign.ID != fmt.Sprintf("BC-corrupted-%02d", 32-index) || campaign.Graph != "course-v2" || campaign.Environment != "independent" || campaign.Round != "Q-selected" {
			t.Fatalf("campaign order/scope was guessed from opaque IDs or mixed: %+v", campaign)
		}
		if campaign.Status != "BLOCKED" || campaign.EvidenceStatus != "BLOCKED" || len(campaign.Frontier)+len(campaign.Visits) != 0 || campaign.Coverage.SelectedVariantsCovered {
			t.Fatalf("damaged campaign became green or disappeared: %+v", campaign)
		}
	}
	other, available, err := d.snapshotBusinessCampaigns("course-v2", "other-account-environment", "Q-other")
	if err != nil || available != 1 || len(other) != 1 || other[0].ID != "BC-corrupted-33" {
		t.Fatalf("campaign environment isolation failed: %+v, count=%d %v", other, available, err)
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
	receipt, err := fixture.d.modelRun(result.Run)
	if err != nil || run.RunnerID != "typed-business" || run.RunnerID != receipt.Runner || run.RunnerSHA256 != receipt.RunnerSHA256 || run.RunnerSHA256 == "" {
		t.Fatalf("snapshot omitted the verified runner variant: %+v, %v", run, err)
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
	if damaged.EvidenceStatus != "BLOCKED" || len(damaged.EvidenceErrors) == 0 || len(damaged.Facts) != 0 || damaged.BranchID != "" || damaged.RunnerID != "" || damaged.RunnerSHA256 != "" {
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
	if err := fixture.d.tx([]string{"DROP TABLE business_expectation_captures", "DROP TABLE business_branches", "DROP TABLE preparation_actions", "DROP TABLE business_contracts", "DROP TABLE business_campaign_dispatches", "DROP TABLE business_campaign_events", "DROP TABLE business_campaigns"}); err != nil {
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
	for _, table := range []string{"business_contracts", "business_branches", "preparation_actions", "business_expectation_captures", "business_campaigns", "business_campaign_events", "business_campaign_dispatches"} {
		if _, err := fixture.d.query("SELECT COUNT(*) FROM "+table, 1); err != nil {
			t.Fatalf("new schema missing %s: %v", table, err)
		}
	}
}
