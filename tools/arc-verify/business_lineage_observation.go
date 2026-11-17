package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

func buildBusinessLineageOverlays(g Graph, lineage *BusinessLineage, run SnapshotRun) ([]BusinessLineageOverlay, error) {
	result := []BusinessLineageOverlay{}
	if lineage == nil || g.BusinessContract == nil {
		return result, nil
	}
	branches := map[string]BusinessBranchLineage{}
	for _, branch := range lineage.Branches {
		branches[branch.Branch] = branch
	}
	preparations := map[string]PreparationAction{}
	for _, preparation := range g.BusinessContract.Preparation {
		preparations[preparation.ID] = preparation
	}
	attributes := businessAttributeIndex(g.BusinessContract)
	occurrences := map[string]int{}
	scopeKey := func(step int, item BusinessExpectationObservation) string {
		return fmt.Sprintf("%d/%d:%s/%d", step, len(item.Actor), item.Actor, item.ActionIndex)
	}
	for _, fact := range run.Facts {
		for _, item := range fact.BusinessExpectations {
			occurrences[scopeKey(fact.Seq, item)]++
		}
	}
	readings := 0
	for _, fact := range run.Facts {
		for _, item := range fact.BusinessExpectations {
			frozen := item.Decision.Frozen
			if frozen == nil {
				continue
			}
			branch, found := branches[frozen.Branch]
			if !found {
				return nil, fmt.Errorf("run %s step %d freezes undeclared lineage branch %s", run.Trace.Run, fact.Seq, frozen.Branch)
			}
			readings += len(branch.Nodes)
			if readings > businessLineageMaxEdges {
				return nil, fmt.Errorf("%w: exceeds %d observation readings; select fewer runs", errBusinessLineageBudget, businessLineageMaxEdges)
			}
			overlay := BusinessLineageOverlay{Branch: frozen.Branch, Run: run.Trace.Run, Step: fact.Seq, ActionIndex: item.ActionIndex, Actor: item.Actor, Phase: "before", EvidenceSHA256: fact.EvidenceSHA256, EvidenceStatus: "PASS", Nodes: []BusinessLineageReading{}}
			blocked := businessLineageScopeError(g, lineage, run, fact, item)
			if occurrences[scopeKey(fact.Seq, item)] != 1 {
				blocked = "more than one frozen record claims this step, actor and action occurrence"
			}
			if blocked != "" {
				overlay.EvidenceStatus = "BLOCKED"
				overlay.Reason = blocked
			}
			for _, node := range branch.Nodes {
				reading := BusinessLineageReading{Node: node.ID, Status: "BLOCKED", Reason: blocked}
				if blocked == "" {
					reading = businessLineageReadNode(node, attributes, preparations, fact, item)
					if reading.Status == "BLOCKED" {
						overlay.EvidenceStatus = "BLOCKED"
					}
				}
				overlay.Nodes = append(overlay.Nodes, reading)
			}
			if overlay.EvidenceStatus != "PASS" {
				if overlay.Reason == "" {
					overlay.Reason = "this occurrence has incomplete or invalid observation provenance"
				}
				for index := range overlay.Nodes {
					overlay.Nodes[index].Status = "BLOCKED"
					if overlay.Nodes[index].Reason == "" {
						overlay.Nodes[index].Reason = "this occurrence has incomplete or invalid observation provenance"
					}
				}
			}
			result = append(result, overlay)
		}
	}
	return result, nil
}

func businessLineageScopeError(g Graph, lineage *BusinessLineage, run SnapshotRun, fact UnifiedStepFact, item BusinessExpectationObservation) string {
	if run.EvidenceStatus != "PASS" || fact.EvidenceStatus != "PASS" || fact.EvidenceVersion < runtimeEvidenceVersion {
		return "run and step evidence must both be independently verified before displaying satisfied conditions"
	}
	if fact.Run != run.Trace.Run || fact.Graph != g.ID || fact.Seq < 1 || item.Actor == "" || item.ActionIndex < 1 {
		return "frozen observation does not identify this run, model, step, actor and action occurrence"
	}
	frozen := item.Decision.Frozen
	if item.Decision.Status != "READY" || frozen.ContractSHA256 != lineage.ContractSHA256 {
		return "frozen branch does not belong to this confirmed model version"
	}
	original := FreezeBusinessBranch(g, frozen.Branch, frozen.Before)
	if original.Frozen == nil || !sameBusinessJSON(original.Frozen, frozen) {
		return "frozen expectation differs from the model and its recorded independent prestate"
	}
	return ""
}

