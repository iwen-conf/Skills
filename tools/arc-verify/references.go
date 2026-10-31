package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const referenceRiskCap = 50

var referenceTrust = map[string]int{"project_document": 0, "public_spec": 1, "user_design_document": 2, "project_code": 3}
var referenceSlotKinds = map[string]bool{"action": true, "condition": true, "state": true, "capability": true, "invariant": true, "feature": true}

// ReferenceCatalog is a project-local JSONC file of peer invariants. An entry
// can only ever produce a RISK candidate; it never becomes a RULE.
type ReferenceCatalog struct {
	Catalog string           `json:"catalog"`
	Entries []ReferenceEntry `json:"entries"`
}

type ReferenceEntry struct {
	ID        string              `json:"id"`
	Domain    string              `json:"domain"`
	Invariant string              `json:"invariant"`
	Slots     []ReferenceSlot     `json:"slots"`
	Relations []ReferenceRelation `json:"relations,omitempty"`
	Trust     string              `json:"trust"`
	Source    string              `json:"source"`
	SourceRef string              `json:"source_ref"`
}

type ReferenceSlot struct {
	Name  string   `json:"name"`
	Kind  string   `json:"kind"`
	Hints []string `json:"hints,omitempty"`
}

type ReferenceRelation struct {
	Kind string `json:"kind"`
	From string `json:"from"`
	To   string `json:"to"`
}

// ReferenceBinding lets the model resolve a reference entry's slots explicitly,
// mark it not applicable, or name what already covers it.
type ReferenceBinding struct {
	Entry         string            `json:"entry"`
	Bindings      map[string]string `json:"bindings,omitempty"`
	NotApplicable bool              `json:"not_applicable,omitempty"`
	CoveredBy     string            `json:"covered_by,omitempty"`
	Reason        string            `json:"reason,omitempty"`
	Source        string            `json:"source"`
	SourceRef     string            `json:"source_ref"`
}

// ReferenceMatch is one entry's outcome on one graph.
type ReferenceMatch struct {
	Entry         string              `json:"entry"`
	CatalogSHA256 string              `json:"catalog_sha256"`
	Outcome       string              `json:"outcome"`
	Bindings      map[string]string   `json:"bindings,omitempty"`
	Candidates    []map[string]string `json:"candidates,omitempty"`
	Slot          string              `json:"slot,omitempty"`
	SuppressedBy  string              `json:"suppressed_by,omitempty"`
	Risk          *RiskEntry          `json:"risk,omitempty"`
}

// ReferenceMatchReport is one bounded matching run.
type ReferenceMatchReport struct {
	Graph     string           `json:"graph"`
	Matches   []ReferenceMatch `json:"matches"`
	Truncated bool             `json:"truncated"`
}

func validateReferenceCatalog(catalog ReferenceCatalog) error {
	if strings.TrimSpace(catalog.Catalog) == "" || len(catalog.Entries) == 0 || len(catalog.Entries) > 2000 {
		return errors.New("reference catalog needs a name and 1..2000 entries")
	}
	ids := map[string]bool{}
	for _, entry := range catalog.Entries {
		if entry.ID == "" || ids[entry.ID] || strings.ContainsAny(entry.ID, "@ \t\n") {
			return fmt.Errorf("reference entry %q needs a unique id without '@' or spaces", entry.ID)
		}
		ids[entry.ID] = true
		if _, known := referenceTrust[entry.Trust]; !known {
			return fmt.Errorf("reference entry %s trust must be project_document, public_spec, user_design_document or project_code", entry.ID)
		}
		if strings.TrimSpace(entry.Source) == "" || strings.TrimSpace(entry.SourceRef) == "" || strings.TrimSpace(entry.Invariant) == "" {
			return fmt.Errorf("reference entry %s needs an invariant, a source and a source_ref", entry.ID)
		}
		if len(entry.Slots) == 0 || len(entry.Slots) > 8 {
			return fmt.Errorf("reference entry %s needs 1..8 slots", entry.ID)
		}
		slots := map[string]bool{}
		for _, slot := range entry.Slots {
			if slot.Name == "" || slots[slot.Name] || !referenceSlotKinds[slot.Kind] {
				return fmt.Errorf("reference entry %s slot %q needs a unique name and a model node kind", entry.ID, slot.Name)
			}
			for _, hint := range slot.Hints {
				if hint == "" || hint != strings.ToLower(hint) {
					return fmt.Errorf("reference entry %s slot %s hints must be non-empty lowercase text", entry.ID, slot.Name)
				}
			}
			slots[slot.Name] = true
		}
		for _, relation := range entry.Relations {
			if !slots[relation.From] || !slots[relation.To] || relation.Kind == "" {
				return fmt.Errorf("reference entry %s relation must join two declared slots", entry.ID)
			}
		}
	}
	return nil
}

