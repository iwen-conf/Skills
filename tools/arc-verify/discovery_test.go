package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type observationCase struct {
	Entry, Actor, Principal, Action, Before, After string
	Denied                                         bool
	Login                                          bool
	Enrolled                                       int
}

// explorationFile writes an exploration record with one evidence file per observation.
func explorationFile(t *testing.T, cases ...observationCase) string {
	t.Helper()
	dir := t.TempDir()
	observations := []map[string]any{}
	for index, item := range cases {
		evidence := filepath.Join(dir, fmt.Sprintf("observation-%d.json", index))
		if err := os.WriteFile(evidence, []byte(fmt.Sprintf(`{"request":"%s %s","response":"recorded by the exploring runner %d"}`, item.Action, item.Entry, index)), 0o600); err != nil {
			t.Fatal(err)
		}
		observations = append(observations, map[string]any{
			"entry": item.Entry, "role": item.Actor, "principal": item.Principal, "action": item.Action,
			"before_state": item.Before, "after_state": item.After, "denied": item.Denied,
			"conditions": map[string]bool{"login": item.Login}, "measurements": map[string]int{"enrolled": item.Enrolled},
			"evidence": evidence,
		})
	}
	body, err := json.Marshal(map[string]any{"graph": "course-v2", "exploration": "explore-1", "observations": observations})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "exploration.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

var (
	withinContract    = observationCase{Entry: "/enroll", Actor: "STUDENT", Principal: "primary", Action: "enroll", Before: "pending", After: "ready", Login: true}
	rejectedAsRuled   = observationCase{Entry: "/enroll", Actor: "STUDENT", Principal: "primary", Action: "enroll", Before: "pending", After: "pending", Denied: true}
	skipsReady        = observationCase{Entry: "/confirm", Actor: "STUDENT", Principal: "primary", Action: "confirm", Before: "pending", After: "confirmed", Login: true}
	undeclaredAction  = observationCase{Entry: "/refund", Actor: "STUDENT", Principal: "primary", Action: "refund", Before: "ready", After: "pending", Login: true, Enrolled: 1}
	undeclaredState   = observationCase{Entry: "/enroll", Actor: "STUDENT", Principal: "primary", Action: "enroll", Before: "pending", After: "waitlisted", Login: true}
	acceptedLoggedOut = observationCase{Entry: "/enroll", Actor: "STUDENT", Principal: "primary", Action: "enroll", Before: "pending", After: "ready"}
	deniedButChanged  = observationCase{Entry: "/confirm", Actor: "STUDENT", Principal: "primary", Action: "confirm", Before: "ready", After: "confirmed", Denied: true, Login: true}
)

func recordExploration(t *testing.T, d db, cases ...observationCase) DiscoveryReport {
	t.Helper()
	var report DiscoveryReport
	if err := json.Unmarshal(cliJSON(t, "discover", "record", "--db", d.file, "--file", explorationFile(t, cases...)), &report); err != nil {
		t.Fatal(err)
	}
	return report
}

func TestDiscoveryOutsideContractIsUnspecified(t *testing.T) {
	d := modelTestDB(t, kindsModel(t))
	report := recordExploration(t, d, withinContract, rejectedAsRuled, skipsReady, undeclaredAction, undeclaredState, acceptedLoggedOut, deniedButChanged)
	if report.WithinContract != 2 {
		t.Fatalf("declared behavior must not become UNSPECIFIED: %+v", report)
	}
	kinds := map[string]string{}
	for _, item := range report.Unspecified {
		kinds[item.Action+"/"+item.AfterState] = item.Kind
		if item.Status != "UNSPECIFIED" || len(item.EvidenceSHA256) != 64 || item.Entry == "" || item.Principal == "" {
			t.Fatalf("UNSPECIFIED item lacks status, evidence or subject: %+v", item)
		}
	}
	want := map[string]string{"confirm/confirmed": "UNDECLARED_TRANSITION", "refund/pending": "UNDECLARED_ACTION", "enroll/waitlisted": "UNDECLARED_STATE", "enroll/ready": "UNDECLARED_TRANSITION"}
	deniedChanged := false
	for _, item := range report.Unspecified {
		deniedChanged = deniedChanged || item.Denied && item.Action == "confirm" && strings.Contains(item.Reason, "rejected but the state changed")
	}
	if !deniedChanged {
		t.Fatalf("a rejection that still changed the state must be UNSPECIFIED: %+v", report.Unspecified)
	}
	for key, kind := range want {
		if kinds[key] != kind {
			t.Fatalf("%s should be %s: %+v", key, kind, kinds)
		}
	}
	if len(report.Unspecified) != len(want)+1 {
		t.Fatalf("unexpected UNSPECIFIED items: %+v", report.Unspecified)
	}
}

func TestDiscoveryNeverPassOrFail(t *testing.T) {
	g := kindsModel(t)
	d := modelTestDB(t, g)
	before, err := d.analyzeModel(g, "", "")
	if err != nil {
		t.Fatal(err)
	}
	recordExploration(t, d, skipsReady, undeclaredAction)
	after, err := d.analyzeModel(g, "", "")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(before.Coverage)
	b, _ := json.Marshal(after.Coverage)
	if string(a) != string(b) || before.Verdict != after.Verdict {
		t.Fatal("exploration changed the acceptance denominator or verdict")
	}
	items, err := d.unspecified("course-v2")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Status == "PASS" || item.Status == "FAIL" {
			t.Fatalf("UNSPECIFIED was judged: %+v", item)
		}
	}
}

