package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// chainTarget is a synthetic course target; each field reproduces one stop reason.
type chainTarget struct {
	mu              sync.Mutex
	enrolled        int
	brokenCapacity  bool
	unknownState    bool
	loggedOut       bool
	blockEnroll     bool
	enrollCancelled bool
	enrollCalls     int
}

func newChainTarget(t *testing.T) (*chainTarget, *httptest.Server) {
	t.Helper()
	target := &chainTarget{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/enroll" {
			target.mu.Lock()
			target.enrollCalls++
			block := target.blockEnroll
			target.mu.Unlock()
			if block {
				<-r.Context().Done()
				target.mu.Lock()
				target.enrollCancelled = true
				target.mu.Unlock()
				return
			}
		}
		target.mu.Lock()
		defer target.mu.Unlock()
		switch r.URL.Path {
		case "/enroll":
			target.enrolled = 1
			fmt.Fprint(w, `{}`)
		case "/reset":
			target.enrolled = 0
			fmt.Fprint(w, `{}`)
		default:
			state, capacity := "pending", 1
			if target.enrolled > 0 {
				state = "ready"
				if target.unknownState {
					state = "mystery"
				}
				if target.brokenCapacity {
					capacity = 0
				}
			}
			fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":%t,"enrolled":%d,"capacity":%d}`, state, !target.loggedOut, target.enrolled, capacity)
		}
	}))
	t.Cleanup(server.Close)
	return target, server
}

