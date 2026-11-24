package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "turso.tech/database/tursogo"
)

const defaultVerifyDB = ".arc/verify/story.db"

var storySchema = []string{
	`CREATE TABLE IF NOT EXISTS story_definitions (
		story_id TEXT PRIMARY KEY,
		focal_entity TEXT NOT NULL,
		story_sha256 TEXT NOT NULL,
		source_path TEXT NOT NULL,
		raw_json TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS story_checkpoints (
		story_id TEXT NOT NULL,
		save_id TEXT NOT NULL,
		parent_id TEXT NOT NULL,
		engine TEXT NOT NULL,
		snapshot_path TEXT NOT NULL,
		digest_sha256 TEXT NOT NULL,
		created_at TEXT NOT NULL,
		PRIMARY KEY(story_id, save_id)
	)`,
	`CREATE TABLE IF NOT EXISTS story_runs (
		run_id TEXT PRIMARY KEY,
		story_id TEXT NOT NULL,
		story_sha256 TEXT NOT NULL,
		repo_root TEXT NOT NULL DEFAULT '',
		source_commit TEXT NOT NULL,
		source_tree_sha256 TEXT NOT NULL DEFAULT '',
		blind_mode INTEGER NOT NULL DEFAULT 0,
		verdict TEXT NOT NULL,
		started_at TEXT NOT NULL,
		finished_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS story_route_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id TEXT NOT NULL,
		story_id TEXT NOT NULL,
		story_sha256 TEXT NOT NULL,
		route_id TEXT NOT NULL,
		load_save TEXT NOT NULL,
		runtime_ns TEXT NOT NULL,
		verdict TEXT NOT NULL,
		softlock_detected INTEGER NOT NULL DEFAULT 0,
		probes_total INTEGER NOT NULL DEFAULT 0,
		probes_passed INTEGER NOT NULL DEFAULT 0,
		unlocked_ending TEXT NOT NULL DEFAULT '',
		raw_output TEXT NOT NULL DEFAULT '',
		repo_root TEXT NOT NULL DEFAULT '',
		source_commit TEXT NOT NULL DEFAULT '',
		source_tree_sha256 TEXT NOT NULL DEFAULT '',
		receipt_sha256 TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_story_route_lookup ON story_route_results(story_id, route_id, id)`,
	`CREATE TABLE IF NOT EXISTS story_step_traces (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id TEXT NOT NULL,
		story_id TEXT NOT NULL,
		story_sha256 TEXT NOT NULL,
		route_id TEXT NOT NULL,
		step_num INTEGER NOT NULL,
		choice TEXT NOT NULL,
		expected_behavior TEXT NOT NULL DEFAULT '',
		preconditions_json TEXT NOT NULL DEFAULT '{}',
		preconditions_actual_json TEXT NOT NULL DEFAULT '{}',
		expected_input_json TEXT NOT NULL DEFAULT '{}',
		actual_input_json TEXT NOT NULL DEFAULT '{}',
		before_state_json TEXT NOT NULL DEFAULT '{}',
		after_state_json TEXT NOT NULL DEFAULT '{}',
		executed_cmd TEXT NOT NULL DEFAULT '',
		duration_ms INTEGER NOT NULL DEFAULT 0,
		expect_status TEXT NOT NULL DEFAULT '',
		actual_status TEXT NOT NULL DEFAULT '',
		expect_result_json TEXT NOT NULL DEFAULT '{}',
		actual_result_json TEXT NOT NULL DEFAULT '{}',
		expected_probes_json TEXT NOT NULL DEFAULT '{}',
		actual_probes_json TEXT NOT NULL DEFAULT '{}',
		forbidden_effects_json TEXT NOT NULL DEFAULT '[]',
		observed_effects_json TEXT NOT NULL DEFAULT '[]',
		anti_softlock TEXT NOT NULL DEFAULT '',
		verdict TEXT NOT NULL,
		diff_reason TEXT NOT NULL DEFAULT '',
		raw_output TEXT NOT NULL DEFAULT '',
		trace_sha256 TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_story_step_lookup ON story_step_traces(story_id, run_id, route_id, step_num, id)`,
	`CREATE TABLE IF NOT EXISTS graphs (
		id TEXT PRIMARY KEY,
		raw_json TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS code_verify_plans (
		plan_id TEXT PRIMARY KEY,
		repo_root TEXT NOT NULL,
		primary_language TEXT NOT NULL,
		max_tier TEXT NOT NULL,
		plan_sha256 TEXT NOT NULL,
		raw_json TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS code_verify_runs (
		run_id TEXT PRIMARY KEY,
		plan_id TEXT NOT NULL,
		plan_sha256 TEXT NOT NULL,
		repo_root TEXT NOT NULL,
		source_commit TEXT NOT NULL,
		source_tree_sha256 TEXT NOT NULL DEFAULT '',
		max_tier TEXT NOT NULL,
		verdict TEXT NOT NULL,
		required_total INTEGER NOT NULL DEFAULT 0,
		passed_count INTEGER NOT NULL DEFAULT 0,
		failed_count INTEGER NOT NULL DEFAULT 0,
		blocked_count INTEGER NOT NULL DEFAULT 0,
		started_at TEXT NOT NULL,
		finished_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS code_verify_results (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id TEXT NOT NULL,
		plan_id TEXT NOT NULL,
		plan_sha256 TEXT NOT NULL,
		technique_id TEXT NOT NULL,
		priority TEXT NOT NULL,
		language TEXT NOT NULL,
		tool_name TEXT NOT NULL,
		executed_cmd TEXT NOT NULL,
		verdict TEXT NOT NULL,
		exit_code INTEGER NOT NULL DEFAULT 0,
		duration_ms INTEGER NOT NULL DEFAULT 0,
		metrics_json TEXT NOT NULL DEFAULT '{}',
		coverage_json TEXT NOT NULL DEFAULT '{}',
		failure_reason TEXT NOT NULL DEFAULT '',
		missing_prerequisite TEXT NOT NULL DEFAULT '',
		artifact_path TEXT NOT NULL DEFAULT '',
		artifact_sha256 TEXT NOT NULL DEFAULT '',
		raw_output TEXT NOT NULL DEFAULT '',
		repo_root TEXT NOT NULL DEFAULT '',
		source_commit TEXT NOT NULL DEFAULT '',
		source_tree_sha256 TEXT NOT NULL DEFAULT '',
		receipt_sha256 TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_code_verify_results_lookup ON code_verify_results(plan_id, technique_id, id)`,
	`CREATE INDEX IF NOT EXISTS idx_code_verify_results_run ON code_verify_results(plan_id, run_id, id)`,
}

