package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// Metadata is keyed by stable identity so legacy Node/Edge values stay unchanged.
type NodeContract struct {
	Subject      string   `json:"subject"`
	Description  string   `json:"description"`
	Inputs       []string `json:"inputs,omitempty"`
	Outputs      []string `json:"outputs,omitempty"`
	Requirements []string `json:"requirements,omitempty"`
	Source       string   `json:"source"`
	SourceRef    string   `json:"source_ref"`
	Unconfirmed  bool     `json:"unconfirmed,omitempty"`
}

type ConditionLiteral struct {
	Condition string `json:"condition"`
	Value     bool   `json:"value"`
}

func (literal *ConditionLiteral) UnmarshalJSON(raw []byte) error {
	var value struct {
		Condition string `json:"condition"`
		Value     *bool  `json:"value"`
	}
	if err := decodeStrictJSON(raw, &value); err != nil {
		return err
	}
	if value.Value == nil {
		return errors.New("condition literal requires an explicit boolean value")
	}
	literal.Condition, literal.Value = value.Condition, *value.Value
	return nil
}

type ConditionAssignment map[string]bool

func (assignment *ConditionAssignment) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := decodeUniqueJSON(raw, &fields); err != nil {
		return err
	}
	if fields == nil {
		return errors.New("preconditions must be an object of explicit booleans")
	}
	values := ConditionAssignment{}
	for id, body := range fields {
		var value *bool
		if err := json.Unmarshal(body, &value); err != nil {
			return fmt.Errorf("condition %s: %w", id, err)
		}
		if value == nil {
			return fmt.Errorf("condition %s is unknown, not false", id)
		}
		values[id] = *value
	}
	*assignment = values
	return nil
}

type ConditionClause struct {
	ID        string             `json:"id"`
	Literals  []ConditionLiteral `json:"literals"`
	Source    string             `json:"source"`
	SourceRef string             `json:"source_ref"`
}

type RuleGuard struct {
	Rule  string               `json:"rule"`
	AnyOf [][]ConditionLiteral `json:"any_of"`
}

type BranchOutcome string

const (
	branchSuccess      BranchOutcome = "success"
	branchRejection    BranchOutcome = "rejection"
	branchRetry        BranchOutcome = "retry"
	branchCompensation BranchOutcome = "compensation"
	branchError        BranchOutcome = "error"
	branchParallel     BranchOutcome = "parallel"
	branchJoin         BranchOutcome = "join"
)

type TransitionContract struct {
	Edge          string              `json:"edge"`
	Action        string              `json:"action"`
	Rule          string              `json:"rule"`
	Preconditions ConditionAssignment `json:"preconditions"`
	Outcome       BranchOutcome       `json:"outcome"`
	ParallelGroup string              `json:"parallel_group,omitempty"`
}

type BehaviorFlow struct {
	ID         string `json:"id"`
	Capability string `json:"capability"`
	Initial    string `json:"initial"`
}

type ComparisonOperator string

const (
	compareEqual        ComparisonOperator = "eq"
	compareNotEqual     ComparisonOperator = "ne"
	compareLess         ComparisonOperator = "lt"
	compareLessEqual    ComparisonOperator = "lte"
	compareGreater      ComparisonOperator = "gt"
	compareGreaterEqual ComparisonOperator = "gte"
)

type PropertyCheck struct {
	Left     string             `json:"left"`
	Operator ComparisonOperator `json:"operator"`
	Right    string             `json:"right,omitempty"`
	Value    json.RawMessage    `json:"value,omitempty"`
}

type BusinessInvariant struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Scope               []string        `json:"scope"`
	Oracle              string          `json:"oracle"`
	ObservationBoundary string          `json:"observation_boundary"`
	Checks              []PropertyCheck `json:"checks"`
	Source              string          `json:"source"`
	SourceRef           string          `json:"source_ref"`
}

type TestKind string

const (
	testPositive    TestKind = "POSITIVE"
	testNegative    TestKind = "NEGATIVE"
	testForced      TestKind = "FORCED"
	testTransition  TestKind = "TRANSITION"
	testBoundary    TestKind = "BOUNDARY"
	testConcurrency TestKind = "CONCURRENCY"
	testIdempotency TestKind = "IDEMPOTENCY"
	testRecovery    TestKind = "RECOVERY"
	testInvariant   TestKind = "INVARIANT"
	testFault       TestKind = "FAULT"
	testExistence   TestKind = "EXISTENCE"
	testSequence    TestKind = "SEQUENCE"
	testConsistency TestKind = "CONSISTENCY"
)

