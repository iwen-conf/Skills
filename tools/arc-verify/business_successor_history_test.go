package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Simulate an internally consistent capture produced by the former engine.
// Keep original model/runner/before receipts and make disk RAW, capture receipt,
// indexed step, fact projection and assertions agree. The regression must reach
// semantic replay; a bad SHA or stale projection is not an adequate test.
func replaceSuccessorHistoricalCapture(t *testing.T, d db, raw StepRAW) string {
	t.Helper()
	rows, err := d.query(fmt.Sprintf("SELECT facts_json FROM step_facts WHERE run=%s AND seq=%d AND source_kind IN ('execute','sequence')", quote(raw.Run), raw.Step), 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("missing source fact: %v", err)
	}
	var fact UnifiedStepFact
	if err := decodeStrictJSON([]byte(rows[0][0]), &fact); err != nil {
		t.Fatal(err)
	}
	path, body, err := writeRunnerRAW(d, raw.Run, raw.Step, raw)
	if err != nil {
		t.Fatal(err)
	}
	hash := digestBytes(body)
	fact.EvidencePath, fact.EvidenceSHA256 = path, hash
	fact.Oracles, fact.BusinessExpectations = raw.Model.Oracles, raw.Model.BusinessExpectations
	for i := range fact.Assertions {
		assertion := &fact.Assertions[i]
		if assertion.Kind != "business_expectation" {
			continue
		}
		for _, entry := range raw.Model.BusinessExpectations {
			if entry.Actor == assertion.Subject && entry.Comparison != nil {
				encoded, _ := json.Marshal(entry.Comparison)
				assertion.Actual = string(encoded)
			}
		}
	}
	factBody, _ := json.Marshal(fact)
	statements := []string{
		fmt.Sprintf("UPDATE steps SET evidence_path=%s,evidence_sha256=%s WHERE run=%s AND idx=%d", quote(path), quote(hash), quote(raw.Run), raw.Step),
		fmt.Sprintf("UPDATE step_facts SET evidence_path=%s,evidence_sha256=%s,raw_json=%s,facts_json=%s WHERE run=%s AND seq=%d AND source_kind=%s", quote(path), quote(hash), quote(string(body)), quote(string(factBody)), quote(raw.Run), raw.Step, quote(fact.SourceKind)),
	}
	for i, assertion := range fact.Assertions {
		statements = append(statements, fmt.Sprintf("UPDATE step_assertions SET actual_json=%s WHERE run=%s AND seq=%d AND source_kind=%s AND assertion_idx=%d", quote(assertion.Actual), quote(raw.Run), raw.Step, quote(fact.SourceKind), i+1))
	}
	if err := d.transition(statements); err != nil {
		t.Fatal(err)
	}
	if err := validateFactObservation(fact, raw); err != nil {
		t.Fatalf("test left inconsistent fact projection: %v", err)
	}
	if _, got, err := d.workbenchEvidence(raw.Run, "action", raw.Step); err != nil || got != hash {
		t.Fatalf("test left invalid RAW/engine capture: %s %v", got, err)
	}
	return hash
}

func TestBusinessSuccessorOfflineRechecksHistoricalSequenceDespiteConsistentHashes(t *testing.T) {
	target, server := newKindsServer(t)
	g := successorSequenceGraph(t, BusinessSuccessorPolicy{"confirm"})
	g.BusinessContract.Subjects[0].Attributes = append(g.BusinessContract.Subjects[0].Attributes, BusinessAttribute{Name: "count", Type: "integer"})
	expected := &g.BusinessContract.Branches[0].Expected
	expected.Mode, expected.Reason = "allowed_outcomes", "synthetic sourced count-dependent continuations"
	expected.Outcomes[0].Predicate = businessPredicate("eq", "fixture", "count", "integer", "1")
	other := expected.Outcomes[0]
	other.ID, other.Predicate, other.NextAllowed = "pending-review", businessPredicate("eq", "fixture", "count", "integer", "2"), BusinessSuccessorPolicy{"enroll"}
	expected.Outcomes = append(expected.Outcomes, other)
	f := prepareExecutionFixture(t, g, server.URL)
	file := sequenceManifest(t, "successor-history")
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var manifest RunnerManifest
	if err := decodeStrictJSON(body, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.Steps[0].Oracles[0].Measurements["fixture.count"] = "/enrolled"
	body, _ = json.Marshal(manifest)
	if err := os.WriteFile(file, body, 0600); err != nil {
		t.Fatal(err)
	}
	f.registerAndRound(t, file)
	result := executeKinds(t, f, manifest.ID, "test.sequence.enroll-confirm")
	if result.Verdict != "PASS" || target.confirmCalls != 1 {
		t.Fatalf("control did not produce real passing captures: %+v", result)
	}
	first, _, err := f.d.workbenchEvidence(result.Run, "action", 1)
	if err != nil {
		t.Fatal(err)
	}
	// Both outcomes permit the first action's observed state. In the historical
	// capture, its independent count selects the continuation forbidding confirm.
	first.Model.Oracles[0].After.Values["fixture.count"] = json.RawMessage(`2`)
	first.After, _ = factsHash(first.Model.Oracles[0].After)
	if err := recordRuntimeBusinessComparisons(g, ModelObligation{Definition: g.Tests[1]}, runnerBatches(manifest)[0], first.Model); err != nil {
		t.Fatal(err)
	}
	hash := replaceSuccessorHistoricalCapture(t, f.d, first)
	contract, err := f.d.runTaskContract(result.Run)
	if err != nil {
		t.Fatal(err)
	}
	if verdict, reason, _, err := f.d.evaluateModelObservationLocal(contract, first, 1, hash); err != nil || verdict != "PASS" {
		t.Fatalf("first local result must remain a valid allowed outcome: %s %s %v", verdict, reason, err)
	}
	second, secondHash, err := f.d.workbenchEvidence(result.Run, "action", 2)
	if err != nil {
		t.Fatal(err)
	}
	if verdict, reason, _, err := f.d.evaluateModelObservationLocal(contract, second, 2, secondHash); err != nil || verdict != "PASS" {
		t.Fatalf("second local result must pass before cross-step validation: %s %s %v", verdict, reason, err)
	}
	verdict, reason, _, err := f.d.evaluateModelObservation(contract, second, 2, secondHash)
	if err != nil || verdict != "BLOCKED" || !strings.Contains(reason, "next_allowed") {
		t.Fatalf("offline sequence violation escaped: %s %s %v", verdict, reason, err)
	}
	if verified, err := f.d.verifiedRunFacts(result.Run); err == nil && verified.EvidenceStatus == "PASS" {
		t.Fatalf("historical invalid sequence retained strong evidence: %+v", verified)
	}
	if err := f.d.qualifyModelRun(result.Run); err == nil {
		t.Fatal("historical invalid sequence remained eligible for coverage/dependencies")
	}
}
