package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func preparationTestReceipt(step PlannedPreparationStep) PreparationReceipt {
	return PreparationReceipt{Action: ActionObservation{Node: step.Action, Actor: "primary", Principal: "acct-A", RequestSHA256: strings.Repeat("a", 64), ResponseSHA256: strings.Repeat("b", 64), Started: "2026-11-17T10:00:00Z", Completed: "2026-11-17T10:00:01Z"}}
}

func TestBusinessPreparationExecutesAutomaticallyAndReReadsEachRealEffect(t *testing.T) {
	graph, request := preparationModel(t), preparationRequest()
	store := request.Observed
	called := []string{}
	reads := 0
	observe := func(context.Context) (BusinessObservation, error) { reads++; return store, nil }
	act := func(_ context.Context, step PlannedPreparationStep) (PreparationReceipt, error) {
		called = append(called, step.ID)
		for _, effect := range step.Effects {
			store.Values[effect.Subject+"."+effect.Attribute] = effect.Value
		}
		return preparationTestReceipt(step), nil
	}
	result := ExecutePreparation(context.Background(), graph, request, observe, act)
	if result.Status != "READY" || !result.TargetReady || result.ParallelExecuted || len(result.Steps) != 3 || len(called) != 3 || reads < 7 {
		t.Fatalf("preparation did not actually execute/read each action: %+v calls=%v reads=%d", result, called, reads)
	}
	if string(result.InitialObserved.Values["other.active"]) != "false" || string(result.FinalObserved.Values["order.stored"]) != "true" {
		t.Fatal("initial observations were replaced by predicted or subsequent state")
	}
	for _, step := range result.Steps {
		if step.Status != "PASS" || step.After == nil || step.Requirements.Value != businessTrue || step.Ensures.Value != businessTrue {
			t.Fatalf("missing execution evidence: %+v", step)
		}
	}
	if err := ValidatePreparationExecution(graph, request, result); err != nil {
		t.Fatalf("valid actual preparation cannot be independently replayed: %v", err)
	}
}

func TestBusinessPreparationActualViolationStopsTargetAndKeepsFirstEvidence(t *testing.T) {
	graph, request := preparationModel(t), preparationRequest()
	store := request.Observed
	calls := 0
	result := ExecutePreparation(context.Background(), graph, request,
		func(context.Context) (BusinessObservation, error) { return store, nil },
		func(_ context.Context, step PlannedPreparationStep) (PreparationReceipt, error) {
			calls++
			return preparationTestReceipt(step), nil
		},
	)
	if result.Status != "BLOCKED" || result.TargetReady || calls != 1 || len(result.Steps) != 1 || result.Steps[0].Status != "FAIL" || result.FirstFailureIndex == nil || *result.FirstFailureIndex != 0 {
		t.Fatalf("ineffective actual setup continued to target: %+v calls=%d", result, calls)
	}
	step := result.Steps[0]
	if step.After == nil || step.Ensures.Value != businessFalse || len(step.Receipt.Action.ResponseSHA256) != 64 || len(step.Planned.Effects) == 0 {
		t.Fatal("first mismatch lost expected/actual/action capture")
	}
}

func TestBusinessPreparationUncertainSentActionIsNotRetriedEvenIfStateChanged(t *testing.T) {
	graph, request := preparationModel(t), preparationRequest()
	request.Goal = businessPredicate("eq", "other", "active", "boolean", "true")
	store := request.Observed
	calls := 0
	result := ExecutePreparation(context.Background(), graph, request,
		func(context.Context) (BusinessObservation, error) { return store, nil },
		func(_ context.Context, step PlannedPreparationStep) (PreparationReceipt, error) {
			calls++
			store.Values["other.active"] = json.RawMessage("true")
			return preparationTestReceipt(step), errors.New("connection lost after send")
		},
	)
	if result.Status != "BLOCKED" || result.TargetReady || calls != 1 || result.Steps[0].After == nil || string(result.Steps[0].After.Values["other.active"]) != "true" {
		t.Fatalf("uncertain side effect was retried or treated as success: %+v calls=%d", result, calls)
	}
}

func TestBusinessPreparationAsyncPrestateChangeReplansWithoutWrongAction(t *testing.T) {
	graph, request := preparationModel(t), preparationRequest()
	store := request.Observed
	reads, calls := 0, 0
	result := ExecutePreparation(context.Background(), graph, request,
		func(context.Context) (BusinessObservation, error) {
			reads++
			if reads == 2 {
				store.Values["order.stored"] = json.RawMessage("true")
			}
			return store, nil
		},
		func(_ context.Context, step PlannedPreparationStep) (PreparationReceipt, error) {
			calls++
			return preparationTestReceipt(step), nil
		},
	)
	if result.Status != "ALREADY_SATISFIED" || !result.TargetReady || calls != 0 || string(result.InitialObserved.Values["order.stored"]) != "false" || string(result.FinalObserved.Values["order.stored"]) != "true" {
		t.Fatalf("stale plan forced an action after observed event: %+v", result)
	}
	if err := ValidatePreparationExecution(graph, request, result); err != nil {
		t.Fatalf("observed event replan cannot be independently replayed: %v", err)
	}
}

