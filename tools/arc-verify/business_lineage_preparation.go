package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (d db) buildPreparationLineageOverlays(g Graph, lineage *BusinessLineage, run SnapshotRun) ([]BusinessLineageOverlay, error) {
	result := []BusinessLineageOverlay{}
	if lineage == nil || g.BusinessContract == nil || run.EvidenceStatus != "PASS" {
		return result, nil
	}
	var setup *UnifiedStepFact
	for index := range run.Facts {
		fact := &run.Facts[index]
		if fact.SourceKind != "setup" || fact.Preparation == nil {
			continue
		}
		if setup != nil || fact.Seq != -1 || fact.Run != run.Trace.Run || fact.Graph != g.ID || fact.EvidenceStatus != "PASS" || fact.EvidenceVersion < runtimeEvidenceVersion {
			return nil, fmt.Errorf("preparation genealogy has no unique verified setup fact for this run")
		}
		setup = fact
	}
	if setup == nil {
		return result, nil
	}
	raw, setupHash, err := d.workbenchEvidence(run.Trace.Run, "setup", 0)
	if err != nil {
		return nil, fmt.Errorf("preparation genealogy setup RAW: %w", err)
	}
	if raw.Model == nil || raw.Model.Preparation == nil || setupHash != setup.EvidenceSHA256 || !sameBusinessJSON(raw.Model.Preparation, setup.Preparation) {
		return nil, fmt.Errorf("preparation genealogy differs from the original setup RAW capture")
	}
	contract, err := d.runTaskContract(run.Trace.Run)
	if err != nil {
		return nil, fmt.Errorf("preparation genealogy frozen run: %w", err)
	}
	if contract.Model == nil || contract.Receipt == nil || contract.Graph != g.ID {
		return nil, fmt.Errorf("preparation genealogy has no frozen model and execution receipt")
	}
	runner, err := d.registeredRunner(contract.Receipt.Runner)
	if err != nil {
		return nil, fmt.Errorf("preparation genealogy frozen observer manifest: %w", err)
	}
	if runner.Manifest.Preparation == nil {
		return nil, fmt.Errorf("preparation genealogy has no frozen observer manifest")
	}
	manifest, execution := runner.Manifest, raw.Model.Preparation
	if runner.SHA256 != contract.Receipt.RunnerSHA256 || !sameBusinessJSON(raw.Model.Participants, contract.Receipt.Participants) || execution.ContractSHA256 != lineage.ContractSHA256 {
		return nil, fmt.Errorf("preparation genealogy model, participants or runner identity differs from its receipt")
	}
	observer := manifest.Preparation.Observer
	actors := 0
	for _, participant := range contract.Receipt.Participants {
		if participant.Alias == observer.Actor && participant.Account != "" && participant.Principal != "" {
			actors++
		}
	}
	if actors != 1 {
		return nil, fmt.Errorf("preparation genealogy observer has no unique frozen account identity")
	}
	if len(execution.Readbacks) > manifest.Preparation.MaxSteps*4+4 || execution.ObservationReads != len(execution.Readbacks) {
		return nil, fmt.Errorf("preparation genealogy readback history exceeds or disagrees with its declared budget")
	}
	branches := map[string]BusinessBranchLineage{}
	for _, branch := range lineage.Branches {
		branches[branch.Branch] = branch
	}
	targets := map[string][]RunnerStep{}
	batches := runnerBatches(manifest)
	if len(batches) > 0 {
		for _, step := range batches[0] {
			if step.BusinessBranch == "" {
				continue
			}
			branch, exists := branches[step.BusinessBranch]
			if !exists || branch.Action != step.Node {
				return nil, fmt.Errorf("preparation genealogy initial target does not identify its declared branch")
			}
			targets[branch.Branch] = append(targets[branch.Branch], step)
		}
	}
	verified := setup.Verdict == "PASS" && execution.TargetReady
	if verified {
		p := manifest.Preparation
		if err := ValidatePreparationExecution(g, PreparationRequest{Goal: p.Goal, MaxSteps: p.MaxSteps, MaxStates: p.MaxStates}, *execution); err != nil {
			return nil, fmt.Errorf("preparation genealogy completion proof: %w", err)
		}
		if err := validateRuntimePreparationIdentityEvidence(g, manifest, raw.Model); err != nil {
			return nil, fmt.Errorf("preparation genealogy subject proof: %w", err)
		}
	}
	attributes := businessAttributeIndex(g.BusinessContract)
	preparations := map[string]PreparationAction{}
	for _, action := range g.BusinessContract.Preparation {
		preparations[action.ID] = action
	}
	readings := 0
	for _, branch := range lineage.Branches {
		steps := targets[branch.Branch]
		if len(steps) == 0 {
			continue
		}
		for index, readback := range execution.Readbacks {
			if readback.Index != index || index == 0 && readback.Phase != "initial" || index > 0 && readback.Phase != "pre_action" && readback.Phase != "post_action" {
				return nil, fmt.Errorf("preparation genealogy readback identity or phase is invalid")
			}
			readings += len(branch.Nodes)
			if readings > businessLineageMaxEdges {
				return nil, fmt.Errorf("%w: preparation genealogy exceeds %d observation readings", errBusinessLineageBudget, businessLineageMaxEdges)
			}
			readbackIndex := readback.Index
			overlay := BusinessLineageOverlay{Branch: branch.Branch, Run: run.Trace.Run, Step: -1, ActionIndex: 0, Actor: observer.Actor,
				Phase: "preparation", ReadbackIndex: &readbackIndex, ReadbackPhase: readback.Phase, EvidenceSHA256: setupHash,
				EvidenceStatus: "PASS", Nodes: []BusinessLineageReading{}}
			if !verified {
				overlay.EvidenceStatus, overlay.Diagnostic = "BLOCKED", true
				overlay.Reason = "captured preparation diagnostics; target readiness is not proven: " + execution.Reason
			}
			identityContext, scopeError := preparationLineageReadbackScope(g, manifest, raw.Model, readback, branch, steps)
			if scopeError != nil {
				overlay.EvidenceStatus, overlay.Diagnostic = "BLOCKED", true
				if overlay.Reason != "" {
					overlay.Reason += "; "
				}
				overlay.Reason += scopeError.Error()
			}
			for _, node := range branch.Nodes {
				reading := BusinessLineageReading{Node: node.ID, Status: "BLOCKED", Reason: overlay.Reason}
				if scopeError == nil {
					reading = preparationLineageReadNode(node, attributes, preparations, *readback.Observation, observer, setupHash)
					enrichBusinessLineageNodeEntities(&reading, observer, identityContext)
					if reading.Status == "BLOCKED" {
						overlay.EvidenceStatus, overlay.Diagnostic = "BLOCKED", true
						if overlay.Reason == "" {
							overlay.Reason = reading.Reason
						}
					}
				}
				overlay.Nodes = append(overlay.Nodes, reading)
			}
			if overlay.Diagnostic {
				for index := range overlay.Nodes {
					overlay.Nodes[index].Status = "BLOCKED"
					overlay.Nodes[index].Reason = overlay.Reason
				}
			}
			result = append(result, overlay)
		}
	}
	return result, nil
}

