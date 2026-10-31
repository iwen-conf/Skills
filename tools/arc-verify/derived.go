package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// testDerived labels obligations derived at runtime from an observed failure.
const testDerived TestKind = "DERIVED"

// DerivedObligation asks a base obligation to hold again from the business
// state a failed run actually reached (template FROM_OBSERVED_STATE), or records
// an undeclared state for exploration (UNDECLARED_STATE, inference only).
type DerivedObligation struct {
	ID         string `json:"id"`
	Template   string `json:"template"`
	Origin     string `json:"origin"`
	Base       string `json:"base,omitempty"`
	Rule       string `json:"rule,omitempty"`
	StartState string `json:"start_state"`
	Key        string `json:"equivalence_key"`
	Score      int    `json:"score"`
	SourceRun  string `json:"source_run"`
	SourceStep int    `json:"source_step"`
	Reason     string `json:"reason"`
}

// DerivationReport is the outcome of deriving from one failed run.
type DerivationReport struct {
	Run       string              `json:"run"`
	Derived   []DerivedObligation `json:"derived"`
	Merged    []string            `json:"merged"`
	Skipped   []string            `json:"skipped"`
	Truncated bool                `json:"truncated"`
}

// observedFacts is the judged oracle's independent reading at one step.
type observedFacts struct {
	facts IndependentFacts
	actor string
	step  int
}

// derive turns the first divergence of a failed model run into new
// obligations. Rule-derived items join the acceptance denominator; an
// undeclared state is only an inferred exploration item. The original run,
// its first divergence and the frozen tasks are never changed.
func (d db) derive(run string, budget int) (DerivationReport, error) {
	report := DerivationReport{Run: run, Derived: []DerivedObligation{}, Merged: []string{}, Skipped: []string{}}
	if budget < 1 || budget > 256 {
		return report, errors.New("derivation budget must be 1..256")
	}
	contract, err := d.runTaskContract(run)
	if err != nil {
		return report, err
	}
	if contract.Model == nil || contract.Obligation == nil {
		return report, errors.New("derivation needs a model run")
	}
	if contract.Status != "FAIL" {
		return report, fmt.Errorf("derivation needs an independently observed failure; run %s is %s", run, contract.Status)
	}
	trace, err := d.trace(run)
	if err != nil {
		return report, err
	}
	observed, err := failureFacts(*contract.Model, *contract.Obligation, trace)
	if err != nil {
		return report, err
	}
	g := *contract.Model
	x := indexModel(g)
	approved := map[string]bool{}
	rows, err := d.query("SELECT id,approved FROM rules WHERE graph="+quote(g.ID), 2)
	if err != nil {
		return report, err
	}
	for _, row := range rows {
		approved[row[0]] = row[1] == "1"
	}
	state := observed.facts.State
	candidates := []DerivedObligation{}
	if modelState(g, state) {
		for _, edge := range nextTransitions(g, state) {
			transition := x.contracts[edge]
			if !approved[transition.Rule] {
				continue
			}
			key := runtimeKey(g, x, x.rules[transition.Rule].Action, observed, run)
			candidates = append(candidates, DerivedObligation{Template: "FROM_OBSERVED_STATE", Origin: "rule", Base: "transition/" + edge, Rule: transition.Rule, StartState: state, Key: key, Score: 1,
				Reason: fmt.Sprintf("declared transition %s must hold from the state %s that run %s actually reached", edge, state, run)})
		}
	} else if state != "" && state != "UNCHANGED" && state != "UNKNOWN" {
		key := runtimeKey(g, x, contract.Rule.Action, observed, run)
		candidates = append(candidates, DerivedObligation{Template: "UNDECLARED_STATE", Origin: "inference", Rule: contract.Rule.ID, StartState: state, Key: key, Score: 2,
			Reason: fmt.Sprintf("run %s reached state %s, which the model does not declare; explore it before confirming any rule", run, state)})
	}
	for index := range candidates {
		item := &candidates[index]
		digest := sha256.Sum256([]byte(item.Template + "\x00" + item.Base + "\x00" + item.Key))
		item.ID = g.ID + "/derived/" + hex.EncodeToString(digest[:8])
		item.SourceRun, item.SourceStep = run, observed.step
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].ID < candidates[j].ID
	})
	statements := []string{}
	for _, item := range candidates {
		existing, err := d.query("SELECT id FROM derived_obligations WHERE id="+quote(item.ID), 1)
		if err != nil {
			return report, err
		}
		if len(existing) == 1 {
			report.Merged = append(report.Merged, item.ID)
		} else if len(report.Derived) < budget {
			report.Derived = append(report.Derived, item)
			statements = append(statements, fmt.Sprintf("INSERT INTO derived_obligations(id,graph,template,origin,base,rule,start_state,equivalence_key,score,source_run,source_step,reason) VALUES(%s,%s,%s,%s,%s,%s,%s,%s,%d,%s,%d,%s);",
				quote(item.ID), quote(g.ID), quote(item.Template), quote(item.Origin), quote(item.Base), quote(item.Rule), quote(item.StartState), quote(item.Key), item.Score, quote(run), observed.step, quote(item.Reason)))
		} else {
			report.Skipped = append(report.Skipped, item.ID)
			report.Truncated = true
			continue
		}
		statements = append(statements, fmt.Sprintf("INSERT OR IGNORE INTO derivation_sources(derived,run,step) VALUES(%s,%s,%d);", quote(item.ID), quote(run), observed.step))
	}
	statements = append(statements, fmt.Sprintf("INSERT OR IGNORE INTO derivation_runs(run,graph,derived,truncated) VALUES(%s,%s,%d,%d);", quote(run), quote(g.ID), len(report.Derived), boolInt(report.Truncated)))
	return report, d.tx(statements)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// failureFacts reads the judged oracle at the first divergent step: the state
