package main

import "testing"

func TestDiscoveryRefreshDoesNotInventBusinessConclusions(t *testing.T) {
	g := referenceModel(t)
	adapter := seatExploreAdapter(t, seatIsolationScope)
	report := ExploreReport{
		Steps: []ExploreStep{
			{Candidate: CandidateAction{Action:"claim",Role:"STUDENT"}, Calls: []ExploreCallRecord{{StatusCode:200}}},
			{Candidate: CandidateAction{Action:"claim",Role:"CONTENDER"}, Calls: []ExploreCallRecord{{StatusCode:200}}},
		},
	}
	next := refreshBoundaryDiscovery(DiscoveryContext{}, g, map[string]bool{}, adapter, report, nil)
	if len(next.Hypotheses)!=0 || len(next.Explanations)!=0 || len(next.Experiments)!=0 || len(next.Revisions)!=0 {
		t.Fatalf("unconfirmed business conclusions manufactured from HTTP: %+v",next)
	}
}

func TestDiscoveryRefreshPreservesRecordedHistory(t *testing.T) {
	g := referenceModel(t)
	prev := DiscoveryContext{
		Hypotheses: []BoundaryHypothesis{{ID:"recorded", Assumption:"unconfirmed",Status:hypProposed}},
		Revisions: []UnderstandingRevision{{Version:1, Believed:"recorded"}},
	}
	next:=refreshBoundaryDiscovery(prev,g,map[string]bool{}, ExploreAdapter{},ExploreReport{},nil)
	if len(next.Hypotheses)!=1 || next.Hypotheses[0].ID!="recorded" || len(next.Revisions)!=1 {
		t.Fatalf("existing evidence/history was lost: %+v",next)
	}
}
