package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func legacyBlindStringScore(report ExploreReport) (detected, missed []string) {
	body, err := json.Marshal(report)
	if err != nil {
		return nil, []string{err.Error()}
	}
	joined := strings.ToLower(string(body))
	type sealed struct {
		ID, Signal, Reachable string
	}
	unsealed := []sealed{
		{ID: "B-STATE", Signal: "R-CLAIM", Reachable: "claim while the course is open"},
		{ID: "B-AUTH", Signal: "R-NO-FORCE", Reachable: "student force"},
		{ID: "B-IDEM", Signal: "records", Reachable: "repeat the same request id"},
		{ID: "B-RACE", Signal: "invariant.capacity", Reachable: "concurrent claim"},
		{ID: "B-VALID", Signal: "course_open", Reachable: "not reachable; the adapter has no action that closes the course"},
	}
	for _, item := range unsealed {
		if item.ID == "B-VALID" {
			missed = append(missed, item.ID+": "+item.Reachable)
			continue
		}
		if strings.Contains(joined, strings.ToLower(item.Signal)) {
			detected = append(detected, item.ID)
			continue
		}
		missed = append(missed, item.ID+": explorer did not emit "+item.Signal+" after "+item.Reachable)
	}
	return detected, missed
}

func TestLegacyBlindStringMatchMisattributesHealthyRecords(t *testing.T) {
	report := ExploreReport{
		Campaign: "X-healthy",
		Steps: []ExploreStep{{
			Candidate: CandidateAction{Action: "claim", Role: "STUDENT"},
			Before:    IndependentFacts{State: "idle", Values: map[string]json.RawMessage{"records": json.RawMessage("0"), "capacity": json.RawMessage("1")}},
			After:     IndependentFacts{State: "pending", Values: map[string]json.RawMessage{"records": json.RawMessage("1"), "capacity": json.RawMessage("1")}},
			Findings:  []ExploreFinding{{Kind: "PASS", Rule: "R-CLAIM", Reason: "reached pending"}},
		}},
	}
	detected, _ := legacyBlindStringScore(report)
	if !containsToken(detected, "B-IDEM") {
		t.Fatal("old string scorer must credit B-IDEM from the word records so the misattribution is visible")
	}
	if !containsToken(detected, "B-STATE") {
		t.Fatal("old string scorer must credit B-STATE from the rule name R-CLAIM without skip-review evidence")
	}
}

func TestBlindEvaluatorRejectsStringMatchAttribution(t *testing.T) {
	report := ExploreReport{
		Campaign: "X-healthy",
		Steps: []ExploreStep{{
			Candidate: CandidateAction{ID: "c1", Action: "claim", Role: "STUDENT", RequestID: "e-1"},
			Before:    IndependentFacts{State: "idle", Values: map[string]json.RawMessage{"records": json.RawMessage("0"), "capacity": json.RawMessage("1")}},
			After:     IndependentFacts{State: "pending", Values: map[string]json.RawMessage{"records": json.RawMessage("1"), "capacity": json.RawMessage("1")}},
			Findings:  []ExploreFinding{{Kind: "PASS", Rule: "R-CLAIM", Reason: "reached pending"}},
			RAW:       "raw/X-healthy/001.json",
		}},
	}
	sealed := []SealedMutation{
		{ID: "B-STATE", Category: "illegal_transition", InjectSite: "claimLocked.skipReview", Rule: "R-CLAIM", Activation: MutationActivation{Action: "claim", ObservedState: "enrolled"}, Link: MutationLink{Rule: "R-CLAIM", Kind: "FAIL", ObservedState: "enrolled"}},
		{ID: "B-IDEM", Category: "idempotency", InjectSite: "claimLocked.duplicateRecords", Rule: "invariant.capacity", Activation: MutationActivation{Action: "claim", SameRequestIDGrowth: true}, Link: MutationLink{Rule: "invariant.capacity", Kind: "FAIL", SameRequestID: true}},
		{ID: "B-RACE", Category: "race_oversell", InjectSite: "raceClaim.staleOccupancy", Rule: "invariant.capacity", Activation: MutationActivation{Action: "claim", DistinctRequestIDs: 2, OverCapacity: true}, Link: MutationLink{Rule: "invariant.capacity", Kind: "FAIL", DistinctIDs: true}},
		{ID: "B-AUTH", Category: "unauthorized_access", InjectSite: "handleForce.skipAdmin", Rule: "R-NO-FORCE", Activation: MutationActivation{Action: "force", Role: "STUDENT", Changed: true}, Link: MutationLink{Rule: "R-NO-FORCE", Kind: "FAIL", Action: "force"}},
		{ID: "B-VALID", Category: "missing_server_validation", InjectSite: "claimLocked.acceptWhenClosed", Rule: "R-CLAIM", Activation: MutationActivation{Action: "claim", GuardFalse: "course_open", Changed: true}, Link: MutationLink{Rule: "R-CLAIM", Kind: "FAIL", GuardFalse: "course_open"}},
	}
	result := AttributeBlindCampaign(report, sealed, ExploreRuntimeBudget{MaxActions: 12})
	for _, id := range []string{"B-STATE", "B-IDEM", "B-RACE", "B-AUTH"} {
		item := result.ByID(id)
		if item.Verdict == FaultDetected {
			t.Fatalf("healthy report must not detect %s; got %+v", id, item)
		}
		if item.Activated {
			t.Fatalf("%s was not injected and must not be marked activated: %+v", id, item)
		}
	}
	if item := result.ByID("B-VALID"); item.Verdict != FaultUnreachable {
		t.Fatalf("B-VALID never entered a closed course and must be unreachable, got %+v", item)
	}
	if result.Counts[FaultDetected] != 0 || result.Counts[FaultFalsePositive] != 0 {
		t.Fatalf("string-like signals must not count as detections: %+v", result)
	}
}

