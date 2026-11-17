package main

import (
	"context"
	"encoding/json"
	"testing"
)

// An action's independently checked postcondition need not enumerate every
// declared effect. A true postcondition does not make a still-missing effect
// disappear from the preparation search.
func TestBusinessPreparationKeepsMissingEffectsWhenEnsuresAlreadyTrue(t *testing.T) {
	graph := preparationModel(t)
	graph.BusinessContract.Preparation = []PreparationAction{{
		ID: "prepare-order", Action: "enroll", Requires: BusinessPredicate{Op: "true"},
		Ensures: businessPredicate("eq", "primary", "active", "boolean", "true"),
		Effects: []PreparationEffect{{Subject: "order", Attribute: "stored", Value: json.RawMessage("true")}},
		Source:  "document", SourceRef: "test:prepare-missing-effect",
	}}
	request := PreparationRequest{
		Observed: contractObservation(true), Goal: businessPredicate("eq", "order", "stored", "boolean", "true"),
		MaxSteps: 1, MaxStates: 8,
	}
	if err := ValidateBusinessContract(graph); err != nil {
		t.Fatal(err)
	}
	plan := PlanPreparation(graph, request)
	if plan.Status != "READY" || len(plan.Layers) != 1 || len(plan.Layers[0].Steps) != 1 || plan.Layers[0].Steps[0].ID != "prepare-order" || containsBusinessID(plan.AlreadySatisfied, "prepare-order") {
		t.Fatalf("true ensures hid a declared effect needed by the goal: %+v", plan)
	}
	if string(request.Observed.Values["order.stored"]) != "false" {
		t.Fatal("a preparation prediction replaced the independent initial observation")
	}
	for _, actualEffect := range []bool{true, false} {
		name := "readback-confirms-effect"
		if !actualEffect {
			name = "missing-actual-effect-blocks-target"
		}
		t.Run(name, func(t *testing.T) {
			store, err := cloneBusinessJSON(request.Observed)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			result := ExecutePreparation(context.Background(), graph, request,
				func(context.Context) (BusinessObservation, error) { return store, nil },
				func(_ context.Context, step PlannedPreparationStep) (PreparationReceipt, error) {
					calls++
					if actualEffect {
						store.Values["order.stored"] = json.RawMessage("true")
					}
					return preparationTestReceipt(step), nil
				},
			)
			if calls != 1 || len(result.Steps) != 1 || result.TargetReady != actualEffect {
				t.Fatalf("preparation must execute once and use actual goal readback: calls=%d result=%+v", calls, result)
			}
			if actualEffect {
				if result.Status != "READY" || string(result.InitialObserved.Values["order.stored"]) != "false" || string(result.FinalObserved.Values["order.stored"]) != "true" {
					t.Fatalf("real preparation did not preserve initial and final facts: %+v", result)
				}
				if err := ValidatePreparationExecution(graph, request, result); err != nil {
					t.Fatalf("real readbacks cannot be independently replayed: %v", err)
				}
			} else if result.Status != "BLOCKED" {
				t.Fatalf("predicted effect was accepted without actual goal readback: %+v", result)
			}
		})
	}
}

