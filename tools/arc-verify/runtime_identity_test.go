package main

import (
	"encoding/json"
	"testing"
)

func runtimeAmbiguousIdentity() ModelObservation {
	return ModelObservation{
		EvidenceVersion: runtimeEvidenceVersion,
		Participants:    []ParticipantBinding{{Alias: "primary", Account: "account-A", Principal: "principal-A"}},
		BeforeEntities:  []EntityBinding{{Alias: "primary", ID: "order-B"}},
		BeforeSessions:  []SessionBinding{{Scope: "primary", Digest: "session-C", Active: true}},
		Oracles: []OracleObservation{{Actor: "primary", ActionIndex: 1, ID: "db", Before: IndependentFacts{State: "pending", Values: map[string]json.RawMessage{
			"primary.active": json.RawMessage(`true`),
		}}}},
	}
}

func TestRuntimeBindingsAmbiguousNamespacesStayUnknown(t *testing.T) {
	for _, conflict := range []string{"actor_entity", "actor_session", "entity_session", "all"} {
		t.Run(conflict, func(t *testing.T) {
			observed := runtimeAmbiguousIdentity()
			switch conflict {
			case "actor_entity":
				observed.BeforeSessions = nil
			case "actor_session":
				observed.BeforeEntities = nil
			case "entity_session":
				observed.Participants = nil
			}
			step := RunnerStep{Node: "enroll", Actor: "primary"}
			before, err := runtimeBusinessObservation("flow.enrollment", "db", step, 1, true, &observed)
			if err != nil {
				t.Fatal(err)
			}
			if before.Bindings["primary"] != "" {
				t.Fatalf("ambiguous default alias selected a different namespace: %+v", before.Bindings)
			}
			if decision := freezeRuntimeBusiness(contractModel(t), "flow.enrollment", Rule{ID: "R1", Oracle: "db"}, step, 1, &observed).Decision; decision.Status != "BLOCKED" {
				t.Fatalf("unresolved identity was executable: %+v", decision)
			}
		})
	}
}

func TestRuntimeBindingsExplicitNamespaceResolvesAmbiguity(t *testing.T) {
	for source, want := range map[string]string{"actor.primary.principal": "principal-A", "actor.primary.account": "account-A", "entity.primary.id": "order-B", "session.primary.digest": "session-C"} {
		t.Run(source, func(t *testing.T) {
			observed := runtimeAmbiguousIdentity()
			step := RunnerStep{Node: "enroll", Actor: "primary", SubjectBindings: map[string]string{"primary": source}}
			before, err := runtimeBusinessObservation("flow.enrollment", "db", step, 1, true, &observed)
			if err != nil || before.Bindings["primary"] != want {
				t.Fatalf("explicit source did not resolve its exact identity: %+v %v", before.Bindings, err)
			}
			if source == "actor.primary.principal" {
				if decision := freezeRuntimeBusiness(contractModel(t), "flow.enrollment", Rule{ID: "R1", Oracle: "db"}, step, 1, &observed).Decision; decision.Status != "READY" {
					t.Fatalf("explicit correct actor binding was blocked: %+v", decision)
				}
			}
		})
	}
}
