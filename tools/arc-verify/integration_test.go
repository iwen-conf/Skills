package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestTursoLocalBusinessChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	d, err := openDB(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.close()
	for _, sql := range schema {
		if _, err = d.sql(sql); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	g := sample()
	if err = validateGraph(g); err != nil {
		t.Fatal(err)
	}
	if err = d.importGraph(g); err != nil {
		t.Fatal(err)
	}
	if err = d.approve(g.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err = d.approve(g.ID, "R1"); err != nil {
		t.Fatal(err)
	}
	if _, err = d.plan(g.ID); err != nil {
		t.Fatal(err)
	}
	cards, err := d.next(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 3 {
		t.Fatalf("one condition must generate 3 obligations: %+v", cards)
	}
	for _, card := range cards {
		if card.Actor != "STUDENT" || card.Entry != "/courses" || card.Oracle == "" {
			t.Fatalf("incomplete task card %+v", card)
		}
		run, err := startTestRun(t, d, card.ID, "isolated-test", "0123456789012345678901234567890123456789")
		if err != nil {
			t.Fatal(err)
		}
		raw := StepRAW{
			Identity: observedBinding(t, d, run).Identity,
			Run:      run, Step: 1, Expected: card.Expected, Observed: card.Expected,
			Oracle: card.Oracle, Runner: "test-e2e-adapter", ExitCode: 0,
			Preconditions: map[string]bool{"login": card.Type == "POSITIVE"},
		}
		if card.Expected == "UNCHANGED" {
			raw.Before = "PENDING"
			raw.After = "PENDING"
			raw.Denied = true
		}
		content, _ := json.Marshal(raw)
		proof := filepath.Join(t.TempDir(), run+".json")
		if err = os.WriteFile(proof, content, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err = d.step(run, "", card.Expected, proof); err != nil {
			t.Fatal(err)
		}
		if _, err = d.finish(run); err != nil {
			t.Fatal(err)
		}
	}
	// Separate DB object opens the same local file; no daemon or cloud endpoint.
	other, err := openDB(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer other.close()
	result, err := other.gate(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result["verdict"] != "PASS" {
		t.Fatalf("expected full verified task set, got %+v", result)
	}
	// Orphan/extra tasks must not disappear from a gate via an inner join.
	if err = other.tx([]string{"INSERT INTO tasks(id,graph,rule,kind,condition,expected,status) VALUES('orphan', 'academic-v1','not-a-rule','POSITIVE','','ready','PASS');"}); err != nil {
		t.Fatal(err)
	}
	result, err = other.gate(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result["verdict"] != "FAIL" {
		t.Fatalf("orphan task must fail closed, got %+v", result)
	}
	if err = other.tx([]string{"DELETE FROM tasks WHERE id='orphan';"}); err != nil {
		t.Fatal(err)
	}
	// A recorded PASS cannot survive RAW tampering.
	if err = other.tx([]string{fmt.Sprintf("UPDATE steps SET evidence_sha256='tampered' WHERE run=(SELECT id FROM runs LIMIT 1);")}); err != nil {
		t.Fatal(err)
	}
	result, err = other.gate(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result["verdict"] != "FAIL" {
		t.Fatalf("changed evidence hash must fail closed, got %+v", result)
	}
}

func TestTursoLazyPlannerHonorsBoundedFrontier(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazy.db")
	d, err := openDB(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.close()
	for _, q := range schema {
		if _, err = d.sql(q); err != nil {
			t.Fatal(err)
		}
	}
	g := sample()
	for i := 1; i < 50; i++ {
		rule := g.Rules[0]
		rule.ID = fmt.Sprintf("RULE-%03d", i)
		g.Rules = append(g.Rules, rule)
	}
	if err = validateGraph(g); err != nil {
		t.Fatal(err)
	}
	if err = d.importGraph(g); err != nil {
		t.Fatal(err)
	}
	if err = d.approve(g.ID, ""); err != nil {
		t.Fatal(err)
	}
	var approvals []string
	for _, r := range g.Rules {
		approvals = append(approvals, "UPDATE rules SET approved=1 WHERE graph="+quote(g.ID)+" AND id="+quote(r.ID)+";")
	}
	if err = d.tx(approvals); err != nil {
		t.Fatal(err)
	}
	first, err := d.plan(g.ID, 32)
	if err != nil || first != 32 {
		t.Fatalf("first batch %d %v", first, err)
	}
	verdict, err := d.gate(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if verdict["verdict"] != "FAIL" || verdict["missing_tasks"] == 0 {
		t.Fatalf("partial frontier must fail gate %+v", verdict)
	}
	total := first
	for i := 0; i < 10; i++ {
		n, err := d.plan(g.ID, 32)
		if err != nil {
			t.Fatal(err)
		}
		total += n
		if n == 0 {
			break
		}
	}
	if total != 150 {
		t.Fatalf("expected 150 obligations, materialized %d", total)
	}
	if n, err := d.plan(g.ID, 32); err != nil || n != 0 {
		t.Fatalf("duplicate batch %d %v", n, err)
	}
}

func TestRecoveryReadsOnlyRelevantApprovedGraphSlice(t *testing.T) {
	d, err := openDB(filepath.Join(t.TempDir(), "recover.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.close()
	for _, q := range schema {
		if _, err = d.sql(q); err != nil {
			t.Fatal(err)
		}
	}
	g := sample()
	g.Nodes = append(g.Nodes, Node{ID: "login_action", Kind: "action", Name: "Login"})
	g.Edges = append(g.Edges, Edge{ID: "recovery", From: "login_action", To: "login", Kind: "recovers"})
	if err = validateGraph(g); err != nil {
		t.Fatal(err)
	}
	if err = d.importGraph(g); err != nil {
		t.Fatal(err)
	}
	result, err := d.recover(g.ID, "login", 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Actions) != 1 || result.Actions[0].Approved || len(result.Unresolved) != 1 {
		t.Fatalf("unconfirmed recovery must be unresolved: %+v", result)
	}
	if result.Truncated {
		t.Fatal("single recovery unexpectedly truncated")
	}
}

func TestPlannerCursorRewindsAfterLateUserApproval(t *testing.T) {
	d, err := openDB(filepath.Join(t.TempDir(), "approval.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer d.close()
	for _, q := range schema {
		if _, err = d.sql(q); err != nil {
			t.Fatal(err)
		}
	}
	g := sample()
	earlier := g.Rules[0]
	earlier.ID = "R0"
	later := g.Rules[0]
	later.ID = "R2"
	g.Rules = append(g.Rules, earlier, later)
	if err = validateGraph(g); err != nil {
		t.Fatal(err)
	}
	if err = d.importGraph(g); err != nil {
		t.Fatal(err)
	}
	if err = d.approve(g.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err = d.approve(g.ID, later.ID); err != nil {
		t.Fatal(err)
	}
	first, err := d.plan(g.ID, 16)
	if err != nil || first != 3 {
		t.Fatalf("first %d %v", first, err)
	}
	if err = d.approve(g.ID, earlier.ID); err != nil {
		t.Fatal(err)
	}
	second, err := d.plan(g.ID, 16)
	if err != nil || second != 6 {
		t.Fatalf("rewind %d %v", second, err)
	}
	rows, err := d.query("SELECT "+fieldsSQL("CAST(COUNT(*) AS TEXT)")+" FROM tasks WHERE graph="+quote(g.ID)+";", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][0] != "6" {
		t.Fatalf("task count after user approval %+v", rows)
	}
	if n, err := d.plan(g.ID, 16); err != nil || n != 0 {
		t.Fatalf("cursor termination %d %v", n, err)
	}
}

// TestInitFreshAndRepeated exercises the public CLI entry, not just schema SQL.
// The scan index used to be created before its table; a new database failed.
func TestInitFreshAndRepeated(t *testing.T) {
	database := filepath.Join(t.TempDir(), "verify.db")
	for attempt := 0; attempt < 2; attempt++ {
		if err := run([]string{"init", "--db", database}); err != nil {
			t.Fatalf("init attempt %d: %v", attempt+1, err)
		}
	}
	d, err := openDB(database, false)
	if err != nil {
		t.Fatal(err)
	}
	defer d.close()
	rows, err := d.query("SELECT "+fieldsSQL("name")+" FROM sqlite_master WHERE type='index' AND name='idx_scans_action_history'", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("implementation scan index not created: %+v", rows)
	}
	rows, err = d.query("SELECT "+fieldsSQL("name")+" FROM sqlite_master WHERE type='table' AND name='implementation_scans'", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatal("implementation scan table not created")
	}
}

// Existing local ledgers are upgraded in place without dropping prior tasks.
func TestInitUpgradesExistingLedger(t *testing.T) {
	database := filepath.Join(t.TempDir(), "existing.db")
	d, err := openDB(database, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.sql("CREATE TABLE IF NOT EXISTS graphs (id TEXT PRIMARY KEY, scope TEXT NOT NULL, approved INTEGER NOT NULL DEFAULT 0)"); err != nil {
		t.Fatal(err)
	}
	if _, err = d.sql("INSERT INTO graphs(id,scope,approved) VALUES('legacy-v1','test',1)"); err != nil {
		t.Fatal(err)
	}
	if err = d.close(); err != nil {
		t.Fatal(err)
	}
	if err = run([]string{"init", "--db", database}); err != nil {
		t.Fatal(err)
	}
	upgraded, err := openDB(database, false)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.close()
	rows, err := upgraded.query("SELECT "+fieldsSQL("scope")+" FROM graphs WHERE id='legacy-v1'", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][0] != "test" {
		t.Fatalf("upgrade dropped old graph: %+v", rows)
	}
	rows, err = upgraded.query("SELECT "+fieldsSQL("name")+" FROM sqlite_master WHERE type='table' AND name='plan_cursor'", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatal("plan cursor table was not created")
	}
}

func TestNativeDriverSelftest(t *testing.T) {
	if err := selftestLocalTurso(); err != nil {
		t.Fatal(err)
	}
}
