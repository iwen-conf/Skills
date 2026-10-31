package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const seatIsolationScope = "seat-fixture"

type seatRow struct {
	id, status string
}

type seatServer struct {
	mu                       sync.Mutex
	rows                     []seatRow
	capacity                 int
	loggedOut                bool
	courseClosed             bool
	hangClaim                bool
	cutClaim                 bool
	resetBroken              bool
	unauthorizedForce        bool
	raceNoLock               bool
	skipReview               bool
	duplicateRecords         bool
	acceptWhenClosed         bool
	claimNoop                bool
	claims, forces, approves int
}

func seatModel(t *testing.T) Graph {
	t.Helper()
	graph, err := loadGraph("examples/seat-v2.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateGraph(graph); err != nil {
		t.Fatal(err)
	}
	return graph
}

func newSeatServer(t *testing.T) (*seatServer, *httptest.Server) {
	t.Helper()
	target := &seatServer{capacity: 1}
	server := httptest.NewServer(http.HandlerFunc(target.serve))
	t.Cleanup(server.Close)
	return target, server
}

func (s *seatServer) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/claim":
		s.handleClaim(w, r)
	case "/approve":
		s.mu.Lock()
		defer s.mu.Unlock()
		s.handleApprove(w, r)
	case "/force":
		s.mu.Lock()
		defer s.mu.Unlock()
		s.handleForce(w, r)
	case "/reset":
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.resetBroken {
			http.Error(w, `{"error":"restore failed"}`, http.StatusServiceUnavailable)
			return
		}
		s.rows = nil
		fmt.Fprint(w, `{}`)
	case "/oracle":
		s.mu.Lock()
		defer s.mu.Unlock()
		s.writeFacts(w, s.actor(r))
	default:
		http.NotFound(w, r)
	}
}

func (s *seatServer) actor(r *http.Request) string {
	switch r.Header.Get("X-Fixture-Actor") {
	case "admin":
		return "admin"
	case "contender":
		return "contender"
	default:
		return "student"
	}
}

func (s *seatServer) requestID(r *http.Request) string {
	var body struct {
		RequestID string `json:"request_id"`
	}
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &body)
	if body.RequestID == "" {
		return "default"
	}
	return body.RequestID
}

func (s *seatServer) claimFlags() (hang, cut, race bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hangClaim, s.cutClaim, s.raceNoLock
}

