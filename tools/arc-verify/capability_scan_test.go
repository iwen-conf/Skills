package main

import "testing"

// An endpoint/action is only a clue: business capability comes from a
// declared Graph "implements" relationship, not from a URL or action name.
func TestCapabilityScanMapsOnlyDeclaredBusinessCapability(t *testing.T) {
	g := Graph{
		ID: "business-v2", SchemaVersion: 2,
		Nodes: []Node{{ID: "cap.order", Kind: "capability"}, {ID: "claim", Kind: "action"}},
		Edges: []Edge{{ID: "implements-order", Kind: "implements", From: "cap.order", To: "claim"}},
	}
	adapter := ExploreAdapter{IsolationScope: "isolated", Entries: []ExploreEntry{{Action: "claim", Entry: "/claim", Method: "POST"}}}
	entries := []DiscoveredEntry{
		{Class: entryConfirmed, Action: "claim", Path: "/claim", File: "api.go"},
		{Class: entryUndeclared, Path: "/metrics", File: "infra.go"},
	}
	facts := buildEvidenceContext(g, nil, ExploreAdapter{}, ExploreReport{}, adapter, entries).Facts
	seenAdapter, seenCode, seenUnmapped := false, false, false
	for _, fact := range facts {
		switch {
		case fact.SourceType == "entry" && fact.Location == "/claim":
			seenAdapter = true
			if fact.Capability != "cap.order" {
				t.Fatalf("adapter capability = %q, want cap.order", fact.Capability)
			}
		case fact.SourceType == "source" && fact.Statement == "CONFIRMED_ENTRY /claim from api.go":
			seenCode = true
			if fact.Capability != "cap.order" {
				t.Fatalf("source capability = %q, want cap.order", fact.Capability)
			}
		case fact.SourceType == "source" && fact.Statement == "UNDECLARED_ENTRY /metrics from infra.go":
			seenUnmapped = true
			if fact.Capability != "" {
				t.Fatalf("unmapped infrastructure entry became capability %q", fact.Capability)
			}
		}
	}
	if !seenAdapter || !seenCode || !seenUnmapped {
		t.Fatalf("missing scan facts: adapter=%v source=%v unmapped=%v", seenAdapter, seenCode, seenUnmapped)
	}
}

func TestCapabilityScanDoesNotGuessAmbiguousOwnership(t *testing.T) {
	g := Graph{Nodes: []Node{{ID: "cap.a", Kind: "capability"}, {ID: "cap.b", Kind: "capability"}, {ID: "action", Kind: "action"}},
		Edges: []Edge{{ID: "ea", Kind: "implements", From: "cap.a", To: "action"}, {ID: "eb", Kind: "implements", From: "cap.b", To: "action"}}}
	if got := confirmedCapabilityForAction(indexModel(g), "action"); got != "" {
		t.Fatalf("ambiguous action incorrectly assigned to %s", got)
	}
}
