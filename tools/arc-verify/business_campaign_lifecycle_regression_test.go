package main

import (
	"context"
	"testing"
)

func TestBusinessCampaignCannotResumePastVerifiedFailureByChangingStopStatus(t *testing.T) {
	for _, verdict := range []string{"FAIL", "BLOCKED"} {
		for _, replaceStop := range []bool{false, true} {
			name := verdict + "/original-stop"
			if replaceStop {
				name = verdict + "/rehashed-budget-stop"
			}
			t.Run(name, func(t *testing.T) {
				target, server := newRuntimeBusinessTarget(t, true)
				graph := campaignTestGraph(t, false)
				graph.Tests[0].Kind = testPositive
				first := campaignTestManifest(t, "failed-first-variant", false)
				second := campaignTestManifest(t, "ready-second-variant", false)
				second.Test = graph.Tests[0].ID // independent READY task after the first failure
				fixture, request := newCampaignTestFixture(t, graph, server.URL, first, second)
				request.Budget.MaxActions = 4
				hooks := &businessCampaignHooks{
					afterIntent: func(_ BusinessCampaign, dispatch *businessCampaignExecution) error {
						if dispatch.Intent.Sequence == 1 {
							target.mu.Lock()
							target.wrongOutcome = verdict == "FAIL"
							target.unknownActive = verdict == "BLOCKED"
							target.mu.Unlock()
						}
						return nil
					},
					afterExecute: func(BusinessCampaign, *businessCampaignExecution) error {
						target.mu.Lock()
						target.wrongOutcome, target.unknownActive = false, false
						target.mu.Unlock()
						return nil
					},
				}
				campaign, err := fixture.d.runBusinessCampaignWithHooks(context.Background(), request, hooks)
				if err != nil || len(campaign.Visits) != 1 || campaign.Visits[0].Status != verdict || campaign.FirstDivergence != campaign.Visits[0].Run {
					t.Fatalf("fixture did not capture its real first failure: campaign=%+v err=%v", campaign, err)
				}
				target.mu.Lock()
				beforeRequests := target.requests
				target.mu.Unlock()
				if replaceStop {
					campaign.Status = "BUDGET"
					// Only the last stop projection changes. Original RAW, the
					// failed child and FirstDivergence remain intact; hashes are
					// deliberately made consistent to require semantic replay.
					rehashCampaignLastEvent(t, fixture.d, campaign)
					if _, err := fixture.d.readBusinessCampaign(campaign.ID); err != nil {
						t.Fatalf("counterexample was not a self-consistent event chain: %v", err)
					}
				}
				checked, err := fixture.d.loadBusinessCampaign(campaign.ID)
				if err != nil {
					t.Fatal(err)
				}
				resumed, resumeErr := fixture.d.resumeBusinessCampaign(context.Background(), BusinessCampaignRequest{Campaign: campaign.ID})
				target.mu.Lock()
				afterRequests := target.requests
				target.mu.Unlock()
				if afterRequests != beforeRequests {
					t.Fatalf("stop marker reopened a campaign after verified %s: before=%d after=%d checked=%s/%s resumed=%s visits=%d err=%v", verdict, beforeRequests, afterRequests, checked.Status, checked.EvidenceStatus, resumed.Status, len(resumed.Visits), resumeErr)
				}
				if replaceStop {
					if checked.Status != "BLOCKED" || checked.EvidenceStatus != "BLOCKED" || resumed.Status != "BLOCKED" || resumed.EvidenceStatus != "BLOCKED" {
						t.Fatalf("inconsistent failure/stop state qualified as evidence: checked=%+v resumed=%+v err=%v", checked, resumed, resumeErr)
					}
				} else if resumeErr != nil || checked.EvidenceStatus != "PASS" || resumed.Status != campaign.Status || resumed.FirstDivergence != campaign.FirstDivergence || len(resumed.Visits) != 1 {
					t.Fatalf("legitimate stopped failure history was lost: checked=%+v resumed=%+v err=%v", checked, resumed, resumeErr)
				}
				if !replaceStop {
					if _, err := fixture.d.abortBusinessCampaign(campaign.ID, "operator closes the failed campaign"); err != nil {
						t.Fatal(err)
					}
					aborted, err := fixture.d.resumeBusinessCampaign(context.Background(), BusinessCampaignRequest{Campaign: campaign.ID})
					target.mu.Lock()
					requests := target.requests
					target.mu.Unlock()
					if err != nil || aborted.Status != "ABORTED" || aborted.EvidenceStatus != "PASS" || aborted.FirstDivergence != campaign.FirstDivergence || len(aborted.Visits) != 1 || requests != beforeRequests {
						t.Fatalf("explicit abort must retain authentic failure without dispatch: aborted=%+v requests=%d err=%v", aborted, requests, err)
					}
				}
			})
		}
	}
}

func TestBusinessCampaignResumeContinuesHealthyCancellationStop(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, true)
	fixture, request := newCampaignTestFixture(t, campaignTestGraph(t, false), server.URL,
		campaignTestManifest(t, "cancellation-first", false), campaignTestManifest(t, "cancellation-second", false))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	campaign, err := fixture.d.runBusinessCampaignWithHooks(ctx, request, &businessCampaignHooks{
		afterExecute: func(BusinessCampaign, *businessCampaignExecution) error {
			cancel()
			return nil
		},
	})
	if err != nil || campaign.Status != "BUDGET" || campaign.FirstDivergence != "" || len(campaign.Visits) != 1 || campaign.Visits[0].Status != "PASS" || target.requests != 1 {
		t.Fatalf("control needs a healthy cancellation stop: campaign=%+v requests=%d err=%v", campaign, target.requests, err)
	}
	checked, err := fixture.d.loadBusinessCampaign(campaign.ID)
	if err != nil || checked.Status != "BUDGET" || checked.EvidenceStatus != "PASS" {
		t.Fatalf("healthy budget stop lost its verified evidence: campaign=%+v err=%v", checked, err)
	}
	resumed, err := fixture.d.resumeBusinessCampaign(context.Background(), BusinessCampaignRequest{Campaign: campaign.ID})
	if err != nil || resumed.Status != "COMPLETED_IN_SCOPE" || resumed.EvidenceStatus != "PASS" || resumed.FirstDivergence != "" || len(resumed.Visits) != 2 || resumed.Visits[0].Run != campaign.Visits[0].Run || target.requests != 2 {
		t.Fatalf("healthy resume failed or replayed its recorded child: campaign=%+v requests=%d err=%v", resumed, target.requests, err)
	}
	checked, err = fixture.d.loadBusinessCampaign(campaign.ID)
	if err != nil || checked.Status != "COMPLETED_IN_SCOPE" || checked.EvidenceStatus != "PASS" {
		t.Fatalf("resumed history cannot be independently verified: campaign=%+v err=%v", checked, err)
	}
}
