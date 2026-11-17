package main

import (
	"context"
	"os"
	"reflect"
	"testing"
)

func TestWorkbenchExportObservedBusinessCampaign(t *testing.T) {
	out, assets := os.Getenv("ARC_CAMPAIGN_EXPORT_DIR"), os.Getenv("ARC_WORKBENCH_ASSETS")
	if out == "" || assets == "" {
		t.Skip("set ARC_CAMPAIGN_EXPORT_DIR and ARC_WORKBENCH_ASSETS to export an actual HTTP tool campaign; not product E2E")
	}
	target, server := newCampaignStateTarget(t, false, false)
	graph := campaignCycleGraph(t, true)
	fixture, request := newCampaignTestFixture(t, graph, server.URL,
		campaignCheckpointManifest(t, "clear", "clear-active", false),
		campaignCheckpointManifest(t, "enroll-a", "accept", true),
		campaignCheckpointManifest(t, "enroll-b", "accept", true))
	request.Budget.DeadlineMS = 120000
	campaign, err := fixture.d.runBusinessCampaign(context.Background(), request)
	if err != nil || campaign.Status != "COMPLETED_IN_SCOPE" || len(campaign.Visits) != 3 || !reflect.DeepEqual(target.actions, []string{"enroll", "clear", "enroll"}) || target.preparations != 1 || campaign.Spent.TargetActions != 3 || campaign.Spent.PreparationActions != 1 || campaign.Spent.RecoveryActions != 0 {
		t.Fatalf("actual state-driven campaign did not complete its declared scope: %+v actions=%v prep=%d %v", campaign, target.actions, target.preparations, err)
	}
	for _, visit := range campaign.Visits {
		if visit.Before == nil || visit.After == nil || visit.Before.Bindings["order"] != target.instanceID || visit.After.Bindings["order"] != target.instanceID {
			t.Fatalf("continuous campaign lost its independently read business instance identity: %+v", visit)
		}
	}
	path, err := fixture.d.writeSnapshot(SnapshotRequest{Graph: graph.ID, Environment: request.Environment, Round: fixture.round,
		Config: fixture.config, Assets: assets, Out: out, Argv: []string{"tool-campaign-integration", "not-pay-gateway-acceptance"}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := readSnapshot(t, path)
	if len(snapshot.BusinessCampaigns) != 1 || snapshot.AvailableCampaigns != 1 || snapshot.CampaignsTruncated || snapshot.BusinessCampaigns[0].ID != campaign.ID || snapshot.BusinessCampaigns[0].EvidenceStatus != "PASS" || len(snapshot.Runs) != 3 {
		t.Fatal("native export omitted checked campaign scope, history or child runs")
	}
	for _, run := range snapshot.Runs {
		if run.EvidenceStatus != "PASS" || run.RunnerID == "" || run.RunnerSHA256 == "" {
			t.Fatalf("export lost verified child-run identity: %+v", run)
		}
	}
	t.Logf("observed HTTP tool campaign %s with three checkpoint child runs exported to %s; product E2E remains unproven", campaign.ID, path)
}
