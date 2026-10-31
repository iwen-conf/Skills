package main

import (
	"encoding/json"
	"sort"
)

func semanticSubjects(g Graph) (map[string]string, error) {
	fragments := map[string][]string{}
	appendValue := func(id string, value any) error {
		body, err := json.Marshal(value)
		if err != nil {
			return err
		}
		fragments[id] = append(fragments[id], string(body))
		return nil
	}
	for _, node := range g.Nodes {
		if err := appendValue(node.ID, node); err != nil {
			return nil, err
		}
	}
	for _, rule := range g.Rules {
		if err := appendValue(rule.ID, rule); err != nil {
			return nil, err
		}
	}
	for _, invariant := range g.Invariants {
		if err := appendValue(invariant.ID, invariant); err != nil {
			return nil, err
		}
	}
	for _, test := range g.Tests {
		if err := appendValue(test.ID, test); err != nil {
			return nil, err
		}
	}
	for _, flow := range g.Flows {
		if err := appendValue(flow.ID, flow); err != nil {
			return nil, err
		}
		if err := appendValue(flow.Capability, flow); err != nil {
			return nil, err
		}
	}
	for _, edge := range g.Edges {
		if err := appendValue(edge.ID, edge); err != nil {
			return nil, err
		}
		if err := appendValue(edge.From, edge); err != nil {
			return nil, err
		}
		if err := appendValue(edge.To, edge); err != nil {
			return nil, err
		}
	}
	for _, metadata := range g.Contracts {
		if err := appendValue(metadata.Subject, metadata); err != nil {
			return nil, err
		}
	}
	for _, guard := range g.Guards {
		if err := appendValue(guard.Rule, guard); err != nil {
			return nil, err
		}
	}
	for _, constraint := range g.Constraints {
		for _, literal := range constraint.Literals {
			if err := appendValue(literal.Condition, constraint); err != nil {
				return nil, err
			}
		}
	}
	for _, transition := range g.Transitions {
		if err := appendValue(transition.Edge, transition); err != nil {
			return nil, err
		}
		if err := appendValue(transition.Action, transition); err != nil {
			return nil, err
		}
	}
	for _, shared := range g.SharedOwners {
		if err := appendValue(shared.Subject, shared); err != nil {
			return nil, err
		}
	}
	for _, scenario := range g.Applicability {
		if err := appendValue(scenario.Capability, scenario); err != nil {
			return nil, err
		}
	}
	result := map[string]string{}
	for id, parts := range fragments {
		sort.Strings(parts)
		body, err := json.Marshal(parts)
		if err != nil {
			return nil, err
		}
		result[id] = digestBytes(body)
	}
	return result, nil
}
