package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func checkpointManifest(t *testing.T, m *RunnerManifest, state string) {
	t.Helper()
	// Set through JSON so the red test also compiles against the legacy type.
	body, err := json.Marshal(m.Recovery)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	fields["mode"] = json.RawMessage(`"checkpoint"`)
	fields["steps"] = json.RawMessage(`[]`)
	fields["state"], _ = json.Marshal(state)
	fields["checks"] = json.RawMessage(`[{"left":"enrolled","operator":"eq","value":1}]`)
	body, _ = json.Marshal(fields)
	if err := json.Unmarshal(body, m.Recovery); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeCheckpointPreservesActualAndVerifiesOffline(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	fixture := setupRuntimeBusiness(t, runtimeBusinessGraph(t), target, server, "accept", func(m *RunnerManifest) {
		checkpointManifest(t, m, "ready")
	})
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "PASS" || result.Recovery.Verdict != "PASS" || !target.stored || target.requests != 1 {
		t.Fatalf("checkpoint did not retain the independently verified actual state: %+v stored=%t requests=%d", result, target.stored, target.requests)
	}
	if strings.Contains(result.Recovery.Reason, "restored") || strings.Contains(result.Reason, "restoration") {
		t.Fatalf("retained state is incorrectly described as restored: %+v", result)
	}
	raw, _, err := fixture.d.workbenchEvidence(result.Run, "fixture", 0)
	if err != nil || raw.Model == nil || raw.Model.Node != "fixture.checkpoint" || raw.Expected != "ready" || raw.Observed != "ready" || len(raw.Model.Actions) != 0 {
		t.Fatalf("checkpoint RAW is incomplete: %+v %v", raw, err)
	}
	contract, err := fixture.d.runTaskContract(result.Run)
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.validateModelRecovery(contract); err != nil {
		t.Fatalf("checkpoint offline replay failed: %v", err)
	}
	for _, incorrectMode := range []string{"", "restore"} {
		changed := contract
		receipt := *contract.Receipt
		changed.Receipt = &receipt
		changed.Receipt.Recovery.Mode = incorrectMode
		if err := fixture.d.validateModelRecovery(changed); err == nil {
			t.Fatalf("checkpoint was accepted with mode %q", incorrectMode)
		}
	}
	facts, err := fixture.d.verifiedRunFacts(result.Run)
	if err != nil || facts.EvidenceStatus != "PASS" {
		t.Fatalf("checkpoint facts could not be verified: %+v %v", facts, err)
	}
	locks, err := fixture.d.query("SELECT fixture FROM fixture_locks WHERE run="+quote(result.Run), 1)
	if err != nil || len(locks) != 0 {
		t.Fatalf("verified checkpoint did not release its execution lease: %v %v", locks, err)
	}
	// Embedded evidence must remain usable when the original capture file is
	// unavailable; preserving history does not mean trusting a saved PASS.
	if err := os.Remove(result.Recovery.Evidence); err != nil {
		t.Fatal(err)
	}
	if err := fixture.d.validateModelRecovery(contract); err != nil {
		t.Fatalf("embedded checkpoint replay failed: %v", err)
	}
}

func TestRuntimeCheckpointCannotRetainAnotherPrincipal(t *testing.T) {
	target, source := newRuntimeBusinessTarget(t, true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/checkpoint" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"state":"ready","principal":"another-account","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":1,"capacity":1}`)
			return
		}
		source.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	fixture := setupRuntimeBusiness(t, runtimeBusinessGraph(t), target, server, "accept", func(m *RunnerManifest) {
		checkpointManifest(t, m, "ready")
		m.Recovery.Oracle.Call.Path = "/checkpoint"
	})
	result := executeRuntimeBusiness(t, fixture)
	if result.Verdict != "BLOCKED" || result.Recovery.Safe || !target.stored || len(result.Recovery.Actions) != 0 {
		t.Fatalf("checkpoint retained an unverified account: %+v stored=%t", result, target.stored)
	}
}

func TestRuntimeCheckpointRejectsWritesAndUndeclaredTargets(t *testing.T) {
	g := runtimeBusinessGraph(t)
	for _, invalid := range []string{"write", "state", "mode", "checks", "implicit"} {
		t.Run(invalid, func(t *testing.T) {
			file := rewriteFixtureManifest(t, "checkpoint-validation", "R1/POSITIVE", func(m *RunnerManifest) {
				checkpointManifest(t, m, "ready")
				switch invalid {
				case "write":
					m.Recovery.Steps = []RecoveryStep{{Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: "POST", Path: "/reset", SuccessStatuses: []int{200}}}}
				case "state":
					m.Recovery.State = "invented-final-state"
				case "mode":
					m.Recovery.Mode = "best-effort"
				case "checks":
					m.Recovery.Checks = nil
				case "implicit":
					m.Recovery.Mode = ""
				}
			})
			body, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var manifest RunnerManifest
			if err := json.Unmarshal(body, &manifest); err != nil {
				t.Fatal(err)
			}
			if _, err := validateManifest(g, manifest); err == nil {
				t.Fatal("unsafe checkpoint declaration was accepted")
			}
		})
	}
}

func TestRuntimeCheckpointWrongFinalStateOrPropertiesBlocks(t *testing.T) {
	for _, scenario := range []string{"state", "property"} {
		t.Run(scenario, func(t *testing.T) {
			target, server := newRuntimeBusinessTarget(t, true)
			fixture := setupRuntimeBusiness(t, runtimeBusinessGraph(t), target, server, "accept", func(m *RunnerManifest) {
				checkpointManifest(t, m, "ready")
				if scenario == "state" {
					m.Recovery.State = "pending"
				} else {
					m.Recovery.Checks[0].Value = json.RawMessage(`0`)
				}
			})
			result := executeRuntimeBusiness(t, fixture)
			if result.Verdict != "BLOCKED" || result.Recovery.Safe || result.Recovery.Verdict != "BLOCKED" || !target.stored || target.requests != 1 {
				t.Fatalf("unverified final checkpoint was treated as safe: %+v stored=%t", result, target.stored)
			}
			locks, err := fixture.d.query("SELECT fixture FROM fixture_locks WHERE run="+quote(result.Run), 1)
			if err != nil || len(locks) == 0 {
				t.Fatalf("failed checkpoint released its isolation lock: %v %v", locks, err)
			}
		})
	}
}