func preparationLineageReadbackScope(g Graph, manifest RunnerManifest, setup *ModelObservation, readback PreparationReadback, branch BusinessBranchLineage, targets []RunnerStep) (ModelObservation, error) {
	if readback.Observation == nil || readback.Error != "" {
		return ModelObservation{}, fmt.Errorf("preparation readback %d is unavailable: %s", readback.Index, readback.Error)
	}
	beforeAction := len(manifest.Setup) + 1
	for _, step := range setup.Preparation.Steps {
		index := step.Receipt.Action.ActionIndex
		if step.AfterRead < 0 || step.AfterRead > readback.Index || index == 0 {
			continue
		}
		if index < 1 || index > len(setup.Actions) || !sameBusinessJSON(step.Receipt.Action, setup.Actions[index-1]) {
			return ModelObservation{}, fmt.Errorf("preparation readback %d has no matching preceding action capture", readback.Index)
		}
		if beforeAction < index+1 {
			beforeAction = index + 1
		}
	}
	context, err := preparationContextBeforeAction(manifest, setup, beforeAction)
	if err != nil {
		return ModelObservation{}, err
	}
	observed := *readback.Observation
	expected, err := runtimeBusinessBindings(manifest.Preparation.Observer, true, &context)
	if err != nil || !sameBusinessJSON(expected, observed.Bindings) {
		return ModelObservation{}, fmt.Errorf("preparation readback %d bindings differ from the frozen observer's actual context", readback.Index)
	}
	subjects := []string{}
	for _, node := range branch.Nodes {
		for _, ref := range node.References {
			subjects = append(subjects, ref.Subject)
		}
	}
	for _, target := range targets {
		bindings, err := runtimeBusinessBindings(target, true, &context)
		if err != nil {
			return ModelObservation{}, err
		}
		if err := validatePreparationSubjectAgreement(g, target, subjects, bindings, observed.Bindings, false); err != nil {
			return ModelObservation{}, fmt.Errorf("preparation readback %d cannot establish this target's subject scope: %w", readback.Index, err)
		}
		for _, subject := range uniqueSortedBusiness(subjects) {
			observerEntity := businessLineageEntityReference(BusinessLineageValue{Subject: subject, Binding: observed.Bindings[subject]}, manifest.Preparation.Observer, context)
			targetEntity := businessLineageEntityReference(BusinessLineageValue{Subject: subject, Binding: bindings[subject]}, target, context)
			if (observerEntity != nil || targetEntity != nil) && !sameBusinessLineageEntity(observerEntity, targetEntity) {
				return ModelObservation{}, fmt.Errorf("preparation readback %d subject %s does not identify the target's canonical entity", readback.Index, subject)
			}
		}
	}
	return context, nil
}

