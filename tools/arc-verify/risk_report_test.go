package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func riskTestDB(t *testing.T) (db, Graph) {
	t.Helper()
	graph, err := loadGraph("examples/academic-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return modelTestDB(t, graph), graph
}

func riskByID(t *testing.T, report RiskReport, id string) RiskEntry {
	t.Helper()
	for _, entry := range report.Risks {
		if entry.ID == id {
			return entry
		}
	}
	t.Fatalf("risk %s missing from report", id)
	return RiskEntry{}
}

func TestRiskOriginValidated(t *testing.T) {
	cases := map[string]Risk{
		"unknown origin":                {ID: "K-X", Description: "d", Action: "enroll", Origin: "guess"},
		"peer reference without source": {ID: "K-X", Description: "d", Action: "enroll", Origin: "peer_reference"},
		"reference without peer origin": {ID: "K-X", Description: "d", Action: "enroll", Origin: "ai_inference", Reference: "catalog/enrollment"},
	}
	for name, risk := range cases {
		graph := sample()
		graph.Risks = []Risk{risk}
		if err := validateGraph(graph); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	graph := sample()
	graph.Risks = []Risk{{ID: "K-OK", Description: "d", Action: "enroll", Origin: "peer_reference", Reference: "reference-catalog/enrollment-capacity"}}
	if err := validateGraph(graph); err != nil {
		t.Fatalf("declared peer reference rejected: %v", err)
	}
}

func TestLegacyRiskOriginMarkedUndeclared(t *testing.T) {
	d, graph := riskTestDB(t)
	report, err := d.riskReport(graph.ID)
	if err != nil {
		t.Fatal(err)
	}
	if entry := riskByID(t, report, "K-SCHEDULE"); entry.Origin != "undeclared" || entry.Status != "OPEN" {
		t.Fatalf("risk without origin must read undeclared/OPEN, got %+v", entry)
	}
}

func TestRiskOriginStoredAndReported(t *testing.T) {
	graph, err := loadGraph("examples/academic-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	graph.Risks[0].Origin, graph.Risks[0].Reference = "peer_reference", "reference-catalog/course-schedule"
	d := modelTestDB(t, graph)
	report, err := d.riskReport(graph.ID)
	if err != nil {
		t.Fatal(err)
	}
	if entry := riskByID(t, report, "K-SCHEDULE"); entry.Origin != "peer_reference" || entry.Reference != "reference-catalog/course-schedule" {
		t.Fatalf("declared origin lost: %+v", entry)
	}
	stored, err := d.model(graph.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Risks[0].Origin != "peer_reference" || stored.Risks[0].Reference != "reference-catalog/course-schedule" {
		t.Fatalf("ledger readback dropped risk origin: %+v", stored.Risks[0])
	}
}

func TestAcceptRiskRequiresScopeAndQuote(t *testing.T) {
	d, graph := riskTestDB(t)
	for _, c := range []struct{ risk, scope, quote string }{
		{"K-SCHEDULE", "", "选修课时间重叠我们允许"},
		{"K-SCHEDULE", "仅限选修课", " "},
		{"K-MISSING", "仅限选修课", "选修课时间重叠我们允许"},
	} {
		if err := d.acceptRisk(graph.ID, c.risk, c.scope, c.quote); err == nil {
			t.Errorf("accepted %+v", c)
		}
	}
	if err := d.acceptRisk(graph.ID, "K-SCHEDULE", "仅限选修课", "选修课时间重叠我们允许"); err != nil {
		t.Fatal(err)
	}
	if err := d.acceptRisk(graph.ID, "K-SCHEDULE", "全部课程", "都允许"); err == nil {
		t.Fatal("a decided risk was accepted a second time")
	}
}

func TestAcceptRiskRequiresInteractiveTerminal(t *testing.T) {
	path, err := filepath.Abs("examples/academic-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	dbfile := filepath.Join(t.TempDir(), "data.db")
	for _, args := range [][]string{{"init", "--db", dbfile}, {"import", "--db", dbfile, "--file", path}} {
		if err := run(args); err != nil {
			t.Fatal(err)
		}
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writer.WriteString("ACCEPT K-SCHEDULE\n选修课时间重叠我们允许\n"); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	old := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = old; reader.Close() }()
	err = run([]string{"accept-risk", "--db", dbfile, "--graph", "academic-v1", "--risk", "K-SCHEDULE", "--scope", "仅限选修课"})
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("piped acceptance must be refused, got %v", err)
	}
}

func TestPromoteRiskNeedsApprovedRule(t *testing.T) {
	graph, err := loadGraph("examples/academic-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	unapproved, err := openDB(filepath.Join(t.TempDir(), "data.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer unapproved.close()
	if err = unapproved.tx(schema); err != nil {
		t.Fatal(err)
	}
	if err = unapproved.importGraph(graph); err != nil {
		t.Fatal(err)
	}
	if err = unapproved.promoteRisk(graph.ID, "K-SCHEDULE", "R-ENROLL"); err == nil {
		t.Fatal("risk promoted to a rule the user has not approved")
	}
	d, _ := riskTestDB(t)
	if err = d.promoteRisk(graph.ID, "K-SCHEDULE", "R-MISSING"); err == nil {
		t.Fatal("risk promoted to a nonexistent rule")
	}
	if err = d.promoteRisk(graph.ID, "K-SCHEDULE", "R-ENROLL"); err != nil {
		t.Fatal(err)
	}
	report, err := d.riskReport(graph.ID)
	if err != nil {
		t.Fatal(err)
	}
	if entry := riskByID(t, report, "K-SCHEDULE"); entry.Status != "PROMOTED" || entry.Rule != "R-ENROLL" {
		t.Fatalf("promotion not recorded: %+v", entry)
	}
}

func TestRiskReportListsEvidenceAndDecisions(t *testing.T) {
	d, graph := riskTestDB(t)
	report, err := d.riskReport(graph.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.PendingDecisions) != 1 || report.PendingDecisions[0] != "K-SCHEDULE" {
		t.Fatalf("open risk must await a user decision, got %v", report.PendingDecisions)
	}
	if err = d.acceptRisk(graph.ID, "K-SCHEDULE", "仅限选修课", "选修课时间重叠我们允许"); err != nil {
		t.Fatal(err)
	}
	if report, err = d.riskReport(graph.ID); err != nil {
		t.Fatal(err)
	}
	entry := riskByID(t, report, "K-SCHEDULE")
	if entry.Status != "ACCEPTED_RISK" || entry.Scope != "仅限选修课" || entry.Quote != "选修课时间重叠我们允许" || len(entry.History) != 1 {
		t.Fatalf("decision not reported: %+v", entry)
	}
	if len(report.PendingDecisions) != 0 {
		t.Fatalf("decided risk still pending: %v", report.PendingDecisions)
	}
}

func TestOpenRiskCountExcludesDecided(t *testing.T) {
	d, graph := riskTestDB(t)
	before, err := d.gate(graph.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before["open_risks"] != 1 {
		t.Fatalf("open risks before decision = %v", before["open_risks"])
	}
	if err = d.acceptRisk(graph.ID, "K-SCHEDULE", "仅限选修课", "选修课时间重叠我们允许"); err != nil {
		t.Fatal(err)
	}
	after, err := d.gate(graph.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after["open_risks"] != 0 {
		t.Fatalf("accepted risk still counted as open: %v", after["open_risks"])
	}
}

func TestAcceptanceReportExcludesRisks(t *testing.T) {
	d, graph := riskTestDB(t)
	report, err := d.acceptanceReport(graph.ID)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(body)), "risk") {
		t.Fatalf("acceptance report mixes in risks: %s", body)
	}
	for _, key := range []string{"verdict", "candidate_gaps"} {
		if _, ok := report[key]; !ok {
			t.Errorf("acceptance report lacks %s", key)
		}
	}
}

func TestRiskReviewGateOnlyWhenConfigured(t *testing.T) {
	disabled := map[string]any{"verdict": "PASS", "open_risks": 1}
	applyRiskReview(disabled, false)
	if disabled["verdict"] != "PASS" {
		t.Fatal("open risks blocked acceptance without a configured risk review")
	}
	enabled := map[string]any{"verdict": "PASS", "open_risks": 1}
	applyRiskReview(enabled, true)
	if enabled["verdict"] != "FAIL" || enabled["risk_review_blocked"] != true {
		t.Fatalf("configured risk review ignored open risks: %v", enabled)
	}
	clear := map[string]any{"verdict": "PASS", "open_risks": 0}
	applyRiskReview(clear, true)
	if clear["verdict"] != "PASS" {
		t.Fatalf("risk review failed without open risks: %v", clear)
	}
}

func TestRisksMigrationIdempotent(t *testing.T) {
	path, err := filepath.Abs("examples/academic-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	dbfile := filepath.Join(t.TempDir(), "data.db")
	for _, args := range [][]string{{"init", "--db", dbfile}, {"init", "--db", dbfile}, {"import", "--db", dbfile, "--file", path}} {
		if err := run(args); err != nil {
			t.Fatal(err)
		}
	}
	d, err := openDB(dbfile, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = d.approve("academic-v1", ""); err != nil {
		t.Fatal(err)
	}
	if err = d.acceptRisk("academic-v1", "K-SCHEDULE", "仅限选修课", "选修课时间重叠我们允许"); err != nil {
		t.Fatal(err)
	}
	d.close()
	if err = run([]string{"init", "--db", dbfile}); err != nil {
		t.Fatal(err)
	}
	if d, err = openDB(dbfile, false); err != nil {
		t.Fatal(err)
	}
	defer d.close()
	report, err := d.riskReport("academic-v1")
	if err != nil {
		t.Fatal(err)
	}
	if entry := riskByID(t, report, "K-SCHEDULE"); entry.Status != "ACCEPTED_RISK" || len(entry.History) != 1 {
		t.Fatalf("re-init lost the risk decision: %+v", entry)
	}
}

func TestHandoffShowsEffectiveRiskStatus(t *testing.T) {
	d, graph := riskTestDB(t)
	if err := d.acceptRisk(graph.ID, "K-SCHEDULE", "仅限选修课", "选修课时间重叠我们允许"); err != nil {
		t.Fatal(err)
	}
	packet, err := d.handoff("arc-plan", graph.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(packet.Risks) != 1 || packet.Risks[0][2] != "ACCEPTED_RISK" {
		t.Fatalf("handoff shows stale risk status: %v", packet.Risks)
	}
}

func TestAcceptanceReportGapsFollowScanObservations(t *testing.T) {
	d, graph := riskTestDB(t)
	dir := t.TempDir()
	scan := func(status string) {
		t.Helper()
		evidence := filepath.Join(dir, status+".json")
		if err := os.WriteFile(evidence, []byte(`{"observed":"`+status+`"}`), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := d.scan(Implementation{Graph: graph.ID, Entrances: []Entrance{{Action: "enroll", Status: status, Evidence: evidence}}}); err != nil {
			t.Fatal(err)
		}
	}
	gaps := func() []map[string]string {
		t.Helper()
		report, err := d.acceptanceReport(graph.ID)
		if err != nil {
			t.Fatal(err)
		}
		return report["candidate_gaps"].([]map[string]string)
	}
	if got := gaps(); len(got) != 1 || got[0]["observation"] != "UNOBSERVED" {
		t.Fatalf("unscanned action must stay a candidate gap, got %v", got)
	}
	scan("PRESENT")
	if got := gaps(); len(got) != 0 {
		t.Fatalf("present entrance reported as a gap: %v", got)
	}
	scan("NOT_FOUND")
	if got := gaps(); len(got) != 1 || got[0]["observation"] != "NOT_FOUND" {
		t.Fatalf("latest NOT_FOUND observation must be a candidate gap, got %v", got)
	}
}
