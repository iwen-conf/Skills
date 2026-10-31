package main

import (
	"testing"
)

func TestPerformExploreKeepsBothConcurrentResults(t *testing.T) {
	target, server := newSeatServer(t)
	target.raceNoLock = true
	target.raceGate = newOccupancyBarrier(2)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	binding, err := fixture.d.exploreBinding("seat-v2", "independent", fixture.commit, fixture.account, fixture.fixture, fixture.root, fixture.config, "seat-store", "explore/concurrent")
	if err != nil {
		t.Fatal(err)
	}
	adapter := seatExploreAdapter(t, seatIsolationScope)
	accounts, _, _, err := fixture.d.exploreAccounts(adapter, binding, fixture.account, fixture.fixture)
	if err != nil {
		t.Fatal(err)
	}
	_, calls, callErr := fixture.d.performExplore(t.Context(), binding, adapter, accounts, CandidateAction{
		Kind: kindConcurrent, Action: "claim", Entry: "/claim", Role: "STUDENT", Account: "fixture.student",
		PartnerRole: "CONTENDER", RequestID: "pair-keep",
	})
	if callErr != nil && !isTimeout(callErr) {
		t.Fatal(callErr)
	}
	if len(calls) != 2 {
		t.Fatalf("both concurrent calls must be kept: %+v", calls)
	}
	if calls[0].RequestID == calls[1].RequestID {
		t.Fatalf("partners must keep distinct request ids: %+v", calls)
	}
	if calls[0].Started == "" || calls[1].Started == "" || calls[0].Completed == "" || calls[1].Completed == "" {
		t.Fatalf("each partner needs start and end times: %+v", calls)
	}
	if calls[0].Role == calls[1].Role {
		t.Fatalf("partners must keep distinct subjects: %+v", calls)
	}
}
