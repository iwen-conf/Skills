package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// enrollmentCatalog mirrors the W19 research drafts (sources are the user's design dialog).
const enrollmentCatalog = `{
  // W19 draft entries; matching only ever produces RISK candidates.
  "catalog": "enrollment",
  "entries": [
    {"id": "ENR-CAPACITY-RACE", "domain": "enrollment", "invariant": "concurrent submissions for the last seat never exceed capacity",
     "slots": [{"name": "submit", "kind": "action"}, {"name": "capacity", "kind": "condition", "hints": ["seat", "capacity", "quota", "名额"]}],
     "relations": [{"kind": "requires", "from": "capacity", "to": "submit"}],
     "trust": "user_design_document", "source": "自动化测试规范设计-6ae40e01.md", "source_ref": "L340, L442, L547"},
    {"id": "ENR-NO-DUPLICATE", "domain": "enrollment", "invariant": "re-submitting while enrolled never creates a second enrollment",
     "slots": [{"name": "submit", "kind": "action"}, {"name": "enrolled", "kind": "state", "hints": ["enrolled", "已报名"]}],
     "relations": [{"kind": "transitions", "from": "submit", "to": "enrolled"}],
     "trust": "user_design_document", "source": "自动化测试规范设计-6ae40e01.md", "source_ref": "L333"},
    {"id": "ENR-CANCEL-RELEASES", "domain": "enrollment", "invariant": "cancelling an enrollment releases exactly one seat",
     "slots": [{"name": "cancel", "kind": "action", "hints": ["cancel", "取消"]}],
     "trust": "user_design_document", "source": "自动化测试规范设计-6ae40e01.md", "source_ref": "L775, L865"},
    {"id": "ENR-CLOSED-ACTIVITY", "domain": "enrollment", "invariant": "after the activity is cancelled no enrollment is accepted",
     "slots": [{"name": "submit", "kind": "action"}, {"name": "open", "kind": "condition", "hints": ["open", "active", "window", "开放"]}],
     "relations": [{"kind": "requires", "from": "open", "to": "submit"}],
     "trust": "user_design_document", "source": "自动化测试规范设计-6ae40e01.md", "source_ref": "L628"}
  ]
}`

