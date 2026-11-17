package main

import (
	"fmt"
	"strings"
	"unicode"
)

func businessIdentifier(id string) bool {
	return id != "" && len(id) <= 256 && strings.IndexFunc(id, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) < 0
}

// A typed transition has a per-prestate, sourced expectation. The old global
// Rule.Expected/Rejected cannot describe all those targets. Full validation
// below still checks its action, rule, exact from-state and allowed targets.
func hasTypedBusinessTransition(g Graph, edge string) bool {
	if g.BusinessContract != nil {
		for _, branch := range g.BusinessContract.Branches {
			if branch.Transition == edge {
				return true
			}
		}
	}
	return false
}

func validateBusinessAttributeEncoding(attribute BusinessAttribute) error {
	if attribute.Encoding == "" {
		return nil
	}
	if attribute.Encoding != "json_i64" || attribute.Type != "integer" {
		return fmt.Errorf("attribute encoding %q requires supported integer/json_i64", attribute.Encoding)
	}
	if strings.TrimSpace(attribute.SourceRef) == "" {
		return fmt.Errorf("encoded attribute requires an explicit source_ref")
	}
	return nil
}

func businessOperandType(operand *BusinessOperand, attributes map[string]businessAttributeInfo) (string, error) {
	if operand == nil || (operand.Ref == nil) == (operand.Literal == nil) {
		return "", fmt.Errorf("operand requires exactly one ref or typed literal")
	}
	if operand.Ref != nil {
		attribute, exists := attributes[operand.Ref.Key()]
		if !exists {
			return "", fmt.Errorf("undeclared attribute %s", operand.Ref.Key())
		}
		if err := validateBusinessAttributeEncoding(attribute.attribute); err != nil {
			return "", fmt.Errorf("attribute %s: %w", operand.Ref.Key(), err)
		}
		return attribute.attribute.Type, nil
	}
	if _, err := parseBusinessValue(operand.Literal.Type, operand.Literal.Value); err != nil {
		return "", fmt.Errorf("literal: %w", err)
	}
	return operand.Literal.Type, nil
}

func validateBusinessPredicate(predicate BusinessPredicate, attributes map[string]businessAttributeInfo, depth int, count *int) error {
	*count = *count + 1
	if depth > 32 || *count > 2048 {
		return fmt.Errorf("predicate exceeds depth 32 or 2048 nodes")
	}
	switch predicate.Op {
	case "true", "false":
		if len(predicate.Args) != 0 || predicate.Left != nil || predicate.Right != nil {
			return fmt.Errorf("%s takes no operands", predicate.Op)
		}
		return nil
	case "and", "or", "not":
		if predicate.Left != nil || predicate.Right != nil || len(predicate.Args) == 0 || predicate.Op == "not" && len(predicate.Args) != 1 {
			return fmt.Errorf("%s has invalid logical operands", predicate.Op)
		}
		for _, arg := range predicate.Args {
			if err := validateBusinessPredicate(arg, attributes, depth+1, count); err != nil {
				return err
			}
		}
		return nil
	}
	if len(predicate.Args) > 0 {
		return fmt.Errorf("comparison %s cannot have logical args", predicate.Op)
	}
	left, err := businessOperandType(predicate.Left, attributes)
	if err != nil {
		return err
	}
	right, err := businessOperandType(predicate.Right, attributes)
	if err != nil {
		return err
	}
	valid := false
	switch predicate.Op {
	case "eq", "ne":
		valid = businessCompatible(left, right)
	case "lt", "lte", "gt", "gte":
		valid = businessNumeric(left) && businessNumeric(right) || left == "string" && right == "string"
	case "in":
		valid = !businessSetType(left) && businessSetType(right) && businessCompatible(left, businessBaseType(right))
	case "contains":
		valid = businessSetType(left) && !businessSetType(right) && businessCompatible(businessBaseType(left), right)
	case "subset", "intersects":
		valid = businessSetType(left) && businessSetType(right) && businessCompatible(left, right)
	case "between":
		valid = businessNumeric(left) && businessSetType(right) && businessNumeric(businessBaseType(right)) && predicate.Right.Literal != nil
		if valid {
			bounds, _ := parseBusinessValue(right, predicate.Right.Literal.Value)
			if len(bounds.items) != 2 {
				return fmt.Errorf("between requires an explicit two-member numeric bound set")
			}
			comparison, _ := businessCompare(bounds.items[0], bounds.items[1])
			if comparison > 0 {
				return fmt.Errorf("between lower bound exceeds upper bound")
			}
		}
	default:
		return fmt.Errorf("unsupported business predicate operator %q", predicate.Op)
	}
	if !valid {
		return fmt.Errorf("operator %s cannot compare %s to %s", predicate.Op, left, right)
	}
	return nil
}

