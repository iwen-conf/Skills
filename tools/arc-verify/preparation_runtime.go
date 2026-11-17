package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// PreparationReceipt carries an actual adapter capture, not a caller supplied
// pass flag. The independent after observation is still mandatory.
type PreparationReceipt struct {
	Action    ActionObservation `json:"action"`
	Artifacts []string          `json:"artifacts,omitempty"`
}

type PreparationActualStep struct {
	Index        int                        `json:"index"`
	BeforeRead   int                        `json:"before_read"`
	AfterRead    int                        `json:"after_read"`
	Planned      PlannedPreparationStep     `json:"planned"`
	Before       BusinessObservation        `json:"before"`
	Requirements TruthEvaluation            `json:"requirements"`
	Dependencies map[string]TruthEvaluation `json:"dependencies"`
	Receipt      PreparationReceipt         `json:"receipt"`
	After        *BusinessObservation       `json:"after,omitempty"`
	Ensures      TruthEvaluation            `json:"ensures"`
	Status       string                     `json:"status"`
	Reason       string                     `json:"reason"`
}

type PreparationReadback struct {
	Index       int                  `json:"index"`
	Phase       string               `json:"phase"`
	Observation *BusinessObservation `json:"observation,omitempty"`
	Error       string               `json:"error,omitempty"`
}

type PreparationExecution struct {
	Status            string                  `json:"status"`
	Reason            string                  `json:"reason"`
	ContractSHA256    string                  `json:"contract_sha256"`
	InitialObserved   BusinessObservation     `json:"initial_observed"`
	FinalObserved     BusinessObservation     `json:"final_observed"`
	Plans             []PreparationPlan       `json:"plans"`
	Steps             []PreparationActualStep `json:"steps"`
	Readbacks         []PreparationReadback   `json:"readbacks"`
	ObservationReads  int                     `json:"observation_reads"`
	ExploredStates    int                     `json:"explored_states"`
	ParallelExecuted  bool                    `json:"parallel_executed"`
	TargetReady       bool                    `json:"target_ready"`
	FirstFailureIndex *int                    `json:"first_failure_index,omitempty"`
}

func preparationReceiptError(receipt PreparationReceipt, planned PlannedPreparationStep) error {
	action := receipt.Action
	if action.Node != planned.Action || action.Actor == "" || action.Principal == "" {
		return fmt.Errorf("preparation action receipt does not identify its declared action and actual actor")
	}
	for _, digest := range []string{action.RequestSHA256, action.ResponseSHA256} {
		if len(digest) != 64 {
			return fmt.Errorf("preparation action lacks captured request/response SHA-256")
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return fmt.Errorf("preparation action has invalid capture digest")
		}
	}
	started, firstError := time.Parse(time.RFC3339Nano, action.Started)
	completed, secondError := time.Parse(time.RFC3339Nano, action.Completed)
	if firstError != nil || secondError != nil || completed.Before(started) {
		return fmt.Errorf("preparation action lacks a valid started/completed interval")
	}
	if action.Denied {
		return fmt.Errorf("preparation action was denied")
	}
	return nil
}

func preparationIdentityContinuity(contract *BusinessContract, before, after BusinessObservation) TruthEvaluation {
	unknown := []string{}
	for _, subject := range contract.Subjects {
		identity := before.Bindings[subject.ID]
		if identity == "" {
			continue
		}
		if after.Bindings[subject.ID] == "" {
			unknown = append(unknown, "binding:"+subject.ID)
			continue
		}
		if after.Bindings[subject.ID] != identity {
			return TruthEvaluation{Value: businessFalse, Reason: "business identity changed for " + subject.ID}
		}
	}
	if len(unknown) > 0 {
		return unknownBusiness(unknown, "previously observed business identity is now unknown")
	}
	return businessTruth(true)
}

