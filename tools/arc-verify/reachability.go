package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type ExplorationBudget struct {
	MaxStates int `json:"max_states"`
	MaxSteps  int `json:"max_steps"`
	MaxPaths  int `json:"max_paths"`
	TimeoutMS int `json:"timeout_ms"`
}

type FlowExploration struct {
	Flow        string             `json:"flow"`
	Initial     string             `json:"initial"`
	Paths       [][]string         `json:"paths"`
	Visited     int                `json:"visited"`
	Truncated   bool               `json:"truncated"`
	Reason      string             `json:"reason"`
	Scope       string             `json:"scope"`
	Constraints []string           `json:"constraints"`
	Merges      []ExplorationMerge `json:"merges"`
}

// exploreFlow enumerates declared transitions breadth-first. A state reached
// again with the same equivalence key (state plus declared history) is not
// expanded twice; the path is recorded as a merge so it stays traceable.
func exploreFlow(g Graph, id string, budget ExplorationBudget) (FlowExploration, error) {
	result := FlowExploration{Flow: id, Paths: [][]string{}, Scope: "bounded paths over declared state transitions; no runtime business execution", Constraints: []string{}, Merges: []ExplorationMerge{}}
	budget, err := budget.normalized()
	if err != nil {
		return result, err
	}
	x := indexModel(g)
	flow, found := x.flows[id]
	if !found {
		return result, fmt.Errorf("flow %s not found", id)
	}
	result.Initial = flow.Initial
	adjacency := flowAdjacency(x, flow, g.Transitions)
	for _, clause := range g.Constraints {
		result.Constraints = append(result.Constraints, clause.ID)
	}
	sort.Strings(result.Constraints)
	attributes := flowHistory(g, id)
	key := func(state string, history map[string]bool) (string, []string) {
		basis := []string{"state=" + state}
		for _, attribute := range attributes {
			basis = append(basis, fmt.Sprintf("h:%s=%t", attribute.ID, history[attribute.ID]))
		}
		return strings.Join(basis, "|"), basis
	}
	enabled := func(edge string, history map[string]bool) bool {
		for _, attribute := range attributes {
			for _, gated := range attribute.Enables {
				if gated == edge && !history[attribute.ID] {
					return false
				}
			}
		}
		return true
	}
	type frontier struct {
		state   string
		history map[string]bool
		path    []string
	}
	queue := []frontier{{state: flow.Initial, history: map[string]bool{}, path: []string{}}}
	explored := map[string][]string{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(budget.TimeoutMS)*time.Millisecond)
	defer cancel()
	for len(queue) > 0 {
		if result.Visited >= budget.MaxStates || ctx.Err() != nil || len(result.Paths)+len(result.Merges) >= budget.MaxPaths {
			result.Truncated = true
			result.Reason = "flow exploration budget exhausted"
			break
		}
		current := queue[0]
		queue = queue[1:]
		equivalence, basis := key(current.state, current.history)
		if kept, seen := explored[equivalence]; seen {
			result.Merges = append(result.Merges, ExplorationMerge{Key: equivalence, Kept: kept, Merged: current.path, Basis: basis})
			continue
		}
		explored[equivalence] = current.path
		result.Visited++
		if len(current.path) >= budget.MaxSteps {
			if len(adjacency[current.state]) > 0 {
				result.Truncated = true
			}
			result.Paths = append(result.Paths, current.path)
			continue
		}
		progress := false
		for _, transition := range adjacency[current.state] {
			if !enabled(transition.Edge, current.history) {
				continue
			}
			remaining := budget
			remaining.MaxStates = budget.MaxStates - result.Visited
			if remaining.MaxStates < 1 {
				result.Truncated = true
				break
			}
			reach, err := solveConditions(g, transition.Preconditions, nil, remaining)
			if err != nil {
				return result, err
			}
			result.Visited += reach.Visited
			if reach.Verdict == "UNREACHABLE" {
				continue
			}
			if reach.Verdict == "UNKNOWN" {
				result.Truncated = true
				continue
			}
			if len(queue)+len(result.Paths)+len(result.Merges) >= budget.MaxPaths {
				result.Truncated = true
				break
			}
			history := map[string]bool{}
			for attribute, value := range current.history {
				history[attribute] = value
			}
			for _, attribute := range attributes {
				for _, edge := range attribute.SetBy {
					if edge == transition.Edge {
						history[attribute.ID] = true
					}
				}
				for _, edge := range attribute.ClearedBy {
					if edge == transition.Edge {
						history[attribute.ID] = false
					}
				}
			}
			queue = append(queue, frontier{state: x.edges[transition.Edge].To, history: history, path: append(append([]string{}, current.path...), transition.Edge)})
			progress = true
		}
		if !progress {
			result.Paths = append(result.Paths, current.path)
		}
	}
	if result.Reason == "" {
		if result.Truncated {
			result.Reason = "depth, path or condition scope truncated; uncovered transitions remain obligations"
		} else {
			result.Reason = "declared reachable transition paths enumerated within the stated bounds; equivalent states merged"
		}
	}
	return result, nil
}

