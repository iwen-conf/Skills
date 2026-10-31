package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func refutationLedger(t *testing.T) db {
	t.Helper()
	graph, err := loadGraph("examples/academic-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return modelTestDB(t, graph)
}

func refutationEvidence(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "refutation.md")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func refutedRisk(t *testing.T, d db, id string) RiskEntry {
	t.Helper()
	report, err := d.riskReport("academic-v1")
	if err != nil {
		t.Fatal(err)
	}
	for _, risk := range append(report.Risks, report.ReferenceRisks...) {
		if risk.ID == id {
			return risk
		}
	}
	t.Fatalf("risk %s missing from the report", id)
	return RiskEntry{}
}

func TestUnrefutedRiskShownAsNotAttempted(t *testing.T) {
	d := refutationLedger(t)
	if risk := refutedRisk(t, d, "K-SCHEDULE"); risk.Refutation != "not_attempted" {
		t.Fatalf("a risk nobody tried to refute must say so: %+v", risk)
	}
}

func TestRefutationNeedsEvidenceHash(t *testing.T) {
	d := refutationLedger(t)
	for _, args := range [][]string{
		{"risk", "refute", "--db", d.file, "--graph", "academic-v1", "--risk", "K-SCHEDULE", "--result", "refuted"},
		{"risk", "refute", "--db", d.file, "--graph", "academic-v1", "--risk", "K-SCHEDULE", "--result", "maybe", "--evidence", refutationEvidence(t, "x")},
		{"risk", "refute", "--db", d.file, "--graph", "academic-v1", "--risk", "K-MISSING", "--result", "refuted", "--evidence", refutationEvidence(t, "x")},
	} {
		if err := run(args); err == nil {
			t.Fatalf("an unsupported refutation was recorded: %v", args)
		}
	}
	evidence := refutationEvidence(t, "arc-idx search 'f:src/ schedule conflict' found an explicit allow-overlap flag for electives")
	cliJSON(t, "risk", "refute", "--db", d.file, "--graph", "academic-v1", "--risk", "K-SCHEDULE", "--result", "refuted", "--evidence", evidence)
	risk := refutedRisk(t, d, "K-SCHEDULE")
	if risk.Refutation != "refuted" || len(risk.RefutationSHA256) != 64 || !strings.HasSuffix(risk.RefutationEvidence, "refutation.md") {
		t.Fatalf("refutation lost its evidence binding: %+v", risk)
	}
}

func TestRefutedRiskStaysOpen(t *testing.T) {
	d := refutationLedger(t)
	cliJSON(t, "risk", "refute", "--db", d.file, "--graph", "academic-v1", "--risk", "K-SCHEDULE", "--result", "refuted", "--evidence", refutationEvidence(t, "product allows overlapping electives"))
	if risk := refutedRisk(t, d, "K-SCHEDULE"); risk.Status != "OPEN" {
		t.Fatalf("a refutation must not close the risk; the user decides: %+v", risk)
	}
	open, err := d.openRisks("academic-v1")
	if err != nil || open != 1 {
		t.Fatalf("a refuted risk is still undecided: %d %v", open, err)
	}
	cliJSON(t, "risk", "refute", "--db", d.file, "--graph", "academic-v1", "--risk", "K-SCHEDULE", "--result", "supported", "--evidence", refutationEvidence(t, "a real run enrolled two overlapping required courses"))
	if risk := refutedRisk(t, d, "K-SCHEDULE"); risk.Refutation != "supported" || len(risk.RefutationHistory) != 2 {
		t.Fatalf("later refutation evidence must be kept with its history: %+v", risk)
	}
}

func TestRefutationEvidenceTamperDetected(t *testing.T) {
	d := refutationLedger(t)
	evidence := refutationEvidence(t, "search log")
	cliJSON(t, "risk", "refute", "--db", d.file, "--graph", "academic-v1", "--risk", "K-SCHEDULE", "--result", "not_found", "--evidence", evidence)
	if err := os.WriteFile(evidence, []byte("edited afterwards"), 0o600); err != nil {
		t.Fatal(err)
	}
	if risk := refutedRisk(t, d, "K-SCHEDULE"); risk.Refutation != "evidence_changed" {
		t.Fatalf("changed refutation evidence must not be shown as a result: %+v", risk)
	}
}

func TestReferenceRiskCanBeRefuted(t *testing.T) {
	d := refutationLedger(t)
	matchCatalog(t, d, "academic-v1", enrollmentCatalog)
	cliJSON(t, "risk", "refute", "--db", d.file, "--graph", "academic-v1", "--risk", "ref/ENR-CAPACITY-RACE", "--result", "supported", "--evidence", refutationEvidence(t, "two concurrent requests both succeeded for the last seat"))
	if risk := refutedRisk(t, d, "ref/ENR-CAPACITY-RACE"); risk.Refutation != "supported" || risk.Origin != "peer_reference" {
		t.Fatalf("a peer reference risk must carry its refutation: %+v", risk)
	}
}
