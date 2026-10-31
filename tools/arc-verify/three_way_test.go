package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// threeWayModel adds an action no rule requires, so implementation evidence can exceed the requirements.
func threeWayModel(t *testing.T) Graph {
	t.Helper()
	g := kindsModel(t)
	g.Nodes = append(g.Nodes, Node{ID: "export", Kind: "action", Name: "导出报名名单"})
	g.Edges = append(g.Edges, Edge{ID: "implements-export", From: "enrollment.submit", To: "export", Kind: "implements"})
	if err := validateGraph(g); err != nil {
		t.Fatal(err)
	}
	return g
}

const readyCatalog = `{"catalog": "ready", "entries": [{"id": "ENR-NO-DUPLICATE", "domain": "enrollment", "invariant": "re-submitting while enrolled never creates a second enrollment",
  "slots": [{"name": "submit", "kind": "action"}, {"name": "enrolled", "kind": "state", "hints": ["ready"]}], "relations": [{"kind": "transitions", "from": "submit", "to": "enrolled"}],
  "trust": "user_design_document", "source": "自动化测试规范设计-6ae40e01.md", "source_ref": "L333"}]}`

func threeWayLedger(t *testing.T) (db, ThreeWayReport) {
	t.Helper()
	g := threeWayModel(t)
	d := modelTestDB(t, g)
	if _, err := d.scan(Implementation{Graph: g.ID, Entrances: []Entrance{
		{Action: "confirm", Status: "NOT_FOUND", Evidence: writeScanEvidence(t, "arc-idx search found no confirm handler")},
		{Action: "export", Status: "PRESENT", Evidence: writeScanEvidence(t, "handlers/export.go:12 GET /enrollments/export")},
		{Action: "enroll", Status: "PRESENT", Evidence: writeScanEvidence(t, "handlers/enroll.go:20 POST /enroll")},
	}}); err != nil {
		t.Fatal(err)
	}
	recordExploration(t, d, undeclaredAction)
	matchCatalog(t, d, g.ID, readyCatalog)
	report, err := d.riskReport(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	return d, report.ThreeWay
}

func TestThreeWayReportKeepsSourcesApart(t *testing.T) {
	_, report := threeWayLedger(t)
	columns := map[string][]ThreeWayEntry{"required_not_implemented": report.RequiredNotImplemented, "implemented_not_required": report.ImplementedNotRequired, "referenced_not_required": report.ReferencedNotRequired}
	want := map[string]string{"required_not_implemented": "requirement", "implemented_not_required": "implementation", "referenced_not_required": "peer_reference"}
	for column, entries := range columns {
		if len(entries) == 0 {
			t.Fatalf("column %s is empty", column)
		}
		for _, entry := range entries {
			if entry.Source != want[column] || entry.Subject == "" || entry.Evidence == "" {
				t.Fatalf("column %s mixes sources or lacks evidence: %+v", column, entry)
			}
		}
	}
	if len(report.RequiredNotImplemented) != 1 || report.RequiredNotImplemented[0].Subject != "R2" || len(report.RequiredNotImplemented[0].SHA256) != 64 {
		t.Fatalf("missing implementation must cite the rule and the scan evidence: %+v", report.RequiredNotImplemented)
	}
	subjects := []string{}
	for _, entry := range report.ImplementedNotRequired {
		subjects = append(subjects, entry.Subject)
	}
	if strings.Contains(","+strings.Join(subjects, ",")+",", ",enroll,") {
		t.Fatalf("an implemented and required action is not a difference: %v", subjects)
	}
	if !strings.Contains(strings.Join(subjects, ","), "export") || !strings.Contains(strings.Join(subjects, ","), "refund") {
		t.Fatalf("observed and scanned behavior without a requirement must both be listed: %v", subjects)
	}
}

func TestImplementationEvidenceNeverBecomesRule(t *testing.T) {
	d, _ := threeWayLedger(t)
	rules, err := d.query("SELECT COUNT(*) FROM rules WHERE graph='course-v2'", 1)
	if err != nil || rules[0][0] != "2" {
		t.Fatalf("implementation evidence changed the requirements: %v %v", rules, err)
	}
	acceptance, err := d.acceptanceReport("course-v2")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(acceptance)
	if strings.Contains(string(body), "export") && !strings.Contains(string(body), "candidate_gaps") {
		t.Fatalf("implementation evidence entered the acceptance report as a requirement: %s", body)
	}
}

func TestPeerReferenceOnlyYieldsRisk(t *testing.T) {
	d, report := threeWayLedger(t)
	if len(report.ReferencedNotRequired) != 1 || report.ReferencedNotRequired[0].Subject != "ref/ENR-NO-DUPLICATE" || !strings.HasPrefix(report.ReferencedNotRequired[0].Detail, "ENR-NO-DUPLICATE@") {
		t.Fatalf("the peer reference column must cite its catalog entry: %+v", report.ReferencedNotRequired)
	}
	acceptance, err := d.acceptanceReport("course-v2")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(acceptance)
	if strings.Contains(string(body), "ENR-NO-DUPLICATE") {
		t.Fatalf("a peer reference entered the acceptance report: %s", body)
	}
}