func TestBlindEvaluatorLinksFailToOneActivatedFault(t *testing.T) {
	report := ExploreReport{
		Campaign: "X-idem",
		Steps: []ExploreStep{{
			Candidate: CandidateAction{ID: "c1", Kind: kindRepeat, Action: "claim", Role: "STUDENT", RequestID: "dup-1"},
			Before:    IndependentFacts{State: "pending", Values: map[string]json.RawMessage{"records": json.RawMessage("1"), "capacity": json.RawMessage("1")}},
			After:     IndependentFacts{State: "pending", Values: map[string]json.RawMessage{"records": json.RawMessage("2"), "capacity": json.RawMessage("1")}},
			Findings:  []ExploreFinding{{Kind: "FAIL", Rule: "invariant.capacity", Reason: "records lte capacity", Evidence: "raw/X-idem/001.json"}},
			RAW:       "raw/X-idem/001.json",
		}},
	}
	sealed := []SealedMutation{
		{ID: "B-IDEM", Category: "idempotency", InjectSite: "claimLocked.duplicateRecords", Rule: "invariant.capacity", Activation: MutationActivation{Action: "claim", SameRequestIDGrowth: true}, Link: MutationLink{Rule: "invariant.capacity", Kind: "FAIL", SameRequestID: true}},
		{ID: "B-RACE", Category: "race_oversell", InjectSite: "raceClaim.staleOccupancy", Rule: "invariant.capacity", Activation: MutationActivation{Action: "claim", DistinctRequestIDs: 2, OverCapacity: true}, Link: MutationLink{Rule: "invariant.capacity", Kind: "FAIL", DistinctIDs: true}},
	}
	result := AttributeBlindCampaign(report, sealed, ExploreRuntimeBudget{MaxActions: 8})
	if item := result.ByID("B-IDEM"); item.Verdict != FaultDetected || len(item.Linked) != 1 || item.RunID == "" {
		t.Fatalf("repeat request_id oversell must detect only B-IDEM with oracle evidence: %+v", item)
	}
	if item := result.ByID("B-RACE"); item.Verdict == FaultDetected {
		t.Fatalf("one invariant FAIL must not also count as B-RACE: %+v", item)
	}
}

func containsToken(values []string, want string) bool {
	for _, item := range values {
		if item == want {
			return true
		}
	}
	return false
}

