package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestBusinessCampaignFreshEntityRebindingAndChangedInputIsBlockedBeforeTarget(t *testing.T) {
	for _, changeIdentity := range []bool{false, true} {
		t.Run(fmt.Sprint(changeIdentity), func(t *testing.T) {
			var mu sync.Mutex
			entity, stored, requests := "order-original", false, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				switch {
				case strings.HasPrefix(r.URL.Path, "/enroll/"):
					requests++
					if r.URL.Path != "/enroll/"+entity {
						t.Errorf("wrong entity reached target: %s", r.URL.Path)
					}
					stored = true
					fmt.Fprint(w, `{"code":"OK"}`)
				case r.URL.Path == "/reset":
					stored = false
					fmt.Fprint(w, `{}`)
				case r.URL.Path == "/oracle":
					state, enrolled := "pending", 0
					if stored {
						state, enrolled = "ready", 1
					}
					fmt.Fprintf(w, `{"state":%q,"principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":%d,"capacity":1,"active":true,"stored":%t,"entity_id":%q}`, state, enrolled, stored, entity)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			g := campaignTestGraph(t, false)
			for i := range g.BusinessContract.Subjects {
				if g.BusinessContract.Subjects[i].ID == "order" {
					g.BusinessContract.Subjects[i].RequiresBinding = true
					g.BusinessContract.Subjects[i].Attributes = append(g.BusinessContract.Subjects[i].Attributes, BusinessAttribute{Name: "id", Type: "string"})
				}
			}
			m := campaignTestManifest(t, "entity-rebind", false)
			m.Steps[0].SubjectBindings["order"] = "entity.current.id"
			m.Steps[0].Oracles[0].Measurements["order.id"] = "/entity_id"
			m.Steps[0].Call.Path = "/enroll/{{var.current_id}}"
			m.Steps[0].Call.Body = json.RawMessage(`{"entity":"{{var.current_id}}"}`)
			m.Steps[0].Call.Extract = []ExtractRule{{Name: "current_id", Source: "oracle", OracleID: "fixture-independent-store", Path: "/entity_id", EntityKind: "order", EntityAlias: "current", Required: true}}
			f, request := newCampaignTestFixture(t, g, server.URL, m)
			hooks := &businessCampaignHooks{}
			if changeIdentity {
				hooks.afterIntent = func(BusinessCampaign, *businessCampaignExecution) error {
					mu.Lock()
					entity = "order-replaced"
					mu.Unlock()
					return nil
				}
			}
			result, err := f.d.runBusinessCampaignWithHooks(context.Background(), request, hooks)
			if err != nil {
				t.Fatal(err)
			}
			if changeIdentity {
				if requests != 0 || result.Status != "BLOCKED" || len(result.Visits) != 1 || !strings.Contains(result.Visits[0].Reason, "target changed after selection") {
					t.Fatalf("changed subject/request reached target: %+v requests=%d", result, requests)
				}
			} else {
				if requests != 1 || result.Status != "COMPLETED_IN_SCOPE" || len(result.Visits) != 1 || result.Visits[0].Before.Bindings["order"] != "order-original" {
					t.Fatalf("independent rebinding was lost: %+v requests=%d", result, requests)
				}
				loaded, err := f.d.loadBusinessCampaign(result.ID)
				if err != nil || loaded.EvidenceStatus != "PASS" {
					t.Fatalf("entity binding failed offline verification: %+v %v", loaded, err)
				}
			}
		})
	}
}

func TestBusinessCampaignContextFingerprintNeverCollapsesNumericLexemesOrIdentity(t *testing.T) {
	c := BusinessCampaign{ID: "campaign", HistorySHA256: "history-A"}
	runtime := businessCampaignVariantRuntime{Variant: BusinessCampaignVariant{ID: "variant", Runner: "registered"}, Binding: RunBinding{Identity: ExecutionIdentity{Graph: "g", Environment: "isolated", Account: "buyer-A", Fixture: "fixture-A", SourceSHA256: "source-A"}}}
	base := BusinessObservation{State: "ready", Values: map[string]json.RawMessage{"request.amount": json.RawMessage(`100`)}, Bindings: map[string]string{"buyer": "principal-A", "order": "entity-A"}}
	observation := ModelObservation{EvidenceVersion: runtimeEvidenceVersion, Requests: []RequestObservation{{ResolvedInput: json.RawMessage(`{"amount":100}`)}}}
	first := campaignObservationKey(c, runtime, "reserved-run", observation, base)
	for _, token := range []string{"100.0", "1e2", "10000000000000000001"} {
		changed := base
		changed.Values = map[string]json.RawMessage{"request.amount": json.RawMessage(token)}
		if campaignObservationKey(c, runtime, "reserved-run", observation, changed) == first {
			t.Fatalf("collapsed numeric token %s into 100", token)
		}
	}
	for _, part := range []string{"account", "fixture", "source", "environment", "history", "entity", "run"} {
		copyCampaign, copyRuntime, copyBefore, run := c, runtime, base, "reserved-run"
		switch part {
		case "account":
			copyRuntime.Binding.Identity.Account = "buyer-B"
		case "fixture":
			copyRuntime.Binding.Identity.Fixture = "fixture-B"
		case "source":
			copyRuntime.Binding.Identity.SourceSHA256 = "source-B"
		case "environment":
			copyRuntime.Binding.Identity.Environment = "other"
		case "history":
			copyCampaign.HistorySHA256 = "history-B"
		case "entity":
			copyBefore.Bindings = map[string]string{"buyer": "principal-A", "order": "entity-B"}
		case "run":
			run = "another-run"
		}
		if campaignObservationKey(copyCampaign, copyRuntime, run, observation, copyBefore) == first {
			t.Fatalf("collapsed %s scope", part)
		}
	}
}

func TestBusinessCampaignPreparationProbeCannotBorrowAnotherPrincipalOrRunner(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, false)
	f, request := newCampaignTestFixture(t, campaignTestGraph(t, true), server.URL, campaignTestManifest(t, "prep-identity", true))
	result, err := f.d.runBusinessCampaign(context.Background(), request)
	if err != nil || result.Status != "COMPLETED_IN_SCOPE" || target.preparations != 1 {
		t.Fatalf("real preparation fixture failed: %+v %v", result, err)
	}
	g, err := f.d.model(result.Graph)
	if err != nil {
		t.Fatal(err)
	}
	intents, err := f.d.businessCampaignDispatches(result)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"principal", "role", "scope", "round", "runner", "oracle", "boundary"} {
		t.Run(field, func(t *testing.T) {
			body, err := json.Marshal(intents[0].Intent.Candidate)
			if err != nil {
				t.Fatal(err)
			}
			var candidate BusinessCampaignCandidate
			if err := json.Unmarshal(body, &candidate); err != nil {
				t.Fatal(err)
			}
			obs := candidate.PreparationObservation
			if obs == nil || len(obs.Oracles) == 0 {
				t.Fatal("test has no actual preparation probe")
			}
			switch field {
			case "principal":
				obs.Oracles[0].Before.Principal = "foreign-principal"
			case "role":
				obs.Oracles[0].Before.Role = "OTHER"
			case "scope":
				obs.Oracles[0].Before.Scope = "foreign-fixture"
			case "round":
				obs.Round = "foreign-round"
			case "runner":
				obs.RunnerSHA256 = strings.Repeat("0", 64)
			case "oracle":
				obs.Oracles[0].ID = "foreign-oracle"
			case "boundary":
				obs.Oracles[0].Boundary = "foreign-boundary"
			}
			if err := f.d.verifyBusinessCampaignCandidate(result, g, candidate, intents[0].Intent.HistorySHA256); err == nil {
				t.Fatalf("borrowed preparation %s was accepted as independently observed", field)
			}
		})
	}
}
