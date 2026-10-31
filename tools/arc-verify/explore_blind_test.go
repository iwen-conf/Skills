package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.ToLower(string(body))
	type sealed struct {
		ID, Category, Signal, Reachable string
	}
	unsealed := []sealed{
		{ID: "B-STATE", Category: "illegal_transition", Signal: "R-CLAIM", Reachable: "claim while the course is open"},
		{ID: "B-AUTH", Category: "unauthorized_access", Signal: "R-NO-FORCE", Reachable: "student force"},
		{ID: "B-IDEM", Category: "idempotency", Signal: "records", Reachable: "repeat the same request id"},
		{ID: "B-RACE", Category: "race_oversell", Signal: "invariant.capacity", Reachable: "concurrent claim"},
		{ID: "B-VALID", Category: "missing_server_validation", Signal: "course_open", Reachable: "not reachable; the adapter has no action that closes the course"},
	}
	detected, missed, unspecified := []string{}, []string{}, []string{}
	for _, item := range report.Unspecified {
		unspecified = append(unspecified, item.Kind+":"+item.Action)
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
	writeExploreArtifact(t, "reports/blind-seat.json", map[string]any{
		"campaign": report.Campaign, "actions": report.Actions, "stop": report.StopReason,
		"rule_fails": report.RuleFails, "unspecified": unspecified, "risks": report.Risks,
		"detected": detected, "missed": missed, "within_contract": report.WithinContract,
		"note": "this is not a prewritten five-test mutation campaign; the explorer chose its own candidates",
	})
	if len(detected) == 0 && len(report.Unspecified) == 0 && len(report.RuleFails) == 0 {
		t.Fatalf("blind run produced no findings and no unspecified observations: %+v", report)
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
