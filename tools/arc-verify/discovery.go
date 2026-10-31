package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Exploration is what an exploring runner (browser, device or HTTP) observed;
// the CLI only registers it and compares it with the business model.
type Exploration struct {
	Graph        string                `json:"graph"`
	Exploration  string                `json:"exploration"`
	Observations []ExploredObservation `json:"observations"`
}

type ExploredObservation struct {
	Entry        string                     `json:"entry"`
	Role         string                     `json:"role"`
	Principal    string                     `json:"principal"`
	Action       string                     `json:"action"`
	BeforeState  string                     `json:"before_state"`
	AfterState   string                     `json:"after_state"`
	Denied       bool                       `json:"denied"`
	Conditions   map[string]bool            `json:"conditions"`
	Measurements map[string]json.RawMessage `json:"measurements"`
	Evidence     string                     `json:"evidence"`
}

// UnspecifiedItem is behavior observed outside the business model. It is never
// PASS or FAIL; only the user's triage gives it a meaning.
type UnspecifiedItem struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Entry          string   `json:"entry"`
	Role           string   `json:"role"`
	Principal      string   `json:"principal"`
	Action         string   `json:"action"`
	Capabilities   []string `json:"capabilities,omitempty"`
	BeforeState    string   `json:"before_state"`
	AfterState     string   `json:"after_state"`
	Denied         bool     `json:"denied"`
	Reason         string   `json:"reason"`
	Evidence       string   `json:"evidence"`
	EvidenceSHA256 string   `json:"evidence_sha256"`
	Key            string   `json:"equivalence_key"`
	Status         string   `json:"status"`
	Triage         string   `json:"triage"`
	Words          string   `json:"words,omitempty"`
}

// DiscoveryReport is the outcome of registering one exploration.
type DiscoveryReport struct {
	Exploration    string            `json:"exploration"`
	WithinContract int               `json:"within_contract"`
	Unspecified    []UnspecifiedItem `json:"unspecified"`
	Merged         []string          `json:"merged"`
	Truncated      bool              `json:"truncated"`
}

var triageDecisions = map[string]bool{"missing_requirement": true, "allowed_extension": true, "should_not_exist": true}