// StoryLedger 通过具名私有互斥锁与数据库句柄管理六维验证账本（禁止暴露内部锁或DB）
type StoryLedger struct {
	// mu 保护并发写入顺序性的内部互斥锁
	mu sync.Mutex
	// db 单连接SQLite数据库句柄
	db *sql.DB
	// path 账本文件路径
	path string
}

// CheckpointRecord 描述持久化在账本中的检查点快照记录
type CheckpointRecord struct {
	// StoryID 故事线唯一标识
	StoryID string `json:"story_id"`
	// SaveID 检查点存档标识
	SaveID string `json:"save_id"`
	// ParentID 父级检查点标识
	ParentID string `json:"parent_id"`
	// Engine 存储引擎类型
	Engine string `json:"engine"`
	// SnapshotPath 快照存储路径或连接串
	SnapshotPath string `json:"snapshot_path"`
	// DigestSHA256 快照内容SHA256摘要
	DigestSHA256 string `json:"digest_sha256"`
	// CreatedAt 创建UTC时间戳
	CreatedAt string `json:"created_at"`
}

// openStoryLedger 打开单连接Storyline验证账本并幂等初始化表结构
// 请求体
// dbPath 账本数据库路径（为空时回退默认路径）
// 响应体
// ledger 初始化完成的账本实例指针
// err 目录创建、连接探测或建表失败错误
func openStoryLedger(dbPath string) (*StoryLedger, error) {
	// 1. 解析默认账本路径并创建父目录
	if dbPath == "" {
		dbPath = defaultVerifyDB
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	// 2. 打开单连接Turso数据库并配置busy_timeout
	db, err := sql.Open("turso", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	_, _ = db.ExecContext(ctx, "PRAGMA busy_timeout=5000")
	// 3. 依次执行六维验证账本与兼容图表的DDL语句及历史库列迁移
	for _, stmt := range storySchema {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("init story schema: %w", err)
		}
	}
	if err := ensureLedgerColumns(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate story schema: %w", err)
	}
	return &StoryLedger{db: db, path: dbPath}, nil
}

// Close 刷盘截断WAL日志并关闭验证账本数据库连接
// 请求体
// 无
// 响应体
// err 关闭底层数据库错误
func (l *StoryLedger) Close() error {
	// 1. 空实例安全返回
	if l == nil || l.db == nil {
		return nil
	}
	// 2. 执行WAL检查点截断并关闭句柄
	_, _ = l.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return l.db.Close()
}

// SaveStoryDefinition 幂等保存或更新故事线定义及兼容graphs表记录
// 请求体
// s 解析完成的故事线对象指针
// 响应体
// err JSON序列化或SQL写入错误
func (l *StoryLedger) SaveStoryDefinition(s *Storyline) error {
	// 1. 加锁序列化故事线定义为JSON文本
	l.mu.Lock()
	defer l.mu.Unlock()
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	// 2. 写入或更新 story_definitions 主表及兼容 graphs 表
	now := nowUTC()
	_, err = l.db.Exec(
		`INSERT INTO story_definitions(story_id, focal_entity, story_sha256, source_path, raw_json, updated_at)
		 VALUES(?,?,?,?,?,?)
		 ON CONFLICT(story_id) DO UPDATE SET
		   focal_entity=excluded.focal_entity,
		   story_sha256=excluded.story_sha256,
		   source_path=excluded.source_path,
		   raw_json=excluded.raw_json,
		   updated_at=excluded.updated_at`,
		s.StoryID, s.FocalEntity, s.SHA256, s.SourcePath, string(raw), now,
	)
	if err != nil {
		return err
	}
	_, _ = l.db.Exec(
		`INSERT INTO graphs(id, raw_json, created_at) VALUES(?,?,?)
		 ON CONFLICT(id) DO UPDATE SET raw_json=excluded.raw_json`,
		s.StoryID, string(raw), now,
	)
	return nil
}

// LoadStoryDefinition 从验证账本加载指定ID的故事线定义与指纹元数据
// 请求体
// storyID 故事线唯一标识
// 响应体
// s 反序列化后的故事线对象指针
// err 未找到记录或JSON解析失败错误
func (l *StoryLedger) LoadStoryDefinition(storyID string) (*Storyline, error) {
	// 1. 加锁查询存储的JSON文本、SHA256与源文件路径
	l.mu.Lock()
	defer l.mu.Unlock()
	var raw, sha, src string
	err := l.db.QueryRow(
		`SELECT raw_json, story_sha256, source_path FROM story_definitions WHERE story_id=?`,
		storyID,
	).Scan(&raw, &sha, &src)
	if err != nil {
		return nil, err
	}
	// 2. 反序列化结构体并回填版本指纹与源路径
	var s Storyline
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}
	s.SHA256 = sha
	s.SourcePath = src
	return &s, nil
}

