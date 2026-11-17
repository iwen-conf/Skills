package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestBusinessCampaignPreparationCannotApplyAnotherSubjectsEffects(t *testing.T) {
	for _, scenario := range []string{"foreign-observer", "foreign-adapter", "same-subject"} {
		foreignSubject := scenario != "same-subject"
		for _, viaCampaign := range []bool{true, false} {
			name := scenario + "-blocks-before-preparation"
			if !foreignSubject {
				name = "different-observer-same-subject-can-prepare"
			}
			if viaCampaign {
				name = "campaign/" + name
			} else {
				name = "execute/" + name
			}
			t.Run(name, func(t *testing.T) {
				var mu sync.Mutex
				active := map[string]bool{"primary": false, "contender": false}
				stored := false
				preparations, targets, recoveries := 0, 0, 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mu.Lock()
					defer mu.Unlock()
					actor := "primary"
					principal, role := "synthetic-STUDENT", "STUDENT"
					if r.Header.Get("X-Fixture-Actor") == "contender" {
						actor, principal = "contender", "synthetic-CONTENDER"
						role = "ADMIN"
					}
					w.Header().Set("Content-Type", "application/json")
					switch r.URL.Path {
					case "/activate", "/activate/primary":
						preparations++
						subject := actor
						if r.URL.Path == "/activate/primary" {
							subject = "primary"
						}
						active[subject] = true
						fmt.Fprint(w, `{"code":"ACTIVATED"}`)
					case "/enroll":
						targets++
						if !active[actor] {
							w.WriteHeader(http.StatusUnauthorized)
							fmt.Fprint(w, `{"code":"ACCOUNT_INACTIVE"}`)
							return
						}
						stored = true
						fmt.Fprint(w, `{"code":"OK"}`)
					case "/reset":
						recoveries++
						active[actor], stored = false, false
						fmt.Fprint(w, `{}`)
					default:
						subject := actor
						if r.URL.Path == "/oracle/primary" {
							// The observer's own identity stays distinct from the
							// explicitly bound account whose state it reads.
							subject = "primary"
						}
						state, enrolled := "pending", 0
						if stored {
							state, enrolled = "ready", 1
						}
						fmt.Fprintf(w, `{"state":%q,"principal":%q,"role":%q,"scope":"course-fixture","login":true,"enrolled":%d,"capacity":1,"active":%t,"stored":%t}`, state, principal, role, enrolled, active[subject], stored)
					}
				}))
				t.Cleanup(server.Close)
				graph := campaignTestGraph(t, true)
				graph.Nodes = append(graph.Nodes, Node{ID: "ADMIN", Kind: "role", Name: "工具测试管理员"})
				graph.Tests[0].Kind = testPositive
				graph.Tests[0].Actors = []ActorRequirement{{Alias: "contender", Role: "ADMIN"}}
				manifest := campaignTestManifest(t, "preparation-subject", true)
				manifest.Test = graph.Tests[0].ID
				observer, err := cloneBusinessJSON(manifest.Preparation.Observer)
				if err != nil {
					t.Fatal(err)
				}
				observer.Actor = "contender"
				observer.Oracles[0].Call.CredentialHeader = "X-Fixture-Actor"
				adapter := manifest.Preparation.Adapters["activate-account"]
				adapter.Actor = "contender"
				adapter.Call.CredentialHeader = "X-Fixture-Actor"
				adapter.SubjectBindings = map[string]string{"account": "actor.primary.principal"}
				if scenario == "foreign-observer" {
					observer.SubjectBindings["account"] = "actor.contender.principal"
				} else {
					observer.Oracles[0].Call.Path = "/oracle/primary"
					adapter.Call.Path = "/activate/primary"
					if scenario == "foreign-adapter" {
						adapter.SubjectBindings["account"] = "actor.contender.principal"
						adapter.Call.Path = "/activate"
					}
				}
				manifest.Preparation.Observer = observer
				manifest.Preparation.Adapters["activate-account"] = adapter
				fixture, request := newCampaignTestFixture(t, graph, server.URL, manifest)
				t.Setenv("ARC_PREPARATION_IDENTITY_CONTENDER", "contender")
				if _, err := fixture.d.importFixtures(Fixtures{Fixtures: []Fixture{{ID: "fixture.contender", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_PREPARATION_IDENTITY_CONTENDER", Refs: map[string]string{"role": "ADMIN", "principal": "synthetic-CONTENDER", "environment": "independent"}}}}); err != nil {
					t.Fatal(err)
				}
				round, err := fixture.d.createRound(graph.ID, request.Environment, fixture.commit, fixture.root, fixture.config)
				if err != nil {
					t.Fatal(err)
				}
				request.Round = round.ID
				request.Accounts = map[string]string{"contender": "fixture.contender"}
				if !viaCampaign {
					if _, err := fixture.d.plan(graph.ID, 128); err != nil {
						t.Fatal(err)
					}
					result, err := fixture.d.execute(context.Background(), ExecuteRequest{Task: graph.ID + "/" + manifest.Test, Runner: manifest.ID, Round: request.Round, Environment: request.Environment, Commit: request.Commit, Root: request.Root, Config: request.Config, Account: request.Account, Fixture: request.Fixture, Accounts: request.Accounts})
					mu.Lock()
					prepCount, targetCount, recoveryCount, foreignActivated := preparations, targets, recoveries, active["contender"]
					mu.Unlock()
					if err != nil {
						t.Fatal(err)
					}
					if foreignSubject {
						if prepCount != 0 || targetCount != 0 || recoveryCount != 0 || result.Verdict != "BLOCKED" || !strings.Contains(result.Reason, "subject") {
							t.Fatalf("execute prepared another subject before target validation: prep=%d target=%d recovery=%d contender_active=%t result=%+v", prepCount, targetCount, recoveryCount, foreignActivated, result)
						}
						return
					}
					if prepCount != 1 || targetCount != 1 || recoveryCount != 1 || foreignActivated || result.Verdict != "PASS" {
						t.Fatalf("same-subject preparation by another role failed: prep=%d target=%d recovery=%d result=%+v", prepCount, targetCount, recoveryCount, result)
					}
					facts, err := fixture.d.verifiedRunFacts(result.Run)
					if err != nil || facts.EvidenceStatus != "PASS" {
						t.Fatalf("same-subject actual execute evidence cannot be verified: %+v err=%v", facts, err)
					}
					return
				}
				campaign, err := fixture.d.runBusinessCampaign(context.Background(), request)
				mu.Lock()
				prepCount, targetCount, recoveryCount, foreignActivated := preparations, targets, recoveries, active["contender"]
				mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				if foreignSubject {
					if prepCount != 0 || targetCount != 0 || recoveryCount != 0 || len(campaign.Visits) != 0 || campaign.Status != "BLOCKED" || len(campaign.Frontier) != 1 || !strings.Contains(campaign.Frontier[0].Reason, "subject") {
						t.Fatalf("foreign preparation effects were projected onto the target: prep=%d target=%d recovery=%d contender_active=%t campaign=%+v", prepCount, targetCount, recoveryCount, foreignActivated, campaign)
					}
					return
				}
				if prepCount != 1 || targetCount != 1 || recoveryCount != 1 || foreignActivated || len(campaign.Visits) != 1 || campaign.Status != "COMPLETED_IN_SCOPE" {
					t.Fatalf("same subject observed by a different actor cannot prepare: prep=%d target=%d recovery=%d contender_active=%t campaign=%+v", prepCount, targetCount, recoveryCount, foreignActivated, campaign)
				}
				checked, err := fixture.d.loadBusinessCampaign(campaign.ID)
				if err != nil || checked.EvidenceStatus != "PASS" || checked.Status != "COMPLETED_IN_SCOPE" {
					t.Fatalf("same-subject preparation failed independent replay: %+v err=%v", checked, err)
				}
			})
		}
	}
}

