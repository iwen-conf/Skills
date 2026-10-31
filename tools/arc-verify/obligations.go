package main

import (
	"fmt"
	"sort"
)

type ModelObligation struct {
	Definition      TestDefinition `json:"definition"`
	Task            taskSpec       `json:"-"`
	Targets         []string       `json:"targets"`
	Transition      string         `json:"transition,omitempty"`
	Reachability    *Reachability  `json:"reachability,omitempty"`
	Excluded        bool           `json:"excluded,omitempty"`
	ExclusionReason string         `json:"exclusion_reason,omitempty"`
}

func modelRule(g Graph, id string) (Rule, bool) {
	for _, rule := range g.Rules {
		if rule.ID == id {
			return rule, true
		}
	}
	return Rule{}, false
}

func actionCapabilities(g Graph, action string) []string {
	return indexModel(g).capsByAction[action]
}

func ruleAllows(g Graph, rule Rule, values map[string]bool) bool {
	for _, guard := range g.Guards {
		if guard.Rule != rule.ID {
			continue
		}
		for _, group := range guard.AnyOf {
			matches := true
			for _, literal := range group {
				actual, found := values[literal.Condition]
				if !found || actual != literal.Value {
					matches = false
					break
				}
			}
			if matches {
				return true
			}
		}
		return false
	}
	for _, condition := range rule.Conditions {
		if !values[condition] {
			return false
		}
	}
	return true
}

func modelRuleObligations(g Graph, id string) []ModelObligation {
	return indexedRuleObligations(g, indexModel(g), id)
}

func indexedRuleObligations(g Graph, x modelIndex, id string) []ModelObligation {
	rule, ok := x.rules[id]
	if !ok {
		return nil
	}
	result := []ModelObligation{}
	add := func(definition TestDefinition, transition string, reachability *Reachability) {
		targets := []string{id, rule.Action}
		if definition.Expected != "UNCHANGED" && definition.Expected != "" {
			targets = append(targets, definition.Expected)
		}
		targets = append(targets, x.capsByAction[rule.Action]...)
		if definition.Capability != "" {
			targets = append(targets, definition.Capability)
		}
		for condition := range definition.Preconditions {
			targets = append(targets, condition)
		}
		if definition.Kind != testExistence {
			for _, invariant := range x.invariants[id] {
				targets = append(targets, invariant.ID)
			}
		}
		targets = append(targets, x.verifies[definition.ID]...)
		if transition != "" {
			targets = append(targets, transition)
			targets = append(targets, x.edges[transition].From, x.edges[transition].To)
		}
		if definition.Kind == testSequence {
			targets = append(targets, definition.Flow, x.flows[definition.Flow].Capability)
			for _, step := range definition.Steps {
				contract := x.contracts[step]
				targets = append(targets, step, x.edges[step].From, x.edges[step].To, contract.Rule, contract.Action)
			}
		}
		definition.DependsOn = uniqueIDs(append(append([]string{}, definition.DependsOn...), x.dependencies[definition.ID]...))
		result = append(result, ModelObligation{
			Definition: definition,
			Task:       taskSpec{ID: g.ID + "/" + definition.ID, Rule: id, Kind: "MODEL", Condition: definition.ID, Expected: definition.Expected},
			Targets:    uniqueIDs(targets), Transition: transition, Reachability: reachability,
		})
	}
	makeDefinition := func(logicalID, name string, kind TestKind, values map[string]bool) TestDefinition {
		definition := TestDefinition{ID: logicalID, Name: name, Rule: id, Kind: kind, Preconditions: values, Expected: rule.Expected, Source: rule.Source, SourceRef: "rule:" + id}
		if !ruleAllows(g, rule, values) {
			definition.Expected, definition.Denied = rule.Rejected, true
		}
		return definition
	}
	for _, legacy := range ruleTasks(g.ID, rule) {
		values := map[string]bool{}
		for _, condition := range rule.Conditions {
			values[condition] = condition != legacy.Condition
		}
		add(makeDefinition(legacy.ID[len(g.ID)+1:], rule.Description+" · "+legacy.Kind, TestKind(legacy.Kind), values), "", nil)
	}
	if guard, exists := x.guards[id]; exists {
		witness := func(fixed map[string]bool, clauses [][]ConditionLiteral) (map[string]bool, *Reachability) {
			reach, err := solveConditions(g, fixed, clauses, ExplorationBudget{})
			if err != nil {
				reach = Reachability{Verdict: "UNKNOWN", Reason: err.Error(), Truncated: true, Scope: "finite declared condition model", Constraints: []string{}}
			}
			values := map[string]bool{}
			for _, condition := range rule.Conditions {
				values[condition] = false
			}
			for condition, value := range fixed {
				values[condition] = value
			}
			for condition, value := range reach.Assignment {
				values[condition] = value
			}
			return values, &reach
		}
		for pos, group := range guard.AnyOf {
			fixed := map[string]bool{}
			for _, literal := range group {
				fixed[literal.Condition] = literal.Value
			}
			values, reach := witness(fixed, nil)
			add(makeDefinition(fmt.Sprintf("%s/GUARD_ALLOW/%d", id, pos+1), rule.Description+" · declared guard alternative", testPositive, values), "", reach)
		}
		clauses := [][]ConditionLiteral{}
		for _, group := range guard.AnyOf {
			clause := []ConditionLiteral{}
			for _, literal := range group {
				clause = append(clause, ConditionLiteral{Condition: literal.Condition, Value: !literal.Value})
			}
			clauses = append(clauses, clause)
		}
		values, reach := witness(nil, clauses)
		definition := makeDefinition(id+"/GUARD_REJECTION", rule.Description+" · declared guard rejection", testNegative, values)
		definition.Expected, definition.Denied = rule.Rejected, true
		add(definition, "", reach)
	}
	for _, transition := range x.transitions[id] {
		edge := x.edges[transition.Edge]
		definition := TestDefinition{ID: "transition/" + edge.ID, Name: rule.Description + " · " + string(transition.Outcome), Rule: id, Kind: testTransition, Preconditions: transition.Preconditions, Expected: edge.To, Denied: transition.Outcome == branchRejection, Source: rule.Source, SourceRef: "transition:" + edge.ID}
		if definition.Denied && rule.Rejected == "UNCHANGED" {
			definition.Expected = "UNCHANGED"
		}
		add(definition, edge.ID, nil)
	}
	for _, invariant := range x.invariants[id] {
		values := map[string]bool{}
		for _, condition := range rule.Conditions {
			values[condition] = true
		}
		add(makeDefinition("invariant/"+invariant.ID+"/"+id, invariant.Name+" · independent measurement", testInvariant, values), "", nil)
	}
	for _, test := range x.tests[id] {
		if test.PendingReason == "" {
			transition := ""
			if test.Kind == testSequence {
				transition = test.Steps[len(test.Steps)-1]
			}
			add(test, transition, nil)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Task.ID < result[j].Task.ID })
	return result
}

