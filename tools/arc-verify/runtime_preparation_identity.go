package main

import (
	"fmt"
	"strings"
)

func preparationGoalSubjects(goal BusinessPredicate) []string {
	subjects := []string{}
	for _, key := range businessPredicateRefs(goal) {
		subjects = append(subjects, strings.SplitN(key, ".", 2)[0])
	}
	return uniqueSortedBusiness(subjects)
}

func preparationPlanSubjects(goal BusinessPredicate, plan PreparationPlan) []string {
	subjects := preparationGoalSubjects(goal)
	for _, layer := range plan.Layers {
		for _, step := range layer.Steps {
			for _, effect := range step.Effects {
				subjects = append(subjects, effect.Subject)
			}
		}
	}
	return uniqueSortedBusiness(subjects)
}

// Preparation is scoped to the initial target batch. Later sequence steps may
// legitimately act on a different account or entity and are checked there.
func preparationTargetSubjects(g Graph, step RunnerStep) map[string]bool {
	needed := map[string]bool{}
	attributes := businessAttributeIndex(g.BusinessContract)
	for subject := range step.SubjectBindings {
		needed[subject] = true
	}
	for _, oracle := range step.Oracles {
		for key := range oracle.Measurements {
			if info, exists := attributes[key]; exists {
				needed[info.subject.ID] = true
			}
		}
	}
	for _, branch := range g.BusinessContract.Branches {
		if branch.Action == step.Node && (step.BusinessBranch == "" || branch.ID == step.BusinessBranch) {
			for _, subject := range preparationGoalSubjects(branch.When) {
				needed[subject] = true
			}
		}
	}
	return needed
}

func validatePreparationSubjectAgreement(g Graph, step RunnerStep, subjects []string, target, prepared map[string]string, knownOnly bool) error {
	needed := preparationTargetSubjects(g, step)
	requiresBinding := map[string]bool{}
	for _, subject := range g.BusinessContract.Subjects {
		requiresBinding[subject.ID] = subject.RequiresBinding
	}
	for _, subject := range uniqueSortedBusiness(subjects) {
		if !needed[subject] {
			continue
		}
		left, right := target[subject], prepared[subject]
		if left == "" || right == "" {
			// Static setup may create this entity. Known account mismatches
			// are still rejected immediately; missing identities must be
			// resolved before any actual planned preparation is admitted.
			if knownOnly || left == "" && right == "" && !requiresBinding[subject] {
				continue
			}
			return fmt.Errorf("preparation subject %s has no complete target identity", subject)
		}
		if left != right {
			return fmt.Errorf("preparation subject %s differs from the target identity", subject)
		}
	}
	return nil
}

func validateRuntimePreparationSubjectScope(g Graph, manifest RunnerManifest, subjects []string, prepared map[string]string, observed *ModelObservation, knownOnly bool) error {
	batches := runnerBatches(manifest)
	if len(batches) == 0 || g.BusinessContract == nil {
		return fmt.Errorf("preparation target scope is not declared")
	}
	registered, err := runtimeBusinessBindings(manifest.Preparation.Observer, true, observed)
	if err != nil {
		return err
	}
	for _, step := range batches[0] {
		target, err := runtimeBusinessBindings(step, true, observed)
		if err != nil {
			return err
		}
		if err := validatePreparationSubjectAgreement(g, step, subjects, registered, prepared, knownOnly); err != nil {
			return fmt.Errorf("registered preparation observer binding: %w", err)
		}
		if err := validatePreparationSubjectAgreement(g, step, subjects, target, prepared, knownOnly); err != nil {
			return err
		}
	}
	return nil
}

func validateRuntimePreparationPreflight(g Graph, manifest RunnerManifest, observed *ModelObservation) error {
	if manifest.Preparation == nil {
		return nil
	}
	prepared, err := runtimeBusinessBindings(manifest.Preparation.Observer, true, observed)
	if err != nil {
		return err
	}
	return validateRuntimePreparationSubjectScope(g, manifest, preparationGoalSubjects(manifest.Preparation.Goal), prepared, observed, true)
}

func validateRuntimePreparationPlanSubjectScope(g Graph, manifest RunnerManifest, plan PreparationPlan, prepared map[string]string, observed *ModelObservation) error {
	subjects := preparationPlanSubjects(manifest.Preparation.Goal, plan)
	if err := validateRuntimePreparationSubjectScope(g, manifest, subjects, prepared, observed, false); err != nil {
		return err
	}
	for _, layer := range plan.Layers {
		for _, planned := range layer.Steps {
			adapter, exists := manifest.Preparation.Adapters[planned.ID]
			if !exists {
				return fmt.Errorf("preparation has no registered adapter for %s", planned.ID)
			}
			explicit := []string{}
			for _, subject := range subjects {
				if _, mapped := adapter.SubjectBindings[subject]; mapped {
					explicit = append(explicit, subject)
				}
			}
			bindings, err := runtimeBusinessBindings(adapter, true, observed)
			if err != nil {
				return err
			}
			for _, targetStep := range runnerBatches(manifest)[0] {
				target, err := runtimeBusinessBindings(targetStep, true, observed)
				if err != nil {
					return err
				}
				if err := validatePreparationSubjectAgreement(g, targetStep, explicit, target, bindings, false); err != nil {
					return fmt.Errorf("registered preparation adapter %s: %w", planned.ID, err)
				}
			}
		}
	}
	return nil
}

