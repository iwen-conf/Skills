package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

var policyStoreSchema = []string{
	`CREATE TABLE IF NOT EXISTS arc_policy_state (id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL CHECK(version>=1), body TEXT NOT NULL, updated_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_policy_receipts (sha256 TEXT PRIMARY KEY, task_id TEXT NOT NULL, verdict TEXT NOT NULL, body TEXT NOT NULL, captured_at TEXT NOT NULL)`,
	`CREATE INDEX IF NOT EXISTS arc_policy_receipts_task ON arc_policy_receipts(task_id, captured_at)`,
}

// policyDocument 描述策略状态存储模式、版本号与原始JSON文档
type policyDocument struct {
	// Mode 存储引擎模式（legacy或arc.db）
	Mode string `json:"mode"`
	// Version 乐观锁版本号
	Version int64 `json:"version"`
	// State 策略状态JSON原文
	State json.RawMessage `json:"state"`
}

// policyReceipt 描述归档到 arc_policy_receipts 表的强类型收据记录
type policyReceipt struct {
	// Hash 收据规范化JSON的SHA256摘要
	Hash string
	// TaskID 所属任务唯一ID
	TaskID string
	// Verdict 收据判定结果（PASS或AUTHORIZED）
	Verdict string
	// Body 规范化收据JSON字符串
	Body string
}

// policyActiveTaskFields 用于强类型校验活跃策略任务必填标识字段
type policyActiveTaskFields struct {
	// ID 任务唯一ID
	ID string `json:"id"`
	// Slug 任务短标识
	Slug string `json:"slug"`
	// Skill 归属技能名称
	Skill string `json:"skill"`
	// Receipt 可选嵌套收据JSON
	Receipt json.RawMessage `json:"receipt"`
}

// policyReceiptHeader 用于强类型校验归档收据的身份与判定头字段
type policyReceiptHeader struct {
	// TaskID 收据内声明的任务ID
	TaskID string `json:"task_id"`
	// Verdict 收据判定标签
	Verdict string `json:"verdict"`
}

// loadPolicyDocument 读取当前激活的策略文档（未建表时安全回退 legacy 模式）
// 请求体
// db 数据库连接句柄
// 响应体
// doc 策略状态文档
// err 元数据查询或状态格式损坏错误
func loadPolicyDocument(db *sql.DB) (policyDocument, error) {
	// 1. 检测 arc_policy_state 表是否存在
	hasTable, tableErr := arcTableExists(context.Background(), db, "arc_policy_state")
	if tableErr != nil {
		return policyDocument{}, tableErr
	}
	if !hasTable {
		return policyDocument{Mode: "legacy", Version: 0, State: json.RawMessage("{}")}, nil
	}
	// 2. 读取主行数据并校验结构合法性
	var body string
	var version int64
	err := db.QueryRow("SELECT version, body FROM arc_policy_state WHERE id=1").Scan(&version, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return policyDocument{Mode: "legacy", Version: 0, State: json.RawMessage("{}")}, nil
	}
	if err != nil {
		return policyDocument{}, err
	}
	if _, err := decodePolicyState([]byte(body)); err != nil {
		return policyDocument{}, err
	}
	return policyDocument{Mode: "arc.db", Version: version, State: json.RawMessage(body)}, nil
}

