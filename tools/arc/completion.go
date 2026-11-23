package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"
)

// Historical policy receipts remain readable for migration and audit. They do
// not authorize new completion; native linked evidence is mandatory below.
type policyTaskStatus struct {
	RepoRoot  string `json:"repo_root"`
	ReceiptOK bool   `json:"receipt_ok"`
	Tree      struct {
		Head   string   `json:"head"`
		Digest string   `json:"digest"`
		Dirty  []string `json:"dirty"`
	} `json:"tree"`
	Task struct {
		ID      string          `json:"id"`
		Receipt json.RawMessage `json:"receipt"`
	} `json:"task"`
}

type verifiedTaskProof struct {
	TaskID         string
	ReceiptSHA256  string
	ReceiptHead    string
	ReceiptDigest  string
	ReceiptTree    string
	VerifiedAt     string
	SourceSnapshot string
	Checks         []policyCheckItem
	Native         bool
	LinksSHA256    string
	NativeProofs   []nativeTaskProof
}

// requirePolicyProof 校验历史策略收据的强类型修订绑定与测试检查项完整性
// 请求体
// taskID 目标任务唯一ID
// status 策略状态快照结构
// 响应体
// proof 校验通过的完成证明结构
// err 收据缺失、豁免伪造或工作树漂移错误
func requirePolicyProof(taskID string, status policyTaskStatus) (verifiedTaskProof, error) {
	// 1. 校验收据归属任务ID与有效标志并反序列化强类型收据体
	if !status.ReceiptOK || status.Task.ID != taskID || taskID == "" {
		return verifiedTaskProof{}, errors.New("missing current, matching Arc Policy verification receipt")
	}
	var receipt struct {
		TaskID    string            `json:"task_id"`
		Verdict   string            `json:"verdict"`
		CreatedAt string            `json:"created_at"`
		Head      string            `json:"head"`
		Digest    string            `json:"digest"`
		Tree      string            `json:"tree"`
		Dirty     []string          `json:"dirty"`
		Checks    []policyCheckItem `json:"checks"`
	}
	if err := json.Unmarshal(status.Task.Receipt, &receipt); err != nil {
		return verifiedTaskProof{}, fmt.Errorf("invalid policy receipt: %w", err)
	}
	if Verdict(receipt.Verdict) != VerdictPass || receipt.TaskID != taskID ||
		receipt.Head == "" || receipt.Digest == "" || receipt.Tree == "" ||
		receipt.CreatedAt == "" {
		return verifiedTaskProof{}, errors.New("task completion requires a genuine revision-bound PASS, not authorization or absent evidence")
	}
	// 2. 核对快照、CLI收据及独立测试检查项全部为PASS且工作树一致
	snapshotOK, receiptOK, independentOK := false, false, false
	for _, check := range receipt.Checks {
		switch check.ID {
		case "snapshot":
			snapshotOK = Verdict(check.Verdict) == VerdictPass
		case "cli_receipt":
			receiptOK = Verdict(check.Verdict) == VerdictPass
		case "tests", "business_graph":
			if Verdict(check.Verdict) == VerdictPass {
				independentOK = true
			}
		}
	}
	if !snapshotOK || !receiptOK || !independentOK {
		return verifiedTaskProof{}, errors.New("policy PASS lacks successful snapshot, receipt, or independent tests/business graph evidence")
	}
	if status.Tree.Head != receipt.Head || status.Tree.Digest != receipt.Digest ||
		!reflect.DeepEqual(status.Tree.Dirty, receipt.Dirty) {
		return verifiedTaskProof{}, errors.New("policy receipt fingerprint differs from current tree reported by Arc Policy")
	}
	// 3. 计算规范化JSON收据SHA256摘要并返回
	sum, err := canonicalPolicyReceiptHash(status.Task.Receipt)
	if err != nil {
		return verifiedTaskProof{}, err
	}
	return verifiedTaskProof{
		TaskID:        taskID,
		ReceiptSHA256: sum,
		ReceiptHead:   receipt.Head,
		ReceiptDigest: receipt.Digest,
		ReceiptTree:   receipt.Tree,
		VerifiedAt:    receipt.CreatedAt,
		Checks:        receipt.Checks,
	}, nil
}