func (b ExplorationBudget) normalized() (ExplorationBudget, error) {
	if b.MaxStates == 0 {
		b.MaxStates = 10000
	}
	if b.MaxSteps == 0 {
		b.MaxSteps = 64
	}
	if b.MaxPaths == 0 {
		b.MaxPaths = 256
	}
	if b.TimeoutMS == 0 {
		b.TimeoutMS = 1000
	}
	if b.MaxStates < 1 || b.MaxStates > 1000000 || b.MaxSteps < 1 || b.MaxSteps > 128 || b.MaxPaths < 1 || b.MaxPaths > 10000 || b.TimeoutMS < 1 || b.TimeoutMS > 10000 {
		return b, errors.New("exploration budget exceeds states 1..1000000, steps 1..128, paths 1..10000 or timeout 1..10000ms")
	}
	return b, nil
}

type Reachability struct {
	Verdict     string          `json:"verdict"`
	Assignment  map[string]bool `json:"assignment,omitempty"`
	Constraints []string        `json:"constraints"`
	Visited     int             `json:"visited"`
	Truncated   bool            `json:"truncated"`
	Reason      string          `json:"reason"`
	Scope       string          `json:"scope"`
}

// CNF clauses describe allowed combinations; execution must still observe them.
func solveConditions(g Graph, fixed map[string]bool, extra [][]ConditionLiteral, budget ExplorationBudget) (Reachability, error) {
	result := Reachability{Verdict: "UNKNOWN", Constraints: []string{}, Scope: "finite declared condition model"}
	budget, err := budget.normalized()
	if err != nil {
		return result, err
	}
	kinds := modelKinds(g)
	variables := map[string]bool{}
	clauses := make([][]ConditionLiteral, 0, len(g.Constraints)+len(extra))
	for _, clause := range g.Constraints {
		clauses = append(clauses, clause.Literals)
		result.Constraints = append(result.Constraints, clause.ID)
	}
	clauses = append(clauses, extra...)
	values := map[string]bool{}
	for id, value := range fixed {
		if kinds[id] != "condition" {
			return result, errors.New("unknown fixed condition " + id)
		}
		variables[id], values[id] = true, value
	}
	for _, clause := range clauses {
		for _, literal := range clause {
			if kinds[literal.Condition] != "condition" {
				return result, errors.New("unknown constraint condition " + literal.Condition)
			}
			variables[literal.Condition] = true
		}
	}
	if len(variables) > budget.MaxSteps {
		result.Truncated, result.Reason = true, "condition scope exceeds the step bound"
		return result, nil
	}
	ids := make([]string, 0, len(variables))
	for id := range variables {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	sort.Strings(result.Constraints)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(budget.TimeoutMS)*time.Millisecond)
	defer cancel()
	var search func(map[string]bool) map[string]bool
	search = func(candidate map[string]bool) map[string]bool {
		if ctx.Err() != nil || result.Visited >= budget.MaxStates {
			result.Truncated = true
			return nil
		}
		result.Visited++
		for {
			changed := false
			for _, clause := range clauses {
				satisfied, unknown := false, 0
				var last ConditionLiteral
				for _, literal := range clause {
					actual, known := candidate[literal.Condition]
					if known && actual == literal.Value {
						satisfied = true
						break
					}
					if !known {
						unknown++
						last = literal
					}
				}
				if satisfied {
					continue
				}
				if unknown == 0 {
					return nil
				}
				if unknown == 1 {
					candidate[last.Condition] = last.Value
					changed = true
				}
			}
			if !changed {
				break
			}
			if ctx.Err() != nil {
				result.Truncated = true
				return nil
			}
		}
		for _, id := range ids {
			if _, known := candidate[id]; known {
				continue
			}
			for _, value := range []bool{true, false} {
				branch := make(map[string]bool, len(candidate)+1)
				for key, actual := range candidate {
					branch[key] = actual
				}
				branch[id] = value
				if witness := search(branch); witness != nil {
					return witness
				}
				if result.Truncated {
					return nil
				}
			}
			return nil
		}
		return candidate
	}
	result.Assignment = search(values)
	switch {
	case result.Assignment != nil:
		result.Verdict, result.Reason = "REACHABLE", "a declared-condition witness exists; fixture readiness is not proved"
	case result.Truncated:
		result.Reason = "condition search budget exhausted; no obligation may be excluded"
	default:
		result.Verdict, result.Reason = "UNREACHABLE", "all assignments in the declared constraint scope were eliminated"
	}
	return result, nil
}
