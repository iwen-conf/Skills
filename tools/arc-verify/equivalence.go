package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// HistoryAttribute is a model-declared fact about the path taken that changes
// later behavior even when the visible state is the same.
type HistoryAttribute struct {
	ID          string   `json:"id"`
	Flow        string   `json:"flow"`
	SetBy       []string `json:"set_by,omitempty"`
	ClearedBy   []string `json:"cleared_by,omitempty"`
	Enables     []string `json:"enables,omitempty"`
	Measurement string   `json:"measurement,omitempty"`
	Source      string   `json:"source"`
	SourceRef   string   `json:"source_ref"`
}

// EquivalencePolicy lists runtime measurements that must not split states
// (timestamps, request ids); every exclusion needs its own source.
type EquivalencePolicy struct {
	Ignore []IgnoredMeasurement `json:"ignore"`
}

type IgnoredMeasurement struct {
	Measurement string `json:"measurement"`
	Reason      string `json:"reason"`
	Source      string `json:"source"`
	SourceRef   string `json:"source_ref"`
}

func validateEquivalencePolicy(policy *EquivalencePolicy) error {
	if policy == nil {
		return nil
	}
	if len(policy.Ignore) == 0 || len(policy.Ignore) > 256 {
		return errors.New("equivalence.ignore needs 1 to 256 entries")
	}
	seen := map[string]bool{}
	for _, item := range policy.Ignore {
		if strings.TrimSpace(item.Measurement) == "" || strings.TrimSpace(item.Reason) == "" || !explicitSource(item.Source, item.SourceRef) || seen[item.Measurement] {
			return fmt.Errorf("equivalence.ignore %q needs a unique measurement, a reason and a user or document source", item.Measurement)
		}
		seen[item.Measurement] = true
	}
	return nil
}

// ExplorationMerge records a path that reached an already explored equivalent state.
type ExplorationMerge struct {
	Key    string   `json:"key"`
	Kept   []string `json:"kept"`
	Merged []string `json:"merged"`
	Basis  []string `json:"basis"`
}

// EquivalenceInput is one observed or explored business world state.
type EquivalenceInput struct {
	Flow, State, Role, Principal string
	Conditions                   map[string]bool
	History                      map[string]bool
	Measurements                 map[string]json.RawMessage
}

// equivalenceKey returns the canonical key, its basis and, when any part of the
// basis is unobserved, the reason the state must not be merged (design 05,
// "状态等价"). Runtime callers pass every declared oracle measurement.
func equivalenceKey(g Graph, input EquivalenceInput) (string, []string, string) {
	x := indexModel(g)
	flow, found := x.flows[input.Flow]
	if !found {
		return "", nil, "EQUIVALENCE_UNDECIDED: unknown flow " + input.Flow
	}
	basis := []string{"flow=" + input.Flow, "state=" + input.State, "role=" + input.Role, "principal=" + input.Principal}
	for _, condition := range relevantConditions(g, x, flow, input.State) {
		value, observed := input.Conditions[condition]
		if !observed {
			return "", nil, "EQUIVALENCE_UNDECIDED: relevant condition " + condition + " is unobserved"
		}
		basis = append(basis, fmt.Sprintf("%s=%t", condition, value))
	}
	for _, attribute := range flowHistory(g, input.Flow) {
		value, observed := input.History[attribute.ID]
		if !observed {
			return "", nil, "EQUIVALENCE_UNDECIDED: declared history " + attribute.ID + " is unobserved"
		}
		basis = append(basis, fmt.Sprintf("h:%s=%t", attribute.ID, value))
	}
	names := make([]string, 0, len(input.Measurements))
	for name := range input.Measurements {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		var value any
		if err := json.Unmarshal(input.Measurements[name], &value); err != nil {
			return "", nil, "EQUIVALENCE_UNDECIDED: measurement " + name + " is not valid JSON"
		}
		canonical, err := json.Marshal(value)
		if err != nil {
			return "", nil, "EQUIVALENCE_UNDECIDED: measurement " + name + ": " + err.Error()
		}
		basis = append(basis, "m:"+name+"="+string(canonical))
	}
	return strings.Join(basis, "|"), basis, ""
}

