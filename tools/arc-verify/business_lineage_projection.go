package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

const (
	businessLineageMaxNodes = 65536
	businessLineageMaxEdges = 131072
)

type businessLineageBudget struct{ nodes, edges int }

type businessLineageCondition struct {
	id        string
	predicate BusinessPredicate
}

type businessLineageBuilder struct {
	graph        BusinessBranchLineage
	attributes   map[string]businessAttributeInfo
	preparations map[string]PreparationAction
	names        map[string]string
	included     map[string]bool
	budget       *businessLineageBudget
	err          error
}

func buildBusinessLineage(g Graph) (*BusinessLineage, error) {
	if g.BusinessContract == nil {
		return nil, nil
	}
	if err := ValidateBusinessContract(g); err != nil {
		return nil, fmt.Errorf("business lineage contract: %w", err)
	}
	hash, err := businessJSONHash(g)
	if err != nil {
		return nil, err
	}
	result := &BusinessLineage{Version: 1, ContractSHA256: hash, Branches: []BusinessBranchLineage{}}
	attributes, names, preparations := businessAttributeIndex(g.BusinessContract), map[string]string{}, map[string]PreparationAction{}
	for _, node := range g.Nodes {
		names[node.ID] = node.Name
	}
	for _, action := range g.BusinessContract.Preparation {
		preparations[action.ID] = action
	}
	budget := &businessLineageBudget{}
	for _, branch := range g.BusinessContract.Branches {
		prefix := "branch:" + url.QueryEscape(branch.ID)
		b := businessLineageBuilder{
			graph:      BusinessBranchLineage{Branch: branch.ID, Action: branch.Action, Flow: branch.Flow, Target: prefix + ":target", Nodes: []BusinessLineageNode{}, Edges: []BusinessLineageEdge{}},
			attributes: attributes, names: names, preparations: preparations, included: map[string]bool{}, budget: budget,
		}
		confirmed := branch.Confirmed
		b.node(BusinessLineageNode{ID: b.graph.Target, Kind: "target", Name: names[branch.Action] + " · " + branch.ID, Action: branch.Action, SourceRef: branch.SourceRef, Confirmed: &confirmed, FromStates: branch.FromStates, Predicate: &branch.When, References: b.references(businessPredicateRefs(branch.When))})
		b.node(BusinessLineageNode{ID: prefix + ":state", Kind: "state", Name: "前态 " + strings.Join(branch.FromStates, " / "), FromStates: branch.FromStates, SourceRef: branch.SourceRef})
		b.edge(prefix+":state", b.graph.Target, "requires", branch.SourceRef, false)
		conditions := b.predicate(prefix+":when", branch.When, "", branch.SourceRef, true)
		b.edge(prefix+":when", b.graph.Target, "requires", branch.SourceRef, false)
		for _, condition := range conditions {
			for _, preparation := range g.BusinessContract.Preparation {
				if b.provides(preparation, condition.predicate) {
					b.preparation(preparation.ID)
					b.edge(lineagePreparationID(preparation.ID), condition.id, "candidate_provider", preparation.SourceRef, true)
				}
			}
		}
		if b.err != nil {
			return nil, b.err
		}
		result.Branches = append(result.Branches, b.graph)
	}
	return result, nil
}

func lineagePreparationID(id string) string { return "prep:" + url.QueryEscape(id) }

func (b *businessLineageBuilder) node(node BusinessLineageNode) {
	if b.err != nil {
		return
	}
	if b.budget.nodes >= businessLineageMaxNodes {
		b.err = fmt.Errorf("%w: exceeds %d nodes; narrow the business model", errBusinessLineageBudget, businessLineageMaxNodes)
		return
	}
	node.Branch = b.graph.Branch
	b.graph.Nodes = append(b.graph.Nodes, node)
	b.budget.nodes++
}

func (b *businessLineageBuilder) edge(from, to, kind, source string, alternative bool) {
	if b.err != nil {
		return
	}
	if b.budget.edges >= businessLineageMaxEdges {
		b.err = fmt.Errorf("%w: exceeds %d edges; narrow the business model", errBusinessLineageBudget, businessLineageMaxEdges)
		return
	}
	b.graph.Edges = append(b.graph.Edges, BusinessLineageEdge{ID: from + "->" + to + ":" + kind, From: from, To: to, Kind: kind, SourceRef: source, Alternative: alternative})
	b.budget.edges++
}

