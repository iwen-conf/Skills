package main

import (
	"os"
	"strings"
	"testing"
)

func TestIsolatedSeatMutationRecoveryBranchIsNotVacuous(t *testing.T) {
	body, err := os.ReadFile("isolated_seat_test.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	if strings.Contains(src, "// recovery should still restore after a product FAIL") {
		t.Fatal("mutation campaign still has a comment-only recovery branch; a detected defect is not a restored fixture")
	}
	if strings.Contains(src, `item.category != "race_oversell"`) && strings.Contains(src, "Recovery.Verdict != \"PASS\"") {
		t.Fatal("recovery is still skipped for race_oversell instead of being classified independently")
	}
}

func TestIsolatedSeatRaceInjectionHasNoSleep(t *testing.T) {
	body, err := os.ReadFile("isolated_seat_test.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	start := strings.Index(src, "func (s *seatServer) raceClaim")
	if start < 0 {
		t.Fatal("raceClaim is missing")
	}
	rest := src[start:]
	end := strings.Index(rest[1:], "\nfunc ")
	if end > 0 {
		rest = rest[:end+1]
	}
	if strings.Contains(rest, "time.Sleep") {
		t.Fatal("race injection still waits on sleep instead of a deterministic occupancy barrier")
	}
}

func TestClassifyRecoveryDoesNotTreatDetectionAsRestored(t *testing.T) {
	detected := ExecuteResult{Verdict: "FAIL", Recovery: RecoveryOutcome{Verdict: "NOT_RUN", Reason: "recovery was not observed"}}
	if classifyRecovery(detected.Recovery, 1) == RecoveryRestored {
		t.Fatal("a product FAIL was treated as a restored fixture")
	}
	if !mustNotReuseFixture(classifyRecovery(detected.Recovery, 1)) {
		t.Fatal("unknown recovery must forbid fixture reuse")
	}
	restored := RecoveryOutcome{Verdict: "PASS", Safe: true, State: "idle", Evidence: "raw.json"}
	if classifyRecovery(restored, 0) != RecoveryRestored {
		t.Fatalf("independent restore was not classified as restored: %s", classifyRecovery(restored, 0))
	}
	failed := RecoveryOutcome{Verdict: "BLOCKED", Reason: "restore failed", Evidence: "raw.json"}
	if classifyRecovery(failed, 1) != RecoveryFailedLocked {
		t.Fatalf("failed restore with retained lock: %s", classifyRecovery(failed, 1))
	}
	unknown := RecoveryOutcome{Verdict: "BLOCKED", Reason: "oracle missing"}
	if classifyRecovery(unknown, 0) != RecoveryUnknown {
		t.Fatalf("recovery without lock or evidence must stay unknown: %s", classifyRecovery(unknown, 0))
	}
}
