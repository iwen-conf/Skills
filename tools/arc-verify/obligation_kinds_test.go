package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const obligationKindsModel = `{
  "schema_version": 2,
  "id": "course-v2",
  "system": "course",
  "scope": "Synthetic tool fixture for EXISTENCE, SEQUENCE and CONSISTENCY; not a product contract",
  "nodes": [
    {"id":"course","kind":"system","name":"课程系统"},
    {"id":"enrollment","kind":"domain","name":"报名业务"},
    {"id":"enrollment.submit","kind":"capability","name":"提交并确认报名"},
    {"id":"course.boundary","kind":"boundary","name":"课程边界"},
    {"id":"STUDENT","kind":"role","name":"学员"},
    {"id":"enroll","kind":"action","name":"创建报名记录"},
    {"id":"confirm","kind":"action","name":"确认报名"},
    {"id":"login","kind":"condition","name":"已认证"},
    {"id":"pending","kind":"state","name":"未报名"},
    {"id":"ready","kind":"state","name":"报名成功"},
    {"id":"confirmed","kind":"state","name":"已确认"}
  ],
  "edges": [
    {"id":"contains-domain","from":"course","to":"enrollment","kind":"contains"},
    {"id":"contains-capability","from":"enrollment","to":"enrollment.submit","kind":"contains"},
    {"id":"implements-enroll","from":"enrollment.submit","to":"enroll","kind":"implements"},
    {"id":"implements-confirm","from":"enrollment.submit","to":"confirm","kind":"implements"},
    {"id":"owns-submit","from":"course.boundary","to":"enrollment.submit","kind":"owns"},
    {"id":"serves-student","from":"STUDENT","to":"enrollment","kind":"serves"},
    {"id":"requires-login","from":"login","to":"enroll","kind":"requires"},
    {"id":"requires-login-confirm","from":"login","to":"confirm","kind":"requires"},
    {"id":"creates-ready","from":"enroll","to":"ready","kind":"transitions"},
    {"id":"creates-confirmed","from":"confirm","to":"confirmed","kind":"transitions"},
    {"id":"submit-flow","from":"pending","to":"ready","kind":"transitions_to"},
    {"id":"confirm-flow","from":"ready","to":"confirmed","kind":"transitions_to"}
  ],
  "rules": [
    {"id":"R1","action":"enroll","actor":"STUDENT","entry":"/enroll","oracle":"fixture-independent-store","conditions":["login"],"expected":"ready","rejected":"UNCHANGED","source":"document","description":"Authenticated enrollment"},
    {"id":"R2","action":"confirm","actor":"STUDENT","entry":"/confirm","oracle":"confirm-store","conditions":["login"],"expected":"confirmed","rejected":"UNCHANGED","source":"document","description":"Authenticated confirmation"}
  ],
  "transitions": [
    {"edge":"submit-flow","action":"enroll","rule":"R1","preconditions":{"login":true},"outcome":"success"},
    {"edge":"confirm-flow","action":"confirm","rule":"R2","preconditions":{"login":true},"outcome":"success"}
  ],
  "flows": [{"id":"flow.enrollment","capability":"enrollment.submit","initial":"pending"}],
  "tests": [
    {"id":"test.exists.enroll","name":"报名入口必须存在","rule":"R1","kind":"EXISTENCE","source":"document","source_ref":"fixture:must_exist"},
    {"id":"test.sequence.enroll-confirm","name":"报名后确认","rule":"R2","kind":"SEQUENCE","flow":"flow.enrollment","steps":["submit-flow","confirm-flow"],"expected":"confirmed","source":"document","source_ref":"fixture:sequence"},
    {"id":"test.consistency.enroll","name":"报名结果各层一致","rule":"R1","kind":"CONSISTENCY","preconditions":{"login":true},"expected":"ready","layers":[{"layer":"api","oracle":"fixture-independent-store"},{"layer":"database","oracle":"db-store"}],"source":"document","source_ref":"fixture:consistency"}
  ],
  "risks": []
}`

