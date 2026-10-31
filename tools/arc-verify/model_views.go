package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type BusinessView string

const (
	viewOverview     BusinessView = "overview"
	viewCapabilities BusinessView = "capabilities"
	viewBoundaries   BusinessView = "boundaries"
	viewFlows        BusinessView = "flows"
	viewVerification BusinessView = "verification"
	viewImpact       BusinessView = "impact"
)

type ViewQuery struct {
	Graph       string       `json:"graph"`
	View        BusinessView `json:"view"`
	Root        string       `json:"root,omitempty"`
	Environment string       `json:"environment,omitempty"`
	Round       string       `json:"round,omitempty"`
	Role        string       `json:"role,omitempty"`
	Status      string       `json:"status,omitempty"`
	Readiness   string       `json:"readiness,omitempty"`
	Offset      int          `json:"offset"`
	Limit       int          `json:"limit"`
}

type ProjectedNode struct {
	Node
	BusinessNodeStatus
	Parent      string `json:"parent,omitempty"`
	Domain      string `json:"domain,omitempty"`
	Description string `json:"description,omitempty"`
}

type BusinessProjection struct {
	Graph           string               `json:"graph"`
	View            BusinessView         `json:"view"`
	Root            string               `json:"root,omitempty"`
	Environment     string               `json:"environment,omitempty"`
	Round           string               `json:"round,omitempty"`
	Nodes           []ProjectedNode      `json:"nodes"`
	Edges           []Edge               `json:"edges"`
	Coverage        BusinessCoverage     `json:"coverage"`
	Obligations     []ObligationStatus   `json:"obligations"`
	Diagnostics     []BusinessDiagnostic `json:"diagnostics"`
	Verdict         string               `json:"verdict"`
	Reason          string               `json:"reason"`
	Context         ProofContext         `json:"context"`
	Total           int                  `json:"total"`
	Offset          int                  `json:"offset"`
	NextOffset      int                  `json:"next_offset"`
	Truncated       bool                 `json:"truncated"`
	HistoryFailures int                  `json:"history_failures"`
}

func validView(view BusinessView) bool {
	switch view {
	case viewOverview, viewCapabilities, viewBoundaries, viewFlows, viewVerification, viewImpact:
		return true
	default:
		return false
	}
}

