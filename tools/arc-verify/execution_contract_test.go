package main

import (
	"context"
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

// rejectionServer reports login=false; a correct target answers 401 and keeps the
// enrollment unchanged, a defective one accepts the action anyway.
func rejectionServer(t *testing.T, acceptsAnyway bool) *httptest.Server {
	t.Helper()
	var mutex sync.Mutex
	enrolled := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()
		switch r.URL.Path {
		case "/enroll":
			if !acceptsAnyway {
				http.Error(w, `{"error":"login required"}`, http.StatusUnauthorized)
				return
			}
			enrolled = 1
			fmt.Fprint(w, `{}`)
		case "/reset":
			enrolled = 0
			fmt.Fprint(w, `{}`)
		default:
			state := "pending"
			if enrolled > 0 {
				state = "ready"
			}
			fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":false,"enrolled":%d,"capacity":1}`, state, enrolled)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestExecuteModelRejectionNeedsUnchangedState(t *testing.T) {
	for _, scenario := range []struct {
		name          string
		acceptsAnyway bool
		verdict       string
	}{
		{"rejected and unchanged", false, "PASS"},
		{"accepted despite failed condition", true, "FAIL"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			fixture := prepareExecutionFixture(t, referenceModel(t), rejectionServer(t, scenario.acceptsAnyway).URL)
			// The fixture's restored state is "logged out", so recovery must check login=false.
			fixture.registerAndRound(t, rewriteFixtureManifest(t, "negative-runner", "R1/NEGATIVE/login", func(m *RunnerManifest) {
				m.Recovery.Conditions = map[string]bool{"login": false}
			}))
			var result ExecuteResult
			if err := json.Unmarshal(fixture.execute(t, "negative-runner", "R1/NEGATIVE/login"), &result); err != nil {
				t.Fatal(err)
			}
			if result.Verdict != scenario.verdict || result.Recovery.Verdict != "PASS" {
				t.Fatalf("rejection obligation verdict: %+v", result)
			}
			trace, err := fixture.d.trace(result.Run)
			if err != nil || len(trace.Steps) != 1 || trace.Steps[0].Observation == nil || len(trace.Steps[0].Observation.Actions) != 1 {
				t.Fatalf("rejection attempt left no captured action: %+v %v", trace, err)
			}
			if denied := trace.Steps[0].Observation.Actions[0].Denied; denied == scenario.acceptsAnyway {
				t.Fatalf("captured transport outcome is wrong: denied=%v", denied)
			}
		})
	}
}

func TestRunnerRejectsActionResponseAsOracle(t *testing.T) {
	fixture := prepareExecutionFixture(t, referenceModel(t), "http://127.0.0.1:9")
	file := rewriteFixtureManifest(t, "self-oracle", "R1/POSITIVE", func(m *RunnerManifest) {
		m.Steps[0].Call = m.Steps[0].Oracles[0].Call // a read whose own response would be its oracle
	})
	err := run([]string{"runner", "--db", fixture.d.file, "--file", file})
	if err == nil || !strings.Contains(err.Error(), "independent oracle") {
		t.Fatalf("the action's own response was accepted as its oracle: %v", err)
	}
}

func TestExecuteRefusesStaleRound(t *testing.T) {
	fixture := prepareExecutionFixture(t, referenceModel(t), rejectionServer(t, false).URL)
	fixture.registerAndRound(t, fixtureManifest(t, "stale-runner", "R1/POSITIVE"))
	if _, err := fixture.d.plan("course-v2", 128); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.root, "after-round.txt"), []byte("candidate changed after the round was opened"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := fixture.d.execute(context.Background(), ExecuteRequest{Task: "course-v2/R1/POSITIVE", Runner: "stale-runner", Round: fixture.round, Environment: "independent", Commit: fixture.commit, Root: fixture.root, Config: fixture.config, Account: fixture.account, Fixture: fixture.fixture})
	if err == nil {
		t.Fatal("execution ran against a round whose candidate changed")
	}
	rows, err := fixture.d.query("SELECT COUNT(*) FROM runs WHERE task='course-v2/R1/POSITIVE'", 1)
	if err != nil || len(rows) != 1 || rows[0][0] != "0" {
		t.Fatalf("a stale round still created a run: %v %v", rows, err)
	}
}

func TestExecutePreconditionMismatchRecordsRAW(t *testing.T) {
	target, server := newChainTarget(t)
	target.loggedOut = true
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	fixture.registerAndRound(t, rewriteFixtureManifest(t, "logged-out", "R1/POSITIVE", func(m *RunnerManifest) {
		m.Recovery.Conditions = map[string]bool{"login": false}
	}))
	var result ExecuteResult
	if err := json.Unmarshal(fixture.execute(t, "logged-out", "R1/POSITIVE"), &result); err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "BLOCKED" || !strings.Contains(result.Reason, "preconditions do not match") {
		t.Fatalf("an unmet precondition must block with its real reason: %+v", result)
	}
	trace, err := fixture.d.trace(result.Run)
	if err != nil || len(trace.Steps) != 1 || trace.Steps[0].EvidenceStatus != "PASS" || trace.Steps[0].Verdict != "BLOCKED" {
		t.Fatalf("the not-attempted step lost its RAW: %+v %v", trace, err)
	}
}
