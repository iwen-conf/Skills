package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRuntimeFactsPreserveAllConditionSubjects(t *testing.T) {
	g := Graph{ConditionSubjects: []ConditionSubject{
		{Condition: "owned", SubjectKind: "actor", Subject: "buyer", Attribute: "principal", Operator: "eq", Expected: "order.owner"},
		{Condition: "owned", SubjectKind: "entity", Subject: "order", Attribute: "owner", Operator: "eq", Expected: "buyer.principal"},
	}}
	got := buildConditionReadings(g, "buyer", "store", OracleAdapter{Conditions: map[string]string{"owned": "/owned"}}, map[string]bool{"owned": true}, map[string]bool{"owned": true}, []string{"owned"})
	if len(got) != 2 || got[0].Subject != "buyer" || got[1].Subject != "order" {
		t.Fatalf("condition subjects were collapsed: %+v", got)
	}
	body, err := json.Marshal(got)
	if err != nil || !strings.Contains(string(body), `"operator":"eq"`) || !strings.Contains(string(body), `"expected":"order.owner"`) {
		t.Fatalf("condition annotation lost declared comparison: %s %v", body, err)
	}
}

// Both actors deliberately use the same oracle ID and the same state name.
// Only their independent property readings and entity identities differ.
func runParallelRuntimeFacts(t *testing.T, waitForContender bool) (executionFixture, ExecuteResult) {
	t.Helper()
	graph := referenceModel(t)
	graph.Tests[0].Kind = testConcurrency
	graph.Tests[0].Actors = []ActorRequirement{{Alias: "contender", Role: "STUDENT"}}
	var mu sync.Mutex
	enrolled := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/enroll":
			enrolled++
			fmt.Fprint(w, `{}`)
		case "/reset":
			enrolled = 0
			fmt.Fprint(w, `{}`)
		default:
			principal, order, settled := "synthetic-STUDENT", "order-primary", true
			if r.Header.Get("X-Fixture-Actor") == "contender" {
				principal, order, settled = "synthetic-CONTENDER", "order-contender", false
			}
			state := "pending"
			if enrolled > 0 {
				state = "ready"
			}
			fmt.Fprintf(w, `{"state":%q,"principal":%q,"role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d,"capacity":2,"order_id":%q,"settled":%t}`, state, principal, enrolled, order, settled)
		}
	}))
	t.Cleanup(server.Close)
	fixture := prepareExecutionFixture(t, graph, server.URL)
	t.Setenv("ARC_TEST_CONTENDER", "contender")
	if _, err := fixture.d.importFixtures(Fixtures{Fixtures: []Fixture{{ID: "fixture.contender", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_TEST_CONTENDER", Refs: map[string]string{"role": "STUDENT", "principal": "synthetic-CONTENDER", "environment": "independent"}}}}); err != nil {
		t.Fatal(err)
	}
	const runnerID = "parallel-runtime-facts"
	file := rewriteFixtureManifest(t, runnerID, "test.idempotency", func(m *RunnerManifest) {
		primary := m.Steps[0]
		primary.ParallelGroup = "same-oracle-two-actors"
		primary.Call.Extract = []ExtractRule{{Name: "primary_order", Source: "oracle", OracleID: "fixture-independent-store", Path: "/order_id", EntityKind: "order", Required: true}}
		primary.Oracles[0].Measurements["settled"] = "/settled"
		contender := primary
		contender.Actor = "contender"
		contender.Call.CredentialHeader = "X-Fixture-Actor"
		contender.Call.Extract = []ExtractRule{{Name: "contender_order", Source: "oracle", OracleID: "fixture-independent-store", Path: "/order_id", EntityKind: "order", Required: true}}
		contender.Oracles = append([]OracleAdapter(nil), primary.Oracles...)
		contender.Oracles[0].Call.CredentialHeader = "X-Fixture-Actor"
		if waitForContender {
			wait := WaitContract{OracleID: "fixture-independent-store", RuleSourceRef: "spec/settlement", MaxWaitMS: 100, PollIntervalMS: 25, UntilChecks: []PropertyCheck{{Left: "settled", Operator: "eq", Value: json.RawMessage(`true`)}}}
			primary.AsyncWait, contender.AsyncWait = &wait, &wait
		}
		m.Steps = []RunnerStep{primary, contender}
	})
	fixture.registerAndRound(t, file)
	if _, err := fixture.d.plan("course-v2", 128); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.d.execute(context.Background(), ExecuteRequest{Task: "course-v2/test.idempotency", Runner: runnerID, Round: fixture.round, Environment: "independent", Commit: fixture.commit, Root: fixture.root, Config: fixture.config, Account: fixture.account, Fixture: fixture.fixture, Accounts: map[string]string{"contender": "fixture.contender"}})
	if err != nil {
		t.Fatal(err)
	}
	return fixture, result
}

func TestRuntimeFactsParallelOracleExtractionsStayWithActor(t *testing.T) {
	fixture, result := runParallelRuntimeFacts(t, false)
	if result.Verdict != "PASS" {
		t.Fatalf("control run failed: %+v", result)
	}
	raw, _, err := fixture.d.workbenchEvidence(result.Run, "action", 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, extracted := range raw.Model.Extractions {
		want := map[string]string{"primary_order": "order-primary", "contender_order": "order-contender"}[extracted.Name]
		if extracted.Value != want {
			t.Fatalf("cross-account oracle extraction: %+v want=%s", extracted, want)
		}
	}
	if len(raw.Model.Extractions) != 2 {
		t.Fatalf("missing independent extraction records: %+v", raw.Model.Extractions)
	}
}

func TestRuntimeFactsParallelWaitsCannotReuseAnotherActorSuccess(t *testing.T) {
	fixture, result := runParallelRuntimeFacts(t, true)
	if result.Verdict != "FAIL" {
		t.Fatalf("contender never settled but borrowed another actor's successful wait: %+v", result)
	}
	raw, _, err := fixture.d.workbenchEvidence(result.Run, "action", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw.Model.Waits) != 2 || raw.Model.Waits[0].TimedOut || !raw.Model.Waits[1].TimedOut {
		t.Fatalf("test did not establish distinct actor outcomes: %+v", raw.Model.Waits)
	}
}

func TestRuntimeFactsWaitHasCompletionAndActorIdentity(t *testing.T) {
	_, server := newBusinessChainServer(t)
	fixture, _ := setupBusinessChainFixture(t, server.URL, businessChainManifest("wait-fact-identity"))
	result := runBusinessChain(t, fixture, "wait-fact-identity")
	if result.Verdict != "PASS" {
		t.Fatalf("control run failed: %+v", result)
	}
	raw, _, err := fixture.d.workbenchEvidence(result.Run, "action", 3)
	if err != nil || len(raw.Model.Waits) != 1 {
		t.Fatalf("missing wait evidence: %+v %v", raw, err)
	}
	wait := raw.Model.Waits[0]
	started, startErr := time.Parse(time.RFC3339Nano, wait.Started)
	completed, endErr := time.Parse(time.RFC3339Nano, wait.Completed)
	if startErr != nil || endErr != nil || completed.Before(started) {
		t.Fatalf("wait time interval is incomplete: %+v start=%v end=%v", wait, startErr, endErr)
	}
	body, _ := json.Marshal(wait)
	var fields map[string]any
	_ = json.Unmarshal(body, &fields)
	if fields["actor"] != "worker" || fields["step"] != float64(3) || fields["action_index"] != float64(1) {
		t.Fatalf("wait lacks unambiguous execution identity: %s", body)
	}
}

func TestRuntimeFactsExplainRejectsCorruptEvidence(t *testing.T) {
	_, server := newBusinessChainServer(t)
	fixture, _ := setupBusinessChainFixture(t, server.URL, businessChainManifest("explain-integrity"))
	result := runBusinessChain(t, fixture, "explain-integrity")
	if result.Verdict != "PASS" {
		t.Fatalf("control run failed: %+v", result)
	}
	where := " WHERE source_kind='sequence' AND run=" + quote(result.Run) + " AND seq=1"
	rows, err := fixture.d.query("SELECT raw_json,facts_json,evidence_path FROM step_facts"+where, 3)
	if err != nil || len(rows) != 1 {
		t.Fatalf("missing fact: %v %v", rows, err)
	}
	for _, scenario := range []struct{ name, column, value string }{
		{"invalid_facts_json", "facts_json", "{"},
		{"embedded_raw_hash_mismatch", "raw_json", "{}"},
		{"valid_json_disagrees_with_raw", "facts_json", strings.Replace(rows[0][1], `"verdict":"PASS"`, `"verdict":"FAIL"`, 1)},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if err := fixture.d.tx([]string{"UPDATE step_facts SET " + scenario.column + "=" + quote(scenario.value) + where}); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := fixture.d.tx([]string{"UPDATE step_facts SET raw_json=" + quote(rows[0][0]) + ",facts_json=" + quote(rows[0][1]) + where}); err != nil {
					t.Fatal(err)
				}
			}()
			if explained, err := fixture.d.explainFacts("", result.Run, "", explainAllSteps); err == nil {
				t.Fatalf("explain certified corrupt evidence: %+v", explained)
			}
		})
	}
	t.Run("assertion_projection_disagrees", func(t *testing.T) {
		assertWhere := " WHERE source_kind='sequence' AND run=" + quote(result.Run) + " AND seq=1 AND assertion_idx=1"
		before, err := fixture.d.query("SELECT actual_json FROM step_assertions"+assertWhere, 1)
		if err != nil || len(before) != 1 {
			t.Fatalf("missing assertion: %v %v", before, err)
		}
		if err := fixture.d.tx([]string{"UPDATE step_assertions SET actual_json='corrupted'" + assertWhere}); err != nil {
			t.Fatal(err)
		}
		defer fixture.d.tx([]string{"UPDATE step_assertions SET actual_json=" + quote(before[0][0]) + assertWhere})
		if explained, err := fixture.d.explainFacts("", result.Run, "", explainAllSteps); err == nil {
			t.Fatalf("explain trusted a changed assertion projection: %+v", explained)
		}
	})
	t.Run("disk_raw_hash_mismatch", func(t *testing.T) {
		path := rows[0][2]
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(body, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
		defer os.WriteFile(path, body, 0600)
		if explained, err := fixture.d.explainFacts("", result.Run, "", explainAllSteps); err == nil {
			t.Fatalf("explain ignored changed disk RAW: %+v", explained)
		}
	})
}

func TestRuntimeFactsExplainDoesNotMoveFutureEntitiesIntoSetup(t *testing.T) {
	fixture, result := runParallelRuntimeFacts(t, false)
	explained, err := fixture.d.explainFacts("", result.Run, "", -1)
	if err != nil {
		t.Fatal(err)
	}
	steps := explained["steps"].([]ExplainStep)
	if len(steps) != 1 {
		t.Fatalf("missing setup: %+v", steps)
	}
	entities, _ := steps[0].BusinessObjects["entities"].([]EntityBinding)
	if len(entities) != 0 {
		t.Fatalf("setup acquired entities that were only extracted by a future action: %+v", entities)
	}
}

func TestRuntimeFactsConcurrentCookieScopes(t *testing.T) {
	sess := newExecSession("cookie-race", nil)
	const workers = 32
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			for j := 0; j < 16; j++ {
				if sess.cookieJar(fmt.Sprintf("actor-%d", i%4)) == nil {
					t.Error("cookie scope was not initialized")
				}
			}
		}(i)
	}
	close(start)
	wg.Wait()
}

