package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// This isolated audit regression describes a safe successor intersection.
// Both explicitly allowed outcomes fit the same real response, but there is
// no action their continuation contracts both permit.
func TestBusinessSuccessorCampaignDisjointOutcomesBlockAllNextActions(t *testing.T) {
	target, server := newCampaignStateTarget(t, true, false)
	g := campaignCycleGraph(t, false)
	// A legal, explicitly sourced non-target operation lets both continuation
	// sets exclude clear. Even a union interpretation must therefore reject it.
	g.Nodes = append(g.Nodes, Node{ID: "query", Kind: "action", Name: "只读查询隔离报名实例"})
	g.Edges = append(g.Edges,
		Edge{ID: "requires-query-login", From: "login", To: "query", Kind: "requires"},
		Edge{ID: "implements-query", From: "enrollment.submit", To: "query", Kind: "implements"},
		Edge{ID: "queries-ready", From: "query", To: "ready", Kind: "transitions"},
		Edge{ID: "query-flow", From: "ready", To: "ready", Kind: "transitions_to"})
	g.Rules = append(g.Rules, Rule{ID: "R3", Action: "query", Actor: "STUDENT", Entry: "/query", Oracle: "fixture-independent-store", Conditions: []string{"login"}, Expected: "ready", Rejected: "UNCHANGED", Source: "document", Description: "Synthetic audit contract: query leaves the isolated stored enrollment intact"})
	g.Transitions = append(g.Transitions, TransitionContract{Edge: "query-flow", Action: "query", Rule: "R3", Preconditions: ConditionAssignment{"login": true}, Outcome: branchSuccess})
	g.BusinessContract.Branches = append(g.BusinessContract.Branches, BusinessBranch{ID: "query-stored", Action: "query", Rule: "R3", Flow: "flow.enrollment", Transition: "query-flow", FromStates: []string{"ready"}, When: businessPredicate("eq", "account", "active", "boolean", "true"), Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{{ID: "still-stored", State: "ready", Denied: businessBool(false), Predicate: businessPredicate("eq", "order", "stored", "boolean", "true")}}}, Source: "document", SourceRef: "synthetic-audit:query-does-not-mutate", Confirmed: true})
	for i := range g.BusinessContract.Branches {
		branch := &g.BusinessContract.Branches[i]
		if branch.ID != "accept" {
			continue
		}
		branch.Expected.Mode = "allowed_outcomes"
		branch.Expected.Reason = "Synthetic confirmed outcomes overlap in observable poststate but permit disjoint next actions"
		branch.Expected.Outcomes[0].NextAllowed = []string{"enroll"}
		second := branch.Expected.Outcomes[0]
		second.ID = "accepted-restrict-query"
		second.NextAllowed = []string{"query"}
		branch.Expected.Outcomes = append(branch.Expected.Outcomes, second)
	}
	if err := ValidateBusinessContract(g); err != nil {
		t.Fatal(err)
	}
	f, request := newCampaignTestFixture(t, g, server.URL,
		campaignCheckpointManifest(t, "enroll", "accept", false),
		campaignCheckpointManifest(t, "clear", "clear-active", false))
	result, err := f.d.runBusinessCampaign(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	loaded, loadErr := f.d.loadBusinessCampaign(result.ID)
	t.Logf("status=%s actions=%s visits=%d first-comparison=%+v reload-evidence=%s reload-error=%v", result.Status, strings.Join(target.actions, ","), len(result.Visits), result.Visits[0].Comparison, loaded.EvidenceStatus, loadErr)
	if len(target.actions) != 1 || target.actions[0] != "enroll" {
		t.Fatalf("empty allowed-outcome successor intersection widened to unrestricted: actions=%s status=%s evidence=%s", strings.Join(target.actions, ","), result.Status, loaded.EvidenceStatus)
	}
	if result.Status != "BLOCKED" || result.NextAllowed == nil || len(result.NextAllowed) != 0 || loaded.NextAllowed == nil || loaded.EvidenceStatus != "PASS" || loadErr != nil {
		t.Fatalf("explicit empty policy did not survive persistence and verification: %+v %v", loaded, loadErr)
	}
	for _, candidate := range result.Frontier {
		if candidate.NextAllowed || candidate.Eligibility != "NEXT_DISALLOWED" {
			t.Fatalf("empty policy left an eligible successor: %+v", candidate)
		}
	}
	// The former engine omitted this empty intersection. Recreate that old
	// comparison with consistent capture/projection hashes: it must remain
	// readable but cannot regain PASS or dispatch a successor on resume.
	raw, _, err := f.d.workbenchEvidence(result.Visits[0].Run, "action", 1)
	if err != nil {
		t.Fatal(err)
	}
	raw.Model.BusinessExpectations[0].Comparison.NextAllowed = nil
	hash := replaceSuccessorHistoricalCapture(t, f.d, raw)
	contract, err := f.d.runTaskContract(raw.Run)
	if err != nil {
		t.Fatal(err)
	}
	verdict, reason, _, err := f.d.evaluateModelObservation(contract, raw, 1, hash)
	if err != nil || verdict != "BLOCKED" || !strings.Contains(reason, "comparison differs") {
		t.Fatalf("legacy omitted empty policy was not semantically rejected: %s %s %v", verdict, reason, err)
	}
	legacy, err := f.d.loadBusinessCampaign(result.ID)
	if err != nil || legacy.EvidenceStatus != "BLOCKED" {
		t.Fatalf("legacy widened policy retained campaign evidence: %+v %v", legacy, err)
	}
	request.Campaign = result.ID
	resumed, _ := f.d.resumeBusinessCampaign(context.Background(), request)
	if resumed.Status == "COMPLETED_IN_SCOPE" || len(target.actions) != 1 {
		t.Fatalf("legacy omitted policy authorized replay: %+v actions=%v", resumed, target.actions)
	}
}