// Rebuild only bindings available before the nominated setup action. Final
// setup.Entities may include extraction from a later preparation and cannot
// establish the identity of an earlier action.
func preparationContextBeforeAction(manifest RunnerManifest, setup *ModelObservation, beforeAction int) (ModelObservation, error) {
	context := ModelObservation{Participants: setup.Participants}
	if len(setup.BeforeEntities) != 0 || len(setup.BeforeSessions) != 0 {
		return context, fmt.Errorf("preparation setup must start with the new execution session's empty identity bindings")
	}
	entities := map[string]EntityBinding{}
	sessions := map[string]SessionBinding{}
	for actionIndex := 1; actionIndex < beforeAction; actionIndex++ {
		if actionIndex > len(setup.Actions) || setup.Actions[actionIndex-1].ActionIndex != actionIndex {
			return context, fmt.Errorf("preparation binding context has no preceding action %d", actionIndex)
		}
		var call AdapterCall
		if actionIndex <= len(manifest.Setup) {
			call = manifest.Setup[actionIndex-1].Call
		} else {
			index := actionIndex - len(manifest.Setup) - 1
			if setup.Preparation == nil || index >= len(setup.Preparation.Steps) {
				return context, fmt.Errorf("preparation binding context has no preceding preparation action")
			}
			adapter, exists := manifest.Preparation.Adapters[setup.Preparation.Steps[index].Planned.ID]
			if !exists {
				return context, fmt.Errorf("preparation binding context references an unregistered adapter")
			}
			call = adapter.Call
		}
		for _, extracted := range setup.Extractions {
			if extracted.ActionIndex != actionIndex {
				continue
			}
			var rule *ExtractRule
			for i := range call.Extract {
				if call.Extract[i].Name == extracted.Name {
					rule = &call.Extract[i]
					break
				}
			}
			if rule == nil || extracted.Step != -1 || extracted.SourceActor != setup.Actions[actionIndex-1].Actor || extracted.Source != rule.Source || extracted.Path != rule.Path || extracted.OracleID != rule.OracleID || extracted.EntityKind != rule.EntityKind || extracted.EntityAlias != firstNonEmpty(rule.EntityAlias, rule.Name) || extracted.Redacted != rule.Redact || !extracted.Redacted && extracted.SHA256 != digestBytes([]byte(extracted.Value)) {
				return context, fmt.Errorf("preparation binding extraction does not match its preceding registered action")
			}
			if !extracted.Redacted && extracted.EntityKind != "" {
				entities[extracted.EntityAlias] = EntityBinding{Alias: extracted.EntityAlias, Kind: extracted.EntityKind, ID: extracted.Value, OwnerActor: extracted.OwnerActor, OwnerPrincipal: extracted.OwnerPrincipal, SourceStep: extracted.Step, SourceField: extracted.Source + ":" + extracted.Path}
			}
			if rule.SessionScope != "" {
				sessions[rule.SessionScope] = SessionBinding{Scope: rule.SessionScope, Actor: extracted.OwnerActor, Principal: extracted.OwnerPrincipal, AcquiredStep: extracted.Step, Digest: extracted.SHA256, Active: true}
			}
		}
	}
	context.BeforeEntities, context.BeforeSessions = nil, nil
	for _, entity := range entities {
		context.BeforeEntities = append(context.BeforeEntities, entity)
	}
	for _, session := range sessions {
		context.BeforeSessions = append(context.BeforeSessions, session)
	}
	return context, nil
}

func validateRuntimePreparationIdentityEvidence(g Graph, manifest RunnerManifest, setup *ModelObservation) error {
	if manifest.Preparation == nil || setup.Preparation == nil {
		return fmt.Errorf("preparation subject proof lacks its registered contract or captured execution")
	}
	if err := validateRuntimePreparationPreflight(g, manifest, setup); err != nil {
		return err
	}
	execution := setup.Preparation
	for _, readback := range execution.Readbacks {
		if readback.Observation == nil {
			return fmt.Errorf("preparation subject proof lacks an actual readback")
		}
		beforeAction := len(manifest.Setup) + 1
		for _, step := range execution.Steps {
			if step.AfterRead <= readback.Index {
				beforeAction++
			}
		}
		context, err := preparationContextBeforeAction(manifest, setup, beforeAction)
		if err != nil {
			return err
		}
		if err := validateRuntimePreparationSubjectScope(g, manifest, preparationGoalSubjects(manifest.Preparation.Goal), readback.Observation.Bindings, &context, false); err != nil {
			return err
		}
	}
	for index, step := range execution.Steps {
		context, err := preparationContextBeforeAction(manifest, setup, len(manifest.Setup)+index+1)
		if err != nil {
			return err
		}
		var plan *PreparationPlan
		for i := range execution.Plans {
			if execution.Plans[i].ObservedReadback <= step.BeforeRead {
				plan = &execution.Plans[i]
			}
		}
		if plan == nil {
			return fmt.Errorf("preparation subject proof has no admitted plan")
		}
		if err := validateRuntimePreparationPlanSubjectScope(g, manifest, *plan, step.Before.Bindings, &context); err != nil {
			return err
		}
	}
	return nil
}