func kindsModel(t *testing.T) Graph {
	t.Helper()
	graph, err := loadGraph(writeBusinessModel(t, obligationKindsModel))
	if err != nil {
		t.Fatal(err)
	}
	return graph
}

// kindsServer is a synthetic business target; each switch reproduces one defect class.
type kindsServer struct {
	mu                          sync.Mutex
	state                       string
	enrollBroken, confirmBroken bool
	databaseLags, databaseDown  bool
	unknownState                bool
	confirmCalls                int
}

func newKindsServer(t *testing.T) (*kindsServer, *httptest.Server) {
	t.Helper()
	target := &kindsServer{state: "pending"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target.mu.Lock()
		defer target.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		facts := func(state string) {
			enrolled := 0
			if state != "pending" {
				enrolled = 1
			}
			fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d}`, state, enrolled)
		}
		switch r.URL.Path {
		case "/enroll":
			if !target.enrollBroken && target.state == "pending" {
				target.state = "ready"
				if target.unknownState {
					target.state = "mystery"
				}
			}
			fmt.Fprint(w, `{}`)
		case "/confirm":
			target.confirmCalls++
			if !target.confirmBroken && target.state == "ready" {
				target.state = "confirmed"
			}
			fmt.Fprint(w, `{}`)
		case "/reset":
			target.state = "pending"
			fmt.Fprint(w, `{}`)
		case "/oracle", "/confirm-oracle":
			facts(target.state)
		case "/db-oracle":
			if target.databaseDown && target.state != "pending" {
				http.Error(w, "database replica unavailable", http.StatusServiceUnavailable)
				return
			}
			if target.databaseLags {
				facts("pending")
				return
			}
			facts(target.state)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return target, server
}

func kindsOracle(id, path string) string {
	return fmt.Sprintf(`{"id":%q,"boundary":"after committed operation","call":{"kind":"http","method":"GET","path":%q,"success_statuses":[200]},"state_path":"/state","principal_path":"/principal","role_path":"/role","scope_path":"/scope","conditions":{"login":"/login"},"measurements":{"enrolled":"/enrolled"}}`, id, path)
}

func kindsStep(node, transition, path string, oracles ...string) string {
	return fmt.Sprintf(`{"node":%q,"transition":%q,"actor":"primary","call":{"kind":"http","method":"POST","path":%q,"success_statuses":[200]},"oracles":[%s]}`, node, transition, path, strings.Join(oracles, ","))
}

func kindsManifest(t *testing.T, id, test string, steps ...string) string {
	t.Helper()
	body := fmt.Sprintf(`{"schema_version":1,"id":%q,"graph":"course-v2","test":%q,"isolation_scope":"course-fixture","timeout_ms":1200,"step_timeout_ms":200,"parallelism":1,"steps":[%s],
"recovery":{"isolation_scope":"course-fixture","steps":[{"actor":"primary","call":{"kind":"http","method":"POST","path":"/reset","success_statuses":[200]}}],"oracle":%s,"state":"pending","conditions":{"login":true},"checks":[{"left":"enrolled","operator":"eq","value":0}]}}`, id, test, strings.Join(steps, ","), kindsOracle("fixture-independent-store", "/oracle"))
	path := filepath.Join(t.TempDir(), id+".json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func (fixture *executionFixture) openRound(t *testing.T) {
	t.Helper()
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(cliJSON(t, "round", "--db", fixture.d.file, "--graph", "course-v2", "--env", "independent", "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit), &result); err != nil || result.ID == "" {
		t.Fatalf("round not opened: %v %+v", err, result)
	}
	fixture.round = result.ID
}

func obligationStatus(t *testing.T, d db, round, id string) ObligationStatus {
	t.Helper()
	analysis, err := d.analyzeModel(kindsModel(t), "independent", round)
	if err != nil {
		t.Fatal(err)
	}
	for _, obligation := range analysis.Obligations {
		if obligation.Definition.ID == id {
			return obligation
		}
	}
	t.Fatalf("obligation %s is missing from the complete denominator", id)
	return ObligationStatus{}
}

func writeScanEvidence(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scan-evidence.txt")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExistenceObligationNeedsScanEvidence(t *testing.T) {
	fixture := prepareExecutionFixture(t, kindsModel(t), "http://127.0.0.1:9")
	fixture.openRound(t)
	if status := obligationStatus(t, fixture.d, fixture.round, "test.exists.enroll"); status.Status != "NOT_RUN" || !strings.Contains(status.Reason, "implementation scan") {
		t.Fatalf("EXISTENCE without a scan must stay NOT_RUN: %+v", status)
	}
	evidence := writeScanEvidence(t, "tools/arc-verify/fixture.go:12 POST /enroll handler")
	scan := func(commit string) {
		t.Helper()
		if _, err := fixture.d.scan(Implementation{Graph: "course-v2", Commit: commit, Entrances: []Entrance{{Action: "enroll", Status: "PRESENT", Evidence: evidence}}}); err != nil {
			t.Fatal(err)
		}
	}
	scan("")
	if status := obligationStatus(t, fixture.d, fixture.round, "test.exists.enroll"); status.Status != "BLOCKED" || !strings.Contains(status.Reason, "candidate commit") {
		t.Fatalf("an unbound scan must not prove the current candidate: %+v", status)
	}
	scan(strings.Repeat("a", 40))
	if status := obligationStatus(t, fixture.d, fixture.round, "test.exists.enroll"); status.Status != "STALE" {
		t.Fatalf("a scan of another candidate must be STALE: %+v", status)
	}
	scan(fixture.commit)
	if status := obligationStatus(t, fixture.d, fixture.round, "test.exists.enroll"); status.Status != "PASS" {
		t.Fatalf("bound PRESENT evidence must satisfy EXISTENCE: %+v", status)
	}
	analysis, err := fixture.d.analyzeModel(kindsModel(t), "independent", fixture.round)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"enroll", "R1"} {
		readiness := analysis.Nodes[id].Readiness
		if !readiness.Implemented || readiness.ImplementationStatus != "PRESENT" {
			t.Fatalf("%s readiness ignores the bound implementation scan: %+v", id, readiness)
		}
	}
	if readiness := analysis.Nodes["confirm"].Readiness; readiness.Implemented || readiness.ImplementationStatus != "UNOBSERVED" {
		t.Fatalf("unscanned action must stay UNOBSERVED: %+v", readiness)
	}
	if readiness := analysis.Nodes["enrollment.submit"].Readiness; readiness.Implemented || readiness.ImplementationStatus != "UNOBSERVED" {
		t.Fatalf("capability is implemented only when every action is: %+v", readiness)
	}
	if err := os.WriteFile(evidence, []byte("handler removed"), 0600); err != nil {
		t.Fatal(err)
	}
	if status := obligationStatus(t, fixture.d, fixture.round, "test.exists.enroll"); status.Status != "STALE" || !strings.Contains(status.Reason, "evidence") {
		t.Fatalf("changed scan evidence must be STALE: %+v", status)
	}
}

func TestExistenceNotFoundStaysCandidate(t *testing.T) {
	fixture := prepareExecutionFixture(t, kindsModel(t), "http://127.0.0.1:9")
	fixture.openRound(t)
	evidence := writeScanEvidence(t, "arc-idx search 'f:src/ /enroll' returned no handler")
	if _, err := fixture.d.scan(Implementation{Graph: "course-v2", Commit: fixture.commit, Entrances: []Entrance{{Action: "enroll", Status: "NOT_FOUND", Evidence: evidence}}}); err != nil {
		t.Fatal(err)
	}
	status := obligationStatus(t, fixture.d, fixture.round, "test.exists.enroll")
	if status.Status != "BLOCKED" || !strings.Contains(status.Reason, "candidate gap") {
		t.Fatalf("NOT_FOUND must be a candidate gap, never FAIL: %+v", status)
	}
	analysis, err := fixture.d.analyzeModel(kindsModel(t), "independent", fixture.round)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Verdict == "FAIL" {
		t.Fatalf("an unconfirmed missing entry cannot fail the graph: %+v", analysis.Verdict)
	}
	report, err := fixture.d.acceptanceReport("course-v2")
	if err != nil {
		t.Fatal(err)
	}
	if gaps := fmt.Sprint(report["candidate_gaps"]); !strings.Contains(gaps, "NOT_FOUND") {
		t.Fatalf("acceptance report lost the candidate gap: %s", gaps)
	}
	err = run([]string{"runner", "--db", fixture.d.file, "--file", kindsManifest(t, "exists-runner", "test.exists.enroll", kindsStep("enroll", "", "/enroll", kindsOracle("fixture-independent-store", "/oracle")))})
	if err == nil || !strings.Contains(err.Error(), "implementation scan") {
		t.Fatalf("EXISTENCE must not bind a runner: %v", err)
	}
}

func sequenceManifest(t *testing.T, id string) string {
	return kindsManifest(t, id, "test.sequence.enroll-confirm",
		kindsStep("enroll", "submit-flow", "/enroll", kindsOracle("fixture-independent-store", "/oracle")),
		kindsStep("confirm", "confirm-flow", "/confirm", kindsOracle("confirm-store", "/confirm-oracle")))
}

func executeKinds(t *testing.T, fixture executionFixture, runner, test string) ExecuteResult {
	t.Helper()
	var result ExecuteResult
	if err := json.Unmarshal(fixture.execute(t, runner, test), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestSequenceObligationKeepsFirstDivergence(t *testing.T) {
	for _, scenario := range []struct {
		name       string
		breakStep  func(*kindsServer)
		divergence string
		steps      int
		confirms   int
	}{
		{"confirm leaves state unchanged", func(s *kindsServer) { s.confirmBroken = true }, "2", 2, 1},
		{"enroll leaves state unchanged", func(s *kindsServer) { s.enrollBroken = true }, "1", 1, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			target, server := newKindsServer(t)
			scenario.breakStep(target)
			fixture := prepareExecutionFixture(t, kindsModel(t), server.URL)
			fixture.registerAndRound(t, sequenceManifest(t, "sequence-runner"))
			result := executeKinds(t, fixture, "sequence-runner", "test.sequence.enroll-confirm")
			if result.Verdict != "FAIL" || result.Recovery.Verdict != "PASS" {
				t.Fatalf("sequence divergence was not preserved as FAIL: %+v", result)
			}
			trace, err := fixture.d.trace(result.Run)
			if err != nil {
				t.Fatal(err)
			}
			if trace.FirstDivergence != scenario.divergence || len(trace.Steps) != scenario.steps {
				t.Fatalf("first divergence or executed steps are wrong: %+v", trace)
			}
			if failed := trace.Steps[len(trace.Steps)-1]; failed.Verdict != "FAIL" || failed.Transition != map[string]string{"1": "submit-flow", "2": "confirm-flow"}[scenario.divergence] {
				t.Fatalf("divergent step is not bound to its declared transition: %+v", failed)
			}
			target.mu.Lock()
			confirms := target.confirmCalls
			target.mu.Unlock()
			if confirms != scenario.confirms {
				t.Fatalf("steps after the first divergence ran: %d confirm calls", confirms)
			}
			if status := obligationStatus(t, fixture.d, fixture.round, "test.sequence.enroll-confirm"); status.Status != "FAIL" {
				t.Fatalf("SEQUENCE obligation lost its failure: %+v", status)
			}
		})
	}
}

func TestSequenceObligationPassesStepByStep(t *testing.T) {
	_, server := newKindsServer(t)
	fixture := prepareExecutionFixture(t, kindsModel(t), server.URL)
	fixture.registerAndRound(t, sequenceManifest(t, "sequence-runner"))
	result := executeKinds(t, fixture, "sequence-runner", "test.sequence.enroll-confirm")
	if result.Verdict != "PASS" || result.Steps != 2 {
		t.Fatalf("each step must be judged by its own transition rule and oracle: %+v", result)
	}
	if status := obligationStatus(t, fixture.d, fixture.round, "test.sequence.enroll-confirm"); status.Status != "PASS" {
		t.Fatalf("qualified SEQUENCE run must pass: %+v", status)
	}
	reversed := kindsManifest(t, "sequence-reversed", "test.sequence.enroll-confirm",
		kindsStep("confirm", "confirm-flow", "/confirm", kindsOracle("confirm-store", "/confirm-oracle")),
		kindsStep("enroll", "submit-flow", "/enroll", kindsOracle("fixture-independent-store", "/oracle")))
	if err := run([]string{"runner", "--db", fixture.d.file, "--file", reversed}); err == nil || !strings.Contains(err.Error(), "SEQUENCE") {
		t.Fatalf("runner must follow the declared step order: %v", err)
	}
	partial := kindsManifest(t, "sequence-partial", "test.sequence.enroll-confirm",
		kindsStep("confirm", "confirm-flow", "/confirm", kindsOracle("confirm-store", "/confirm-oracle")))
	if err := run([]string{"runner", "--db", fixture.d.file, "--file", partial}); err == nil || !strings.Contains(err.Error(), "SEQUENCE") {
		t.Fatalf("runner must execute every declared step: %v", err)
	}
}

func consistencyManifest(t *testing.T, id string, oracles ...string) string {
	return kindsManifest(t, id, "test.consistency.enroll", kindsStep("enroll", "", "/enroll", oracles...))
}

func TestConsistencyMissingLayerBlocked(t *testing.T) {
	target, server := newKindsServer(t)
	target.databaseDown = true
	fixture := prepareExecutionFixture(t, kindsModel(t), server.URL)
	apiOnly := consistencyManifest(t, "consistency-api-only", kindsOracle("fixture-independent-store", "/oracle"))
	if err := run([]string{"runner", "--db", fixture.d.file, "--file", apiOnly}); err == nil || !strings.Contains(err.Error(), "layer") {
		t.Fatalf("runner without every declared layer must be rejected: %v", err)
	}
	fixture.registerAndRound(t, consistencyManifest(t, "consistency-runner", kindsOracle("fixture-independent-store", "/oracle"), kindsOracle("db-store", "/db-oracle")))
	result := executeKinds(t, fixture, "consistency-runner", "test.consistency.enroll")
	if result.Verdict != "BLOCKED" {
		t.Fatalf("an unobserved layer must block, not pass: %+v", result)
	}
	if status := obligationStatus(t, fixture.d, fixture.round, "test.consistency.enroll"); status.Status != "BLOCKED" {
		t.Fatalf("CONSISTENCY with a missing layer must be BLOCKED: %+v", status)
	}
}

func TestConsistencyLayersMustAgree(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		lags    bool
		verdict string
	}{
		{"layers agree", false, "PASS"},
		{"database layer disagrees", true, "FAIL"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			target, server := newKindsServer(t)
			target.databaseLags = scenario.lags
			fixture := prepareExecutionFixture(t, kindsModel(t), server.URL)
			fixture.registerAndRound(t, consistencyManifest(t, "consistency-runner", kindsOracle("fixture-independent-store", "/oracle"), kindsOracle("db-store", "/db-oracle")))
			result := executeKinds(t, fixture, "consistency-runner", "test.consistency.enroll")
			if result.Verdict != scenario.verdict {
				t.Fatalf("layer comparison verdict: %+v", result)
			}
			if status := obligationStatus(t, fixture.d, fixture.round, "test.consistency.enroll"); status.Status != scenario.verdict {
				t.Fatalf("CONSISTENCY obligation status: %+v", status)
			}
		})
	}
}

func TestNewKindsCountedInDenominator(t *testing.T) {
	full := kindsModel(t)
	base := full
	base.Tests = nil
	measure := func(g Graph) (ModelAnalysis, map[string]any, db) {
		t.Helper()
		d := modelTestDB(t, g)
		if _, err := d.plan(g.ID, 128); err != nil {
			t.Fatal(err)
		}
		analysis, err := d.analyzeModel(g, "", "")
		if err != nil {
			t.Fatal(err)
		}
		gate, err := d.modelGate(g)
		if err != nil {
			t.Fatal(err)
		}
		return analysis, gate, d
	}
	withKinds, gate, d := measure(full)
	without, _, _ := measure(base)
	if withKinds.Coverage.Obligations.Total != without.Coverage.Obligations.Total+3 {
		t.Fatalf("new obligation kinds are not in the denominator: %d vs %d", withKinds.Coverage.Obligations.Total, without.Coverage.Obligations.Total)
	}
	if gate["required_tasks"] != withKinds.Coverage.Obligations.Total || gate["missing_tasks"] != 0 {
		t.Fatalf("gate does not count every planned obligation: %+v", gate)
	}
	kinds := map[TestKind]bool{}
	for _, obligation := range withKinds.Obligations {
		if !obligation.Planned {
			t.Fatalf("obligation %s was not planned", obligation.Definition.ID)
		}
		kinds[obligation.Definition.Kind] = true
	}
	for _, kind := range []TestKind{testExistence, testSequence, testConsistency} {
		if !kinds[kind] {
			t.Fatalf("%s obligation missing", kind)
		}
	}
	for _, obligation := range withKinds.Obligations {
		if obligation.Definition.Kind != testSequence {
			continue
		}
		covered := strings.Join(obligation.Targets, ",")
		for _, id := range []string{"submit-flow", "confirm-flow", "R1", "R2", "flow.enrollment"} {
			if !strings.Contains(","+covered+",", ","+id+",") {
				t.Fatalf("SEQUENCE obligation does not cover %s: %s", id, covered)
			}
		}
	}
	for _, rule := range []string{"R1", "R2"} {
		if withKinds.Nodes[rule].Required <= without.Nodes[rule].Required {
			t.Fatalf("new obligations are not counted on rule %s", rule)
		}
	}
	cards, err := d.nextModel(full)
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range cards {
		if card.Type == string(testExistence) && !strings.Contains(card.Instruction, "scan") {
			t.Fatalf("EXISTENCE card must ask for implementation scan evidence: %+v", card)
		}
		if card.Type == string(testSequence) && strings.Join(card.Steps, ",") != "submit-flow,confirm-flow" {
			t.Fatalf("SEQUENCE card lost its ordered steps: %+v", card)
		}
		if card.Type == string(testConsistency) && len(card.Layers) != 2 {
			t.Fatalf("CONSISTENCY card lost its layers: %+v", card)
		}
	}
}

func TestV1HashUnchangedWithNewKinds(t *testing.T) {
	ids := []string{}
	for _, obligation := range modelObligations(referenceModel(t)) {
		ids = append(ids, obligation.Task.ID)
	}
	body, err := json.Marshal(modelObligations(referenceModel(t)))
	if err != nil {
		t.Fatal(err)
	}
	// Captured before W07 from the unchanged reference model; rounds store this digest.
	if digest := digestBytes(body); digest != "58d5a781fec504e2bd24100d6a13530ce9011b34f6e964020b2fbd1e1288b98a" {
		t.Fatalf("existing v2 obligations changed shape: %s %q", digest, ids)
	}
	legacy := `{"id":"legacy-v1","scope":"legacy","nodes":[],"edges":[],"rules":[],"risks":[],"tests":[{"id":"t","name":"exists","rule":"R1","kind":"EXISTENCE","source":"document","source_ref":"x"}]}`
	if _, err := loadGraph(writeBusinessModel(t, legacy)); err == nil || !strings.Contains(err.Error(), "schema_version 2") {
		t.Fatalf("v1 graphs must not accept new obligation kinds: %v", err)
	}
}

func TestNewKindsRejectIllegalDeclarations(t *testing.T) {
	for _, scenario := range []struct {
		name, reason string
		change       func(*Graph)
	}{
		{"existence with preconditions", "EXISTENCE", func(g *Graph) { g.Tests[0].Preconditions = ConditionAssignment{"login": true} }},
		{"existence with expected state", "EXISTENCE", func(g *Graph) { g.Tests[0].Expected = "ready" }},
		{"existence declared twice", "EXISTENCE", func(g *Graph) {
			copy := g.Tests[0]
			copy.ID = "test.exists.again"
			g.Tests = append(g.Tests, copy)
		}},
		{"sequence with one step", "SEQUENCE", func(g *Graph) { g.Tests[1].Steps = []string{"submit-flow"} }},
		{"sequence out of order", "SEQUENCE", func(g *Graph) { g.Tests[1].Steps = []string{"confirm-flow", "submit-flow"} }},
		{"sequence unknown flow", "SEQUENCE", func(g *Graph) { g.Tests[1].Flow = "flow.missing" }},
		{"sequence rule is not the final step", "SEQUENCE", func(g *Graph) { g.Tests[1].Rule = "R1" }},
		{"sequence expected differs from final step", "SEQUENCE", func(g *Graph) { g.Tests[1].Expected = "ready" }},
		{"sequence with preconditions", "SEQUENCE", func(g *Graph) { g.Tests[1].Preconditions = ConditionAssignment{"login": true} }},
		{"steps on another kind", "SEQUENCE", func(g *Graph) { g.Tests[2].Steps = []string{"submit-flow", "confirm-flow"} }},
		{"consistency with one layer", "CONSISTENCY", func(g *Graph) { g.Tests[2].Layers = g.Tests[2].Layers[:1] }},
		{"consistency duplicate layer", "CONSISTENCY", func(g *Graph) { g.Tests[2].Layers[1].Layer = "api" }},
		{"consistency unknown layer", "CONSISTENCY", func(g *Graph) { g.Tests[2].Layers[1].Layer = "cache" }},
		{"consistency duplicate oracle", "CONSISTENCY", func(g *Graph) { g.Tests[2].Layers[1].Oracle = "fixture-independent-store" }},
		{"layers on another kind", "CONSISTENCY", func(g *Graph) {
			g.Tests[0].Layers = []ObservationLayer{{Layer: "api", Oracle: "a"}, {Layer: "database", Oracle: "b"}}
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			g := kindsModel(t)
			g.Tests = append([]TestDefinition{}, g.Tests...)
			g.Tests[2].Layers = append([]ObservationLayer{}, g.Tests[2].Layers...)
			scenario.change(&g)
			if err := validateGraph(g); err == nil || !strings.Contains(err.Error(), scenario.reason) {
				t.Fatalf("illegal declaration accepted or misreported: %v", err)
			}
		})
	}
}

func TestSequenceNeedsEveryStepRuleApproved(t *testing.T) {
	g := kindsModel(t)
	d, err := openDB(filepath.Join(t.TempDir(), "data.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.close() })
	for _, operation := range []func() error{
		func() error { return d.tx(schema) },
		func() error { return d.importGraph(g) },
		func() error { return d.approve(g.ID, "") },
		func() error { return d.approve(g.ID, "R2") },
	} {
		if err := operation(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := d.plan(g.ID, 128); err != nil {
		t.Fatal(err)
	}
	analysis, err := d.analyzeModel(g, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, obligation := range analysis.Obligations {
		if obligation.Definition.Kind == testSequence && (obligation.Planned || obligation.Status != "BLOCKED" || !strings.Contains(obligation.Reason, "approval")) {
			t.Fatalf("SEQUENCE over an unapproved step rule must stay blocked and unplanned: %+v", obligation)
		}
	}
}
