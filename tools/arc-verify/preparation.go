package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type preparationSearchState struct {
	observation BusinessObservation
	path        []int
	used        map[int]bool
}

func preparationWithEffects(observed BusinessObservation, effects []PreparationEffect) BusinessObservation {
	next := observed
	next.Values = make(map[string]json.RawMessage, len(observed.Values)+len(effects))
	for key, value := range observed.Values {
		next.Values[key] = value
	}
	for _, effect := range effects {
		next.Values[effect.Subject+"."+effect.Attribute] = effect.Value
	}
	return next
}

func preparationWrites(action PreparationAction) []string {
	keys := make([]string, 0, len(action.Effects))
	for _, effect := range action.Effects {
		keys = append(keys, effect.Subject+"."+effect.Attribute)
	}
	return uniqueSortedBusiness(keys)
}

func preparationSatisfaction(action PreparationAction, observed BusinessObservation, attributes map[string]businessAttributeInfo) TruthEvaluation {
	ensures := evaluateBusinessPredicate(action.Ensures, observed, attributes)
	if ensures.Value == businessFalse {
		return ensures
	}
	unknown := append([]string(nil), ensures.Unknown...)
	// Ensures can be weaker than the declared effects. Omit an action only
	// when its full predicted effect is already independently known; otherwise
	// it may still be the sole producer of a missing preparation goal.
	for _, effect := range action.Effects {
		info, exists := attributes[effect.Subject+"."+effect.Attribute]
		if !exists {
			return unknownBusiness([]string{effect.Subject + "." + effect.Attribute}, "preparation effect is not declared")
		}
		actual, missing := resolveBusinessOperand(BusinessOperand{Ref: &BusinessAttributeRef{Subject: effect.Subject, Attribute: effect.Attribute}}, observed, attributes)
		expected, err := parseBusinessAttributeValue(info.attribute, effect.Value)
		if err != nil {
			return unknownBusiness([]string{effect.Subject + "." + effect.Attribute}, "preparation effect is ill-typed")
		}
		if len(missing) > 0 {
			unknown = append(unknown, missing...)
		} else if !businessValueEqual(actual, expected) {
			return businessTruth(false)
		}
	}
	if ensures.Value == businessUnknown || len(unknown) > 0 {
		return unknownBusiness(unknown, "preparation effects need independent observations")
	}
	return businessTruth(true)
}

func intersectsBusinessIDs(left, right []string) bool {
	seen := map[string]bool{}
	for _, id := range left {
		seen[id] = true
	}
	for _, id := range right {
		if seen[id] {
			return true
		}
	}
	return false
}

func preparationLayers(actions []PreparationAction, path []int) []PreparationLayer {
	layers := []PreparationLayer{}
	placed := make([]int, len(path))
	byID := map[string]PreparationAction{}
	for _, action := range actions {
		byID[action.ID] = action
	}
	reads := func(action PreparationAction) []string {
		result := append(businessPredicateRefs(action.Requires), businessPredicateRefs(action.Ensures)...)
		for _, dependency := range action.DependsOn {
			result = append(result, businessPredicateRefs(byID[dependency].Ensures)...)
			result = append(result, preparationWrites(byID[dependency])...)
		}
		return uniqueSortedBusiness(result)
	}
	for position, index := range path {
		action := actions[index]
		level := 0
		for previous := 0; previous < position; previous++ {
			predecessor := actions[path[previous]]
			// Read/write and write/write conflicts preserve the discovered order.
			// Sharing a read-only account attribute alone does not serialize actions.
			ordered := containsBusinessID(action.DependsOn, predecessor.ID) ||
				intersectsBusinessIDs(preparationWrites(predecessor), reads(action)) ||
				intersectsBusinessIDs(preparationWrites(action), reads(predecessor)) ||
				intersectsBusinessIDs(preparationWrites(action), preparationWrites(predecessor))
			if ordered && placed[previous]+1 > level {
				level = placed[previous] + 1
			}
		}
		placed[position] = level
		for len(layers) <= level {
			layers = append(layers, PreparationLayer{Index: len(layers), Steps: []PlannedPreparationStep{}})
		}
		layers[level].Steps = append(layers[level].Steps, PlannedPreparationStep{ID: action.ID, Action: action.Action, Requires: action.Requires, Ensures: action.Ensures, Effects: action.Effects, SourceRef: action.SourceRef})
	}
	for index := range layers {
		sort.Slice(layers[index].Steps, func(i, j int) bool { return layers[index].Steps[i].ID < layers[index].Steps[j].ID })
	}
	return layers
}