func businessLineageReadNode(node BusinessLineageNode, attributes map[string]businessAttributeInfo, preparations map[string]PreparationAction, fact UnifiedStepFact, item BusinessExpectationObservation) BusinessLineageReading {
	reading := BusinessLineageReading{Node: node.ID, Status: "NOT_OBSERVED"}
	before := item.Decision.Frozen.Before
	missingBindings := []string{}
	for _, ref := range node.References {
		key := ref.Subject + "." + ref.Attribute
		value := BusinessLineageValue{Subject: ref.Subject, SubjectKind: ref.SubjectKind, Attribute: ref.Attribute, SourceRef: ref.SourceRef, Binding: before.Bindings[ref.Subject], Sources: []BusinessLineageSource{}}
		if attributes[key].subject.RequiresBinding && value.Binding == "" {
			missingBindings = append(missingBindings, "binding:"+ref.Subject)
		}
		if actual, exists := before.Values[key]; exists {
			value.Actual = append(json.RawMessage(nil), actual...)
			sources, err := businessLineageValueSources(key, before, fact, item)
			if err != nil {
				reading.Status, reading.Reason = "BLOCKED", err.Error()
				return reading
			}
			value.Sources = sources
		}
		reading.Values = append(reading.Values, value)
	}
	var truth TruthEvaluation
	switch node.Kind {
	case "target":
		if before.State == "" || before.State == "UNKNOWN" {
			truth = unknownBusiness([]string{"state"}, "target prestate was not independently observed")
		} else if !containsBusinessID(node.FromStates, before.State) {
			truth = businessTruth(false)
		} else if node.Predicate == nil {
			truth = unknownBusiness(nil, "target has no declared guard")
		} else {
			truth = evaluateBusinessPredicate(*node.Predicate, before, attributes)
		}
	case "state":
		reading.ObservedState = before.State
		if before.State == "" || before.State == "UNKNOWN" {
			truth = unknownBusiness([]string{"state"}, "independent prestate was not observed")
		} else {
			truth = businessTruth(containsBusinessID(node.FromStates, before.State))
		}
	case "preparation":
		truth = preparationSatisfaction(preparations[node.Preparation], before, attributes)
		reading.Reason = "observed effects describe current state; only a preparation receipt proves execution"
	case "condition", "logic":
		if node.Predicate == nil {
			reading.Status, reading.Reason = "BLOCKED", "condition has no declared predicate"
			return reading
		}
		truth = evaluateBusinessPredicate(*node.Predicate, before, attributes)
	default:
		reading.Status, reading.Reason = "BLOCKED", "unsupported lineage node kind"
		return reading
	}
	if len(missingBindings) > 0 {
		truth = unknownBusiness(uniqueSortedBusiness(missingBindings), "business subject identity was not independently observed")
	}
	reading.Evaluation = &truth
	switch truth.Value {
	case businessTrue:
		reading.Status = "SATISFIED"
		if node.Kind == "target" {
			reading.Status = "READY"
		}
	case businessFalse:
		reading.Status = "UNSATISFIED"
	case businessUnknown:
		reading.Status, reading.Reason = "NOT_OBSERVED", truth.Reason
	default:
		reading.Status, reading.Reason = "BLOCKED", "unsupported condition truth value"
	}
	return reading
}

func businessLineageValueSources(key string, before BusinessObservation, fact UnifiedStepFact, item BusinessExpectationObservation) ([]BusinessLineageSource, error) {
	result := []BusinessLineageSource{}
	ids := before.Sources[key]
	if len(ids) == 0 {
		return nil, fmt.Errorf("observed attribute %s lacks a scoped independent source", key)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return nil, fmt.Errorf("observed attribute %s repeats source %s", key, id)
		}
		seen[id] = true
		if id == "request-capture" {
			request := item.Request
			if request == nil || request.Actor != item.Actor || request.ActionIndex != item.ActionIndex || request.Node != item.Decision.Frozen.Action || len(request.RequestSHA256) != 64 || !sameJSONValue(request.Values[key], before.Values[key]) {
				return nil, fmt.Errorf("attribute %s does not match this action's frozen request capture", key)
			}
			result = append(result, BusinessLineageSource{Oracle: id, Kind: "request_capture", SHA256: request.RequestSHA256})
			continue
		}
		matches := []OracleObservation{}
		for _, oracle := range fact.Oracles {
			if oracle.Actor == item.Actor && oracle.ActionIndex == item.ActionIndex && oracle.ID == id {
				matches = append(matches, oracle)
			}
		}
		if len(matches) != 1 || len(matches[0].BeforeSHA256) != 64 || !sameJSONValue(matches[0].Before.Values[key], before.Values[key]) {
			return nil, fmt.Errorf("attribute %s has no unique matching before reading for actor %s action %d Oracle %s", key, item.Actor, item.ActionIndex, id)
		}
		if strings.TrimSpace(matches[0].Boundary) == "" {
			return nil, fmt.Errorf("attribute %s Oracle %s lacks an independent boundary", key, id)
		}
		result = append(result, BusinessLineageSource{Oracle: id, Kind: "oracle", Boundary: matches[0].Boundary, SHA256: matches[0].BeforeSHA256})
	}
	return result, nil
}
