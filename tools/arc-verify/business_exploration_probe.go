package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// A new attempt does not inherit an earlier run's variable/cookie maps. Only a
// non-secret entity explicitly extracted from a fresh independent oracle may be
// rebound. Requiring the same pointer as a recorded measurement makes the
// identity reproducible from the child run's normalized before facts.
func rebindBusinessCampaignEntities(step RunnerStep, docs map[oracleIdentity]json.RawMessage, sess *execSession) ([]ExtractedBinding, error) {
	rules := []ExtractRule{}
	for _, extract := range step.Call.Extract {
		if extract.Source != "oracle" || extract.EntityAlias == "" || extract.SessionScope != "" || extract.Redact {
			continue
		}
		measured := false
		for _, oracle := range step.Oracles {
			if oracle.ID == extract.OracleID {
				for _, path := range oracle.Measurements {
					measured = measured || path == extract.Path
				}
			}
		}
		if !measured {
			return nil, fmt.Errorf("campaign entity rebinding %s needs the same pointer in its registered independent oracle measurements", extract.Name)
		}
		extract.Required = false // absent before creation is not a fabricated ID
		rules = append(rules, extract)
	}
	return applyExtractions(rules, 0, 1, step.Actor, nil, docs, sess)
}

func (d db) observeBusinessCampaignStep(ctx context.Context, runtime businessCampaignVariantRuntime, step RunnerStep, sess *execSession, includeInput bool) (ModelObservation, error) {
	obs := ModelObservation{EvidenceVersion: runtimeEvidenceVersion, Node: step.Node, Transition: step.Transition, Round: "", RunnerSHA256: runtime.Runner.SHA256, CodeSHA256: runtime.Runner.CodeSHA256, Input: json.RawMessage(`{}`), Participants: runtime.Participants, Oracles: []OracleObservation{}, Actions: []ActionObservation{}}
	docs := map[oracleIdentity]json.RawMessage{}
	for _, oracle := range step.Oracles {
		single, cancel := context.WithTimeout(ctx, time.Duration(runtime.Runner.Manifest.StepTimeoutMS)*time.Millisecond)
		facts, hash, doc, err := readOracleWithSession(single, oracle, runtime.Binding, runtime.Accounts[step.Actor], step.Actor, runtime.Runner.Manifest.IsolationScope, sess)
		cancel()
		if err != nil {
			return obs, err
		}
		obs.Oracles = append(obs.Oracles, OracleObservation{ActionIndex: 1, Actor: step.Actor, ID: oracle.ID, Boundary: oracle.Boundary, Before: facts, BeforeSHA256: hash})
		docs[oracleIdentity{ActionIndex: 1, Actor: step.Actor, ID: oracle.ID}] = doc
	}
	if includeInput {
		extracted, err := rebindBusinessCampaignEntities(step, docs, sess)
		if err != nil {
			return obs, err
		}
		obs.Extractions = extracted
	}
	obs.BeforeEntities, obs.BeforeSessions = sess.entitySlice(), sess.sessionSlice()
	if includeInput {
		request, err := captureRuntimeRequest(step, 1, sess)
		if err != nil {
			return obs, err
		}
		obs.Requests = []RequestObservation{request}
	}
	body, err := json.Marshal(obs)
	if err != nil {
		return obs, err
	}
	var parsed any
	if err := decodeUniqueJSON(body, &parsed); err != nil {
		return obs, err
	}
	if hasSensitiveKey(parsed) {
		return ModelObservation{}, errors.New("campaign probe contains credential-like data")
	}
	return obs, nil
}

func campaignObservationKey(c BusinessCampaign, runtime businessCampaignVariantRuntime, run string, obs ModelObservation, before BusinessObservation) string {
	// RawMessage preserves integer/decimal/exponent tokens. There is no
	// float64 normalization and no merge across request, actor, entity, source,
	// environment, fixture, run identity or execution history.
	key, _ := businessJSONHash(struct {
		Campaign    string                  `json:"campaign"`
		History     string                  `json:"history"`
		Run         string                  `json:"run"`
		Variant     BusinessCampaignVariant `json:"variant"`
		Binding     RunBinding              `json:"binding"`
		Observation ModelObservation        `json:"observation"`
		Before      BusinessObservation     `json:"before"`
	}{c.ID, c.HistorySHA256, run, businessCampaignStaticVariant(runtime.Variant), runtime.Binding, obs, before})
	return key
}

