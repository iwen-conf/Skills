package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// executeFile 记录单个工作区文件的哈希观测并可选绑定所属任务
// 请求体
// db 数据库连接句柄
// root 仓库根目录路径
// path 仓库相对文件路径
// task 可选归属任务ID
// 响应体
// err 参数校验或事务落库错误
func executeFile(db *sql.DB, root, path, task string) error {
	// 1. 校验必填路径参数
	if path == "" {
		return errors.New("file update requires --path")
	}
	// 2. 在事务内校验可选归属任务、更新文件观测并建立任务文件关联
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var sha string
	err := dbTransaction(ctx, db, func(tx *sql.Tx) error {
		if task != "" {
			var present int
			if qErr := tx.QueryRowContext(ctx, "SELECT 1 FROM arc_tasks WHERE id=?", task).Scan(&present); qErr != nil {
				return fmt.Errorf("owning task: %w", qErr)
			}
		}
		var sErr error
		sha, sErr = storeFile(ctx, tx, root, path)
		if sErr != nil {
			return sErr
		}
		if task != "" {
			_, sErr = tx.ExecContext(ctx, "INSERT OR REPLACE INTO arc_task_files(task_id,path,updated) VALUES(?,?,?)",
				task, filepath.ToSlash(filepath.Clean(path)), nowUTC())
		}
		return sErr
	})
	if err != nil {
		return err
	}
	// 3. 输出具名文件更新响应
	return emitRawJSON(json.Marshal(fileUpdateResponse{
		Path:   filepath.ToSlash(filepath.Clean(path)),
		SHA256: sha,
		Task:   task,
		Index:  "pending",
	}))
}

