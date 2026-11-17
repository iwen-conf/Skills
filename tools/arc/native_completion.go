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
	"strings"
	"time"
)

// A completion link is a declared obligation, not evidence that it passed.
// Updated participates in the digest so relinking during validation invalidates it.
type nativeTaskLink struct {
	Graph        string `json:"graph"`
	ObligationID string `json:"obligation_id"`
	VerifyTaskID string `json:"verify_task_id"`
	Updated      string `json:"updated"`
}

type nativeProofContext struct {
	Graph        string `json:"graph"`
	Environment  string `json:"environment"`
	Target       string `json:"target"`
	SourceCommit string `json:"source_commit"`
	SourceSHA256 string `json:"source_sha256"`
	ConfigSHA256 string `json:"config_sha256"`
	GraphSHA256  string `json:"graph_sha256"`
	SourceRoot   string `json:"source_root"`
	ConfigPath   string `json:"config_path"`
}

type nativeTaskProof struct {
	Version         int                `json:"version"`
	Graph           string             `json:"graph"`
	Task            string             `json:"task"`
	ObligationID    string             `json:"obligation_id"`
	Run             string             `json:"run"`
	Round           string             `json:"round"`
	Verdict         string             `json:"verdict"`
	Reason          string             `json:"reason"`
	ProofScope      string             `json:"proof_scope"`
	InputsUnchanged bool               `json:"inputs_unchanged"`
	EvidenceStatus  string             `json:"evidence_status"`
	Context         nativeProofContext `json:"context"`
	RAWSHA256       []string           `json:"raw_sha256"`
	ReceiptSHA256   string             `json:"receipt_sha256"`
}

type completionQuery interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func completionLinks(ctx context.Context, query completionQuery, taskID string) ([]nativeTaskLink, string, error) {
	rows, err := query.QueryContext(ctx, "SELECT graph,obligation_id,verify_task_id,updated FROM arc_task_obligations WHERE arc_task_id=? ORDER BY graph,obligation_id", taskID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	links := []nativeTaskLink{}
	for rows.Next() {
		var link nativeTaskLink
		if err := rows.Scan(&link.Graph, &link.ObligationID, &link.VerifyTaskID, &link.Updated); err != nil {
			return nil, "", err
		}
		if link.Graph == "" || link.ObligationID == "" {
			return nil, "", errors.New("linked verification obligation has no identity")
		}
		if link.VerifyTaskID == "" {
			link.VerifyTaskID = link.Graph + "/" + link.ObligationID
		}
		links = append(links, link)
		if len(links) > 1024 {
			return nil, "", errors.New("task completion exceeds 1024 linked obligations; split the task scope")
		}
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	if len(links) == 0 {
		return nil, "", errors.New("task has no linked verification obligations; use arc task link-verify before completion (legacy policy receipts do not authorize new completion)")
	}
	body, err := json.Marshal(links)
	if err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(body)
	return links, hex.EncodeToString(sum[:]), nil
}

func validEvidenceSHA(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// The child rechecks RAW and frozen execution context. The parent binds that
// answer to this exact linked obligation and repository; a SQL status is never a proof.
func acceptNativeTaskProof(root, head string, link nativeTaskLink, body []byte) (nativeTaskProof, string, error) {
	var proof nativeTaskProof
	if err := json.Unmarshal(body, &proof); err != nil {
		return proof, "", fmt.Errorf("task-proof returned invalid JSON: %w", err)
	}
	if proof.Version != 1 || proof.Graph != link.Graph || proof.Task != link.VerifyTaskID || proof.ObligationID != link.ObligationID {
		return proof, "", errors.New("task-proof identity differs from the linked graph/task/obligation")
	}
	if proof.Verdict != "PASS" || proof.ProofScope != "linked_task_evidence" || !proof.InputsUnchanged || proof.EvidenceStatus != "PASS" {
		return proof, "", fmt.Errorf("linked obligation %s has no current independently revalidated PASS: %s %s", link.ObligationID, proof.Verdict, proof.Reason)
	}
	if proof.Run == "" || proof.Round == "" || proof.Context.Graph != link.Graph || proof.Context.Environment == "" || proof.Context.Target == "" || proof.Context.SourceCommit != head {
		return proof, "", errors.New("task-proof lacks the current graph/source/environment/run context")
	}
	resolved, err := filepath.EvalSymlinks(proof.Context.SourceRoot)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(root) {
		return proof, "", errors.New("task-proof source root is not the task repository")
	}
	for _, value := range []string{proof.Context.SourceSHA256, proof.Context.ConfigSHA256, proof.Context.GraphSHA256, proof.ReceiptSHA256} {
		if !validEvidenceSHA(value) {
			return proof, "", errors.New("task-proof has a missing or invalid source/config/model/receipt fingerprint")
		}
	}
	if len(proof.RAWSHA256) == 0 {
		return proof, "", errors.New("task-proof has no independently validated RAW fingerprints")
	}
	seen := map[string]bool{}
	for _, value := range proof.RAWSHA256 {
		if !validEvidenceSHA(value) || seen[value] {
			return proof, "", errors.New("task-proof has invalid or duplicate RAW fingerprints")
		}
		seen[value] = true
	}
	// Preserve and fingerprint the full structured answer (including future
	// evidence details); the engine receipt hash has its own canonical definition.
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		return proof, "", err
	}
	canonical, err := json.Marshal(document)
	if err != nil {
		return proof, "", err
	}
	sum := sha256.Sum256(canonical)
	return proof, hex.EncodeToString(sum[:]), nil
}

type nativeProofRunner func(context.Context, string, nativeTaskLink) ([]byte, error)

func runNativeTaskProof(ctx context.Context, root string, link nativeTaskLink) ([]byte, error) {
	bin, err := resolveBackend("arc-verify")
	if err != nil {
		return nil, fmt.Errorf("native verification engine unavailable: %w", err)
	}
	// Call the backend directly while the public arc process holds arc.lock.
	// Calling `arc verify` here would recursively acquire the same lock.
	cmd := exec.CommandContext(ctx, bin, "task-proof", "--db", statePath(root), "--graph", link.Graph, "--task", link.VerifyTaskID)
	cmd.Dir = root
	body, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("task-proof deadline: %w", ctx.Err())
		}
		if exit, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("linked obligation %s failed native proof verification (exit %d): %s", link.ObligationID, exit.ExitCode(), strings.TrimSpace(string(body)))
		}
		return nil, fmt.Errorf("native task-proof unavailable: %w", err)
	}
	return body, nil
}

