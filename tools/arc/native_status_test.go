package main

import (
	"encoding/json"
	"os"
	"testing"
)

// TestVerifyStatusIsStoredSummaryAndUsesInsertionOrder 验证任务验证摘要按插入顺序取最新运行记录且不泄露未重验状态
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
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawFields); err != nil {
		t.Fatal(err)
	}
	if _, ok := rawFields["counts"]; ok {
		t.Fatal("unqualified PASS counts leaked from stored task labels")
	}
	var result taskVerifyStatusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	if result.EvidenceStatus != "NOT_REVALIDATED" || result.ProofScope != "none" {
		t.Fatalf("summary looked verified: %s", body)
	}
	if result.StoredStatusCounts["PASS"] != 1 {
		t.Fatalf("stored status was rewritten: %s", body)
	}
	if len(result.Obligations) != 1 {
		t.Fatalf("expected 1 obligation, got %d", len(result.Obligations))
	}
	link := result.Obligations[0]
	if link.LatestRun != "A-actually-latest" || link.LatestRunStoredStatus != "FAIL" || link.StoredStatus != "PASS" || link.EvidenceStatus != "NOT_REVALIDATED" {
		t.Fatalf("wrong latest stored summary: %s", body)
	}
	var rawObligations []map[string]json.RawMessage
	if err := json.Unmarshal(rawFields["obligations"], &rawObligations); err != nil {
		t.Fatal(err)
	}
	if _, ok := rawObligations[0]["status"]; ok {
		t.Fatal("ambiguous proof status in stored summary")
	}
}
