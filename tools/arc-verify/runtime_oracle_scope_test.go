package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRuntimeSecondaryOracleWrongScopeBlocksBeforeTarget(t *testing.T) {
	for _, field := range []string{"missing_principal", "principal", "role", "scope"} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reverse=%t", field, reverse), func(t *testing.T) {
				target, service := newRuntimeBusinessTarget(t, true)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/secondary-oracle" {
						service.Config.Handler.ServeHTTP(w, r)
						return
					}
					principal, role, scope := "synthetic-STUDENT", "STUDENT", "course-fixture"
					switch field {
					case "missing_principal":
						principal = ""
					case "principal":
						principal = "unrelated-account"
					case "role":
						role = "OTHER"
					case "scope":
						scope = "unrelated-fixture"
					}
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{"state":"pending","principal":%q,"role":%q,"scope":%q,"login":true,"stored":false}`, principal, role, scope)
				}))
				t.Cleanup(server.Close)
				fixture := setupRuntimeBusiness(t, runtimeBusinessGraph(t), target, server, "accept", func(m *RunnerManifest) {
					secondary := m.Steps[0].Oracles[0]
					secondary.ID, secondary.Boundary, secondary.Call.Path = "secondary-store", "after isolated entity commit", "/secondary-oracle"
					secondary.Measurements = map[string]string{"order.stored": "/stored"}
					m.Steps[0].Oracles = append(m.Steps[0].Oracles, secondary)
					if reverse {
						m.Steps[0].Oracles[0], m.Steps[0].Oracles[1] = m.Steps[0].Oracles[1], m.Steps[0].Oracles[0]
					}
				})
				result := executeRuntimeBusiness(t, fixture)
				target.mu.Lock()
				requests := target.requests
				target.mu.Unlock()
				if result.Verdict != "BLOCKED" || requests != 0 {
					t.Fatalf("unbound secondary reader allowed target execution: result=%+v requests=%d", result, requests)
				}
			})
		}
	}
}

func TestRuntimeEveryDeclaredOracleRetainsItsActorAndFixtureScope(t *testing.T) {
	for _, corrupt := range []string{"principal", "principal_reverse", "missing_principal", "role", "scope", "boundary", "duplicate", "undeclared", "reverse"} {
		t.Run(corrupt, func(t *testing.T) {
			target, server := newRuntimeBusinessTarget(t, true)
			g := runtimeBusinessGraph(t)
			g.BusinessContract.Branches[0].Expected.Outcomes[0].Checks = append(g.BusinessContract.Branches[0].Expected.Outcomes[0].Checks,
				BusinessExpectedCheck{ID: "separate-persistence", Kind: "persistence", Oracle: "secondary-store", SourceRef: "synthetic-oracle-scope:independent-persistence", Predicate: businessPredicate("eq", "order", "stored", "boolean", "true")})
			fixture := setupRuntimeBusiness(t, g, target, server, "accept", func(m *RunnerManifest) {
				secondary := m.Steps[0].Oracles[0]
				secondary.ID, secondary.Boundary, secondary.Call.Path = "secondary-store", "after isolated entity commit", "/secondary-oracle"
				secondary.Measurements = map[string]string{"order.stored": "/stored"}
				m.Steps[0].Oracles = append(m.Steps[0].Oracles, secondary)
			})
			result := executeRuntimeBusiness(t, fixture)
			if result.Verdict != "PASS" || target.requests != 1 {
				t.Fatalf("legal dual-oracle control did not execute: %+v", result)
			}
			raw, _, err := fixture.d.workbenchEvidence(result.Run, "action", 1)
			if err != nil || len(raw.Model.Oracles) != 2 {
				t.Fatalf("dual captures missing: %+v %v", raw.Model, err)
			}
			secondary := &raw.Model.Oracles[1]
			switch strings.TrimSuffix(corrupt, "_reverse") {
			case "principal":
				secondary.Before.Principal, secondary.After.Principal = "unrelated-account", "unrelated-account"
			case "missing_principal":
				secondary.Before.Principal, secondary.After.Principal = "", ""
			case "role":
				secondary.Before.Role, secondary.After.Role = "OTHER", "OTHER"
			case "scope":
				secondary.Before.Scope, secondary.After.Scope = "unrelated-fixture", "unrelated-fixture"
			case "boundary":
				secondary.Boundary = "another observation boundary"
			case "duplicate":
				raw.Model.Oracles = append(raw.Model.Oracles, *secondary)
			case "undeclared":
				extra := *secondary
				extra.ID = "not-in-the-frozen-runner"
				raw.Model.Oracles = append(raw.Model.Oracles, extra)
			}
			if strings.HasSuffix(corrupt, "reverse") {
				raw.Model.Oracles[0], raw.Model.Oracles[1] = raw.Model.Oracles[1], raw.Model.Oracles[0]
			}
			hash := replaceSuccessorHistoricalCapture(t, fixture.d, raw)
			contract, err := fixture.d.runTaskContract(result.Run)
			if err != nil {
				t.Fatal(err)
			}
			verdict, reason, _, err := fixture.d.evaluateModelObservation(contract, raw, 1, hash)
			if corrupt == "reverse" {
				if verdict != "PASS" || err != nil {
					t.Fatalf("legal oracle order change was rejected: %s %s %v", verdict, reason, err)
				}
				if verified, err := fixture.d.verifiedRunFacts(result.Run); err != nil || verified.EvidenceStatus != "PASS" {
					t.Fatalf("legal reordered facts not verifiable: %+v %v", verified, err)
				}
				return
			}
			if verdict == "PASS" {
				t.Fatalf("out-of-scope independent oracle retained PASS: case=%s verdict=%s reason=%s err=%v", corrupt, verdict, reason, err)
			}
			if verified, err := fixture.d.verifiedRunFacts(result.Run); err == nil && verified.EvidenceStatus == "PASS" {
				t.Fatal("wrong-scope oracle retained strong archived evidence")
			}
			if err := fixture.d.qualifyModelRun(result.Run); err == nil {
				t.Fatal("wrong-scope oracle remained eligible for coverage")
			}
		})
	}
}