// obligationRules lists every frozen rule an obligation is judged by; a
// SEQUENCE needs each step's rule, not only its final one.
func obligationRules(x modelIndex, obligation ModelObligation) []string {
	rules := []string{obligation.Task.Rule}
	if obligation.Definition.Kind == testSequence {
		for _, step := range obligation.Definition.Steps {
			rules = append(rules, x.contracts[step].Rule)
		}
	}
	return uniqueIDs(rules)
}

func rulesApproved(approvals map[string]bool, rules []string) bool {
	for _, rule := range rules {
		if !approvals[rule] {
			return false
		}
	}
	return len(rules) > 0
}

// stepRule returns the frozen rule one runner step is judged by. Only a
// SEQUENCE spans several rules; every other obligation keeps its own rule.
func stepRule(x modelIndex, obligation ModelObligation, transition string) Rule {
	if obligation.Definition.Kind == testSequence {
		if contract, found := x.contracts[transition]; found {
			return x.rules[contract.Rule]
		}
	}
	return x.rules[obligation.Task.Rule]
}

func modelObligations(g Graph) []ModelObligation {
	x := indexModel(g)
	result := []ModelObligation{}
	for _, rule := range g.Rules {
		result = append(result, indexedRuleObligations(g, x, rule.ID)...)
	}
	for _, test := range g.Tests {
		if test.PendingReason != "" {
			result = append(result, ModelObligation{Definition: test, Targets: []string{test.Capability}, Task: taskSpec{ID: g.ID + "/" + test.ID, Rule: test.Rule, Kind: "MODEL", Condition: test.ID, Expected: test.Expected}})
		}
	}
	children := map[string]bool{}
	for _, parent := range x.parents {
		children[parent] = true
	}
	applicability := map[string]ScenarioApplicability{}
	for _, scenario := range g.Applicability {
		applicability[scenario.Capability+"/"+string(scenario.Kind)] = scenario
	}
	hasScenario := map[string]bool{}
	for _, test := range g.Tests {
		caps := []string{test.Capability}
		if rule, found := x.rules[test.Rule]; found {
			caps = append(caps, x.capsByAction[rule.Action]...)
		}
		for _, cap := range caps {
			hasScenario[cap+"/"+string(test.Kind)] = true
		}
	}
	for _, node := range g.Nodes {
		if node.Kind != "capability" || children[node.ID] {
			continue
		}
		for _, kind := range []TestKind{testBoundary, testConcurrency, testIdempotency, testRecovery, testFault} {
			key := node.ID + "/" + string(kind)
			scenario, declared := applicability[key]
			if hasScenario[key] && (!declared || scenario.Applicable != nil && *scenario.Applicable) {
				continue
			}
			reason := "scenario applicability has not been declared"
			source, reference := "document", "model:"+node.ID
			excluded := false
			if declared {
				reason, source, reference = scenario.Reason, scenario.Source, scenario.SourceRef
				excluded = scenario.Applicable != nil && !*scenario.Applicable
				if !excluded {
					reason = "applicable scenario has no bound test: " + reason
				}
			}
			definition := TestDefinition{ID: "review/" + key, Name: node.Name + " · " + string(kind), Capability: node.ID, Kind: kind, Source: source, SourceRef: reference, PendingReason: reason}
			result = append(result, ModelObligation{Definition: definition, Task: taskSpec{ID: g.ID + "/" + definition.ID, Kind: "MODEL", Condition: definition.ID}, Targets: []string{node.ID}, Excluded: excluded, ExclusionReason: scenario.Reason})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Task.ID < result[j].Task.ID })
	return result
}