func (d db) probeBusinessCampaign(ctx context.Context, c BusinessCampaign, g Graph, runtime businessCampaignVariantRuntime, run string, lease businessCampaignLease) BusinessCampaignCandidate {
	step := runtime.Runner.Manifest.Steps[0]
	result := BusinessCampaignCandidate{VariantID: runtime.Variant.ID, Branch: runtime.Variant.Branch, Runner: runtime.Variant.Runner, Run: run, Eligibility: "BLOCKED", NextAllowed: c.NextAllowed.Allows(step.Node)}
	if !result.NextAllowed {
		result.Eligibility, result.Reason = "NEXT_DISALLOWED", "previous verified outcome restricts the next action"
		return result
	}
	// Known failed/running tasks never enter an automatic retry path.
	rows, err := d.query("SELECT status FROM tasks WHERE id="+quote(runtime.Variant.Task), 1)
	if err != nil {
		result.Reason = err.Error()
		return result
	}
	if len(rows) != 1 || rows[0][0] != "READY" && rows[0][0] != "PASS" {
		result.Reason = "frozen task is missing, already running or has an unresolved failure; campaign cannot retry it"
		return result
	}
	if err := d.validateModelDependencies(g, runtime.Obligation, c.Round); err != nil {
		result.Reason = err.Error()
		return result
	}
	sess := newExecSession(run, runtime.Participants)
	sess.campaignGuard = func() error { return d.validateBusinessCampaignLease(c.ID, lease) }
	obs, err := d.observeBusinessCampaignStep(ctx, runtime, step, sess, true)
	obs.Round = c.Round
	result.Observation = &obs
	if err != nil {
		result.Reason = campaignReason(err.Error())
		return result
	}
	result.Input = &obs.Requests[0]
	judged := stepRule(indexModel(g), runtime.Obligation, step.Transition)
	before, err := runtimeBusinessObservation(c.Flow, judged.Oracle, step, 1, true, &obs)
	if err != nil {
		result.Reason = err.Error()
		return result
	}
	result.Before = &before
	result.StateKey = campaignObservationKey(c, runtime, run, obs, before)
	decision := FreezeBusinessExpectation(g, step.Node, before)
	result.Decision = &decision
	result.Eligibility, result.Reason = decision.Status, decision.Reason
	if decision.Status == "MODEL_ERROR" {
		return result
	}
	if decision.Status == "SPECIFICATION_GAP" {
		declaredHere := false
		for _, branch := range g.BusinessContract.Branches {
			if branch.Action == step.Node && (branch.Flow == "" || branch.Flow == c.Flow) && (len(branch.FromStates) == 0 || containsBusinessID(branch.FromStates, before.State)) {
				declaredHere = true
			}
		}
		if !declaredHere {
			result.Eligibility, result.Reason = "NOT_APPLICABLE", "this action's declared from_states exclude the current independently observed state"
		}
	}
	if decision.Status == "READY" && decision.Frozen != nil {
		if decision.Frozen.Branch != runtime.Variant.Branch {
			result.Eligibility, result.Reason = "NOT_APPLICABLE", "actual state/conditions select another declared branch: "+decision.Frozen.Branch
		} else if step.Transition != "" && (decision.Frozen.Transition != "" && decision.Frozen.Transition != step.Transition || before.State != indexModel(g).edges[step.Transition].From) {
			result.Eligibility, result.Reason = "NOT_APPLICABLE", "actual state does not match this registered transition"
		}
	}
	conditions := runtime.Obligation.Definition.Preconditions
	if step.Transition != "" {
		conditions = indexModel(g).contracts[step.Transition].Preconditions
	}
	conditionsReady := false
	for _, item := range obs.Oracles {
		if item.Actor == step.Actor && item.ID == judged.Oracle {
			conditionsReady = observedConditions(item.Before.Conditions, conditions, judged.Conditions)
		}
	}
	if result.Eligibility == "READY" && !conditionsReady {
		result.Eligibility, result.Reason = "NOT_APPLICABLE", "actual independent conditions do not satisfy the registered test's preconditions"
	}
	preparation := runtime.Runner.Manifest.Preparation
	if preparation == nil {
		return result
	}
	prepObs, err := d.observeBusinessCampaignStep(ctx, runtime, preparation.Observer, sess, false)
	if err != nil {
		result.Eligibility, result.Reason = "BLOCKED", "preparation observation: "+err.Error()
		return result
	}
	prepBefore, err := runtimeBusinessObservation(c.Flow, preparation.Observer.Oracles[0].ID, preparation.Observer, 1, true, &prepObs)
	if err != nil {
		result.Eligibility, result.Reason = "BLOCKED", "preparation observation: "+err.Error()
		return result
	}
	prepObs.Round = c.Round
	result.PreparationBefore, result.PreparationObservation = &prepBefore, &prepObs
	plan := PlanPreparation(g, PreparationRequest{Observed: prepBefore, Goal: preparation.Goal, MaxSteps: preparation.MaxSteps, MaxStates: preparation.MaxStates})
	result.Preparation = &plan
	if plan.Status == "MODEL_ERROR" {
		result.Eligibility, result.Reason = "MODEL_ERROR", plan.Reason
		return result
	}
	if plan.Status == "ALREADY_SATISFIED" {
		return result
	}
	if plan.Status != "READY" {
		result.Eligibility, result.Reason = "BLOCKED", "preparation is not safely planned: "+plan.Reason
		return result
	}
	// This is a feasibility calculation only. No predicted value replaces
	// Before, LastActual or coverage. ExecutePreparation independently reads
	// back every action and target execute freezes again from that real state.
	predicted := before
	for _, layer := range plan.Layers {
		for _, planned := range layer.Steps {
			if _, found := preparation.Adapters[planned.ID]; !found {
				result.Eligibility, result.Reason = "BLOCKED", "preparation has no registered adapter for "+planned.ID
				return result
			}
			predicted = preparationWithEffects(predicted, planned.Effects)
		}
	}
	preparedDecision := FreezeBusinessExpectation(g, step.Node, predicted)
	if preparedDecision.Status != "READY" || preparedDecision.Frozen == nil || preparedDecision.Frozen.Branch != runtime.Variant.Branch {
		// In particular, a wrong request amount/type cannot be repaired by a
		// preparation that only changes account state.
		if preparedDecision.Status == "MODEL_ERROR" {
			result.Eligibility = "MODEL_ERROR"
		}
		return result
	}
	if err := businessCampaignPreparedPreconditions(g, runtime.Obligation, step, obs, before, predicted); err != nil {
		result.Eligibility, result.Reason = "BLOCKED", err.Error()
		return result
	}
	result.Eligibility, result.Reason = "PREPARABLE", "explicit preparation plan may enable the selected branch; target still requires fresh readback and an exact pre-action freeze"
	return result
}

