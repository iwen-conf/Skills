package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestVerifyStatusIsStoredSummaryAndUsesInsertionOrder(t *testing.T) {
	_, db := testArcState(t)
	defer db.Close()
	if err := executeTask(db, "create", "", "Inspect linked evidence", "", 0); err != nil {
		t.Fatal(err)
	}
	var task string
	if err := db.QueryRow("SELECT id FROM arc_tasks LIMIT 1").Scan(&task); err != nil {
		t.Fatal(err)
	}
	insertNativeCompletionLink(t, db, task, "O")
	for _, statement := range []string{
		`CREATE TABLE tasks(id TEXT PRIMARY KEY,status TEXT,kind TEXT,expected TEXT)`,
		`CREATE TABLE runs(id TEXT PRIMARY KEY,task TEXT,status TEXT,environment TEXT,commit_sha TEXT)`,
		`INSERT INTO tasks VALUES('native-self-test/O','PASS','SEQUENCE','ready')`,
		`INSERT INTO runs VALUES('Z-lexicographically-last','native-self-test/O','PASS','isolated','old')`,
		`INSERT INTO runs VALUES('A-actually-latest','native-self-test/O','FAIL','isolated','new')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	output, err := os.CreateTemp(t.TempDir(), "status-*.json")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = output
	err = showTaskVerifyStatus(db, task)
	os.Stdout = original
	if closeErr := output.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result["evidence_status"] != "NOT_REVALIDATED" || result["proof_scope"] != "none" {
		t.Fatalf("summary looked verified: %s", body)
	}
	if _, ok := result["counts"]; ok {
		t.Fatal("unqualified PASS counts leaked from stored task labels")
	}
	counts := result["stored_status_counts"].(map[string]any)
	if counts["PASS"] != float64(1) {
		t.Fatalf("stored status was rewritten: %s", body)
	}
	link := result["obligations"].([]any)[0].(map[string]any)
	if link["latest_run"] != "A-actually-latest" || link["latest_run_stored_status"] != "FAIL" || link["stored_status"] != "PASS" || link["evidence_status"] != "NOT_REVALIDATED" {
		t.Fatalf("wrong latest stored summary: %s", body)
	}
	if _, ok := link["status"]; ok {
		t.Fatal("ambiguous proof status in stored summary")
	}
}
