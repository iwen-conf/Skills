package main

import (
	"context"
	"errors"
	"testing"
)

func TestBusinessCampaignCrashAfterIntentNeverReplaysUnknownDispatch(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	f, request := newCampaignTestFixture(t, campaignTestGraph(t, false), server.URL, campaignTestManifest(t, "intent-only", false))
	crash := errors.New("injected process stop immediately after durable intent")
	result, err := f.d.runBusinessCampaignWithHooks(context.Background(), request, &businessCampaignHooks{afterIntent: func(c BusinessCampaign, execution *businessCampaignExecution) error {
		rows, err := f.d.businessCampaignDispatches(c)
		if err != nil || len(rows) != 1 || rows[0].Status != "INTENT" || rows[0].Intent.Run != execution.Intent.Run {
			t.Fatalf("intent was not durable before hook: %+v %v", rows, err)
		}
		return crash
	}})
	if !errors.Is(err, crash) || target.requests != 0 {
		t.Fatalf("crash boundary sent target: %+v %v", result, err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		resumed, err := f.d.resumeBusinessCampaign(context.Background(), BusinessCampaignRequest{Campaign: result.ID})
		if err != nil || resumed.Status != "UNKNOWN" || target.requests != 0 || len(resumed.Visits) != 0 || resumed.Spent.ReservedActions != 2 {
			t.Fatalf("unproven intent was replayed/reclaimed as safe: %+v %v count=%d", resumed, err, target.requests)
		}
	}
}

func TestBusinessCampaignCrashAfterCompletedChildReconcilesWithoutReplay(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	f, request := newCampaignTestFixture(t, campaignTestGraph(t, false), server.URL, campaignTestManifest(t, "completed-child", false))
	crash := errors.New("injected process stop after child completed before campaign recording")
	child := ""
	result, err := f.d.runBusinessCampaignWithHooks(context.Background(), request, &businessCampaignHooks{afterExecute: func(c BusinessCampaign, execution *businessCampaignExecution) error {
		child = execution.Intent.Run
		receipt, err := f.d.modelRun(child)
		if err != nil || !receipt.Completed {
			t.Fatalf("fault did not reach actual completed child: %+v %v", receipt, err)
		}
		return crash
	}})
	if !errors.Is(err, crash) || target.requests != 1 || len(result.Visits) != 0 {
		t.Fatalf("wrong fault boundary: %+v %v requests=%d", result, err, target.requests)
	}
	resumed, err := f.d.resumeBusinessCampaign(context.Background(), BusinessCampaignRequest{Campaign: result.ID})
	if err != nil || resumed.Status != "COMPLETED_IN_SCOPE" || len(resumed.Visits) != 1 || resumed.Visits[0].Run != child || target.requests != 1 || resumed.Spent.ReservedActions != 0 {
		t.Fatalf("completed child was replayed or discarded: %+v %v requests=%d", resumed, err, target.requests)
	}
	loaded, err := f.d.loadBusinessCampaign(result.ID)
	if err != nil || loaded.EvidenceStatus != "PASS" {
		t.Fatalf("reconciled evidence was not verified: %+v %v", loaded, err)
	}
}

func TestBusinessCampaignAbortFencesAlreadySelectedTargetAndRetainsIntent(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	f, request := newCampaignTestFixture(t, campaignTestGraph(t, false), server.URL, campaignTestManifest(t, "abort-selected", false))
	stopped := errors.New("worker sees external abort")
	result, err := f.d.runBusinessCampaignWithHooks(context.Background(), request, &businessCampaignHooks{afterIntent: func(c BusinessCampaign, execution *businessCampaignExecution) error {
		if _, err := f.d.abortBusinessCampaign(c.ID, "operator stopped the campaign"); err != nil {
			t.Fatal(err)
		}
		_, err := f.d.execute(context.Background(), ExecuteRequest{Task: execution.Intent.Task, Runner: execution.Intent.Runner, Round: c.Round, Environment: c.Environment, Commit: c.Request.Commit, Root: c.Request.Root, Config: c.Request.Config, Account: c.Request.Account, Fixture: c.Request.Fixture, Accounts: c.Request.Accounts, campaign: execution})
		if err == nil {
			t.Fatal("stale selected worker retained mutation authorization")
		}
		return stopped
	}})
	if !errors.Is(err, stopped) || target.requests != 0 || target.preparations != 0 {
		t.Fatalf("aborted worker sent a request: %+v %v", result, err)
	}
	loaded, err := f.d.loadBusinessCampaign(result.ID)
	if err != nil || loaded.Status != "ABORTED" || loaded.Spent.ReservedActions != 2 {
		t.Fatalf("abort erased the unresolved intent/budget: %+v %v", loaded, err)
	}
}

func TestBusinessCampaignRefusesExternalComputedCoverage(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	f, request := newCampaignTestFixture(t, campaignTestGraph(t, false), server.URL, campaignTestManifest(t, "computed", false))
	request.Variants[0].Passes = 1
	result, err := f.d.runBusinessCampaign(context.Background(), request)
	if err == nil || result.Status == "COMPLETED_IN_SCOPE" || target.requests != 0 {
		t.Fatalf("caller-supplied PASS bypassed dispatch: %+v %v", result, err)
	}
}