func projection(g Graph, analysis ModelAnalysis, query ViewQuery) (BusinessProjection, error) {
	result := BusinessProjection{Graph: g.ID, View: query.View, Root: query.Root, Environment: analysis.Environment, Round: analysis.Round, Nodes: []ProjectedNode{}, Edges: []Edge{}, Coverage: analysis.Coverage, Obligations: analysis.Obligations, Diagnostics: analysis.Diagnostics, Verdict: analysis.Verdict, Reason: analysis.Reason, Context: analysis.Context, Offset: query.Offset, HistoryFailures: analysis.HistoryFailures}
	if !validView(query.View) || query.Offset < 0 || query.Limit < 1 || query.Limit > 1024 {
		return result, errors.New("view or pagination is outside the supported bounds")
	}
	x := indexModel(g)
	all := map[string]Node{}
	relations := append([]Edge{}, g.Edges...)
	for _, node := range g.Nodes {
		all[node.ID] = node
	}
	for _, rule := range g.Rules {
		all[rule.ID] = Node{ID: rule.ID, Kind: "rule", Name: rule.Description}
	}
	for _, invariant := range g.Invariants {
		all[invariant.ID] = Node{ID: invariant.ID, Kind: "invariant", Name: invariant.Name}
	}
	for _, obligation := range analysis.Obligations {
		definition := obligation.Definition
		all[definition.ID] = Node{ID: definition.ID, Kind: "test", Name: definition.Name}
		for _, target := range obligation.Targets {
			if x.edges[target].Kind == "transitions_to" {
				edge := x.edges[target]
				all[target] = Node{ID: target, Kind: "transition", Name: all[edge.From].Name + " → " + all[edge.To].Name}
			}
			relations = append(relations, Edge{ID: "projection/" + definition.ID + "/verifies/" + target, From: definition.ID, To: target, Kind: "verifies"})
		}
		for _, dependency := range definition.DependsOn {
			relations = append(relations, Edge{ID: "projection/" + definition.ID + "/requires/" + dependency, From: definition.ID, To: dependency, Kind: "test_depends_on"})
		}
	}
	if query.Root != "" && all[query.Root].ID == "" {
		return result, errors.New("projection root does not exist in this model")
	}
	if query.Role != "" && all[query.Role].Kind != "role" {
		return result, errors.New("projection role does not exist")
	}
	include := func(kind string) bool {
		switch query.View {
		case viewOverview:
			return kind == "system" || kind == "domain" || kind == "role"
		case viewCapabilities:
			return kind == "system" || kind == "domain" || kind == "capability" || kind == "role"
		case viewBoundaries:
			return kind == "boundary" || kind == "capability" || kind == "rule" || kind == "invariant" || kind == "data" || kind == "contract" || kind == "deployment"
		case viewFlows:
			return kind == "capability" || kind == "action" || kind == "condition" || kind == "state" || kind == "rule"
		case viewVerification:
			return kind == "capability" || kind == "action" || kind == "condition" || kind == "state" || kind == "rule" || kind == "test" || kind == "invariant" || kind == "transition"
		case viewImpact:
			return kind == "capability" || kind == "boundary" || kind == "action" || kind == "condition" || kind == "rule" || kind == "contract" || kind == "data" || kind == "invariant" || kind == "test"
		default:
			return false
		}
	}
	allowEdge := func(kind string) bool {
		switch query.View {
		case viewOverview:
			return kind == "contains" || kind == "serves" || kind == "collaborates_with"
		case viewCapabilities:
			return kind == "contains" || kind == "serves"
		case viewBoundaries:
			return kind == "owns" || kind == "depends_on" || kind == "uses_contract" || kind == "collaborates_with" || kind == "deployed_as"
		case viewFlows:
			return kind == "implements" || kind == "requires" || kind == "forbids" || kind == "transitions" || kind == "transitions_to" || kind == "recovers" || kind == "next"
		case viewVerification:
			return kind == "verifies" || kind == "test_depends_on" || kind == "implements"
		case viewImpact:
			return kind == "depends_on" || kind == "requires" || kind == "implements" || kind == "owns" || kind == "uses_contract" || kind == "verifies"
		default:
			return false
		}
	}
	focus := map[string]bool{}
	if query.Root != "" {
		focus[query.Root] = true
		if all[query.Root].Kind == "boundary" {
			for _, edge := range g.Edges {
				if edge.From == query.Root && (edge.Kind == "owns" || edge.Kind == "uses_contract" || edge.Kind == "deployed_as") {
					focus[edge.To] = true
				}
			}
		}
		for id := range all {
			for parent := id; parent != ""; parent = x.parents[parent] {
				if focus[parent] {
					focus[id] = true
					break
				}
			}
		}
		for _, edge := range g.Edges {
			if edge.Kind == "implements" && focus[edge.From] {
				focus[edge.To] = true
			}
		}
		for _, edge := range g.Edges {
			if (edge.Kind == "requires" || edge.Kind == "forbids") && focus[edge.To] {
				focus[edge.From] = true
			}
			if (edge.Kind == "transitions" || edge.Kind == "recovers") && focus[edge.From] {
				focus[edge.To] = true
			}
		}
		for _, obligation := range analysis.Obligations {
			matches := false
			for _, target := range obligation.Targets {
				if focus[target] {
					matches = true
					break
				}
			}
			if matches {
				focus[obligation.Definition.ID] = true
				for _, target := range obligation.Targets {
					focus[target] = true
				}
			}
		}
		for _, edge := range g.Edges {
			if edge.Kind == "owns" && focus[edge.To] {
				focus[edge.From] = true
			}
			if edge.Kind == "serves" && focus[edge.To] {
				focus[edge.From] = true
			}
		}
	}
	roleScope := map[string]bool{}
	if query.Role != "" {
		for _, edge := range g.Edges {
			if edge.Kind == "serves" && edge.From == query.Role {
				roleScope[edge.To] = true
			}
		}
		for id := range all {
			for parent := id; parent != ""; parent = x.parents[parent] {
				if roleScope[parent] {
					roleScope[id] = true
					break
				}
			}
		}
		for _, rule := range g.Rules {
			if rule.Actor == query.Role {
				roleScope[rule.ID], roleScope[rule.Action] = true, true
				for _, cap := range x.capsByAction[rule.Action] {
					roleScope[cap] = true
				}
			}
		}
		for _, obligation := range analysis.Obligations {
			for _, target := range obligation.Targets {
				if roleScope[target] {
					roleScope[obligation.Definition.ID] = true
					break
				}
			}
		}
		roleScope[query.Role] = true
	}
	metadata := map[string]NodeContract{}
	for _, contract := range g.Contracts {
		metadata[contract.Subject] = contract
	}
	ids := []string{}
	for id, node := range all {
		if !include(node.Kind) || query.Root != "" && !focus[id] || query.Role != "" && !roleScope[id] {
			continue
		}
		status := analysis.Nodes[id]
		if status.Status == "" {
			status = BusinessNodeStatus{Status: "NOT_RUN", Reason: "model relation has no independent execution evidence"}
		}
		if query.Status != "" && status.Status != query.Status {
			continue
		}
		switch query.Readiness {
		case "":
		case "described":
			if !status.Readiness.Described {
				continue
			}
		case "implemented":
			if !status.Readiness.Implemented {
				continue
			}
		case "verified":
			if !status.Readiness.Verified {
				continue
			}
		case "live":
			if !status.Readiness.Live {
				continue
			}
		default:
			return result, errors.New("unknown capability readiness filter")
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result.Total = len(ids)
	if query.Offset > len(ids) {
		return result, errors.New("projection offset exceeds the filtered node count")
	}
	end := query.Offset + query.Limit
	if end > len(ids) {
		end = len(ids)
	}
	selected := map[string]bool{}
	for _, id := range ids[query.Offset:end] {
		selected[id] = true
		status := analysis.Nodes[id]
		if status.Status == "" {
			status = BusinessNodeStatus{Status: "NOT_RUN", Reason: "business relation has no execution evidence"}
		}
		domain := ""
		for parent := id; parent != ""; parent = x.parents[parent] {
			if x.kinds[parent] == "domain" {
				domain = parent
			}
		}
		result.Nodes = append(result.Nodes, ProjectedNode{Node: all[id], BusinessNodeStatus: status, Parent: x.parents[id], Domain: domain, Description: metadata[id].Description})
	}
	seen := map[string]bool{}
	for _, edge := range relations {
		if allowEdge(edge.Kind) && selected[edge.From] && selected[edge.To] {
			key := edge.Kind + "\x00" + edge.From + "\x00" + edge.To
			if !seen[key] {
				result.Edges = append(result.Edges, edge)
				seen[key] = true
			}
		}
	}
	sort.Slice(result.Edges, func(i, j int) bool { return result.Edges[i].ID < result.Edges[j].ID })
	result.Truncated = end < len(ids)
	if result.Truncated {
		result.NextOffset = end
	}
	return result, nil
}

func (d db) viewModel(query ViewQuery) (BusinessProjection, error) {
	g, err := d.model(query.Graph)
	if err != nil {
		return BusinessProjection{}, err
	}
	analysis, err := d.analyzeModel(g, query.Environment, query.Round)
	if err != nil {
		return BusinessProjection{}, err
	}
	return projection(g, analysis, query)
}

type ImpactReport struct {
	Graph    string              `json:"graph"`
	Previous string              `json:"previous,omitempty"`
	Changed  []string            `json:"changed"`
	Affected []string            `json:"affected"`
	Tests    []string            `json:"tests"`
	Reasons  map[string][]string `json:"reasons"`
	Scope    string              `json:"scope"`
}

func modelImpact(g Graph, changed []string) (ImpactReport, error) {
	result := ImpactReport{Graph: g.ID, Changed: uniqueIDs(append([]string{}, changed...)), Affected: []string{}, Tests: []string{}, Reasons: map[string][]string{}, Scope: "typed dependency propagation; containment does not imply execution order"}
	x := indexModel(g)
	propagation := map[string][]string{}
	for _, edge := range g.Edges {
		switch edge.Kind {
		case "requires", "forbids":
			propagation[edge.From] = append(propagation[edge.From], edge.To)
		case "depends_on", "uses_contract":
			propagation[edge.To] = append(propagation[edge.To], edge.From)
		case "owns":
			propagation[edge.To] = append(propagation[edge.To], edge.From)
		case "implements":
			propagation[edge.To] = append(propagation[edge.To], edge.From)
		}
	}
	for _, rule := range g.Rules {
		propagation[rule.ID] = append(propagation[rule.ID], rule.Action)
		for _, condition := range rule.Conditions {
			propagation[condition] = append(propagation[condition], rule.ID)
		}
	}
	for _, flow := range g.Flows {
		propagation[flow.ID] = append(propagation[flow.ID], flow.Capability)
	}
	for _, invariant := range g.Invariants {
		for _, scope := range invariant.Scope {
			propagation[invariant.ID] = append(propagation[invariant.ID], scope)
			propagation[scope] = append(propagation[scope], invariant.ID)
		}
	}
	for _, obligation := range modelObligations(g) {
		for _, target := range obligation.Targets {
			propagation[target] = append(propagation[target], obligation.Definition.ID)
		}
		for _, dependency := range obligation.Definition.DependsOn {
			propagation[dependency] = append(propagation[dependency], obligation.Definition.ID)
		}
		x.kinds[obligation.Definition.ID] = "test"
	}
	visited := map[string]bool{}
	queue := append([]string{}, result.Changed...)
	for _, id := range queue {
		if x.kinds[id] == "" && x.edges[id].ID == "" {
			return result, fmt.Errorf("changed business subject %s is not in this version", id)
		}
		visited[id] = true
	}
	for position := 0; position < len(queue); position++ {
		id := queue[position]
		for _, dependent := range propagation[id] {
			result.Reasons[dependent] = append(result.Reasons[dependent], id)
			if !visited[dependent] {
				visited[dependent] = true
				queue = append(queue, dependent)
			}
		}
	}
	for id := range visited {
		result.Affected = append(result.Affected, id)
		if x.kinds[id] == "test" {
			result.Tests = append(result.Tests, id)
		}
	}
	sort.Strings(result.Affected)
	sort.Strings(result.Tests)
	for id, reasons := range result.Reasons {
		result.Reasons[id] = uniqueIDs(reasons)
	}
	return result, nil
}

func (d db) diffModel(graph, previous string, changed []string) (ImpactReport, error) {
	g, err := d.model(graph)
	if err != nil {
		return ImpactReport{}, err
	}
	if previous != "" {
		before, err := d.model(previous)
		if err != nil {
			return ImpactReport{}, err
		}
		if before.System != g.System {
			return ImpactReport{}, errors.New("model versions belong to different systems")
		}
		old, err := semanticSubjects(before)
		if err != nil {
			return ImpactReport{}, err
		}
		current, err := semanticSubjects(g)
		if err != nil {
			return ImpactReport{}, err
		}
		for id, body := range current {
			if old[id] != body {
				changed = append(changed, id)
			}
		}
		for id := range old {
			if _, found := current[id]; !found {
				changed = append(changed, id)
			}
		}
		changed = uniqueIDs(changed)
		present := []string{}
		deleted := []string{}
		for _, id := range changed {
			if _, found := current[id]; found {
				present = append(present, id)
			} else {
				deleted = append(deleted, id)
			}
		}
		result, err := modelImpact(g, present)
		if err != nil {
			return result, err
		}
		result.Previous = previous
		result.Changed = changed
		if len(deleted) > 0 {
			prior, err := modelImpact(before, deleted)
			if err != nil {
				return result, err
			}
			result.Affected = uniqueIDs(append(result.Affected, prior.Affected...))
			result.Tests = uniqueIDs(append(result.Tests, prior.Tests...))
		}
		return result, nil
	}
	if len(changed) == 0 {
		return ImpactReport{}, errors.New("diff requires a previous graph or changed subject IDs")
	}
	return modelImpact(g, changed)
}

func splitIDs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}