func TestBusinessCampaignPreparationUsesMissingEffectAndActualGoalReadback(t *testing.T) {
	for _, lying := range []bool{false, true} {
		name := "actual-goal-enables-target"
		if lying {
			name = "true-ensures-without-actual-goal-blocks-target"
		}
		t.Run(name, func(t *testing.T) {
			target, server := newRuntimeBusinessTarget(t, false)
			target.lyingPreparation = lying
			graph := campaignTestGraph(t, true)
			// The activation contract guarantees the order is left absent. Its
			// account.active effect is separately required by the actual goal.
			graph.BusinessContract.Preparation[0].Ensures = businessPredicate("eq", "order", "stored", "boolean", "false")
			fixture, request := newCampaignTestFixture(t, graph, server.URL, campaignTestManifest(t, "partial-ensures", true))
			target.ledger = &fixture.d
			campaign, err := fixture.d.runBusinessCampaign(context.Background(), request)
			if err != nil || len(campaign.Visits) != 1 || campaign.Visits[0].Preparation == nil || target.preparations != 1 {
				t.Fatalf("reachable preparation did not execute exactly once: err=%v prep=%d campaign=%+v", err, target.preparations, campaign)
			}
			preparation := campaign.Visits[0].Preparation
			if preparation.TargetReady == lying || string(preparation.InitialObserved.Values["account.active"]) != "false" {
				t.Fatalf("actual preparation readiness or initial facts changed: %+v", preparation)
			}
			if lying {
				if campaign.Status != "BLOCKED" || target.requests != 0 || preparation.Status != "BLOCKED" {
					t.Fatalf("a predicted activation reached the target: status=%s targets=%d preparation=%+v", campaign.Status, target.requests, preparation)
				}
				return
			}
			if campaign.Status != "COMPLETED_IN_SCOPE" || target.requests != 1 || !target.preFrozen || string(preparation.FinalObserved.Values["account.active"]) != "true" || campaign.Spent.Actions != 3 || campaign.Spent.ReservedActions != 0 {
				t.Fatalf("real activation, target freeze and budget were not preserved: campaign=%+v targets=%d frozen=%t", campaign, target.requests, target.preFrozen)
			}
			checked, err := fixture.d.loadBusinessCampaign(campaign.ID)
			if err != nil || checked.EvidenceStatus != "PASS" || checked.Status != "COMPLETED_IN_SCOPE" || len(checked.Visits) != 1 || checked.Visits[0].Run != campaign.Visits[0].Run {
				t.Fatalf("actual preparation/child cannot be independently replayed: %+v err=%v", checked, err)
			}
		})
	}
}

func TestBusinessPreparationDependencyNeedsAllObservedEffects(t *testing.T) {
	graph := preparationModel(t)
	graph.BusinessContract.Preparation = []PreparationAction{
		{ID: "activate-other", Action: "enroll", Requires: BusinessPredicate{Op: "true"}, Ensures: businessPredicate("eq", "primary", "active", "boolean", "true"), Effects: []PreparationEffect{{Subject: "other", Attribute: "active", Value: json.RawMessage("true")}}, Source: "document", SourceRef: "test:dependency-effects"},
		{ID: "prepare-order", Action: "enroll", Requires: BusinessPredicate{Op: "true"}, Ensures: businessPredicate("eq", "order", "stored", "boolean", "true"), DependsOn: []string{"activate-other"}, Effects: []PreparationEffect{{Subject: "order", Attribute: "stored", Value: json.RawMessage("true")}}, Source: "document", SourceRef: "test:dependent-order"},
	}
	request := preparationRequest()
	request.Observed.Values["primary.active"] = json.RawMessage("true")
	request.MaxSteps, request.MaxStates = 2, 16
	plan := PlanPreparation(graph, request)
	if plan.Status != "READY" || len(plan.Layers) != 2 || len(plan.Layers[0].Steps) != 1 || plan.Layers[0].Steps[0].ID != "activate-other" || plan.Layers[1].Steps[0].ID != "prepare-order" {
		t.Fatalf("dependency was satisfied by a partial ensures while its effect remained false: %+v", plan)
	}
	store, _ := cloneBusinessJSON(request.Observed)
	called := []string{}
	result := ExecutePreparation(context.Background(), graph, request,
		func(context.Context) (BusinessObservation, error) { return store, nil },
		func(_ context.Context, step PlannedPreparationStep) (PreparationReceipt, error) {
			called = append(called, step.ID)
			if step.ID == "prepare-order" && string(store.Values["other.active"]) != "true" {
				t.Fatal("dependent preparation ran before the prerequisite effect was observed")
			}
			for _, effect := range step.Effects {
				store.Values[effect.Subject+"."+effect.Attribute] = effect.Value
			}
			return preparationTestReceipt(step), nil
		},
	)
	if result.Status != "READY" || len(called) != 2 || called[0] != "activate-other" || called[1] != "prepare-order" {
		t.Fatalf("dependency schedule was not executed from real readbacks: called=%v result=%+v", called, result)
	}
	if err := ValidatePreparationExecution(graph, request, result); err != nil {
		t.Fatal(err)
	}
}
