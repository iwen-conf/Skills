package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func referenceModel(t *testing.T) Graph {
	t.Helper()
	graph, err := loadGraph(writeBusinessModel(t, businessModelExample))
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

func modelTestDB(t *testing.T, graph Graph) db {
	t.Helper()
	d, err := openDB(filepath.Join(t.TempDir(), "data.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.close() })
	for _, operation := range []func() error{
		func() error { return d.tx(schema) },
		func() error { return d.importGraph(graph) },
		func() error { return d.approve(graph.ID, "") },
	} {
		if err := operation(); err != nil {
			t.Fatal(err)
		}
	}
	for _, rule := range graph.Rules {
		if err := d.approve(graph.ID, rule.ID); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

const businessModelExample = `{
  "schema_version": 2,
  "id": "course-v2",
  "system": "course",
  "scope": "Reference course fixture for tool acceptance; not a product contract",
  "nodes": [
    {"id":"course","kind":"system","name":"课程系统"},
    {"id":"enrollment","kind":"domain","name":"报名业务"},
    {"id":"enrollment.submit","kind":"capability","name":"提交报名"},
    {"id":"course.boundary","kind":"boundary","name":"课程边界"},
    {"id":"STUDENT","kind":"role","name":"学员"},
    {"id":"enroll","kind":"action","name":"创建报名记录"},
    {"id":"login","kind":"condition","name":"已认证"},
    {"id":"pending","kind":"state","name":"未报名"},
    {"id":"ready","kind":"state","name":"报名成功"},
    {"id":"enrollment.records","kind":"data","name":"报名记录"}
  ],
  "contracts": [
    {"subject":"enrollment.submit","description":"Reference enrollment operation","inputs":["student","course"],"outputs":["enrollment_id"],"source":"document","source_ref":"reference-fixture"}
  ],
  "edges": [
    {"id":"contains-domain","from":"course","to":"enrollment","kind":"contains"},
    {"id":"contains-capability","from":"enrollment","to":"enrollment.submit","kind":"contains"},
    {"id":"implements-enroll","from":"enrollment.submit","to":"enroll","kind":"implements"},
    {"id":"owns-enroll","from":"course.boundary","to":"enrollment.submit","kind":"owns"},
    {"id":"owns-data","from":"course.boundary","to":"enrollment.records","kind":"owns"},
    {"id":"serves-student","from":"STUDENT","to":"enrollment","kind":"serves"},
    {"id":"requires-login","from":"login","to":"enroll","kind":"requires"},
    {"id":"creates-ready","from":"enroll","to":"ready","kind":"transitions"},
    {"id":"submit-flow","from":"pending","to":"ready","kind":"transitions_to"},
    {"id":"test-capacity","from":"test.idempotency","to":"invariant.capacity","kind":"verifies"}
  ],
  "rules": [{"id":"R1","action":"enroll","actor":"STUDENT","entry":"/enroll","oracle":"fixture-independent-store","conditions":["login"],"expected":"ready","rejected":"UNCHANGED","source":"document","description":"Authenticated reference fixture enrollment"}],
  "transitions": [{"edge":"submit-flow","action":"enroll","rule":"R1","preconditions":{"login":true},"outcome":"success"}],
  "flows": [{"id":"flow.enrollment","capability":"enrollment.submit","initial":"pending"}],
  "invariants": [{"id":"invariant.capacity","name":"报名不得超过容量","scope":["enrollment.submit"],"oracle":"fixture-independent-store","observation_boundary":"after committed enrollment and fixture restoration","checks":[{"left":"enrolled","operator":"lte","right":"capacity"}],"source":"document","source_ref":"reference-fixture"}],
  "tests": [{"id":"test.idempotency","name":"重复报名保持不变量","rule":"R1","kind":"IDEMPOTENCY","preconditions":{"login":true},"expected":"ready","source":"document","source_ref":"reference-fixture"}],
  "risks": []
}`

func writeBusinessModel(t *testing.T, raw string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "graph.json")
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestModelV2ThroughCLI(t *testing.T) {
	if err := run([]string{"validate", "--file", writeBusinessModel(t, businessModelExample)}); err != nil {
		t.Fatalf("valid three-layer model must be accepted through CLI: %v", err)
	}
}

func TestModelV2KeepsNewContractsOnRoundTrip(t *testing.T) {
	var graph Graph
	if err := decodeUniqueJSON([]byte(businessModelExample), &graph); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	var preserved map[string]json.RawMessage
	if err := json.Unmarshal(body, &preserved); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema_version", "system", "contracts", "transitions", "flows", "invariants", "tests"} {
		if len(preserved[key]) == 0 {
			t.Errorf("model silently discarded %s", key)
		}
	}
}

func TestModelUnknownFieldsAreNotSilentlyIgnored(t *testing.T) {
	body, err := json.Marshal(sample())
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	value["test_definitons"] = json.RawMessage(`[{"id":"lost.required.test"}]`)
	body, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadGraph(writeBusinessModel(t, string(body))); err == nil {
		t.Fatal("misspelled required model field was silently dropped")
	}
}

func TestModelV2PlannerRetainsDeclaredObligations(t *testing.T) {
	graph, err := loadGraph(writeBusinessModel(t, businessModelExample))
	if err != nil {
		t.Fatalf("valid model unavailable: %v", err)
	}
	d, err := openDB(filepath.Join(t.TempDir(), "data.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.close() })
	if err := d.tx(schema); err != nil {
		t.Fatal(err)
	}
	if err := d.importGraph(graph); err != nil {
		t.Fatal(err)
	}
	if err := d.approve(graph.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.approve(graph.ID, "R1"); err != nil {
		t.Fatal(err)
	}
	count, err := d.plan(graph.ID, 128)
	if err != nil {
		t.Fatal(err)
	}
	if count <= 3 {
		t.Fatalf("full model obligations collapsed to %d legacy tasks", count)
	}
	rows, err := d.query("SELECT id FROM tasks WHERE id="+quote(graph.ID+"/test.idempotency"), 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("declared idempotency test missing: %v, %v", rows, err)
	}
}

func TestModelV2ForwardVerificationReferenceIsOrderIndependent(t *testing.T) {
	graph := referenceModel(t)
	graph.Edges = append([]Edge{{ID: "verify-flow", From: "test.idempotency", To: "submit-flow", Kind: "verifies"}}, graph.Edges...)
	if err := validateGraph(graph); err != nil {
		t.Fatalf("verification of a later-declared transition must be accepted: %v", err)
	}
}

func TestModelV2RejectsWeakenedDerivedObligations(t *testing.T) {
	graph := referenceModel(t)
	graph.Tests[0].ID = "R1/POSITIVE"
	if err := validateGraph(graph); err == nil {
		t.Fatal("explicit test can replace the derived positive obligation")
	}
}

func TestModelV2RejectsContradictorySuccessTransition(t *testing.T) {
	graph := referenceModel(t)
	graph.Transitions[0].Preconditions["login"] = false
	if err := validateGraph(graph); err == nil {
		t.Fatal("a success transition contradicting the declared rule guard was accepted")
	}
}

func TestModelV2RejectsDuplicateSharedOwners(t *testing.T) {
	graph := referenceModel(t)
	graph.SharedOwners = []SharedOwnership{{Subject: "enrollment.submit", Owners: []string{"course.boundary", "course.boundary"}, Reason: "reference sharing", SourceRef: "reference-fixture"}}
	if err := validateGraph(graph); err == nil {
		t.Fatal("duplicate boundary names fabricated a shared ownership declaration")
	}
}

func TestModelV2StoredContractMustMatchCoreLedger(t *testing.T) {
	graph := referenceModel(t)
	d := modelTestDB(t, graph)
	if err := d.tx([]string{"UPDATE rules SET oracle='changed-observer' WHERE graph=" + quote(graph.ID)}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.model(graph.ID); err == nil {
		t.Fatal("model readback concealed a rule changed outside the immutable model")
	}
}

func TestModelV2NextUsesDeclaredContract(t *testing.T) {
	graph := referenceModel(t)
	d := modelTestDB(t, graph)
	if _, err := d.plan(graph.ID, 128); err != nil {
		t.Fatal(err)
	}
	cards, err := d.next(graph.ID)
	if err != nil {
		t.Fatalf("model tasks cannot reach the task-card CLI: %v", err)
	}
	found := false
	for _, card := range cards {
		if card.ID == graph.ID+"/test.idempotency" {
			found = card.Type == "IDEMPOTENCY" && card.Expected == "ready" && card.Preconditions["login"] == "SATISFIED"
		}
	}
	if !found {
		t.Fatal("declared test lost its kind, expectation or observed preconditions")
	}
}

func TestModelV2GateCountsAllDeclaredObligations(t *testing.T) {
	graph := referenceModel(t)
	d := modelTestDB(t, graph)
	if _, err := d.plan(graph.ID, 128); err != nil {
		t.Fatal(err)
	}
	gate, err := d.gate(graph.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gate["required_tasks"] != len(modelObligations(graph)) {
		t.Fatalf("gate shrank full model obligations to %v legacy tasks", gate["required_tasks"])
	}
}

func TestModelV2ORGuardHasARealRejectionObligation(t *testing.T) {
	graph := referenceModel(t)
	graph.Nodes = append(graph.Nodes, Node{ID: "guest", Kind: "condition", Name: "Reference guest exception"})
	graph.Edges = append(graph.Edges, Edge{ID: "requires-guest", From: "guest", To: "enroll", Kind: "requires"})
	graph.Rules[0].Conditions = []string{"login", "guest"}
	graph.Guards = []RuleGuard{{Rule: "R1", AnyOf: [][]ConditionLiteral{{{Condition: "login", Value: true}}, {{Condition: "guest", Value: true}}}}}
	graph.Transitions[0].Preconditions["guest"] = false
	graph.Tests[0].Preconditions["guest"] = false
	if err := validateGraph(graph); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, obligation := range modelRuleObligations(graph, "R1") {
		definition := obligation.Definition
		if definition.Denied && !definition.Preconditions["login"] && !definition.Preconditions["guest"] {
			found = true
		}
	}
	if !found {
		t.Fatal("one-false legacy combinations omitted the declared OR guard rejection")
	}
}

func TestModelV2UnknownConditionIsNotFalse(t *testing.T) {
	body := strings.Replace(businessModelExample, `"preconditions":{"login":true}`, `"preconditions":{"login":null}`, 1)
	if _, err := loadGraph(writeBusinessModel(t, body)); err == nil {
		t.Fatal("unknown condition was coerced to a false observation")
	}
	for _, literal := range []string{`{"condition":"login"}`, `{"condition":"login","value":null}`} {
		body := strings.TrimSuffix(businessModelExample, "}") + `,"constraints":[{"id":"declared","literals":[` + literal + `],"source":"document","source_ref":"reference-fixture"}]}`
		if _, err := loadGraph(writeBusinessModel(t, body)); err == nil {
			t.Errorf("missing or null literal polarity was accepted: %s", literal)
		}
	}
}

func TestModelV2OldClientGuardExistsBeforePlanning(t *testing.T) {
	graph := referenceModel(t)
	d := modelTestDB(t, graph)
	rows, err := d.query("SELECT kind,status FROM tasks WHERE id="+quote(graph.ID+"/_schema/v2"), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][0] != "MODEL_SCHEMA_GUARD" || rows[0][1] != "BLOCKED" {
		t.Fatalf("old CLI can plan and pass only legacy obligations: %v", rows)
	}
}

// Golden values come from the pre-v2 source rebuilt from the
// business-graph-workbench gate baseline (2026-10-30T22:51); see
// .arc/artifacts/verify-model-v2/reports/v1-fingerprint-golden.md.
func TestModelV1FingerprintMatchesPreV2Golden(t *testing.T) {
	graph, err := loadGraph("examples/academic-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	d, err := openDB(filepath.Join(t.TempDir(), "data.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.close() })
	if err = d.tx(schema); err != nil {
		t.Fatal(err)
	}
	expect := func(state, want string) {
		t.Helper()
		got, err := d.graphFingerprint(graph.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("%s v1 fingerprint changed: got %s want %s", state, got, want)
		}
	}
	if err = d.importGraph(graph); err != nil {
		t.Fatal(err)
	}
	expect("imported", "39a54fe79735667e694907125b6d3a977391caf6031ec01dc0cf61b67f5db45d")
	if err = d.approve(graph.ID, ""); err != nil {
		t.Fatal(err)
	}
	for _, rule := range graph.Rules {
		if err = d.approve(graph.ID, rule.ID); err != nil {
			t.Fatal(err)
		}
	}
	expect("approved", "96db71de12d67d70760dbb88ec6ffa16a2f5bf72cbcba69ea867fd8cf8838443")
	if _, err = d.plan(graph.ID); err != nil {
		t.Fatal(err)
	}
	expect("planned", "96db71de12d67d70760dbb88ec6ffa16a2f5bf72cbcba69ea867fd8cf8838443")
}

func TestModelV2ImportReadBackThroughCLI(t *testing.T) {
	path := writeBusinessModel(t, businessModelExample)
	dbfile := filepath.Join(t.TempDir(), "data.db")
	for _, args := range [][]string{{"init", "--db", dbfile}, {"import", "--db", dbfile, "--file", path}} {
		if err := run(args); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	d, err := openDB(dbfile, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.close() })
	stored, err := d.model("course-v2")
	if err != nil {
		t.Fatal(err)
	}
	want, err := loadGraph(path)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(stored)
	expected, _ := json.Marshal(want)
	if string(got) != string(expected) {
		t.Fatalf("stored model differs from imported model\n got %s\nwant %s", got, expected)
	}
}

func TestModelV2RejectsIllegalStructure(t *testing.T) {
	type mutation func(t *testing.T, model map[string]any) string
	appendItems := func(key string, items ...map[string]any) mutation {
		return func(t *testing.T, model map[string]any) string {
			list, _ := model[key].([]any)
			for _, item := range items {
				list = append(list, item)
			}
			model[key] = list
			return ""
		}
	}
	node := func(id, kind string) map[string]any { return map[string]any{"id": id, "kind": kind, "name": id} }
	edge := func(id, from, to, kind string) map[string]any {
		return map[string]any{"id": id, "from": from, "to": to, "kind": kind}
	}
	test := func(id string, dependsOn ...string) map[string]any {
		return map[string]any{"id": id, "name": id, "rule": "R1", "kind": "IDEMPOTENCY", "preconditions": map[string]any{"login": true},
			"expected": "ready", "depends_on": dependsOn, "source": "document", "source_ref": "reference-fixture"}
	}
	cases := map[string]struct {
		mutate mutation
		reason string
	}{
		"unknown endpoint": {appendItems("edges", edge("bad-endpoint", "enrollment", "missing.node", "contains")), "unknown endpoint"},
		"capability with two parents": {func(t *testing.T, model map[string]any) string {
			appendItems("nodes", node("enrollment.other", "domain"))(t, model)
			return appendItems("edges", edge("contains-other", "course", "enrollment.other", "contains"),
				edge("contains-twice", "enrollment.other", "enrollment.submit", "contains"))(t, model)
		}, "more than one parent"},
		"contains cycle detached from system": {func(t *testing.T, model map[string]any) string {
			appendItems("nodes", node("loop.a", "domain"), node("loop.b", "domain"))(t, model)
			return appendItems("edges", edge("loop-ab", "loop.a", "loop.b", "contains"), edge("loop-ba", "loop.b", "loop.a", "contains"))(t, model)
		}, "cyclic"},
		"reversed implements endpoints": {appendItems("edges", edge("bad-implements", "enroll", "enrollment.submit", "implements")), "illegal action -> capability"},
		"test dependency cycle":         {appendItems("tests", test("test.a", "test.b"), test("test.b", "test.a")), "test dependency cycle"},
		"duplicate node id":             {appendItems("nodes", node("enroll", "action")), `duplicated identity "enroll"`},
		"unknown nested field": {func(t *testing.T, model map[string]any) string {
			model["nodes"].([]any)[0].(map[string]any)["colour"] = "red"
			return ""
		}, `unknown field "colour"`},
		"duplicate json key": {func(t *testing.T, model map[string]any) string {
			return strings.Replace(businessModelExample, `"id": "course-v2",`, `"id": "course-v2", "id": "course-v3",`, 1)
		}, `duplicate JSON field "id"`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var model map[string]any
			if err := json.Unmarshal([]byte(businessModelExample), &model); err != nil {
				t.Fatal(err)
			}
			raw := tc.mutate(t, model)
			if raw == "" {
				body, err := json.Marshal(model)
				if err != nil {
					t.Fatal(err)
				}
				raw = string(body)
			}
			path := writeBusinessModel(t, raw)
			err := run([]string{"validate", "--file", path})
			if err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("validate must reject with %q, got %v", tc.reason, err)
			}
			dbfile := filepath.Join(t.TempDir(), "data.db")
			if err := run([]string{"init", "--db", dbfile}); err != nil {
				t.Fatal(err)
			}
			if err := run([]string{"import", "--db", dbfile, "--file", path}); err == nil || !strings.Contains(err.Error(), tc.reason) {
				t.Fatalf("import must reject with %q, got %v", tc.reason, err)
			}
			d, err := openDB(dbfile, false)
			if err != nil {
				t.Fatal(err)
			}
			defer d.close()
			for _, table := range []string{"graphs", "nodes", "edges", "rules", "business_models"} {
				rows, err := d.query("SELECT "+fieldsSQL("CAST(COUNT(*) AS TEXT)")+" FROM "+table+";", 1)
				if err != nil {
					t.Fatal(err)
				}
				if rows[0][0] != "0" {
					t.Fatalf("rejected import left %s rows in %s", rows[0][0], table)
				}
			}
		})
	}
}