// ObservationLayer names one independent oracle that reads the same action's
// result at a declared layer (ui, api, database, async).
type ObservationLayer struct {
	Layer  string `json:"layer"`
	Oracle string `json:"oracle"`
}

type ActorRequirement struct {
	Alias string `json:"alias"`
	Role  string `json:"role"`
}

type TestDefinition struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Rule          string              `json:"rule"`
	Capability    string              `json:"capability,omitempty"`
	Kind          TestKind            `json:"kind"`
	Flow          string              `json:"flow,omitempty"`
	Steps         []string            `json:"steps,omitempty"`
	Layers        []ObservationLayer  `json:"layers,omitempty"`
	Preconditions ConditionAssignment `json:"preconditions,omitempty"`
	Expected      string              `json:"expected,omitempty"`
	Denied        bool                `json:"denied,omitempty"`
	DependsOn     []string            `json:"depends_on,omitempty"`
	Actors        []ActorRequirement  `json:"actors,omitempty"`
	ReadOnly      bool                `json:"read_only,omitempty"`
	Source        string              `json:"source"`
	SourceRef     string              `json:"source_ref"`
	PendingReason string              `json:"pending_reason,omitempty"`
}

type SharedOwnership struct {
	Subject   string   `json:"subject"`
	Owners    []string `json:"owners"`
	Reason    string   `json:"reason"`
	SourceRef string   `json:"source_ref"`
}

type ScenarioApplicability struct {
	Capability string   `json:"capability"`
	Kind       TestKind `json:"kind"`
	Applicable *bool    `json:"applicable"`
	Reason     string   `json:"reason"`
	Source     string   `json:"source"`
	SourceRef  string   `json:"source_ref"`
}

func explicitSource(source, reference string) bool {
	return (source == "user" || source == "document") && strings.TrimSpace(reference) != ""
}

func modelKinds(g Graph) map[string]string {
	result := map[string]string{}
	for _, node := range g.Nodes {
		result[node.ID] = node.Kind
	}
	for _, rule := range g.Rules {
		result[rule.ID] = "rule"
	}
	for _, invariant := range g.Invariants {
		result[invariant.ID] = "invariant"
	}
	for _, test := range g.Tests {
		result[test.ID] = "test"
	}
	for _, risk := range g.Risks {
		result[risk.ID] = "risk"
	}
	for _, flow := range g.Flows {
		result[flow.ID] = "flow"
	}
	return result
}