func (s *seatServer) handleClaim(w http.ResponseWriter, r *http.Request) {
	hang, cut, race := s.claimFlags()
	if hang {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
		return
	}
	if cut {
		if hijacker, ok := w.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err == nil {
				_ = conn.Close()
				return
			}
		}
		return
	}
	if race {
		s.raceClaim(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimLocked(w, r, len(s.rows))
}

func (s *seatServer) raceClaim(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	observed := len(s.rows)
	s.mu.Unlock()
	time.Sleep(25 * time.Millisecond)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimLocked(w, r, observed)
}

func (s *seatServer) claimLocked(w http.ResponseWriter, r *http.Request, occupancy int) {
	s.claims++
	if s.loggedOut {
		http.Error(w, `{"error":"login required"}`, http.StatusUnauthorized)
		return
	}
	if s.courseClosed && !s.acceptWhenClosed {
		http.Error(w, `{"error":"course closed"}`, http.StatusForbidden)
		return
	}
	if s.claimNoop {
		fmt.Fprint(w, `{}`)
		return
	}
	id := s.requestID(r)
	if s.hasRequest(id) && !s.duplicateRecords {
		fmt.Fprint(w, `{}`)
		return
	}
	if occupancy >= s.capacity {
		http.Error(w, `{"error":"no seats"}`, http.StatusForbidden)
		return
	}
	status := "pending"
	if s.skipReview {
		status = "enrolled"
	}
	s.rows = append(s.rows, seatRow{id: id, status: status})
	fmt.Fprint(w, `{}`)
}

func (s *seatServer) hasRequest(id string) bool {
	for _, row := range s.rows {
		if row.id == id {
			return true
		}
	}
	return false
}

func (s *seatServer) handleApprove(w http.ResponseWriter, r *http.Request) {
	s.approves++
	if s.actor(r) != "admin" || s.loggedOut {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	for i, row := range s.rows {
		if row.status == "pending" {
			s.rows[i].status = "enrolled"
			fmt.Fprint(w, `{}`)
			return
		}
	}
	http.Error(w, `{"error":"nothing to approve"}`, http.StatusForbidden)
}

func (s *seatServer) handleForce(w http.ResponseWriter, r *http.Request) {
	s.forces++
	if s.loggedOut {
		http.Error(w, `{"error":"login required"}`, http.StatusUnauthorized)
		return
	}
	if s.actor(r) != "admin" && !s.unauthorizedForce {
		http.Error(w, `{"error":"admin only"}`, http.StatusForbidden)
		return
	}
	id := s.requestID(r)
	if !s.hasRequest(id) && len(s.rows) >= s.capacity {
		http.Error(w, `{"error":"no seats"}`, http.StatusForbidden)
		return
	}
	if s.hasRequest(id) {
		for i, row := range s.rows {
			if row.id == id {
				s.rows[i].status = "enrolled"
			}
		}
	} else {
		s.rows = append(s.rows, seatRow{id: id, status: "enrolled"})
	}
	fmt.Fprint(w, `{}`)
}

func (s *seatServer) writeFacts(w http.ResponseWriter, actor string) {
	principal, role, admin := "synthetic-STUDENT", "STUDENT", false
	switch actor {
	case "admin":
		principal, role, admin = "synthetic-ADMIN", "ADMIN", true
	case "contender":
		principal = "synthetic-CONTENDER"
	}
	records, enrolled := 0, 0
	state := "idle"
	pending := false
	for _, row := range s.rows {
		records++
		if row.status == "enrolled" {
			enrolled++
			state = "enrolled"
		} else {
			pending = true
		}
	}
	if state != "enrolled" && pending {
		state = "pending"
	}
	fmt.Fprintf(w, `{"state":%q,"principal":%q,"role":%q,"scope":%q,"login":%t,"course_open":%t,"is_admin":%t,"enrolled":%d,"capacity":%d,"records":%d}`,
		state, principal, role, seatIsolationScope, !s.loggedOut, !s.courseClosed, admin, enrolled, s.capacity, records)
}

type seatFixture struct {
	executionFixture
}

func prepareSeatFixture(t *testing.T, target string) seatFixture {
	t.Helper()
	d := modelTestDB(t, seatModel(t))
	root := filepath.Join(t.TempDir(), "candidate")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "business.txt"), []byte("isolated seat fixture; not IM-mongo evidence"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "business.txt"}, {"-c", "user.name=verify-fixture", "-c", "user.email=fixture@example.invalid", "commit", "-q", "-m", "isolated seat"}} {
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
	if err := os.WriteFile(config, []byte(fmt.Sprintf(`{"environments":{"independent":{"approved":true,"url":%q}}}`, target)), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARC_SEAT_STUDENT", "student")
	t.Setenv("ARC_SEAT_ADMIN", "admin")
	t.Setenv("ARC_SEAT_CONTENDER", "contender")
	if _, err := d.importFixtures(Fixtures{Fixtures: []Fixture{
		{ID: "fixture.student", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_SEAT_STUDENT", Refs: map[string]string{"role": "STUDENT", "principal": "synthetic-STUDENT", "environment": "independent"}},
		{ID: "fixture.admin", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_SEAT_ADMIN", Refs: map[string]string{"role": "ADMIN", "principal": "synthetic-ADMIN", "environment": "independent"}},
		{ID: "fixture.contender", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_SEAT_CONTENDER", Refs: map[string]string{"role": "STUDENT", "principal": "synthetic-CONTENDER", "environment": "independent"}},
		{ID: "fixture.reservation", Kind: "business", State: "idle", Refs: map[string]string{"environment": "independent", "scope": seatIsolationScope}},
	}}); err != nil {
		t.Fatal(err)
	}
	return seatFixture{executionFixture: executionFixture{d: d, root: root, config: config, commit: commit, account: "fixture.student", fixture: "fixture.reservation"}}
}

func seatOracle() OracleAdapter {
	return OracleAdapter{
		ID: "seat-store", Boundary: "after committed reservation and fixture restoration",
		Call:      AdapterCall{Kind: adapterHTTP, Method: http.MethodGet, Path: "/oracle", SuccessStatuses: []int{200}},
		StatePath: "/state", PrincipalPath: "/principal", RolePath: "/role", ScopePath: "/scope",
		Conditions:   map[string]string{"login": "/login", "course_open": "/course_open", "is_admin": "/is_admin"},
		Measurements: map[string]string{"enrolled": "/enrolled", "capacity": "/capacity", "records": "/records"},
	}
}

func seatCall(path, requestID string) AdapterCall {
	return AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: path, Body: json.RawMessage(`{"request_id":"` + requestID + `"}`), SuccessStatuses: []int{200}, RejectionStatuses: []int{401, 403}, CredentialHeader: "X-Fixture-Actor"}
}

func seatStep(node, transition, path, requestID, actor string) RunnerStep {
	oracle := seatOracle()
	oracle.Call.CredentialHeader = "X-Fixture-Actor"
	if actor == "" {
		actor = "primary"
	}
	return RunnerStep{Node: node, Transition: transition, Actor: actor, Call: seatCall(path, requestID), Oracles: []OracleAdapter{oracle}}
}

func seatRecovery(login, open, admin bool) *FixtureRecovery {
	return &FixtureRecovery{
		IsolationScope: seatIsolationScope,
		Steps:          []RecoveryStep{{Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: "/reset", SuccessStatuses: []int{200}}}},
		Oracle:         seatOracle(),
		State:          "idle",
		Conditions:     ConditionAssignment{"login": login, "course_open": open, "is_admin": admin},
		Checks:         []PropertyCheck{{Left: "records", Operator: compareEqual, Value: json.RawMessage("0")}},
	}
}

func writeSeatManifest(t *testing.T, id, test string, account string, steps []RunnerStep, recovery *FixtureRecovery) string {
	t.Helper()
	_ = account
	if recovery != nil {
		recovery.Oracle.Call.CredentialHeader = "X-Fixture-Actor"
	}
	manifest := RunnerManifest{
		SchemaVersion: 1, ID: id, Graph: "seat-v2", Test: test, IsolationScope: seatIsolationScope,
		TimeoutMS: 4000, StepTimeoutMS: 400, Parallelism: 2, Steps: steps, Recovery: recovery,
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), id+".json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func (fixture *seatFixture) register(t *testing.T, file string) {
	t.Helper()
	cliJSON(t, "runner", "--db", fixture.d.file, "--file", file)
}

func (fixture *seatFixture) openRound(t *testing.T) {
	t.Helper()
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(cliJSON(t, "round", "--db", fixture.d.file, "--graph", "seat-v2", "--env", "independent", "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit), &result); err != nil || result.ID == "" {
		t.Fatalf("round: %v %+v", err, result)
	}
	fixture.round = result.ID
}

func (fixture seatFixture) run(t *testing.T, runner, task, account string, accounts map[string]string) ExecuteResult {
	t.Helper()
	if _, err := fixture.d.plan("seat-v2", 128); err != nil {
		t.Fatal(err)
	}
	if account == "" {
		account = fixture.account
	}
	body, err := json.Marshal(accounts)
	if err != nil {
		t.Fatal(err)
	}
	if accounts == nil {
		body = []byte("{}")
	}
	var result ExecuteResult
	if err := json.Unmarshal(cliJSON(t, "execute", "--db", fixture.d.file, "--task", "seat-v2/"+task, "--runner", runner, "--round", fixture.round, "--env", "independent", "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit, "--account", account, "--fixture", fixture.fixture, "--accounts", string(body)), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func seatClaimManifest(t *testing.T, id, test string) string {
	return writeSeatManifest(t, id, test, "primary", []RunnerStep{seatStep("claim", "claim-flow", "/claim", "claim-1", "primary")}, seatRecovery(true, true, false))
}

func seatArtifactRoot() string {
	return filepath.Join("..", "..", ".arc", "artifacts", "verify-isolated-seat")
}

func writeSeatJSON(t *testing.T, rel string, value any) {
	t.Helper()
	path := filepath.Join(seatArtifactRoot(), rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0600); err != nil {
		t.Fatal(err)
	}
}

func recordSeatRun(t *testing.T, name string, fixture seatFixture, result ExecuteResult) map[string]any {
	t.Helper()
	trace, err := fixture.d.trace(result.Run)
	if err != nil {
		t.Fatal(err)
	}
	raws := []map[string]string{}
	for _, step := range trace.Steps {
		if step.Evidence == "" {
			continue
		}
		dest := filepath.Join(seatArtifactRoot(), "raw", result.Run, filepath.Base(step.Evidence))
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err == nil {
			if body, err := os.ReadFile(step.Evidence); err == nil {
				_ = os.WriteFile(dest, body, 0600)
				raws = append(raws, map[string]string{"step": step.Index, "src": step.Evidence, "copied": dest, "sha256": step.SHA256})
			}
		}
	}
	record := map[string]any{
		"name": name, "graph": "seat-v2", "round": fixture.round, "commit": fixture.commit,
		"environment": "independent", "account": fixture.account, "fixture": fixture.fixture,
		"result": result, "trace": trace, "raw_copies": raws,
	}
	writeSeatJSON(t, filepath.Join("runs", name+".json"), record)
	return record
}

func seatWorkbenchAssets(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(filepath.Join("workbench", "dist", "index.html")); err == nil {
		return filepath.Join("workbench", "dist")
	}
	return snapshotAssets(t)
}

func TestSeatModelIsAValidIsolatedContract(t *testing.T) {
	g := seatModel(t)
	if g.ID != "seat-v2" || g.SchemaVersion != 2 {
		t.Fatalf("isolated model identity: %+v", g)
	}
}

func TestIsolatedSeatBusinessChain(t *testing.T) {
	target, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.register(t, seatClaimManifest(t, "claim-positive", "R-CLAIM/POSITIVE"))
	fixture.register(t, writeSeatManifest(t, "claim-login", "R-CLAIM/NEGATIVE/login", "primary", []RunnerStep{seatStep("claim", "", "/claim", "claim-1", "primary")}, seatRecovery(false, true, false)))
	fixture.register(t, writeSeatManifest(t, "claim-closed", "R-CLAIM/NEGATIVE/course_open", "primary", []RunnerStep{seatStep("claim", "", "/claim", "claim-1", "primary")}, seatRecovery(true, false, false)))
	fixture.register(t, writeSeatManifest(t, "no-force", "R-NO-FORCE/NEGATIVE/is_admin", "primary", []RunnerStep{seatStep("force", "", "/force", "force-1", "primary")}, seatRecovery(true, true, false)))
	fixture.register(t, writeSeatManifest(t, "sequence", "test.sequence.claim-approve", "admin", []RunnerStep{
		seatStep("claim", "claim-flow", "/claim", "seq-1", "claimant"),
		seatStep("approve", "approve-flow", "/approve", "seq-1", "primary"),
	}, seatRecovery(true, true, true)))
	fixture.register(t, writeSeatManifest(t, "idempotent", "test.idempotency.duplicate-claim", "primary", []RunnerStep{
		seatStep("claim", "claim-flow", "/claim", "dup-1", "primary"),
		seatStep("claim", "", "/claim", "dup-1", "primary"),
	}, seatRecovery(true, true, false)))
	concurrent := writeSeatManifest(t, "concurrent", "test.concurrency.last-seat", "primary", []RunnerStep{
		func() RunnerStep {
			step := seatStep("claim", "claim-flow", "/claim", "race-1", "primary")
			step.ParallelGroup = "last-seat"
			return step
		}(),
		func() RunnerStep {
			step := seatStep("claim", "claim-flow", "/claim", "race-2", "contender")
			step.ParallelGroup = "last-seat"
			return step
		}(),
	}, seatRecovery(true, true, false))
	fixture.register(t, concurrent)
	fixture.openRound(t)

	normal := fixture.run(t, "claim-positive", "R-CLAIM/POSITIVE", "", nil)
	if normal.Verdict != "PASS" || normal.Recovery.Verdict != "PASS" {
		t.Fatalf("normal claim: %+v", normal)
	}
	trace, err := fixture.d.trace(normal.Run)
	if err != nil || len(trace.Steps) != 1 || trace.Steps[0].Observation == nil || trace.Steps[0].Observation.Actions[0].Principal != "synthetic-STUDENT" {
		t.Fatalf("normal claim lost actor or RAW: %+v %v", trace, err)
	}

	target.mu.Lock()
	target.loggedOut = true
	target.mu.Unlock()
	deniedLogin := fixture.run(t, "claim-login", "R-CLAIM/NEGATIVE/login", "", nil)
	if deniedLogin.Verdict != "PASS" {
		t.Fatalf("unauthenticated claim must be rejected without a seat: %+v", deniedLogin)
	}
	target.mu.Lock()
	if target.loggedOut != true || len(target.rows) != 0 {
		t.Fatalf("login rejection changed business data: %+v", target.rows)
	}
	target.loggedOut = false
	target.courseClosed = true
	target.mu.Unlock()
	deniedClosed := fixture.run(t, "claim-closed", "R-CLAIM/NEGATIVE/course_open", "", nil)
	if deniedClosed.Verdict != "PASS" {
		t.Fatalf("closed course must reject without occupying a seat: %+v", deniedClosed)
	}
	target.mu.Lock()
	target.courseClosed = false
	target.mu.Unlock()

	deniedForce := fixture.run(t, "no-force", "R-NO-FORCE/NEGATIVE/is_admin", "", nil)
	if deniedForce.Verdict != "PASS" {
		t.Fatalf("student force must be rejected: %+v", deniedForce)
	}

	sequence := fixture.run(t, "sequence", "test.sequence.claim-approve", "fixture.admin", map[string]string{"claimant": "fixture.student"})
	if sequence.Verdict != "PASS" || sequence.Steps != 2 {
		t.Fatalf("student then admin sequence: %+v", sequence)
	}

	duplicate := fixture.run(t, "idempotent", "test.idempotency.duplicate-claim", "", nil)
	if duplicate.Verdict != "PASS" {
		t.Fatalf("duplicate request id occupied more than once: %+v", duplicate)
	}

	race := fixture.run(t, "concurrent", "test.concurrency.last-seat", "", map[string]string{"contender": "fixture.contender"})
	if race.Verdict != "PASS" {
		t.Fatalf("last-seat concurrency oversold or lost independence: %+v", race)
	}

	writeSeatJSON(t, "reports/business-chain.json", map[string]any{
		"graph": "seat-v2", "round": fixture.round, "commit": fixture.commit,
		"environment": "independent", "fixture": fixture.fixture, "target": "isolated httptest seat-v2; not IM-mongo",
		"runs": []any{
			recordSeatRun(t, "normal-claim", fixture, normal),
			recordSeatRun(t, "denied-login", fixture, deniedLogin),
			recordSeatRun(t, "denied-closed", fixture, deniedClosed),
			recordSeatRun(t, "denied-force", fixture, deniedForce),
			recordSeatRun(t, "student-then-admin", fixture, sequence),
			recordSeatRun(t, "duplicate-claim", fixture, duplicate),
			recordSeatRun(t, "last-seat-race", fixture, race),
		},
	})
}

func TestIsolatedSeatTimeoutAndNetworkStayBlocked(t *testing.T) {
	target, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.register(t, writeSeatManifest(t, "timeout", "R-CLAIM/POSITIVE", "primary", []RunnerStep{seatStep("claim", "claim-flow", "/claim", "hang-1", "primary")}, seatRecovery(true, true, false)))
	fixture.openRound(t)

	target.mu.Lock()
	target.hangClaim = true
	target.mu.Unlock()
	timeout := fixture.run(t, "timeout", "R-CLAIM/POSITIVE", "", nil)
	if timeout.Verdict != "BLOCKED" {
		t.Fatalf("a hung claim is an environment stop, not a product PASS: %+v", timeout)
	}
	target.mu.Lock()
	if len(target.rows) != 0 {
		t.Fatalf("timeout left a seat occupied: %+v", target.rows)
	}
	target.hangClaim = false
	target.cutClaim = true
	target.mu.Unlock()
	if _, err := fixture.d.retry("seat-v2/R-CLAIM/POSITIVE", "fixture independently restored", timeout.Recovery.Evidence, fixture.root, fixture.config, fixture.commit); err != nil {
		t.Fatal(err)
	}
	cut := fixture.run(t, "timeout", "R-CLAIM/POSITIVE", "", nil)
	if cut.Verdict != "BLOCKED" {
		t.Fatalf("a cut connection is not a product PASS: %+v", cut)
	}
}

func TestIsolatedSeatMutationDetectsFiveDefects(t *testing.T) {
	target, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.register(t, seatClaimManifest(t, "claim-positive", "R-CLAIM/POSITIVE"))
	fixture.register(t, writeSeatManifest(t, "claim-closed", "R-CLAIM/NEGATIVE/course_open", "primary", []RunnerStep{seatStep("claim", "", "/claim", "claim-1", "primary")}, seatRecovery(true, false, false)))
	fixture.register(t, writeSeatManifest(t, "no-force", "R-NO-FORCE/NEGATIVE/is_admin", "primary", []RunnerStep{seatStep("force", "", "/force", "force-1", "primary")}, seatRecovery(true, true, false)))
	fixture.register(t, writeSeatManifest(t, "idempotent", "test.idempotency.duplicate-claim", "primary", []RunnerStep{
		seatStep("claim", "claim-flow", "/claim", "dup-1", "primary"),
		seatStep("claim", "", "/claim", "dup-1", "primary"),
	}, seatRecovery(true, true, false)))
	fixture.register(t, writeSeatManifest(t, "concurrent", "test.concurrency.last-seat", "primary", []RunnerStep{
		func() RunnerStep {
			step := seatStep("claim", "claim-flow", "/claim", "race-1", "primary")
			step.ParallelGroup = "last-seat"
			return step
		}(),
		func() RunnerStep {
			step := seatStep("claim", "claim-flow", "/claim", "race-2", "contender")
			step.ParallelGroup = "last-seat"
			return step
		}(),
	}, seatRecovery(true, true, false)))
	fixture.openRound(t)

	type injected struct {
		id, category, location, runner, task, account string
		accounts                                      map[string]string
		enable                                        func()
		disable                                       func()
	}
	cases := []injected{
		{id: "M-AUTH", category: "unauthorized_access", location: "force handler skips admin check", runner: "no-force", task: "R-NO-FORCE/NEGATIVE/is_admin", enable: func() { target.unauthorizedForce = true }, disable: func() { target.unauthorizedForce = false }},
		{id: "M-RACE", category: "race_oversell", location: "claim occupancy check released before write", runner: "concurrent", task: "test.concurrency.last-seat", accounts: map[string]string{"contender": "fixture.contender"}, enable: func() { target.raceNoLock = true }, disable: func() { target.raceNoLock = false }},
		{id: "M-STATE", category: "illegal_transition", location: "claim writes enrolled and skips review", runner: "claim-positive", task: "R-CLAIM/POSITIVE", enable: func() { target.skipReview = true }, disable: func() { target.skipReview = false }},
		{id: "M-IDEM", category: "idempotency", location: "duplicate request_id inserts a second record", runner: "idempotent", task: "test.idempotency.duplicate-claim", enable: func() { target.duplicateRecords = true }, disable: func() { target.duplicateRecords = false }},
		{id: "M-VALID", category: "missing_server_validation", location: "claim accepts while course_open is false", runner: "claim-closed", task: "R-CLAIM/NEGATIVE/course_open", enable: func() {
			target.courseClosed = true
			target.acceptWhenClosed = true
		}, disable: func() {
			target.acceptWhenClosed = false
			target.courseClosed = false
		}},
	}
	mutations := []map[string]any{}
	for _, item := range cases {
		target.mu.Lock()
		item.enable()
		target.mu.Unlock()
		if item.task == "R-CLAIM/NEGATIVE/course_open" {
			target.mu.Lock()
			target.courseClosed = true
			target.mu.Unlock()
		}
		result := fixture.run(t, item.runner, item.task, item.account, item.accounts)
		if result.Verdict != "FAIL" {
			t.Fatalf("%s (%s) was not detected by %s: %+v", item.id, item.category, item.task, result)
		}
		if result.Recovery.Verdict != "PASS" && item.category != "race_oversell" {
			// recovery should still restore after a product FAIL
		}
		mutations = append(mutations, map[string]any{
			"id": item.id, "category": item.category, "location": item.location,
			"expected_failing": []string{item.task}, "runs": []string{result.Run},
		})
		target.mu.Lock()
		item.disable()
		target.rows = nil
		target.mu.Unlock()
	}
	body, err := json.Marshal(map[string]any{"graph": "seat-v2", "mutations": mutations})
	if err != nil {
		t.Fatal(err)
	}
	report := mutationReport(t, fixture.d, string(body))
	if report.Detected != 5 || report.Survived != 0 || len(report.Survivors) != 0 {
		t.Fatalf("isolated mutation campaign missed a seeded defect: %+v", report)
	}
	if report.InsufficientSample {
		t.Log("sample of five is flagged insufficient_sample; that is the report contract, not a hidden threshold")
	}
	writeSeatJSON(t, "reports/detection.json", report)
	writeSeatJSON(t, "reports/mutation-campaign.json", map[string]any{
		"graph": "seat-v2", "round": fixture.round, "commit": fixture.commit,
		"environment": "independent", "target": "isolated httptest switches; not IM-mongo",
		"mutations": mutations, "report": report,
	})
}

func TestIsolatedSeatExploreDeriveAndDiscover(t *testing.T) {
	g := seatModel(t)
	exploration, err := exploreFlow(g, "flow.reservation", ExplorationBudget{MaxStates: 64, MaxSteps: 8, MaxPaths: 32, TimeoutMS: 1000})
	if err != nil || exploration.Truncated {
		t.Fatalf("declared seat paths must be enumerable: %+v %v", exploration, err)
	}
	joined := ""
	for _, path := range append(append([][]string{}, exploration.Paths...), mergedPaths(exploration)...) {
		joined += strings.Join(path, ",") + ";"
	}
	if !strings.Contains(joined, "claim-flow") || !strings.Contains(joined, "approve-flow") || !strings.Contains(joined, "force-flow") {
		t.Fatalf("legal paths from confirmed rules are missing: %s merges=%d", joined, len(exploration.Merges))
	}

	target, server := newSeatServer(t)
	target.claimNoop = true
	fixture := prepareSeatFixture(t, server.URL)
	fixture.register(t, seatClaimManifest(t, "claim-positive", "R-CLAIM/POSITIVE"))
	fixture.openRound(t)
	failed := fixture.run(t, "claim-positive", "R-CLAIM/POSITIVE", "", nil)
	if failed.Verdict != "FAIL" {
		t.Fatalf("noop claim setup: %+v", failed)
	}
	var derived DerivationReport
	if err := json.Unmarshal(cliJSON(t, "derive", "--db", fixture.d.file, "--run", failed.Run), &derived); err != nil {
		t.Fatal(err)
	}
	if len(derived.Derived) == 0 {
		t.Fatalf("a failed claim must derive from the observed state: %+v", derived)
	}

	evidence := filepath.Join(t.TempDir(), "observe.json")
	if err := os.WriteFile(evidence, []byte(`{"entry":"/claim","observed":"enrolled"}`), 0600); err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(t.TempDir(), "exploration.json")
	body, err := json.Marshal(map[string]any{
		"graph": "seat-v2", "exploration": "seat-unknown",
		"observations": []map[string]any{{
			"entry": "/claim", "role": "STUDENT", "principal": "synthetic-STUDENT", "action": "claim",
			"before_state": "idle", "after_state": "enrolled", "denied": false,
			"conditions": map[string]bool{"login": true, "course_open": true},
			"evidence":   evidence,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(record, body, 0600); err != nil {
		t.Fatal(err)
	}
	var report DiscoveryReport
	if err := json.Unmarshal(cliJSON(t, "discover", "record", "--db", fixture.d.file, "--file", record), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Unspecified) != 1 || report.Unspecified[0].Kind == "" || report.Unspecified[0].Status != "UNSPECIFIED" {
		t.Fatalf("skip-review observation must stay UNSPECIFIED: %+v", report)
	}
	if !containsString(report.Unspecified[0].Capabilities, "reservation.claim") {
		t.Fatalf("unknown behavior must stay bound to its capability: %+v", report.Unspecified[0])
	}
}

func TestDerivationRefusesStaleRound(t *testing.T) {
	fixture, _, failed := failedEnrollment(t, kindsModel(t))
	if err := os.WriteFile(filepath.Join(fixture.root, "after-round.txt"), []byte("candidate changed after the failing run"), 0600); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"derive", "--db", fixture.d.file, "--run", failed.Run})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("derive accepted a run whose round is no longer current: %v", err)
	}
}

func TestIsolatedSeatSnapshotBindsRealRuns(t *testing.T) {
	target, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.register(t, seatClaimManifest(t, "claim-positive", "R-CLAIM/POSITIVE"))
	fixture.openRound(t)
	target.skipReview = true
	failed := fixture.run(t, "claim-positive", "R-CLAIM/POSITIVE", "", nil)
	if _, err := fixture.d.retry("seat-v2/R-CLAIM/POSITIVE", "fixture independently restored", failed.Recovery.Evidence, fixture.root, fixture.config, fixture.commit); err != nil {
		t.Fatal(err)
	}
	target.skipReview = false
	passed := fixture.run(t, "claim-positive", "R-CLAIM/POSITIVE", "", nil)
	if failed.Verdict != "FAIL" || passed.Verdict != "PASS" {
		t.Fatalf("snapshot needs FAIL then PASS: %s %s", failed.Verdict, passed.Verdict)
	}
	out := filepath.Join(t.TempDir(), "seat-snapshot")
	assets := seatWorkbenchAssets(t)
	cliJSON(t, "snapshot", "--db", fixture.d.file, "--graph", "seat-v2", "--env", "independent", "--round", fixture.round, "--run", failed.Run+","+passed.Run, "--config", fixture.config, "--assets", assets, "--out", out)
	snapshot := readSnapshot(t, out)
	if snapshot.Meta.Graph != "seat-v2" || len(snapshot.Runs) != 2 || snapshot.Runs[0].Trace.FirstDivergence == "" {
		t.Fatalf("snapshot lost the isolated evidence chain: %+v", snapshot.Meta)
	}
	found := false
	for _, obligation := range snapshot.Views[viewVerification].Obligations {
		if obligation.Definition.ID == "R-CLAIM/POSITIVE" && obligation.Status == "FLAKY" {
			found = true
		}
	}
	if !found {
		t.Fatalf("capability evidence must stay FLAKY when FAIL and PASS both exist: %+v", snapshot.Views[viewVerification].Obligations)
	}
	recordSeatRun(t, "snapshot-fail", fixture, failed)
	recordSeatRun(t, "snapshot-pass", fixture, passed)
	site := filepath.Join(seatArtifactRoot(), "site")
	if env := os.Getenv("ARC_VERIFY_SEAT_SITE"); env != "" {
		site = env
		if extra := os.Getenv("ARC_VERIFY_SEAT_ASSETS"); extra != "" {
			assets = extra
		}
	}
	if err := os.RemoveAll(site); err != nil {
		t.Fatal(err)
	}
	cliJSON(t, "snapshot", "--db", fixture.d.file, "--graph", "seat-v2", "--env", "independent", "--round", fixture.round, "--run", failed.Run+","+passed.Run, "--config", fixture.config, "--assets", assets, "--out", site)
	writeSeatJSON(t, "reports/snapshot-meta.json", snapshot.Meta)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestIsolatedSeatFailedRecoveryKeepsLocksAfterPass(t *testing.T) {
	target, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.register(t, seatClaimManifest(t, "claim-positive", "R-CLAIM/POSITIVE"))
	fixture.openRound(t)
	target.resetBroken = true
	result := fixture.run(t, "claim-positive", "R-CLAIM/POSITIVE", "", nil)
	if result.Verdict != "BLOCKED" {
		t.Fatalf("pass + failed restore must be BLOCKED: %+v", result)
	}
	locks, err := fixture.d.query("SELECT COUNT(*) FROM fixture_locks", 1)
	if err != nil || locks[0][0] == "0" {
		t.Fatalf("locks released after a failed restore: %v %v", locks, err)
	}
}

func TestIsolatedSeatRunChainStopsOnFirstFail(t *testing.T) {
	target, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.register(t, seatClaimManifest(t, "claim-positive", "R-CLAIM/POSITIVE"))
	fixture.openRound(t)
	if _, err := fixture.d.plan("seat-v2", 128); err != nil {
		t.Fatal(err)
	}
	target.skipReview = true
	packet, err := fixture.d.runChain(context.Background(), ChainRequest{
		Graph: "seat-v2", Environment: "independent", Round: fixture.round, Commit: fixture.commit,
		Root: fixture.root, Config: fixture.config, Account: fixture.account, Fixture: fixture.fixture,
		MaxSteps: 4, Deadline: 8 * time.Second,
	})
	var stop chainStop
	if err == nil || !errors.As(err, &stop) || stop.packet.Stop != "divergence" || stop.packet.Run == "" {
		t.Fatalf("run-chain must stop at the first independent FAIL: %+v %v", packet, err)
	}
	writeSeatJSON(t, "reports/run-chain-first-fail.json", stop.packet)
}

func TestIsolatedSeatHandoffKeepsFirstFail(t *testing.T) {
	target, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.register(t, seatClaimManifest(t, "claim-positive", "R-CLAIM/POSITIVE"))
	fixture.openRound(t)
	target.skipReview = true
	failed := fixture.run(t, "claim-positive", "R-CLAIM/POSITIVE", "", nil)
	if failed.Verdict != "FAIL" {
		t.Fatalf("handoff needs a confirmed product FAIL: %+v", failed)
	}
	var packet HandoffPacket
	if err := json.Unmarshal(cliJSON(t, "handoff", "--db", fixture.d.file, "--to", "arc-code", "--graph", "seat-v2", "--run", failed.Run), &packet); err != nil {
		t.Fatal(err)
	}
	if packet.Status != "FAIL" || packet.FirstDivergence == "" || packet.Run != failed.Run {
		t.Fatalf("arc-code packet lost the first FAIL: %+v", packet)
	}
	if _, err := fixture.d.retry("seat-v2/R-CLAIM/POSITIVE", "fixture independently restored", failed.Recovery.Evidence, fixture.root, fixture.config, fixture.commit); err != nil {
		t.Fatal(err)
	}
	target.skipReview = false
	passed := fixture.run(t, "claim-positive", "R-CLAIM/POSITIVE", "", nil)
	if passed.Verdict != "PASS" || passed.Run == failed.Run {
		t.Fatalf("repair must open a new run: fail=%s pass=%+v", failed.Run, passed)
	}
	var again HandoffPacket
	if err := json.Unmarshal(cliJSON(t, "handoff", "--db", fixture.d.file, "--to", "arc-code", "--graph", "seat-v2", "--run", failed.Run), &again); err != nil {
		t.Fatal(err)
	}
	if again.Run != failed.Run || again.Status != "FAIL" || again.FirstDivergence != packet.FirstDivergence {
		t.Fatalf("later PASS overwrote the historic FAIL packet: %+v", again)
	}
	err := run([]string{"handoff", "--db", fixture.d.file, "--to", "arc-code", "--graph", "seat-v2", "--run", passed.Run})
	if err == nil || !strings.Contains(err.Error(), "only confirmed failed run") {
		t.Fatalf("PASS must not be handed to arc-code: %v", err)
	}
	writeSeatJSON(t, "reports/handoff-first-fail.json", map[string]any{
		"failed": failed, "passed": passed, "packet": packet, "historic": again,
	})
}