// recordExploration keeps observations the model does not declare as
// UNSPECIFIED items, deduplicated by the runtime equivalence key; behavior the
// model declares is only counted.
func (d db) recordExploration(exploration Exploration, budget int) (DiscoveryReport, error) {
	report := DiscoveryReport{Exploration: exploration.Exploration, Unspecified: []UnspecifiedItem{}, Merged: []string{}}
	if budget < 1 || budget > 1024 {
		return report, errors.New("discovery budget must be 1..1024")
	}
	if strings.TrimSpace(exploration.Exploration) == "" || len(exploration.Observations) == 0 || len(exploration.Observations) > 10000 {
		return report, errors.New("exploration needs an id and 1..10000 observations")
	}
	g, err := d.model(exploration.Graph)
	if err != nil {
		return report, err
	}
	if g.SchemaVersion != 2 {
		return report, errors.New("discovery requires a v2 business model")
	}
	x := indexModel(g)
	statements := []string{}
	pending := map[string]bool{}
	for index, observation := range exploration.Observations {
		if observation.Entry == "" || observation.Role == "" || observation.Principal == "" || observation.Action == "" || observation.BeforeState == "" || observation.AfterState == "" {
			return report, fmt.Errorf("observation %d needs entry, role, principal, action and both states", index+1)
		}
		path, raw, err := loadEvidence(observation.Evidence)
		if err != nil {
			return report, fmt.Errorf("observation %d evidence: %w", index+1, err)
		}
		kind, reason := classifyObservation(g, x, observation)
		if kind == "" {
			report.WithinContract++
			continue
		}
		source := fmt.Sprintf("%s#%d", exploration.Exploration, index+1)
		facts := observedFacts{facts: IndependentFacts{State: observation.BeforeState, Role: observation.Role, Conditions: ObservedConditions(observation.Conditions), Values: observation.Measurements}, actor: observation.Principal}
		key := runtimeKey(g, x, observation.Action, facts, source)
		facts.facts.Values = nil
		baseKey := runtimeKey(g, x, observation.Action, facts, source)
		identity := kind + "|" + observation.Action + "|" + observation.AfterState
		item := UnspecifiedItem{Kind: kind, Entry: observation.Entry, Role: observation.Role, Principal: observation.Principal, Action: observation.Action, Capabilities: actionCapabilities(g, observation.Action), BeforeState: observation.BeforeState, AfterState: observation.AfterState, Denied: observation.Denied, Reason: reason, Evidence: path, EvidenceSHA256: digestBytes(raw), Key: key, Status: "UNSPECIFIED", Triage: "OPEN"}
		item.ID = exploration.Graph + "/unspecified/" + shortDigest(identity+"|"+key)
		exists, err := d.query("SELECT id FROM unspecified_items WHERE id="+quote(item.ID), 1)
		if err != nil {
			return report, err
		}
		if len(exists) == 1 || pending[item.ID] {
			report.Merged = append(report.Merged, item.ID)
			statements = append(statements, fmt.Sprintf("INSERT OR IGNORE INTO unspecified_sources(item,exploration,evidence_sha256) VALUES(%s,%s,%s);", quote(item.ID), quote(source), quote(item.EvidenceSHA256)))
			continue
		}
		sibling, err := d.query("SELECT id FROM unspecified_items WHERE graph="+quote(exploration.Graph)+" AND identity="+quote(identity)+" AND base_key="+quote(baseKey)+" ORDER BY id LIMIT 1", 1)
		if err != nil {
			return report, err
		}
		if len(sibling) == 1 {
			item.Kind = "UNDECLARED_DIFFERENCE"
			item.Reason = fmt.Sprintf("same equivalent state as %s except undeclared measurements; declare them in equivalence.ignore or as history, or keep them apart", sibling[0][0])
			item.ID = exploration.Graph + "/unspecified/" + shortDigest("DIFFERENCE|"+identity+"|"+key)
		}
		if len(report.Unspecified) >= budget {
			report.Truncated = true
			continue
		}
		pending[item.ID] = true
		report.Unspecified = append(report.Unspecified, item)
		statements = append(statements,
			fmt.Sprintf("INSERT INTO unspecified_items(id,graph,kind,identity,entry,role,principal,action,before_state,after_state,denied,reason,evidence,evidence_sha256,equivalence_key,base_key,exploration) VALUES(%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%d,%s,%s,%s,%s,%s,%s);",
				quote(item.ID), quote(exploration.Graph), quote(item.Kind), quote(identity), quote(item.Entry), quote(item.Role), quote(item.Principal), quote(item.Action), quote(item.BeforeState), quote(item.AfterState), boolInt(item.Denied), quote(item.Reason), quote(item.Evidence), quote(item.EvidenceSHA256), quote(item.Key), quote(baseKey), quote(source)),
			fmt.Sprintf("INSERT OR IGNORE INTO unspecified_sources(item,exploration,evidence_sha256) VALUES(%s,%s,%s);", quote(item.ID), quote(source), quote(item.EvidenceSHA256)))
	}
	return report, d.tx(statements)
}

func shortDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

// classifyObservation returns "" when the model declares the observed behavior.
func classifyObservation(g Graph, x modelIndex, o ExploredObservation) (string, string) {
	if x.kinds[o.Action] != "action" {
		return "UNDECLARED_ACTION", "action " + o.Action + " is not in the business model"
	}
	for _, state := range []string{o.BeforeState, o.AfterState} {
		if !modelState(g, state) {
			return "UNDECLARED_STATE", "state " + state + " is not in the business model"
		}
	}
	rules := []Rule{}
	for _, rule := range g.Rules {
		if rule.Action == o.Action {
			rules = append(rules, rule)
		}
	}
	if o.Denied {
		if o.BeforeState != o.AfterState {
			return "UNDECLARED_TRANSITION", "the action was rejected but the state changed"
		}
		if len(rules) == 0 {
			return "UNDECLARED_TRANSITION", "no rule declares the action that was rejected"
		}
		return "", ""
	}
	contracts, mismatch := 0, ""
	for _, contract := range g.Transitions {
		if contract.Action != o.Action {
			continue
		}
		contracts++
		edge := x.edges[contract.Edge]
		if edge.From != o.BeforeState || edge.To != o.AfterState || contract.Outcome == branchRejection {
			continue
		}
		unmet := []string{}
		for condition, value := range contract.Preconditions {
			if observed, found := o.Conditions[condition]; !found || observed != value {
				unmet = append(unmet, condition)
			}
		}
		if len(unmet) == 0 {
			return "", ""
		}
		sort.Strings(unmet)
		mismatch = fmt.Sprintf("accepted although the declared preconditions of %s were not observed as required: %s", contract.Edge, strings.Join(unmet, ", "))
	}
	if contracts == 0 {
		for _, rule := range rules {
			if rule.Expected == o.AfterState && ruleAllows(g, rule, o.Conditions) {
				return "", ""
			}
		}
	}
	if mismatch != "" {
		return "UNDECLARED_TRANSITION", mismatch
	}
	return "UNDECLARED_TRANSITION", fmt.Sprintf("no declared transition of %s leads from %s to %s", o.Action, o.BeforeState, o.AfterState)
}

