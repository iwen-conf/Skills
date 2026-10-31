package main

import (
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

type executionFixture struct {
	d                                             db
	root, config, commit, account, fixture, round string
}

func prepareExecutionFixture(t *testing.T, graph Graph, target string) executionFixture {
	t.Helper()
	d := modelTestDB(t, graph)
	root := filepath.Join(t.TempDir(), "candidate")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "business.txt"), []byte("local HTTP acceptance fixture; not IM-mongo evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "business.txt"}, {"-c", "user.name=verify-fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "-m", "synthetic fixture"}} {
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1"}
		if body, err := command.CombinedOutput(); err != nil {
			t.Fatalf("fixture git: %v %s", err, body)
		}
	}
	commit, _, err := sourceFingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(t.TempDir(), "config.jsonc")
	body := fmt.Sprintf(`{"environments":{"independent":{"approved":true,"url":%q}}}`, target)
	if err := os.WriteFile(config, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = d.importFixtures(Fixtures{Fixtures: []Fixture{
		{ID: "fixture.student", Kind: "account", State: "ACTIVE", Refs: map[string]string{"role": "STUDENT", "principal": "synthetic-STUDENT", "environment": "independent"}},
		{ID: "fixture.course", Kind: "business", State: "pending", Refs: map[string]string{"environment": "independent", "scope": "course-fixture"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return executionFixture{d: d, root: root, config: config, commit: commit, account: "fixture.student", fixture: "fixture.course"}
}

func fixtureManifest(t *testing.T, id, test string) string {
	t.Helper()
	body := fmt.Sprintf(`{"schema_version":1,"id":%q,"graph":"course-v2","test":%q,"isolation_scope":"course-fixture","timeout_ms":1200,"step_timeout_ms":200,"parallelism":2,
"steps":[{"node":"enroll","actor":"primary","call":{"kind":"http","method":"POST","path":"/enroll","success_statuses":[200],"rejection_statuses":[401]},"oracles":[{"id":"fixture-independent-store","boundary":"after committed enrollment and fixture restoration","call":{"kind":"http","method":"GET","path":"/oracle","success_statuses":[200]},"state_path":"/state","principal_path":"/principal","role_path":"/role","scope_path":"/scope","conditions":{"login":"/login"},"measurements":{"enrolled":"/enrolled","capacity":"/capacity"}}]}],
"recovery":{"isolation_scope":"course-fixture","steps":[{"actor":"primary","call":{"kind":"http","method":"POST","path":"/reset","success_statuses":[200]}}],"oracle":{"id":"fixture-independent-store","boundary":"after committed enrollment and fixture restoration","call":{"kind":"http","method":"GET","path":"/oracle","success_statuses":[200]},"state_path":"/state","principal_path":"/principal","role_path":"/role","scope_path":"/scope","conditions":{"login":"/login"},"measurements":{"enrolled":"/enrolled","capacity":"/capacity"}},"state":"pending","conditions":{"login":true},"checks":[{"left":"enrolled","operator":"eq","value":0}]}}`, id, test)
	path := filepath.Join(t.TempDir(), "runner.json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func (fixture *executionFixture) registerAndRound(t *testing.T, file string) {
	t.Helper()
	cliJSON(t, "runner", "--db", fixture.d.file, "--file", file)
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(cliJSON(t, "round", "--db", fixture.d.file, "--graph", "course-v2", "--env", "independent", "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit), &result); err != nil {
		t.Fatal(err)
	}
	if result.ID == "" {
		t.Fatal("round identity missing")
	}
	fixture.round = result.ID
}

func (fixture executionFixture) execute(t *testing.T, runner, task string) []byte {
	t.Helper()
	if _, err := fixture.d.plan("course-v2", 128); err != nil {
		t.Fatal(err)
	}
	return cliJSON(t, "execute", "--db", fixture.d.file, "--task", "course-v2/"+task, "--runner", runner, "--round", fixture.round, "--env", "independent", "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit, "--account", fixture.account, "--fixture", fixture.fixture)
}

func TestExecuteUsesIndependentOracleAndRestoresFixture(t *testing.T) {
	var mutex sync.Mutex
	enrolled := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/enroll":
			enrolled = 1
			fmt.Fprint(w, `{"accepted":true}`)
		case "/reset":
			enrolled = 0
			fmt.Fprint(w, `{}`)
		case "/oracle":
			state := "pending"
			if enrolled > 0 {
				state = "ready"
			}
			fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d,"capacity":1}`, state, enrolled)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	fixture.registerAndRound(t, fixtureManifest(t, "fixture.runner", "R1/POSITIVE"))
	var result struct {
		Run, Verdict string
		Recovery     struct{ Verdict string } `json:"recovery"`
	}
	if err := json.Unmarshal(fixture.execute(t, "fixture.runner", "R1/POSITIVE"), &result); err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "PASS" || result.Recovery.Verdict != "PASS" {
		t.Fatalf("real fixture flow did not close: %+v", result)
	}
	mutex.Lock()
	restored := enrolled == 0
	mutex.Unlock()
	if !restored {
		t.Fatal("database-style ledger completion did not restore the external fixture")
	}
	var trace struct {
		Steps []struct {
			Node string `json:"node"`
			SHA  string `json:"sha256"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(cliJSON(t, "trace", "--db", fixture.d.file, "--run", result.Run), &trace); err != nil {
		t.Fatal(err)
	}
	if len(trace.Steps) != 1 || trace.Steps[0].Node != "enroll" || len(trace.Steps[0].SHA) != 64 {
		t.Fatalf("actual replay is missing node/evidence binding: %+v", trace)
	}
}

func TestExecutePropertyCounterexampleCannotPassAndFailedRecoveryKeepsLocks(t *testing.T) {
	enrolled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/enroll":
			enrolled = true
			fmt.Fprint(w, `{"accepted":true}`)
		case "/reset":
			http.Error(w, "fixture reset rejected", http.StatusServiceUnavailable)
		case "/oracle":
			if enrolled {
				fmt.Fprint(w, `{"state":"ready","principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":9007199254740994,"capacity":9007199254740993}`)
			} else {
				fmt.Fprint(w, `{"state":"pending","principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":0,"capacity":9007199254740993}`)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	fixture.registerAndRound(t, fixtureManifest(t, "counterexample", "R1/POSITIVE"))
	var result struct {
		Run, Verdict string
		Recovery     struct{ Verdict string } `json:"recovery"`
	}
	if err := json.Unmarshal(fixture.execute(t, "counterexample", "R1/POSITIVE"), &result); err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "FAIL" || result.Recovery.Verdict == "PASS" {
		t.Fatalf("property or recovery was falsely accepted: %+v", result)
	}
	rows, err := fixture.d.query("SELECT COUNT(*) FROM fixture_locks WHERE run="+quote(result.Run), 1)
	if err != nil || len(rows) != 1 || rows[0][0] != "2" {
		t.Fatalf("unsafe fixture was released: %v %v", rows, err)
	}
	trace, err := fixture.d.trace(result.Run)
	if err != nil || trace.FirstDivergence != "1" || trace.Status != "FAIL" {
		t.Fatalf("first counterexample was lost: %+v %v", trace, err)
	}
}

func TestExecuteWrongPrincipalBlocksBeforeAction(t *testing.T) {
	actions := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/enroll" {
			actions++
			fmt.Fprint(w, `{}`)
			return
		}
		fmt.Fprint(w, `{"state":"pending","principal":"another-account","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":0,"capacity":1}`)
	}))
	defer server.Close()
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	fixture.registerAndRound(t, fixtureManifest(t, "identity", "R1/POSITIVE"))
	var result struct{ Verdict string }
	if err := json.Unmarshal(fixture.execute(t, "identity", "R1/POSITIVE"), &result); err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "BLOCKED" || actions != 0 {
		t.Fatalf("wrong account performed a business action: %+v actions=%d", result, actions)
	}
}

func TestExecuteDeadlineStopsActionAndRecordsRestoration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/enroll" {
			<-r.Context().Done()
			return
		}
		if r.URL.Path == "/reset" {
			fmt.Fprint(w, `{}`)
			return
		}
		fmt.Fprint(w, `{"state":"pending","principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":0,"capacity":1}`)
	}))
	defer server.Close()
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	file := fixtureManifest(t, "deadline", "R1/POSITIVE")
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	body = []byte(strings.Replace(string(body), `"step_timeout_ms":200`, `"step_timeout_ms":25`, 1))
	if err = os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	fixture.registerAndRound(t, file)
	var result struct {
		Verdict  string
		Recovery struct{ Verdict string } `json:"recovery"`
	}
	if err := json.Unmarshal(fixture.execute(t, "deadline", "R1/POSITIVE"), &result); err != nil {
		t.Fatal(err)
	}
	if result.Verdict != "BLOCKED" || result.Recovery.Verdict != "PASS" {
		t.Fatalf("deadline/result/restore did not close safely: %+v", result)
	}
}
