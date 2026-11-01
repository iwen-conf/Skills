package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidEvidenceCanBecomeVerified(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Mode:   "deterministic",
		Budget: ExploreRuntimeBudget{MaxStates: 8, MaxPaths: 8, MaxActions: 2, Deadline: 8 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Steps) == 0 || report.Campaign == "" {
		t.Fatalf("CLI exploration produced no evidence: %+v", report)
	}
	state, err := fixture.d.loadExploreCheckpoint(report.Campaign)
	if err != nil {
		t.Fatal(err)
	}
	result := fixture.d.verifyExploreEvidence(state)
	if result.EvidenceTrust != trustVerified {
		t.Fatalf("bound live evidence must be VERIFIED: %+v", result)
	}
	if result.BusinessVerdict == "" {
		t.Fatal("business verdict must be populated separately from trust")
	}
	cover, err := fixture.d.explorationCoverage("seat-v2")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range cover {
		if item.EvidenceTrust == trustVerified {
			found = true
			if !item.Trusted {
				t.Fatalf("six-view coverage must mark VERIFIED as trusted: %+v", item)
			}
		}
	}
	if !found {
		t.Fatalf("snapshot coverage did not show VERIFIED: %+v", cover)
	}
	proof, gateErr := cliOutput(t, "gate", "--db", fixture.d.file, "--graph", "seat-v2")
	trust := evidenceTrustFromGate(t, proof)
	if trust != trustVerified {
		t.Fatalf("official gate must report VERIFIED for the same evidence: trust=%s err=%v %s", trust, gateErr, proof)
	}
}

func TestInvalidEvidenceCannotBecomeVerified(t *testing.T) {
	d := isolatedLedger(t, sample())
	completeIsolatedGraph(t, d)
	ident := testExploreIdentity(t, d, "academic-v1")
	campaign := putIdentityCampaign(t, d, ident, func(report *ExploreReport, _ *RunBinding) {
		if len(report.Steps) == 0 {
			t.Fatal("campaign needs a step")
		}
		if err := os.WriteFile(report.Steps[0].RAW, []byte(`{"tampered":true}`), 0600); err != nil {
			t.Fatal(err)
		}
	})
	result := d.verifyExploreEvidence(campaign)
	if result.EvidenceTrust == trustVerified {
		t.Fatalf("tampered RAW must not be VERIFIED: %+v", result)
	}
	if result.EvidenceTrust != trustTampered {
		t.Fatalf("tampered RAW should be TAMPERED, got %s (%s)", result.EvidenceTrust, result.InvalidReason)
	}
	if _, err := cliOutput(t, "gate", "--db", d.file, "--graph", "academic-v1"); err == nil {
		t.Fatal("official gate must reject tampered explore evidence")
	}
}

func TestBusinessFailWithTrustedEvidence(t *testing.T) {
	d := isolatedLedger(t, sample())
	completeIsolatedGraph(t, d)
	ident := testExploreIdentity(t, d, "academic-v1")
	campaign := putIdentityCampaign(t, d, ident, func(report *ExploreReport, _ *RunBinding) {
		report.RuleFails = []ExploreFinding{{Kind: "FAIL", Rule: "R1", Reason: "confirmed violation"}}
		report.Steps[0].Findings = report.RuleFails
		report.StopReason = "complete"
	})
	result := d.verifyExploreEvidence(campaign)
	if result.BusinessVerdict != "FAIL" {
		t.Fatalf("business FAIL must stay FAIL: %+v", result)
	}
	if result.EvidenceTrust != trustVerified {
		t.Fatalf("trusted FAIL evidence must remain VERIFIED: %+v", result)
	}
}

func TestEvidenceTrustMatchesOfficialGate(t *testing.T) {
	_, server := newSeatServer(t)
	fixture := prepareSeatFixture(t, server.URL)
	fixture.openRound(t)
	report, err := fixture.d.runExploration(t.Context(), ExploreRequest{
		Graph: "seat-v2", Flow: "flow.reservation", Environment: "independent", Round: fixture.round,
		Root: fixture.root, Config: fixture.config, Commit: fixture.commit,
		Account: fixture.account, Fixture: fixture.fixture, Adapter: seatExploreAdapter(t, seatIsolationScope),
		Mode:   "deterministic",
		Budget: ExploreRuntimeBudget{MaxStates: 6, MaxPaths: 6, MaxActions: 1, Deadline: 8 * time.Second},
	})
	if err != nil {
		t.Fatal(err)
	}
	cover, err := fixture.d.explorationCoverage("seat-v2")
	if err != nil {
		t.Fatal(err)
	}
	workbench := ""
	for _, item := range cover {
		if item.EvidenceTrust != "" {
			workbench = item.EvidenceTrust
			break
		}
	}
	proof, _ := cliOutput(t, "gate", "--db", fixture.d.file, "--graph", "seat-v2")
	official := evidenceTrustFromGate(t, proof)
	if workbench == "" || official == "" || workbench != official {
		t.Fatalf("workbench trust %q must match official gate %q report=%s", workbench, official, proof)
	}
	_ = report
}

