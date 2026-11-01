package main

import (
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func seedExploreCampaign(t *testing.T, d db, campaign string) {
	t.Helper()
	if err := d.saveExploreCheckpoint(ExploreCheckpoint{Campaign: campaign, Graph: "seat-v2", Status: exploreInterrupted}); err != nil {
		t.Fatal(err)
	}
}

func TestExploreConcurrentClaimOnlyOneWinner(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-race"
	seedExploreCampaign(t, fixture.d, campaign)
	left, err := openDB(fixture.d.file, false)
	if err != nil {
		t.Fatal(err)
	}
	defer left.close()
	right, err := openDB(fixture.d.file, false)
	if err != nil {
		t.Fatal(err)
	}
	defer right.close()
	var wins int32
	var fences int64
	owners := make(chan string, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	claim := func(conn db, worker string) {
		defer workers.Done()
		<-start
		lease, err := conn.claimExploreCampaign(campaign, worker)
		if err != nil {
			return
		}
		atomic.AddInt32(&wins, 1)
		atomic.StoreInt64(&fences, lease.Fence)
		owners <- lease.Worker
	}
	workers.Add(2)
	go claim(left, "worker-left")
	go claim(right, "worker-right")
	close(start)
	workers.Wait()
	close(owners)
	if atomic.LoadInt32(&wins) != 1 {
		t.Fatalf("exactly one worker may own a campaign, winners=%d", wins)
	}
	owner := <-owners
	if owner != "worker-left" && owner != "worker-right" {
		t.Fatalf("winner must be one of the two connections, got %q", owner)
	}
	if atomic.LoadInt64(&fences) < 1 {
		t.Fatal("winner must receive a fencing token")
	}
	rows, err := fixture.d.query("SELECT claimed_by,fence FROM exploration_state WHERE campaign="+quote(campaign), 2)
	if err != nil || len(rows) != 1 || rows[0][0] != owner {
		t.Fatalf("ledger owner mismatch: %+v %v", rows, err)
	}
}

func TestExploreStaleWorkerCannotCommit(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-stale"
	seedExploreCampaign(t, fixture.d, campaign)
	first, err := fixture.d.claimExploreCampaign(campaign, "worker-old")
	if err != nil {
		t.Fatal(err)
	}
	if err := expireExploreLease(fixture.d, campaign); err != nil {
		t.Fatal(err)
	}
	second, err := fixture.d.claimExploreCampaign(campaign, "worker-new")
	if err != nil {
		t.Fatal(err)
	}
	if second.Fence <= first.Fence {
		t.Fatalf("takeover must raise the fencing token: old=%d new=%d", first.Fence, second.Fence)
	}
	if err := fixture.d.saveExploreCheckpointOwned(ExploreCheckpoint{Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, Checkpoint: "CK-old"}, first); err == nil {
		t.Fatal("stale worker must not overwrite the new owner's checkpoint")
	}
	if err := fixture.d.saveExploreCheckpointOwned(ExploreCheckpoint{Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, Checkpoint: "CK-new"}, second); err != nil {
		t.Fatal(err)
	}
	state, err := fixture.d.loadExploreCheckpoint(campaign)
	if err != nil || state.Checkpoint != "CK-new" || state.ClaimedBy != "worker-new" {
		t.Fatalf("new owner checkpoint was not kept: %+v %v", state, err)
	}
}

func TestExploreWorkerCrashAndSafeTakeover(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-crash"
	seedExploreCampaign(t, fixture.d, campaign)
	crashed, err := fixture.d.claimExploreCampaign(campaign, "worker-crashed")
	if err != nil {
		t.Fatal(err)
	}
	state := ExploreCheckpoint{
		Campaign: campaign, Graph: "seat-v2", Status: exploreRunning, ClaimedBy: crashed.Worker, Fence: crashed.Fence,
		Actions: []ExploreActionRecord{{
			Candidate:     CandidateAction{Action: "claim", Kind: kindDeclared, RequestID: "in-flight"},
			Status:        actionExecuting,
			SideEffecting: true,
			Before:        IndependentFacts{State: "idle"},
		}},
	}
	if err := fixture.d.saveExploreCheckpointOwned(state, crashed); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.d.claimExploreCampaign(campaign, "worker-takeover"); err == nil {
		t.Fatal("a live lease must block takeover")
	}
	if err := expireExploreLease(fixture.d, campaign); err != nil {
		t.Fatal(err)
	}
	loaded, err := fixture.d.loadExploreCheckpoint(campaign)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Actions[0].Status != actionExecuting && loaded.Actions[0].Status != actionUnknown {
		t.Fatalf("crash must leave the in-flight action visible: %+v", loaded.Actions)
	}
	takeover, err := fixture.d.claimExploreCampaign(campaign, "worker-takeover")
	if err != nil {
		t.Fatal(err)
	}
	if takeover.Fence <= crashed.Fence {
		t.Fatalf("safe takeover must mint a newer fence: %+v vs %+v", takeover, crashed)
	}
	if err := fixture.d.saveExploreCheckpointOwned(ExploreCheckpoint{Campaign: campaign, Graph: "seat-v2", Status: exploreRunning}, crashed); err == nil {
		t.Fatal("crashed worker must not commit after takeover")
	}
}

func TestExploreClaimTransactionRollback(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-rollback"
	seedExploreCampaign(t, fixture.d, campaign)
	_, err := fixture.d.claimExploreCampaignThen(campaign, "worker-tx", func(*sql.Tx) error {
		return errors.New("forced rollback after conditional update")
	})
	if err == nil {
		t.Fatal("injected failure must abort the claim transaction")
	}
	rows, err := fixture.d.query("SELECT claimed_by,status FROM exploration_state WHERE campaign="+quote(campaign), 2)
	if err != nil || len(rows) != 1 {
		t.Fatalf("campaign row missing: %+v %v", rows, err)
	}
	if rows[0][0] != "" || rows[0][1] == exploreRunning {
		t.Fatalf("rolled-back claim must not leave a running owner: %+v", rows[0])
	}
}

func TestExploreWorkerOwnershipAfterRestart(t *testing.T) {
	fixture := prepareSeatFixture(t, "http://127.0.0.1:9")
	campaign := "X-restart"
	seedExploreCampaign(t, fixture.d, campaign)
	first, err := fixture.d.claimExploreCampaign(campaign, "worker-keep")
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := openDB(fixture.d.file, false)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.close()
	state, err := reopened.loadExploreCheckpoint(campaign)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := reopened.query("SELECT claimed_by,fence FROM exploration_state WHERE campaign="+quote(campaign), 2)
	if err != nil || len(rows) != 1 || rows[0][0] != "worker-keep" {
		t.Fatalf("ownership must survive process restart: state=%+v rows=%v %v", state, rows, err)
	}
	same, err := reopened.claimExploreCampaign(campaign, "worker-keep")
	if err != nil {
		t.Fatal(err)
	}
	if same.Fence <= first.Fence {
		t.Fatalf("same worker reclaim must advance the fence: %+v %+v", first, same)
	}
	if _, err := reopened.claimExploreCampaign(campaign, "worker-other"); err == nil {
		t.Fatal("another worker must not steal a live lease after restart")
	}
	if time.Since(mustParseTime(t, same.ClaimedAt)) > time.Minute {
		t.Fatalf("reclaim timestamp is stale: %+v", same)
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
