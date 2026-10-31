package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// triageModel declares risks from three origins so decisions can be grouped.
func triageModel(t *testing.T) Graph {
	t.Helper()
	g := kindsModel(t)
	g.Risks = []Risk{
		{ID: "K-AI-1", Description: "AI suspects a double submit", Action: "enroll", Origin: "ai_inference"},
		{ID: "K-AI-2", Description: "AI suspects confirmation without login", Action: "confirm", Origin: "ai_inference"},
		{ID: "K-AI-3", Description: "AI suspects stale counts", Action: "enroll", Origin: "ai_inference"},
		{ID: "K-PRACTICE", Description: "industry practice: rate limit enrollment", Action: "enroll", Origin: "industry_practice"},
	}
	if err := validateGraph(g); err != nil {
		t.Fatal(err)
	}
	return g
}

func summaryFor(t *testing.T, d db, origin string) TriageSummary {
	t.Helper()
	report, err := d.riskReport("course-v2")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range report.TriageSummary {
		if row.Origin == origin {
			return row
		}
	}
	t.Fatalf("origin %s missing from the triage summary: %+v", origin, report.TriageSummary)
	return TriageSummary{}
}

func TestTriageLedgerGroupsByOrigin(t *testing.T) {
	d := modelTestDB(t, triageModel(t))
	if err := d.acceptRisk("course-v2", "K-AI-1", "enrollment only", "双击会重复提交，接受这个风险先上线"); err != nil {
		t.Fatal(err)
	}
	if err := d.rejectRisk("course-v2", "K-AI-2", "确认接口有登录拦截，不是问题"); err != nil {
		t.Fatal(err)
	}
	if err := d.rejectRisk("course-v2", "K-PRACTICE", "  "); err == nil {
		t.Fatal("a rejection without the user's words was recorded")
	}
	if err := d.rejectRisk("course-v2", "K-AI-2", "again"); err == nil {
		t.Fatal("a decided risk was rejected twice")
	}
	report := recordExploration(t, d, skipsReady, undeclaredAction)
	if err := d.triageUnspecified("course-v2", report.Unspecified[0].ID, "missing_requirement", "确实漏了"); err != nil {
		t.Fatal(err)
	}
	if err := d.triageUnspecified("course-v2", report.Unspecified[1].ID, "allowed_extension", "允许"); err != nil {
		t.Fatal(err)
	}
	ai := summaryFor(t, d, "ai_inference")
	if ai.Accepted != 1 || ai.Rejected != 1 || ai.Pending != 1 || ai.Sample != 3 {
		t.Fatalf("ai_inference decisions miscounted: %+v", ai)
	}
	practice := summaryFor(t, d, "industry_practice")
	if practice.Pending != 1 || practice.Sample != 1 {
		t.Fatalf("industry_practice decisions miscounted: %+v", practice)
	}
	discovery := summaryFor(t, d, "discovery")
	if discovery.Accepted != 1 || discovery.Rejected != 1 || discovery.Sample != 2 {
		t.Fatalf("UNSPECIFIED triage miscounted: %+v", discovery)
	}
	risks, err := d.riskReport("course-v2")
	if err != nil {
		t.Fatal(err)
	}
	for _, risk := range risks.Risks {
		if risk.ID == "K-AI-2" && (risk.Status != "REJECTED" || risk.Quote != "确认接口有登录拦截，不是问题") {
			t.Fatalf("a rejection must keep the user's words: %+v", risk)
		}
	}
}

func TestTriageLedgerShowsSampleSize(t *testing.T) {
	d := modelTestDB(t, triageModel(t))
	if row := summaryFor(t, d, "ai_inference"); !row.InsufficientSample || row.Sample != 3 {
		t.Fatalf("fewer than 10 decisions must be flagged: %+v", row)
	}
	cases := []observationCase{}
	for index := 0; index < 10; index++ {
		item := undeclaredAction
		item.Action = fmt.Sprintf("refund-%d", index)
		cases = append(cases, item)
	}
	var report DiscoveryReport
	if err := decodeStrictJSON(cliJSON(t, "discover", "record", "--db", d.file, "--file", explorationFile(t, cases...), "--budget", "32"), &report); err != nil {
		t.Fatal(err)
	}
	for _, item := range report.Unspecified {
		if err := d.triageUnspecified("course-v2", item.ID, "should_not_exist", "不该有"); err != nil {
			t.Fatal(err)
		}
	}
	if row := summaryFor(t, d, "discovery"); row.InsufficientSample || row.Sample != 10 || row.Accepted != 10 {
		t.Fatalf("ten decisions are a reportable sample: %+v", row)
	}
}

func TestTriageLedgerNotAGate(t *testing.T) {
	g := triageModel(t)
	d := modelTestDB(t, g)
	before, err := d.modelGate(g)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.rejectRisk("course-v2", "K-AI-3", "不是问题"); err != nil {
		t.Fatal(err)
	}
	after, err := d.modelGate(g)
	if err != nil {
		t.Fatal(err)
	}
	if before["verdict"] != after["verdict"] || before["required_tasks"] != after["required_tasks"] {
		t.Fatal("the triage ledger changed the acceptance verdict")
	}
	if _, found := after["triage_summary"]; found {
		t.Fatal("the triage summary must not appear in the gate")
	}
}

func TestRejectedReferenceIsSuppressed(t *testing.T) {
	d := academicLedger(t)
	matchCatalog(t, d, "academic-v1", enrollmentCatalog)
	if err := d.rejectRisk("academic-v1", "ref/ENR-CAPACITY-RACE", "名额由数据库唯一约束保证，不用再查"); err != nil {
		t.Fatal(err)
	}
	report, err := d.matchReferences("academic-v1")
	if err != nil {
		t.Fatal(err)
	}
	got := outcomes(report)["ENR-CAPACITY-RACE"]
	if got.Outcome != "SUPPRESSED" || !strings.Contains(got.SuppressedBy, "rejected by the user") {
		t.Fatalf("a rejected reference with the same bindings must stay suppressed: %+v", got)
	}
	if row := summaryForGraph(t, d, "academic-v1", "peer_reference"); row.Rejected != 1 {
		t.Fatalf("the reference rejection is missing from the ledger: %+v", row)
	}
}

func summaryForGraph(t *testing.T, d db, graph, origin string) TriageSummary {
	t.Helper()
	report, err := d.riskReport(graph)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range report.TriageSummary {
		if row.Origin == origin {
			return row
		}
	}
	t.Fatalf("origin %s missing: %+v", origin, report.TriageSummary)
	return TriageSummary{}
}

func TestRejectRiskRequiresInteractiveTerminal(t *testing.T) {
	d := modelTestDB(t, triageModel(t))
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writer.WriteString("REJECT K-AI-1\n不是问题\n"); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	old := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = old; reader.Close() }()
	if err := run([]string{"reject-risk", "--db", d.file, "--graph", "course-v2", "--risk", "K-AI-1"}); err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("piped rejection must be refused: %v", err)
	}
}
