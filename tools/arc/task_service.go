package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var taskTransitions = map[TaskStatus]map[TaskStatus]bool{
	TaskStatusDraft:     {TaskStatusReady: true},
	TaskStatusReady:     {TaskStatusRunning: true, TaskStatusBlocked: true},
	TaskStatusRunning:   {TaskStatusVerifying: true, TaskStatusBlocked: true},
	TaskStatusBlocked:   {TaskStatusReady: true, TaskStatusRunning: true},
	TaskStatusVerifying: {TaskStatusRunning: true, TaskStatusDone: true, TaskStatusBlocked: true},
	TaskStatusDone:      {TaskStatusReopened: true},
	TaskStatusReopened:  {TaskStatusReady: true, TaskStatusRunning: true},
}

// addEventPayload 在事务内将已序列化的强类型事件JSON追加到不可变事件日志与索引队列
// 请求体
// ctx 请求上下文
// tx 当前数据库事务句柄
// resource 聚合根类型标识
// id 聚合根唯一ID
// event 事件名称
// payload 强类型结构体序列化后的JSON字节
// marshalErr 上游序列化错误
// 响应体
// err 序列化校验或SQL写入失败错误
func addEventPayload(ctx context.Context, tx *sql.Tx, resource, id, event string, payload []byte, marshalErr error) error {
	// 1. 校验上游强类型载荷序列化无误并生成事件唯一ID
	if marshalErr != nil {
		return marshalErr
	}
	eventID, err := newID("EV-")
	if err != nil {
		return err
	}
	// 2. 写入不可变事件日志表 arc_events
	if _, err = tx.ExecContext(ctx, "INSERT INTO arc_events(id,aggregate,aggregate_id,kind,payload,created) VALUES(?,?,?,?,?,?)",
		eventID, resource, id, event, string(payload), nowUTC()); err != nil {
		return err
	}
	// 3. 仅当文件或变更集变动时写入索引待办队列
	if resource == "file" || resource == "change" {
		_, err = tx.ExecContext(ctx, "INSERT INTO arc_outbox(event_id,status) VALUES(?,'pending')", eventID)
	}
	return err
}

// executeTask 执行任务的创建、幂等绑定、详情读取与乐观锁状态流转
// 请求体
// db 数据库连接句柄
// action 任务动作名称
// id 任务唯一ID
// title 任务标题
// to 目标流转状态
// expected 乐观锁期望版本号
// 响应体
// err 参数非法、版本冲突或SQL执行失败错误
func executeTask(db *sql.DB, action, id, title, to string, expected int) error {
	// 1. 创建受控超时上下文并根据动作分发处理
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	switch action {
	case "create":
		return createArcTask(ctx, db, title)
	case "ensure":
		return ensureArcTask(ctx, db, id, title)
	case "show":
		return showArcTaskBasic(ctx, db, id)
	case "transition":
		return transitionArcTask(ctx, db, id, to, expected)
	default:
		return fmt.Errorf("unsupported task action %q (create|ensure|show|transition|complete|link-verify|verify-status)", action)
	}
}

// createArcTask 创建草稿态新任务并记录强类型创建事件
// 请求体
// ctx 请求上下文
// db 数据库连接句柄
// title 任务标题
// 响应体
// err 标题校验或事务提交失败错误
func createArcTask(ctx context.Context, db *sql.DB, title string) error {
	// 1. 校验标题非空且不超过500字节并生成任务ID
	if strings.TrimSpace(title) == "" || len(title) > 500 {
		return errors.New("--title required (1..500 bytes)")
	}
	newTaskID, err := newID("TASK-")
	if err != nil {
		return err
	}
	// 2. 在事务内落库初始draft版本并记录强类型 taskCreatedEventPayload
	err = dbTransaction(ctx, db, func(tx *sql.Tx) error {
		now := nowUTC()
		if _, execErr := tx.ExecContext(ctx, "INSERT INTO arc_tasks(id,title,status,version,created,updated) VALUES(?,?,?,1,?,?)",
			newTaskID, strings.TrimSpace(title), string(TaskStatusDraft), now, now); execErr != nil {
			return execErr
		}
		raw, mErr := json.Marshal(taskCreatedEventPayload{Title: title})
		return addEventPayload(ctx, tx, "task", newTaskID, "task.created", raw, mErr)
	})
	if err != nil {
		return err
	}
	// 3. 输出具名任务变更响应
	return emitRawJSON(json.Marshal(taskMutationResponse{ID: newTaskID, Status: string(TaskStatusDraft), Version: 1}))
}

