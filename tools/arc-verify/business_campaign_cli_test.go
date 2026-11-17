package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBusinessCampaignCLIRejectsImplicitScopeBeforeOpeningLedger(t *testing.T) {
	for _, args := range [][]string{
		{"campaign"},
		{"campaign", "guess"},
		{"campaign", "run"},
		{"campaign", "resume"},
		{"campaign", "status"},
		{"campaign", "list"},
		{"campaign", "abort", "--id", "BC-example"},
		{"campaign", "status", "--id", "BC-example", "--file", "request.json"},
		{"campaign", "run", "--file", "request.json", "unexpected"},
	} {
		err := run(args)
		if err == nil || strings.Contains(err.Error(), "database not initialized") {
			t.Fatalf("invalid campaign request reached a ledger: %v: %v", args, err)
		}
	}
}

func TestBusinessCampaignInstalledPublicCLIExecutesWithoutReplay(t *testing.T) {
	binary := os.Getenv("ARC_CAMPAIGN_BINARY")
	if binary == "" {
		t.Skip("set ARC_CAMPAIGN_BINARY to the installed public arc executable for a real process integration")
	}
	target, server := newRuntimeBusinessTarget(t, true)
	fixture := prepareExecutionFixture(t, campaignTestGraph(t, false), server.URL)
	if err := fixture.d.close(); err != nil {
		t.Fatal(err)
	}
	cli := func(args ...string) []byte {
		t.Helper()
		command := exec.Command(binary, append([]string{"verify"}, args...)...)
		command.Dir = fixture.root
		var stderr strings.Builder
		command.Stderr = &stderr
		body, err := command.Output()
		if err != nil {
			t.Fatalf("installed arc %v: %v %s %s", args, err, stderr.String(), body)
		}
		return body
	}
	manifest := campaignTestManifest(t, "installed.campaign", false)
	runnerFile := filepath.Join(t.TempDir(), "runner.json")
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runnerFile, body, 0600); err != nil {
		t.Fatal(err)
	}
	cli("runner", "--db", fixture.d.file, "--file", runnerFile)
	var round struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(cli("round", "--db", fixture.d.file, "--graph", "course-v2", "--env", "independent", "--root", fixture.root, "--config", fixture.config, "--commit", fixture.commit), &round); err != nil || round.ID == "" {
		t.Fatalf("installed CLI round: %+v %v", round, err)
	}
	request := BusinessCampaignRequest{Graph: "course-v2", Flow: "flow.enrollment", Environment: "independent", Round: round.ID,
		Commit: fixture.commit, Root: fixture.root, Config: fixture.config, Account: fixture.account, Fixture: fixture.fixture,
		Variants: []BusinessCampaignVariant{{ID: "installed-input", Branch: "accept", Runner: manifest.ID}},
		Budget:   BusinessCampaignBudget{MaxActions: 4, MaxStates: 8, DeadlineMS: 60000}}
	body, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	requestFile := filepath.Join(t.TempDir(), "campaign.json")
	if err := os.WriteFile(requestFile, body, 0600); err != nil {
		t.Fatal(err)
	}
	var result BusinessCampaign
	if err := json.Unmarshal(cli("campaign", "run", "--db", fixture.d.file, "--file", requestFile), &result); err != nil || result.Status != "COMPLETED_IN_SCOPE" || len(result.Visits) != 1 || result.EvidenceStatus != "PASS" || target.requests != 1 {
		t.Fatalf("installed public CLI did not complete the real HTTP tool scope: %+v %v", result, err)
	}
	for _, command := range []string{"status", "resume"} {
		var replay BusinessCampaign
		if err := json.Unmarshal(cli("campaign", command, "--db", fixture.d.file, "--id", result.ID), &replay); err != nil || replay.ID != result.ID || replay.EvidenceStatus != "PASS" || target.requests != 1 {
			t.Fatalf("installed public CLI %s changed scope or replayed requests: %+v %v", command, replay, err)
		}
	}
}