// decodePolicyState 使用强类型结构体与 json.RawMessage 校验策略JSON顶层结构且保留扩展字段
// 请求体
// raw 待解析的策略状态JSON字节
// 响应体
// fields 顶层键值到规范化json.RawMessage的强类型映射
// err 超出16MiB上限、非JSON对象或必填字段缺失错误
func decodePolicyState(raw []byte) (map[string]json.RawMessage, error) {
	// 1. 校验体积上限与顶层必须为非null的JSON对象
	if len(raw) == 0 || len(raw) > 16<<20 {
		return nil, errors.New("policy state size must be 1..16 MiB")
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, errors.New("policy state must be a JSON object")
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return nil, fmt.Errorf("invalid policy JSON: %w", err)
	}
	if value == nil {
		return nil, errors.New("policy state must be a JSON object")
	}
	// 2. 强类型校验可选 history 必须为JSON数组且 active 必须包含 id/slug/skill
	if histRaw, ok := value["history"]; ok {
		ht := bytes.TrimSpace(histRaw)
		if len(ht) == 0 || ht[0] != '[' {
			return nil, errors.New("policy state history must be an array")
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(ht, &arr); err != nil {
			return nil, errors.New("policy state history must be an array")
		}
	}
	if actRaw, ok := value["active"]; ok {
		at := bytes.TrimSpace(actRaw)
		if len(at) > 0 && string(at) != "null" {
			if at[0] != '{' {
				return nil, errors.New("policy active task must be an object or null")
			}
			var item policyActiveTaskFields
			if err := json.Unmarshal(at, &item); err != nil {
				return nil, errors.New("policy active task must be an object or null")
			}
			if strings.TrimSpace(item.ID) == "" {
				return nil, errors.New("policy active task missing id")
			}
			if strings.TrimSpace(item.Slug) == "" {
				return nil, errors.New("policy active task missing slug")
			}
			if strings.TrimSpace(item.Skill) == "" {
				return nil, errors.New("policy active task missing skill")
			}
		}
	}
	return value, nil
}

// collectPolicyReceipts 从活跃任务与历史任务列表中提取去重后的强类型归档收据
// 请求体
// state 顶层策略状态RawMessage映射
// 响应体
// out 去重后的策略收据切片
// err 历史条目非对象、任务ID冲突或非法verdict错误
func collectPolicyReceipts(state map[string]json.RawMessage) ([]policyReceipt, error) {
	// 1. 收集 active 与 history 中的全部任务JSON条目
	var entries []json.RawMessage
	if actRaw, ok := state["active"]; ok {
		at := bytes.TrimSpace(actRaw)
		if len(at) > 0 && at[0] == '{' {
			entries = append(entries, at)
		}
	}
	if histRaw, ok := state["history"]; ok {
		var hist []json.RawMessage
		if err := json.Unmarshal(histRaw, &hist); err == nil {
			entries = append(entries, hist...)
		}
	}
	// 2. 逐条使用强类型结构体校验任务归属与收据合法性并做确定性哈希去重
	unique := map[string]policyReceipt{}
	for _, rawTask := range entries {
		tt := bytes.TrimSpace(rawTask)
		if len(tt) == 0 || tt[0] != '{' {
			return nil, errors.New("policy history entry must be an object")
		}
		var task policyActiveTaskFields
		if err := json.Unmarshal(tt, &task); err != nil {
			return nil, errors.New("policy history entry must be an object")
		}
		rt := bytes.TrimSpace(task.Receipt)
		if len(rt) == 0 || string(rt) == "null" {
			continue
		}
		if rt[0] != '{' {
			return nil, errors.New("policy receipt must be an object")
		}
		if task.ID == "" {
			return nil, errors.New("policy receipt task lacks ID")
		}
		var hdr policyReceiptHeader
		if err := json.Unmarshal(rt, &hdr); err != nil {
			return nil, errors.New("policy receipt must be an object")
		}
		if hdr.TaskID != "" && hdr.TaskID != task.ID {
			return nil, errors.New("policy receipt and owner task IDs disagree")
		}
		if hdr.Verdict != string(VerdictPass) && hdr.Verdict != "AUTHORIZED" {
			return nil, fmt.Errorf("invalid archived policy receipt verdict %q", hdr.Verdict)
		}
		canonical, err := canonicalizeJSONBytes(rt)
		if err != nil {
			return nil, err
		}
		id := shaBytes(canonical)
		unique[id] = policyReceipt{Hash: id, TaskID: task.ID, Verdict: hdr.Verdict, Body: string(canonical)}
	}
	out := make([]policyReceipt, 0, len(unique))
	for _, v := range unique {
		out = append(out, v)
	}
	return out, nil
}

// persistPolicyReceipts 在事务内幂等批量写入策略收据归档记录
// 请求体
// ctx 请求上下文
// tx 当前数据库事务句柄
// items 待持久化的策略收据列表
// 响应体
// err SQL写入错误
func persistPolicyReceipts(ctx context.Context, tx *sql.Tx, items []policyReceipt) error {
	// 1. 逐条执行 INSERT OR IGNORE 写入不可变收据归档表
	for _, receipt := range items {
		if _, err := tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO arc_policy_receipts(sha256,task_id,verdict,body,captured_at) VALUES(?,?,?,?,?)",
			receipt.Hash, receipt.TaskID, receipt.Verdict, receipt.Body, nowUTC()); err != nil {
			return err
		}
	}
	return nil
}

