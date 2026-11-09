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
	"os"
	"path/filepath"
	"reflect"
	"time"
)

// Arc Policy is authoritative for independent test results until its receipt
// store is physically migrated. This adapter reads/validates an existing PASS,
// never invokes verification or treats AUTHORIZED as a successful test.
type policyTaskStatus struct {
	RepoRoot  string `json:"repo_root"`
	ReceiptOK bool   `json:"receipt_ok"`
	Tree struct {
		Head string `json:"head"`
		Digest string `json:"digest"`
		Dirty []string `json:"dirty"`
	} `json:"tree"`
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
		Dirty     []string         `json:"dirty"`
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
	snapshotOK, receiptOK, independentOK := false, false, false
	for _, check := range receipt.Checks {
		switch check["id"] {
		case "snapshot":
			snapshotOK = check["verdict"] == "PASS"
		case "cli_receipt":
			receiptOK = check["verdict"] == "PASS"
		case "tests", "business_graph":
			if check["verdict"] == "PASS" { independentOK = true }
		}
	}
	if !snapshotOK || !receiptOK || !independentOK {
		return verifiedTaskProof{}, errors.New("policy PASS lacks successful snapshot, receipt, or independent tests/business graph evidence")
	}
	if status.Tree.Head != receipt.Head || status.Tree.Digest != receipt.Digest ||
		!reflect.DeepEqual(status.Tree.Dirty, receipt.Dirty) {
		return verifiedTaskProof{}, errors.New("policy receipt fingerprint differs from current tree reported by Arc Policy")
	}
	sum, err := canonicalPolicyReceiptHash(status.Task.Receipt)
	if err != nil { return verifiedTaskProof{}, err }
	return verifiedTaskProof{
		TaskID: taskID, ReceiptSHA256: sum,
		ReceiptHead: receipt.Head, ReceiptDigest: receipt.Digest,
		ReceiptTree: receipt.Tree, VerifiedAt: receipt.CreatedAt,
		Checks: receipt.Checks,
	}, nil
}

// JSON canonicalization is required because Arc Policy writes state.json
// separately from the CLI's formatted JSON status output.
func canonicalPolicyReceiptHash(raw []byte) (string, error) {
	var data any
	if err := json.Unmarshal(raw, &data); err != nil { return "", err }
	if data == nil { return "", errors.New("empty policy receipt") }
	canonical, err := json.Marshal(data)
	if err != nil { return "", err }
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

// Caller MUST hold the policy task.lock before checking this file and until
// the arc.db transaction completes. A revoked/replaced receipt never
// authorizes a new Done state on the same source tree.
func validateLivePolicyReceipt(root string, proof verifiedTaskProof) error {
	path := filepath.Join(root, ".arc", "policy", "state.json")
	st, err := os.Lstat(path)
	if err != nil { return fmt.Errorf("policy state unavailable: %w", err) }
	if !st.Mode().IsRegular() { return errors.New("unsafe policy task state file") }
	raw, err := os.ReadFile(path)
	if err != nil { return err }
	var state struct {
		Active struct {
			ID string `json:"id"`
			Receipt json.RawMessage `json:"receipt"`
		} `json:"active"`
	}
	if err := json.Unmarshal(raw, &state); err != nil { return fmt.Errorf("invalid current policy state: %w",err) }
	if state.Active.ID != proof.TaskID { return errors.New("policy task changed during completion") }
	hash, err := canonicalPolicyReceiptHash(state.Active.Receipt)
	if err != nil { return fmt.Errorf("policy receipt revoked: %w",err) }
	if hash != proof.ReceiptSHA256 { return errors.New("policy receipt changed during completion") }
	return nil
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
	if err := tx.QueryRowContext(ctx,"SELECT COUNT(*) FROM arc_outbox o JOIN arc_events e ON e.id=o.event_id WHERE o.status='pending' AND e.aggregate IN ('file','change')").Scan(&pending); err != nil { return err }
	if pending != 0 { return fmt.Errorf("%d outbox events unacknowledged; run arc index flush first", pending) }
	var indexedSnapshot string
	if err := tx.QueryRowContext(ctx, "SELECT value FROM arc_meta WHERE key='index_last_snapshot'").Scan(&indexedSnapshot); err != nil {
		if errors.Is(err,sql.ErrNoRows) { return errors.New("no confirmed code index; run arc index flush first") }
		return err
	}
	if indexedSnapshot == "" || indexedSnapshot != proof.SourceSnapshot {
		return errors.New("code index snapshot is stale for this verified tree; run arc index flush")
	}
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


// Task completion is historical, not a perpetual PASS. Showing a completed
// task compares the immutable completion event with the current source tree,
// and explicitly does NOT claim the external business proof was rerun.
func showTaskWithProof(db *sql.DB,root,id string)error{
	if id==""{return errors.New("task show requires --id")}
	ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second)
	defer cancel()
	var title,status,updated string
	var version int
	if err:=db.QueryRowContext(ctx,
		"SELECT title,status,version,updated FROM arc_tasks WHERE id=?",id).
		Scan(&title,&status,&version,&updated);err!=nil{return err}
	out:=map[string]any{"id":id,"title":title,"status":status,"version":version,"updated":updated}
	if status=="done"{
		var raw string
		err:=db.QueryRowContext(ctx,
			"SELECT payload FROM arc_events WHERE aggregate='task' AND aggregate_id=? AND kind='task.completed' ORDER BY created DESC,id DESC LIMIT 1",id).Scan(&raw)
		if err!=nil && !errors.Is(err,sql.ErrNoRows){return err}
		var record struct{
			ReceiptSHA256 string `json:"receipt_sha256"`
			Head string `json:"head"`
			SourceSnapshot string `json:"source_snapshot"`
		}
		if err==nil{
			if jsonErr:=json.Unmarshal([]byte(raw),&record);jsonErr!=nil{return jsonErr}
		}
		snapshot,snapshotErr:=sourceSnapshot(root)
		if snapshotErr!=nil{return snapshotErr}
		out["completion"]=map[string]any{
			"receipt_sha256":record.ReceiptSHA256,"head":record.Head,
			"source_revision_current":record.SourceSnapshot!="" && record.SourceSnapshot==snapshot,
			"independent_proof_rechecked":false,
		}
	}
	return emitJSON(out)
}