// PlanPreparation searches a bounded subset of the declared preparation DAG
// starting at real Observed facts. Effects are predictions, never observations.
// READY is a plan, not a successful test: each executed layer must re-read its
// requires/ensures and stop/replan if the SUT differs. State graph cycles are
// legal and never fed into this preparation scheduler.
func PlanPreparation(g Graph, request PreparationRequest) PreparationPlan {
	result := PreparationPlan{Status: "BLOCKED", Layers: []PreparationLayer{}, AlreadySatisfied: []string{}, Blocked: []string{}, Gaps: []string{}, MaxStates: request.MaxStates, MaxSteps: request.MaxSteps, RequiresReadback: true}
	result.ObservedSHA256, _ = businessJSONHash(request.Observed)
	if g.BusinessContract == nil {
		result.Status, result.Reason = "LEGACY_UNDECLARED", "no deterministic preparation contract is declared"
		return result
	}
	if err := ValidateBusinessContract(g); err != nil {
		result.Status, result.Reason = "MODEL_ERROR", err.Error()
		return result
	}
	if request.MaxSteps < 1 || request.MaxSteps > 128 || request.MaxStates < 1 || request.MaxStates > 10000 {
		result.Reason = "preparation requires explicit max_steps 1..128 and max_states 1..10000"
		return result
	}
	attributes := businessAttributeIndex(g.BusinessContract)
	count := 0
	if err := validateBusinessPredicate(request.Goal, attributes, 0, &count); err != nil {
		result.Status, result.Reason = "MODEL_ERROR", "preparation goal: "+err.Error()
		return result
	}
	initialGoal := evaluateBusinessPredicate(request.Goal, request.Observed, attributes)
	actions := append([]PreparationAction(nil), g.BusinessContract.Preparation...)
	sort.Slice(actions, func(i, j int) bool { return actions[i].ID < actions[j].ID })
	byID := map[string]PreparationAction{}
	for _, action := range actions {
		byID[action.ID] = action
		if preparationSatisfaction(action, request.Observed, attributes).Value == businessTrue {
			result.AlreadySatisfied = append(result.AlreadySatisfied, action.ID)
		}
	}
	if initialGoal.Value == businessTrue {
		result.Status, result.Reason = "ALREADY_SATISFIED", "independent observed attributes already satisfy the preparation goal"
		return result
	}
	blocked, gaps := map[string]bool{}, map[string]bool{}
	queue := []preparationSearchState{{observation: request.Observed, used: map[int]bool{}}}
	seen := map[string]bool{}
	budgetExhausted := false
	// Bound both search states and allocated attribute cells. A declaration can
	// contain 2048 attributes; max_states alone would permit excessive memory.
	stateLimit := request.MaxStates
	if cells := len(request.Observed.Values) + len(attributes); cells > 0 && stateLimit > 2000000/cells {
		stateLimit = 2000000 / cells
	}
	if stateLimit < 1 {
		stateLimit = 1
	}
	for head := 0; head < len(queue); head++ {
		if result.ExploredStates >= stateLimit {
			budgetExhausted = true
			break
		}
		current := queue[head]
		// Release retained maps as breadth-first exploration advances.
		queue[head] = preparationSearchState{}
		result.ExploredStates++
		if evaluateBusinessPredicate(request.Goal, current.observation, attributes).Value == businessTrue {
			result.Status, result.Reason = "READY", "bounded preparation plan reaches the goal under declared effects; every action still requires execution and independent readback"
			result.Layers = preparationLayers(actions, current.path)
			return result
		}
		if len(current.path) >= request.MaxSteps {
			budgetExhausted = true
			continue
		}
		for index, action := range actions {
			if current.used[index] {
				continue
			}
			if preparationSatisfaction(action, current.observation, attributes).Value == businessTrue {
				continue
			}
			dependenciesReady := true
			for _, dependency := range action.DependsOn {
				observed := preparationSatisfaction(byID[dependency], current.observation, attributes)
				if observed.Value != businessTrue {
					dependenciesReady = false
					if observed.Value == businessUnknown {
						for _, key := range observed.Unknown {
							blocked[dependency+":"+key] = true
						}
					}
				}
			}
			if !dependenciesReady {
				continue
			}
			requirements := evaluateBusinessPredicate(action.Requires, current.observation, attributes)
			if requirements.Value == businessUnknown {
				for _, key := range requirements.Unknown {
					blocked[action.ID+":"+key] = true
				}
				continue
			}
			if requirements.Value == businessFalse {
				continue
			}
			next := preparationWithEffects(current.observation, action.Effects)
			postcondition := evaluateBusinessPredicate(action.Ensures, next, attributes)
			if postcondition.Value == businessFalse {
				result.Status, result.Reason = "MODEL_ERROR", fmt.Sprintf("preparation %s declared effects contradict its ensures under a reachable preparation state", action.ID)
				return result
			}
			if postcondition.Value == businessUnknown {
				for _, key := range postcondition.Unknown {
					blocked[action.ID+":ensures:"+key] = true
				}
				continue
			}
			path := append(append([]int(nil), current.path...), index)
			used := make(map[int]bool, len(current.used)+1)
			for usedIndex := range current.used {
				used[usedIndex] = true
			}
			used[index] = true
			usedIDs := []int{}
			for usedIndex := range used {
				usedIDs = append(usedIDs, usedIndex)
			}
			sort.Ints(usedIDs)
			key, err := businessJSONHash(struct {
				Values map[string]json.RawMessage `json:"values"`
				Used   []int                      `json:"used"`
			}{next.Values, usedIDs})
			if err != nil {
				result.Reason = "preparation observation cannot be frozen: " + err.Error()
				return result
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			if len(queue) >= stateLimit {
				budgetExhausted = true
				continue
			}
			queue = append(queue, preparationSearchState{observation: next, path: path, used: used})
		}
	}
	produced := map[string]bool{}
	for _, action := range actions {
		for _, key := range preparationWrites(action) {
			produced[key] = true
		}
	}
	for _, key := range businessPredicateRefs(request.Goal) {
		if !produced[key] {
			gaps[key] = true
		}
	}
	for key := range blocked {
		result.Blocked = append(result.Blocked, key)
	}
	if initialGoal.Value == businessUnknown {
		result.Blocked = append(result.Blocked, initialGoal.Unknown...)
	}
	for key := range gaps {
		result.Gaps = append(result.Gaps, key)
	}
	result.Blocked, result.Gaps = uniqueSortedBusiness(result.Blocked), uniqueSortedBusiness(result.Gaps)
	switch {
	case budgetExhausted:
		result.Status, result.Reason = "BUDGET_EXHAUSTED", fmt.Sprintf("no preparation proof within max_steps=%d and state budget=%d (attribute-cell bound may reduce it); unexplored plans are not declared impossible", request.MaxSteps, stateLimit)
	case len(result.Blocked) > 0:
		result.Reason = "preparation is blocked by unknown observations or bindings: " + strings.Join(result.Blocked, ", ")
	default:
		result.Status, result.Reason = "SPECIFICATION_GAP", "no declared acyclic preparation plan reaches the goal from the observed facts"
	}
	return result
}
