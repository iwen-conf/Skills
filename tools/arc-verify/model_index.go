package main

import "sort"

type modelIndex struct {
	kinds        map[string]string
	parents      map[string]string
	edges        map[string]Edge
	rules        map[string]Rule
	capsByAction map[string][]string
	guards       map[string]RuleGuard
	transitions  map[string][]TransitionContract
	contracts    map[string]TransitionContract
	flows        map[string]BehaviorFlow
	tests        map[string][]TestDefinition
	verifies     map[string][]string
	dependencies map[string][]string
	invariants   map[string][]BusinessInvariant
}

func indexModel(g Graph) modelIndex {
	x := modelIndex{kinds: modelKinds(g), parents: map[string]string{}, edges: map[string]Edge{}, rules: map[string]Rule{}, capsByAction: map[string][]string{}, guards: map[string]RuleGuard{}, transitions: map[string][]TransitionContract{}, contracts: map[string]TransitionContract{}, flows: map[string]BehaviorFlow{}, tests: map[string][]TestDefinition{}, verifies: map[string][]string{}, dependencies: map[string][]string{}, invariants: map[string][]BusinessInvariant{}}
	for _, edge := range g.Edges {
		x.edges[edge.ID] = edge
		if edge.Kind == "contains" {
			x.parents[edge.To] = edge.From
		}
		if edge.Kind == "implements" {
			x.capsByAction[edge.To] = append(x.capsByAction[edge.To], edge.From)
		}
		if edge.Kind == "verifies" {
			x.verifies[edge.From] = append(x.verifies[edge.From], edge.To)
		}
		if edge.Kind == "test_depends_on" {
			x.dependencies[edge.From] = append(x.dependencies[edge.From], edge.To)
		}
	}
	for action, direct := range x.capsByAction {
		set := map[string]bool{}
		for _, id := range direct {
			for current := id; current != "" && !set[current]; current = x.parents[current] {
				set[current] = true
			}
		}
		caps := []string{}
		for id := range set {
			if x.kinds[id] == "capability" {
				caps = append(caps, id)
			}
		}
		sort.Strings(caps)
		x.capsByAction[action] = caps
	}
	byScope := map[string][]BusinessInvariant{}
	for _, invariant := range g.Invariants {
		for _, scope := range invariant.Scope {
			byScope[scope] = append(byScope[scope], invariant)
		}
	}
	for _, rule := range g.Rules {
		x.rules[rule.ID] = rule
		seen := map[string]bool{}
		for _, scope := range append([]string{rule.ID, rule.Action}, x.capsByAction[rule.Action]...) {
			for _, invariant := range byScope[scope] {
				if !seen[invariant.ID] {
					x.invariants[rule.ID] = append(x.invariants[rule.ID], invariant)
					seen[invariant.ID] = true
				}
			}
		}
		sort.Slice(x.invariants[rule.ID], func(i, j int) bool { return x.invariants[rule.ID][i].ID < x.invariants[rule.ID][j].ID })
	}
	for _, guard := range g.Guards {
		x.guards[guard.Rule] = guard
	}
	for _, transition := range g.Transitions {
		x.transitions[transition.Rule] = append(x.transitions[transition.Rule], transition)
		x.contracts[transition.Edge] = transition
	}
	for _, flow := range g.Flows {
		x.flows[flow.ID] = flow
	}
	for _, test := range g.Tests {
		x.tests[test.Rule] = append(x.tests[test.Rule], test)
		x.dependencies[test.ID] = append(x.dependencies[test.ID], test.DependsOn...)
	}
	return x
}

func uniqueIDs(ids []string) []string {
	sort.Strings(ids)
	result := []string{}
	for _, id := range ids {
		if len(result) == 0 || result[len(result)-1] != id {
			result = append(result, id)
		}
	}
	return result
}
