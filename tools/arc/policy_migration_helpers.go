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
	"time"
)

// retireLegacyPolicyFiles 在校验备份一致后将旧版 state.json 替换为防回写目录墓碑
// 请求体
// root 仓库根目录路径
// expectedHash 原始策略状态文件的SHA256摘要
// 响应体
// err 备份校验不匹配或墓碑目录创建失败错误
func retireLegacyPolicyFiles(root, expectedHash string) error {
	// 1. 核对内容寻址备份文件存在且校验和一致
	if len(expectedHash) != 64 {
		return errors.New("missing original policy backup fingerprint")
	}
	base := filepath.Join(root, ".arc")
	backup := filepath.Join(base, "backups", "policy-state-"+expectedHash+".json")
	if err := checkRegular(backup); err != nil {
		return fmt.Errorf("policy backup unavailable: %w", err)
	}
	checksum, err := shaFile(backup)
	if err != nil || checksum != expectedHash {
		return errors.New("policy backup checksum mismatch")
	}
	// 2. 移除已验证备份的旧 state.json 并建立同名空目录阻断旧版进程回写
	policyDir := filepath.Join(base, "policy")
	legacy := filepath.Join(policyDir, "state.json")
	st, err := os.Lstat(legacy)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(legacy, 0o700); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if st.IsDir() {
		entries, rErr := os.ReadDir(legacy)
		if rErr != nil {
			return rErr
		}
		if len(entries) != 0 {
			return errors.New("legacy state tombstone contains unexpected files")
		}
	} else if st.Mode().IsRegular() {
		liveHash, hErr := shaFile(legacy)
		if hErr != nil {
			return hErr
		}
		if liveHash != expectedHash {
			return errors.New("legacy state changed after migration; manual recovery required")
		}
		if err := os.Remove(legacy); err != nil {
			return err
		}
		if err := os.Mkdir(legacy, 0o700); err != nil {
			return fmt.Errorf("cannot install old-runtime tombstone: %w", err)
		}
	} else {
		return errors.New("unsafe legacy policy state path")
	}
	// 3. 将旧版 receipts 目录归档移入 backups
	receipts := filepath.Join(policyDir, "receipts")
	if st, err := os.Lstat(receipts); err == nil {
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe legacy receipts path")
		}
		dest := filepath.Join(base, "backups", "policy-receipts-"+expectedHash[:16])
		if _, statErr := os.Lstat(dest); statErr == nil {
			return errors.New("legacy receipt archive already exists; inspect before retrying")
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return statErr
		}
		if err := os.Rename(receipts, dest); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// policyMigrationFingerprint 从迁移领域事件中读取原始策略备份SHA256指纹
// 请求体
// db 数据库连接句柄
// 响应体
// sha 64位十六进制备份摘要
// err 事件缺失或格式非法错误
func policyMigrationFingerprint(db *sql.DB) (string, error) {
	// 1. 查询最新 policy.state.migrated 事件并解析 state_sha256
	var payload string
	err := db.QueryRow("SELECT payload FROM arc_events WHERE aggregate='policy' AND kind='policy.state.migrated' ORDER BY created DESC,id DESC LIMIT 1").Scan(&payload)
	if err != nil {
		return "", err
	}
	var v struct {
		SHA string `json:"state_sha256"`
	}
	if err := json.Unmarshal([]byte(payload), &v); err != nil {
		return "", err
	}
	if len(v.SHA) != 64 {
		return "", errors.New("migration event has no policy backup fingerprint")
	}
	return v.SHA, nil
}

// migratePolicyState 执行旧版JSON策略与独立收据文件向 arc.db 的原子等价迁移
// 请求体
// root 仓库根目录路径
// 响应体
// err 文件不安全、收据损坏、回读不等价或墓碑设置失败错误
func migratePolicyState(root string) error {
	// 1. 初始化策略表结构并在已完成迁移时幂等收尾目录墓碑
	if err := ensureArcPrivateFiles(root); err != nil {
		return err
	}
	db, err := openActiveArcDB(root)
	if err != nil {
		return err
	}
	defer db.Close()
	for _, stmt := range policyStoreSchema {
		if _, execErr := db.Exec(stmt); execErr != nil {
			return fmt.Errorf("policy schema: %w", execErr)
		}
	}
	doc, err := loadPolicyDocument(db)
	if err != nil {
		return err
	}
	if doc.Mode == "arc.db" {
		hash, fErr := policyMigrationFingerprint(db)
		if fErr != nil {
			return fErr
		}
		var verified string
		if qErr := db.QueryRow("SELECT value FROM arc_meta WHERE key='policy_migration_verified'").Scan(&verified); qErr != nil {
			return fmt.Errorf("Policy migration requires manual recovery: receipt readback was not marked complete: %w", qErr)
		}
		if verified != hash {
			return errors.New("Policy migration verification fingerprint mismatch; refusing legacy archival")
		}
		if rErr := retireLegacyPolicyFiles(root, hash); rErr != nil {
			return rErr
		}
		return emitRawJSON(json.Marshal(policyMigrateResponse{
			Storage:         "arc.db",
			AlreadyMigrated: true,
			LegacyGuard:     "directory",
			Active:          true,
		}))
	}
	// 2. 读取旧版 state.json 与 receipts/*.json 并执行强类型收据校验收集
	legacy := filepath.Join(root, ".arc", "policy", "state.json")
	body := []byte("{}")
	if st, statErr := os.Lstat(legacy); statErr == nil {
		if !st.Mode().IsRegular() {
			return errors.New("unsafe legacy policy state path")
		}
		body, err = os.ReadFile(legacy)
		if err != nil {
			return err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	state, err := decodePolicyState(body)
	if err != nil {
		return err
	}
	receipts, err := collectPolicyReceipts(state)
	if err != nil {
		return err
	}
	receiptsDir := filepath.Join(root, ".arc", "policy", "receipts")
	if info, statErr := os.Lstat(receiptsDir); statErr == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe legacy policy receipts directory")
		}
		list, gErr := filepath.Glob(filepath.Join(receiptsDir, "*.json"))
		if gErr != nil {
			return gErr
		}
		if len(list) > 10000 {
			return errors.New("legacy receipt import exceeds 10000 files")
		}
		var totalBytes int64
		for _, file := range list {
			if cErr := checkRegular(file); cErr != nil {
				return cErr
			}
			fInfo, sErr := os.Stat(file)
			if sErr != nil {
				return sErr
			}
			if fInfo.Size() > 16<<20 {
				return fmt.Errorf("legacy receipt %s exceeds 16 MiB", file)
			}
			totalBytes += fInfo.Size()
			if totalBytes > 64<<20 {
				return errors.New("legacy receipts exceed 64 MiB import budget")
			}
			raw, rErr := os.ReadFile(file)
			if rErr != nil {
				return rErr
			}
			rt := bytes.TrimSpace(raw)
			if len(rt) == 0 || rt[0] != '{' {
				return fmt.Errorf("legacy receipt %s is null or not an object", file)
			}
			var hdr policyReceiptHeader
			if uErr := json.Unmarshal(rt, &hdr); uErr != nil {
				return fmt.Errorf("legacy receipt %s: %w", file, uErr)
			}
			if hdr.TaskID == "" {
				return fmt.Errorf("legacy receipt %s has no task ID", file)
			}
			if hdr.Verdict != string(VerdictPass) && hdr.Verdict != "AUTHORIZED" {
				return fmt.Errorf("legacy receipt %s has invalid verdict", file)
			}
			canonicalRec, nErr := canonicalizeJSONBytes(rt)
			if nErr != nil {
				return nErr
			}
			receipts = append(receipts, policyReceipt{
				Hash:    shaBytes(canonicalRec),
				TaskID:  hdr.TaskID,
				Verdict: hdr.Verdict,
				Body:    string(canonicalRec),
			})
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	// 3. 保存原始状态备份并在事务内写入规范化策略状态与全部历史收据
	rawDigest := shaBytes(body)
	backups := filepath.Join(root, ".arc", "backups")
	if err := os.MkdirAll(backups, 0o700); err != nil {
		return err
	}
	if st, statErr := os.Lstat(backups); statErr != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("unsafe policy backup directory")
	}
	backup := filepath.Join(backups, "policy-state-"+rawDigest+".json")
	if _, statErr := os.Lstat(backup); errors.Is(statErr, os.ErrNotExist) {
		out, oErr := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if oErr != nil {
			return oErr
		}
		_, wErr := out.Write(body)
		if wErr == nil {
			wErr = out.Sync()
		}
		if cErr := out.Close(); wErr == nil {
			wErr = cErr
		}
		if wErr != nil {
			return wErr
		}
	} else if statErr != nil {
		return statErr
	}
	backupHash, err := shaFile(backup)
	if err != nil || backupHash != rawDigest {
		return errors.New("policy backup checksum mismatch")
	}
	canonical, err := canonicalizeJSONBytes(body)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	err = dbTransaction(ctx, db, func(tx *sql.Tx) error {
		if _, execErr := tx.ExecContext(ctx,
			"INSERT INTO arc_policy_state(id,version,body,updated_at) VALUES(1,1,?,?)",
			string(canonical), nowUTC()); execErr != nil {
			return execErr
		}
		if pErr := persistPolicyReceipts(ctx, tx, receipts); pErr != nil {
			return pErr
		}
		rawEv, mErr := json.Marshal(policyMigratedEventPayload{
			StateSHA256: rawDigest,
			Receipts:    len(receipts),
			Version:     1,
		})
		return addEventPayload(ctx, tx, "policy", "state", "policy.state.migrated", rawEv, mErr)
	})
	if err != nil {
		return err
	}
	// 4. 逐条回读核对全部收据与状态字节完全一致后写入认证标记并安装旧文件墓碑
	for _, record := range receipts {
		var taskID, verdict, recBody string
		if qErr := db.QueryRow("SELECT task_id,verdict,body FROM arc_policy_receipts WHERE sha256=?", record.Hash).Scan(&taskID, &verdict, &recBody); qErr != nil {
			return fmt.Errorf("receipt import parity failure: %w", qErr)
		}
		if taskID != record.TaskID || verdict != record.Verdict || recBody != record.Body {
			return errors.New("receipt import differs from original; refusing legacy archive")
		}
	}
	live, err := loadPolicyDocument(db)
	if err != nil || live.Mode != "arc.db" {
		return errors.New("policy migration did not commit a readable state")
	}
	reencoded, err := canonicalizeJSONBytes(live.State)
	if err != nil {
		return err
	}
	if string(reencoded) != string(canonical) {
		return errors.New("migrated policy state differs from original; fail closed")
	}
	if _, err := db.Exec("INSERT OR REPLACE INTO arc_meta(key,value) VALUES('policy_migration_verified',?)", rawDigest); err != nil {
		return err
	}
	if err := retireLegacyPolicyFiles(root, rawDigest); err != nil {
		return fmt.Errorf("policy DB migrated, but legacy guard setup failed; rerun migrate offline to finalize: %w", err)
	}
	return emitRawJSON(json.Marshal(policyMigrateResponse{
		Storage:          "arc.db",
		Version:          1,
		Backup:           backup,
		ImportedReceipts: len(receipts),
		LegacyGuard:      "directory",
		Active:           true,
	}))
}