// ensureArcTask 幂等绑定外部策略任务ID且绝不越权提升任务状态
// 请求体
// ctx 请求上下文
// db 数据库连接句柄
// id 稳定任务ID
// title 任务标题
// 响应体
// err ID非法、标题冲突或事务写入错误
func ensureArcTask(ctx context.Context, db *sql.DB, id, title string) error {
	// 1. 校验任务ID字符集安全性与标题长度边界
	if id == "" || len(id) > 128 || strings.Trim(id, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-") != "" {
		return errors.New("task ensure requires a safe --id (1..128 ASCII characters)")
	}
	if strings.TrimSpace(title) == "" || len(title) > 500 {
		return errors.New("task ensure requires --title (1..500 bytes)")
	}
	name := strings.TrimSpace(title)
	// 2. 在事务内检查同ID记录，若已存在则核对标题一致性后幂等返回
	var resp taskMutationResponse
	err := dbTransaction(ctx, db, func(tx *sql.Tx) error {
		var existingTitle, status string
		var version int
		qErr := tx.QueryRowContext(ctx, "SELECT title,status,version FROM arc_tasks WHERE id=?", id).Scan(&existingTitle, &status, &version)
		if qErr == nil {
			if existingTitle != name {
				return fmt.Errorf("task %s identity collision (title differs)", id)
			}
			dup := true
			resp = taskMutationResponse{ID: id, Status: status, Version: version, Duplicate: &dup}
			return nil
		}
		if !errors.Is(qErr, sql.ErrNoRows) {
			return qErr
		}
		now := nowUTC()
		if _, execErr := tx.ExecContext(ctx, "INSERT INTO arc_tasks(id,title,status,version,created,updated) VALUES(?,?,?,1,?,?)",
			id, name, string(TaskStatusDraft), now, now); execErr != nil {
			return execErr
		}
		raw, mErr := json.Marshal(taskImportedEventPayload{Title: name, Source: "arc-policy"})
		if evErr := addEventPayload(ctx, tx, "task", id, "task.imported", raw, mErr); evErr != nil {
			return evErr
		}
		dup := false
		resp = taskMutationResponse{ID: id, Status: string(TaskStatusDraft), Version: 1, Duplicate: &dup}
		return nil
	})
	if err != nil {
		return err
	}
	// 3. 输出幂等绑定响应
	return emitRawJSON(json.Marshal(resp))
}

// showArcTaskBasic 查询基础任务元数据及其关联的文件路径列表
// 请求体
// ctx 请求上下文
// db 数据库连接句柄
// id 任务唯一ID
// 响应体
// err 任务不存在或文件查询失败错误
func showArcTaskBasic(ctx context.Context, db *sql.DB, id string) error {
	// 1. 校验任务ID并读取任务主表元数据
	if id == "" {
		return errors.New("task ID required (--id or positional)")
	}
	var name, status, updated string
	var v int
	if err := db.QueryRowContext(ctx, "SELECT title,status,version,updated FROM arc_tasks WHERE id=?", id).Scan(&name, &status, &v, &updated); err != nil {
		return err
	}
	// 2. 查询任务关联的全部文件路径
	rows, err := db.QueryContext(ctx, "SELECT path FROM arc_task_files WHERE task_id=? ORDER BY path", id)
	if err != nil {
		return err
	}
	files := []string{}
	for rows.Next() {
		var path string
		if e := rows.Scan(&path); e != nil {
			_ = rows.Close()
			return e
		}
		files = append(files, path)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	// 3. 输出具名任务详情响应
	return emitRawJSON(json.Marshal(taskShowResponse{ID: id, Title: name, Status: status, Version: v, Updated: updated, Files: files}))
}

// transitionArcTask 基于CAS版本号执行任务状态机合法跃迁（禁止无证明直接进入done）
// 请求体
// ctx 请求上下文
// db 数据库连接句柄
// id 任务唯一ID
// to 目标状态字符串
// expected 期望当前版本号
// 响应体
// err 版本冲突、非法状态跃迁或阻断无证据完成错误
func transitionArcTask(ctx context.Context, db *sql.DB, id, to string, expected int) error {
	// 1. 校验参数完整性
	if id == "" || expected < 1 {
		return errors.New("task transition requires --id and --expected-version >=1")
	}
	// 2. 在事务内校验当前版本号、闭合状态机转移矩阵并执行CAS更新
	err := dbTransaction(ctx, db, func(tx *sql.Tx) error {
		var old string
		var version int
		if err := tx.QueryRowContext(ctx, "SELECT status,version FROM arc_tasks WHERE id=?", id).Scan(&old, &version); err != nil {
			return err
		}
		if version != expected {
			return fmt.Errorf("task %s conflict: expected version %d, actual %d", id, expected, version)
		}
		if !taskTransitions[TaskStatus(old)][TaskStatus(to)] {
			return fmt.Errorf("invalid task transition %s -> %s", old, to)
		}
		if TaskStatus(to) == TaskStatusDone {
			return errors.New("done requires accepted verification evidence; arc task verification is not implemented yet")
		}
		res, err := tx.ExecContext(ctx, "UPDATE arc_tasks SET status=?,version=version+1,updated=? WHERE id=? AND version=? AND status=?",
			to, nowUTC(), id, expected, old)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil || affected != 1 {
			return errors.New("task changed concurrently; transition aborted")
		}
		raw, mErr := json.Marshal(taskTransitionedEventPayload{From: old, To: to, FromVersion: version})
		return addEventPayload(ctx, tx, "task", id, "task.transitioned", raw, mErr)
	})
	if err != nil {
		return err
	}
	// 3. 输出跃迁后的具名状态响应
	return emitRawJSON(json.Marshal(taskMutationResponse{ID: id, Status: to, Version: expected + 1}))
}
