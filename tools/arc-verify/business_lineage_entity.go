package main

import (
	"fmt"
	"strings"
)

func businessLineageEntityReference(value BusinessLineageValue, step RunnerStep, before ModelObservation) *EntityBinding {
	if value.Binding == "" {
		return nil
	}
	bindings, err := runtimeBusinessBindings(step, true, &before)
	if err != nil || bindings[value.Subject] != value.Binding {
		return nil
	}
	alias := value.Subject
	if source, declared := step.SubjectBindings[value.Subject]; declared {
		parts := strings.Split(source, ".")
		if len(parts) != 3 || parts[0] != "entity" || parts[2] != "id" {
			return nil
		}
		alias = parts[1]
	}
	var match *EntityBinding
	for _, entity := range before.BeforeEntities {
		if entity.Alias != alias {
			continue
		}
		if match != nil {
			return nil
		}
		copy := entity
		match = &copy
	}
	if match == nil || match.ID != value.Binding || strings.TrimSpace(match.Kind) == "" || strings.TrimSpace(match.OwnerActor) == "" || strings.TrimSpace(match.OwnerPrincipal) == "" {
		return nil
	}
	if match.Metadata != nil {
		metadata := map[string]string{}
		for key, value := range match.Metadata {
			metadata[key] = value
		}
		match.Metadata = metadata
	}
	return match
}

func enrichBusinessLineageNodeEntities(reading *BusinessLineageReading, step RunnerStep, before ModelObservation) {
	for index := range reading.Values {
		value := &reading.Values[index]
		value.EntityRef = businessLineageEntityReference(*value, step, before)
	}
}

func sameBusinessLineageEntity(left, right *EntityBinding) bool {
	return left != nil && right != nil && left.Alias == right.Alias && left.Kind == right.Kind && left.ID == right.ID && left.OwnerActor == right.OwnerActor && left.OwnerPrincipal == right.OwnerPrincipal && left.ChannelAccountID == right.ChannelAccountID
}

// Subject IDs are model aliases. The frozen runner identifies which entity
// namespace they read; only the action's before capture supplies ownership.
func (d db) enrichBusinessLineageEntityReferences(g Graph, run *SnapshotRun) error {
	if run.EvidenceStatus != "PASS" || run.RunnerID == "" || run.RunnerSHA256 == "" || len(run.BusinessLineage) == 0 {
		return nil
	}
	registered, err := d.registeredRunner(run.RunnerID)
	if err != nil {
		return fmt.Errorf("lineage entity mapping has no frozen runner: %w", err)
	}
	if registered.SHA256 != run.RunnerSHA256 || registered.Manifest.Graph != g.ID {
		return fmt.Errorf("lineage entity mapping runner differs from the verified run receipt")
	}
	batches := runnerBatches(registered.Manifest)
	facts := map[int]UnifiedStepFact{}
	for _, fact := range run.Facts {
		if fact.Seq > 0 {
			if _, exists := facts[fact.Seq]; exists {
				return fmt.Errorf("lineage entity mapping has more than one fact for step %d", fact.Seq)
			}
			facts[fact.Seq] = fact
		}
	}
	for index := range run.BusinessLineage {
		overlay := &run.BusinessLineage[index]
		if overlay.Phase != "before" || overlay.EvidenceStatus != "PASS" {
			continue
		}
		fact, exists := facts[overlay.Step]
		if !exists || fact.Run != run.Trace.Run || overlay.Step < 1 || overlay.Step > len(batches) || overlay.ActionIndex < 1 || overlay.ActionIndex > len(batches[overlay.Step-1]) {
			return fmt.Errorf("lineage entity mapping has no unique captured action for step %d occurrence %d", overlay.Step, overlay.ActionIndex)
		}
		step := batches[overlay.Step-1][overlay.ActionIndex-1]
		if step.Actor != overlay.Actor || step.BusinessBranch != "" && step.BusinessBranch != overlay.Branch {
			return fmt.Errorf("lineage entity mapping action differs from the frozen actor or branch")
		}
		matches := 0
		for _, item := range fact.BusinessExpectations {
			if item.Actor == overlay.Actor && item.ActionIndex == overlay.ActionIndex && item.Decision.Frozen != nil && item.Decision.Frozen.Branch == overlay.Branch && item.Decision.Frozen.Action == step.Node {
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("lineage entity mapping does not identify one frozen business occurrence")
		}
		before := ModelObservation{Participants: fact.Participants, BeforeEntities: fact.BeforeEntities, BeforeSessions: fact.BeforeSessions}
		for index := range overlay.Nodes {
			enrichBusinessLineageNodeEntities(&overlay.Nodes[index], step, before)
		}
	}
	return nil
}