func validateReferenceBindings(bindings []ReferenceBinding, kinds map[string]string) error {
	seen := map[string]bool{}
	for _, binding := range bindings {
		if binding.Entry == "" || seen[binding.Entry] || !explicitSource(binding.Source, binding.SourceRef) {
			return fmt.Errorf("reference_bindings %q needs a unique entry and a user or document source", binding.Entry)
		}
		seen[binding.Entry] = true
		if binding.NotApplicable && strings.TrimSpace(binding.Reason) == "" {
			return fmt.Errorf("reference_bindings %s not_applicable needs a reason", binding.Entry)
		}
		if !binding.NotApplicable && len(binding.Bindings) == 0 && binding.CoveredBy == "" {
			return fmt.Errorf("reference_bindings %s needs bindings, covered_by or not_applicable", binding.Entry)
		}
		for slot, node := range binding.Bindings {
			if slot == "" || kinds[node] == "" {
				return fmt.Errorf("reference_bindings %s binds slot %q to unknown node %q", binding.Entry, slot, node)
			}
		}
		if binding.CoveredBy != "" {
			switch kinds[binding.CoveredBy] {
			case "rule", "test", "invariant":
			default:
				return fmt.Errorf("reference_bindings %s covered_by must name a rule, test or invariant", binding.Entry)
			}
		}
	}
	return nil
}

// importReferences stores one validated catalog file by its SHA-256.
func (d db) importReferences(path string) (map[string]any, error) {
	_, raw, err := loadEvidence(path)
	if err != nil {
		return nil, fmt.Errorf("reference catalog: %w", err)
	}
	clean, err := cleanJSONC(string(raw))
	if err != nil {
		return nil, err
	}
	var catalog ReferenceCatalog
	if err := decodeStrictJSON([]byte(clean), &catalog); err != nil {
		return nil, fmt.Errorf("reference catalog: %w", err)
	}
	if err := validateReferenceCatalog(catalog); err != nil {
		return nil, err
	}
	sum := digestBytes(raw)
	if err := d.tx([]string{fmt.Sprintf("INSERT OR IGNORE INTO reference_catalogs(sha256,name,body) VALUES(%s,%s,%s);", quote(sum), quote(catalog.Catalog), quote(clean))}); err != nil {
		return nil, err
	}
	return map[string]any{"catalog": catalog.Catalog, "sha256": sum, "entries": len(catalog.Entries)}, nil
}

// matchReferences binds every stored entry's slots on one graph. Exactly one
// binding yields a peer_reference RISK candidate; none names the slot that
// failed; several give one AMBIGUOUS diagnostic. Results never touch rules or
// the acceptance gate.
func (d db) matchReferences(graph string) (ReferenceMatchReport, error) {
	report := ReferenceMatchReport{Graph: graph, Matches: []ReferenceMatch{}}
	g, err := d.model(graph)
	if err != nil {
		return report, err
	}
	rows, err := d.query("SELECT sha256,body FROM reference_catalogs ORDER BY sha256", 2)
	if err != nil {
		return report, err
	}
	if len(rows) == 0 {
		return report, errors.New("no reference catalog imported; run reference-import first")
	}
	type stored struct {
		entry ReferenceEntry
		sha   string
	}
	entries := []stored{}
	for _, row := range rows {
		var catalog ReferenceCatalog
		if err := decodeStrictJSON([]byte(row[1]), &catalog); err != nil {
			return report, fmt.Errorf("stored catalog %s: %w", row[0], err)
		}
		for _, entry := range catalog.Entries {
			entries = append(entries, stored{entry, row[0]})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		left, right := referenceTrust[entries[i].entry.Trust], referenceTrust[entries[j].entry.Trust]
		if left != right {
			return left < right
		}
		return entries[i].entry.ID < entries[j].entry.ID
	})
	overrides := map[string]ReferenceBinding{}
	for _, binding := range g.ReferenceBindings {
		overrides[binding.Entry] = binding
	}
	edges := map[string]bool{}
	for _, edge := range g.Edges {
		edges[edge.Kind+"|"+edge.From+"|"+edge.To] = true
	}
	rejections, err := d.referenceRejections(graph)
	if err != nil {
		return report, err
	}
	risks := 0
	statements := []string{}
	for _, item := range entries {
		match := ReferenceMatch{Entry: item.entry.ID, CatalogSHA256: item.sha}
		override, overridden := overrides[item.entry.ID]
		switch {
		case overridden && override.NotApplicable:
			match.Outcome, match.SuppressedBy = "SUPPRESSED", "not applicable: "+override.Reason+" ("+override.SourceRef+")"
		default:
			bindings, slot, truncated := referenceBindings(g, item.entry, override.Bindings, edges)
			report.Truncated = report.Truncated || truncated
			switch {
			case len(bindings) == 0:
				match.Outcome, match.Slot = "NO_BINDING", slot
			case len(bindings) > 1:
				match.Outcome, match.Candidates = "AMBIGUOUS", bindings
			case rejections[item.entry.ID+"|"+bindingDetail(bindings[0])] != "":
				match.Outcome, match.Bindings, match.SuppressedBy = "SUPPRESSED", bindings[0], "rejected by the user: "+rejections[item.entry.ID+"|"+bindingDetail(bindings[0])]
			case overridden && override.CoveredBy != "":
				match.Outcome, match.Bindings, match.SuppressedBy = "SUPPRESSED", bindings[0], "covered by "+override.CoveredBy+" ("+override.SourceRef+")"
			case risks >= referenceRiskCap:
				match.Outcome, match.Bindings = "TRUNCATED", bindings[0]
				report.Truncated = true
			default:
				risks++
				match.Outcome, match.Bindings = "RISK", bindings[0]
				names := make([]string, 0, len(bindings[0]))
				for name, node := range bindings[0] {
					names = append(names, name+"="+node)
				}
				sort.Strings(names)
				action := ""
				for _, slot := range item.entry.Slots {
					if slot.Kind == "action" {
						action = bindings[0][slot.Name]
						break
					}
				}
				match.Risk = &RiskEntry{ID: "ref/" + item.entry.ID, Description: item.entry.Invariant + " [" + strings.Join(names, ", ") + "] · " + item.entry.Trust + " · " + item.entry.Source + " " + item.entry.SourceRef,
					Action: action, Origin: "peer_reference", Reference: item.entry.ID + "@" + item.sha[:12], Status: "OPEN", Capabilities: actionCapabilities(g, action), History: []RiskDecision{}}
			}
		}
		statements = append(statements, fmt.Sprintf("INSERT INTO reference_matches(graph,entry,catalog_sha256,outcome,detail) VALUES(%s,%s,%s,%s,%s);", quote(graph), quote(item.entry.ID), quote(item.sha), quote(match.Outcome), quote(referenceDetail(match))))
		report.Matches = append(report.Matches, match)
	}
	statements = append([]string{fmt.Sprintf("DELETE FROM reference_current WHERE graph=%s;", quote(graph))}, statements...)
	for _, match := range report.Matches {
		if match.Risk != nil {
			statements = append(statements, fmt.Sprintf("INSERT INTO reference_current(graph,risk,entry,reference,action,description) VALUES(%s,%s,%s,%s,%s,%s);", quote(graph), quote(match.Risk.ID), quote(match.Entry), quote(match.Risk.Reference), quote(match.Risk.Action), quote(match.Risk.Description)))
		}
	}
	return report, d.tx(statements)
}