// after the action, or before it when the action never ran.
func failureFacts(g Graph, obligation ModelObligation, trace TraceReport) (observedFacts, error) {
	for _, step := range trace.Steps {
		if step.Index != trace.FirstDivergence || step.Observation == nil {
			continue
		}
		index, err := strconv.Atoi(step.Index)
		if err != nil {
			return observedFacts{}, err
		}
		judged := stepRule(indexModel(g), obligation, step.Observation.Transition)
		for _, oracle := range step.Observation.Oracles {
			if oracle.ID != judged.Oracle {
				continue
			}
			facts := oracle.After
			if facts.State == "" {
				facts = oracle.Before
			}
			return observedFacts{facts: facts, actor: oracle.Actor, step: index}, nil
		}
	}
	return observedFacts{}, errors.New("failed run has no verified independent observation at its first divergence")
}

// runtimeKey applies the design 05 runtime rule: the equivalence key plus every
// declared measurement except equivalence.ignore. An undecidable key never
// merges, so it is made unique to its source run.
func runtimeKey(g Graph, x modelIndex, action string, observed observedFacts, run string) string {
	ignored := map[string]bool{}
	if g.Equivalence != nil {
		for _, item := range g.Equivalence.Ignore {
			ignored[item.Measurement] = true
		}
	}
	measurements := map[string]json.RawMessage{}
	for name, value := range observed.facts.Values {
		if !ignored[name] {
			measurements[name] = value
		}
	}
	history := map[string]bool{}
	flow := ""
	for _, capability := range x.capsByAction[action] {
		for _, candidate := range g.Flows {
			if candidate.Capability == capability && (flow == "" || candidate.ID < flow) {
				flow = candidate.ID
			}
		}
	}
	for _, attribute := range flowHistory(g, flow) {
		var value bool
		if raw, found := observed.facts.Values[attribute.Measurement]; attribute.Measurement != "" && found && json.Unmarshal(raw, &value) == nil {
			history[attribute.ID] = value
		}
	}
	if flow == "" {
		basis := []string{"state=" + observed.facts.State, "role=" + observed.facts.Role, "principal=" + observed.actor}
		for name, value := range observed.facts.Conditions {
			basis = append(basis, fmt.Sprintf("%s=%t", name, value))
		}
		for name, value := range measurements {
			basis = append(basis, "m:"+name+"="+string(value))
		}
		sort.Strings(basis[3:])
		return strings.Join(basis, "|")
	}
	key, _, undecided := equivalenceKey(g, EquivalenceInput{Flow: flow, State: observed.facts.State, Role: observed.facts.Role, Principal: observed.actor, Conditions: map[string]bool(observed.facts.Conditions), History: history, Measurements: measurements})
	if undecided != "" {
		return "undecided:" + run + ":" + undecided
	}
	return key
}