// RecordCheckpoint 记录或更新单个检查点快照元数据
// 请求体
// rec 检查点快照记录
// 响应体
// err SQL写入错误
func (l *StoryLedger) RecordCheckpoint(rec CheckpointRecord) error {
	// 1. 加锁补全创建时间并执行Upsert写入
	l.mu.Lock()
	defer l.mu.Unlock()
	if rec.CreatedAt == "" {
		rec.CreatedAt = nowUTC()
	}
	_, err := l.db.Exec(
		`INSERT INTO story_checkpoints(story_id, save_id, parent_id, engine, snapshot_path, digest_sha256, created_at)
		 VALUES(?,?,?,?,?,?,?)
		 ON CONFLICT(story_id, save_id) DO UPDATE SET
		   parent_id=excluded.parent_id,
		   engine=excluded.engine,
		   snapshot_path=excluded.snapshot_path,
		   digest_sha256=excluded.digest_sha256,
		   created_at=excluded.created_at`,
		rec.StoryID, rec.SaveID, rec.ParentID, rec.Engine, rec.SnapshotPath, rec.DigestSHA256, rec.CreatedAt,
	)
	return err
}

// ListCheckpoints 按存档ID升序读取指定故事线的全部检查点记录
// 请求体
// storyID 故事线唯一标识
// 响应体
// out 检查点记录切片
// err SQL查询错误
func (l *StoryLedger) ListCheckpoints(storyID string) ([]CheckpointRecord, error) {
	// 1. 加锁查询检查点表并逐行扫描返回
	l.mu.Lock()
	defer l.mu.Unlock()
	rows, err := l.db.Query(
		`SELECT story_id, save_id, parent_id, engine, snapshot_path, digest_sha256, created_at
		 FROM story_checkpoints WHERE story_id=? ORDER BY save_id`, storyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CheckpointRecord
	for rows.Next() {
		var r CheckpointRecord
		if err := rows.Scan(&r.StoryID, &r.SaveID, &r.ParentID, &r.Engine, &r.SnapshotPath, &r.DigestSHA256, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecordRun 记录单次Storyline完整运行批次元数据与总判定结果
// 请求体
// runID 运行批次ID
// storyID 故事线唯一标识
// storySHA 故事线版本SHA256摘要
// repoRoot 执行时规范化仓库根目录
// commit 当前源码Git提交哈希
// treeSHA 工作树实时内容摘要
// blind 是否开启盲测模式
// verdict 批次总判定结果
// startedAt 批次启动UTC时间戳
// 响应体
// err SQL插入失败错误
func (l *StoryLedger) RecordRun(runID, storyID, storySHA, repoRoot, commit, treeSHA string, blind bool, verdict, startedAt string) error {
	// 1. 加锁转换布尔标记并插入 story_runs 记录
	l.mu.Lock()
	defer l.mu.Unlock()
	blindInt := 0
	if blind {
		blindInt = 1
	}
	_, err := l.db.Exec(
		`INSERT INTO story_runs(run_id, story_id, story_sha256, repo_root, source_commit, source_tree_sha256, blind_mode, verdict, started_at, finished_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		runID, storyID, storySHA, repoRoot, commit, treeSHA, blindInt, verdict, startedAt, nowUTC(),
	)
	return err
}

// LatestRunID 查询指定故事线最近一次完成的运行批次ID
// 请求体
// storyID 故事线唯一标识
// 响应体
// runID 最新运行批次ID（无记录时返回空字符串）
// err 数据库查询错误
func (l *StoryLedger) LatestRunID(storyID string) (string, error) {
	// 1. 加锁按完成时间与rowid倒序读取首条批次ID并用errors.Is识别空结果
	l.mu.Lock()
	defer l.mu.Unlock()
	var runID string
	err := l.db.QueryRow(
		`SELECT run_id FROM story_runs WHERE story_id=? ORDER BY finished_at DESC, rowid DESC LIMIT 1`,
		storyID,
	).Scan(&runID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return runID, err
}