// flowAdjacency lists the declared transitions whose action belongs to the
// flow's capability, keyed by source state and ordered by edge identity.
func flowAdjacency(x modelIndex, flow BehaviorFlow, transitions []TransitionContract) map[string][]TransitionContract {
	adjacency := map[string][]TransitionContract{}
	for _, transition := range transitions {
		for _, capability := range x.capsByAction[transition.Action] {
			if capability == flow.Capability {
				adjacency[x.edges[transition.Edge].From] = append(adjacency[x.edges[transition.Edge].From], transition)
				break
			}
		}
	}
	for state, items := range adjacency {
		sort.Slice(items, func(i, j int) bool { return items[i].Edge < items[j].Edge })
		adjacency[state] = items
	}
	return adjacency
}

// relevantConditions are the conditions read by any flow transition reachable
// from state, closed over constraint clauses that mention one of them.
func relevantConditions(g Graph, x modelIndex, flow BehaviorFlow, state string) []string {
	adjacency := flowAdjacency(x, flow, g.Transitions)
	seen, queue, conditions := map[string]bool{state: true}, []string{state}, map[string]bool{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, transition := range adjacency[current] {
			for condition := range transition.Preconditions {
				conditions[condition] = true
			}
			if next := x.edges[transition.Edge].To; !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, clause := range g.Constraints {
			touches := false
			for _, literal := range clause.Literals {
				touches = touches || conditions[literal.Condition]
			}
			for _, literal := range clause.Literals {
				if touches && !conditions[literal.Condition] {
					conditions[literal.Condition], changed = true, true
				}
			}
		}
	}
	result := make([]string, 0, len(conditions))
	for condition := range conditions {
		result = append(result, condition)
	}
	sort.Strings(result)
	return result
}

func flowHistory(g Graph, flow string) []HistoryAttribute {
	result := []HistoryAttribute{}
	for _, attribute := range g.History {
		if attribute.Flow == flow {
			result = append(result, attribute)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func validateHistory(x modelIndex, attribute HistoryAttribute) error {
	flow, found := x.flows[attribute.Flow]
	if !found {
		return fmt.Errorf("history %s references unknown flow %q", attribute.ID, attribute.Flow)
	}
	if !explicitSource(attribute.Source, attribute.SourceRef) {
		return fmt.Errorf("history %s needs an explicit user or document source", attribute.ID)
	}
	if len(attribute.SetBy) == 0 && strings.TrimSpace(attribute.Measurement) == "" {
		return fmt.Errorf("history %s needs set_by transitions or a runtime measurement", attribute.ID)
	}
	if len(attribute.SetBy)+len(attribute.ClearedBy)+len(attribute.Enables) > 256 {
		return fmt.Errorf("history %s lists more than 256 transitions", attribute.ID)
	}
	set := map[string]bool{}
	for _, group := range []struct {
		role  string
		edges []string
	}{{"set_by", attribute.SetBy}, {"cleared_by", attribute.ClearedBy}, {"enables", attribute.Enables}} {
		for _, edge := range group.edges {
			contract, found := x.contracts[edge]
			if !found {
				return fmt.Errorf("history %s %s %s is not a declared transition", attribute.ID, group.role, edge)
			}
			inFlow := false
			for _, capability := range x.capsByAction[contract.Action] {
				inFlow = inFlow || capability == flow.Capability
			}
			if !inFlow {
				return fmt.Errorf("history %s %s %s is outside flow %s", attribute.ID, group.role, edge, flow.ID)
			}
			if group.role == "set_by" {
				set[edge] = true
			}
			if group.role == "cleared_by" && set[edge] {
				return fmt.Errorf("history %s is set and cleared by the same transition %s", attribute.ID, edge)
			}
		}
	}
	return nil
}