func writeCatalog(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "catalog.jsonc")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func academicLedger(t *testing.T) db {
	t.Helper()
	graph, err := loadGraph("examples/academic-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return modelTestDB(t, graph)
}

func matchCatalog(t *testing.T, d db, graph, catalog string) ReferenceMatchReport {
	t.Helper()
	cliJSON(t, "reference-import", "--db", d.file, "--file", writeCatalog(t, catalog))
	var report ReferenceMatchReport
	if err := json.Unmarshal(cliJSON(t, "reference-match", "--db", d.file, "--graph", graph), &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func outcomes(report ReferenceMatchReport) map[string]ReferenceMatch {
	result := map[string]ReferenceMatch{}
	for _, match := range report.Matches {
		result[match.Entry] = match
	}
	return result
}

func TestReferenceImportRejectsUnsourcedEntry(t *testing.T) {
	d := academicLedger(t)
	for name, body := range map[string]string{
		"no source":     strings.Replace(enrollmentCatalog, `"source_ref": "L333"`, `"source_ref": ""`, 1),
		"unknown trust": strings.Replace(enrollmentCatalog, `"trust": "user_design_document", "source": "自动化测试规范设计-6ae40e01.md", "source_ref": "L628"`, `"trust": "ai_guess", "source": "x", "source_ref": "y"`, 1),
		"unknown slot":  strings.Replace(enrollmentCatalog, `{"kind": "requires", "from": "open", "to": "submit"}`, `{"kind": "requires", "from": "nowhere", "to": "submit"}`, 1),
		"duplicate id":  strings.Replace(enrollmentCatalog, `"id": "ENR-CLOSED-ACTIVITY"`, `"id": "ENR-CAPACITY-RACE"`, 1),
	} {
		if err := run([]string{"reference-import", "--db", d.file, "--file", writeCatalog(t, body)}); err == nil {
			t.Fatalf("%s: an invalid catalog was imported", name)
		}
	}
	rows, err := d.query("SELECT COUNT(*) FROM reference_catalogs", 1)
	if err != nil || rows[0][0] != "0" {
		t.Fatalf("a rejected catalog left rows: %v %v", rows, err)
	}
}

func TestReferenceMatchUniqueBindingCreatesPeerRisk(t *testing.T) {
	d := academicLedger(t)
	report := matchCatalog(t, d, "academic-v1", enrollmentCatalog)
	got := outcomes(report)
	for _, id := range []string{"ENR-CAPACITY-RACE", "ENR-NO-DUPLICATE", "ENR-CLOSED-ACTIVITY"} {
		match := got[id]
		if match.Outcome != "RISK" || match.Risk == nil || match.Risk.Origin != "peer_reference" || !strings.HasPrefix(match.Risk.Reference, id+"@") || len(match.CatalogSHA256) != 64 {
			t.Fatalf("%s must become a traceable peer_reference RISK: %+v", id, match)
		}
	}
	if got["ENR-CAPACITY-RACE"].Bindings["capacity"] != "seats_available" || got["ENR-CAPACITY-RACE"].Bindings["submit"] != "enroll" {
		t.Fatalf("binding lost: %+v", got["ENR-CAPACITY-RACE"])
	}
	risks, err := d.riskReport("academic-v1")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(risks)
	if !strings.Contains(string(body), `"origin":"peer_reference"`) || !strings.Contains(string(body), "ENR-CAPACITY-RACE@") {
		t.Fatalf("matched references are missing from the risk report: %s", body)
	}
}

func TestReferenceNoBindingNamesSlot(t *testing.T) {
	d := academicLedger(t)
	got := outcomes(matchCatalog(t, d, "academic-v1", enrollmentCatalog))
	if match := got["ENR-CANCEL-RELEASES"]; match.Outcome != "NO_BINDING" || match.Slot != "cancel" || match.Risk != nil {
		t.Fatalf("a missing binding must name its slot and create no risk: %+v", match)
	}
}

func TestReferenceAmbiguousGivesOneDiagnostic(t *testing.T) {
	d := academicLedger(t)
	catalog := `{"catalog": "generic", "entries": [{"id": "GEN-ANY-CONDITION", "domain": "generic", "invariant": "every condition gating an action is enforced server-side",
	  "slots": [{"name": "guard", "kind": "condition"}, {"name": "act", "kind": "action"}], "relations": [{"kind": "requires", "from": "guard", "to": "act"}],
	  "trust": "public_spec", "source": "https://owasp.org/API-Security/", "source_ref": "API5"}]}`
	report := matchCatalog(t, d, "academic-v1", catalog)
	if len(report.Matches) != 1 || report.Matches[0].Outcome != "AMBIGUOUS" || len(report.Matches[0].Candidates) != 3 || report.Matches[0].Risk != nil {
		t.Fatalf("several bindings must give one diagnostic and no risk: %+v", report.Matches)
	}
}

func TestReferenceSuppressedByCoversAndBindings(t *testing.T) {
	g := referenceModel(t)
	catalog := strings.Replace(enrollmentCatalog, `"ENR-NO-DUPLICATE"`, `"ENR-NO-DUPLICATE"`, 1)
	d := modelTestDB(t, g)
	if got := outcomes(matchCatalog(t, d, "course-v2", catalog)); got["ENR-NO-DUPLICATE"].Outcome != "NO_BINDING" || got["ENR-NO-DUPLICATE"].Slot != "enrolled" {
		t.Fatalf("the synonym 报名成功 is not matched by hints alone: %+v", got["ENR-NO-DUPLICATE"])
	}
	bound := referenceModel(t)
	bound.ID = "course-v3"
	bound.ReferenceBindings = []ReferenceBinding{
		{Entry: "ENR-NO-DUPLICATE", Bindings: map[string]string{"enrolled": "ready"}, Source: "user", SourceRef: "user: 报名成功 就是已报名"},
		{Entry: "ENR-CLOSED-ACTIVITY", NotApplicable: true, Reason: "courses are never cancelled in this product", Source: "document", SourceRef: "fixture"},
	}
	if err := validateGraph(bound); err != nil {
		t.Fatal(err)
	}
	d2 := modelTestDB(t, bound)
	got := outcomes(matchCatalog(t, d2, "course-v3", catalog))
	if got["ENR-NO-DUPLICATE"].Outcome != "RISK" || got["ENR-NO-DUPLICATE"].Bindings["enrolled"] != "ready" {
		t.Fatalf("an explicit binding must resolve the synonym: %+v", got["ENR-NO-DUPLICATE"])
	}
	covered := bound
	covered.ID = "course-v4"
	covered.ReferenceBindings = append([]ReferenceBinding{}, bound.ReferenceBindings...)
	covered.ReferenceBindings[0].CoveredBy = "invariant.capacity"
	d3 := modelTestDB(t, covered)
	got = outcomes(matchCatalog(t, d3, "course-v4", catalog))
	if got["ENR-NO-DUPLICATE"].Outcome != "SUPPRESSED" || !strings.Contains(got["ENR-NO-DUPLICATE"].SuppressedBy, "invariant.capacity") {
		t.Fatalf("a declared cover must suppress the candidate: %+v", got["ENR-NO-DUPLICATE"])
	}
	if got["ENR-CLOSED-ACTIVITY"].Outcome != "SUPPRESSED" || !strings.Contains(got["ENR-CLOSED-ACTIVITY"].SuppressedBy, "not applicable") {
		t.Fatalf("a sourced not-applicable binding must suppress the entry: %+v", got["ENR-CLOSED-ACTIVITY"])
	}
	invalid := covered
	invalid.ReferenceBindings = []ReferenceBinding{{Entry: "ENR-NO-DUPLICATE", Bindings: map[string]string{"enrolled": "missing-node"}, Source: "user", SourceRef: "x"}}
	if err := validateGraph(invalid); err == nil || !strings.Contains(err.Error(), "reference_bindings") {
		t.Fatalf("a binding to an unknown node was accepted: %v", err)
	}
}

func TestReferenceRiskNeverEntersAcceptanceGate(t *testing.T) {
	g := referenceModel(t)
	d := modelTestDB(t, g)
	before, err := d.modelGate(g)
	if err != nil {
		t.Fatal(err)
	}
	catalog := strings.Replace(enrollmentCatalog, `"hints": ["enrolled", "已报名"]`, `"hints": ["ready", "报名成功"]`, 1)
	report := matchCatalog(t, d, "course-v2", catalog)
	if outcomes(report)["ENR-NO-DUPLICATE"].Outcome != "RISK" {
		t.Fatalf("setup needs a RISK match: %+v", report)
	}
	after, err := d.modelGate(g)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"verdict", "required_tasks", "open_risks"} {
		if fmt.Sprint(before[key]) != fmt.Sprint(after[key]) {
			t.Fatalf("a reference match changed the acceptance gate %s: %v -> %v", key, before[key], after[key])
		}
	}
	rules, err := d.query("SELECT COUNT(*) FROM rules WHERE graph='course-v2'", 1)
	if err != nil || rules[0][0] != "1" {
		t.Fatalf("a reference match wrote a rule: %v %v", rules, err)
	}
}

func TestReferenceMatchIsCapped(t *testing.T) {
	d := academicLedger(t)
	entries := []string{}
	for index := 0; index < 55; index++ {
		entries = append(entries, fmt.Sprintf(`{"id": "CAP-%02d", "domain": "enrollment", "invariant": "capacity rule %d", "slots": [{"name": "capacity", "kind": "condition", "hints": ["seat"]}], "trust": "public_spec", "source": "https://example.org/spec", "source_ref": "section %d"}`, index, index, index))
	}
	report := matchCatalog(t, d, "academic-v1", `{"catalog": "capped", "entries": [`+strings.Join(entries, ",")+`]}`)
	risks := 0
	for _, match := range report.Matches {
		if match.Outcome == "RISK" {
			risks++
		}
	}
	if risks != 50 || !report.Truncated {
		t.Fatalf("one match run may produce at most 50 risks and must say it was truncated: %d %v", risks, report.Truncated)
	}
}

func TestExampleReferenceCatalogsAreValid(t *testing.T) {
	d := academicLedger(t)
	paths, err := filepath.Glob("examples/references/*.jsonc")
	if err != nil || len(paths) < 2 {
		t.Fatalf("example catalogs missing: %v %v", paths, err)
	}
	for _, path := range paths {
		if _, err := d.importReferences(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	report, err := d.matchReferences("academic-v1")
	if err != nil {
		t.Fatal(err)
	}
	risks := 0
	for _, match := range report.Matches {
		if match.Outcome == "RISK" {
			risks++
		}
	}
	// W19 research: academic-v1 yields three enrollment candidates and no IM candidate.
	if risks != 3 || len(report.Matches) != 9 {
		t.Fatalf("example catalogs no longer reproduce the W19 demonstration: %d risks, %d matches", risks, len(report.Matches))
	}
}