func TestRuntimeFactsLegacyWaitReadableButCannotClaimNewIdentityProof(t *testing.T) {
	_, server := newBusinessChainServer(t)
	fixture, _ := setupBusinessChainFixture(t, server.URL, businessChainManifest("legacy-wait"))
	result := runBusinessChain(t, fixture, "legacy-wait")
	if result.Verdict != "PASS" {
		t.Fatalf("control run failed: %+v", result)
	}
	raw, _, err := fixture.d.workbenchEvidence(result.Run, "action", 3)
	if err != nil {
		t.Fatal(err)
	}
	raw.Model.EvidenceVersion = 0
	for i := range raw.Model.Actions {
		raw.Model.Actions[i].ActionIndex = 0
	}
	for i := range raw.Model.Oracles {
		raw.Model.Oracles[i].ActionIndex = 0
	}
	for i := range raw.Model.Waits {
		wait := &raw.Model.Waits[i]
		wait.Actor, wait.Node, wait.Transition, wait.Completed, wait.StableSince = "", "", "", "", ""
		wait.Step, wait.ActionIndex = 0, 0
	}
	body, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := parseStepRAW(body, raw.Run, raw.Expected, raw.Observed, raw.Step)
	if err != nil {
		t.Fatalf("historical RAW is no longer readable: %v", err)
	}
	// Emulate the capture receipt an old engine would have made. The test
	// changes no saved current run or its original historical PASS rows.
	if err := fixture.d.captureModelRAW(raw.Run, raw.Step, body); err != nil {
		t.Fatal(err)
	}
	contract, err := fixture.d.runTaskContract(raw.Run)
	if err != nil {
		t.Fatal(err)
	}
	verdict, reason, _, err := fixture.d.evaluateModelObservation(contract, legacy, raw.Step, digestBytes(body))
	if err != nil || verdict != "BLOCKED" || !strings.Contains(reason, "legacy async evidence") {
		t.Fatalf("missing historical identity was silently upgraded: verdict=%s reason=%s err=%v", verdict, reason, err)
	}
}

func TestRuntimeFactsDeletedStepCannotLeaveVerifiedRunComplete(t *testing.T) {
	fixture, result := runParallelRuntimeFacts(t, false)
	if result.Verdict != "PASS" {
		t.Fatal(result)
	}
	if err := fixture.d.tx([]string{"DELETE FROM step_facts WHERE run=" + quote(result.Run) + " AND seq=1"}); err != nil {
		t.Fatal(err)
	}
	if facts, err := fixture.d.verifiedRunFacts(result.Run); err == nil {
		t.Fatalf("missing original action was called complete: %+v", facts)
	}
}