// derivedObligationStatuses adds rule-derived obligations to the analysis. One
// is satisfied by a qualified run of its base obligation in the selected round
// whose independent before-state has the same equivalence key.
func (d db) derivedObligationStatuses(g Graph, round string, roundCurrent bool, staleReason string, approvals map[string]bool, catalogApproved bool) ([]ObligationStatus, error) {
	rows, err := d.query("SELECT id,base,rule,start_state,equivalence_key,source_run,source_step FROM derived_obligations WHERE graph="+quote(g.ID)+" AND origin='rule' ORDER BY id LIMIT 10001", 7)
	if err != nil {
		return nil, err
	}
	if len(rows) > 10000 {
		return nil, errors.New("derived obligations exceed 10000")
	}
	x := indexModel(g)
	bases := map[string]ModelObligation{}
	for _, obligation := range modelObligations(g) {
		bases[obligation.Definition.ID] = obligation
	}
	result := []ObligationStatus{}
	for _, row := range rows {
		base, found := bases[row[1]]
		if !found {
			return nil, fmt.Errorf("derived obligation %s refers to unknown base %s", row[0], row[1])
		}
		definition := TestDefinition{ID: strings.TrimPrefix(row[0], g.ID+"/"), Name: base.Definition.Name + " · from observed " + row[3], Rule: row[2], Kind: testDerived, Expected: base.Definition.Expected, Source: base.Definition.Source, SourceRef: "run:" + row[5] + "#" + row[6]}
		obligation := ModelObligation{Definition: definition, Task: taskSpec{ID: row[0], Rule: row[2], Kind: "DERIVED", Condition: row[1], Expected: base.Definition.Expected}, Targets: uniqueIDs(append(append([]string{}, base.Targets...), row[3])), Transition: base.Transition}
		status := ObligationStatus{ModelObligation: obligation, Status: "NOT_RUN", Reason: fmt.Sprintf("run %s as %s starting from a state equivalent to %s", row[1], row[0], row[3]), Planned: true}
		if !catalogApproved || !approvals[row[2]] {
			status.Status, status.Reason = "BLOCKED", "business catalog or rule needs explicit user approval"
			result = append(result, status)
			continue
		}
		runs, err := d.query("SELECT r.id,r.status FROM runs r JOIN model_runs m ON m.run=r.id WHERE r.task="+quote(g.ID+"/"+row[1])+" AND m.round="+quote(round)+" ORDER BY r.rowid", 2)
		if err != nil {
			return nil, err
		}
		for _, candidate := range runs {
			trace, err := d.trace(candidate[0])
			if err != nil || len(trace.Steps) == 0 || trace.Steps[0].Observation == nil {
				continue
			}
			judged := stepRule(x, base, trace.Steps[0].Observation.Transition)
			matched := false
			for _, oracle := range trace.Steps[0].Observation.Oracles {
				if oracle.ID == judged.Oracle {
					start := observedFacts{facts: oracle.Before, actor: oracle.Actor, step: 1}
					matched = runtimeKey(g, x, x.rules[row[2]].Action, start, candidate[0]) == row[4]
					break
				}
			}
			if !matched {
				continue
			}
			status.Run = candidate[0]
			switch candidate[1] {
			case "FAIL":
				status.Status, status.Reason = "FAIL", "base obligation failed from the equivalent start state"
			case "PASS":
				if !roundCurrent {
					status.Status, status.Reason = "STALE", staleReason
				} else if err := d.qualifyModelRun(candidate[0]); err != nil {
					status.Status, status.Reason = "STALE", err.Error()
				} else {
					status.Status, status.Reason = "PASS", "base obligation passed from the equivalent start state with current evidence"
				}
			default:
				status.Status, status.Reason = "BLOCKED", "execution from the equivalent start state did not complete"
			}
		}
		result = append(result, status)
	}
	return result, nil
}

// inferredDerivations lists exploration items that never enter acceptance.
func (d db) inferredDerivations(graph string) ([]DerivedObligation, error) {
	rows, err := d.query("SELECT id,template,rule,start_state,equivalence_key,score,source_run,source_step,reason FROM derived_obligations WHERE graph="+quote(graph)+" AND origin='inference' ORDER BY score DESC, id LIMIT 1001", 9)
	if err != nil {
		return nil, err
	}
	result := []DerivedObligation{}
	for _, row := range rows {
		score, _ := strconv.Atoi(row[5])
		step, _ := strconv.Atoi(row[7])
		result = append(result, DerivedObligation{ID: row[0], Template: row[1], Origin: "inference", Rule: row[2], StartState: row[3], Key: row[4], Score: score, SourceRun: row[6], SourceStep: step, Reason: row[8]})
	}
	return result, nil
}