// ExecutePreparation is an execution seam used by the existing runner. It
// never performs IO itself: observe must independently read the SUT, and act
// must invoke a registered adapter. It automatically replans from actual
// observations, serializes layers deliberately, and never writes predictions
// into Actual. Any uncertainty stops before the caller's target action.
func ExecutePreparation(ctx context.Context, g Graph, request PreparationRequest,
	observe func(context.Context) (BusinessObservation, error),
	act func(context.Context, PlannedPreparationStep) (PreparationReceipt, error),
	admission ...func(PreparationPlan, BusinessObservation) error,
) PreparationExecution {
	result := PreparationExecution{Status: "BLOCKED", Plans: []PreparationPlan{}, Steps: []PreparationActualStep{}, Readbacks: []PreparationReadback{}, ParallelExecuted: false}
	if observe == nil || act == nil {
		result.Reason = "automatic preparation requires independent observer and registered action adapters"
		return result
	}
	if len(admission) > 1 {
		result.Reason = "automatic preparation accepts at most one target-scope admission check"
		return result
	}
	if g.BusinessContract == nil {
		result.Reason = "automatic preparation requires a declared deterministic business contract"
		return result
	}
	if err := ValidateBusinessContract(g); err != nil {
		result.Reason = "invalid preparation model: " + err.Error()
		return result
	}
	if request.MaxSteps < 1 || request.MaxSteps > 128 || request.MaxStates < 1 || request.MaxStates > 10000 {
		result.Reason = "automatic preparation requires bounded max_steps and max_states"
		return result
	}
	model, err := cloneBusinessJSON(g)
	if err != nil {
		result.Reason = err.Error()
		return result
	}
	request, err = cloneBusinessJSON(request)
	if err != nil {
		result.Reason = err.Error()
		return result
	}
	result.ContractSHA256, _ = businessJSONHash(model)
	read := func(phase string) (BusinessObservation, int, error) {
		if err := ctx.Err(); err != nil {
			return BusinessObservation{}, -1, err
		}
		index := len(result.Readbacks)
		result.ObservationReads++
		observed, err := observe(ctx)
		if err != nil {
			result.Readbacks = append(result.Readbacks, PreparationReadback{Index: index, Phase: phase, Error: err.Error()})
			return BusinessObservation{}, index, err
		}
		// Detach evidence before another callback can reuse a mutable map.
		copied, err := cloneBusinessJSON(observed)
		if err != nil {
			result.Readbacks = append(result.Readbacks, PreparationReadback{Index: index, Phase: phase, Error: err.Error()})
			return BusinessObservation{}, index, err
		}
		result.Readbacks = append(result.Readbacks, PreparationReadback{Index: index, Phase: phase, Observation: &copied})
		return copied, index, nil
	}
	initial, lastRead, err := read("initial")
	if err != nil {
		result.Reason = "initial independent preparation read failed: " + err.Error()
		return result
	}
	result.InitialObserved, result.FinalObserved = initial, initial
	attributes := businessAttributeIndex(model.BusinessContract)
	count := 0
	if err := validateBusinessPredicate(request.Goal, attributes, 0, &count); err != nil {
		result.Reason = "invalid preparation goal: " + err.Error()
		return result
	}
	preparations := map[string]PreparationAction{}
	for _, preparation := range model.BusinessContract.Preparation {
		preparations[preparation.ID] = preparation
	}
	maxReads := request.MaxSteps*4 + 4
	for len(result.Steps) < request.MaxSteps && result.ObservationReads < maxReads {
		if err := ctx.Err(); err != nil {
			result.Reason = "preparation deadline or cancellation: " + err.Error()
			return result
		}
		goal := evaluateBusinessPredicate(request.Goal, result.FinalObserved, attributes)
		if goal.Value == businessTrue {
			result.Status, result.Reason, result.TargetReady = "READY", "preparation goal independently observed; target may now verify and freeze its own branch", true
			if len(result.Steps) == 0 {
				result.Status = "ALREADY_SATISFIED"
			}
			return result
		}
		remainingStates := request.MaxStates - result.ExploredStates
		if remainingStates < 1 {
			result.Reason = "automatic preparation exhausted its cumulative planning state budget"
			return result
		}
		planRequest := request
		planRequest.Observed, planRequest.MaxSteps, planRequest.MaxStates = result.FinalObserved, request.MaxSteps-len(result.Steps), remainingStates
		plan := PlanPreparation(model, planRequest)
		plan.ObservedReadback = lastRead
		result.ExploredStates += plan.ExploredStates
		result.Plans = append(result.Plans, plan)
		if plan.Status != "READY" || len(plan.Layers) == 0 || len(plan.Layers[0].Steps) == 0 {
			result.Reason = "automatic preparation cannot schedule a proven next step: " + plan.Status + ": " + plan.Reason
			return result
		}
		planned, err := cloneBusinessJSON(plan.Layers[0].Steps[0])
		if err != nil {
			result.Reason = err.Error()
			return result
		}
		before, beforeRead, err := read("pre_action")
		if err != nil {
			result.Reason = "preparation pre-action read failed: " + err.Error()
			return result
		}
		identity := preparationIdentityContinuity(model.BusinessContract, result.FinalObserved, before)
		if identity.Value != businessTrue {
			result.Reason = "preparation pre-action identity not continuous: " + identity.Reason
			return result
		}
		previousHash, _ := businessJSONHash(result.FinalObserved)
		currentHash, _ := businessJSONHash(before)
		result.FinalObserved = before
		lastRead = beforeRead
		if previousHash != currentHash {
			// An asynchronous event may have changed the prestate. Replan from
			// it, instead of forcing the old planned step to look applicable.
			continue
		}
		if len(admission) == 1 && admission[0] != nil {
			admittedPlan, planError := cloneBusinessJSON(plan)
			admittedBefore, beforeError := cloneBusinessJSON(before)
			if planError != nil || beforeError != nil {
				result.Reason = "preparation target-scope admission could not freeze its plan or actual observation"
				return result
			}
			if err := admission[0](admittedPlan, admittedBefore); err != nil {
				result.Reason = "preparation target-scope admission: " + err.Error()
				return result
			}
		}
		step := PreparationActualStep{Index: len(result.Steps), BeforeRead: beforeRead, AfterRead: -1, Planned: planned, Before: before, Dependencies: map[string]TruthEvaluation{}, Status: "BLOCKED", Ensures: unknownBusiness(nil, "action has not executed")}
		step.Requirements = evaluateBusinessPredicate(planned.Requires, before, attributes)
		ready := step.Requirements.Value == businessTrue
		for _, dependency := range preparations[planned.ID].DependsOn {
			reading := preparationSatisfaction(preparations[dependency], before, attributes)
			step.Dependencies[dependency] = reading
			ready = ready && reading.Value == businessTrue
		}
		if !ready {
			step.Reason = "fresh independent preparation requirements or dependency effects are not true"
			result.Steps = append(result.Steps, step)
			result.FirstFailureIndex = &step.Index
			result.Reason = step.Reason
			return result
		}
		callbackPlan, cloneError := cloneBusinessJSON(planned)
		if cloneError != nil {
			result.Reason = cloneError.Error()
			return result
		}
		step.Receipt, err = act(ctx, callbackPlan)
		actError := err
		if copiedReceipt, copyError := cloneBusinessJSON(step.Receipt); copyError == nil {
			step.Receipt = copiedReceipt
		} else if actError == nil {
			actError = copyError
		}
		// Always try to capture the resulting state, including an action error.
		// A timeout after send may have committed; it must never be auto-retried.
		after, afterRead, readError := read("post_action")
		step.AfterRead = afterRead
		if readError == nil {
			step.After = &after
			result.FinalObserved = after
			lastRead = afterRead
			step.Ensures = evaluateBusinessPredicate(planned.Ensures, after, attributes)
		}
		captureError := preparationReceiptError(step.Receipt, planned)
		switch {
		case actError != nil:
			step.Reason = "preparation action failed or its result is uncertain: " + actError.Error()
		case captureError != nil:
			step.Reason = captureError.Error()
		case readError != nil:
			step.Reason = "preparation independent post-action read failed: " + readError.Error()
		case preparationIdentityContinuity(model.BusinessContract, before, after).Value != businessTrue:
			step.Reason = "preparation post-action business identity changed or is unknown"
		case step.Ensures.Value == businessFalse:
			step.Status, step.Reason = "FAIL", "actual preparation effects violate the frozen ensures"
		case step.Ensures.Value != businessTrue:
			step.Reason = "preparation effects are not independently known"
		default:
			step.Status, step.Reason = "PASS", "actual action capture and independent poststate satisfy the frozen preparation contract"
		}
		result.Steps = append(result.Steps, step)
		if step.Status != "PASS" {
			result.FirstFailureIndex = &step.Index
			result.Reason = step.Reason
			return result
		}
	}
	// The last permitted action can reach the goal. Check its actual readback
	// before declaring the step budget exhausted.
	if evaluateBusinessPredicate(request.Goal, result.FinalObserved, attributes).Value == businessTrue {
		result.Status, result.Reason, result.TargetReady = "READY", "preparation goal independently observed at the action budget boundary", true
		return result
	}
	result.Reason = fmt.Sprintf("automatic preparation exhausted %d actions or %d independent reads; target remains blocked", request.MaxSteps, maxReads)
	return result
}

