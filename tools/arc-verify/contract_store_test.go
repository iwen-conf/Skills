package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBusinessContractStoredAtomicallyAndProjectionTamperingBlocksRead(t *testing.T) {
	g := contractModel(t)
	d := modelTestDB(t, g)
	rows, err := d.query("SELECT id,action,rule_id FROM business_branches WHERE graph="+quote(g.ID)+" ORDER BY id", 3)
	if err != nil || len(rows) != 2 || rows[0][1] != "enroll" {
		t.Fatalf("typed branches were not queryable: %v %v", rows, err)
	}
	if _, err := d.model(g.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.tx([]string{"UPDATE business_branches SET guard_json='{}' WHERE graph=" + quote(g.ID) + " AND id='accept'"}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.model(g.ID); err == nil || !strings.Contains(err.Error(), "projection contradicts") {
		t.Fatalf("tampered branch was accepted: %v", err)
	}
}

func TestBusinessContractImportRollbackPreservesPreviousModel(t *testing.T) {
	g := contractModel(t)
	d := modelTestDB(t, g)
	next := g
	next.ID, next.Previous = "course-v3", g.ID
	// Force a uniqueness violation after graph/core/contract statements have
	// actually executed, rather than failing validation before the transaction.
	if err := d.tx([]string{"INSERT INTO business_branches SELECT 'course-v3',id,flow,action,transition_id,rule_id,capability,confirmed,source_ref,guard_json,expected_json,body FROM business_branches WHERE graph=" + quote(g.ID) + " AND id='accept'"}); err != nil {
		t.Fatal(err)
	}
	if err := d.importGraph(next); err == nil || !strings.Contains(strings.ToLower(err.Error()), "unique") {
		t.Fatalf("expected mid-transaction uniqueness failure: %v", err)
	}
	for _, table := range []string{"graphs", "business_models", "business_contracts"} {
		key := "graph"
		if table == "graphs" {
			key = "id"
		}
		rows, err := d.query("SELECT COUNT(*) FROM "+table+" WHERE "+key+"='course-v3'", 1)
		if err != nil || len(rows) != 1 || rows[0][0] != "0" {
			t.Fatalf("failed import left %s: %v %v", table, rows, err)
		}
	}
	rows, err := d.query("SELECT COUNT(*) FROM business_branches WHERE graph="+quote(g.ID), 1)
	if err != nil || rows[0][0] != "2" {
		t.Fatalf("old branch history lost: %v %v", rows, err)
	}
	if _, err := d.model(g.ID); err != nil {
		t.Fatalf("previous model no longer readable: %v", err)
	}
}

func writeContractJSON(t *testing.T, value any) string {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "input.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBusinessContractCLIMatchesAndComparesFrozenContract(t *testing.T) {
	g := contractModel(t)
	model := writeContractJSON(t, g)
	observed := contractObservation(true)
	var decision BusinessDecision
	if err := json.Unmarshal(cliJSON(t, "contract", "match", "--file", model, "--action", "enroll", "--input", writeContractJSON(t, observed)), &decision); err != nil || decision.Frozen == nil || decision.Branch != "accept" {
		t.Fatalf("CLI did not freeze unique branch: %+v %v", decision, err)
	}
	observed.State = "ready"
	request := map[string]any{"frozen": decision.Frozen, "actual": observed, "denied": false}
	if err := run([]string{"contract", "compare", "--file", model, "--input", writeContractJSON(t, request)}); err != nil {
		t.Fatal(err)
	}
	observed.State = "pending"
	request["actual"], request["denied"] = observed, true
	if err := run([]string{"contract", "compare", "--file", model, "--input", writeContractJSON(t, request)}); err == nil {
		t.Fatal("CLI rematched a denial as PASS after freezing accept")
	}
	delete(request, "denied")
	if err := run([]string{"contract", "compare", "--file", model, "--input", writeContractJSON(t, request)}); err == nil {
		t.Fatal("CLI filled in missing denial observation")
	}
}

func TestBusinessContractCLIFiniteGapAndLegacyExitNonzero(t *testing.T) {
	g := contractModel(t)
	if err := run([]string{"contract", "check", "--file", writeContractJSON(t, g)}); err != nil {
		t.Fatal(err)
	}
	g.BusinessContract.Branches = g.BusinessContract.Branches[:1]
	if err := run([]string{"contract", "check", "--file", writeContractJSON(t, g)}); err == nil {
		t.Fatal("finite gap exited successfully")
	}
	g.BusinessContract = nil
	if err := run([]string{"contract", "check", "--file", writeContractJSON(t, g)}); err == nil {
		t.Fatal("legacy model acquired strong proof")
	}
}