func TestDiscoveryDedupByEquivalenceKey(t *testing.T) {
	d := modelTestDB(t, kindsModel(t))
	first := recordExploration(t, d, skipsReady)
	second := recordExploration(t, d, skipsReady)
	if len(first.Unspecified) != 1 || len(second.Unspecified) != 0 || len(second.Merged) != 1 {
		t.Fatalf("an equivalent observation must merge: %+v %+v", first, second)
	}
	differentCount := skipsReady
	differentCount.Enrolled = 7
	third := recordExploration(t, d, differentCount)
	if len(third.Unspecified) != 1 || third.Unspecified[0].Kind != "UNDECLARED_DIFFERENCE" {
		t.Fatalf("an undeclared measurement difference must be its own risk candidate: %+v", third)
	}
	budget := []observationCase{}
	for index := 0; index < 3; index++ {
		item := undeclaredAction
		item.Action = fmt.Sprintf("refund-%d", index)
		budget = append(budget, item)
	}
	var capped DiscoveryReport
	if err := json.Unmarshal(cliJSON(t, "discover", "record", "--db", d.file, "--file", explorationFile(t, budget...), "--budget", "2"), &capped); err != nil {
		t.Fatal(err)
	}
	if len(capped.Unspecified) != 2 || !capped.Truncated {
		t.Fatalf("each exploration must respect its item budget: %+v", capped)
	}
}

func TestTriageRequiresInteractiveConfirmation(t *testing.T) {
	d := modelTestDB(t, kindsModel(t))
	report := recordExploration(t, d, skipsReady)
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = writer.WriteString("TRIAGE " + report.Unspecified[0].ID + " missing_requirement\n确认跳过报名直接确认是缺的需求\n"); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	old := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = old; reader.Close() }()
	err = run([]string{"discover", "triage", "--db", d.file, "--graph", "course-v2", "--id", report.Unspecified[0].ID, "--as", "missing_requirement"})
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("piped triage must be refused: %v", err)
	}
	items, err := d.unspecified("course-v2")
	if err != nil || items[0].Triage != "OPEN" {
		t.Fatalf("a refused triage changed the item: %+v %v", items, err)
	}
}

func TestTriageMissingRequirementBecomesCandidate(t *testing.T) {
	d := modelTestDB(t, kindsModel(t))
	report := recordExploration(t, d, skipsReady, undeclaredAction, undeclaredState)
	ids := map[string]string{}
	for _, item := range report.Unspecified {
		ids[item.Kind+"/"+item.Action] = item.ID
	}
	if err := d.triageUnspecified("course-v2", ids["UNDECLARED_TRANSITION/confirm"], "missing_requirement", "跳过报名直接确认是遗漏的需求"); err != nil {
		t.Fatal(err)
	}
	if err := d.triageUnspecified("course-v2", ids["UNDECLARED_ACTION/refund"], "should_not_exist", "系统不应提供退款入口"); err != nil {
		t.Fatal(err)
	}
	if err := d.triageUnspecified("course-v2", ids["UNDECLARED_STATE/enroll"], "allowed_extension", ""); err == nil {
		t.Fatal("a triage without the user's words was accepted")
	}
	if err := d.triageUnspecified("course-v2", ids["UNDECLARED_ACTION/refund"], "allowed_extension", "改主意了"); err == nil {
		t.Fatal("an already triaged item was decided twice")
	}
	packet, err := d.handoff("arc-plan", "course-v2", "")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(packet)
	if !strings.Contains(string(body), ids["UNDECLARED_TRANSITION/confirm"]) || !strings.Contains(string(body), "跳过报名直接确认是遗漏的需求") {
		t.Fatalf("a missing requirement must reach arc-plan as a candidate: %s", body)
	}
	rules, err := d.query("SELECT COUNT(*) FROM rules WHERE graph='course-v2'", 1)
	if err != nil || rules[0][0] != "2" {
		t.Fatalf("triage must not write rules into the frozen graph version: %v %v", rules, err)
	}
	risks, err := d.riskReport("course-v2")
	if err != nil {
		t.Fatal(err)
	}
	riskBody, _ := json.Marshal(risks)
	if !strings.Contains(string(riskBody), ids["UNDECLARED_ACTION/refund"]) || strings.Contains(string(riskBody), ids["UNDECLARED_TRANSITION/confirm"]) {
		t.Fatalf("only should_not_exist items become risks: %s", riskBody)
	}
}
