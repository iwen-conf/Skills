package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

// Arc Policy is authoritative for independent test results until its receipt
// store is physically migrated. This adapter reads/validates an existing PASS,
// never invokes verification or treats AUTHORIZED as a successful test.
type policyTaskStatus struct {
	RepoRoot  string `json:"repo_root"`
	ReceiptOK bool   `json:"receipt_ok"`
	Task      struct {
		ID      string          `json:"id"`
		Receipt json.RawMessage `json:"receipt"`
	} `json:"task"`
}
type verifiedTaskProof struct {
	TaskID          string
	ReceiptSHA256   string
	ReceiptHead     string
	ReceiptDigest   string
	ReceiptTree     string
	VerifiedAt      string
	SourceSnapshot  string
	Checks          []map[string]any
}

func requirePolicyProof(taskID string, status policyTaskStatus) (verifiedTaskProof, error) {
	if !status.ReceiptOK || status.Task.ID != taskID || taskID == "" {
		return verifiedTaskProof{}, errors.New("missing current, matching Arc Policy verification receipt")
	}
	var receipt struct {
		TaskID    string           `json:"task_id"`
		Verdict   string           `json:"verdict"`
		CreatedAt string           `json:"created_at"`
		Head      string           `json:"head"`
		Digest    string           `json:"digest"`
		Tree      string           `json:"tree"`
		Checks    []map[string]any `json:"checks"`
	}
	if err := json.Unmarshal(status.Task.Receipt, &receipt); err != nil {
		return verifiedTaskProof{}, fmt.Errorf("invalid policy receipt: %w", err)
	}
	// User-authorized waiver is NOT an executed test, even if receipt_ok=true.
	if receipt.Verdict != "PASS" || receipt.TaskID != taskID ||
		receipt.Head == "" || receipt.Digest == "" || receipt.Tree == "" ||
		receipt.CreatedAt == "" {
		return verifiedTaskProof{}, errors.New("task completion requires a genuine revision-bound PASS, not authorization or absent evidence")
	}
	snapshotOK, receiptOK := false, false
	for _, check := range receipt.Checks {
		switch check["id"] {
		case "snapshot":
			snapshotOK = check["verdict"] == "PASS"
		case "cli_receipt":
			receiptOK = check["verdict"] == "PASS"
		}
	}
	if !snapshotOK || !receiptOK {
		return verifiedTaskProof{}, errors.New("policy PASS lacks successful snapshot and receipt checks")
	}
	sum := sha256.Sum256(status.Task.Receipt)
	return verifiedTaskProof{
		TaskID: taskID, ReceiptSHA256: hex.EncodeToString(sum[:]),
		ReceiptHead: receipt.Head, ReceiptDigest: receipt.Digest,
		ReceiptTree: receipt.Tree, VerifiedAt: receipt.CreatedAt,
		Checks: receipt.Checks,
	}, nil
}

func policyTaskStatusFromCLI(ctx context.Context, root string) (policyTaskStatus, error) {
	bin, err := exec.LookPath("arc-policy")
	if err != nil { return policyTaskStatus{}, errors.New("arc-policy CLI required to verify a task's current receipt") }
	cmd := exec.CommandContext(ctx, bin, "--cwd", root, "task", "status")
	output, err := cmd.Output()
	if err != nil { return policyTaskStatus{}, fmt.Errorf("cannot validate Arc Policy task: %w", err) }
	var status policyTaskStatus
	if err := json.Unmarshal(output, &status); err != nil { return status, fmt.Errorf("invalid policy status JSON: %w", err) }
	resolved, err := filepath.EvalSymlinks(status.RepoRoot)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(root) {
		return status, errors.New("Arc Policy receipt belongs to a different repository")
	}
	return status, nil
}

// Must run BEFORE obtaining withArcLock: Arc Policy status re-checks business
// graph evidence via arc verify, which itself acquires that same repository lock.
func loadCompletionProof(root, taskID string) (verifiedTaskProof, error) {
	before, err := sourceSnapshot(root)
	if err != nil { return verifiedTaskProof{}, err }
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	status, err := policyTaskStatusFromCLI(ctx, root)
	if err != nil { return verifiedTaskProof{}, err }
	proof, err := requirePolicyProof(taskID, status)
	if err != nil { return verifiedTaskProof{}, err }
	after, err := sourceSnapshot(root)
	if err != nil { return verifiedTaskProof{}, err }
	if before != after { return verifiedTaskProof{}, errors.New("source changed while checking task evidence") }
	proof.SourceSnapshot = before
	return proof, nil
}

// Atomic CAS + immutable evidence event; no background tests are run here.
// Pending outbox records prohibit completion until a real index flush succeeds.
func commitTaskCompletion(db *sql.DB, root, id string, expected int, proof verifiedTaskProof) error {
	if id == "" || expected < 1 || proof.TaskID != id || proof.ReceiptSHA256 == "" {
		return errors.New("task completion requires ID, expected version, matching PASS evidence")
	}
	current, err := sourceSnapshot(root)
	if err != nil { return err }
	if proof.SourceSnapshot != current { return errors.New("worktree changed since the verified receipt; retry task completion") }
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()
	var old string
	var version int
	if err := tx.QueryRowContext(ctx, "SELECT status,version FROM arc_tasks WHERE id=?", id).Scan(&old,&version); err != nil { return err }
	if version != expected { return fmt.Errorf("task version conflict: expected %d actual %d", expected, version) }
	if old != "verifying" { return fmt.Errorf("task must be verifying before done, currently %s", old) }
	var pending int
	if err := tx.QueryRowContext(ctx,"SELECT COUNT(*) FROM arc_outbox WHERE status='pending'").Scan(&pending); err != nil { return err }
	if pending != 0 { return fmt.Errorf("%d outbox events unacknowledged; run arc index flush first", pending) }
	// The completion claim is revision-specific. Keep the immutable PASS receipt
	// fingerprint and all checks in the event, not as a mutable status blob.
	res, err := tx.ExecContext(ctx,
		"UPDATE arc_tasks SET status='done',version=version+1,updated=? WHERE id=? AND version=? AND status='verifying'",
		nowUTC(), id, expected)
	if err != nil { return err }
	affected, err := res.RowsAffected()
	if err != nil || affected != 1 { return errors.New("concurrent task change; completion not committed") }
	evidence := map[string]any{
		"task_id": id, "from_version": version,
		"receipt_sha256": proof.ReceiptSHA256,
		"head": proof.ReceiptHead, "tree": proof.ReceiptTree,
		"digest": proof.ReceiptDigest, "verified_at": proof.VerifiedAt,
		"source_snapshot": proof.SourceSnapshot, "checks": proof.Checks,
	}
	if err := addEvent(ctx, tx, "task", id, "task.completed", evidence); err != nil { return err }
	if err := tx.Commit(); err != nil { return err }
	return emitJSON(map[string]any{"id":id, "status":"done", "version":expected+1, "receipt_sha256": proof.ReceiptSHA256})
}
