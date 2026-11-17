package main

import (
	"encoding/json"
	"fmt"
)

// Preparation may repair a legacy boolean only when the registered target
// reader measures a declared boolean attribute at that same response pointer.
// ConditionSubjects are annotations, and another actor/reader's matching path
// cannot supply this association. The prediction remains planning evidence;
// execute independently reads the complete preconditions again before writing.
func businessCampaignPreparedPreconditions(g Graph, obligation ModelObligation, step RunnerStep, observed ModelObservation, before, predicted BusinessObservation) error {
	x := indexModel(g)
	judged := stepRule(x, obligation, step.Transition)
	expected := obligation.Definition.Preconditions
	if step.Transition != "" {
		expected = x.contracts[step.Transition].Preconditions
		if predicted.State != x.edges[step.Transition].From {
			return fmt.Errorf("preparation does not establish the registered transition source state")
		}
	}
	var reader *OracleAdapter
	for i := range step.Oracles {
		if step.Oracles[i].ID == judged.Oracle {
			if reader != nil {
				return fmt.Errorf("target preconditions have duplicate registered readers")
			}
			reader = &step.Oracles[i]
		}
	}
	var actual *IndependentFacts
	for i := range observed.Oracles {
		item := &observed.Oracles[i]
		if item.Actor == step.Actor && item.ActionIndex == 1 && item.ID == judged.Oracle {
			if actual != nil {
				return fmt.Errorf("target preconditions have duplicate actor/action observations")
			}
			actual = &item.Before
		}
	}
	if reader == nil || actual == nil {
		return fmt.Errorf("target preconditions lack their registered actor/action reader")
	}
	attributes := businessAttributeIndex(g.BusinessContract)
	conditions := append([]string(nil), judged.Conditions...)
	for condition := range expected {
		conditions = append(conditions, condition)
	}
	for _, condition := range uniqueSortedBusiness(conditions) {
		got, known := actual.Conditions[condition]
		if !known {
			return fmt.Errorf("target precondition %s is not independently observed", condition)
		}
		want, constrained := expected[condition]
		path := reader.Conditions[condition]
		if path == "" {
			return fmt.Errorf("target precondition %s has no registered reader pointer", condition)
		}
		mapped := false
		for _, key := range sortedBusinessMeasurementKeys(reader.Measurements) {
			attribute, declared := attributes[key]
			if reader.Measurements[key] != path || !declared || attribute.attribute.Type != "boolean" {
				continue
			}
			mapped = true
			if attribute.subject.RequiresBinding && (before.Bindings[attribute.subject.ID] == "" || predicted.Bindings[attribute.subject.ID] != before.Bindings[attribute.subject.ID]) {
				return fmt.Errorf("target precondition %s has an unknown or changed typed subject identity", condition)
			}
			var measured, projected *bool
			if err := json.Unmarshal(actual.Values[key], &measured); err != nil || measured == nil || *measured != got || !sameJSONValue(actual.Values[key], before.Values[key]) || !containsBusinessID(before.Sources[key], reader.ID) {
				return fmt.Errorf("target precondition %s has no matching typed measurement from its actual reader", condition)
			}
			if err := json.Unmarshal(predicted.Values[key], &projected); err != nil || projected == nil {
				return fmt.Errorf("target precondition %s has an unknown predicted typed value", condition)
			}
			if constrained && *projected != want {
				return fmt.Errorf("preparation effects do not establish target precondition %s", condition)
			}
		}
		if !mapped && constrained && got != want {
			return fmt.Errorf("target precondition %s is unsatisfied and has no same-reader typed repair mapping", condition)
		}
	}
	return nil
}

func sortedBusinessMeasurementKeys(measurements map[string]string) []string {
	keys := make([]string, 0, len(measurements))
	for key := range measurements {
		keys = append(keys, key)
	}
	return uniqueSortedBusiness(keys)
}