// canonicalPolicyReceiptHash 对非空非null的收据JSON执行确定性键序规范化并计算SHA256摘要
// 请求体
// raw 原始收据JSON字节切片
// 响应体
// digest 64位十六进制SHA256摘要
// err 空收据、null收据或非法JSON错误
func canonicalPolicyReceiptHash(raw []byte) (string, error) {
	// 1. 拦截空白或字面量 null 收据
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return "", errors.New("empty policy receipt")
	}
	// 2. 规范化键序后计算SHA256摘要
	canonical, err := canonicalizeJSONBytes(trimmed)
	if err != nil {
		return "", err
	}
	return shaBytes(canonical), nil
}

// validateLivePolicyReceipt 核验实时策略存储中的活跃任务收据未被撤销或替换
// 请求体
// root 仓库根目录路径
// proof 待核验的完成证明结构
// 响应体
// err 收据被撤销、任务切换或哈希变动错误
func validateLivePolicyReceipt(root string, proof verifiedTaskProof) error {
	// 1. 优先读取已迁移的 arc.db 策略行，否则读取旧版 .arc/policy/state.json
	db, err := openActiveArcDB(root)
	if err != nil {
		return err
	}
	doc, err := loadPolicyDocument(db)
	if e := db.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	var raw []byte
	if doc.Mode == "arc.db" {
		raw = doc.State
	} else {
		path := filepath.Join(root, ".arc", "policy", "state.json")
		st, statErr := os.Lstat(path)
		if statErr != nil {
			return fmt.Errorf("policy state unavailable: %w", statErr)
		}
		if !st.Mode().IsRegular() {
			return errors.New("unsafe policy task state file")
		}
		raw, err = os.ReadFile(path)
		if err != nil {
			return err
		}
	}
	// 2. 反序列化当前活跃任务并核对收据SHA256一致
	var state struct {
		Active struct {
			ID      string          `json:"id"`
			Receipt json.RawMessage `json:"receipt"`
		} `json:"active"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return fmt.Errorf("invalid current policy state: %w", err)
	}
	if state.Active.ID != proof.TaskID {
		return errors.New("policy task changed during completion")
	}
	hash, err := canonicalPolicyReceiptHash(state.Active.Receipt)
	if err != nil {
		return fmt.Errorf("policy receipt revoked: %w", err)
	}
	if hash != proof.ReceiptSHA256 {
		return errors.New("policy receipt changed during completion")
	}
	return nil
}

// commitTaskCompletion 在检查索引队列清空与源码快照匹配后以CAS原子提交任务完成态
// 请求体
// db 数据库连接句柄
// root 仓库根目录路径
// id 任务唯一ID
// expected 期望当前版本号
// proof 已核验通过的原生完成证明结构
// 响应体
// err 源码漂移、队列未清空、CAS冲突或提交失败错误
func commitTaskCompletion(db *sql.DB, root, id string, expected int, proof verifiedTaskProof) error {
	// 1. 强制要求原生关联验证证明并核对当前工作区源码快照未发生变动
	if id == "" || expected < 1 || proof.TaskID != id || proof.ReceiptSHA256 == "" || !proof.Native || proof.LinksSHA256 == "" || len(proof.NativeProofs) == 0 {
		return errors.New("task completion requires ID, expected version, and native linked PASS evidence; historical policy receipts are audit-only")
	}
	current, err := sourceSnapshot(root)
	if err != nil {
		return err
	}
	if proof.SourceSnapshot != current {
		return errors.New("worktree changed since the verified receipt; retry task completion")
	}
	// 2. 在显式事务内校验任务版本、关联义务集合、索引队列清零及索引快照新鲜度
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	err = dbTransaction(ctx, db, func(tx *sql.Tx) error {
		var old string
		var version int
		if qErr := tx.QueryRowContext(ctx, "SELECT status,version FROM arc_tasks WHERE id=?", id).Scan(&old, &version); qErr != nil {
			return qErr
		}
		if version != expected {
			return fmt.Errorf("task version conflict: expected %d actual %d", expected, version)
		}
		if TaskStatus(old) != TaskStatusVerifying {
			return fmt.Errorf("task must be verifying before done, currently %s", old)
		}
		links, currentLinksHash, lErr := completionLinksFromTx(ctx, tx, id)
		if lErr != nil {
			return lErr
		}
		if currentLinksHash != proof.LinksSHA256 || len(links) != len(proof.NativeProofs) {
			return errors.New("verification obligations changed during completion; revalidate the current linked scope")
		}
		var pending int
		if qErr := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM arc_outbox o JOIN arc_events e ON e.id=o.event_id WHERE o.status='pending' AND e.aggregate IN ('file','change')").Scan(&pending); qErr != nil {
			return qErr
		}
		if pending != 0 {
			return fmt.Errorf("%d outbox events unacknowledged; run arc index flush first", pending)
		}
		var indexedSnapshot string
		if qErr := tx.QueryRowContext(ctx, "SELECT value FROM arc_meta WHERE key='index_last_snapshot'").Scan(&indexedSnapshot); qErr != nil {
			if errors.Is(qErr, sql.ErrNoRows) {
				return errors.New("no confirmed code index; run arc index flush first")
			}
			return qErr
		}
		if indexedSnapshot == "" || indexedSnapshot != proof.SourceSnapshot {
			return errors.New("code index snapshot is stale for this verified tree; run arc index flush")
		}
		// 3. 执行CAS状态更新并追加强类型不可变完成事件
		res, execErr := tx.ExecContext(ctx,
			"UPDATE arc_tasks SET status='done',version=version+1,updated=? WHERE id=? AND version=? AND status='verifying'",
			nowUTC(), id, expected)
		if execErr != nil {
			return execErr
		}
		affected, aErr := res.RowsAffected()
		if aErr != nil || affected != 1 {
			return errors.New("concurrent task change; completion not committed")
		}
		rawEv, mErr := json.Marshal(taskCompletedEventPayload{
			TaskID:         id,
			FromVersion:    version,
			ReceiptSHA256:  proof.ReceiptSHA256,
			Head:           proof.ReceiptHead,
			Tree:           proof.ReceiptTree,
			Digest:         proof.ReceiptDigest,
			VerifiedAt:     proof.VerifiedAt,
			SourceSnapshot: proof.SourceSnapshot,
			Checks:         proof.Checks,
			ProofKind:      "native_linked_task_evidence",
			LinksSHA256:    proof.LinksSHA256,
			Links:          links,
		})
		return addEventPayload(ctx, tx, "task", id, "task.completed", rawEv, mErr)
	})
	if err != nil {
		return err
	}
	// 4. 输出具名完成状态响应
	return emitRawJSON(json.Marshal(taskMutationResponse{
		ID:            id,
		Status:        string(TaskStatusDone),
		Version:       expected + 1,
		ReceiptSHA256: proof.ReceiptSHA256,
	}))
}

// showTaskWithProof 查询任务详情并在已完成时对比不可变完成事件与当前工作区快照
// 请求体
// db 数据库连接句柄
// root 仓库根目录路径
// id 任务唯一ID
// 响应体
// err 任务不存在或快照计算失败错误
func showTaskWithProof(db *sql.DB, root, id string) error {
	// 1. 查询任务基础元数据
	if id == "" {
		return errors.New("task show requires --id")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var title, status, updated string
	var version int
	if err := db.QueryRowContext(ctx,
		"SELECT title,status,version,updated FROM arc_tasks WHERE id=?", id).
		Scan(&title, &status, &version, &updated); err != nil {
		return err
	}
	out := taskShowResponse{ID: id, Title: title, Status: status, Version: version, Updated: updated}
	// 2. 若处于done状态则读取最新完成事件比对当前源码快照一致性
	if TaskStatus(status) == TaskStatusDone {
		var raw string
		err := db.QueryRowContext(ctx,
			"SELECT payload FROM arc_events WHERE aggregate='task' AND aggregate_id=? AND kind='task.completed' ORDER BY created DESC,id DESC LIMIT 1", id).Scan(&raw)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var record struct {
			ReceiptSHA256  string `json:"receipt_sha256"`
			Head           string `json:"head"`
			SourceSnapshot string `json:"source_snapshot"`
		}
		if err == nil {
			if jsonErr := json.Unmarshal([]byte(raw), &record); jsonErr != nil {
				return jsonErr
			}
		}
		snapshot, snapshotErr := sourceSnapshot(root)
		if snapshotErr != nil {
			return snapshotErr
		}
		out.Completion = &taskCompletionSummary{
			ReceiptSHA256:             record.ReceiptSHA256,
			Head:                      record.Head,
			SourceRevisionCurrent:     record.SourceSnapshot != "" && record.SourceSnapshot == snapshot,
			IndependentProofRechecked: false,
		}
	}
	// 3. 输出具名任务详情响应
	return emitRawJSON(json.Marshal(out))
}
