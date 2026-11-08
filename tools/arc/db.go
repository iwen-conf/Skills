package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "turso.tech/database/tursogo"
)

const arcSchemaVersion = 1
const arcDBName = ".arc/arc.db"
const arcManifestName = ".arc/arc.json"
const legacyVerifyDB = ".arc/verify/data.db"

type arcManifest struct {
	Version       int    `json:"schema_version"`
	Active        bool   `json:"active"`
	DB            string `json:"db"`
	LegacyDB      string `json:"legacy_db,omitempty"`
	LegacySHA256  string `json:"legacy_sha256,omitempty"`
	MigrationTime string `json:"migrated_at,omitempty"`
	ParityGraphs  int    `json:"parity_graphs"`
}

var arcSchema = []string{
	`CREATE TABLE IF NOT EXISTS arc_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_tasks (id TEXT PRIMARY KEY, title TEXT NOT NULL, status TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1, created TEXT NOT NULL, updated TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_files (path TEXT PRIMARY KEY, sha256 TEXT NOT NULL, bytes INTEGER NOT NULL, version INTEGER NOT NULL, changed TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_changes (id TEXT PRIMARY KEY, task TEXT NOT NULL, fingerprint TEXT NOT NULL UNIQUE, status TEXT NOT NULL, worktree TEXT NOT NULL, revision TEXT NOT NULL, created TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_change_files (change_id TEXT NOT NULL, path TEXT NOT NULL, sha256 TEXT NOT NULL, operation TEXT NOT NULL, PRIMARY KEY (change_id,path))`,
	`CREATE TABLE IF NOT EXISTS arc_events (id TEXT PRIMARY KEY, aggregate TEXT NOT NULL, aggregate_id TEXT NOT NULL, kind TEXT NOT NULL, payload TEXT NOT NULL, created TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_outbox (event_id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0)`,
	`CREATE INDEX IF NOT EXISTS arc_events_aggregate ON arc_events(aggregate,aggregate_id,created)`,
	`CREATE INDEX IF NOT EXISTS arc_changes_task ON arc_changes(task,created)`,
}

func repositoryRoot(arg string) (string, error) {
	abs, err := filepath.Abs(arg)
	if err != nil { return "", err }
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil { return "", err }
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() { return "", fmt.Errorf("not a repository directory: %s", abs) }
	if arg == "." {
		for dir := abs; ; dir = filepath.Dir(dir) {
			if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil { return dir, nil }
			if filepath.Dir(dir) == dir { break }
		}
	}
	return abs, nil
}

func statePath(root string) string { return filepath.Join(root, arcDBName) }
func manifestPath(root string) string { return filepath.Join(root, arcManifestName) }

func readManifest(root string) (*arcManifest, error) {
	raw, err := os.ReadFile(manifestPath(root))
	if errors.Is(err, os.ErrNotExist) { return nil, nil }
	if err != nil { return nil, err }
	var m arcManifest
	if err := json.Unmarshal(raw, &m); err != nil { return nil, fmt.Errorf("invalid arc manifest: %w", err) }
	if m.Version != arcSchemaVersion || m.DB != arcDBName || !m.Active {
		return nil, errors.New("arc manifest has unsupported schema or inactive state (fail closed)")
	}
	if err := checkRegular(statePath(root)); err != nil { return nil, fmt.Errorf("active arc db: %w", err) }
	return &m, nil
}

func checkRegular(path string) error {
	st, err := os.Lstat(path)
	if err != nil { return err }
	if !st.Mode().IsRegular() { return fmt.Errorf("not an ordinary file: %s", path) }
	return nil
}

func openArcDB(path string) (*sql.DB, error) {
	db, err := sql.Open("turso", path)
	if err != nil { return nil, err }
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil { db.Close(); return nil, err }
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil { db.Close(); return nil, err }
	return db, nil
}

func createArcSchema(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()
	for _, stmt := range arcSchema {
		if _, err := tx.ExecContext(ctx, stmt); err != nil { return fmt.Errorf("arc schema: %w", err) }
	}
	if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO arc_meta(key,value) VALUES('schema_version',?)", fmt.Sprint(arcSchemaVersion)); err != nil { return err }
	var v string
	if err := tx.QueryRowContext(ctx, "SELECT value FROM arc_meta WHERE key='schema_version'").Scan(&v); err != nil { return err }
	if v != fmt.Sprint(arcSchemaVersion) { return fmt.Errorf("arc schema version mismatch: %s", v) }
	return tx.Commit()
}

func shaFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil { return "", err }
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil { return "", err }
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFileExact(src, dest string) error {
	in, err := os.Open(src)
	if err != nil { return err }
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil { return err }
	_, e := io.Copy(out, in)
	if e == nil { e = out.Sync() }
	if closeErr := out.Close(); e == nil { e = closeErr }
	if e != nil { os.Remove(dest) }
	return e
}