func equalBusinessCampaignJSON(a, b any) bool {
	left, errA := json.Marshal(a)
	right, errB := json.Marshal(b)
	return errA == nil && errB == nil && sameJSONValue(left, right)
}

func verifyBusinessCampaignRebindings(step RunnerStep, observed ModelObservation) error {
	for _, entity := range observed.BeforeEntities {
		matched := false
		for _, extract := range observed.Extractions {
			if extract.Step != 0 || extract.Source != "oracle" || extract.EntityAlias != entity.Alias || extract.Value != entity.ID || extract.SourceActor != step.Actor || extract.ActionIndex != 1 {
				continue
			}
			for _, oracle := range step.Oracles {
				if oracle.ID != extract.OracleID {
					continue
				}
				for key, path := range oracle.Measurements {
					if path != extract.Path {
						continue
					}
					for _, reading := range observed.Oracles {
						if reading.Actor != step.Actor || reading.ActionIndex != 1 || reading.ID != oracle.ID {
							continue
						}
						value, err := scalarJSONString(reading.Before.Values[key])
						if err == nil && value == entity.ID && extract.SHA256 == digestBytes([]byte(value)) && entity.OwnerActor == extract.OwnerActor && entity.OwnerPrincipal == extract.OwnerPrincipal {
							matched = true
						}
					}
				}
			}
		}
		if !matched {
			return fmt.Errorf("campaign before entity %s has no independently measured rebinding", entity.Alias)
		}
	}
	if len(observed.BeforeSessions) != 0 {
		return errors.New("campaign attempts cannot inherit unobserved sessions from another run")
	}
	return nil
}
