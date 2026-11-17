package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func preparationModel(t *testing.T) Graph {
	t.Helper()
	graph := contractModel(t)
	primary := businessPredicate("eq", "primary", "active", "boolean", "true")
	other := businessPredicate("eq", "other", "active", "boolean", "true")
	stored := businessPredicate("eq", "order", "stored", "boolean", "true")
	graph.BusinessContract.Preparation = []PreparationAction{
		{ID: "activate-primary", Action: "enroll", Requires: BusinessPredicate{Op: "true"}, Ensures: primary, Effects: []PreparationEffect{{Subject: "primary", Attribute: "active", Value: json.RawMessage("true")}}, Source: "document", SourceRef: "test:primary-setup"},
		{ID: "activate-other", Action: "enroll", Requires: BusinessPredicate{Op: "true"}, Ensures: other, Effects: []PreparationEffect{{Subject: "other", Attribute: "active", Value: json.RawMessage("true")}}, Source: "document", SourceRef: "test:other-setup"},
		{ID: "prepare-order", Action: "enroll", Requires: BusinessPredicate{Op: "and", Args: []BusinessPredicate{primary, other}}, Ensures: stored, DependsOn: []string{"activate-primary", "activate-other"}, Effects: []PreparationEffect{{Subject: "order", Attribute: "stored", Value: json.RawMessage("true")}}, Source: "document", SourceRef: "test:prepare-order"},
	}
	return graph
}

func preparationRequest() PreparationRequest {
	observed := contractObservation(false)
	observed.Values["other.active"] = json.RawMessage("false")
	return PreparationRequest{Observed: observed, Goal: businessPredicate("eq", "order", "stored", "boolean", "true"), MaxSteps: 10, MaxStates: 100}
}

func TestBusinessContractD10PreparationStartsAtObservedStateAndExplainsParallelLayers(t *testing.T) {
	graph := preparationModel(t)
	request := preparationRequest()
	before, _ := businessJSONHash(request.Observed)
	plan := PlanPreparation(graph, request)
	if plan.Status != "READY" || len(plan.Layers) != 2 || len(plan.Layers[0].Steps) != 2 || plan.Layers[1].Steps[0].ID != "prepare-order" || !plan.RequiresReadback {
		t.Fatalf("preparation is not an observed-state DAG plan: %+v", plan)
	}
	for _, layer := range plan.Layers {
		for _, step := range layer.Steps {
			if step.SourceRef == "" || len(step.Effects) == 0 {
				t.Fatal("plan lost sourced preparation contract")
			}
		}
	}
	after, _ := businessJSONHash(request.Observed)
	if before != after || string(request.Observed.Values["order.stored"]) != "false" {
		t.Fatal("predicted plan effects were written into Observed")
	}
	request.Observed.Values["primary.active"] = json.RawMessage("true")
	plan = PlanPreparation(graph, request)
	if plan.Status != "READY" || len(plan.Layers[0].Steps) != 1 || plan.Layers[0].Steps[0].ID != "activate-other" || !containsBusinessID(plan.AlreadySatisfied, "activate-primary") {
		t.Fatalf("planner restarted already-satisfied preparation: %+v", plan)
	}
	request.Observed.Values["order.stored"] = json.RawMessage("true")
	plan = PlanPreparation(graph, request)
	if plan.Status != "ALREADY_SATISFIED" || len(plan.Layers) != 0 {
		t.Fatalf("already achieved observed goal needed synthetic setup: %+v", plan)
	}
}

func TestBusinessContractD10StateRetryCycleIsLegalButPreparationCycleIsNot(t *testing.T) {
	graph := preparationModel(t)
	graph.Edges = append(graph.Edges, Edge{ID: "retry-enroll", From: "ready", To: "pending", Kind: "transitions_to"})
	graph.Transitions = append(graph.Transitions, TransitionContract{Edge: "retry-enroll", Action: "enroll", Rule: "R1", Preconditions: ConditionAssignment{"login": true}, Outcome: branchRetry})
	if err := validateGraph(graph); err != nil {
		t.Fatalf("legal retry state cycle was mistaken for preparation dependency cycle: %v", err)
	}
	if plan := PlanPreparation(graph, preparationRequest()); plan.Status != "READY" {
		t.Fatalf("state cycle blocked independent preparation DAG: %+v", plan)
	}
	graph.BusinessContract.Preparation[0].DependsOn = []string{"prepare-order"}
	if err := validateGraph(graph); err == nil || !strings.Contains(err.Error(), "preparation dependency cycle") {
		t.Fatalf("preparation dependency cycle was accepted: %v", err)
	}
	if plan := PlanPreparation(graph, preparationRequest()); plan.Status != "MODEL_ERROR" {
		t.Fatalf("cyclic preparation produced executable schedule: %+v", plan)
	}
}