func TestRuntimePreparationBindsEntityCreatedByActualSetup(t *testing.T) {
	for _, mode := range []string{"static", "automatic", "static-missing-observer-id"} {
		automatic := mode == "automatic"
		missingObserver := mode == "static-missing-observer-id"
		name := "static-setup-creates-entity"
		if automatic {
			name = "automatic-preparation-creates-entity"
		}
		if missingObserver {
			name = "setup-created-target-with-unknown-preparation-subject-is-blocked"
		}
		t.Run(name, func(t *testing.T) {
			target, original := newRuntimeBusinessTarget(t, false)
			var mu sync.Mutex
			created, creations, boundWrites := "", 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/create-member" {
					mu.Lock()
					creations++
					created = fmt.Sprintf("prepared-member-%d", creations)
					value := created
					mu.Unlock()
					if automatic {
						target.mu.Lock()
						target.active = true
						target.mu.Unlock()
					}
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{"member_id":%q}`, value)
					return
				}
				if r.URL.Path == "/activate" || r.URL.Path == "/enroll" {
					body, err := io.ReadAll(r.Body)
					var input struct {
						MemberID string `json:"member_id"`
					}
					if err != nil || json.Unmarshal(body, &input) != nil {
						t.Errorf("invalid actual member request: %s err=%v", body, err)
					}
					mu.Lock()
					if created == "" || input.MemberID != created {
						t.Errorf("write used an uncreated or different entity: %q want=%q", input.MemberID, created)
					} else {
						boundWrites++
					}
					mu.Unlock()
				}
				original.Config.Handler.ServeHTTP(w, r)
			}))
			t.Cleanup(server.Close)
			graph := campaignTestGraph(t, true)
			if automatic {
				// A not-yet-created entity has no ID. The declaration deliberately
				// permits this initial state; the creating action supplies its ID.
				graph.BusinessContract.Subjects[0].RequiresBinding = false
			}
			manifest := campaignTestManifest(t, "setup-created-entity", true)
			observer, err := cloneBusinessJSON(manifest.Preparation.Observer)
			if err != nil {
				t.Fatal(err)
			}
			manifest.Steps[0].SubjectBindings["account"] = "entity.member.id"
			observer.SubjectBindings["account"] = "entity.member.id"
			if missingObserver {
				observer.SubjectBindings["account"] = "entity.missing.id"
			}
			manifest.Preparation.Observer = observer
			manifest.Setup = []RecoveryStep{{Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: "/create-member", SuccessStatuses: []int{200}, Extract: []ExtractRule{{Name: "member_id", Source: "response_body", Path: "/member_id", EntityKind: "account", EntityAlias: "member", Required: true}}}}}
			manifest.Steps[0].Call.Body = json.RawMessage(`{"member_id":"{{var.member_id}}"}`)
			adapter := manifest.Preparation.Adapters["activate-account"]
			adapter.Call.Body = json.RawMessage(`{"member_id":"{{var.member_id}}"}`)
			if automatic {
				adapter.Call = manifest.Setup[0].Call
				manifest.Setup = nil
			}
			manifest.Preparation.Adapters["activate-account"] = adapter
			fixture, request := newCampaignTestFixture(t, graph, server.URL, manifest)
			target.ledger = &fixture.d
			if _, err := fixture.d.plan(graph.ID, 128); err != nil {
				t.Fatal(err)
			}
			result, err := fixture.d.execute(context.Background(), ExecuteRequest{Task: graph.ID + "/" + manifest.Test, Runner: manifest.ID, Round: request.Round, Environment: request.Environment, Commit: request.Commit, Root: request.Root, Config: request.Config, Account: request.Account, Fixture: request.Fixture})
			mu.Lock()
			createCount, writeCount := creations, boundWrites
			mu.Unlock()
			if missingObserver {
				if err != nil || result.Verdict != "BLOCKED" || createCount != 1 || writeCount != 0 || target.preparations != 0 || target.requests != 0 || !strings.Contains(result.Reason, "subject") {
					t.Fatalf("a setup-created target borrowed an unknown preparation identity: result=%+v err=%v create=%d writes=%d prepare=%d target=%d", result, err, createCount, writeCount, target.preparations, target.requests)
				}
				return
			}
			wantBoundWrites, wantActivations := 2, 1
			if automatic {
				wantBoundWrites, wantActivations = 1, 0
			}
			if err != nil || result.Verdict != "PASS" || createCount != 1 || writeCount != wantBoundWrites || target.preparations != wantActivations || target.requests != 1 {
				t.Fatalf("legitimate setup-created entity was blocked or reused: result=%+v err=%v create=%d writes=%d prepare=%d target=%d", result, err, createCount, writeCount, target.preparations, target.requests)
			}
			facts, err := fixture.d.verifiedRunFacts(result.Run)
			if err != nil || facts.EvidenceStatus != "PASS" {
				t.Fatalf("setup-created identity does not replay from actual evidence: %+v err=%v", facts, err)
			}
			raw, _, err := fixture.d.workbenchEvidence(result.Run, "setup", 0)
			if err != nil || raw.Model == nil || len(raw.Model.Extractions) != 1 || raw.Model.Extractions[0].ActionIndex != 1 {
				t.Fatalf("missing original setup extraction: raw=%+v err=%v", raw, err)
			}
			for _, change := range []string{"future-extraction", "wrong-source-actor", "invented-genesis"} {
				t.Run(change, func(t *testing.T) {
					changed, err := cloneBusinessJSON(*raw.Model)
					if err != nil {
						t.Fatal(err)
					}
					switch change {
					case "future-extraction":
						changed.Extractions[0].ActionIndex = 2
					case "wrong-source-actor":
						changed.Extractions[0].SourceActor = "somebody-else"
					case "invented-genesis":
						changed.Extractions[0].ActionIndex = 2
						changed.BeforeEntities = append([]EntityBinding(nil), changed.Entities...)
					}
					if err := validateRuntimePreparationIdentityEvidence(graph, manifest, &changed); err == nil {
						t.Fatal("a later/final entity or another actor's extraction authorized an earlier preparation")
					}
				})
			}
		})
	}
}
