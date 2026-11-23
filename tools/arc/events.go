package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"time"
)

// eventCommand 解析事件查询参数并输出受限持久事件列表
// 请求体
// args 命令行参数切片
// 响应体
// err 参数校验或查询执行错误
func eventCommand(args []string) error {
	// 1. 校验子命令动作并解析查询过滤标志
	if len(args) == 0 || args[0] != "list" {
		return errors.New("usage: arc event list [--repo DIR] [--resource KIND] [--id RESOURCE_ID] [--limit 1..200]")
	}
	fs := flag.NewFlagSet("arc event list", flag.ContinueOnError)
	repo := fs.String("repo", ".", "repository root")
	resource := fs.String("resource", "", "aggregate kind")
	id := fs.String("id", "", "aggregate ID")
	limit := fs.Int("limit", 50, "maximum event rows (1..200)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	// 2. 校验分页条数范围与资源过滤依赖关系
	if *limit < 1 || *limit > 200 {
		return errors.New("--limit must be between 1 and 200")
	}
	if *id != "" && *resource == "" {
		return errors.New("--id requires --resource")
	}
	// 3. 在仓库级锁保护下读取激活态数据库并输出事件列表
	root, err := repositoryRoot(*repo)
	if err != nil {
		return err
	}
	return withArcLock(root, func() error {
		db, err := openActiveArcDB(root)
		if err != nil {
			return err
		}
		defer db.Close()
		return printEvents(db, *resource, *id, *limit)
	})
}

// printEvents 按强类型分支参数查询持久化领域事件并输出具名结构体JSON响应
// 请求体
// db 数据库连接句柄
// aggregate 聚合根类型过滤条件
// aggregateID 聚合根ID过滤条件
// limit 最大返回行数
// 响应体
// err SQL查询、JSON校验或标准输出编码错误
func printEvents(db *sql.DB, aggregate, aggregateID string, limit int) error {
	// 1. 按确定性过滤组合直接传入强类型参数查询（禁用 []any 动态切片）
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	const baseSelect = "SELECT id,aggregate,aggregate_id,kind,payload,created FROM arc_events"
	const orderLimit = " ORDER BY created DESC,id DESC LIMIT ?"
	var (
		rows *sql.Rows
		err  error
	)
	switch {
	case aggregate != "" && aggregateID != "":
		rows, err = db.QueryContext(ctx, baseSelect+" WHERE aggregate=? AND aggregate_id=?"+orderLimit, aggregate, aggregateID, limit)
	case aggregate != "":
		rows, err = db.QueryContext(ctx, baseSelect+" WHERE aggregate=?"+orderLimit, aggregate, limit)
	default:
		rows, err = db.QueryContext(ctx, baseSelect+orderLimit, limit)
	}
	if err != nil {
		return err
	}
	defer rows.Close()
	// 2. 逐行扫描并校验原始事件JSON载荷合法性
	items := make([]eventRecordResponse, 0, limit)
	for rows.Next() {
		var e eventRecordResponse
		var payload string
		if err := rows.Scan(&e.ID, &e.Aggregate, &e.AggregateID, &e.Kind, &payload, &e.Created); err != nil {
			return err
		}
		if !json.Valid([]byte(payload)) {
			return fmt.Errorf("event %s has invalid JSON payload", e.ID)
		}
		e.Payload = json.RawMessage(payload)
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// 3. 序列化具名响应结构写入标准输出
	return emitRawJSON(json.Marshal(eventListResponse{Events: items, Count: len(items)}))
}
