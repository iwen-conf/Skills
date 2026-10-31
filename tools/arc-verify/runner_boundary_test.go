package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCommandCaptureFailureCannotPass(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{filepath.Join(t.TempDir(), "missing-command")},
		{python, "-c", "import os\ntry:\n for i in range(2048): os.write(1,b'x'*4096)\nexcept OSError: pass\nos._exit(0)"},
	} {
		output, err := performCall(context.Background(), AdapterCall{Kind: adapterCommand, Args: args, SuccessExitCodes: []int{0}}, RunBinding{SourceRoot: t.TempDir()}, Fixture{})
		if err == nil {
			t.Errorf("execution/capture failure became success: exit=%d bytes=%d", output.ExitCode, len(output.Body))
		}
	}
}

func rewriteFixtureManifest(t *testing.T, id, test string, change func(*RunnerManifest)) string {
	t.Helper()
	file := fixtureManifest(t, id, test)
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var manifest RunnerManifest
	if err := decodeStrictJSON(body, &manifest); err != nil {
		t.Fatal(err)
	}
	change(&manifest)
	body, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestExecuteVerifiesIsolationBeforeAnyFixtureWrite(t *testing.T) {
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			writes++
			fmt.Fprint(w, `{}`)
			return
		}
		fmt.Fprint(w, `{"state":"pending","principal":"wrong-account","role":"STUDENT","scope":"another-course","login":true,"enrolled":0,"capacity":1}`)
	}))
	defer server.Close()
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	file := rewriteFixtureManifest(t, "isolation-first", "R1/POSITIVE", func(m *RunnerManifest) {
		m.Setup = []RecoveryStep{{Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: "/prepare", SuccessStatuses: []int{200}}}}
	})
	fixture.registerAndRound(t, file)
	var result ExecuteResult
	if err := json.Unmarshal(fixture.execute(t, "isolation-first", "R1/POSITIVE"), &result); err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "BLOCKED" || writes != 0 || result.Recovery.Verdict == "PASS" {
		t.Fatalf("unverified identity/scope allowed setup or recovery writes: result=%+v writes=%d", result, writes)
	}
}

func TestExecuteFailedRecoveryHasDurableRAW(t *testing.T) {
	enrolled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/enroll":
			enrolled = true
			fmt.Fprint(w, `{}`)
		case "/reset":
			http.Error(w, "reset failed", http.StatusServiceUnavailable)
		default:
			state, count := "pending", 0
			if enrolled {
				state, count = "ready", 1
			}
			fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d,"capacity":1}`, state, count)
		}
	}))
	defer server.Close()
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	fixture.registerAndRound(t, fixtureManifest(t, "recovery-failure", "R1/POSITIVE"))
	var result ExecuteResult
	if err := json.Unmarshal(fixture.execute(t, "recovery-failure", "R1/POSITIVE"), &result); err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "BLOCKED" || result.Recovery.Verdict != "BLOCKED" || result.Recovery.Evidence == "" || len(result.Recovery.SHA256) != 64 {
		t.Fatalf("failed external restoration lost its independently captured evidence: %+v", result)
	}
	if _, _, err := loadEvidence(result.Recovery.Evidence); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteReadOnlyCheckpointUsesIndependentBeforeAfter(t *testing.T) {
	graph := referenceModel(t)
	graph.Tests[0].ReadOnly, graph.Tests[0].Expected = true, "pending"
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writes++
		}
		fmt.Fprint(w, `{"state":"pending","principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":0,"capacity":1}`)
	}))
	defer server.Close()
	fixture := prepareExecutionFixture(t, graph, server.URL)
	file := rewriteFixtureManifest(t, "read-only", "test.idempotency", func(m *RunnerManifest) {
		m.Steps[0].Call.Method = http.MethodGet
		m.Steps[0].Call.RejectionStatuses = nil
		m.Recovery = nil
	})
	fixture.registerAndRound(t, file)
	var result ExecuteResult
	if err := json.Unmarshal(fixture.execute(t, "read-only", "test.idempotency"), &result); err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "PASS" || result.Recovery.Verdict != "PASS" || !result.Recovery.Safe || result.Recovery.Evidence == "" || writes != 0 {
		t.Fatalf("read-only observation has no independently proven checkpoint: %+v writes=%d", result, writes)
	}
}

func TestCommandScriptMustBeInFrozenInputs(t *testing.T) {
	file := filepath.Join(t.TempDir(), "oracle.py")
	if err := os.WriteFile(file, []byte("print('{}')\n"), 0600); err != nil {
		t.Fatal(err)
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	manifest := RunnerManifest{Steps: []RunnerStep{{Call: AdapterCall{Kind: adapterCommand, Args: []string{python, file}, SuccessExitCodes: []int{0}}}}}
	if _, err := runnerCodeFingerprint(manifest); err == nil {
		t.Fatal("executable script omitted from the frozen runner inputs")
	}
	manifest.InputFiles = []string{file}
	before, err := runnerCodeFingerprint(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("print('{\"state\":\"changed\"}')\n"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := runnerCodeFingerprint(manifest)
	if err != nil || before == after {
		t.Fatalf("changed script retained its frozen code identity: %s %s %v", before, after, err)
	}
}

func TestModelRetryUsesCapturedRecoveryAndKeepsFailure(t *testing.T) {
	graph := referenceModel(t)
	enrolled, fail := 0, true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/enroll":
			enrolled = 1
			fmt.Fprint(w, `{}`)
		case "/reset":
			enrolled = 0
			fmt.Fprint(w, `{}`)
		default:
			state, capacity := "pending", 1
			if enrolled > 0 {
				state = "ready"
				if fail {
					capacity = 0
				}
			}
			fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d,"capacity":%d}`, state, enrolled, capacity)
		}
	}))
	defer server.Close()
	fixture := prepareExecutionFixture(t, graph, server.URL)
	fixture.registerAndRound(t, fixtureManifest(t, "model-retry", "R1/POSITIVE"))
	var first ExecuteResult
	if err := json.Unmarshal(fixture.execute(t, "model-retry", "R1/POSITIVE"), &first); err != nil {
		t.Fatal(err)
	}
	if first.Verdict != "FAIL" || first.Recovery.Verdict != "PASS" {
		t.Fatalf("counterexample was not preserved: %+v", first)
	}
	if _, err := fixture.d.retry("course-v2/R1/POSITIVE", "fixture independently restored", first.Recovery.Evidence, fixture.root, fixture.config, fixture.commit); err != nil {
		t.Fatalf("MODEL task cannot use its captured independent recovery: %v", err)
	}
	fail = false
	var second ExecuteResult
	if err := json.Unmarshal(fixture.execute(t, "model-retry", "R1/POSITIVE"), &second); err != nil {
		t.Fatal(err)
	}
	if second.Verdict != "PASS" || second.Run == first.Run {
		t.Fatalf("recovery did not start a new attempt: %+v", second)
	}
	trace, err := fixture.d.trace(first.Run)
	if err != nil || !strings.EqualFold(trace.Status, "FAIL") {
		t.Fatalf("retry changed the historical counterexample: %+v %v", trace, err)
	}
}

