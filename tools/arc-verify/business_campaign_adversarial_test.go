package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func campaignStoreFixture(t *testing.T) (db, BusinessCampaign, businessCampaignLease) {
	t.Helper()
	d, err := openDB(filepath.Join(t.TempDir(), "campaign.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.close() })
	if err := d.tx(schema); err != nil {
		t.Fatal(err)
	}
	budget := BusinessCampaignBudget{MaxActions: 8, MaxStates: 10, DeadlineMS: 30000}
	variants := []BusinessCampaignVariant{{ID: "amount-100", Branch: "accept", Runner: "runner-100", Task: "contract/accept", Action: "enroll", Actor: "primary", RunnerSHA256: strings.Repeat("a", 64), CodeSHA256: strings.Repeat("b", 64)}}
	c := BusinessCampaign{
		Version: 1, ID: "BC-store-test", Graph: "contract", Flow: "enrollment", Environment: "isolated", Round: "Q-one", Status: "RUNNING", EvidenceStatus: "NOT_RUN",
		Identity: ExecutionIdentity{Graph: "contract", Environment: "isolated", Account: "student-A", Principal: "principal-A", Fixture: "course-A", SourceCommit: "commit-A"},
		Request:  BusinessCampaignRequest{Graph: "contract", Flow: "enrollment", Environment: "isolated", Round: "Q-one", Account: "student-A", Fixture: "course-A", Commit: "commit-A", Variants: append([]BusinessCampaignVariant(nil), variants...), Budget: budget},
		Variants: variants, Budget: budget, Frontier: []BusinessCampaignCandidate{}, Visits: []BusinessCampaignVisit{},
	}
	lease, err := d.createBusinessCampaign(&c, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return d, c, lease
}

func campaignStoredCount(t *testing.T, d db, table, id string) string {
	t.Helper()
	rows, err := d.query("SELECT COUNT(*) FROM "+table+" WHERE campaign_id="+quote(id), 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("count %s: %v %v", table, rows, err)
	}
	return rows[0][0]
}

func TestBusinessCampaignStoreEventChainCASAndFencedOwnership(t *testing.T) {
	d, c, lease := campaignStoreFixture(t)
	stale := c
	c.Reason = "one independent observation recorded"
	c.Spent.Observations = 1
	if err := d.saveBusinessCampaign(&c, lease, "OBSERVE"); err != nil {
		t.Fatal(err)
	}
	got, err := d.readBusinessCampaign(c.ID)
	if err != nil || got.Revision != 2 || got.Spent.Observations != 1 || got.Created != c.Created {
		t.Fatalf("saved revision was not verified: %+v %v", got, err)
	}
	stale.Reason = "stale worker overwrites observation"
	if err := d.saveBusinessCampaign(&stale, lease, "OBSERVE"); err == nil {
		t.Fatal("stale revision overwrote a newer observation")
	}
	if err := d.releaseBusinessCampaign(c.ID, lease); err != nil {
		t.Fatal(err)
	}
	if err := d.validateBusinessCampaignLease(c.ID, lease); err == nil {
		t.Fatal("released owner retained permission to dispatch")
	}
	next, err := d.claimBusinessCampaign(c, time.Now().Add(time.Minute))
	if err != nil || next.Fence != lease.Fence+1 || next.Owner == lease.Owner {
		t.Fatalf("new owner lacks a distinct fence: %+v %v", next, err)
	}
	if err := d.saveBusinessCampaign(&c, lease, "OLD_WORKER"); err == nil {
		t.Fatal("old owner committed after a new owner claimed the campaign")
	}
	if err := d.releaseBusinessCampaign(c.ID, lease); err == nil {
		t.Fatal("old owner released the new owner's lease")
	}
	if err := d.validateBusinessCampaignLease(c.ID, next); err != nil {
		t.Fatal(err)
	}
	if campaignStoredCount(t, d, "business_campaign_events", c.ID) != "2" {
		t.Fatal("claim/release or stale writes replaced or appended business history")
	}
}

func TestBusinessCampaignStoreIntentAndProjectionCommitAtomically(t *testing.T) {
	d, c, lease := campaignStoreFixture(t)
	c.Spent.ReservedActions = 3
	insert := "INSERT INTO business_campaign_dispatches(campaign_id,seq,run_id,variant_id,runner_id,task,status,intent_sha256,body_json,created,updated) VALUES(" + quote(c.ID) + ",1,'R-one','amount-100','runner-100','contract/accept','INTENT','not-qualified','{}','now','now')"
	if err := d.saveBusinessCampaign(&c, lease, "INTENT", insert, "UPDATE business_campaign_dispatches SET status='STARTED' WHERE run_id='missing-run'"); err == nil {
		t.Fatal("zero-row guarded intent update was accepted")
	}
	got, err := d.readBusinessCampaign(c.ID)
	if err != nil || got.Revision != 1 || got.Spent.ReservedActions != 0 || c.Revision != 1 {
		t.Fatalf("failed transaction changed projection: %+v %v", got, err)
	}
	if campaignStoredCount(t, d, "business_campaign_events", c.ID) != "1" || campaignStoredCount(t, d, "business_campaign_dispatches", c.ID) != "0" {
		t.Fatal("failed intent transaction left a partial event or dispatch")
	}
	if err := d.saveBusinessCampaign(&c, lease, "INTENT", insert); err != nil {
		t.Fatal(err)
	}
	if campaignStoredCount(t, d, "business_campaign_dispatches", c.ID) != "1" {
		t.Fatal("intent was not committed with its reservation")
	}
}

// Recompute the final event and projection hashes deliberately. This defeats a
// shallow wrong-SHA test and checks invariants against the unchanged CREATE.
func rehashCampaignLastEvent(t *testing.T, d db, c BusinessCampaign) {
	t.Helper()
	rows, err := d.query("SELECT previous_sha256 FROM business_campaign_events WHERE campaign_id="+quote(c.ID)+" AND seq="+strconv.Itoa(c.Revision), 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("read last hash: %v %v", rows, err)
	}
	body, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	hash := businessCampaignEventHash(rows[0][0], string(body))
	if err := d.tx([]string{
		"UPDATE business_campaign_events SET body_json=" + quote(string(body)) + ",sha256=" + quote(hash) + ",created=" + quote(c.Updated) + " WHERE campaign_id=" + quote(c.ID) + " AND seq=" + strconv.Itoa(c.Revision),
		"UPDATE business_campaigns SET body_json=" + quote(string(body)) + ",sha256=" + quote(hash) + ",graph=" + quote(c.Graph) + ",flow=" + quote(c.Flow) + ",environment=" + quote(c.Environment) + ",round=" + quote(c.Round) + ",status=" + quote(c.Status) + ",updated=" + quote(c.Updated) + " WHERE id=" + quote(c.ID),
	}); err != nil {
		t.Fatal(err)
	}
}

func TestBusinessCampaignStoreRehashedScopeReplacementIsBlocked(t *testing.T) {
	for _, kind := range []string{"principal", "account", "fixture", "round", "variant-runner", "input-variant", "budget", "flow"} {
		t.Run(kind, func(t *testing.T) {
			d, c, lease := campaignStoreFixture(t)
			if err := d.saveBusinessCampaign(&c, lease, "OBSERVE"); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "principal":
				c.Identity.Principal = "principal-B"
			case "account":
				c.Identity.Account, c.Request.Account = "student-B", "student-B"
			case "fixture":
				c.Identity.Fixture, c.Request.Fixture = "course-B", "course-B"
			case "round":
				c.Round, c.Request.Round = "Q-other", "Q-other"
			case "variant-runner":
				c.Variants[0].Runner = "runner-other"
			case "input-variant":
				c.Request.Variants[0].ID = "amount-100.0"
			case "budget":
				c.Budget.MaxActions, c.Request.Budget.MaxActions = 9999, 9999
			case "flow":
				c.Flow, c.Request.Flow = "other-flow", "other-flow"
			}
			rehashCampaignLastEvent(t, d, c)
			if _, err := d.readBusinessCampaign(c.ID); err == nil || !strings.Contains(err.Error(), "frozen") {
				t.Fatalf("self-consistent tampered scope was accepted: %v", err)
			}
			public, err := d.loadBusinessCampaign(c.ID)
			if err != nil || public.ID != c.ID || public.Status != "BLOCKED" || public.EvidenceStatus != "BLOCKED" || len(public.Variants)+len(public.Frontier)+len(public.Visits) != 0 || public.Identity != (ExecutionIdentity{}) {
				t.Fatalf("corrupt content leaked into public evidence: %+v %v", public, err)
			}
		})
	}
}

func TestBusinessCampaignStoreRehashedSpentResetCannotRestoreBudgets(t *testing.T) {
	for _, field := range []string{"states", "observations", "elapsed", "dispatches", "actions", "target", "preparation", "recovery"} {
		t.Run(field, func(t *testing.T) {
			d, c, lease := campaignStoreFixture(t)
			c.Spent = BusinessCampaignSpent{Actions: 4, TargetActions: 2, PreparationActions: 1, RecoveryActions: 1, Dispatches: 2, States: 8, Observations: 6, ElapsedMS: 50, ReservedActions: 4}
			if err := d.saveBusinessCampaign(&c, lease, "OBSERVE"); err != nil {
				t.Fatal(err)
			}
			// A reservation may shrink on settlement; it cannot be confused
			// with the monotonic counters of already observed/spent work.
			c.Spent.ReservedActions = 0
			if err := d.saveBusinessCampaign(&c, lease, "SETTLE"); err != nil {
				t.Fatalf("legal conservative reservation release was rejected: %v", err)
			}
			switch field {
			case "states":
				c.Spent.States = 0
			case "observations":
				c.Spent.Observations = 0
			case "elapsed":
				c.Spent.ElapsedMS = 0
			case "dispatches":
				c.Spent.Dispatches = 0
			case "actions":
				c.Spent.Actions = 0
			case "target":
				c.Spent.TargetActions = 0
			case "preparation":
				c.Spent.PreparationActions = 0
			case "recovery":
				c.Spent.RecoveryActions = 0
			}
			if err := d.saveBusinessCampaign(&c, lease, "RESET_SPENT"); err == nil {
				t.Fatal("normal save reset an already spent budget")
			}
			rehashCampaignLastEvent(t, d, c)
			if _, err := d.readBusinessCampaign(c.ID); err == nil || !strings.Contains(err.Error(), "budget history regressed") {
				t.Fatalf("self-consistent latest hash erased spent history: %v", err)
			}
		})
	}
}

func TestBusinessCampaignStoreRejectsDeletedOrDuplicatedEventHistory(t *testing.T) {
	for _, mutation := range []string{"delete-first", "revision-gap", "duplicate-create", "unknown-field"} {
		t.Run(mutation, func(t *testing.T) {
			d, c, lease := campaignStoreFixture(t)
			if err := d.saveBusinessCampaign(&c, lease, "OBSERVE"); err != nil {
				t.Fatal(err)
			}
			var statement string
			switch mutation {
			case "delete-first":
				statement = "DELETE FROM business_campaign_events WHERE campaign_id=" + quote(c.ID) + " AND seq=1"
			case "revision-gap":
				statement = "UPDATE business_campaign_events SET seq=3 WHERE campaign_id=" + quote(c.ID) + " AND seq=2"
			case "duplicate-create":
				statement = "UPDATE business_campaign_events SET kind='CREATE' WHERE campaign_id=" + quote(c.ID) + " AND seq=2"
			case "unknown-field":
				statement = "UPDATE business_campaigns SET body_json='{" + `"allow_replay":true` + "}' WHERE id=" + quote(c.ID)
			}
			if err := d.tx([]string{statement}); err != nil {
				t.Fatal(err)
			}
			if _, err := d.readBusinessCampaign(c.ID); err == nil {
				t.Fatalf("%s history was trusted", mutation)
			}
		})
	}
}

func TestBusinessCampaignStoreAbortFencesWorkerButRetainsUnknownIntentAndLocks(t *testing.T) {
	d, c, lease := campaignStoreFixture(t)
	c.Spent.ReservedActions = 3
	insert := "INSERT INTO business_campaign_dispatches(campaign_id,seq,run_id,variant_id,runner_id,task,status,intent_sha256,body_json,created,updated) VALUES(" + quote(c.ID) + ",1,'R-uncertain','amount-100','runner-100','contract/accept','STARTED','opaque','{}','now','now')"
	lock := "INSERT INTO fixture_locks(fixture,run) VALUES('course-A','R-uncertain')"
	if err := d.saveBusinessCampaign(&c, lease, "INTENT", insert, lock); err != nil {
		t.Fatal(err)
	}
	aborted, err := d.abortBusinessCampaign(c.ID, "operator stopped this campaign")
	if err != nil || aborted.Status != "ABORTED" || aborted.EvidenceStatus != "BLOCKED" || aborted.Spent.ReservedActions != 3 || !strings.Contains(aborted.Reason, "must not be replayed") {
		t.Fatalf("abort lost unresolved side effects: %+v %v", aborted, err)
	}
	if err := d.validateBusinessCampaignLease(c.ID, lease); err == nil {
		t.Fatal("aborted worker may still dispatch")
	}
	if err := d.saveBusinessCampaign(&c, lease, "OLD_WORKER"); err == nil {
		t.Fatal("old worker wrote a successful snapshot after abort")
	}
	if _, err := d.claimBusinessCampaign(aborted, time.Now().Add(time.Minute)); err == nil {
		t.Fatal("aborted campaign was resumed")
	}
	if campaignStoredCount(t, d, "business_campaign_dispatches", c.ID) != "1" {
		t.Fatal("abort erased uncertain dispatch")
	}
	rows, err := d.query("SELECT run FROM fixture_locks WHERE fixture='course-A'", 1)
	if err != nil || len(rows) != 1 || rows[0][0] != "R-uncertain" {
		t.Fatalf("abort silently released an uncertain fixture: %v %v", rows, err)
	}
	if _, err := d.readBusinessCampaign(c.ID); err != nil {
		t.Fatal(err)
	}
}

func TestBusinessCampaignStoreCorruptAbortRevokesLeaseWithoutInventingHistory(t *testing.T) {
	d, c, lease := campaignStoreFixture(t)
	if err := d.tx([]string{"UPDATE business_campaigns SET body_json='corrupt evidence body' WHERE id=" + quote(c.ID)}); err != nil {
		t.Fatal(err)
	}
	// This is the actual adversarial window: a damaged body alone does not
	// revoke the separate lightweight ownership guard used before dispatch.
	if err := d.validateBusinessCampaignLease(c.ID, lease); err != nil {
		t.Fatalf("fixture did not retain the active worker: %v", err)
	}
	result, err := d.abortBusinessCampaign(c.ID, "stop damaged campaign")
	if err != nil || result.Status != "BLOCKED" || result.EvidenceStatus != "BLOCKED" || !strings.Contains(result.Reason, "lease revoked") {
		t.Fatalf("corrupt abort incorrectly claimed normal completion: %+v %v", result, err)
	}
	if err := d.validateBusinessCampaignLease(c.ID, lease); err == nil {
		t.Fatal("corrupt abort left the old worker authorized to dispatch")
	}
	rows, err := d.query("SELECT body_json,status,owner FROM business_campaigns WHERE id="+quote(c.ID), 3)
	if err != nil || len(rows) != 1 || rows[0][0] != "corrupt evidence body" || rows[0][1] != "RUNNING" || rows[0][2] != "" {
		t.Fatalf("revocation rewrote corrupt history or failed to revoke: %v %v", rows, err)
	}
	if campaignStoredCount(t, d, "business_campaign_events", c.ID) != "1" {
		t.Fatal("corrupt history was laundered by appending a normal ABORT event")
	}
	if _, err := d.claimBusinessCampaign(c, time.Now().Add(time.Minute)); err == nil {
		t.Fatal("damaged campaign was claimed after diagnostic revocation")
	}
}

func TestBusinessCampaignStoreExpiredLeaseAndConcurrentClaim(t *testing.T) {
	d, c, old := campaignStoreFixture(t)
	expired := time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)
	if err := d.tx([]string{"UPDATE business_campaigns SET lease_until=" + quote(expired) + " WHERE id=" + quote(c.ID)}); err != nil {
		t.Fatal(err)
	}
	old.Until = expired
	if err := d.validateBusinessCampaignLease(c.ID, old); err == nil {
		t.Fatal("expired lease authorized dispatch")
	}
	if err := d.saveBusinessCampaign(&c, old, "EXPIRED"); err == nil {
		t.Fatal("expired lease saved a new intent")
	}
	d2, err := openDB(d.file, false)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.close()
	start := make(chan struct{})
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, handle := range []db{d, d2} {
		workers.Add(1)
		go func(handle db) {
			defer workers.Done()
			<-start
			_, err := handle.claimBusinessCampaign(c, time.Now().Add(time.Minute))
			results <- err
		}(handle)
	}
	close(start)
	workers.Wait()
	close(results)
	winners := 0
	for err := range results {
		if err == nil {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("concurrent campaign ownership has %d winners, want exactly one", winners)
	}
}

func rewriteCampaignEventBodies(t *testing.T, d db, id string, mutate func(*BusinessCampaign)) {
	t.Helper()
	rows, err := d.query("SELECT seq,body_json FROM business_campaign_events WHERE campaign_id="+quote(id)+" ORDER BY seq", 2)
	if err != nil || len(rows) == 0 {
		t.Fatalf("campaign history unavailable: %v %v", rows, err)
	}
	previous := ""
	statements := []string{}
	var last string
	for _, row := range rows {
		var c BusinessCampaign
		if err := decodeStrictJSON([]byte(row[1]), &c); err != nil {
			t.Fatal(err)
		}
		mutate(&c)
		body, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		hash := businessCampaignEventHash(previous, string(body))
		statements = append(statements, "UPDATE business_campaign_events SET body_json="+quote(string(body))+",previous_sha256="+quote(previous)+",sha256="+quote(hash)+" WHERE campaign_id="+quote(id)+" AND seq="+row[0])
		previous, last = hash, string(body)
	}
	statements = append(statements, "UPDATE business_campaigns SET body_json="+quote(last)+",sha256="+quote(previous)+" WHERE id="+quote(id))
	if err := d.tx(statements); err != nil {
		t.Fatal(err)
	}
}

func TestBusinessCampaignResumeReplaysActualActionBudgetBeforeAnyNewDispatch(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	f, request := newCampaignTestFixture(t, campaignTestGraph(t, false), server.URL, campaignTestManifest(t, "budget-a", false), campaignTestManifest(t, "budget-b", false))
	request.Budget.MaxActions = 2 // one target and one restore; second variant cannot run
	c, err := f.d.runBusinessCampaign(context.Background(), request)
	if err != nil || c.Status != "BUDGET" || target.requests != 1 || len(c.Visits) != 1 || c.Spent.Actions != 2 {
		t.Fatalf("counterexample needs a real, budget-exhausted child: status=%s reason=%s spent=%+v requests=%d err=%v", c.Status, c.Reason, c.Spent, target.requests, err)
	}
	// Rewrite every event consistently, preserving monotonic zero counters,
	// valid SHA chains, exact request budget, original children and RAW. This
	// specifically requires resume's child-evidence replay, not a hash check.
	rewriteCampaignEventBodies(t, f.d, c.ID, func(snapshot *BusinessCampaign) {
		snapshot.Spent.Actions = 0
		snapshot.Spent.TargetActions = 0
		snapshot.Spent.PreparationActions = 0
		snapshot.Spent.RecoveryActions = 0
	})
	if _, err := f.d.readBusinessCampaign(c.ID); err != nil {
		t.Fatalf("mutation was not internally hash-consistent: %v", err)
	}
	resumed, resumeErr := f.d.resumeBusinessCampaign(context.Background(), BusinessCampaignRequest{Campaign: c.ID})
	if target.requests != 1 || resumed.Status != "BLOCKED" || resumed.EvidenceStatus != "BLOCKED" {
		t.Fatalf("self-consistent counters reset the real action budget: status=%s reason=%s requests=%d err=%v", resumed.Status, resumed.Reason, target.requests, resumeErr)
	}
}

func TestBusinessCampaignCannotTransplantAnEarlierOrdinaryVerifiedPass(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	g := campaignTestGraph(t, false)
	g.BusinessContract.Subjects = append(g.BusinessContract.Subjects, BusinessSubject{ID: "request", Kind: "environment", Attributes: []BusinessAttribute{{Name: "variant", Type: "string"}}})
	m := campaignTestManifest(t, "same-input", false)
	m.Steps[0].InputMeasurements = map[string]string{"request.variant": "/variant"}
	f, request := newCampaignTestFixture(t, g, server.URL, m)
	if _, err := f.d.plan(g.ID, 128); err != nil {
		t.Fatal(err)
	}
	old, err := f.d.execute(context.Background(), ExecuteRequest{Task: g.ID + "/" + m.Test, Runner: m.ID, Round: f.round, Environment: request.Environment, Commit: f.commit, Root: f.root, Config: f.config, Account: f.account, Fixture: f.fixture})
	if err != nil || old.Verdict != "PASS" {
		t.Fatalf("old ordinary run is not independently valid: %+v %v", old, err)
	}
	oldFacts, err := f.d.verifiedRunFacts(old.Run)
	if err != nil || oldFacts.EvidenceStatus != "PASS" || !oldFacts.OfflineComplete {
		t.Fatalf("old run has no actual qualified RAW: %+v %v", oldFacts, err)
	}
	c, err := f.d.runBusinessCampaign(context.Background(), request)
	if err != nil || c.Status != "COMPLETED_IN_SCOPE" || len(c.Visits) != 1 || target.requests != 2 {
		t.Fatalf("campaign control failed: status=%s reason=%s requests=%d err=%v", c.Status, c.Reason, target.requests, err)
	}
	dispatches, err := f.d.businessCampaignDispatches(c)
	if err != nil || len(dispatches) != 1 {
		t.Fatalf("control dispatch unavailable: %v %v", dispatches, err)
	}
	intent := dispatches[0].Intent
	intent.Run, intent.Candidate.Run = old.Run, old.Run
	_, runtimes, err := f.d.businessCampaignRuntimes(c.Request, false)
	if err != nil {
		t.Fatal(err)
	}
	prior := c
	prior.HistorySHA256 = intent.HistorySHA256
	intent.Candidate.StateKey = campaignObservationKey(prior, runtimes[0], old.Run, *intent.Candidate.Observation, *intent.Candidate.Before)
	body, err := json.Marshal(intent)
	if err != nil {
		t.Fatal(err)
	}
	hash := digestBytes(body)
	if err := f.d.tx([]string{"UPDATE business_campaign_dispatches SET run_id=" + quote(old.Run) + ",intent_sha256=" + quote(hash) + ",body_json=" + quote(string(body)) + " WHERE campaign_id=" + quote(c.ID)}); err != nil {
		t.Fatal(err)
	}
	// Build the transplanted visit from genuine old facts, not handwritten
	// outcomes, and recompute every affected projection hash. It is still the
	// wrong child: it never consumed this campaign's one-shot intent.
	oldReceipt, err := f.d.modelRun(old.Run)
	if err != nil {
		t.Fatal(err)
	}
	visit := c.Visits[0]
	visit.Run, visit.DispatchSHA256, visit.BeforeStateKey = old.Run, hash, intent.Candidate.StateKey
	visit.Recovery = &oldReceipt.Recovery
	for _, fact := range oldFacts.Facts {
		if fact.SourceKind == "setup" {
			visit.Preparation = fact.Preparation
		}
		if fact.SourceKind == "execute" || fact.SourceKind == "sequence" {
			if len(fact.BusinessExpectations) != 1 || fact.BusinessExpectations[0].Decision.Frozen == nil {
				t.Fatal("old actual expected/actual comparison is absent")
			}
			entry := fact.BusinessExpectations[0]
			before := entry.Decision.Frozen.Before
			visit.Before, visit.After, visit.Comparison = &before, entry.After, entry.Comparison
			visit.Principal, visit.Entities, visit.Reason = fact.Principal, fact.Entities, campaignReason(fact.Reason)
		}
	}
	visit.AfterStateKey, _ = businessJSONHash(struct {
		History string
		Run     string
		Binding RunBinding
		Facts   []UnifiedStepFact
	}{intent.HistorySHA256, old.Run, intent.Binding, oldFacts.Facts})
	newHistory, _ := businessJSONHash(struct {
		Previous string
		Visit    BusinessCampaignVisit
	}{intent.HistorySHA256, visit})
	rewriteCampaignEventBodies(t, f.d, c.ID, func(snapshot *BusinessCampaign) {
		if len(snapshot.Dispatches) > 0 {
			snapshot.Dispatches[0] = BusinessCampaignDispatchReference{Sequence: 1, Run: old.Run, SHA256: hash}
			for i := range snapshot.Frontier {
				if snapshot.Frontier[i].Selected {
					snapshot.Frontier[i] = intent.Candidate
				}
			}
		}
		if len(snapshot.Visits) > 0 {
			snapshot.Visits[0] = visit
			snapshot.Variants[0].LastRun = old.Run
			snapshot.LastRun, snapshot.LastActual = old.Run, visit.After
			snapshot.HistorySHA256 = newHistory
		}
	})
	c, err = f.d.readBusinessCampaign(c.ID)
	if err != nil {
		t.Fatalf("transplant failed before actual-child checks: %v", err)
	}
	if _, err := f.d.businessCampaignDispatches(c); err != nil {
		t.Fatalf("transplant failed before atomic child provenance checks: %v", err)
	}
	loaded, err := f.d.loadBusinessCampaign(c.ID)
	if err != nil || loaded.Status != "BLOCKED" || loaded.EvidenceStatus != "BLOCKED" || !strings.Contains(loaded.Reason, "atomic campaign start provenance") || target.requests != 2 {
		t.Fatalf("another genuine ordinary PASS was accepted as this campaign's child: status=%s reason=%s requests=%d err=%v", loaded.Status, loaded.Reason, target.requests, err)
	}
}

func TestBusinessCampaignDeadlineAlsoBoundsRecoveryDispatch(t *testing.T) {
	var targets, recoveries atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/enroll":
			targets.Add(1)
			_, _ = io.Copy(io.Discard, r.Body)
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		case "/reset":
			recoveries.Add(1)
			fmt.Fprint(w, `{}`)
		default:
			fmt.Fprint(w, `{"state":"pending","principal":"synthetic-STUDENT","role":"STUDENT","scope":"course-fixture","login":true,"enrolled":0,"capacity":1,"active":true,"stored":false}`)
		}
	}))
	defer server.Close()
	m := campaignTestManifest(t, "bounded-recovery", false)
	m.TimeoutMS, m.StepTimeoutMS = 6000, 3000
	f, request := newCampaignTestFixture(t, campaignTestGraph(t, false), server.URL, m)
	request.Budget.DeadlineMS = 1000
	c, err := f.d.runBusinessCampaign(context.Background(), request)
	if targets.Load() != 1 || recoveries.Load() != 0 || c.Status == "COMPLETED_IN_SCOPE" {
		t.Fatalf("recovery escaped the campaign deadline: status=%s reason=%s targets=%d recoveries=%d err=%v", c.Status, c.Reason, targets.Load(), recoveries.Load(), err)
	}
}