func validateBusinessModel(g Graph) error {
	if strings.TrimSpace(g.ID) == "" || strings.TrimSpace(g.Scope) == "" || g.System == "" || g.Previous == g.ID {
		return errors.New("v2 requires graph id, scope, stable system and a distinct previous version")
	}
	if len(g.Nodes) > 10000 || len(g.Edges) > 30000 || len(g.Rules) > 2000 || len(g.Tests) > 20000 || len(g.Constraints) > 2000 || len(g.Invariants) > 2000 || len(g.History) > 2000 {
		return errors.New("model exceeds the bounded entity limits")
	}
	ids := map[string]string{}
	add := func(id, kind string) error {
		if id == "" || len(id) > 256 || strings.IndexFunc(id, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 || ids[id] != "" {
			return fmt.Errorf("empty, oversized or duplicated identity %q", id)
		}
		ids[id] = kind
		return nil
	}
	systems := 0
	legacy := Graph{ID: g.ID, Scope: g.Scope, Rules: g.Rules, Risks: g.Risks}
	for _, node := range g.Nodes {
		if strings.TrimSpace(node.Name) == "" {
			return fmt.Errorf("node %s needs a name", node.ID)
		}
		switch node.Kind {
		case "system":
			systems++
		case "domain", "capability", "boundary", "role", "data", "contract", "deployment":
		case "action", "condition", "state":
			legacy.Nodes = append(legacy.Nodes, node)
		default:
			return fmt.Errorf("unsupported v2 node kind %q", node.Kind)
		}
		if err := add(node.ID, node.Kind); err != nil {
			return err
		}
	}
	if systems != 1 || ids[g.System] != "system" {
		return errors.New("v2 requires exactly one stable system root")
	}
	rules := map[string]Rule{}
	for _, rule := range g.Rules {
		if err := add(rule.ID, "rule"); err != nil {
			return err
		}
		if ids[rule.Actor] != "role" {
			return fmt.Errorf("rule %s actor is not a declared role", rule.ID)
		}
		rules[rule.ID] = rule
	}
	for _, invariant := range g.Invariants {
		if err := add(invariant.ID, "invariant"); err != nil {
			return err
		}
	}
	for _, test := range g.Tests {
		if err := add(test.ID, "test"); err != nil {
			return err
		}
	}
	for _, risk := range g.Risks {
		if err := add(risk.ID, "risk"); err != nil {
			return err
		}
	}
	for _, flow := range g.Flows {
		if err := add(flow.ID, "flow"); err != nil {
			return err
		}
	}
	edges := map[string]Edge{}
	for _, edge := range g.Edges {
		if err := add(edge.ID, "edge"); err != nil {
			return err
		}
		edges[edge.ID] = edge
	}
	parents := map[string]string{}
	implemented := map[string]bool{}
	owners := map[string]map[string]bool{}
	relations := map[string]bool{}
	for _, edge := range g.Edges {
		from, to := ids[edge.From], ids[edge.To]
		if from == "" || to == "" {
			return fmt.Errorf("edge %s has an unknown endpoint", edge.ID)
		}
		valid := false
		switch edge.Kind {
		case "contains":
			valid = from == "system" && to == "domain" || from == "domain" && (to == "domain" || to == "capability") || from == "capability" && to == "capability"
			if parents[edge.To] != "" {
				return fmt.Errorf("contains gives %s more than one parent", edge.To)
			}
			parents[edge.To] = edge.From
		case "implements":
			valid = from == "capability" && to == "action"
			implemented[edge.To] = true
		case "depends_on":
			valid = from == to && (from == "capability" || from == "boundary")
		case "owns":
			valid = from == "boundary" && (to == "capability" || to == "rule" || to == "invariant" || to == "data" || to == "contract")
			if owners[edge.To] == nil {
				owners[edge.To] = map[string]bool{}
			}
			owners[edge.To][edge.From] = true
		case "uses_contract":
			valid = (from == "boundary" || from == "capability") && to == "contract"
		case "serves":
			valid = from == "role" && (to == "domain" || to == "capability")
		case "collaborates_with":
			valid = from == to && (from == "domain" || from == "boundary")
		case "deployed_as":
			valid = from == "boundary" && to == "deployment"
		case "requires", "forbids":
			valid = from == "condition" && to == "action"
			legacy.Edges = append(legacy.Edges, edge)
		case "transitions":
			valid = from == "action" && to == "state"
			legacy.Edges = append(legacy.Edges, edge)
		case "recovers":
			valid = from == "action" && to == "condition"
			legacy.Edges = append(legacy.Edges, edge)
		case "next":
			valid = from == "state" && to == "action"
			legacy.Edges = append(legacy.Edges, edge)
		case "transitions_to":
			valid = from == "state" && to == "state"
		case "verifies":
			valid = from == "test" && (to == "capability" || to == "rule" || to == "action" || to == "condition" || to == "state" || to == "invariant" || to == "edge" && edges[edge.To].Kind == "transitions_to")
		case "test_depends_on":
			valid = from == "test" && to == "test"
		default:
			return fmt.Errorf("unsupported relation %q", edge.Kind)
		}
		if !valid {
			return fmt.Errorf("relation %s (%s) has illegal %s -> %s endpoints", edge.ID, edge.Kind, from, to)
		}
		key := edge.Kind + "\x00" + edge.From + "\x00" + edge.To
		if relations[key] && edge.Kind != "transitions_to" {
			return fmt.Errorf("duplicated %s relation %s -> %s", edge.Kind, edge.From, edge.To)
		}
		relations[key] = true
	}
	treeState := map[string]int{g.System: 2}
	treeDepth := map[string]int{g.System: 0}
	var reachesSystem func(string, int) bool
	reachesSystem = func(id string, depth int) bool {
		if treeState[id] == 2 {
			return true
		}
		if id == "" || treeState[id] == 1 || depth > 128 {
			return false
		}
		treeState[id] = 1
		if !reachesSystem(parents[id], depth+1) {
			return false
		}
		treeDepth[id] = treeDepth[parents[id]] + 1
		if treeDepth[id] > 128 {
			return false
		}
		treeState[id] = 2
		return true
	}
	for _, node := range g.Nodes {
		if node.Kind == "domain" || node.Kind == "capability" {
			if !reachesSystem(node.ID, 0) {
				return fmt.Errorf("contains for %s is cyclic, detached or exceeds depth 128", node.ID)
			}
		}
		if node.Kind == "action" && !implemented[node.ID] {
			return fmt.Errorf("action %s needs an explicit implements relation", node.ID)
		}
	}
	if err := validateLegacyGraph(legacy); err != nil {
		return err
	}
	for _, contract := range g.Contracts {
		if ids[contract.Subject] == "" || strings.TrimSpace(contract.Description) == "" || !explicitSource(contract.Source, contract.SourceRef) {
			return fmt.Errorf("invalid contract metadata for %s", contract.Subject)
		}
	}
	seen := map[string]bool{}
	for _, contract := range g.Contracts {
		if seen[contract.Subject] {
			return fmt.Errorf("duplicate metadata for %s", contract.Subject)
		}
		seen[contract.Subject] = true
	}
	for _, flow := range g.Flows {
		if ids[flow.Capability] != "capability" || ids[flow.Initial] != "state" {
			return fmt.Errorf("flow %s has invalid capability or initial state", flow.ID)
		}
	}
	seen = map[string]bool{}
	for _, transition := range g.Transitions {
		edge, ok := edges[transition.Edge]
		rule, exists := rules[transition.Rule]
		if !ok || edge.Kind != "transitions_to" || !exists || rule.Action != transition.Action || seen[transition.Edge] {
			return fmt.Errorf("invalid transition contract %s", transition.Edge)
		}
		seen[transition.Edge] = true
		if err := validateConditionAssignment(transition.Preconditions, rule, ids); err != nil {
			return fmt.Errorf("transition %s: %w", transition.Edge, err)
		}
		switch transition.Outcome {
		case branchSuccess, branchRejection, branchRetry, branchCompensation, branchError, branchParallel, branchJoin:
		default:
			return fmt.Errorf("invalid transition outcome %q", transition.Outcome)
		}
		if transition.Outcome == branchRejection && (rule.Rejected == "UNCHANGED" && edge.From != edge.To || rule.Rejected != "UNCHANGED" && edge.To != rule.Rejected) {
			return fmt.Errorf("rejection transition %s contradicts rejected state", transition.Edge)
		}
		if transition.Outcome == branchSuccess && (!ruleAllows(g, rule, transition.Preconditions) || edge.To != rule.Expected) {
			return fmt.Errorf("success transition %s contradicts its rule guard or expected state", transition.Edge)
		}
		if transition.Outcome == branchRejection && ruleAllows(g, rule, transition.Preconditions) {
			return fmt.Errorf("rejection transition %s has a satisfied rule guard", transition.Edge)
		}
		if (transition.Outcome == branchParallel || transition.Outcome == branchJoin) && transition.ParallelGroup == "" {
			return fmt.Errorf("parallel/join transition %s requires its group", transition.Edge)
		}
	}
	for _, edge := range g.Edges {
		if edge.Kind == "transitions_to" && !seen[edge.ID] {
			return fmt.Errorf("transition %s lacks its action/guard contract", edge.ID)
		}
	}
	for _, invariant := range g.Invariants {
		if invariant.Name == "" || len(invariant.Scope) == 0 || invariant.Oracle == "" || strings.TrimSpace(invariant.ObservationBoundary) == "" || len(invariant.Checks) == 0 || !explicitSource(invariant.Source, invariant.SourceRef) {
			return fmt.Errorf("invariant %s needs scope, independent oracle, observation boundary and executable checks", invariant.ID)
		}
		for _, id := range invariant.Scope {
			if ids[id] != "capability" && ids[id] != "action" && ids[id] != "rule" {
				return fmt.Errorf("invariant %s has invalid scope %s", invariant.ID, id)
			}
		}
		for _, check := range invariant.Checks {
			if check.Left == "" || (check.Right == "") == (len(check.Value) == 0) {
				return fmt.Errorf("invariant %s check requires left and exactly one right/value", invariant.ID)
			}
			switch check.Operator {
			case compareEqual, compareNotEqual, compareLess, compareLessEqual, compareGreater, compareGreaterEqual:
			default:
				return fmt.Errorf("invalid property operator %q", check.Operator)
			}
			if len(check.Value) != 0 && !json.Valid(check.Value) {
				return fmt.Errorf("invariant %s has invalid comparison value", invariant.ID)
			}
		}
	}
	x := indexModel(g)
	existence := map[string]bool{}
	for _, test := range g.Tests {
		if strings.TrimSpace(test.Name) == "" || !explicitSource(test.Source, test.SourceRef) {
			return fmt.Errorf("test %s needs name and explicit source", test.ID)
		}
		switch test.Kind {
		case testPositive, testNegative, testForced, testTransition, testBoundary, testConcurrency, testIdempotency, testRecovery, testInvariant, testFault, testExistence, testSequence, testConsistency:
		default:
			return fmt.Errorf("invalid test kind %q", test.Kind)
		}
		if (test.Flow != "" || len(test.Steps) != 0) && test.Kind != testSequence {
			return fmt.Errorf("test %s declares SEQUENCE flow or steps on kind %s", test.ID, test.Kind)
		}
		if len(test.Layers) != 0 && test.Kind != testConsistency {
			return fmt.Errorf("test %s declares CONSISTENCY layers on kind %s", test.ID, test.Kind)
		}
		if test.Capability != "" && ids[test.Capability] != "capability" {
			return fmt.Errorf("test %s has invalid capability", test.ID)
		}
		aliases := map[string]bool{}
		for _, actor := range test.Actors {
			if actor.Alias == "" || aliases[actor.Alias] || ids[actor.Role] != "role" {
				return fmt.Errorf("test %s has invalid participant", test.ID)
			}
			aliases[actor.Alias] = true
		}
		if test.PendingReason != "" {
			if test.Capability == "" || test.Rule != "" && ids[test.Rule] != "rule" {
				return fmt.Errorf("pending test %s needs its capability", test.ID)
			}
			continue
		}
		rule, ok := rules[test.Rule]
		switch test.Kind {
		case testExistence:
			if !ok || test.Expected != "" || len(test.Preconditions) != 0 || test.Denied || len(test.Actors) != 0 {
				return fmt.Errorf("EXISTENCE test %s needs only its frozen rule; preconditions, expected state, denial and participants do not apply", test.ID)
			}
			if existence[test.Rule] {
				return fmt.Errorf("EXISTENCE for rule %s is declared twice", test.Rule)
			}
			existence[test.Rule] = true
			continue
		case testSequence:
			if err := validateSequence(x, test); err != nil {
				return err
			}
			continue
		}
		if !ok || test.Expected != "UNCHANGED" && ids[test.Expected] != "state" {
			return fmt.Errorf("test %s needs a frozen rule and expected state", test.ID)
		}
		if err := validateConditionAssignment(test.Preconditions, rule, ids); err != nil {
			return fmt.Errorf("test %s: %w", test.ID, err)
		}
		if test.Kind == testConsistency {
			if err := validateLayers(test); err != nil {
				return err
			}
		}
	}
	if err := validateEquivalencePolicy(g.Equivalence); err != nil {
		return err
	}
	if err := validateReferenceBindings(g.ReferenceBindings, ids); err != nil {
		return err
	}
	for _, attribute := range g.History {
		if err := add(attribute.ID, "history"); err != nil {
			return err
		}
		if err := validateHistory(x, attribute); err != nil {
			return err
		}
	}
	dependencies := map[string][]string{}
	for _, test := range g.Tests {
		for _, dependency := range test.DependsOn {
			if ids[dependency] != "test" {
				return fmt.Errorf("test %s has unknown dependency %s", test.ID, dependency)
			}
			dependencies[test.ID] = append(dependencies[test.ID], dependency)
		}
	}
	for _, edge := range g.Edges {
		if edge.Kind == "test_depends_on" {
			dependencies[edge.From] = append(dependencies[edge.From], edge.To)
		}
	}
	if cycle := dependencyCycle(dependencies); len(cycle) != 0 {
		return fmt.Errorf("test dependency cycle: %s", strings.Join(cycle, " -> "))
	}
	seen = map[string]bool{}
	for _, clause := range g.Constraints {
		if err := add(clause.ID, "constraint"); err != nil {
			return err
		}
		if len(clause.Literals) == 0 || !explicitSource(clause.Source, clause.SourceRef) {
			return fmt.Errorf("constraint %s needs explicit literals and source", clause.ID)
		}
		if err := validateLiterals(clause.Literals, ids); err != nil {
			return fmt.Errorf("constraint %s: %w", clause.ID, err)
		}
	}
	for _, guard := range g.Guards {
		rule, ok := rules[guard.Rule]
		if !ok || seen[guard.Rule] || len(guard.AnyOf) == 0 {
			return fmt.Errorf("invalid or duplicate guard for %s", guard.Rule)
		}
		seen[guard.Rule] = true
		required := map[string]bool{}
		for _, condition := range rule.Conditions {
			required[condition] = true
		}
		for _, group := range guard.AnyOf {
			if len(group) == 0 {
				return fmt.Errorf("guard %s has an empty AND group", guard.Rule)
			}
			if err := validateLiterals(group, ids); err != nil {
				return err
			}
			for _, literal := range group {
				if !required[literal.Condition] {
					return fmt.Errorf("guard %s references a condition absent from its frozen rule", guard.Rule)
				}
			}
		}
	}
	seen = map[string]bool{}
	for _, share := range g.SharedOwners {
		if ids[share.Subject] == "" || len(share.Owners) < 2 || strings.TrimSpace(share.Reason) == "" || strings.TrimSpace(share.SourceRef) == "" {
			return errors.New("shared ownership needs subject, owners, reason and source")
		}
		if seen[share.Subject] || len(owners[share.Subject]) != len(share.Owners) {
			return fmt.Errorf("shared ownership for %s does not match its owns relations", share.Subject)
		}
		seen[share.Subject] = true
		seenOwners := map[string]bool{}
		for _, owner := range share.Owners {
			if ids[owner] != "boundary" || seenOwners[owner] || !owners[share.Subject][owner] {
				return fmt.Errorf("shared owner %s is not a boundary", owner)
			}
			seenOwners[owner] = true
		}
	}
	seen = map[string]bool{}
	for _, scenario := range g.Applicability {
		key := scenario.Capability + "/" + string(scenario.Kind)
		if ids[scenario.Capability] != "capability" || seen[key] || scenario.Applicable == nil || strings.TrimSpace(scenario.Reason) == "" || !explicitSource(scenario.Source, scenario.SourceRef) {
			return fmt.Errorf("scenario applicability %s needs a capability, explicit boolean, reason and source", key)
		}
		switch scenario.Kind {
		case testBoundary, testConcurrency, testIdempotency, testRecovery, testFault:
		default:
			return fmt.Errorf("invalid optional scenario category %s", scenario.Kind)
		}
		seen[key] = true
	}
	generated := map[string]bool{}
	for _, obligation := range modelObligations(g) {
		if generated[obligation.Task.ID] {
			return fmt.Errorf("conflicting generated or declared test identity %q", obligation.Definition.ID)
		}
		generated[obligation.Task.ID] = true
	}
	return nil
}

// validateSequence requires a contiguous chain of declared transitions that
// starts at the flow's initial state and ends with the test's own rule.
func validateSequence(x modelIndex, test TestDefinition) error {
	flow, found := x.flows[test.Flow]
	if !found {
		return fmt.Errorf("SEQUENCE test %s references unknown flow %q", test.ID, test.Flow)
	}
	if len(test.Steps) < 2 || len(test.Steps) > 64 {
		return fmt.Errorf("SEQUENCE test %s needs 2 to 64 ordered steps", test.ID)
	}
	if len(test.Preconditions) != 0 {
		return fmt.Errorf("SEQUENCE test %s takes preconditions from each step's transition contract", test.ID)
	}
	from := flow.Initial
	for position, step := range test.Steps {
		contract, found := x.contracts[step]
		if !found {
			return fmt.Errorf("SEQUENCE test %s step %d (%s) is not a declared transition", test.ID, position+1, step)
		}
		edge := x.edges[step]
		if edge.From != from {
			return fmt.Errorf("SEQUENCE test %s step %d (%s) does not continue from state %s", test.ID, position+1, step, from)
		}
		inFlow := false
		for _, capability := range x.capsByAction[contract.Action] {
			inFlow = inFlow || capability == flow.Capability
		}
		if !inFlow {
			return fmt.Errorf("SEQUENCE test %s step %d action %s is outside flow capability %s", test.ID, position+1, contract.Action, flow.Capability)
		}
		from = edge.To
	}
	last := x.contracts[test.Steps[len(test.Steps)-1]]
	if test.Rule != last.Rule {
		return fmt.Errorf("SEQUENCE test %s rule must be its final step's rule %s", test.ID, last.Rule)
	}
	expected := x.edges[last.Edge].To
	if last.Outcome == branchRejection && x.rules[last.Rule].Rejected == "UNCHANGED" {
		expected = "UNCHANGED"
	}
	if test.Expected != expected || test.Denied != (last.Outcome == branchRejection) {
		return fmt.Errorf("SEQUENCE test %s expected state and denial must match its final step (%s)", test.ID, expected)
	}
	return nil
}

var observationLayers = map[string]bool{"ui": true, "api": true, "database": true, "async": true}

func validateLayers(test TestDefinition) error {
	if len(test.Layers) < 2 || len(test.Layers) > len(observationLayers) {
		return fmt.Errorf("CONSISTENCY test %s needs 2 to %d observation layers", test.ID, len(observationLayers))
	}
	layers, oracles := map[string]bool{}, map[string]bool{}
	for _, layer := range test.Layers {
		if !observationLayers[layer.Layer] {
			return fmt.Errorf("CONSISTENCY test %s has unknown layer %q", test.ID, layer.Layer)
		}
		if strings.TrimSpace(layer.Oracle) == "" || layers[layer.Layer] || oracles[layer.Oracle] {
			return fmt.Errorf("CONSISTENCY test %s repeats a layer or oracle, or leaves an oracle empty", test.ID)
		}
		layers[layer.Layer], oracles[layer.Oracle] = true, true
	}
	return nil
}

func validateConditionAssignment(values map[string]bool, rule Rule, kinds map[string]string) error {
	for _, condition := range rule.Conditions {
		if _, found := values[condition]; !found {
			return fmt.Errorf("missing observed precondition %s", condition)
		}
	}
	for condition := range values {
		if kinds[condition] != "condition" {
			return fmt.Errorf("unknown precondition %s", condition)
		}
	}
	return nil
}

func validateLiterals(literals []ConditionLiteral, kinds map[string]string) error {
	seen := map[string]bool{}
	for _, literal := range literals {
		if kinds[literal.Condition] != "condition" || seen[literal.Condition] {
			return fmt.Errorf("invalid or duplicate condition literal %s", literal.Condition)
		}
		seen[literal.Condition] = true
	}
	return nil
}

func dependencyCycle(edges map[string][]string) []string {
	state := map[string]int{}
	parent := map[string]string{}
	var visit func(string) []string
	visit = func(node string) []string {
		state[node] = 1
		for _, target := range edges[node] {
			if state[target] == 1 {
				cycle := []string{node}
				for current := node; current != target; {
					current = parent[current]
					cycle = append(cycle, current)
				}
				for left, right := 0, len(cycle)-1; left < right; left, right = left+1, right-1 {
					cycle[left], cycle[right] = cycle[right], cycle[left]
				}
				return append(cycle, target)
			}
			if state[target] == 0 {
				parent[target] = node
				if cycle := visit(target); len(cycle) != 0 {
					return cycle
				}
			}
		}
		state[node] = 2
		return nil
	}
	keys := make([]string, 0, len(edges))
	for node := range edges {
		keys = append(keys, node)
	}
	sort.Strings(keys)
	for _, node := range keys {
		if state[node] == 0 {
			if cycle := visit(node); len(cycle) != 0 {
				return cycle
			}
		}
	}
	return nil
}
