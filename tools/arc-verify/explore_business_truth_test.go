package main

import "testing"

func TestSourceEntryIsNotVerifiedBusinessOutcome(t *testing.T) {
	g := referenceModel(t)
	adapter := seatExploreAdapter(t, seatIsolationScope)
	ctx := buildEvidenceContext(g, nil, ExploreAdapter{}, ExploreReport{}, adapter, []DiscoveredEntry{{Class: entryUndeclared, Path: "/metrics", File: "server.go"}})
	for _, fact := range ctx.Facts {
		if fact.SourceType == "entry" {
			if fact.Observed || fact.Trust == "VERIFIED" {
				t.Fatalf("adapter declaration was treated as executed evidence: %+v", fact)
			}
		}
		if fact.SourceType == "source" {
			if fact.Trust == "VERIFIED" || fact.Capability != "" {
				t.Fatalf("unmapped route became a verified business capability: %+v", fact)
			}
		}
	}
	for _, q := range ctx.Questions {
		if q == "UNKNOWN: UNDECLARED_ENTRY /metrics from server.go" {
			t.Fatal("infrastructure route must not create an automatic product decision")
		}
	}
}
