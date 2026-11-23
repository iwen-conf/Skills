package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		source_commit TEXT NOT NULL,
		blind_mode INTEGER NOT NULL DEFAULT 0,
		verdict TEXT NOT NULL,
		started_at TEXT NOT NULL,
		finished_at TEXT NOT NULL
	)`,
	// Immutable append-only route results: a later PASS NEVER deletes or overwrites a historical FAIL.
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
		receipt_sha256 TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_story_route_lookup ON story_route_results(story_id, route_id, id)`,
	// Immutable 6-dimension step trace ledger: records preconditions, expected vs actual input,
	// before -> after state, execution command & duration, expected behavior, and expected vs actual results/probes.
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
	// Compatibility table so `arc db migrate` and legacy status checks work cleanly.
	`CREATE TABLE IF NOT EXISTS graphs (
		id TEXT PRIMARY KEY,
		raw_json TEXT NOT NULL DEFAULT '{}',
		created_at TEXT NOT NULL DEFAULT ''
	)`,
}

type StoryLedger struct {
	mu   sync.Mutex
	db   *sql.DB
	path string
}

func openStoryLedger(dbPath string) (*StoryLedger, error) {
	if dbPath == "" {
		dbPath = defaultVerifyDB
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("turso", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	_, _ = db.ExecContext(ctx, "PRAGMA busy_timeout=5000")
	for _, stmt := range storySchema {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("init story schema: %w", err)
		}
	}
	return &StoryLedger{db: db, path: dbPath}, nil
}

func (l *StoryLedger) Close() error {
	if l == nil || l.db == nil {
		return nil
	}
	_, _ = l.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return l.db.Close()
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func (l *StoryLedger) SaveStoryDefinition(s *Storyline) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
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

func (l *StoryLedger) LoadStoryDefinition(storyID string) (*Storyline, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var raw string
	var sha, src string
	err := l.db.QueryRow(
		`SELECT raw_json, story_sha256, source_path FROM story_definitions WHERE story_id=?`,
		storyID,
	).Scan(&raw, &sha, &src)
	if err != nil {
		return nil, err
	}
	var s Storyline
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}
	s.SHA256 = sha
	s.SourcePath = src
	return &s, nil
}

type CheckpointRecord struct {
	StoryID      string `json:"story_id"`
	SaveID       string `json:"save_id"`
	ParentID     string `json:"parent_id"`
	Engine       string `json:"engine"`
	SnapshotPath string `json:"snapshot_path"`
	DigestSHA256 string `json:"digest_sha256"`
	CreatedAt    string `json:"created_at"`
}

func (l *StoryLedger) RecordCheckpoint(rec CheckpointRecord) error {
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

func (l *StoryLedger) ListCheckpoints(storyID string) ([]CheckpointRecord, error) {
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

type RouteResultRecord struct {
	ID               int64  `json:"id"`
	RunID            string `json:"run_id"`
	StoryID          string `json:"story_id"`
	StorySHA256      string `json:"story_sha256"`
	RouteID          string `json:"route_id"`
	LoadSave         string `json:"load_save"`
	RuntimeNS        string `json:"runtime_ns"`
	Verdict          string `json:"verdict"` // PASS | FAIL | BLOCKED
	SoftlockDetected bool   `json:"softlock_detected"`
	ProbesTotal      int    `json:"probes_total"`
	ProbesPassed     int    `json:"probes_passed"`
	UnlockedEnding   string `json:"unlocked_ending"`
	RawOutput        string `json:"raw_output"`
	ReceiptSHA256    string `json:"receipt_sha256"`
	CreatedAt        string `json:"created_at"`
}

type StepTraceRecord struct {
	ID                  int64          `json:"id"`
	RunID               string         `json:"run_id"`
	StoryID             string         `json:"story_id"`
	StorySHA256         string         `json:"story_sha256"`
	RouteID             string         `json:"route_id"`
	StepNum             int            `json:"step_num"`
	Choice              string         `json:"choice"`
	ExpectedBehavior    string         `json:"expected_behavior,omitempty"`
	Preconditions       map[string]any `json:"preconditions,omitempty"`
	PreconditionsActual map[string]any `json:"preconditions_actual,omitempty"`
	ExpectedInput       map[string]any `json:"expected_input,omitempty"`
	ActualInput         map[string]any `json:"actual_input,omitempty"`
	BeforeState         map[string]any `json:"before_state,omitempty"`
	AfterState          map[string]any `json:"after_state,omitempty"`
	ExecutedCmd         string         `json:"executed_cmd,omitempty"`
	DurationMS          int64          `json:"duration_ms"`
	ExpectStatus        string         `json:"expect_status,omitempty"`
	ActualStatus        string         `json:"actual_status,omitempty"`
	ExpectResult        map[string]any `json:"expect_result,omitempty"`
	ActualResult        map[string]any `json:"actual_result,omitempty"`
	ExpectedProbes      map[string]any `json:"expected_probes,omitempty"`
	ActualProbes        map[string]any `json:"actual_probes,omitempty"`
	ForbiddenEffects    []string       `json:"forbidden_effects,omitempty"`
	ObservedEffects     []string       `json:"observed_effects,omitempty"`
	AntiSoftlock        string         `json:"anti_softlock,omitempty"`
	Verdict             string         `json:"verdict"` // PASS | FAIL | BLOCKED
	DiffReason          string         `json:"diff_reason,omitempty"`
	RawOutput           string         `json:"raw_output,omitempty"`
	TraceSHA256         string         `json:"trace_sha256"`
	CreatedAt           string         `json:"created_at"`
}

func marshalMapJSON(m map[string]any) string {
	if len(m) == 0 {
		return "{}"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func unmarshalMapJSON(raw string) map[string]any {
	if raw == "" || raw == "{}" {
		return nil
	}
	var m map[string]any
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&m); err != nil || len(m) == 0 {
		return nil
	}
	return m
}

func marshalSliceJSON(s []string) string {
	if len(s) == 0 {
		return "[]"
	}
	b, err := json.Marshal(s)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func unmarshalSliceJSON(raw string) []string {
	if raw == "" || raw == "[]" {
		return nil
	}
	var s []string
	if err := json.Unmarshal([]byte(raw), &s); err != nil || len(s) == 0 {
		return nil
	}
	return s
}

func (l *StoryLedger) RecordRun(runID, storyID, storySHA, commit string, blind bool, verdict, startedAt string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	blindInt := 0
	if blind {
		blindInt = 1
	}
	_, err := l.db.Exec(
		`INSERT INTO story_runs(run_id, story_id, story_sha256, source_commit, blind_mode, verdict, started_at, finished_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		runID, storyID, storySHA, commit, blindInt, verdict, startedAt, nowUTC(),
	)
	return err
}

func (l *StoryLedger) LatestRunID(storyID string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var runID string
	err := l.db.QueryRow(
		`SELECT run_id FROM story_runs WHERE story_id=? ORDER BY finished_at DESC, rowid DESC LIMIT 1`,
		storyID,
	).Scan(&runID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return runID, err
}

func (l *StoryLedger) AppendRouteResult(rec RouteResultRecord) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if rec.CreatedAt == "" {
		rec.CreatedAt = nowUTC()
	}
	softInt := 0
	if rec.SoftlockDetected {
		softInt = 1
	}
	_, err := l.db.Exec(
		`INSERT INTO story_route_results(
			run_id, story_id, story_sha256, route_id, load_save, runtime_ns,
			verdict, softlock_detected, probes_total, probes_passed, unlocked_ending,
			raw_output, receipt_sha256, created_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.RunID, rec.StoryID, rec.StorySHA256, rec.RouteID, rec.LoadSave, rec.RuntimeNS,
		rec.Verdict, softInt, rec.ProbesTotal, rec.ProbesPassed, rec.UnlockedEnding,
		rec.RawOutput, rec.ReceiptSHA256, rec.CreatedAt,
	)
	return err
}

func (l *StoryLedger) AppendStepTrace(tr StepTraceRecord) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if tr.CreatedAt == "" {
		tr.CreatedAt = nowUTC()
	}
	_, err := l.db.Exec(
		`INSERT INTO story_step_traces(
			run_id, story_id, story_sha256, route_id, step_num, choice, expected_behavior,
			preconditions_json, preconditions_actual_json, expected_input_json, actual_input_json,
			before_state_json, after_state_json, executed_cmd, duration_ms,
			expect_status, actual_status, expect_result_json, actual_result_json,
			expected_probes_json, actual_probes_json, forbidden_effects_json, observed_effects_json,
			anti_softlock, verdict, diff_reason, raw_output, trace_sha256, created_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		tr.RunID, tr.StoryID, tr.StorySHA256, tr.RouteID, tr.StepNum, tr.Choice, tr.ExpectedBehavior,
		marshalMapJSON(tr.Preconditions), marshalMapJSON(tr.PreconditionsActual),
		marshalMapJSON(tr.ExpectedInput), marshalMapJSON(tr.ActualInput),
		marshalMapJSON(tr.BeforeState), marshalMapJSON(tr.AfterState),
		tr.ExecutedCmd, tr.DurationMS,
		tr.ExpectStatus, tr.ActualStatus,
		marshalMapJSON(tr.ExpectResult), marshalMapJSON(tr.ActualResult),
		marshalMapJSON(tr.ExpectedProbes), marshalMapJSON(tr.ActualProbes),
		marshalSliceJSON(tr.ForbiddenEffects), marshalSliceJSON(tr.ObservedEffects),
		tr.AntiSoftlock, tr.Verdict, tr.DiffReason, tr.RawOutput, tr.TraceSHA256, tr.CreatedAt,
	)
	return err
}

func (l *StoryLedger) ListStepTraces(storyID, runID, routeID string) ([]StepTraceRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	query := `SELECT id, run_id, story_id, story_sha256, route_id, step_num, choice, expected_behavior,
		preconditions_json, preconditions_actual_json, expected_input_json, actual_input_json,
		before_state_json, after_state_json, executed_cmd, duration_ms,
		expect_status, actual_status, expect_result_json, actual_result_json,
		expected_probes_json, actual_probes_json, forbidden_effects_json, observed_effects_json,
		anti_softlock, verdict, diff_reason, raw_output, trace_sha256, created_at
	 FROM story_step_traces WHERE story_id=?`
	args := []any{storyID}
	if runID != "" {
		query += ` AND run_id=?`
		args = append(args, runID)
	}
	if routeID != "" {
		query += ` AND route_id=?`
		args = append(args, routeID)
	}
	query += ` ORDER BY id ASC`

	rows, err := l.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StepTraceRecord
	for rows.Next() {
		var tr StepTraceRecord
		var preJ, preActJ, expInJ, actInJ, befJ, aftJ, expResJ, actResJ, expPrJ, actPrJ, forbJ, obsJ string
		if err := rows.Scan(
			&tr.ID, &tr.RunID, &tr.StoryID, &tr.StorySHA256, &tr.RouteID, &tr.StepNum, &tr.Choice, &tr.ExpectedBehavior,
			&preJ, &preActJ, &expInJ, &actInJ, &befJ, &aftJ, &tr.ExecutedCmd, &tr.DurationMS,
			&tr.ExpectStatus, &tr.ActualStatus, &expResJ, &actResJ, &expPrJ, &actPrJ, &forbJ, &obsJ,
			&tr.AntiSoftlock, &tr.Verdict, &tr.DiffReason, &tr.RawOutput, &tr.TraceSHA256, &tr.CreatedAt,
		); err != nil {
			return nil, err
		}
		tr.Preconditions = unmarshalMapJSON(preJ)
		tr.PreconditionsActual = unmarshalMapJSON(preActJ)
		tr.ExpectedInput = unmarshalMapJSON(expInJ)
		tr.ActualInput = unmarshalMapJSON(actInJ)
		tr.BeforeState = unmarshalMapJSON(befJ)
		tr.AfterState = unmarshalMapJSON(aftJ)
		tr.ExpectResult = unmarshalMapJSON(expResJ)
		tr.ActualResult = unmarshalMapJSON(actResJ)
		tr.ExpectedProbes = unmarshalMapJSON(expPrJ)
		tr.ActualProbes = unmarshalMapJSON(actPrJ)
		tr.ForbiddenEffects = unmarshalSliceJSON(forbJ)
		tr.ObservedEffects = unmarshalSliceJSON(obsJ)
		out = append(out, tr)
	}
	return out, rows.Err()
}

func (l *StoryLedger) ListRouteResults(storyID string) ([]RouteResultRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rows, err := l.db.Query(
		`SELECT id, run_id, story_id, story_sha256, route_id, load_save, runtime_ns,
		        verdict, softlock_detected, probes_total, probes_passed, unlocked_ending,
		        raw_output, receipt_sha256, created_at
		 FROM story_route_results WHERE story_id=? ORDER BY id ASC`, storyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RouteResultRecord
	for rows.Next() {
		var r RouteResultRecord
		var soft int
		if err := rows.Scan(
			&r.ID, &r.RunID, &r.StoryID, &r.StorySHA256, &r.RouteID, &r.LoadSave, &r.RuntimeNS,
			&r.Verdict, &soft, &r.ProbesTotal, &r.ProbesPassed, &r.UnlockedEnding,
			&r.RawOutput, &r.ReceiptSHA256, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		r.SoftlockDetected = soft != 0
		out = append(out, r)
	}
	return out, rows.Err()
}
