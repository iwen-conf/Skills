package main

import (
	"context"
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

// nativeTaskLink 描述本地任务声明绑定的单条验证义务记录
type nativeTaskLink struct {
	// Graph 验证图或故事线ID
	Graph string `json:"graph"`
	// ObligationID 验证义务或路由ID
	ObligationID string `json:"obligation_id"`
	// VerifyTaskID 验证侧任务完整ID
	VerifyTaskID string `json:"verify_task_id"`
	// Updated 绑定更新时间戳
	Updated string `json:"updated"`
}

// nativeProofContext 描述原生任务证明绑定的环境与源码指纹上下文
type nativeProofContext struct {
	// Graph 验证图ID
	Graph string `json:"graph"`
	// Environment 运行环境或命名空间
	Environment string `json:"environment"`
	// Target 核心目标实体标识
	Target string `json:"target"`
	// SourceCommit 源码Git提交哈希
	SourceCommit string `json:"source_commit"`
	// SourceSHA256 源码关联指纹摘要
	SourceSHA256 string `json:"source_sha256"`
	// ConfigSHA256 配置SHA256摘要
	ConfigSHA256 string `json:"config_sha256"`
	// GraphSHA256 图定义SHA256摘要
	GraphSHA256 string `json:"graph_sha256"`
	// SourceRoot 仓库根目录真实路径
	SourceRoot string `json:"source_root"`
	// ConfigPath 配置文件路径
	ConfigPath string `json:"config_path"`
}

// nativeTaskProof 描述 arc-verify task-proof 返回的强类型原生验证证明结构
type nativeTaskProof struct {
	// Version 协议版本号
	Version int `json:"version"`
	// Graph 验证图ID
	Graph string `json:"graph"`
	// Task 验证任务完整ID
	Task string `json:"task"`
	// ObligationID 验证义务ID
	ObligationID string `json:"obligation_id"`
	// Run 运行批次ID
	Run string `json:"run"`
	// Round 起始检查点标识
	Round string `json:"round"`
	// Verdict 证明判定结果
	Verdict string `json:"verdict"`
	// Reason 原因说明
	Reason string `json:"reason"`
	// ProofScope 证明范围标识
	ProofScope string `json:"proof_scope"`
	// InputsUnchanged 输入未变更标志
	InputsUnchanged bool `json:"inputs_unchanged"`
	// EvidenceStatus 证据重验状态
	EvidenceStatus string `json:"evidence_status"`
	// Context 环境与源码指纹上下文
	Context nativeProofContext `json:"context"`
	// RAWSHA256 原始收据与步骤轨迹哈希列表
	RAWSHA256 []string `json:"raw_sha256"`
	// ReceiptSHA256 路由收据SHA256摘要
	ReceiptSHA256 string `json:"receipt_sha256"`
}

type nativeProofRunner func(context.Context, string, nativeTaskLink) ([]byte, error)

// completionLinksFromDB 从数据库连接读取任务绑定的全部验证义务及列表指纹
// 请求体
// ctx 请求上下文
// db 数据库连接句柄
// taskID 本地任务ID
// 响应体
// links 验证义务切片
// digest 绑定列表SHA256摘要
// err 未绑定义务或查询失败错误
func completionLinksFromDB(ctx context.Context, db *sql.DB, taskID string) ([]nativeTaskLink, string, error) {
	// 1. 执行数据库级绑定记录查询并交由统一行扫描器解析
	rows, err := db.QueryContext(ctx, "SELECT graph,obligation_id,verify_task_id,updated FROM arc_task_obligations WHERE arc_task_id=? ORDER BY graph,obligation_id", taskID)
	if err != nil {
		return nil, "", err
	}
	return scanCompletionLinkRows(rows)
}

// completionLinksFromTx 在显式事务内读取任务绑定的全部验证义务及列表指纹
// 请求体
// ctx 请求上下文
// tx 当前数据库事务句柄
// taskID 本地任务ID
// 响应体
// links 验证义务切片
// digest 绑定列表SHA256摘要
// err 未绑定义务或查询失败错误
func completionLinksFromTx(ctx context.Context, tx *sql.Tx, taskID string) ([]nativeTaskLink, string, error) {
	// 1. 执行事务内绑定记录查询并交由统一行扫描器解析
	rows, err := tx.QueryContext(ctx, "SELECT graph,obligation_id,verify_task_id,updated FROM arc_task_obligations WHERE arc_task_id=? ORDER BY graph,obligation_id", taskID)
	if err != nil {
		return nil, "", err
	}
	return scanCompletionLinkRows(rows)
}

// scanCompletionLinkRows 扫描验证义务行集合并计算不可变列表指纹
// 请求体
// rows SQL查询结果集句柄
// 响应体
// links 验证义务切片
// digest 绑定列表SHA256摘要
// err 空列表、超上限或序列化错误
func scanCompletionLinkRows(rows *sql.Rows) ([]nativeTaskLink, string, error) {
	// 1. 逐行扫描验证义务记录并校验非空与1024条上限
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
	// 2. 序列化关联列表并计算SHA256摘要返回
	body, err := json.Marshal(links)
	if err != nil {
		return nil, "", err
	}
	return links, shaBytes(body), nil
}

// validEvidenceSHA 校验字符串是否为合法64位十六进制SHA256摘要
// 请求体
// value 待校验哈希字符串
// 响应体
// ok 是否为合法64位十六进制字符串
func validEvidenceSHA(value string) bool {
	// 1. 校验长度等于64且可解码为十六进制字节
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// acceptNativeTaskProof 严格核验子引擎返回的原生任务证明与当前仓库提交及义务标识一致
// 请求体
// root 仓库根目录真实路径
// head 当前Git提交哈希
// link 声明的验证义务记录
// body 子引擎输出的原始JSON字节
// 响应体
// proof 解析通过的强类型证明对象
// digest 规范化完整证明JSON的SHA256摘要
// err 身份不匹配、非PASS或指纹缺失错误
func acceptNativeTaskProof(root, head string, link nativeTaskLink, body []byte) (nativeTaskProof, string, error) {
	// 1. 反序列化强类型证明结构并核对协议版本、身份、PASS判定与证明范围
	var proof nativeTaskProof
	if err := json.Unmarshal(body, &proof); err != nil {
		return proof, "", fmt.Errorf("task-proof returned invalid JSON: %w", err)
	}
	if proof.Version != 1 || proof.Graph != link.Graph || proof.Task != link.VerifyTaskID || proof.ObligationID != link.ObligationID {
		return proof, "", errors.New("task-proof identity differs from the linked graph/task/obligation")
	}
	if Verdict(proof.Verdict) != VerdictPass || proof.ProofScope != "linked_task_evidence" || !proof.InputsUnchanged || Verdict(proof.EvidenceStatus) != VerdictPass {
		return proof, "", fmt.Errorf("linked obligation %s has no current independently revalidated PASS: %s %s", link.ObligationID, proof.Verdict, proof.Reason)
	}
	// 2. 核对运行批次、提交哈希、真实根目录路径及全部SHA256指纹合法无重复
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
	// 3. 使用强类型递归规范化器消除JSON空白格式差异并计算完整证明摘要
	canonical, err := canonicalizeJSONBytes(body)
	if err != nil {
		return proof, "", err
	}
	return proof, shaBytes(canonical), nil
}

// runNativeTaskProof 直接调用私有的 arc-verify 子引擎执行单条验证义务证明重验
// 请求体
// ctx 超时控制上下文
// root 仓库根目录路径
// link 目标验证义务记录
// 响应体
// body 子引擎输出JSON字节
// err 子引擎缺失、超时或验证未通过错误
func runNativeTaskProof(ctx context.Context, root string, link nativeTaskLink) ([]byte, error) {
	// 1. 解析 arc-verify 二进制并在当前持有锁状态下直接传入 --db 调用
	bin, err := resolveBackend("arc-verify")
	if err != nil {
		return nil, fmt.Errorf("native verification engine unavailable: %w", err)
	}
	cmd := exec.CommandContext(ctx, bin, "task-proof", "--db", statePath(root), "--graph", link.Graph, "--task", link.VerifyTaskID)
	cmd.Dir = root
	body, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("task-proof deadline: %w", ctx.Err())
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, fmt.Errorf("linked obligation %s failed native proof verification (exit %d): %s", link.ObligationID, exit.ExitCode(), strings.TrimSpace(string(body)))
		}
		return nil, fmt.Errorf("native task-proof unavailable: %w", err)
	}
	return body, nil
}

