package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

var errGraphNotFound = errors.New("graph not found")

func (d db) model(graph string) (Graph, error) {
	extended, err := d.extendedModel(graph)
	if err != nil {
		return Graph{}, err
	}
	if extended != nil {
		return *extended, nil
	}
	g, err := d.ledgerModel(graph)
	if err != nil {
		return g, err
	}
	return g, validateGraph(g)
}

// Legacy hot paths only probe the extension table; they do not reload the graph.
func (d db) extendedModel(graph string) (*Graph, error) {
	models, err := d.query("SELECT schema_version,body FROM business_models WHERE graph="+quote(graph), 2)
	if err != nil {
		return nil, fmt.Errorf("model schema unavailable; run arc-verify init: %w", err)
	}
	if len(models) == 0 {
		return nil, nil
	}
	if len(models) != 1 || models[0][0] != "2" || len(models[0][1]) > 4<<20 {
		return nil, errors.New("unsupported or oversized stored model")
	}
	var g Graph
	if err := decodeStrictJSON([]byte(models[0][1]), &g); err != nil {
		return nil, err
	}
	if g.ID != graph || g.SchemaVersion != 2 {
		return nil, errors.New("model identity does not match its ledger")
	}
	core, err := d.ledgerModel(graph)
	if err != nil {
		return nil, err
	}
	want, err := canonicalModelCore(g)
	if err != nil {
		return nil, err
	}
	actual, err := canonicalModelCore(core)
	if err != nil {
		return nil, err
	}
	if want != actual {
		return nil, errors.New("immutable business model contradicts the core ledger")
	}
	return &g, validateGraph(g)
}

func canonicalModelCore(g Graph) (string, error) {
	core := Graph{ID: g.ID, Scope: g.Scope, Nodes: append([]Node{}, g.Nodes...), Edges: append([]Edge{}, g.Edges...), Rules: append([]Rule{}, g.Rules...), Risks: append([]Risk{}, g.Risks...)}
	sort.Slice(core.Nodes, func(i, j int) bool { return core.Nodes[i].ID < core.Nodes[j].ID })
	sort.Slice(core.Edges, func(i, j int) bool { return core.Edges[i].ID < core.Edges[j].ID })
	sort.Slice(core.Rules, func(i, j int) bool { return core.Rules[i].ID < core.Rules[j].ID })
	sort.Slice(core.Risks, func(i, j int) bool { return core.Risks[i].ID < core.Risks[j].ID })
	body, err := json.Marshal(core)
	return string(body), err
}

func (d db) ledgerModel(graph string) (Graph, error) {
	rows, err := d.query("SELECT scope FROM graphs WHERE id="+quote(graph), 1)
	if err != nil {
		return Graph{}, err
	}
	if len(rows) != 1 {
		return Graph{}, errGraphNotFound
	}
	g := Graph{ID: graph, Scope: rows[0][0], Nodes: []Node{}, Edges: []Edge{}, Rules: []Rule{}, Risks: []Risk{}}
	nodes, err := d.query("SELECT id,kind,name FROM nodes WHERE graph="+quote(graph)+" ORDER BY id", 3)
	if err != nil {
		return g, err
	}
	for _, row := range nodes {
		g.Nodes = append(g.Nodes, Node{ID: row[0], Kind: row[1], Name: row[2]})
	}
	edges, err := d.query("SELECT id,source,target,kind FROM edges WHERE graph="+quote(graph)+" ORDER BY id", 4)
	if err != nil {
		return g, err
	}
	for _, row := range edges {
		g.Edges = append(g.Edges, Edge{ID: row[0], From: row[1], To: row[2], Kind: row[3]})
	}
	rules, err := d.query("SELECT id,action,actor,entry,oracle,conditions,expected,rejected,source,description FROM rules WHERE graph="+quote(graph)+" ORDER BY id", 10)
	if err != nil {
		return g, err
	}
	for _, row := range rules {
		rule := Rule{ID: row[0], Action: row[1], Actor: row[2], Entry: row[3], Oracle: row[4], Expected: row[6], Rejected: row[7], Source: row[8], Description: row[9]}
		if err := json.Unmarshal([]byte(row[5]), &rule.Conditions); err != nil {
			return g, err
		}
		g.Rules = append(g.Rules, rule)
	}
	risks, err := d.query("SELECT r.id,r.description,r.action,COALESCE(m.origin,''),COALESCE(m.reference,'') FROM risks r LEFT JOIN risk_meta m ON m.graph=r.graph AND m.id=r.id WHERE r.graph="+quote(graph)+" ORDER BY r.id", 5)
	if err != nil {
		return g, fmt.Errorf("risk schema unavailable; run arc-verify init: %w", err)
	}
	for _, row := range risks {
		g.Risks = append(g.Risks, Risk{ID: row[0], Description: row[1], Action: row[2], Origin: row[3], Reference: row[4]})
	}
	return g, nil
}
