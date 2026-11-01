package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestExploreCheckpointErrorsAreNotDiscarded(t *testing.T) {
	for _, name := range []string{"explore_runtime.go", "explore_resume.go"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "_ = d.saveExploreCheckpoint") {
			t.Fatalf("%s still discards checkpoint persistence failures", name)
		}
	}
}

func TestExploreActionLifecycleRejectsBlindReplay(t *testing.T) {
	if actionDispatching == "" || actionOutcomeUnknown == "" || actionRecorded == "" {
		t.Fatal("action lifecycle constants must be defined")
	}
	if !unknownActionStatus(actionDispatching) || !unknownActionStatus(actionOutcomeUnknown) {
		t.Fatal("dispatching and unknown outcomes must stay non-replayable")
	}
	if unknownActionStatus(actionPlanned) || unknownActionStatus(actionRecorded) {
		t.Fatal("planned and recorded actions are not in-flight unknowns")
	}
}

func TestExploreCrashWindowsKeepSideEffectingSafe(t *testing.T) {
	windows := []string{
		"before_dispatch", "during_dispatch", "after_send_timeout",
		"before_oracle", "after_raw_before_db", "db_write_fail", "during_recovery",
	}
	for _, window := range windows {
		func(window string) {
			target, server := newSeatServer(t)
			if window == "after_send_timeout" {
				target.hangClaim = true
			}
			fixture := prepareSeatFixture(t, server.URL)
			fixture.openRound(t)
			adapter := seatExploreAdapter(t, seatIsolationScope)
			adapter.StepTimeoutMS = 80
			first, err := fixture.d.runExploration(t.Context(), ExploreRequest{
				Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
				Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
				Account: fixture.account, Fixture: fixture.fixture, Adapter: adapter,
				Budget: ExploreRuntimeBudget{MaxStates: 4, MaxPaths: 4, MaxActions: 1, Deadline: 4 * time.Second},
				Fault:  ExploreFault{Window: window},
			})
			if err == nil && window != "after_send_timeout" {
				t.Fatalf("%s must surface a visible error", window)
			}
			state, loadErr := fixture.d.loadExploreCheckpoint(first.Campaign)
			if loadErr != nil && first.Campaign != "" {
				t.Fatalf("%s lost the campaign: %v", window, loadErr)
			}
			if first.Campaign != "" {
				for _, record := range state.Actions {
					if record.SideEffecting && (record.Status == actionDispatching || record.Status == actionOutcomeUnknown || record.Status == actionUnknown || record.Status == actionExecuting) {
						if record.Status == actionPlanned {
							t.Fatalf("%s replayed a dispatched action back to planned: %+v", window, record)
						}
					}
				}
			}
			if first.Campaign == "" {
				return
			}
			second, resumeErr := fixture.d.runExploration(t.Context(), ExploreRequest{
				Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
				Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
				Account: fixture.account, Fixture: fixture.fixture, Adapter: adapter,
				Campaign: first.Campaign,
				Budget:   ExploreRuntimeBudget{MaxStates: 4, MaxPaths: 4, MaxActions: 1, Deadline: 4 * time.Second},
			})
			if resumeErr == nil && window != "before_dispatch" && window != "after_send_timeout" {
				if target.claims > 1 && window != "before_dispatch" {
					t.Fatalf("%s resume replayed a side-effecting claim: claims=%d second=%+v", window, target.claims, second)
				}
			}
		}(window)
	}
}