func TestWorkbenchCannotInferTrustFromRAWPresence(t *testing.T) {
	d := isolatedLedger(t, sample())
	raw := filepath.Join(t.TempDir(), "step.json")
	body := []byte(`{"identity":{"graph":"academic-v1"},"run":"X-raw-only","step":1}`)
	if err := os.WriteFile(raw, body, 0600); err != nil {
		t.Fatal(err)
	}
	report := ExploreReport{
		Campaign: "X-raw-only", Graph: "academic-v1", StopReason: "complete",
		Steps: []ExploreStep{{
			Candidate: CandidateAction{Action: "submit", Scope: "R1"},
			RAW:       raw, SHA256: digestBytes(body),
			Findings: []ExploreFinding{{Kind: "FAIL", Rule: "R1"}},
		}},
		RuleFails: []ExploreFinding{{Kind: "FAIL", Rule: "R1"}},
	}
	if err := d.persistExploration(report); err != nil {
		t.Fatal(err)
	}
	cover, err := d.explorationCoverage("academic-v1")
	if err != nil {
		t.Fatal(err)
	}
	item := cover["R1"]
	if item.Trusted || item.EvidenceTrust == trustVerified {
		t.Fatalf("RAW presence must not mint VERIFIED: %+v", item)
	}
}

func TestEvidenceBecomesStaleAfterDeploymentChange(t *testing.T) {
	d := isolatedLedger(t, sample())
	completeIsolatedGraph(t, d)
	ident := testExploreIdentity(t, d, "academic-v1")
	campaign := putIdentityCampaign(t, d, ident, nil)
	before := d.verifyExploreEvidence(campaign)
	if before.EvidenceTrust != trustVerified {
		t.Fatalf("setup must be VERIFIED: %+v", before)
	}
	campaign.Binding.Identity.Target = "http://127.0.0.1:65535/new-deploy"
	campaign.Report.Identity.Target = campaign.Binding.Identity.Target
	body, err := json.Marshal(campaign)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.tx([]string{fmt.Sprintf(
		"UPDATE exploration_state SET body=%s WHERE campaign=%s",
		quote(string(body)), quote(campaign.Campaign),
	)}); err != nil {
		t.Fatal(err)
	}
	if err := d.persistExploreBinding(campaign.Campaign, campaign.Binding); err != nil {
		t.Fatal(err)
	}
	after := d.verifyExploreEvidence(campaign)
	if after.EvidenceTrust != trustStale && after.EvidenceTrust != trustUnbound {
		t.Fatalf("deployment change must stale or unbind evidence: %+v", after)
	}
	proof, err := cliOutput(t, "gate", "--db", d.file, "--graph", "academic-v1")
	if err == nil {
		t.Fatal("official gate must reject the changed deployment identity")
	}
	trust := evidenceTrustFromGate(t, proof)
	if trust != after.EvidenceTrust {
		t.Fatalf("gate trust %s must match verifier %s (%s)", trust, after.EvidenceTrust, proof)
	}
}

func TestGateRejectsMissingTargetIdentity(t *testing.T) {
	assertGateRejectsIdentity(t, "target", func(report *ExploreReport, binding *RunBinding) {
		binding.Identity.Target = ""
		report.Identity.Target = ""
	}, "MISSING_IDENTITY")
}

func TestGateRejectsMissingFixtureIdentity(t *testing.T) {
	assertGateRejectsIdentity(t, "fixture", func(report *ExploreReport, binding *RunBinding) {
		binding.Identity.Fixture = ""
		report.Identity.Fixture = ""
	}, "MISSING_IDENTITY")
}

func TestGateRejectsMissingActorIdentity(t *testing.T) {
	assertGateRejectsIdentity(t, "actor", func(report *ExploreReport, binding *RunBinding) {
		binding.Identity.Actor = ""
		binding.Identity.Principal = ""
		report.Identity.Actor = ""
		report.Identity.Principal = ""
	}, "MISSING_IDENTITY")
}

func TestGateRejectsMissingOracleEvidence(t *testing.T) {
	assertGateRejectsIdentity(t, "oracle", func(report *ExploreReport, binding *RunBinding) {
		report.OracleFields = nil
		report.Steps[0].OracleFields = nil
		binding.Oracle = ""
		report.Oracle = ""
	}, "MISSING_ORACLE")
}