func successorSequenceGraph(t *testing.T, next BusinessSuccessorPolicy) Graph {
	t.Helper()
	g := kindsModel(t)
	g.BusinessContract = &BusinessContract{Version: 1,
		Subjects: []BusinessSubject{{ID: "fixture", Kind: "environment", Attributes: []BusinessAttribute{{Name: "enabled", Type: "boolean"}}}},
		Branches: []BusinessBranch{
			{ID: "enroll-only-again", Action: "enroll", Rule: "R1", Flow: "flow.enrollment", Transition: "submit-flow", FromStates: []string{"pending"}, When: BusinessPredicate{Op: "true"}, Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{{ID: "created", State: "ready", Denied: businessBool(false), Predicate: BusinessPredicate{Op: "true"}, NextAllowed: next}}}, Source: "document", SourceRef: "synthetic-audit:explicit-successor-restriction", Confirmed: true},
			{ID: "confirm-if-otherwise-ready", Action: "confirm", Rule: "R2", Flow: "flow.enrollment", Transition: "confirm-flow", FromStates: []string{"ready"}, When: BusinessPredicate{Op: "true"}, Expected: BusinessExpectation{Mode: "deterministic", Outcomes: []BusinessExpectedOutcome{{ID: "confirmed", State: "confirmed", Denied: businessBool(false), Predicate: BusinessPredicate{Op: "true"}}}}, Source: "document", SourceRef: "synthetic-audit:confirmation", Confirmed: true},
		},
	}
	if err := ValidateBusinessContract(g); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestBusinessSuccessorOrdinarySequenceBlocksForbiddenNextAction(t *testing.T) {
	target, server := newKindsServer(t)
	g := successorSequenceGraph(t, BusinessSuccessorPolicy{"enroll"})
	f := prepareExecutionFixture(t, g, server.URL)
	f.registerAndRound(t, sequenceManifest(t, "audit-sequence"))
	result := executeKinds(t, f, "audit-sequence", "test.sequence.enroll-confirm")
	verified, err := f.d.verifiedRunFacts(result.Run)
	t.Logf("verdict=%s steps=%d confirms=%d evidence=%s evidence-error=%v", result.Verdict, result.Steps, target.confirmCalls, verified.EvidenceStatus, err)
	if target.confirmCalls != 0 || result.Verdict == "PASS" {
		t.Fatalf("ordinary sequence ignored the frozen successor restriction: verdict=%s steps=%d confirms=%d evidence=%s", result.Verdict, result.Steps, target.confirmCalls, verified.EvidenceStatus)
	}
	if result.Verdict != "BLOCKED" || !strings.Contains(result.Reason, "next_allowed") || verified.EvidenceStatus != "PASS" || err != nil {
		t.Fatalf("blocked successor did not retain verifiable evidence: %+v %v", result, err)
	}
}