func TestBusinessCampaignCLIExecutesScopedVariantsAndReadsTheirOwnHistory(t *testing.T) {
	target, server := newRuntimeBusinessTarget(t, false)
	fixture, request := newCampaignTestFixture(t, campaignTestGraph(t, true), server.URL,
		campaignTestManifest(t, "cli.input-a", true), campaignTestManifest(t, "cli.input-b", true))
	target.ledger = &fixture.d
	path := filepath.Join(t.TempDir(), "campaign.json")
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	body, err = cliOutput(t, "campaign", "run", "--db", fixture.d.file, "--file", path)
	var executed BusinessCampaign
	if decodeErr := json.Unmarshal(body, &executed); err != nil || decodeErr != nil || executed.Status != "COMPLETED_IN_SCOPE" {
		t.Fatalf("campaign CLI did not return a scoped execution receipt: %s %v %v", body, err, decodeErr)
	}
	if target.requests != 2 || target.preparations != 2 || len(executed.Visits) != 2 || !executed.Coverage.SelectedVariantsCovered || executed.Coverage.FullBusinessCoverage {
		t.Fatalf("CLI conflated variants, actual setup or business completeness: %+v", executed)
	}
	var status BusinessCampaign
	if err := json.Unmarshal(cliJSON(t, "campaign", "status", "--db", fixture.d.file, "--id", executed.ID), &status); err != nil || status.EvidenceStatus != "PASS" || len(status.Visits) != 2 {
		t.Fatalf("CLI status did not rehydrate its own checked child facts: %+v %v", status, err)
	}
	var listed struct {
		Campaigns []BusinessCampaign `json:"campaigns"`
		Available int                `json:"available"`
		Truncated bool               `json:"truncated"`
	}
	if err := json.Unmarshal(cliJSON(t, "campaign", "list", "--db", fixture.d.file, "--graph", request.Graph, "--env", request.Environment, "--round", request.Round), &listed); err != nil || len(listed.Campaigns) != 1 || listed.Available != 1 || listed.Truncated {
		t.Fatalf("CLI history lost its scope/count: %+v %v", listed, err)
	}
	if listed.Campaigns[0].ID != executed.ID {
		t.Fatal("CLI returned another campaign's history")
	}
	cliJSON(t, "campaign", "resume", "--db", fixture.d.file, "--id", executed.ID)
	if target.requests != 2 || target.preparations != 2 {
		t.Fatal("resume replayed already completed child operations")
	}
}

func TestBusinessCampaignRequestRejectsUnknownFieldsAndMismatchedResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "campaign.json")
	if err := os.WriteFile(path, []byte(`{"campaign":"BC-original","graph":"payment-v2","variants":[{"id":"amount-100","branch":"refund","runner":"refund-100"}],"budget":{"max_actions":5,"max_states":8,"deadline_ms":1000}}`), 0600); err != nil {
		t.Fatal(err)
	}
	request, err := readBusinessCampaignRequest(path)
	if err != nil || request.Graph != "payment-v2" || len(request.Variants) != 1 || request.Variants[0].ID != "amount-100" || request.Budget.MaxActions != 5 {
		t.Fatalf("campaign request was lost during decoding: %+v %v", request, err)
	}
	err = run([]string{"campaign", "resume", "--id", "BC-other", "--file", path})
	if err == nil || !strings.Contains(err.Error(), "identity differs") {
		t.Fatalf("resume accepted a request from another campaign: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"graph":"payment-v2","automatically_approve_rules":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readBusinessCampaignRequest(path); err == nil {
		t.Fatal("unsupported campaign instructions were silently ignored")
	}
}

func TestBusinessCampaignExitCodesDoNotTurnStopsIntoBusinessPass(t *testing.T) {
	for _, status := range []string{"RUNNING", "BUDGET", "BLOCKED", "MODEL_ERROR", "SPECIFICATION_GAP", "DIVERGED", "UNKNOWN", "ABORTED"} {
		err := businessCampaignExecutionError(BusinessCampaign{Status: status, EvidenceStatus: "PASS", Coverage: BusinessCampaignCoverage{SelectedVariantsCovered: true}})
		if err == nil || exitCode(fmt.Errorf("wrapper: %w", err)) < 10 {
			t.Fatalf("%s was accepted as successful execution: %v", status, err)
		}
	}
	for _, campaign := range []BusinessCampaign{
		{Status: "COMPLETED_IN_SCOPE", EvidenceStatus: "BLOCKED", Coverage: BusinessCampaignCoverage{SelectedVariantsCovered: true}},
		{Status: "COMPLETED_IN_SCOPE", EvidenceStatus: "PASS"},
	} {
		if err := businessCampaignExecutionError(campaign); err == nil {
			t.Fatal("damaged or incomplete selected coverage returned success")
		}
	}
	if err := businessCampaignExecutionError(BusinessCampaign{Status: "COMPLETED_IN_SCOPE", EvidenceStatus: "PASS", Coverage: BusinessCampaignCoverage{SelectedVariantsCovered: true}}); err != nil {
		t.Fatal(err)
	}
	if exitCode(errors.New("invalid request")) != 1 {
		t.Fatal("invalid request exit status changed")
	}
}
