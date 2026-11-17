package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func runRound3ParallelInvariant(t *testing.T, typed bool, offender string, reverse bool) (executionFixture, ExecuteResult) {
	t.Helper()
	g := referenceModel(t)
	if typed {
		g = contractModel(t)
		for i := range g.BusinessContract.Branches {
			g.BusinessContract.Branches[i].Flow = ""
		}
		g.BusinessContract.Coverage.Scopes[0].Flow = ""
	}
	g.Tests[0].Kind = testConcurrency
	g.Tests[0].Actors = []ActorRequirement{{Alias: "contender", Role: "STUDENT"}}
	var mu sync.Mutex
	active, requests := false, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/enroll":
			active, requests = true, requests+1
			fmt.Fprint(w, `{}`)
		case "/reset":
			active = false
			fmt.Fprint(w, `{}`)
		default:
			actor, principal := "primary", "synthetic-STUDENT"
			if r.Header.Get("X-Fixture-Actor") == "contender" {
				actor, principal = "contender", "synthetic-CONTENDER"
			}
			state, count := "pending", 0
			if active {
				state, count = "ready", 1
				if actor == offender {
					count = 3
				}
			}
			fmt.Fprintf(w, `{"state":%q,"principal":%q,"role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d,"capacity":2}`, state, principal, count)
		}
	}))
	t.Cleanup(server.Close)
	f := prepareExecutionFixture(t, g, server.URL)
	t.Setenv("ARC_ROUND3_CONTENDER", "contender")
	if _, err := f.d.importFixtures(Fixtures{Fixtures: []Fixture{{ID: "fixture.contender", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_ROUND3_CONTENDER", Refs: map[string]string{"role": "STUDENT", "principal": "synthetic-CONTENDER", "environment": "independent"}}}}); err != nil {
		t.Fatal(err)
	}
	const runnerID = "round3-parallel-invariants"
	file := rewriteFixtureManifest(t, runnerID, "test.idempotency", func(m *RunnerManifest) {
		first := m.Steps[0]
		first.ParallelGroup = "two-subjects"
		if typed {
			first.BusinessBranch = "accept"
			first.SubjectBindings = map[string]string{"primary": "actor.primary.principal"}
			first.Oracles[0].Measurements["primary.active"] = "/login"
		}
		second := first
		second.Actor = "contender"
		second.Call.CredentialHeader = "X-Fixture-Actor"
		second.Oracles = append([]OracleAdapter(nil), first.Oracles...)
		second.Oracles[0].Call.CredentialHeader = "X-Fixture-Actor"
		if typed {
			second.SubjectBindings = map[string]string{"primary": "actor.contender.principal"}
		}
		m.Steps = []RunnerStep{first, second}
		if reverse {
			m.Steps[0], m.Steps[1] = m.Steps[1], m.Steps[0]
		}
	})
	f.registerAndRound(t, file)
	if _, err := f.d.plan(g.ID, 128); err != nil {
		t.Fatal(err)
	}
	result, err := f.d.execute(context.Background(), ExecuteRequest{Task: g.ID + "/test.idempotency", Runner: runnerID, Round: f.round, Environment: "independent", Commit: f.commit, Root: f.root, Config: f.config, Account: f.account, Fixture: f.fixture, Accounts: map[string]string{"contender": "fixture.contender"}})
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	count := requests
	mu.Unlock()
	if count != 2 || result.Recovery.Verdict != "PASS" {
		t.Fatalf("control must execute both real actions and restore: requests=%d result=%+v", count, result)
	}
	return f, result
}

func TestRuntimeRound3ParallelInvariantsCoverEveryParticipant(t *testing.T) {
	for _, typed := range []bool{false, true} {
		for _, c := range []struct {
			name, offender string
			reverse        bool
		}{
			{"healthy", "", false},
			{"first_primary_violates", "primary", false},
			{"first_contender_violates", "contender", true},
			{"last_contender_violates", "contender", false},
		} {
			t.Run(fmt.Sprintf("typed=%t/%s", typed, c.name), func(t *testing.T) {
				f, result := runRound3ParallelInvariant(t, typed, c.offender, c.reverse)
				want := "PASS"
				if c.offender != "" {
					want = "FAIL"
				}
				if result.Verdict != want {
					t.Fatalf("participant's independently observed capacity violation disappeared: offender=%s want=%s result=%+v", c.offender, want, result)
				}
				raw, hash, err := f.d.workbenchEvidence(result.Run, "action", 1)
				if err != nil {
					t.Fatal(err)
				}
				contract, err := f.d.runTaskContract(result.Run)
				if err != nil {
					t.Fatal(err)
				}
				verdict, _, assertions, err := f.d.evaluateModelObservation(contract, raw, 1, hash)
				if err != nil || verdict != want {
					t.Fatalf("offline model verdict disagrees: %s %v", verdict, err)
				}
				actors := map[string]string{}
				for _, assertion := range assertions {
					if assertion.Kind == "invariant" {
						actors[assertion.Subject] = assertion.Verdict
					}
				}
				if len(actors) != 2 || c.offender != "" && actors[c.offender] != "FAIL" {
					t.Fatalf("invariant assertions omit an executed subject: %+v", actors)
				}
				if evidence, err := f.d.verifiedRunFacts(result.Run); err != nil || evidence.EvidenceStatus != "PASS" {
					t.Fatalf("observed failure must remain valid evidence: %+v %v", evidence, err)
				}
			})
		}
	}
}

func TestRuntimeRound3HistoricalInvariantCannotBorrowHealthyFinalParticipant(t *testing.T) {
	f, result := runRound3ParallelInvariant(t, false, "", false)
	if result.Verdict != "PASS" {
		t.Fatalf("control run: %+v", result)
	}
	raw, _, err := f.d.workbenchEvidence(result.Run, "action", 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := range raw.Model.Oracles {
		if raw.Model.Oracles[i].Actor == "primary" {
			raw.Model.Oracles[i].After.Values["enrolled"] = json.RawMessage(`3`)
		}
	}
	hash := replaceSuccessorHistoricalCapture(t, f.d, raw)
	contract, err := f.d.runTaskContract(result.Run)
	if err != nil {
		t.Fatal(err)
	}
	verdict, reason, _, err := f.d.evaluateModelObservation(contract, raw, 1, hash)
	if err != nil || verdict != "FAIL" {
		t.Fatalf("valid-hash history skipped the first participant invariant: verdict=%s reason=%s err=%v", verdict, reason, err)
	}
	if err := f.d.qualifyModelRun(result.Run); err == nil {
		t.Fatal("historical invariant violation still qualifies as PASS")
	}
}

func TestRuntimeRound3RepeatedActorInvariantsKeepActionIndex(t *testing.T) {
	for _, badFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("first_violates=%t", badFirst), func(t *testing.T) {
			var mu sync.Mutex
			active, calls := false, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/enroll":
					active, calls = true, calls+1
					fmt.Fprint(w, `{}`)
				case "/reset":
					active = false
					fmt.Fprint(w, `{}`)
				default:
					state, count := "pending", 0
					if active {
						state, count = "ready", 1
						if badFirst && r.URL.Path == "/oracle-first" {
							count = 3
						}
					}
					fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d,"capacity":2}`, state, count)
				}
			}))
			t.Cleanup(server.Close)
			f := prepareExecutionFixture(t, referenceModel(t), server.URL)
			const runner = "round3-repeated-actor-invariants"
			file := rewriteFixtureManifest(t, runner, "test.idempotency", func(m *RunnerManifest) {
				first := m.Steps[0]
				first.ParallelGroup = "same-principal"
				first.Oracles[0].Call.Path = "/oracle-first"
				second := first
				second.Oracles = append([]OracleAdapter(nil), first.Oracles...)
				second.Oracles[0].Call.Path = "/oracle-second"
				m.Steps = []RunnerStep{first, second}
			})
			f.registerAndRound(t, file)
			if _, err := f.d.plan("course-v2", 128); err != nil {
				t.Fatal(err)
			}
			result, err := f.d.execute(context.Background(), ExecuteRequest{Task: "course-v2/test.idempotency", Runner: runner, Round: f.round, Environment: "independent", Commit: f.commit, Root: f.root, Config: f.config, Account: f.account, Fixture: f.fixture})
			want := "PASS"
			if badFirst {
				want = "FAIL"
			}
			mu.Lock()
			attempts := calls
			mu.Unlock()
			if err != nil || attempts != 2 || result.Verdict != want || result.Recovery.Verdict != "PASS" {
				t.Fatalf("first action borrowed the final action's invariant: want=%s calls=%d result=%+v err=%v", want, attempts, result, err)
			}
			raw, hash, err := f.d.workbenchEvidence(result.Run, "action", 1)
			if err != nil {
				t.Fatal(err)
			}
			contract, err := f.d.runTaskContract(result.Run)
			if err != nil {
				t.Fatal(err)
			}
			verdict, _, assertions, err := f.d.evaluateModelObservation(contract, raw, 1, hash)
			var values []string
			for _, assertion := range assertions {
				if assertion.Kind == "invariant" {
					values = append(values, assertion.Actual)
				}
			}
			first := "1"
			if badFirst {
				first = "3"
			}
			if err != nil || verdict != want || len(values) != 2 || values[0] != first || values[1] != "1" {
				t.Fatalf("same-name readers were not kept at their action indices: verdict=%s values=%v err=%v", verdict, values, err)
			}
		})
	}
}