// showFile 读取数据库中最近一次持久化的单文件观测记录
// 请求体
// db 数据库连接句柄
// path 仓库相对文件路径
// 响应体
// err 路径非法或无存储记录错误
func showFile(db *sql.DB, path string) error {
	// 1. 校验相对路径合法性并拒绝访问 .arc 内部路径
	if path == "" || filepath.IsAbs(path) {
		return errors.New("file show requires a relative --path")
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean == ".arc" || strings.HasPrefix(clean, ".arc/") {
		return errors.New("unsafe file path")
	}
	// 2. 查询 arc_files 表最新存储元数据
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var sha, changed string
	var size int64
	var version int
	if err := db.QueryRowContext(ctx, "SELECT sha256,bytes,version,changed FROM arc_files WHERE path=?", clean).Scan(&sha, &size, &version, &changed); err != nil {
		return err
	}
	// 3. 输出具名文件详情响应
	return emitRawJSON(json.Marshal(fileShowResponse{
		Path:      clean,
		SHA256:    sha,
		Bytes:     size,
		Version:   version,
		ChangedAt: changed,
	}))
}

// collectChanges 幂等采集当前Git工作区变更并生成内容寻址ChangeSet记录
// 请求体
// db 数据库连接句柄
// root 仓库根目录路径
// task 可选归属任务ID
// worktree 工作树路径参数
// 响应体
// err 工作树参数不支持、Git状态异常或数据库写入失败错误
func collectChanges(db *sql.DB, root, task, worktree string) error {
	// 1. 校验工作树范围并枚举当前Git变更文件列表
	if worktree != "." {
		return errors.New("v1 change collection requires the same repository worktree; --worktree switching is not yet supported")
	}
	files, err := changedGitPaths(root)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return emitRawJSON(json.Marshal(changeCollectResponse{ChangedFiles: 0, ChangeID: nil, Index: "unchanged"}))
	}
	// 2. 读取当前HEAD提交并逐文件计算内容指纹构造幂等变更集摘要
	revision, err := gitHead(root)
	if err != nil {
		return err
	}
	var observed []observedFileEntry
	for _, f := range files {
		sha, _, op, obsErr := observedFile(root, f.Path)
		if obsErr != nil {
			return obsErr
		}
		if op == "deleted" {
			f.Operation = "deleted"
		}
		observed = append(observed, observedFileEntry{Path: f.Path, Operation: f.Operation, SHA: sha})
	}
	digestInput, _ := json.Marshal(changeFingerprintInput{Task: task, Root: root, Revision: revision, Files: observed})
	fingerprint := shaBytes(digestInput)
	// 3. 在事务内检查幂等指纹，未命中时写入新ChangeSet及文件明细并触发索引队列
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var resp changeCollectResponse
	err = dbTransaction(ctx, db, func(tx *sql.Tx) error {
		if task != "" {
			var present int
			if qErr := tx.QueryRowContext(ctx, "SELECT 1 FROM arc_tasks WHERE id=?", task).Scan(&present); qErr != nil {
				return fmt.Errorf("owning task: %w", qErr)
			}
		}
		var existing string
		qErr := tx.QueryRowContext(ctx, "SELECT id FROM arc_changes WHERE fingerprint=?", fingerprint).Scan(&existing)
		if qErr == nil {
			for _, f := range observed {
				if _, sErr := storeFile(ctx, tx, root, f.Path); sErr != nil {
					return sErr
				}
				if task != "" {
					if _, sErr := tx.ExecContext(ctx, "INSERT OR REPLACE INTO arc_task_files(task_id,path,updated) VALUES(?,?,?)",
						task, filepath.ToSlash(f.Path), nowUTC()); sErr != nil {
						return sErr
					}
				}
			}
			dup := true
			resp = changeCollectResponse{ChangeID: &existing, ChangedFiles: len(files), Duplicate: &dup, Index: "pending"}
			return nil
		}
		if !errors.Is(qErr, sql.ErrNoRows) {
			return qErr
		}
		id, idErr := newID("CH-")
		if idErr != nil {
			return idErr
		}
		for _, f := range observed {
			if _, sErr := storeFile(ctx, tx, root, f.Path); sErr != nil {
				return sErr
			}
			if task != "" {
				if _, sErr := tx.ExecContext(ctx, "INSERT OR REPLACE INTO arc_task_files(task_id,path,updated) VALUES(?,?,?)",
					task, filepath.ToSlash(f.Path), nowUTC()); sErr != nil {
					return sErr
				}
			}
		}
		if _, execErr := tx.ExecContext(ctx, "INSERT INTO arc_changes(id,task,fingerprint,status,worktree,revision,created) VALUES(?,?,?,'collected',?,?,?)",
			id, task, fingerprint, root, revision, nowUTC()); execErr != nil {
			return execErr
		}
		for _, f := range observed {
			if _, execErr := tx.ExecContext(ctx, "INSERT INTO arc_change_files(change_id,path,sha256,operation) VALUES(?,?,?,?)",
				id, filepath.ToSlash(f.Path), f.SHA, f.Operation); execErr != nil {
				return execErr
			}
		}
		rawEv, mErr := json.Marshal(changeCollectedEventPayload{Task: task, Revision: revision, Files: len(files)})
		if evErr := addEventPayload(ctx, tx, "change", id, "change.collected", rawEv, mErr); evErr != nil {
			return evErr
		}
		dup := false
		resp = changeCollectResponse{ChangeID: &id, ChangedFiles: len(files), Duplicate: &dup, Index: "pending", Status: "collected"}
		return nil
	})
	if err != nil {
		return err
	}
	// 4. 输出具名变更集采集响应
	return emitRawJSON(json.Marshal(resp))
}

// showChange 查询指定变更集元数据及其包含的文件变更明细
// 请求体
// db 数据库连接句柄
// id 变更集唯一ID
// 响应体
// err ID缺失或数据库查询失败错误
func showChange(db *sql.DB, id string) error {
	// 1. 校验变更集ID并读取主表元数据
	if id == "" {
		return errors.New("change ID required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var task, status, rev, created string
	if err := db.QueryRowContext(ctx, "SELECT task,status,revision,created FROM arc_changes WHERE id=?", id).Scan(&task, &status, &rev, &created); err != nil {
		return err
	}
	// 2. 查询变更集内的全部文件明细条目
	rows, err := db.QueryContext(ctx, "SELECT path,sha256,operation FROM arc_change_files WHERE change_id=? ORDER BY path", id)
	if err != nil {
		return err
	}
	defer rows.Close()
	paths := []changeFileItem{}
	for rows.Next() {
		var item changeFileItem
		if err := rows.Scan(&item.Path, &item.SHA256, &item.Operation); err != nil {
			return err
		}
		paths = append(paths, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// 3. 输出具名变更集详情响应
	return emitRawJSON(json.Marshal(changeShowResponse{
		ID:       id,
		Task:     task,
		Status:   status,
		Revision: rev,
		Created:  created,
		Files:    paths,
	}))
}