const unspecifiedTriageSQL = "COALESCE((SELECT t.decision FROM unspecified_triage t WHERE t.item=u.id ORDER BY t.seq DESC LIMIT 1),'OPEN')"

func (d db) unspecified(graph string) ([]UnspecifiedItem, error) {
	rows, err := d.query("SELECT u.id,u.kind,u.entry,u.role,u.principal,u.action,u.before_state,u.after_state,u.denied,u.reason,u.evidence,u.evidence_sha256,u.equivalence_key,"+unspecifiedTriageSQL+",COALESCE((SELECT t.words FROM unspecified_triage t WHERE t.item=u.id ORDER BY t.seq DESC LIMIT 1),'') FROM unspecified_items u WHERE u.graph="+quote(graph)+" ORDER BY u.id LIMIT 10001", 15)
	if err != nil {
		return nil, err
	}
	g, err := d.model(graph)
	if err != nil {
		return nil, err
	}
	result := []UnspecifiedItem{}
	for _, row := range rows {
		result = append(result, UnspecifiedItem{ID: row[0], Kind: row[1], Entry: row[2], Role: row[3], Principal: row[4], Action: row[5], Capabilities: actionCapabilities(g, row[5]), BeforeState: row[6], AfterState: row[7], Denied: row[8] == "1", Reason: row[9], Evidence: row[10], EvidenceSHA256: row[11], Key: row[12], Status: "UNSPECIFIED", Triage: row[13], Words: row[14]})
	}
	return result, nil
}

// triageUnspecified records the user's decision once: a missing requirement is
// handed to arc-plan as a candidate (the frozen graph keeps its rules), a
// behavior that should not exist becomes a risk, an allowed extension is
// excluded with its reason.
func (d db) triageUnspecified(graph, id, decision, words string) error {
	words = strings.TrimSpace(words)
	if !triageDecisions[decision] {
		return errors.New("triage decision must be missing_requirement, allowed_extension or should_not_exist")
	}
	if words == "" {
		return errors.New("triage needs the user's own words")
	}
	rows, err := d.query("SELECT u.graph,"+unspecifiedTriageSQL+" FROM unspecified_items u WHERE u.id="+quote(id), 2)
	if err != nil {
		return err
	}
	if len(rows) != 1 || rows[0][0] != graph {
		return fmt.Errorf("unspecified item %s not found in %s", id, graph)
	}
	if rows[0][1] != "OPEN" {
		return fmt.Errorf("unspecified item %s was already triaged as %s", id, rows[0][1])
	}
	return d.transition([]string{fmt.Sprintf("INSERT INTO unspecified_triage(graph,item,decision,words) SELECT %s,%s,%s,%s WHERE NOT EXISTS (SELECT 1 FROM unspecified_triage WHERE item=%s);", quote(graph), quote(id), quote(decision), quote(words), quote(id))})
}

func (d db) triagedUnspecified(graph, decision string) ([]UnspecifiedItem, error) {
	items, err := d.unspecified(graph)
	if err != nil {
		return nil, err
	}
	result := []UnspecifiedItem{}
	for _, item := range items {
		if item.Triage == decision {
			result = append(result, item)
		}
	}
	return result, nil
}