func validateBusinessCheck(check BusinessExpectedCheck, attributes map[string]businessAttributeInfo, invariant bool) error {
	if !businessIdentifier(check.ID) || strings.TrimSpace(check.Oracle) == "" || strings.TrimSpace(check.SourceRef) == "" {
		return fmt.Errorf("expected check needs stable id, independent oracle and source_ref")
	}
	switch check.Kind {
	case "response", "persistence", "event", "side_effect", "forbidden_side_effect", "invariant", "identity", "next_allowed":
	default:
		return fmt.Errorf("expected check %s has unsupported kind %s", check.ID, check.Kind)
	}
	if invariant && check.Kind != "invariant" {
		return fmt.Errorf("common check %s must be an invariant", check.ID)
	}
	count := 0
	if err := validateBusinessPredicate(check.Predicate, attributes, 0, &count); err != nil {
		return err
	}
	if len(businessPredicateRefs(check.Predicate)) == 0 {
		return fmt.Errorf("check %s must inspect independent observations", check.ID)
	}
	return nil
}

// ValidateBusinessContract validates only the optional deterministic extension.
// Existing Graph validation continues to own tree, state graph and legacy data.
// In particular, state transition cycles are never passed to the DAG checker.
func ValidateBusinessContract(g Graph) error {
	contract := g.BusinessContract
	if contract == nil {
		return nil
	}
	if g.SchemaVersion != 2 || contract.Version != 1 {
		return fmt.Errorf("business_contract version 1 requires graph schema_version 2")
	}
	if len(contract.Subjects) == 0 || len(contract.Subjects) > 256 || len(contract.Branches) == 0 || len(contract.Branches) > 2048 || len(contract.Preparation) > 128 {
		return fmt.Errorf("business_contract needs 1..256 subjects, 1..2048 branches and at most 128 preparation actions")
	}
	attributes := map[string]businessAttributeInfo{}
	subjects := map[string]bool{}
	for _, subject := range contract.Subjects {
		if !businessIdentifier(subject.ID) || strings.Contains(subject.ID, ".") || subjects[subject.ID] || len(subject.Attributes) == 0 || len(subject.Attributes) > 256 {
			return fmt.Errorf("invalid or duplicate business subject %q", subject.ID)
		}
		subjects[subject.ID] = true
		switch subject.Kind {
		case "actor", "account", "session", "entity", "relation", "environment":
		default:
			return fmt.Errorf("subject %s has unsupported kind %s", subject.ID, subject.Kind)
		}
		for _, attribute := range subject.Attributes {
			key := subject.ID + "." + attribute.Name
			if !businessIdentifier(attribute.Name) || strings.Contains(attribute.Name, ".") || !businessKnownType(attribute.Type) || attributes[key].attribute.Name != "" {
				return fmt.Errorf("invalid or duplicate typed attribute %s", key)
			}
			if err := validateBusinessAttributeEncoding(attribute); err != nil {
				return fmt.Errorf("attribute %s: %w", key, err)
			}
			attributes[key] = businessAttributeInfo{subject, attribute}
		}
	}
	if len(attributes) > 2048 {
		return fmt.Errorf("business contract exceeds 2048 attributes")
	}
	kinds := modelKinds(g)
	rules := map[string]Rule{}
	for _, rule := range g.Rules {
		rules[rule.ID] = rule
	}
	flows := map[string]BehaviorFlow{}
	for _, flow := range g.Flows {
		flows[flow.ID] = flow
	}
	transitions := map[string]TransitionContract{}
	for _, transition := range g.Transitions {
		transitions[transition.Edge] = transition
	}
	edges := map[string]Edge{}
	for _, edge := range g.Edges {
		edges[edge.ID] = edge
	}
	seenBranches := map[string]bool{}
	for _, branch := range contract.Branches {
		if !businessIdentifier(branch.ID) || seenBranches[branch.ID] || kinds[branch.Action] != "action" || rules[branch.Rule].Action != branch.Action || !explicitSource(branch.Source, branch.SourceRef) {
			return fmt.Errorf("branch %s needs unique id, declared action/rule and explicit source", branch.ID)
		}
		seenBranches[branch.ID] = true
		if len(branch.FromStates) == 0 {
			return fmt.Errorf("branch %s must explicitly declare from_states", branch.ID)
		}
		states := map[string]bool{}
		for _, state := range branch.FromStates {
			if kinds[state] != "state" || states[state] {
				return fmt.Errorf("branch %s has invalid or duplicate from_state %s", branch.ID, state)
			}
			states[state] = true
		}
		if branch.Flow != "" && flows[branch.Flow].ID == "" {
			return fmt.Errorf("branch %s has unknown flow %s", branch.ID, branch.Flow)
		}
		if branch.Capability != "" && kinds[branch.Capability] != "capability" {
			return fmt.Errorf("branch %s has unknown capability", branch.ID)
		}
		if branch.Capability != "" {
			implemented := false
			for _, edge := range g.Edges {
				if edge.Kind == "implements" && edge.From == branch.Capability && edge.To == branch.Action {
					implemented = true
				}
			}
			if !implemented {
				return fmt.Errorf("branch %s capability does not implement its action", branch.ID)
			}
			if branch.Flow != "" && !flowHasCapability(flows[branch.Flow], branch.Capability) {
				return fmt.Errorf("branch %s capability is outside its flow", branch.ID)
			}
		}
		seenTests := map[string]bool{}
		for _, test := range branch.Obligations {
			if kinds[test] != "test" || seenTests[test] {
				return fmt.Errorf("branch %s has unknown or duplicate test obligation %s", branch.ID, test)
			}
			seenTests[test] = true
		}
		count := 0
		if err := validateBusinessPredicate(branch.When, attributes, 0, &count); err != nil {
			return fmt.Errorf("branch %s: %w", branch.ID, err)
		}
		expected := branch.Expected
		if expected.Mode != "deterministic" && expected.Mode != "allowed_outcomes" || expected.Mode == "deterministic" && len(expected.Outcomes) != 1 || expected.Mode == "allowed_outcomes" && (len(expected.Outcomes) < 2 || strings.TrimSpace(expected.Reason) == "") || len(expected.Outcomes) > 64 {
			return fmt.Errorf("branch %s must declare one deterministic outcome or a sourced, bounded allowed_outcomes set with reason", branch.ID)
		}
		outcomeIDs, checkIDs := map[string]bool{}, map[string]bool{}
		for _, check := range expected.CommonInvariants {
			if err := validateBusinessCheck(check, attributes, true); err != nil {
				return fmt.Errorf("branch %s: %w", branch.ID, err)
			}
			if checkIDs[check.ID] {
				return fmt.Errorf("branch %s duplicates expected check %s", branch.ID, check.ID)
			}
			checkIDs[check.ID] = true
		}
		for _, outcome := range expected.Outcomes {
			if !businessIdentifier(outcome.ID) || outcomeIDs[outcome.ID] || outcome.State != "" && outcome.State != "UNCHANGED" && kinds[outcome.State] != "state" {
				return fmt.Errorf("branch %s has invalid or duplicate expected outcome %s", branch.ID, outcome.ID)
			}
			outcomeIDs[outcome.ID] = true
			count = 0
			if err := validateBusinessPredicate(outcome.Predicate, attributes, 0, &count); err != nil {
				return fmt.Errorf("outcome %s: %w", outcome.ID, err)
			}
			if outcome.State == "" && outcome.Denied == nil && len(businessPredicateRefs(outcome.Predicate)) == 0 && len(outcome.Checks) == 0 {
				return fmt.Errorf("outcome %s has no observable expectation", outcome.ID)
			}
			localCheckIDs := map[string]bool{}
			for _, check := range outcome.Checks {
				if err := validateBusinessCheck(check, attributes, false); err != nil {
					return fmt.Errorf("outcome %s: %w", outcome.ID, err)
				}
				if checkIDs[check.ID] || localCheckIDs[check.ID] {
					return fmt.Errorf("outcome %s duplicates check %s", outcome.ID, check.ID)
				}
				localCheckIDs[check.ID] = true
			}
			next := map[string]bool{}
			for _, action := range outcome.NextAllowed {
				if kinds[action] != "action" || next[action] {
					return fmt.Errorf("outcome %s has invalid next_allowed action %s", outcome.ID, action)
				}
				next[action] = true
			}
		}
		if branch.Transition != "" {
			transition, exists := transitions[branch.Transition]
			edge := edges[branch.Transition]
			if !exists || transition.Action != branch.Action || transition.Rule != branch.Rule || !states[edge.From] || len(branch.FromStates) != 1 {
				return fmt.Errorf("branch %s transition does not match action, rule or prestate", branch.ID)
			}
			found := false
			for _, outcome := range expected.Outcomes {
				if outcome.State == edge.To || outcome.State == "UNCHANGED" && edge.From == edge.To {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("branch %s transition target is outside its expected outcomes", branch.ID)
			}
		}
	}
	preparations := map[string]PreparationAction{}
	for _, preparation := range contract.Preparation {
		if !businessIdentifier(preparation.ID) || preparations[preparation.ID].ID != "" || kinds[preparation.Action] != "action" || !explicitSource(preparation.Source, preparation.SourceRef) || len(preparation.Effects) == 0 || len(preparation.Effects) > 256 {
			return fmt.Errorf("preparation %s needs id, declared action, sourced contract and explicit effects", preparation.ID)
		}
		preparations[preparation.ID] = preparation
		for _, predicate := range []BusinessPredicate{preparation.Requires, preparation.Ensures} {
			count := 0
			if err := validateBusinessPredicate(predicate, attributes, 0, &count); err != nil {
				return fmt.Errorf("preparation %s: %w", preparation.ID, err)
			}
		}
		written := map[string]bool{}
		for _, effect := range preparation.Effects {
			key := effect.Subject + "." + effect.Attribute
			info, exists := attributes[key]
			if !exists || written[key] {
				return fmt.Errorf("preparation %s has unknown or duplicate effect %s", preparation.ID, key)
			}
			written[key] = true
			if _, err := parseBusinessAttributeValue(info.attribute, effect.Value); err != nil {
				return fmt.Errorf("preparation %s effect %s: %w", preparation.ID, key, err)
			}
		}
	}
	dependencies := map[string][]string{}
	for _, preparation := range contract.Preparation {
		seen := map[string]bool{}
		for _, dependency := range preparation.DependsOn {
			if preparations[dependency].ID == "" || seen[dependency] {
				return fmt.Errorf("preparation %s has unknown or duplicate dependency %s", preparation.ID, dependency)
			}
			seen[dependency] = true
			dependencies[preparation.ID] = append(dependencies[preparation.ID], dependency)
		}
	}
	if cycle := dependencyCycle(dependencies); len(cycle) > 0 {
		return fmt.Errorf("preparation dependency cycle: %s", strings.Join(cycle, " -> "))
	}
	if coverage := contract.Coverage; coverage != nil {
		if strings.TrimSpace(coverage.Scope) == "" || coverage.MaxCases < 1 || coverage.MaxCases > 1000000 || len(coverage.Scopes) == 0 || len(coverage.Scopes) > 2048 {
			return fmt.Errorf("finite coverage requires explicit scope, action/flow/state scopes and max_cases 1..1000000")
		}
		seen := map[string]bool{}
		for _, domain := range coverage.Domains {
			key := domain.Subject + "." + domain.Attribute
			info, exists := attributes[key]
			if !exists || seen[key] || len(domain.Values) == 0 || len(domain.Values) > 1024 {
				return fmt.Errorf("coverage domain %s needs declared attribute and 1..1024 distinct typed values", key)
			}
			seen[key] = true
			values := map[string]bool{}
			for _, raw := range domain.Values {
				value, err := parseBusinessAttributeValue(info.attribute, raw)
				if err != nil || values[value.key()] {
					return fmt.Errorf("coverage domain %s has ill-typed or duplicate values", key)
				}
				values[value.key()] = true
			}
		}
		seen = map[string]bool{}
		for _, scope := range coverage.Scopes {
			if kinds[scope.Action] != "action" || scope.Flow != "" && flows[scope.Flow].ID == "" || len(scope.States) == 0 || len(scope.States) > 1024 {
				return fmt.Errorf("coverage action/flow/state scope is invalid")
			}
			for _, state := range scope.States {
				key := scope.Action + "\x00" + scope.Flow + "\x00" + state
				if kinds[state] != "state" || seen[key] {
					return fmt.Errorf("coverage has invalid or repeated action/flow/state scope")
				}
				seen[key] = true
			}
		}
	}
	return nil
}