// Caller holds arc.lock through this revalidation and commitTaskCompletion.
// No tests are launched, no policy plugin is invoked, and failure never falls back.
func loadCompletionProof(root, taskID string) (verifiedTaskProof, error) {
	return loadNativeCompletionProof(root, taskID, runNativeTaskProof)
}

func loadNativeCompletionProof(root, taskID string, runner nativeProofRunner) (verifiedTaskProof, error) {
	before, err := sourceSnapshot(root)
	if err != nil {
		return verifiedTaskProof{}, err
	}
	head, err := gitHead(root)
	if err != nil {
		return verifiedTaskProof{}, err
	}
	db, err := openActiveArcDB(root)
	if err != nil {
		return verifiedTaskProof{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	links, linksHash, err := completionLinks(ctx, db, taskID)
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return verifiedTaskProof{}, err
	}
	checks := []map[string]any{}
	proofs := []nativeTaskProof{}
	for _, link := range links {
		body, err := runner(ctx, root, link)
		if err != nil {
			return verifiedTaskProof{}, err
		}
		proof, hash, err := acceptNativeTaskProof(root, head, link, body)
		if err != nil {
			return verifiedTaskProof{}, err
		}
		proofs = append(proofs, proof)
		checks = append(checks, map[string]any{"id": "linked_task_evidence", "graph": link.Graph, "obligation_id": link.ObligationID, "task": link.VerifyTaskID, "run": proof.Run, "round": proof.Round, "verdict": "PASS", "proof_sha256": hash, "proof": json.RawMessage(body)})
	}
	after, err := sourceSnapshot(root)
	if err != nil {
		return verifiedTaskProof{}, err
	}
	if before != after {
		return verifiedTaskProof{}, errors.New("source changed while native task evidence was revalidated")
	}
	body, err := json.Marshal(map[string]any{"task_id": taskID, "source_snapshot": before, "links_sha256": linksHash, "checks": checks})
	if err != nil {
		return verifiedTaskProof{}, err
	}
	sum := sha256.Sum256(body)
	return verifiedTaskProof{TaskID: taskID, ReceiptSHA256: hex.EncodeToString(sum[:]), ReceiptHead: head, ReceiptDigest: before, VerifiedAt: nowUTC(), SourceSnapshot: before, Checks: checks, Native: true, LinksSHA256: linksHash, NativeProofs: proofs}, nil
}