func preparationLineageReadNode(node BusinessLineageNode, attributes map[string]businessAttributeInfo, preparations map[string]PreparationAction, observed BusinessObservation, observer RunnerStep, setupHash string) BusinessLineageReading {
	reading := BusinessLineageReading{Node: node.ID, Status: "NOT_OBSERVED"}
	missing := []string{}
	for _, ref := range node.References {
		key := ref.Subject + "." + ref.Attribute
		value := BusinessLineageValue{Subject: ref.Subject, SubjectKind: ref.SubjectKind, Attribute: ref.Attribute, SourceRef: ref.SourceRef,
			Binding: observed.Bindings[ref.Subject], Sources: []BusinessLineageSource{}}
		if attributes[key].subject.RequiresBinding && value.Binding == "" {
			missing = append(missing, "binding:"+ref.Subject)
		}
		if actual, exists := observed.Values[key]; exists {
			value.Actual = append(json.RawMessage(nil), actual...)
			sources, err := preparationLineageValueSources(key, observed, observer, setupHash)
			if err != nil {
				reading.Status, reading.Reason = "BLOCKED", err.Error()
			} else {
				value.Sources = sources
			}
		}
		reading.Values = append(reading.Values, value)
	}
	if reading.Status == "BLOCKED" {
		return reading
	}
	if node.Kind == "target" {
		reading.Reason = "preparation history does not freeze or qualify the target business action"
		return reading
	}
	var truth TruthEvaluation
	switch node.Kind {
	case "expected", "assertion", "invariant":
		reading.Reason = "post-action outcome cannot be proved by preparation readback"
		return reading
	case "input_data":
		key := node.References[0].Subject + "." + node.References[0].Attribute
		if len(reading.Values) == 1 && len(reading.Values[0].Actual) > 0 && len(reading.Values[0].Sources) > 0 {
			truth = businessTruth(true)
		} else {
			truth = unknownBusiness([]string{key}, "input data or independent source missing in preparation readback")
		}
	case "state":
		reading.ObservedState = observed.State
		if observed.State == "" || observed.State == "UNKNOWN" {
			truth = unknownBusiness([]string{"state"}, "preparation readback did not observe a business state")
		} else {
			truth = businessTruth(containsBusinessID(node.FromStates, observed.State))
		}
	case "preparation":
		truth = preparationSatisfaction(preparations[node.Preparation], observed, attributes)
		reading.Reason = "effects at this historical readback; the separate preparation receipt records execution"
	case "condition", "logic":
		if node.Predicate == nil {
			reading.Status, reading.Reason = "BLOCKED", "preparation dependency has no declared predicate"
			return reading
		}
		truth = evaluateBusinessPredicate(*node.Predicate, observed, attributes)
	default:
		reading.Status, reading.Reason = "BLOCKED", "unsupported preparation lineage node kind"
		return reading
	}
	if len(missing) > 0 {
		truth = unknownBusiness(uniqueSortedBusiness(missing), "preparation readback has no complete business subject identity")
	}
	reading.Evaluation = &truth
	switch truth.Value {
	case businessTrue:
		reading.Status = "SATISFIED"
	case businessFalse:
		reading.Status = "UNSATISFIED"
	default:
		reading.Status, reading.Reason = "NOT_OBSERVED", truth.Reason
	}
	return reading
}

func preparationLineageValueSources(key string, observed BusinessObservation, observer RunnerStep, setupHash string) ([]BusinessLineageSource, error) {
	if strings.HasPrefix(key, "request.") || len(observed.Sources[key]) == 0 {
		return nil, fmt.Errorf("preparation attribute %s has no declared independent readback source", key)
	}
	result, seen := []BusinessLineageSource{}, map[string]bool{}
	for _, id := range observed.Sources[key] {
		if seen[id] {
			return nil, fmt.Errorf("preparation attribute %s repeats its observer source", key)
		}
		seen[id] = true
		matches := 0
		for _, oracle := range observer.Oracles {
			if oracle.ID == id && oracle.Measurements[key] != "" && strings.TrimSpace(oracle.Boundary) != "" {
				matches++
				result = append(result, BusinessLineageSource{Oracle: id, Kind: "preparation_readback", Boundary: oracle.Boundary, SHA256: setupHash})
			}
		}
		if matches != 1 {
			return nil, fmt.Errorf("preparation attribute %s lacks a unique measurement in frozen observer Oracle %s", key, id)
		}
	}
	return result, nil
}