func TestGateRejectsForeignCampaignEvidence(t *testing.T) {
	d := isolatedLedger(t, sample())
	completeIsolatedGraph(t, d)
	ident := testExploreIdentity(t, d, "academic-v1")
	putIdentityCampaign(t, d, ident, func(report *ExploreReport, _ *RunBinding) {
		raw := mustRead(t, report.Steps[0].RAW)
		var doc StepRAW
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		doc.Run = "X-foreign"
		rewriteStepRAW(t, report, 0, doc)
	})
	if _, err := cliOutput(t, "gate", "--db", d.file, "--graph", "academic-v1"); err == nil {
		t.Fatal("official gate must reject RAW from another campaign")
	}
}

func TestGateRejectsForeignStepEvidence(t *testing.T) {
	d := isolatedLedger(t, sample())
	completeIsolatedGraph(t, d)
	ident := testExploreIdentity(t, d, "academic-v1")
	putIdentityCampaign(t, d, ident, func(report *ExploreReport, binding *RunBinding) {
		raw := mustRead(t, report.Steps[0].RAW)
		var doc StepRAW
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		doc.Identity.Task = "explore/X-other-step"
		binding.Identity.Task = "explore/" + report.Campaign
		rewriteStepRAW(t, report, 0, doc)
	})
	if _, err := cliOutput(t, "gate", "--db", d.file, "--graph", "academic-v1"); err == nil {
		t.Fatal("official gate must reject RAW from another step")
	}
}

func TestGateRejectsSyntheticOnlyOracle(t *testing.T) {
	assertGateRejectsIdentity(t, "synthetic", func(report *ExploreReport, _ *RunBinding) {
		report.OracleFields = []OracleField{{Name: "state", Class: oracleSynthetic, Source: "fake", Synthetic: true}}
		report.Steps[0].OracleFields = report.OracleFields
	}, "SYNTHETIC_ONLY")
}

func TestGateRejectsDeploymentMismatch(t *testing.T) {
	d := isolatedLedger(t, sample())
	completeIsolatedGraph(t, d)
	ident := testExploreIdentity(t, d, "academic-v1")
	putIdentityCampaign(t, d, ident, func(report *ExploreReport, binding *RunBinding) {
		binding.Identity.Target = "http://127.0.0.1:9/other"
	})
	if _, err := cliOutput(t, "gate", "--db", d.file, "--graph", "academic-v1"); err == nil {
		t.Fatal("official gate must reject a deployment mismatch")
	}
}

func TestLegacyEvidenceIsNotAutomaticallyTrusted(t *testing.T) {
	d := isolatedLedger(t, sample())
	completeIsolatedGraph(t, d)
	raw := filepath.Join(t.TempDir(), "legacy.json")
	body := []byte(`{"identity":{"graph":"academic-v1"},"run":"X-legacy","step":1}`)
	if err := os.WriteFile(raw, body, 0600); err != nil {
		t.Fatal(err)
	}
	report := ExploreReport{
		Campaign: "X-legacy", Graph: "academic-v1", StopReason: "complete",
		Steps: []ExploreStep{{Candidate: CandidateAction{Action: "submit", Scope: "R1"}, RAW: raw, SHA256: digestBytes(body)}},
	}
	if err := d.persistExploration(report); err != nil {
		t.Fatal(err)
	}
	if err := d.saveExploreCheckpoint(ExploreCheckpoint{Campaign: "X-legacy", Graph: "academic-v1", Status: exploreCompleted, Report: report}); err != nil {
		t.Fatal(err)
	}
	result := d.verifyExploreEvidence(ExploreCheckpoint{Campaign: "X-legacy", Graph: "academic-v1", Report: report})
	if result.EvidenceTrust == trustVerified {
		t.Fatalf("legacy evidence must not auto-upgrade: %+v", result)
	}
	if result.EvidenceTrust != trustLegacy {
		t.Fatalf("legacy evidence should stay LEGACY_UNVERIFIED, got %s", result.EvidenceTrust)
	}
	proof, err := cliOutput(t, "gate", "--db", d.file, "--graph", "academic-v1")
	if err != nil {
		t.Fatalf("legacy explore evidence must not fail the existing academic graph: %v %s", err, proof)
	}
}