// PreparationFailureSummary is intentionally concise enough for the existing
// run reason while the complete evidence remains in PreparationExecution.
func PreparationFailureSummary(execution PreparationExecution) string {
	return strings.TrimSpace(execution.Status + ": " + execution.Reason)
}

func sameBusinessJSON(left, right any) bool {
	leftHash, leftError := businessJSONHash(left)
	rightHash, rightError := businessJSONHash(right)
	return leftError == nil && rightError == nil && leftHash == rightHash
}

// ValidatePreparationExecution replays the captured preparation proof without
// IO. The execution marker alone is never evidence: every plan, observation,
// receipt, dependency, frozen ensures and final goal is independently checked.
// The existing runner additionally binds these captures to its manifest and
// original RAW capture receipt; this helper cannot authenticate arbitrary IO.
func ValidatePreparationExecution(g Graph, request PreparationRequest, execution PreparationExecution) error {
	if g.BusinessContract == nil {
		return fmt.Errorf("preparation proof has no business contract")
	}
	if err := ValidateBusinessContract(g); err != nil {
		return err
	}
	if request.MaxSteps < 1 || request.MaxSteps > 128 || request.MaxStates < 1 || request.MaxStates > 10000 {
		return fmt.Errorf("preparation proof has invalid execution budgets")
	}
	contractHash, err := businessJSONHash(g)
	if err != nil || execution.ContractSHA256 != contractHash {
		return fmt.Errorf("preparation proof belongs to another frozen model")
	}
	if !execution.TargetReady || execution.Status != "READY" && execution.Status != "ALREADY_SATISFIED" || execution.FirstFailureIndex != nil || execution.ParallelExecuted {
		return fmt.Errorf("preparation did not complete as a verified serial execution")
	}
	if len(execution.Steps) > request.MaxSteps || len(execution.Readbacks) < 1 || len(execution.Readbacks) > request.MaxSteps*4+4 || execution.ObservationReads != len(execution.Readbacks) {
		return fmt.Errorf("preparation evidence exceeds budgets or omits independent readbacks")
	}
	attributes := businessAttributeIndex(g.BusinessContract)
	count := 0
	if err := validateBusinessPredicate(request.Goal, attributes, 0, &count); err != nil {
		return err
	}
	readbacks := execution.Readbacks
	for index, readback := range readbacks {
		if readback.Index != index || readback.Observation == nil || readback.Error != "" {
			return fmt.Errorf("preparation readback %d is missing or failed", index)
		}
		if index == 0 && readback.Phase != "initial" || index > 0 && readback.Phase != "pre_action" && readback.Phase != "post_action" {
			return fmt.Errorf("preparation readback %d has invalid phase", index)
		}
		if index > 0 && preparationIdentityContinuity(g.BusinessContract, *readbacks[index-1].Observation, *readback.Observation).Value != businessTrue {
			return fmt.Errorf("preparation readbacks do not preserve business identity")
		}
	}
	if !sameBusinessJSON(execution.InitialObserved, *readbacks[0].Observation) || !sameBusinessJSON(execution.FinalObserved, *readbacks[len(readbacks)-1].Observation) {
		return fmt.Errorf("preparation initial/final observations disagree with captured readbacks")
	}
	if evaluateBusinessPredicate(request.Goal, execution.FinalObserved, attributes).Value != businessTrue {
		return fmt.Errorf("preparation final observed goal is false or unknown")
	}
	if len(execution.Steps) == 0 && execution.Status != "ALREADY_SATISFIED" || len(execution.Steps) > 0 && execution.Status != "READY" {
		return fmt.Errorf("preparation execution marker disagrees with actual action count")
	}
	priorPlanRead, totalStates := -1, 0
	for _, plan := range execution.Plans {
		index := plan.ObservedReadback
		if index < 0 || index >= len(readbacks) || index <= priorPlanRead {
			return fmt.Errorf("preparation plan is not attached to an ordered independent observation")
		}
		priorPlanRead = index
		completedSteps := 0
		for _, step := range execution.Steps {
			if step.AfterRead <= index {
				completedSteps++
			}
		}
		replayRequest := request
		replayRequest.Observed, replayRequest.MaxSteps, replayRequest.MaxStates = *readbacks[index].Observation, request.MaxSteps-completedSteps, request.MaxStates-totalStates
		if replayRequest.MaxSteps < 1 || replayRequest.MaxStates < 1 {
			return fmt.Errorf("preparation plan exceeds the remaining budget")
		}
		recomputed := PlanPreparation(g, replayRequest)
		recomputed.ObservedReadback = index
		if recomputed.Status != "READY" || !sameBusinessJSON(recomputed, plan) {
			return fmt.Errorf("recorded preparation plan differs from deterministic observed-state planning")
		}
		totalStates += recomputed.ExploredStates
	}
	if totalStates != execution.ExploredStates || totalStates > request.MaxStates {
		return fmt.Errorf("preparation cumulative planning budget does not match its proof")
	}
	preparations := map[string]PreparationAction{}
	for _, preparation := range g.BusinessContract.Preparation {
		preparations[preparation.ID] = preparation
	}
	lastAfter, usedPostReads := 0, map[int]bool{}
	for index, step := range execution.Steps {
		if step.Index != index || step.Status != "PASS" || step.BeforeRead <= lastAfter || step.AfterRead != step.BeforeRead+1 || step.AfterRead >= len(readbacks) || step.After == nil {
			return fmt.Errorf("preparation step %d has incomplete or unordered action evidence", index)
		}
		lastAfter = step.AfterRead
		usedPostReads[step.AfterRead] = true
		if readbacks[step.BeforeRead].Phase != "pre_action" || readbacks[step.AfterRead].Phase != "post_action" || !sameBusinessJSON(step.Before, *readbacks[step.BeforeRead].Observation) || !sameBusinessJSON(*step.After, *readbacks[step.AfterRead].Observation) {
			return fmt.Errorf("preparation step %d disagrees with independent before/after captures", index)
		}
		declared, exists := preparations[step.Planned.ID]
		if !exists {
			return fmt.Errorf("preparation step %d names an undeclared preparation action", index)
		}
		want := PlannedPreparationStep{ID: declared.ID, Action: declared.Action, Requires: declared.Requires, Ensures: declared.Ensures, Effects: declared.Effects, SourceRef: declared.SourceRef}
		if !sameBusinessJSON(step.Planned, want) {
			return fmt.Errorf("preparation step %d rewrites its frozen requires/effects/ensures", index)
		}
		var selectedPlan *PreparationPlan
		for planIndex := range execution.Plans {
			plan := &execution.Plans[planIndex]
			if plan.ObservedReadback < step.BeforeRead {
				selectedPlan = plan
			}
		}
		if selectedPlan == nil || len(selectedPlan.Layers) == 0 || len(selectedPlan.Layers[0].Steps) == 0 || !sameBusinessJSON(selectedPlan.Layers[0].Steps[0], step.Planned) {
			return fmt.Errorf("preparation step %d was not selected by the deterministic plan", index)
		}
		beforeHash, err := businessJSONHash(step.Before)
		if err != nil || selectedPlan.ObservedSHA256 != beforeHash {
			return fmt.Errorf("preparation step %d executed after its planned prestate changed", index)
		}
		requirements := evaluateBusinessPredicate(declared.Requires, step.Before, attributes)
		if requirements.Value != businessTrue || !sameBusinessJSON(requirements, step.Requirements) {
			return fmt.Errorf("preparation step %d lacks verified actual requirements", index)
		}
		if len(step.Dependencies) != len(declared.DependsOn) {
			return fmt.Errorf("preparation step %d omits dependency observations", index)
		}
		for _, dependency := range declared.DependsOn {
			reading := preparationSatisfaction(preparations[dependency], step.Before, attributes)
			if reading.Value != businessTrue || !sameBusinessJSON(reading, step.Dependencies[dependency]) {
				return fmt.Errorf("preparation step %d depends on an unverified actual effect", index)
			}
		}
		if err := preparationReceiptError(step.Receipt, step.Planned); err != nil {
			return err
		}
		ensures := evaluateBusinessPredicate(declared.Ensures, *step.After, attributes)
		if ensures.Value != businessTrue || !sameBusinessJSON(ensures, step.Ensures) {
			return fmt.Errorf("preparation step %d actual readback violates frozen ensures", index)
		}
	}
	for _, readback := range readbacks {
		if readback.Phase == "post_action" && !usedPostReads[readback.Index] {
			return fmt.Errorf("preparation contains a post-action read without its action receipt")
		}
	}
	return nil
}