func bindingDetail(bindings map[string]string) string {
	names := []string{}
	for name, node := range bindings {
		names = append(names, name+"="+node)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

func referenceDetail(match ReferenceMatch) string {
	switch {
	case match.Slot != "":
		return "slot " + match.Slot
	case match.SuppressedBy != "":
		return match.SuppressedBy
	case len(match.Candidates) > 0:
		return fmt.Sprintf("%d candidate bindings", len(match.Candidates))
	}
	return bindingDetail(match.Bindings)
}

// referenceBindings searches slot assignments with bounded backtracking.
func referenceBindings(g Graph, entry ReferenceEntry, explicit map[string]string, edges map[string]bool) ([]map[string]string, string, bool) {
	candidates := map[string][]string{}
	truncated := false
	for _, slot := range entry.Slots {
		options := []string{}
		for _, node := range g.Nodes {
			if node.Kind != slot.Kind {
				continue
			}
			if bound, found := explicit[slot.Name]; found {
				if node.ID == bound {
					options = append(options, node.ID)
				}
				continue
			}
			text := strings.ToLower(node.ID + " " + node.Name)
			matched := len(slot.Hints) == 0
			for _, hint := range slot.Hints {
				matched = matched || strings.Contains(text, hint)
			}
			if matched {
				options = append(options, node.ID)
			}
		}
		sort.Strings(options)
		if len(options) > 64 {
			options, truncated = options[:64], true
		}
		if len(options) == 0 {
			return nil, slot.Name, truncated
		}
		candidates[slot.Name] = options
	}
	result := []map[string]string{}
	current := map[string]string{}
	var walk func(int)
	walk = func(position int) {
		if len(result) > 16 {
			truncated = true
			return
		}
		if position == len(entry.Slots) {
			for _, relation := range entry.Relations {
				if !edges[relation.Kind+"|"+current[relation.From]+"|"+current[relation.To]] {
					return
				}
			}
			copy := map[string]string{}
			for name, node := range current {
				copy[name] = node
			}
			result = append(result, copy)
			return
		}
		slot := entry.Slots[position]
		for _, node := range candidates[slot.Name] {
			current[slot.Name] = node
			walk(position + 1)
		}
		delete(current, slot.Name)
	}
	walk(0)
	if len(result) == 0 && len(entry.Relations) > 0 {
		return nil, "relations", truncated
	}
	return result, "", truncated
}

// referenceRisks lists the current peer_reference candidates of a graph.
func (d db) referenceRisks(graph string) ([]RiskEntry, error) {
	rows, err := d.query("SELECT risk,reference,action,description FROM reference_current WHERE graph="+quote(graph)+" ORDER BY risk", 4)
	if err != nil {
		return nil, err
	}
	g, err := d.model(graph)
	if err != nil {
		return nil, err
	}
	result := []RiskEntry{}
	for _, row := range rows {
		result = append(result, RiskEntry{ID: row[0], Description: row[3], Action: row[2], Origin: "peer_reference", Reference: row[1], Status: "OPEN", Capabilities: actionCapabilities(g, row[2]), History: []RiskDecision{}})
	}
	return result, nil
}