func assertGateRejectsIdentity(t *testing.T, name string, mutate func(*ExploreReport, *RunBinding), want string) {
	t.Helper()
	d := isolatedLedger(t, sample())
	completeIsolatedGraph(t, d)
	ident := testExploreIdentity(t, d, "academic-v1")
	putIdentityCampaign(t, d, ident, mutate)
	proof, err := cliOutput(t, "gate", "--db", d.file, "--graph", "academic-v1")
	if err == nil {
		t.Fatalf("%s: official gate accepted incomplete identity: %s", name, proof)
	}
	if !strings.Contains(string(proof), want) && !strings.Contains(err.Error(), want) {
		blob := string(proof) + err.Error()
		if !strings.Contains(blob, want) {
			t.Fatalf("%s: gate output must mention %s: %s / %v", name, want, proof, err)
		}
	}
}

func testExploreIdentity(t *testing.T, d db, graph string) ExecutionIdentity {
	t.Helper()
	sha, err := d.graphFingerprint(graph)
	if err != nil {
		t.Fatal(err)
	}
	return ExecutionIdentity{
		Graph: graph, Task: "explore/X-id", Environment: "isolated-test", Target: "http://127.0.0.1:9/academic",
		SourceCommit: "0123456789012345678901234567890123456789", GraphSHA256: sha,
		Actor: "STUDENT", Account: "fixture.student", Principal: "student-1", Fixture: "fixture.course",
	}
}

func putIdentityCampaign(t *testing.T, d db, ident ExecutionIdentity, mutate func(*ExploreReport, *RunBinding)) ExploreCheckpoint {
	t.Helper()
	campaignID := "X-id-" + shortDigest(ident.Target + ident.Actor + time.Now().String())[:8]
	ident.Task = "explore/" + campaignID
	doc := StepRAW{Identity: ident, Phase: "action", Run: campaignID, Step: 1, Expected: "ready", Observed: "ready", Oracle: "readback", Runner: "isolated-native-test", ExitCode: 0, Preconditions: map[string]bool{}}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), campaignID+".json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	fields := []OracleField{{Name: "state", Class: oracleObserved, Source: "readback", Target: ident.Target, Actor: ident.Actor, Entity: "ready"}}
	report := ExploreReport{
		Campaign: campaignID, Graph: ident.Graph, StopReason: "complete", Mode: "heuristic",
		EvidenceContract: exploreIdentityContract, Identity: ident, Oracle: "readback", OracleFields: fields,
		Steps: []ExploreStep{{
			Candidate: CandidateAction{Action: "submit", Scope: "R1", Role: ident.Actor, Kind: kindDeclared},
			RAW:       path, SHA256: digestBytes(body), After: IndependentFacts{State: "ready", Role: ident.Actor, Principal: ident.Principal},
			OracleFields: fields,
		}},
	}
	binding := RunBinding{Identity: ident, Oracle: "readback"}
	if mutate != nil {
		mutate(&report, &binding)
		report.Identity = binding.Identity
		if report.Oracle == "" {
			report.Oracle = binding.Oracle
		}
	}
	campaign := ExploreCheckpoint{
		Campaign: campaignID, Graph: ident.Graph, Status: exploreCompleted, SourceCommit: ident.SourceCommit,
		ModelSHA: ident.GraphSHA256, Fixture: binding.Identity.Fixture, Environment: ident.Environment,
		Binding: binding, Report: report, Mode: "heuristic",
	}
	if err := d.saveExploreCheckpoint(campaign); err != nil {
		t.Fatal(err)
	}
	if err := d.persistExploreBinding(campaignID, binding); err != nil {
		t.Fatal(err)
	}
	if err := d.persistExploration(report); err != nil {
		t.Fatal(err)
	}
	return campaign
}

func rewriteStepRAW(t *testing.T, report *ExploreReport, index int, doc StepRAW) {
	t.Helper()
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(report.Steps[index].RAW, body, 0600); err != nil {
		t.Fatal(err)
	}
	report.Steps[index].SHA256 = digestBytes(body)
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func cliOutput(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = writer
	type readResult struct {
		body []byte
		err  error
	}
	read := make(chan readResult, 1)
	go func() { body, err := io.ReadAll(reader); read <- readResult{body, err} }()
	runErr := run(args)
	writer.Close()
	os.Stdout = old
	output := <-read
	reader.Close()
	if output.err != nil {
		t.Fatal(output.err)
	}
	return output.body, runErr
}

func evidenceTrustFromGate(t *testing.T, proof []byte) string {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(proof, &doc); err != nil {
		t.Fatalf("gate JSON: %v %s", err, proof)
	}
	raw, _ := json.Marshal(doc["evidence_gate"])
	var rows []map[string]string
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("evidence_gate: %v %s", err, proof)
	}
	for _, row := range rows {
		if row["evidence_trust"] != "" {
			return row["evidence_trust"]
		}
	}
	return ""
}
