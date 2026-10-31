package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "turso.tech/database/tursogo"
)

// db is an in-process local Turso handle. Every arc-verify invocation is still
// short-lived; no server, cloud auth, Python worker or SQL-shell process exists.
type db struct {
	file   string
	handle *sql.DB
}

func openDB(path string, create bool) (db, error) {
	if strings.Contains(path, "://") || strings.TrimSpace(path) == "" {
		return db{}, errors.New("only a local database file path is accepted")
	}
	abs, e := filepath.Abs(path)
	if e != nil {
		return db{}, e
	}
	if create {
		if e = os.MkdirAll(filepath.Dir(abs), 0700); e != nil {
			return db{}, e
		}
	} else {
		st, e := os.Lstat(abs)
		if e != nil || !st.Mode().IsRegular() {
			return db{}, fmt.Errorf("local database not initialized or unsafe path: %s", abs)
		}
	}
	conn, e := sql.Open("turso", abs)
	if e != nil {
		return db{}, e
	}
	// Single writer and only one connection per short command.
	// Each invocation may run concurrently with others; Turso's file locks
	// and transaction handling remain the source of truth.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if e = conn.PingContext(ctx); e != nil {
		conn.Close()
		return db{}, e
	}
	return db{file: abs, handle: conn}, nil
}
func (d db) close() error {
	if d.handle == nil {
		return nil
	}
	return d.handle.Close()
}
func (d db) sql(statement string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, e := d.handle.ExecContext(ctx, statement)
	return "", e
}
func quote(v string) string { return "'" + strings.ReplaceAll(v, "'", "''") + "'" }
func fieldsSQL(columns ...string) string {
	// Pass the projection as real SQL columns. No hex/string concatenation.
	return strings.Join(columns, ",")
}

// each streams a typed SQL result. A single reusable scan-target allocation
// replaces hex encode/decode and splitting of one huge joined TEXT cell.
func (d db) each(statement string, columns int, visit func([]string) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rows, e := d.handle.QueryContext(ctx, statement)
	if e != nil {
		return e
	}
	defer rows.Close()
	values := make([]sql.NullString, columns)
	targets := make([]any, columns)
	row := make([]string, columns)
	for i := range values {
		targets[i] = &values[i]
	}
	for rows.Next() {
		if e = rows.Scan(targets...); e != nil {
			return e
		}
		for i, value := range values {
			if value.Valid {
				row[i] = value.String
			} else {
				row[i] = ""
			}
		}
		// A visitor must not retain the row; query() copies it for callers
		// that materialize results. Gate visitors consume it immediately.
		if e = visit(row); e != nil {
			return e
		}
	}
	return rows.Err()
}
func (d db) query(statement string, columns int) ([][]string, error) {
	result := make([][]string, 0, 16)
	e := d.each(statement, columns, func(row []string) error {
		result = append(result, append([]string(nil), row...))
		return nil
	})
	return result, e
}
func (d db) tx(statements []string) error {
	if len(statements) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tx, e := d.handle.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, statement := range statements {
		if _, e = tx.ExecContext(ctx, statement); e != nil {
			return e
		}
	}
	return tx.Commit()
}

// A state transition succeeds only when every guarded write changed its one row.
func (d db) transition(statements []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tx, err := d.handle.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range statements {
		result, err := tx.ExecContext(ctx, statement)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return errors.New("state changed concurrently; transition was not committed")
		}
	}
	return tx.Commit()
}