func TestBlindFaultCombinationsAndSeeds(t *testing.T) {
	cases := []struct {
		name  string
		seed  int
		apply func(*seatServer)
		want  string
	}{
		{name: "state-only", seed: 1, apply: func(s *seatServer) { s.skipReview = true }, want: "B-STATE"},
		{name: "auth-only", seed: 2, apply: func(s *seatServer) { s.unauthorizedForce = true }, want: "B-AUTH"},
		{name: "idem-only", seed: 3, apply: func(s *seatServer) { s.duplicateRecords = true }, want: "B-IDEM"},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			target, server := newSeatServer(t)
			item.apply(target)
			fixture := prepareSeatFixture(t, server.URL)
			fixture.openRound(t)
			report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
				Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
				Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
				Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
				Budget: ExploreRuntimeBudget{MaxStates: 16, MaxPaths: 16, MaxActions: 10, Deadline: 12 * time.Second, MaxReplans: 6, Seed: item.seed},
			})
			if err != nil {
				t.Fatal(err)
			}
			result := AttributeBlindCampaign(report, seatBlindMutations(), ExploreRuntimeBudget{MaxActions: 10, Seed: item.seed})
			for _, attribution := range result.Attributions {
				if attribution.Verdict == FaultDetected && attribution.MutationID != item.want {
					t.Fatalf("seed %d injected %s but attributed %s: %+v", item.seed, item.want, attribution.MutationID, attribution)
				}
				if attribution.MutationID == item.want && attribution.Verdict == FaultDetected && (len(attribution.Linked) == 0 || attribution.RunID == "") {
					t.Fatalf("detected %s lacks oracle linkage: %+v", item.want, attribution)
				}
			}
			if result.ByID("B-VALID").Verdict != FaultUnreachable {
				t.Fatalf("B-VALID must stay unreachable: %+v", result.ByID("B-VALID"))
			}
		})
	}
}

func TestAutonomousExploreBlindSeat(t *testing.T) {
	target, server := newSeatServer(t)
	target.mu.Lock()
	target.skipReview = true
	target.unauthorizedForce = true
	target.duplicateRecords = true
	target.raceNoLock = true
	target.raceGate = newOccupancyBarrier(2)
	target.mu.Unlock()
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Budget: ExploreRuntimeBudget{MaxStates: 24, MaxPaths: 24, MaxActions: 12, Deadline: 15 * time.Second, MaxReplans: 8},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Actions == 0 {
		t.Fatal("blind explorer executed nothing")
	}
	result := AttributeBlindCampaign(report, seatBlindMutations(), ExploreRuntimeBudget{MaxActions: 12})
	for _, item := range result.Attributions {
		if item.Verdict == FaultDetected && (len(item.Linked) == 0 || item.RunID == "" || !item.Activated) {
			t.Fatalf("detection of %s is not traced to an activated fault, path and oracle: %+v", item.MutationID, item)
		}
		if item.Verdict == FaultFalseNegative && !item.Activated {
			t.Fatalf("unactivated %s must not be counted as a miss: %+v", item.MutationID, item)
		}
	}
	if item := result.ByID("B-VALID"); item.Verdict != FaultUnreachable {
		t.Fatalf("B-VALID has no close-course action and must stay unreachable: %+v", item)
	}
	unspecified := []string{}
	for _, item := range report.Unspecified {
		unspecified = append(unspecified, item.Kind+":"+item.Action)
	}
	writeExploreArtifact(t, "reports/blind-seat.json", map[string]any{
		"campaign": report.Campaign, "actions": report.Actions, "stop": report.StopReason,
		"rule_fails": report.RuleFails, "unspecified": unspecified, "risks": report.Risks,
		"attributions": result.Attributions, "counts": result.Counts, "spent": result.Spent,
		"note": "attribution uses independent oracle facts, not report string search",
	})
	if result.Counts[FaultDetected]+len(report.Unspecified)+len(report.RuleFails) == 0 {
		t.Fatalf("blind run produced no attributed defect, unspecified observation or rule FAIL: %+v %s", report, formatAttribution(result))
	}
}

func writeExploreArtifact(t *testing.T, rel string, value any) {
	t.Helper()
	path := filepath.Join("..", "..", ".arc", "artifacts", "verify-explore-e2e", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
}
