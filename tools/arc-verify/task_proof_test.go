package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskProofRevalidatesOneLinkedObligationWithoutFullGraphClaim(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	run := latestRun(t, fixture)
	rows, err := fixture.d.query("SELECT task FROM runs WHERE id="+quote(run), 1)
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	proof, err := fixture.d.taskProof("course-v2", rows[0][0])
	if err != nil || proof.Verdict != "PASS" || !proof.InputsUnchanged || proof.ProofScope != "linked_task_evidence" || proof.Run != run || len(proof.RAWSHA256) == 0 {
		t.Fatalf("valid linked proof failed: %+v %v", proof, err)
	}
	want := proof.ReceiptSHA256
	proof.ReceiptSHA256 = ""
	got, err := businessJSONHash(proof)
	if err != nil || got != want {
		t.Fatal("receipt digest does not bind the complete proof")
	}
	// Current source is checked, not just the stored PASS text.
	if err := os.WriteFile(filepath.Join(fixture.root, "changed-after-proof.txt"), []byte("different candidate"), 0600); err != nil {
		t.Fatal(err)
	}
	if proof, err := fixture.d.taskProof("course-v2", rows[0][0]); err == nil || proof.Verdict == "PASS" || proof.InputsUnchanged {
		t.Fatalf("stale task proof accepted: %+v %v", proof, err)
	}
}

func TestTaskProofNeverSkipsLatestUnfinishedAttempt(t *testing.T) {
	fixture, stop := passingCourseFixture(t)
	defer stop()
	run := latestRun(t, fixture)
	rows, err := fixture.d.query("SELECT task FROM runs WHERE id="+quote(run), 1)
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	if err := fixture.d.tx([]string{"INSERT INTO runs(id,task,environment,commit_sha,status) SELECT 'later-unfinished',task,environment,commit_sha,'RUNNING' FROM runs WHERE id=" + quote(run)}); err != nil {
		t.Fatal(err)
	}
	proof, err := fixture.d.taskProof("course-v2", rows[0][0])
	if err == nil || proof.Verdict == "PASS" || proof.Run != "later-unfinished" {
		t.Fatalf("proof cherry-picked old PASS: %+v %v", proof, err)
	}
}