func TestBusinessSuccessorExistingUnrestrictedAndAllowedSequencesRemainVerifiable(t *testing.T) {
	for _, next := range []BusinessSuccessorPolicy{nil, {"confirm"}} {
		t.Run(fmt.Sprint(next == nil), func(t *testing.T) {
			target, server := newKindsServer(t)
			f := prepareExecutionFixture(t, successorSequenceGraph(t, next), server.URL)
			f.registerAndRound(t, sequenceManifest(t, "successor-valid"))
			result := executeKinds(t, f, "successor-valid", "test.sequence.enroll-confirm")
			verified, err := f.d.verifiedRunFacts(result.Run)
			if result.Verdict != "PASS" || target.confirmCalls != 1 || err != nil || verified.EvidenceStatus != "PASS" {
				t.Fatalf("existing valid serialized policy stopped passing: %+v %v", result, err)
			}
			if err := f.d.qualifyModelRun(result.Run); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBusinessSuccessorActorChangeCannotBypassOrderedRestriction(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		t.Run(fmt.Sprint(allowed), func(t *testing.T) {
			target, original := newKindsServer(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				recorder := httptest.NewRecorder()
				original.Config.Handler.ServeHTTP(recorder, r)
				body := recorder.Body.String()
				if r.Header.Get("X-Fixture-Actor") == "contender" {
					body = strings.ReplaceAll(body, "synthetic-STUDENT", "synthetic-CONTENDER")
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(recorder.Code)
				fmt.Fprint(w, body)
			}))
			t.Cleanup(server.Close)
			next := BusinessSuccessorPolicy{"enroll"}
			if allowed {
				next = BusinessSuccessorPolicy{"confirm"}
			}
			g := successorSequenceGraph(t, next)
			for i := range g.Tests {
				if g.Tests[i].ID == "test.sequence.enroll-confirm" {
					g.Tests[i].Actors = []ActorRequirement{{Alias: "contender", Role: "STUDENT"}}
				}
			}
			g.Rules[1].Oracle = "fixture-independent-store"
			f := prepareExecutionFixture(t, g, server.URL)
			t.Setenv("ARC_SUCCESSOR_TEST_CONTENDER", "contender")
			if _, err := f.d.importFixtures(Fixtures{Fixtures: []Fixture{{ID: "fixture.contender", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_SUCCESSOR_TEST_CONTENDER", Refs: map[string]string{"role": "STUDENT", "principal": "synthetic-CONTENDER", "environment": "independent"}}}}); err != nil {
				t.Fatal(err)
			}
			file := sequenceManifest(t, "successor-two-actors")
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var manifest RunnerManifest
			if err := decodeStrictJSON(body, &manifest); err != nil {
				t.Fatal(err)
			}
			manifest.Steps[1].Actor = "contender"
			manifest.Steps[1].Call.CredentialHeader = "X-Fixture-Actor"
			manifest.Steps[1].Oracles[0].ID = "fixture-independent-store"
			manifest.Steps[1].Oracles[0].Call.CredentialHeader = "X-Fixture-Actor"
			body, _ = json.Marshal(manifest)
			if err := os.WriteFile(file, body, 0600); err != nil {
				t.Fatal(err)
			}
			f.registerAndRound(t, file)
			if _, err := f.d.plan("course-v2", 128); err != nil {
				t.Fatal(err)
			}
			result, err := f.d.execute(context.Background(), ExecuteRequest{Task: "course-v2/test.sequence.enroll-confirm", Runner: manifest.ID, Round: f.round, Environment: "independent", Commit: f.commit, Root: f.root, Config: f.config, Account: f.account, Fixture: f.fixture, Accounts: map[string]string{"contender": "fixture.contender"}})
			if err != nil {
				t.Fatal(err)
			}
			want, calls := "BLOCKED", 0
			if allowed {
				want, calls = "PASS", 1
			}
			if result.Verdict != want || target.confirmCalls != calls {
				t.Fatalf("actor change lost ordered policy or confused independent identities: %+v confirms=%d", result, target.confirmCalls)
			}
			raw, _, err := f.d.workbenchEvidence(result.Run, "action", 2)
			if err != nil || len(raw.Model.Oracles) != 1 || raw.Model.Oracles[0].Actor != "contender" || raw.Model.Oracles[0].Before.Principal != "synthetic-CONTENDER" {
				t.Fatalf("second account did not supply its own before observation: %+v %v", raw.Model, err)
			}
			if verified, err := f.d.verifiedRunFacts(result.Run); err != nil || verified.EvidenceStatus != "PASS" {
				t.Fatalf("scoped actor evidence failed offline verification: %+v %v", verified, err)
			}
		})
	}
}