func (b *businessLineageBuilder) references(keys []string) []BusinessLineageReference {
	result := []BusinessLineageReference{}
	for _, key := range uniqueSortedBusiness(keys) {
		info := b.attributes[key]
		result = append(result, BusinessLineageReference{Subject: info.subject.ID, SubjectKind: info.subject.Kind, Attribute: info.attribute.Name, SourceRef: info.attribute.SourceRef})
	}
	return result
}

func (b *businessLineageBuilder) predicate(id string, predicate BusinessPredicate, preparation, source string, providers bool) []businessLineageCondition {
	if b.err != nil {
		return nil
	}
	kind := "condition"
	if predicate.Op == "and" || predicate.Op == "or" || predicate.Op == "not" {
		kind = "logic"
	}
	b.node(BusinessLineageNode{ID: id, Kind: kind, Name: strings.ToUpper(predicate.Op), Predicate: &predicate, Preparation: preparation, References: b.references(businessPredicateRefs(predicate)), SourceRef: source})
	conditions := []businessLineageCondition{}
	if providers && predicate.Op != "and" && predicate.Op != "or" && len(businessPredicateRefs(predicate)) > 0 {
		conditions = append(conditions, businessLineageCondition{id: id, predicate: predicate})
	}
	for index, child := range predicate.Args {
		childID := id + ":" + strconv.Itoa(index)
		conditions = append(conditions, b.predicate(childID, child, preparation, source, providers && predicate.Op != "not")...)
		b.edge(childID, id, "logical_operand", source, predicate.Op == "or")
	}
	return conditions
}

func (b *businessLineageBuilder) provides(action PreparationAction, predicate BusinessPredicate) bool {
	if b.err != nil || len(businessPredicateRefs(predicate)) == 0 {
		return false
	}
	observed := BusinessObservation{Values: map[string]json.RawMessage{}, Bindings: map[string]string{}}
	// These local symbols permit a declaration proof, not a runtime identity.
	// Only Effects supply values; none of these symbols are exported as facts.
	for _, key := range append(businessPredicateRefs(predicate), businessPredicateRefs(action.Ensures)...) {
		subject := b.attributes[key].subject.ID
		observed.Bindings[subject] = "declared-subject:" + subject
	}
	for _, effect := range action.Effects {
		observed.Values[effect.Subject+"."+effect.Attribute] = effect.Value
	}
	if evaluateBusinessPredicate(action.Ensures, observed, b.attributes).Value == businessFalse {
		return false
	}
	return evaluateBusinessPredicate(predicate, observed, b.attributes).Value == businessTrue
}

func (b *businessLineageBuilder) ancestors(id string, into map[string]bool) {
	for _, parent := range b.preparations[id].DependsOn {
		if !into[parent] {
			into[parent] = true
			b.ancestors(parent, into)
		}
	}
}

func (b *businessLineageBuilder) preparation(id string) {
	if b.err != nil || b.included[id] {
		return
	}
	b.included[id] = true
	action := b.preparations[id]
	nodeID := lineagePreparationID(id)
	keys := append(businessPredicateRefs(action.Ensures), preparationWrites(action)...)
	b.node(BusinessLineageNode{ID: nodeID, Kind: "preparation", Name: b.names[action.Action] + " · " + id, Action: action.Action, Preparation: id, Predicate: &action.Ensures, Effects: action.Effects, References: b.references(keys), SourceRef: action.SourceRef})
	conditions := b.predicate(nodeID+":requires", action.Requires, id, action.SourceRef, true)
	b.edge(nodeID+":requires", nodeID, "requires", action.SourceRef, false)
	for _, parent := range action.DependsOn {
		b.preparation(parent)
		b.edge(lineagePreparationID(parent), nodeID, "depends_on", action.SourceRef, false)
	}
	ancestors := map[string]bool{}
	b.ancestors(id, ancestors)
	ids := []string{}
	for parent := range ancestors {
		ids = append(ids, parent)
	}
	sort.Strings(ids)
	for _, condition := range conditions {
		for _, parent := range ids {
			if b.provides(b.preparations[parent], condition.predicate) {
				b.edge(lineagePreparationID(parent), condition.id, "candidate_provider", b.preparations[parent].SourceRef, true)
			}
		}
	}
}