var schema = []string{
	"CREATE TABLE IF NOT EXISTS graphs (id TEXT PRIMARY KEY, scope TEXT NOT NULL, approved INTEGER NOT NULL DEFAULT 0);",
	"CREATE TABLE IF NOT EXISTS business_models (graph TEXT PRIMARY KEY, schema_version INTEGER NOT NULL, body TEXT NOT NULL);",
	"CREATE TABLE IF NOT EXISTS nodes (graph TEXT NOT NULL, id TEXT NOT NULL, kind TEXT NOT NULL, name TEXT NOT NULL, PRIMARY KEY(graph,id));",
	"CREATE TABLE IF NOT EXISTS edges (graph TEXT NOT NULL, id TEXT NOT NULL, source TEXT NOT NULL, target TEXT NOT NULL, kind TEXT NOT NULL, PRIMARY KEY(graph,id));",
	"CREATE TABLE IF NOT EXISTS rules (graph TEXT NOT NULL, id TEXT NOT NULL, action TEXT NOT NULL, actor TEXT NOT NULL, entry TEXT NOT NULL, oracle TEXT NOT NULL, conditions TEXT NOT NULL, expected TEXT NOT NULL, rejected TEXT NOT NULL, source TEXT NOT NULL, description TEXT NOT NULL, approved INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(graph,id));",
	"CREATE TABLE IF NOT EXISTS risks (graph TEXT NOT NULL, id TEXT NOT NULL, action TEXT NOT NULL, description TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'OPEN', PRIMARY KEY(graph,id));",
	"CREATE TABLE IF NOT EXISTS tasks (id TEXT PRIMARY KEY, graph TEXT NOT NULL, rule TEXT NOT NULL, kind TEXT NOT NULL, condition TEXT NOT NULL, expected TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'READY');",
	"CREATE TABLE IF NOT EXISTS runs (id TEXT PRIMARY KEY, task TEXT NOT NULL, environment TEXT NOT NULL, commit_sha TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'RUNNING', created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE TABLE IF NOT EXISTS steps (run TEXT NOT NULL, idx INTEGER NOT NULL, expected TEXT NOT NULL, observed TEXT NOT NULL, verdict TEXT NOT NULL, decision TEXT NOT NULL, evidence_path TEXT NOT NULL, evidence_sha256 TEXT NOT NULL, PRIMARY KEY(run,idx));",
	"CREATE TABLE IF NOT EXISTS fixtures (id TEXT PRIMARY KEY, body TEXT NOT NULL);",
	"CREATE TABLE IF NOT EXISTS run_bindings (run TEXT PRIMARY KEY, body TEXT NOT NULL);",
	"CREATE TABLE IF NOT EXISTS fixture_locks (fixture TEXT PRIMARY KEY, run TEXT NOT NULL);",
	"CREATE TABLE IF NOT EXISTS fixture_checkpoints (run TEXT PRIMARY KEY, task TEXT NOT NULL, binding TEXT NOT NULL, evidence_path TEXT NOT NULL, evidence_sha256 TEXT NOT NULL);",
	"CREATE TABLE IF NOT EXISTS plan_cursor (graph TEXT PRIMARY KEY, rule_id TEXT NOT NULL, offset INTEGER NOT NULL);",
	"CREATE TABLE IF NOT EXISTS model_plan_cursor (graph TEXT PRIMARY KEY, position INTEGER NOT NULL);",
	"CREATE TABLE IF NOT EXISTS verification_rounds (id TEXT PRIMARY KEY, graph TEXT NOT NULL, environment TEXT NOT NULL, body TEXT NOT NULL);",
	"CREATE TABLE IF NOT EXISTS registered_runners (id TEXT PRIMARY KEY, graph TEXT NOT NULL, test TEXT NOT NULL, body TEXT NOT NULL, sha256 TEXT NOT NULL, code_sha256 TEXT NOT NULL);",
	"CREATE TABLE IF NOT EXISTS model_runs (run TEXT PRIMARY KEY, round TEXT NOT NULL, runner TEXT NOT NULL, body TEXT NOT NULL);",
	"CREATE TABLE IF NOT EXISTS model_captures (run TEXT NOT NULL, idx INTEGER NOT NULL, sha256 TEXT NOT NULL, PRIMARY KEY(run,idx,sha256));",
	"CREATE TABLE IF NOT EXISTS model_recoveries (id INTEGER PRIMARY KEY AUTOINCREMENT, run TEXT NOT NULL, body TEXT NOT NULL);",
	"INSERT OR IGNORE INTO tasks(id,graph,rule,kind,condition,expected,status) SELECT graph||'/_schema/v2',graph,'','MODEL_SCHEMA_GUARD','','','BLOCKED' FROM business_models WHERE schema_version=2;",
	"CREATE INDEX IF NOT EXISTS idx_rounds_graph ON verification_rounds(graph,environment,id);",
	"CREATE INDEX IF NOT EXISTS idx_model_runs_round ON model_runs(round,run);",
	"CREATE INDEX IF NOT EXISTS idx_edges_reverse ON edges(graph,kind,target,source);",
	"CREATE INDEX IF NOT EXISTS idx_rules_action ON rules(graph,action,approved);",
	"CREATE INDEX IF NOT EXISTS idx_tasks_ready ON tasks(graph,status,id);",
	"CREATE INDEX IF NOT EXISTS idx_rules_approved ON rules(graph,approved,id);",
	"CREATE INDEX IF NOT EXISTS idx_runs_task_history ON runs(task,id);",
	"CREATE TABLE IF NOT EXISTS task_events (id INTEGER PRIMARY KEY AUTOINCREMENT, task TEXT NOT NULL, run TEXT NOT NULL, event TEXT NOT NULL, reason TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE TABLE IF NOT EXISTS implementation_scans (id INTEGER PRIMARY KEY AUTOINCREMENT, graph TEXT NOT NULL, action TEXT NOT NULL, status TEXT NOT NULL, evidence TEXT NOT NULL, sha256 TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE INDEX IF NOT EXISTS idx_scans_action_history ON implementation_scans(graph,action,id DESC);",
	"CREATE TABLE IF NOT EXISTS implementation_scan_candidates (scan INTEGER PRIMARY KEY, commit_sha TEXT NOT NULL);",
	"CREATE TABLE IF NOT EXISTS derived_obligations (id TEXT PRIMARY KEY, graph TEXT NOT NULL, template TEXT NOT NULL, origin TEXT NOT NULL, base TEXT NOT NULL, rule TEXT NOT NULL, start_state TEXT NOT NULL, equivalence_key TEXT NOT NULL, score INTEGER NOT NULL, source_run TEXT NOT NULL, source_step INTEGER NOT NULL, reason TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE INDEX IF NOT EXISTS idx_derived_graph ON derived_obligations(graph,origin,id);",
	"CREATE TABLE IF NOT EXISTS derivation_sources (derived TEXT NOT NULL, run TEXT NOT NULL, step INTEGER NOT NULL, PRIMARY KEY(derived,run));",
	"CREATE TABLE IF NOT EXISTS unspecified_items (id TEXT PRIMARY KEY, graph TEXT NOT NULL, kind TEXT NOT NULL, identity TEXT NOT NULL, entry TEXT NOT NULL, role TEXT NOT NULL, principal TEXT NOT NULL, action TEXT NOT NULL, before_state TEXT NOT NULL, after_state TEXT NOT NULL, denied INTEGER NOT NULL, reason TEXT NOT NULL, evidence TEXT NOT NULL, evidence_sha256 TEXT NOT NULL, equivalence_key TEXT NOT NULL, base_key TEXT NOT NULL, exploration TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE INDEX IF NOT EXISTS idx_unspecified_base ON unspecified_items(graph,identity,base_key);",
	"CREATE TABLE IF NOT EXISTS unspecified_sources (item TEXT NOT NULL, exploration TEXT NOT NULL, evidence_sha256 TEXT NOT NULL, PRIMARY KEY(item,evidence_sha256));",
	"CREATE TABLE IF NOT EXISTS unspecified_triage (seq INTEGER PRIMARY KEY AUTOINCREMENT, graph TEXT NOT NULL, item TEXT NOT NULL, decision TEXT NOT NULL, words TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE TABLE IF NOT EXISTS reference_catalogs (sha256 TEXT PRIMARY KEY, name TEXT NOT NULL, body TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE TABLE IF NOT EXISTS reference_matches (seq INTEGER PRIMARY KEY AUTOINCREMENT, graph TEXT NOT NULL, entry TEXT NOT NULL, catalog_sha256 TEXT NOT NULL, outcome TEXT NOT NULL, detail TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE TABLE IF NOT EXISTS reference_current (graph TEXT NOT NULL, risk TEXT NOT NULL, entry TEXT NOT NULL, reference TEXT NOT NULL, action TEXT NOT NULL, description TEXT NOT NULL, PRIMARY KEY(graph,risk));",
	"CREATE TABLE IF NOT EXISTS risk_refutations (seq INTEGER PRIMARY KEY AUTOINCREMENT, graph TEXT NOT NULL, risk TEXT NOT NULL, result TEXT NOT NULL, evidence TEXT NOT NULL, sha256 TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE TABLE IF NOT EXISTS reference_rejections (seq INTEGER PRIMARY KEY AUTOINCREMENT, graph TEXT NOT NULL, entry TEXT NOT NULL, bindings TEXT NOT NULL, words TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE TABLE IF NOT EXISTS derivation_runs (run TEXT PRIMARY KEY, graph TEXT NOT NULL, derived INTEGER NOT NULL, truncated INTEGER NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE TABLE IF NOT EXISTS risk_meta (graph TEXT NOT NULL, id TEXT NOT NULL, origin TEXT NOT NULL, reference TEXT NOT NULL, PRIMARY KEY(graph,id));",
	"CREATE TABLE IF NOT EXISTS risk_events (seq INTEGER PRIMARY KEY AUTOINCREMENT, graph TEXT NOT NULL, risk TEXT NOT NULL, status TEXT NOT NULL, scope TEXT NOT NULL, quote TEXT NOT NULL, rule TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE INDEX IF NOT EXISTS idx_risk_events ON risk_events(graph,risk,seq);",
	"CREATE TABLE IF NOT EXISTS exploration_campaigns (id TEXT PRIMARY KEY, graph TEXT NOT NULL, flow TEXT NOT NULL, environment TEXT NOT NULL, round TEXT NOT NULL, stop_reason TEXT NOT NULL, body TEXT NOT NULL, created TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE INDEX IF NOT EXISTS idx_exploration_campaigns ON exploration_campaigns(graph,created);",
	"CREATE TABLE IF NOT EXISTS exploration_frontier (id TEXT PRIMARY KEY, campaign TEXT NOT NULL, equivalence_key TEXT NOT NULL, state TEXT NOT NULL, role TEXT NOT NULL, body TEXT NOT NULL);",
	"CREATE INDEX IF NOT EXISTS idx_exploration_frontier ON exploration_frontier(campaign,equivalence_key);",
	"CREATE TABLE IF NOT EXISTS exploration_actions (seq INTEGER PRIMARY KEY AUTOINCREMENT, campaign TEXT NOT NULL, candidate TEXT NOT NULL, run TEXT NOT NULL, verdict TEXT NOT NULL, body TEXT NOT NULL);",
	"CREATE INDEX IF NOT EXISTS idx_exploration_actions ON exploration_actions(campaign,seq);",
	"CREATE TABLE IF NOT EXISTS exploration_state (campaign TEXT PRIMARY KEY, graph TEXT NOT NULL, status TEXT NOT NULL, model_sha TEXT NOT NULL, source_commit TEXT NOT NULL, claimed_by TEXT NOT NULL DEFAULT '', claimed_at TEXT NOT NULL DEFAULT '', checkpoint TEXT NOT NULL, body TEXT NOT NULL, updated TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);",
	"CREATE INDEX IF NOT EXISTS idx_exploration_state ON exploration_state(graph,status,updated);",
}