func TestBusinessPreparationMissingReadbackIdentityAndReceiptBlock(t *testing.T) {
	for _, mode := range []string{"unknown-post", "changed-identity", "missing-capture"} {
		t.Run(mode, func(t *testing.T) {
			graph, request := preparationModel(t), preparationRequest()
			store := request.Observed
			calls := 0
			result := ExecutePreparation(context.Background(), graph, request,
				func(context.Context) (BusinessObservation, error) { return store, nil },
				func(_ context.Context, step PlannedPreparationStep) (PreparationReceipt, error) {
					calls++
					for _, effect := range step.Effects {
						store.Values[effect.Subject+"."+effect.Attribute] = effect.Value
					}
					receipt := preparationTestReceipt(step)
					switch mode {
					case "unknown-post":
						delete(store.Values, "other.active")
					case "changed-identity":
						store.Bindings["other"] = "another-account"
					case "missing-capture":
						receipt.Action.ResponseSHA256 = ""
					}
					return receipt, nil
				},
			)
			if result.Status != "BLOCKED" || result.TargetReady || calls != 1 || len(result.Steps) != 1 {
				t.Fatalf("%s did not block target: %+v", mode, result)
			}
		})
	}
}

func TestBusinessPreparationCancellationCannotRunAnyAdapter(t *testing.T) {
	graph, request := preparationModel(t), preparationRequest()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	result := ExecutePreparation(ctx, graph, request,
		func(context.Context) (BusinessObservation, error) { calls++; return request.Observed, nil },
		func(_ context.Context, step PlannedPreparationStep) (PreparationReceipt, error) {
			calls++
			return preparationTestReceipt(step), nil
		},
	)
	if result.Status != "BLOCKED" || result.TargetReady || calls != 0 {
		t.Fatalf("cancelled preparation invoked callbacks: %+v calls=%d", result, calls)
	}
}

func TestBusinessPreparationOfflineVerifierRejectsMarkersAndAlteredExpectedActual(t *testing.T) {
	graph, request := preparationModel(t), preparationRequest()
	store, _ := cloneBusinessJSON(request.Observed)
	execution := ExecutePreparation(context.Background(), graph, request,
		func(context.Context) (BusinessObservation, error) { return store, nil },
		func(_ context.Context, step PlannedPreparationStep) (PreparationReceipt, error) {
			for _, effect := range step.Effects {
				store.Values[effect.Subject+"."+effect.Attribute] = effect.Value
			}
			return preparationTestReceipt(step), nil
		},
	)
	if err := ValidatePreparationExecution(graph, request, execution); err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name   string
		change func(*PreparationExecution)
	}{
		{"marker-only", func(e *PreparationExecution) { e.Readbacks = nil; e.Plans = nil; e.Steps = nil }},
		{"changed-model", func(e *PreparationExecution) { e.ContractSHA256 = strings.Repeat("0", 64) }},
		{"missing-receipt", func(e *PreparationExecution) { e.Steps[0].Receipt.Action.ResponseSHA256 = "" }},
		{"rewritten-expectation", func(e *PreparationExecution) { e.Steps[0].Planned.Ensures = BusinessPredicate{Op: "true"} }},
		{"invented-after", func(e *PreparationExecution) { e.Steps[0].After.Values["other.active"] = json.RawMessage("false") }},
		{"changed-final", func(e *PreparationExecution) { e.FinalObserved.Values["order.stored"] = json.RawMessage("false") }},
		{"omitted-dependency", func(e *PreparationExecution) { e.Steps[len(e.Steps)-1].Dependencies = nil }},
		{"forged-budget", func(e *PreparationExecution) { e.ExploredStates = 0 }},
		{"skipped-actual-action", func(e *PreparationExecution) { e.Steps = e.Steps[1:] }},
		{"fake-parallel", func(e *PreparationExecution) { e.ParallelExecuted = true }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed, _ := cloneBusinessJSON(execution)
			mutation.change(&changed)
			if err := ValidatePreparationExecution(graph, request, changed); err == nil {
				t.Fatal("altered preparation proof qualified solely from TargetReady")
			}
		})
	}
}