// loadCompletionProof 加载并重验指定任务的全部原生关联验证义务证明
// 请求体
// root 仓库根目录路径
// taskID 目标任务ID
// 响应体
// proof 汇总后的完成证明结构
// err 未绑定义务、重验失败或并发源码变动错误
func loadCompletionProof(root, taskID string) (verifiedTaskProof, error) {
	// 1. 委托默认子引擎执行器加载原生完成证明
	return loadNativeCompletionProof(root, taskID, runNativeTaskProof)
}

// loadNativeCompletionProof 通过可注入执行器重验全部关联验证义务并检测重验期间源码竞态
// 请求体
// root 仓库根目录路径
// taskID 目标任务ID
// runner 原生证明执行器函数
// 响应体
// proof 汇总后的完成证明结构
// err 任一义务未通过或重验期间源码被修改错误
func loadNativeCompletionProof(root, taskID string, runner nativeProofRunner) (verifiedTaskProof, error) {
	// 1. 采集前置源码快照、当前Git提交哈希与全部已绑定义务列表
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
	links, linksHash, err := completionLinksFromDB(ctx, db, taskID)
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return verifiedTaskProof{}, err
	}
	// 2. 逐条调用验证器重验并组装强类型 policyCheckItem 列表
	checks := make([]policyCheckItem, 0, len(links))
	proofs := make([]nativeTaskProof, 0, len(links))
	for _, link := range links {
		body, runErr := runner(ctx, root, link)
		if runErr != nil {
			return verifiedTaskProof{}, runErr
		}
		proof, hash, accErr := acceptNativeTaskProof(root, head, link, body)
		if accErr != nil {
			return verifiedTaskProof{}, accErr
		}
		proofs = append(proofs, proof)
		checks = append(checks, policyCheckItem{
			ID:           "linked_task_evidence",
			Graph:        link.Graph,
			ObligationID: link.ObligationID,
			Task:         link.VerifyTaskID,
			Run:          proof.Run,
			Round:        proof.Round,
			Verdict:      string(VerdictPass),
			ProofSHA256:  hash,
			Proof:        json.RawMessage(body),
		})
	}
	// 3. 复核重验前后源码快照未发生竞态篡改并生成汇总收据指纹
	after, err := sourceSnapshot(root)
	if err != nil {
		return verifiedTaskProof{}, err
	}
	if before != after {
		return verifiedTaskProof{}, errors.New("source changed while native task evidence was revalidated")
	}
	body, err := json.Marshal(nativeCompletionDigestPayload{
		TaskID:         taskID,
		SourceSnapshot: before,
		LinksSHA256:    linksHash,
		Checks:         checks,
	})
	if err != nil {
		return verifiedTaskProof{}, err
	}
	return verifiedTaskProof{
		TaskID:         taskID,
		ReceiptSHA256:  shaBytes(body),
		ReceiptHead:    head,
		ReceiptDigest:  before,
		VerifiedAt:     nowUTC(),
		SourceSnapshot: before,
		Checks:         checks,
		Native:         true,
		LinksSHA256:    linksHash,
		NativeProofs:   proofs,
	}, nil
}