// chainFixture registers runners for R1/POSITIVE and transition/submit-flow and opens a round.
func chainFixture(t *testing.T, server *httptest.Server, change func(*RunnerManifest)) executionFixture {
	t.Helper()
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	positive := rewriteFixtureManifest(t, "chain-positive", "R1/POSITIVE", change)
	transition := rewriteFixtureManifest(t, "chain-transition", "transition/submit-flow", func(m *RunnerManifest) {
		m.Steps[0].Transition = "submit-flow"
		change(m)
	})
	cliJSON(t, "runner", "--db", fixture.d.file, "--file", transition)
	fixture.registerAndRound(t, positive)
	if _, err := fixture.d.plan("course-v2", 128); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func chainRequest(fixture executionFixture, deadline time.Duration) ChainRequest {
	return ChainRequest{Graph: "course-v2", Environment: "independent", Round: fixture.round, Commit: fixture.commit, Root: fixture.root, Config: fixture.config, Account: fixture.account, Fixture: fixture.fixture, MaxSteps: 16, Deadline: deadline}
}

func runChainStop(t *testing.T, fixture executionFixture, deadline time.Duration) (ChainPacket, *chainStop) {
	t.Helper()
	packet, err := fixture.d.runChain(context.Background(), chainRequest(fixture, deadline))
	var stop chainStop
	if err == nil {
		return packet, nil
	}
	if !errors.As(err, &stop) {
		t.Fatalf("run-chain failed instead of stopping with a context packet: %v", err)
	}
	return stop.packet, &stop
}

func noChange(*RunnerManifest) {}

func TestRunChainRunsKnownPathAndWritesRAWPerStep(t *testing.T) {
	_, server := newChainTarget(t)
	fixture := chainFixture(t, server, noChange)
	packet, stop := runChainStop(t, fixture, time.Minute)
	if stop != nil || packet.Stop != "completed" || len(packet.Steps) != 2 {
		t.Fatalf("known path did not run to completion: %+v", packet)
	}
	for _, step := range packet.Steps {
		if step.Verdict != "PASS" || step.Run == "" {
			t.Fatalf("chain step lacks its own run: %+v", step)
		}
		trace, err := fixture.d.trace(step.Run)
		if err != nil || len(trace.Steps) == 0 || trace.EvidenceStatus != "PASS" {
			t.Fatalf("chain step %s has no verified RAW: %+v %v", step.Task, trace, err)
		}
		for _, item := range trace.Steps {
			if item.EvidenceStatus != "PASS" || len(item.SHA256) != 64 {
				t.Fatalf("chain step %s skipped independent RAW: %+v", step.Task, item)
			}
		}
	}
	if len(packet.Unautomated) == 0 {
		t.Fatal("ready obligations without a runner must be listed, not silently skipped")
	}
}

func TestRunChainStopsOnDivergence(t *testing.T) {
	target, server := newChainTarget(t)
	target.brokenCapacity = true
	fixture := chainFixture(t, server, noChange)
	packet, stop := runChainStop(t, fixture, time.Minute)
	if stop == nil || stop.code != chainExitDivergence || packet.Stop != "divergence" || len(packet.Steps) != 1 {
		t.Fatalf("divergence must stop the chain at its first card: %+v %+v", packet, stop)
	}
	if packet.Divergence == nil || packet.Divergence.Verdict != "FAIL" || len(packet.Divergence.SHA256) != 64 || packet.Run == "" {
		t.Fatalf("stop packet lacks the divergent step and its RAW: %+v", packet)
	}
	target.mu.Lock()
	calls := target.enrollCalls
	target.mu.Unlock()
	if calls != 1 {
		t.Fatalf("cards after the divergence were executed: %d enroll calls", calls)
	}
}

func TestRunChainStopsOnUnknownState(t *testing.T) {
	target, server := newChainTarget(t)
	target.unknownState = true
	fixture := chainFixture(t, server, noChange)
	packet, stop := runChainStop(t, fixture, time.Minute)
	if stop == nil || stop.code != chainExitUnknownState || packet.Stop != "unknown_state" || packet.State != "mystery" {
		t.Fatalf("an unmodelled observed state must stop the chain: %+v %+v", packet, stop)
	}
}

func TestRunChainStopsOnUnmetPrecondition(t *testing.T) {
	target, server := newChainTarget(t)
	target.loggedOut = true
	fixture := chainFixture(t, server, func(m *RunnerManifest) { m.Recovery.Conditions = map[string]bool{"login": false} })
	packet, stop := runChainStop(t, fixture, time.Minute)
	if stop == nil || stop.code != chainExitPrecondition || packet.Stop != "precondition" {
		t.Fatalf("an unmet precondition must stop before any guess: %+v %+v", packet, stop)
	}
	if packet.State != "pending" || strings.Join(packet.Candidates, ",") != "submit-flow" {
		t.Fatalf("stop packet must name the observed state and its declared next transitions: %+v", packet)
	}
	target.mu.Lock()
	calls := target.enrollCalls
	target.mu.Unlock()
	if calls != 0 {
		t.Fatalf("the action ran despite unmet preconditions: %d calls", calls)
	}
}

func TestRunChainDeadlineStopsActionAndKeepsRAW(t *testing.T) {
	target, server := newChainTarget(t)
	target.blockEnroll = true
	fixture := chainFixture(t, server, func(m *RunnerManifest) { m.TimeoutMS, m.StepTimeoutMS = 6000, 3000 })
	started := time.Now()
	packet, stop := runChainStop(t, fixture, 300*time.Millisecond)
	if stop == nil || stop.code != chainExitBudget || packet.Stop != "budget" {
		t.Fatalf("the chain deadline must stop it: %+v %+v", packet, stop)
	}
	if elapsed := time.Since(started); elapsed > 2500*time.Millisecond {
		t.Fatalf("the deadline did not interrupt the running action: %s", elapsed)
	}
	target.mu.Lock()
	cancelled := target.enrollCancelled
	target.mu.Unlock()
	if !cancelled {
		t.Fatal("the in-flight business action was not cancelled")
	}
	if packet.Run == "" {
		t.Fatal("the interrupted card left no run")
	}
	record, err := fixture.d.modelRun(packet.Run)
	if err != nil || record.SetupEvidence == "" || record.Recovery.Evidence == "" {
		t.Fatalf("the interrupted card lost its setup or recovery RAW: %+v %v", record, err)
	}
}

func TestRunChainContextPacketMinimal(t *testing.T) {
	target, server := newChainTarget(t)
	target.brokenCapacity = true
	fixture := chainFixture(t, server, noChange)
	packet, _ := runChainStop(t, fixture, time.Minute)
	body, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > 8<<10 {
		t.Fatalf("stop packet is not minimal: %d bytes", len(body))
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(body, &keys); err != nil {
		t.Fatal(err)
	}
	for key := range keys {
		if !strings.Contains(" graph round stop reason steps task run state divergence candidates remaining unautomated ", " "+key+" ") {
			t.Fatalf("stop packet carries an unexpected field %q", key)
		}
	}
	if strings.Contains(string(body), "synthetic-STUDENT") {
		t.Fatal("stop packet copies RAW content instead of referencing it")
	}
	if strings.Join(packet.Remaining, ",") != "course-v2/transition/submit-flow" || packet.State != "ready" || len(packet.Candidates) != 0 {
		t.Fatalf("stop packet must name the unexecuted cards and the (terminal) observed state: %+v", packet)
	}
}

func TestRunChainCLIExitCodeNamesStop(t *testing.T) {
	target, server := newChainTarget(t)
	target.brokenCapacity = true
	fixture := chainFixture(t, server, noChange)
	args := []string{"run-chain", "--db", fixture.d.file, "--graph", "course-v2", "--env", "independent", "--round", fixture.round, "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit, "--account", fixture.account, "--fixture", fixture.fixture, "--max-steps", "8", "--deadline", "1m"}
	err := run(args)
	var stop chainStop
	if !errors.As(err, &stop) || stop.code != chainExitDivergence {
		t.Fatalf("CLI must return the divergence stop: %v", err)
	}
	if err := run([]string{"run-chain", "--db", fixture.d.file, "--graph", "course-v2", "--round", fixture.round, "--max-steps", "0", "--deadline", "1m"}); err == nil || errors.As(err, &stop) {
		t.Fatalf("an unbounded chain must be refused before running: %v", err)
	}
}

func TestRunChainStepBudgetStopsAndNamesRemainingCards(t *testing.T) {
	_, server := newChainTarget(t)
	fixture := chainFixture(t, server, noChange)
	request := chainRequest(fixture, time.Minute)
	request.MaxSteps = 1
	packet, err := fixture.d.runChain(context.Background(), request)
	var stop chainStop
	if !errors.As(err, &stop) || stop.code != chainExitBudget || len(packet.Steps) != 1 || len(packet.Remaining) != 1 {
		t.Fatalf("the step budget must stop the chain and name the unexecuted card: %+v %v", packet, err)
	}
}

func TestRunChainExitCodes(t *testing.T) {
	for code, err := range map[int]error{
		chainExitDivergence: chainStop{code: chainExitDivergence},
		chainExitBudget:     fmt.Errorf("wrapped: %w", chainStop{code: chainExitBudget}),
		1:                   errors.New("ordinary failure"),
	} {
		if got := exitCode(err); got != code {
			t.Fatalf("exit code for %v = %d, want %d", err, got, code)
		}
	}
}

func TestRunChainWithoutRunnableCardsIsBlocked(t *testing.T) {
	_, server := newChainTarget(t)
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	fixture.openRound(t)
	if _, err := fixture.d.plan("course-v2", 128); err != nil {
		t.Fatal(err)
	}
	packet, stop := runChainStop(t, fixture, time.Minute)
	if stop == nil || stop.code != chainExitBlocked || packet.Stop != "blocked" || !strings.Contains(packet.Reason, "runner") || len(packet.Unautomated) == 0 {
		t.Fatalf("a chain that can run nothing must not report completion: %+v %+v", packet, stop)
	}
}

// TestRunChainBuiltBinaryExitStatus runs the real process. Only import and the
// human approval happen in-process (an agent cannot approve); runner
// registration, round, plan and run-chain all use the built binary, because
// runners bind the executable that registered them.
func TestRunChainBuiltBinaryExitStatus(t *testing.T) {
	binary := os.Getenv("ARC_VERIFY_CHAIN_BINARY")
	if binary == "" {
		t.Skip("set ARC_VERIFY_CHAIN_BINARY to a built arc-verify to check the real exit status")
	}
	target, server := newChainTarget(t)
	target.brokenCapacity = true
	fixture := prepareExecutionFixture(t, referenceModel(t), server.URL)
	if err := fixture.d.close(); err != nil {
		t.Fatal(err)
	}
	cli := func(arguments ...string) ([]byte, error) {
		command := exec.Command(binary, arguments...)
		var stderr strings.Builder
		command.Stderr = &stderr
		output, err := command.Output()
		if err != nil {
			return output, fmt.Errorf("%w: %s", err, stderr.String())
		}
		return output, nil
	}
	for _, manifest := range []string{
		rewriteFixtureManifest(t, "bin-positive", "R1/POSITIVE", noChange),
		rewriteFixtureManifest(t, "bin-transition", "transition/submit-flow", func(m *RunnerManifest) { m.Steps[0].Transition = "submit-flow" }),
	} {
		if output, err := cli("runner", "--db", fixture.d.file, "--file", manifest); err != nil {
			t.Fatalf("built CLI runner registration: %v %s", err, output)
		}
	}
	output, err := cli("round", "--db", fixture.d.file, "--graph", "course-v2", "--env", "independent", "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit)
	var round struct{ ID string }
	if err != nil || json.Unmarshal(output, &round) != nil || round.ID == "" {
		t.Fatalf("built CLI round: %v %s", err, output)
	}
	if output, err := cli("plan", "--db", fixture.d.file, "--graph", "course-v2"); err != nil {
		t.Fatalf("built CLI plan: %v %s", err, output)
	}
	output, err = cli("run-chain", "--db", fixture.d.file, "--graph", "course-v2", "--env", "independent", "--round", round.ID, "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit, "--account", fixture.account, "--fixture", fixture.fixture, "--max-steps", "8", "--deadline", "1m")
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != chainExitDivergence || !strings.Contains(string(output), `"stop": "divergence"`) {
		t.Fatalf("built CLI did not exit %d with its packet: %v %s", chainExitDivergence, err, output)
	}
	t.Logf("built CLI exit status %d; packet:\n%s", exit.ExitCode(), output)
}