func noLiveSidecars(src string) error {
	for _, ext := range []string{"-wal", "-shm", "-journal", ".wal", ".shm", ".mvcc", "-mvcc", ".lock"} {
		file := src + ext
		st, err := os.Lstat(file)
		if errors.Is(err, os.ErrNotExist) { continue }
		if err != nil { return err }
		if !st.Mode().IsRegular() || st.Size() > 0 { return fmt.Errorf("legacy database has a live or unsafe sidecar %s; quiesce and checkpoint first", file) }
	}
	return nil
}

func writeManifest(root string, m arcManifest) error {
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil { return err }
	path := manifestPath(root)
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_EXCL|os.O_CREATE, 0600)
	if err != nil { return err }
	_, e := f.Write(append(body, '
'))
	if e == nil { e = f.Sync() }
	if closeErr := f.Close(); e == nil { e = closeErr }
	if e != nil { os.Remove(tmp); return e }
	if err := os.Rename(tmp, path); err != nil { os.Remove(tmp); return err }
	return nil
}

func runLegacyVerify(root, dbfile, command string, extra ...string) ([]byte, int, error) {
	bin, err := resolveBackend("arc-verify")
	if err != nil { return nil, 127, fmt.Errorf("arc-verify backend unavailable: %w", err) }
	args := append([]string{command, "--db", dbfile}, extra...)
	cmd := exec.Command(bin, args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err == nil { return out, 0, nil }
	var exit *exec.ExitError
	if errors.As(err, &exit) { return out, exit.ExitCode(), nil }
	return nil, 1, err
}

// Gate parity is only meaningful for structured verifier output. Identical
// empty text or an exit-zero stub must not activate the new database.
func validateGateProof(body []byte, exitCode int, graph string) error {
	var proof map[string]any
	if err := json.Unmarshal(body, &proof); err != nil { return fmt.Errorf("graph %q gate did not return JSON evidence: %w", graph, err) }
	verdict, ok := proof["verdict"].(string)
	if !ok { return fmt.Errorf("graph %q gate missing verdict", graph) }
	switch verdict {
	case "PASS":
		if exitCode != 0 { return fmt.Errorf("graph %q returned PASS with exit %d",graph,exitCode) }
		scope, valid := proof["proof_scope"].(string)
		if !valid || (scope!="planned_rule_tasks" && scope!="complete_business_model_obligations") { return fmt.Errorf("graph %q PASS lacks proof scope",graph) }
		if proof["inputs_unchanged"]!=true { return fmt.Errorf("graph %q PASS lacks unchanged-input attestation",graph) }
		context, valid := proof["context"].(map[string]any)
		if !valid || context["graph"]!=graph { return fmt.Errorf("graph %q PASS has no matching graph context",graph) }
	case "FAIL","BLOCKED","NOT_RUN","FLAKY":
		if exitCode==0 {return fmt.Errorf("graph %q non-PASS verdict %q returned exit 0",graph,verdict)}
	default:
		return fmt.Errorf("graph %q unsupported verdict %q",graph,verdict)
	}
	return nil
}

// parity checks the original gate verdict and full structured output for every
// graph. Any missing backend or divergent check aborts activation.
func verifyParity(root, oldDB, newDB string) (int, error) {
	db, err := openArcDB(oldDB)
	if err != nil { return 0, err }
	rows, err := db.Query("SELECT id FROM graphs ORDER BY id")
	if err != nil { db.Close(); return 0, fmt.Errorf("legacy graph schema missing: %w", err) }
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil { rows.Close(); db.Close(); return 0, err }
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	db.Close()
	if err != nil { return 0, err }
	sort.Strings(ids)
	for _, graph := range ids {
		a, exitA, err := runLegacyVerify(root, oldDB, "gate", "--graph", graph)
		if err != nil { return 0, err }
		b, exitB, err := runLegacyVerify(root, newDB, "gate", "--graph", graph)
		if err != nil { return 0, err }
		if err := validateGateProof(a,exitA,graph); err != nil {return 0,fmt.Errorf("legacy gate not trustworthy: %w",err)}
		if err := validateGateProof(b,exitB,graph); err != nil {return 0,fmt.Errorf("migrated gate not trustworthy: %w",err)}
		if exitA != exitB || string(a) != string(b) {
			return 0, fmt.Errorf("gate parity differs for graph %q (legacy exit %d, new exit %d); arc.db NOT activated", graph, exitA, exitB)
		}
	}
	return len(ids), nil
}

func dbCommand(args []string) error {
	if len(args) == 0 { return errors.New("usage: arc db <status|init|migrate> [--repo DIR] [--confirm-offline]") }
	action := args[0]
	fs := flag.NewFlagSet("arc db "+action, flag.ContinueOnError)
	rootFlag := fs.String("repo", ".", "repository directory")
	confirm := fs.Bool("confirm-offline", false, "assert all legacy verifier processes are stopped")
	if err := fs.Parse(args[1:]); err != nil { return err }
	if len(fs.Args()) != 0 { return fmt.Errorf("unexpected arguments %v", fs.Args()) }
	root, err := repositoryRoot(*rootFlag)
	if err != nil { return err }
	switch action {
	case "status":
		m, err := readManifest(root)
		if err != nil { return err }
		status := map[string]any{"path": statePath(root), "active": m != nil, "schema_version": arcSchemaVersion}
		if m != nil { status["legacy_db"] = m.LegacyDB; status["parity_graphs"] = m.ParityGraphs }
		return json.NewEncoder(os.Stdout).Encode(status)
	case "init", "migrate":
		return withArcLock(root, func() error {
			if m, err := readManifest(root); err != nil { return err } else if m != nil { return errors.New("arc database is already active") }
			if _, err := os.Lstat(statePath(root)); err == nil { return errors.New("unactivated arc.db already exists; inspect/recover explicitly, refusing overwrite") } else if !errors.Is(err, os.ErrNotExist) { return err }
			if _, err := os.Lstat(manifestPath(root)); err == nil { return errors.New("arc manifest already exists; refusing overwrite") } else if !errors.Is(err, os.ErrNotExist) { return err }
			if err := os.MkdirAll(filepath.Join(root, ".arc"), 0700); err != nil { return err }
			m := arcManifest{Version: arcSchemaVersion, Active: true, DB: arcDBName}
			if action == "migrate" {
				if !*confirm { return errors.New("migration requires --confirm-offline after stopping all legacy verification writers") }
				source := filepath.Join(root, legacyVerifyDB)
				if err := checkRegular(source); err != nil { return fmt.Errorf("legacy verification DB: %w", err) }
				if err := noLiveSidecars(source); err != nil { return err }
				original, err := shaFile(source); if err != nil { return err }
				dir := filepath.Join(root, ".arc", "backups")
				if err := os.MkdirAll(dir, 0700); err != nil { return err }
				backup := filepath.Join(dir, "verify-"+original[:16]+".db")
				if _, err := os.Lstat(backup); errors.Is(err, os.ErrNotExist) {
					if err := copyFileExact(source, backup); err != nil { return err }
				} else if err != nil { return err }
				backupHash, err := shaFile(backup)
				if err != nil || backupHash != original { return errors.New("legacy backup checksum mismatch; aborting") }
				if err := copyFileExact(source, statePath(root)); err != nil { return err }
				copied, err := shaFile(statePath(root))
				if err != nil || copied != original { return errors.New("database changed while copying; arc.db remains inactive") }
				m.LegacyDB, m.LegacySHA256 = legacyVerifyDB, original
				m.MigrationTime = time.Now().UTC().Format(time.RFC3339)
			} else {
				if err := checkRegular(filepath.Join(root, legacyVerifyDB)); err == nil {
					return errors.New("legacy graph database exists; use arc db migrate --confirm-offline instead of init")
				} else if !errors.Is(err, os.ErrNotExist) { return err }
				if _, code, err := runLegacyVerify(root, statePath(root), "init"); err != nil || code != 0 {
					return fmt.Errorf("initialize verification schema: error=%v exit=%d", err, code)
				}
			}
			db, err := openArcDB(statePath(root))
			if err != nil { return err }
			err = createArcSchema(db)
			db.Close()
			if err != nil { return err }
			if action == "migrate" {
				if err := noLiveSidecars(filepath.Join(root, legacyVerifyDB)); err != nil { return err }
				h, err := shaFile(filepath.Join(root, legacyVerifyDB))
				if err != nil || h != m.LegacySHA256 { return errors.New("legacy database changed during migration; arc.db NOT activated") }
				n, err := verifyParity(root, filepath.Join(root, legacyVerifyDB), statePath(root))
				if err != nil { return err }
				m.ParityGraphs = n
			}
			if err := writeManifest(root, m); err != nil { return err }
			return json.NewEncoder(os.Stdout).Encode(map[string]any{"database": statePath(root), "active": true, "migrated_graphs": m.ParityGraphs})
		})
	default:
		return fmt.Errorf("unknown arc db action %q", action)
	}
}

func openActiveArcDB(root string) (*sql.DB, error) {
	if m, err := readManifest(root); err != nil { return nil, err } else if m == nil { return nil, errors.New("arc.db not active; use arc db init or explicit arc db migrate") }
	return openArcDB(statePath(root))
}

func isArcDBArgument(args []string) bool {
	for i, a := range args {
		if a == "--db" && i+1 < len(args) { return true }
		if strings.HasPrefix(a, "--db=") { return true }
	}
	return false
}