// policyStateCommand 执行 arc policy state get/put/status/migrate 命令
// 请求体
// args 子命令参数切片
// 响应体
// err 参数错误、CAS版本冲突或迁移失败错误
func policyStateCommand(args []string) error {
	// 1. 解析动作名称与命令行标志并确认当前仓库已激活 arc.db
	if len(args) == 0 {
		return errors.New("usage: arc policy state <get|put|status|migrate> [--repo DIR] [--expected-version N] [--confirm-offline]")
	}
	action := args[0]
	fs := flag.NewFlagSet("arc policy state "+action, flag.ContinueOnError)
	repoFlag := fs.String("repo", ".", "repository root")
	expected := fs.Int64("expected-version", 0, "CAS expected store version")
	confirm := fs.Bool("confirm-offline", false, "explicit offline migration of legacy policy JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	root, err := repositoryRoot(*repoFlag)
	if err != nil {
		return err
	}
	if _, err := readManifest(root); err != nil {
		return err
	}
	// 2. 按 get/status/put/migrate 执行对应状态读写或离线迁移
	switch action {
	case "get", "status":
		return withArcLock(root, func() error {
			db, oErr := openActiveArcDB(root)
			if oErr != nil {
				return oErr
			}
			defer db.Close()
			doc, lErr := loadPolicyDocument(db)
			if lErr != nil {
				return lErr
			}
			if action == "status" {
				return emitRawJSON(json.Marshal(policyVersionResponse{Mode: doc.Mode, Version: doc.Version}))
			}
			return emitRawJSON(json.Marshal(doc))
		})
	case "put":
		if *expected < 1 {
			return errors.New("policy state put requires --expected-version >=1")
		}
		raw, rErr := io.ReadAll(io.LimitReader(os.Stdin, (16<<20)+1))
		if rErr != nil {
			return rErr
		}
		state, dErr := decodePolicyState(raw)
		if dErr != nil {
			return dErr
		}
		receipts, cErr := collectPolicyReceipts(state)
		if cErr != nil {
			return cErr
		}
		body, nErr := canonicalizeJSONBytes(raw)
		if nErr != nil {
			return nErr
		}
		return withArcLock(root, func() error {
			db, oErr := openActiveArcDB(root)
			if oErr != nil {
				return oErr
			}
			defer db.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
			defer cancel()
			var next int64
			txErr := dbTransaction(ctx, db, func(tx *sql.Tx) error {
				res, execErr := tx.ExecContext(ctx,
					"UPDATE arc_policy_state SET body=?,version=version+1,updated_at=? WHERE id=1 AND version=?",
					string(body), nowUTC(), *expected)
				if execErr != nil {
					return execErr
				}
				n, aErr := res.RowsAffected()
				if aErr != nil {
					return aErr
				}
				if n != 1 {
					return fmt.Errorf("policy state version conflict (expected %d); reload before writing", *expected)
				}
				if pErr := persistPolicyReceipts(ctx, tx, receipts); pErr != nil {
					return pErr
				}
				rawEv, mErr := json.Marshal(policyUpdatedEventPayload{Version: *expected + 1, SHA256: shaBytes(body)})
				if evErr := addEventPayload(ctx, tx, "policy", "state", "policy.state.updated", rawEv, mErr); evErr != nil {
					return evErr
				}
				next = *expected + 1
				return nil
			})
			if txErr != nil {
				return txErr
			}
			return emitRawJSON(json.Marshal(policyVersionResponse{Mode: "arc.db", Version: next}))
		})
	case "migrate":
		if !*confirm {
			return errors.New("policy migration requires --confirm-offline and no concurrent legacy policy writers")
		}
		return withPolicyTaskLock(root, func() error {
			return withArcLock(root, func() error {
				return migratePolicyState(root)
			})
		})
	default:
		return fmt.Errorf("unknown arc policy state action %q", action)
	}
}