func TestExecuteConcurrentParticipantPreconditionsAndOracleIdentity(t *testing.T) {
	for _, ready := range []bool{false, true} {
		t.Run(fmt.Sprint(ready), func(t *testing.T) {
			graph := referenceModel(t)
			graph.Tests[0].Kind = testConcurrency
			graph.Tests[0].Actors = []ActorRequirement{{Alias: "contender", Role: "STUDENT"}}
			var mutex sync.Mutex
			enrolled, actions := 0, 0
			started := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mutex.Lock()
				switch r.URL.Path {
				case "/enroll":
					actions++
					enrolled++
					if actions == 2 {
						close(started)
					}
					mutex.Unlock()
					select {
					case <-started:
					case <-r.Context().Done():
					}
					fmt.Fprint(w, `{}`)
				case "/reset":
					enrolled = 0
					mutex.Unlock()
					fmt.Fprint(w, `{}`)
				default:
					principal, login := "synthetic-STUDENT", true
					if r.Header.Get("X-Fixture-Actor") == "contender" {
						principal, login = "synthetic-CONTENDER", ready
					}
					state := "pending"
					if enrolled > 0 {
						state = "ready"
					}
					fmt.Fprintf(w, `{"state":%q,"principal":%q,"role":"STUDENT","scope":"course-fixture","login":%t,"enrolled":%d,"capacity":2}`, state, principal, login, enrolled)
					mutex.Unlock()
				}
			}))
			defer server.Close()
			fixture := prepareExecutionFixture(t, graph, server.URL)
			t.Setenv("ARC_TEST_CONTENDER", "contender")
			if _, err := fixture.d.importFixtures(Fixtures{Fixtures: []Fixture{{ID: "fixture.contender", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_TEST_CONTENDER", Refs: map[string]string{"role": "STUDENT", "principal": "synthetic-CONTENDER", "environment": "independent"}}}}); err != nil {
				t.Fatal(err)
			}
			file := rewriteFixtureManifest(t, "concurrent-participants", "test.idempotency", func(m *RunnerManifest) {
				primary := m.Steps[0]
				primary.ParallelGroup = "last-two-places"
				contender := primary
				contender.Actor = "contender"
				contender.Oracles = append([]OracleAdapter(nil), primary.Oracles...)
				contender.Call.CredentialHeader = "X-Fixture-Actor"
				contender.Oracles[0].Call.CredentialHeader = "X-Fixture-Actor"
				m.Steps = []RunnerStep{contender, primary}
			})
			fixture.registerAndRound(t, file)
			if _, err := fixture.d.plan("course-v2", 128); err != nil {
				t.Fatal(err)
			}
			result, err := fixture.d.execute(context.Background(), ExecuteRequest{Task: "course-v2/test.idempotency", Runner: "concurrent-participants", Round: fixture.round, Environment: "independent", Commit: fixture.commit, Root: fixture.root, Config: fixture.config, Account: fixture.account, Fixture: fixture.fixture, Accounts: map[string]string{"contender": "fixture.contender"}})
			if err != nil {
				t.Fatal(err)
			}
			mutex.Lock()
			count := actions
			mutex.Unlock()
			if !ready && (result.Verdict != "BLOCKED" || count != 0) {
				t.Fatalf("a participant with wrong preconditions performed a business action: %+v actions=%d", result, count)
			}
			if ready && (result.Verdict != "PASS" || count != 2) {
				t.Fatalf("independent oracles from distinct participants were confused: %+v actions=%d", result, count)
			}
		})
	}
}