func TestBusinessPreparationUnknownRequirementsBlockedAndKnownUnreachableGoalIsGap(t *testing.T) {
	graph := preparationModel(t)
	graph.BusinessContract.Preparation[0].Requires = businessPredicate("gt", "primary", "level", "integer", "0")
	request := preparationRequest()
	delete(request.Observed.Values, "primary.level")
	plan := PlanPreparation(graph, request)
	if plan.Status != "BLOCKED" || len(plan.Blocked) == 0 || len(plan.Layers) > 0 {
		t.Fatalf("unknown setup requirement assumed true: %+v", plan)
	}
	graph = preparationModel(t)
	request = preparationRequest()
	request.Goal = businessPredicate("gte", "order", "amount", "number", "100")
	plan = PlanPreparation(graph, request)
	if plan.Status != "SPECIFICATION_GAP" || !containsBusinessID(plan.Gaps, "order.amount") {
		t.Fatalf("known unreachable goal did not identify missing producer: %+v", plan)
	}
}

func TestBusinessPreparationBudgetAndContradictoryEffectsNeverProduceSuccess(t *testing.T) {
	graph := preparationModel(t)
	request := preparationRequest()
	request.MaxSteps = 2
	if plan := PlanPreparation(graph, request); plan.Status != "BUDGET_EXHAUSTED" || len(plan.Layers) > 0 {
		t.Fatalf("bounded incomplete search claimed impossible or ready: %+v", plan)
	}
	request.MaxSteps, request.MaxStates = 10, 1
	if plan := PlanPreparation(graph, request); plan.Status != "BUDGET_EXHAUSTED" {
		t.Fatal(plan)
	}
	request.MaxStates = 100
	graph.BusinessContract.Preparation[0].Effects[0].Value = json.RawMessage("false")
	if plan := PlanPreparation(graph, request); plan.Status != "MODEL_ERROR" {
		t.Fatalf("declared effects contradict ensures but were scheduled: %+v", plan)
	}
}

func TestBusinessPreparationParallelLayersRespectReadWriteConflicts(t *testing.T) {
	first := PreparationAction{ID: "one", Action: "enroll", Requires: BusinessPredicate{Op: "true"}, Ensures: BusinessPredicate{Op: "true"}, Effects: []PreparationEffect{{Subject: "order", Attribute: "balance", Value: json.RawMessage("10")}}}
	second := PreparationAction{ID: "two", Action: "enroll", Requires: businessPredicate("eq", "order", "balance", "integer", "10"), Ensures: BusinessPredicate{Op: "true"}, Effects: []PreparationEffect{{Subject: "order", Attribute: "events", Value: json.RawMessage("1")}}}
	third := PreparationAction{ID: "three", Action: "enroll", Requires: BusinessPredicate{Op: "true"}, Ensures: BusinessPredicate{Op: "true"}, Effects: []PreparationEffect{{Subject: "other", Attribute: "active", Value: json.RawMessage("true")}}}
	layers := preparationLayers([]PreparationAction{first, second, third}, []int{0, 1, 2})
	if len(layers) != 2 || len(layers[0].Steps) != 2 || len(layers[1].Steps) != 1 || layers[1].Steps[0].ID != "two" {
		t.Fatalf("read-after-write dependency was parallelized: %+v", layers)
	}
}

func TestBusinessPreparationDoesNotTreatAssumedEffectsAsRealObservations(t *testing.T) {
	graph := preparationModel(t)
	request := preparationRequest()
	first := PlanPreparation(graph, request)
	if first.Status != "READY" {
		t.Fatal(first)
	}
	// No SUT action and no independent readback occurred. Replanning the same
	// real observation must still require the exact same three actions.
	second := PlanPreparation(graph, request)
	count := 0
	for _, layer := range second.Layers {
		count += len(layer.Steps)
	}
	if second.Status != "READY" || count != 3 || len(second.AlreadySatisfied) != 0 {
		t.Fatalf("a prior predicted plan fabricated achieved state: %+v", second)
	}
}
