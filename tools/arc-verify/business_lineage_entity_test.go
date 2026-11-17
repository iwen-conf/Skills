package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func lineageEntitySnapshot(t *testing.T, preparation bool) Snapshot {
	t.Helper()
	target, original := newRuntimeBusinessTarget(t, !preparation)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oracle" && r.Header.Get("X-Fixture-Actor") == "contender" {
			capture := httptest.NewRecorder()
			original.Config.Handler.ServeHTTP(capture, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(capture.Code)
			_, _ = w.Write(bytes.ReplaceAll(capture.Body.Bytes(), []byte(`"synthetic-STUDENT"`), []byte(`"synthetic-CONTENDER"`)))
			return
		}
		if r.URL.Path == "/create-order" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"order_id":"shared-order-id"}`)
			return
		}
		if r.URL.Path == "/enroll" {
			capture := httptest.NewRecorder()
			original.Config.Handler.ServeHTTP(capture, r)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(capture.Code)
			fmt.Fprint(w, `{"code":"OK","order_id":"shared-order-id"}`)
			return
		}
		original.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	g := runtimeBusinessGraph(t)
	if preparation {
		g = preparationLineageGraph(t)
	}
	g.Tests[0].Kind = testPositive
	g.Tests[0].Actors = []ActorRequirement{{Alias: "contender", Role: "STUDENT"}}
	g.BusinessContract.Subjects[1].RequiresBinding = true
	g.BusinessContract.Branches[0].When = BusinessPredicate{Op: "and", Args: []BusinessPredicate{
		g.BusinessContract.Branches[0].When, businessPredicate("eq", "order", "stored", "boolean", "false"),
	}}
	fixture := prepareExecutionFixture(t, g, server.URL)
	fixture.account = "fixture.lineage-primary"
	target.ledger = &fixture.d
	t.Setenv("ARC_LINEAGE_PRIMARY", "primary")
	t.Setenv("ARC_LINEAGE_CONTENDER", "contender")
	if _, err := fixture.d.importFixtures(Fixtures{Fixtures: []Fixture{
		{ID: "fixture.lineage-primary", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_LINEAGE_PRIMARY", Refs: map[string]string{"role": "STUDENT", "principal": "synthetic-STUDENT", "environment": "independent"}},
		{ID: "fixture.contender", Kind: "account", State: "ACTIVE", SecretRef: "env://ARC_LINEAGE_CONTENDER", Refs: map[string]string{"role": "STUDENT", "principal": "synthetic-CONTENDER", "environment": "independent"}},
	}}); err != nil {
		t.Fatal(err)
	}
	manifest := rewriteFixtureManifest(t, "lineage-entity-ownership", "test.idempotency", func(m *RunnerManifest) {
		step := &m.Steps[0]
		step.BusinessBranch = "accept"
		step.SubjectBindings = map[string]string{"account": "actor.primary.principal", "order": "entity.created-order.id"}
		step.ResponseMeasurements = map[string]string{"reply.code": "/code"}
		step.Oracles[0].Measurements["account.active"] = "/active"
		step.Oracles[0].Measurements["order.stored"] = "/stored"
		step.Oracles[0].Call.CredentialHeader = "X-Fixture-Actor"
		m.Recovery.Oracle.Call.CredentialHeader = "X-Fixture-Actor"
		m.Setup = []RecoveryStep{{Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: "/create-order", SuccessStatuses: []int{200}, Extract: []ExtractRule{{Name: "order_id", Source: "response_body", Path: "/order_id", EntityKind: "order", EntityAlias: "created-order", Required: true}}}}}
		step.Call.Extract = []ExtractRule{{Name: "order_id", Source: "response_body", Path: "/order_id", EntityKind: "order", EntityAlias: "created-order", OwnerActor: "contender", Required: true}}
		if preparation {
			observer := *step
			observer.BusinessBranch = ""
			m.Preparation = &RunnerPreparation{Goal: businessPredicate("eq", "account", "active", "boolean", "true"), MaxSteps: 1, MaxStates: 16, Observer: observer,
				Adapters: map[string]RunnerStep{"activate-account": {Node: "activate", Actor: "primary", Call: AdapterCall{Kind: adapterHTTP, Method: http.MethodPost, Path: "/activate", SuccessStatuses: []int{200}}}}}
		}
	})
	fixture.registerAndRound(t, manifest)
	if _, err := fixture.d.plan(g.ID, 128); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.d.execute(context.Background(), ExecuteRequest{Task: g.ID + "/test.idempotency", Runner: "lineage-entity-ownership", Round: fixture.round, Environment: "independent", Commit: fixture.commit, Root: fixture.root, Config: fixture.config, Account: fixture.account, Fixture: fixture.fixture, Accounts: map[string]string{"contender": "fixture.contender"}})
	if err != nil || result.Verdict != "PASS" {
		t.Fatalf("real entity capture fixture failed before genealogy export: %+v %v", result, err)
	}
	snapshot := preparationLineageSnapshot(t, fixture, result)
	found := false
	for _, fact := range snapshot.Runs[0].Facts {
		if fact.Seq != 1 {
			continue
		}
		if len(fact.BeforeEntities) != 1 || len(fact.Entities) != 1 || fact.BeforeEntities[0].ID != fact.Entities[0].ID || fact.BeforeEntities[0].Alias != "created-order" || fact.BeforeEntities[0].OwnerActor != "primary" || fact.Entities[0].OwnerActor != "contender" {
			t.Fatalf("counterexample needs the same alias/id with different observed before/after ownership: %+v", fact)
		}
		found = true
	}
	if !found {
		t.Fatal("real entity identity capture is missing")
	}
	return snapshot
}

func TestBusinessLineageEntityReferencesPreserveFrozenMappingAndHistoricalOwner(t *testing.T) {
	for _, preparation := range []bool{false, true} {
		name := "target-before"
		if preparation {
			name = "preparation-readback"
		}
		t.Run(name, func(t *testing.T) {
			snapshot := lineageEntitySnapshot(t, preparation)
			body, err := json.Marshal(snapshot.Runs[0].BusinessLineage)
			if err != nil {
				t.Fatal(err)
			}
			var overlays []struct {
				Phase string `json:"phase"`
				Nodes []struct {
					Values []struct {
						Subject string         `json:"subject"`
						Binding string         `json:"binding"`
						Entity  *EntityBinding `json:"entity_ref"`
					} `json:"values"`
				} `json:"nodes"`
			}
			if err := json.Unmarshal(body, &overlays); err != nil {
				t.Fatal(err)
			}
			phases := map[string]bool{}
			for _, overlay := range overlays {
				for _, node := range overlay.Nodes {
					for _, value := range node.Values {
						if value.Subject != "order" {
							continue
						}
						ref := value.Entity
						if ref == nil || ref.Alias != "created-order" || ref.ID != value.Binding || ref.Kind != "order" || ref.OwnerActor != "primary" || ref.OwnerPrincipal != "synthetic-STUDENT" || ref.SourceStep != -1 || ref.SourceField != "response_body:/order_id" {
							t.Fatalf("%s lost the explicit subject-to-entity mapping or borrowed the later owner's same bare ID: %+v", overlay.Phase, value)
						}
						phases[overlay.Phase] = true
					}
				}
			}
			if !phases["before"] || preparation && !phases["preparation"] {
				t.Fatalf("actual entity lineage missing in expected observation phase: %+v", phases)
			}
		})
	}
}

func TestBusinessLineageEntityReferencesNeverGuessNamespaceOrOwner(t *testing.T) {
	first := EntityBinding{Alias: "created-order", Kind: "order", ID: "same-id", OwnerActor: "merchant", OwnerPrincipal: "merchant-A", ChannelAccountID: "channel-A", SourceStep: -1, SourceField: "response_body:/order_id", Metadata: map[string]string{"source": "before"}}
	second := EntityBinding{Alias: "reviewed-order", Kind: "order", ID: "same-id", OwnerActor: "reviewer", OwnerPrincipal: "merchant-B", ChannelAccountID: "channel-B", SourceStep: -1, SourceField: "response_body:/order_id"}
	baseline := ModelObservation{Participants: []ParticipantBinding{{Alias: "reviewer", Principal: "same-id"}}, BeforeEntities: []EntityBinding{first, second}, Entities: []EntityBinding{{Alias: first.Alias, Kind: first.Kind, ID: first.ID, OwnerActor: second.OwnerActor, OwnerPrincipal: second.OwnerPrincipal}}}
	step := RunnerStep{Actor: "reviewer", SubjectBindings: map[string]string{"order": "entity.created-order.id"}}
	value := BusinessLineageValue{Subject: "order", SubjectKind: "entity", Binding: "same-id"}
	for name, change := range map[string]func(*RunnerStep, *ModelObservation, *BusinessLineageValue){
		"after-only": func(_ *RunnerStep, before *ModelObservation, _ *BusinessLineageValue) { before.BeforeEntities = nil },
		"duplicate-alias": func(_ *RunnerStep, before *ModelObservation, _ *BusinessLineageValue) {
			duplicate := before.BeforeEntities[0]
			duplicate.OwnerPrincipal = "unrelated-owner"
			before.BeforeEntities = append(before.BeforeEntities, duplicate)
		},
		"other-id": func(_ *RunnerStep, _ *ModelObservation, reading *BusinessLineageValue) {
			reading.Binding = "different-id"
		},
		"undeclared-alias": func(step *RunnerStep, _ *ModelObservation, _ *BusinessLineageValue) {
			step.SubjectBindings["order"] = "entity.missing-alias.id"
		},
		"actor-namespace": func(step *RunnerStep, _ *ModelObservation, _ *BusinessLineageValue) {
			step.SubjectBindings["order"] = "actor.reviewer.principal"
		},
		"unknown-owner": func(_ *RunnerStep, before *ModelObservation, _ *BusinessLineageValue) {
			before.BeforeEntities[0].OwnerPrincipal = ""
		},
		"default-namespace-collision": func(step *RunnerStep, before *ModelObservation, reading *BusinessLineageValue) {
			step.SubjectBindings = nil
			reading.Subject = first.Alias
			before.Participants = append(before.Participants, ParticipantBinding{Alias: first.Alias, Principal: first.ID})
		},
	} {
		t.Run(name, func(t *testing.T) {
			localStep, err := cloneBusinessJSON(step)
			if err != nil {
				t.Fatal(err)
			}
			localBefore, err := cloneBusinessJSON(baseline)
			if err != nil {
				t.Fatal(err)
			}
			localValue := value
			change(&localStep, &localBefore, &localValue)
			if ref := businessLineageEntityReference(localValue, localStep, localBefore); ref != nil {
				t.Fatalf("missing or ambiguous before identity was guessed from a bare ID: %+v", ref)
			}
		})
	}
	ref := businessLineageEntityReference(value, step, baseline)
	if ref == nil || !reflect.DeepEqual(*ref, first) || ref.OwnerActor == step.Actor {
		t.Fatal("a legitimate cross-role entity reference lost the captured owner or explicit alias mapping")
	}
	ref.Metadata["source"] = "changed-copy"
	if baseline.BeforeEntities[0].Metadata["source"] != "before" {
		t.Fatal("read-only lineage rewrote captured entity metadata")
	}
	step.SubjectBindings["order"] = "entity.reviewed-order.id"
	value.SubjectKind = "account"
	ref = businessLineageEntityReference(value, step, baseline)
	if ref == nil || !reflect.DeepEqual(*ref, second) {
		t.Fatal("an explicitly entity-backed business account was forced into the actor namespace")
	}
	value.Subject, step.SubjectBindings = first.Alias, nil
	ref = businessLineageEntityReference(value, step, baseline)
	if ref == nil || !reflect.DeepEqual(*ref, first) {
		t.Fatal("a unique default entity alias proved by runtime bindings was discarded")
	}
	changedChannel := first
	changedChannel.ChannelAccountID = "other-channel"
	if sameBusinessLineageEntity(&first, &changedChannel) {
		t.Fatal("canonical entity equality ignored the payment channel account")
	}
}
