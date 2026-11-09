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
	"os/exec"
	"path/filepath"
	"time"
)

// arc index flush is an explicitly batched outbox consumer. The current
// arc-idx backend rebuilds the full index; do not label this incremental.
// Source events are acknowledged only after a real successful index and
// healthy doctor result. Worktree fingerprints protect against concurrent edits.
func indexCommand(args []string) error {
	if len(args) == 0 || args[0] != "flush" {
		return errors.New("usage: arc index flush [--repo DIR]")
	}
	flags := flag.NewFlagSet("arc index flush", flag.ContinueOnError)
	repo := flags.String("repo", ".", "target repository")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if len(flags.Args()) != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	root, err := repositoryRoot(*repo)
	if err != nil {
		return err
	}
	return flushIndexOutbox(root)
}

type outboxBatch struct {
	Watermark int64
	Count     int
}

func isGeneratedIndexFile(path string) bool {
	switch filepath.ToSlash(path) {
	case ".ai-code-index/tags.json", ".ai-code-index/daemon.pid", ".ai-code-index/daemon.log":
		return true
	}
	return false
}

func pendingIndexBatch(db *sql.DB) (outboxBatch, error) {
	var batch outboxBatch
	err := db.QueryRow("SELECT COALESCE(MAX(o.rowid),0), COUNT(*) FROM arc_outbox o JOIN arc_events e ON e.id=o.event_id WHERE o.status='pending' AND e.aggregate IN ('file','change')").
		Scan(&batch.Watermark, &batch.Count)
	return batch, err
}

// Current HEAD + dirty/untracked path content, not merely Git mtime.
// Captures the worktree's indexable source candidates while excluding .arc.
func sourceSnapshot(root string) (string, error) {
	head, err := gitHead(root)
	if err != nil {
		return "", err
	}
	changed, err := changedGitPaths(root)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "head=%s\n", head)
	for _, item := range changed {
		if isGeneratedIndexFile(item.Path) { continue }
		sha, size, op, err := observedFile(root, item.Path)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(h, "%d:%s:%s:%s:%d:%s\n", len(item.Path), item.Path, item.Operation, sha, size, op)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// A healthy search index does not repair stale file metadata. Before
// acknowledging events, every dirty worktree path and pending file event must
// match the latest arc_files observation, or reconciliation must run again.
func recordedFilesCurrent(root string) error {
	dirty,err:=changedGitPaths(root)
	if err!=nil{return err}
	paths:=map[string]bool{}
	for _,item:=range dirty {paths[item.Path]=true}
	db,err:=openActiveArcDB(root)
	if err!=nil{return err}
	defer db.Close()
	rows,err:=db.Query(
		"SELECT DISTINCT e.aggregate_id FROM arc_outbox o JOIN arc_events e ON e.id=o.event_id WHERE o.status='pending' AND e.aggregate='file'")
	if err!=nil{return err}
	for rows.Next(){
		var path string
		if err:=rows.Scan(&path);err!=nil{rows.Close();return err}
		paths[path]=true
		if len(paths)>1000{rows.Close();return errors.New("index candidate set exceeds 1000 files; narrow the task")}
	}
	if err:=rows.Err();err!=nil{rows.Close();return err}
	if err:=rows.Close();err!=nil{return err}
	for path:=range paths {
		var recorded string
		if err:=db.QueryRow("SELECT sha256 FROM arc_files WHERE path=?",filepath.ToSlash(path)).Scan(&recorded);err!=nil{
			if errors.Is(err,sql.ErrNoRows){return fmt.Errorf("%s has no recorded file observation; run arc sync reconcile",path)}
			return err
		}
		current,_,_,err:=observedFile(root,path)
		if err!=nil{return err}
		if current!=recorded{
			return fmt.Errorf("%s changed since its recorded file observation; run arc sync reconcile",path)
		}
	}
	return nil
}

type indexDoctor struct {
	RepoRoot    string `json:"repo_root"`
	Healthy     bool   `json:"healthy"`
	IndexStale  bool   `json:"index_stale"`
	TagsPresent bool   `json:"tags_present"`
}

func rebuildIndex(ctx context.Context, root string) error {
	bin, err := resolveBackend("arc-idx")
	if err != nil {
		return fmt.Errorf("index backend unavailable (outbox remains pending): %w", err)
	}
	run := exec.CommandContext(ctx, bin, "index", "--repo", root, "--quiet", "--require-complete")
	if out, err := run.CombinedOutput(); err != nil {
		return fmt.Errorf("arc-idx index failed: %w: %s", err, string(out))
	}
	doctor := exec.CommandContext(ctx, bin, "doctor", "--repo", root, "--format", "json")
	body, err := doctor.Output()
	if err != nil {
		return fmt.Errorf("arc-idx doctor failed after rebuild: %w", err)
	}
	var result indexDoctor
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("index doctor returned invalid JSON: %w", err)
	}
	actual, err := filepath.EvalSymlinks(result.RepoRoot)
	if err != nil || filepath.Clean(actual) != filepath.Clean(root) {
		return fmt.Errorf("index doctor refers to another repository: %q", result.RepoRoot)
	}
	if !result.Healthy || result.IndexStale || !result.TagsPresent {
		return errors.New("index rebuild did not produce healthy, fresh source and symbol indexes")
	}
	return nil
}

// UPDATE uses the snapshot rowid watermark, never rows arriving while
// the external index is building. Replays are safe after crashes.
func markIndexBatch(root string, batch outboxBatch, indexed bool, reason string) (int64, error) {
	var updated int64
	err := withArcLock(root, func() error {
		db, err := openActiveArcDB(root)
		if err != nil {
			return err
		}
		defer db.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		return dbTransaction(ctx, db, func(tx *sql.Tx) error {
			var result sql.Result
			if indexed {
				result, err = tx.ExecContext(ctx,
					"UPDATE arc_outbox SET status='delivered',attempts=attempts+1 WHERE status='pending' AND rowid<=? AND event_id IN (SELECT id FROM arc_events WHERE aggregate IN ('file','change'))",
					batch.Watermark)
			} else {
				result, err = tx.ExecContext(ctx,
					"UPDATE arc_outbox SET attempts=attempts+1 WHERE status='pending' AND rowid<=? AND event_id IN (SELECT id FROM arc_events WHERE aggregate IN ('file','change'))",
					batch.Watermark)
			}
			if err != nil {
				return err
			}
			updated, err = result.RowsAffected()
			if err != nil {
				return err
			}
			lastError := reason
			if len(lastError) > 1024 {
				lastError = lastError[:1024]
			}
			if _, err = tx.ExecContext(ctx,
				"INSERT OR REPLACE INTO arc_meta(key,value) VALUES('index_last_error',?)", lastError); err != nil {
				return err
			}
			if indexed {
				if _, err = tx.ExecContext(ctx,
					"INSERT OR REPLACE INTO arc_meta(key,value) VALUES('index_last_success',?)", nowUTC()); err != nil {
					return err
				}
			}
			return nil
		})
	})
	return updated, err
}

func flushIndexOutbox(root string) error {
	return flushIndexOutboxWith(root,rebuildIndex)
}

// Injectable build function permits deterministic failure/race/retry testing
// without pretending a mock backend proves the production Zoekt/ctags build.
func flushIndexOutboxWith(root string, build func(context.Context,string) error) error {
	var batch outboxBatch
	err := withArcLock(root, func() error {
		db, err := openActiveArcDB(root)
		if err != nil {
			return err
		}
		defer db.Close()
		batch, err = pendingIndexBatch(db)
		if err != nil { return err }
		if batch.Count == 0 {
			var success string
			err = db.QueryRow("SELECT value FROM arc_meta WHERE key='index_last_success'").Scan(&success)
			if err != nil && !errors.Is(err,sql.ErrNoRows) { return err }
			if success != "" {
				var indexedSnapshot string
				if err := db.QueryRow("SELECT value FROM arc_meta WHERE key='index_last_snapshot'").Scan(&indexedSnapshot); err != nil {
					return fmt.Errorf("index snapshot metadata missing: %w", err)
				}
				current, err := sourceSnapshot(root)
				if err != nil { return err }
				if current != indexedSnapshot {
					return errors.New("source changed without a pending index event; run arc sync reconcile first")
				}
				return nil
			}
			// No outbox events exist, but an initial source index was never
			// confirmed. Rebuild once so a later task completion has proof.
			batch.Count = -1
		}
		return nil
	})
	if err != nil {
		return err
	}
	if batch.Count == 0 {
		return emitJSON(map[string]any{"indexed_events": 0, "pending": 0, "index": "no_pending_work"})
	}

	if err := recordedFilesCurrent(root); err != nil { return err }
	before, err := sourceSnapshot(root)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	indexErr := build(ctx, root)
	after, snapshotErr := sourceSnapshot(root)
	if indexErr == nil && snapshotErr != nil {
		indexErr = snapshotErr
	}
	if indexErr == nil && before != after {
		indexErr = errors.New("source changed during indexing: leave events pending and retry")
	}
	if indexErr != nil {
		_, recordErr := markIndexBatch(root, batch, false, indexErr.Error())
		if recordErr != nil {
			return fmt.Errorf("index failed (%v); recording retry failed: %w", indexErr, recordErr)
		}
		return indexErr
	}

	// Check again while holding the DB lock before accepting the snapshot:
	// only the original batch may be acknowledged.
	var processed int64
	err = withArcLock(root, func() error {
		current, err := sourceSnapshot(root)
		if err != nil {
			return err
		}
		if current != before {
			return errors.New("source changed before index acknowledgement; events remain pending")
		}
		if err:=recordedFilesCurrent(root);err!=nil{return err}
		db, err := openActiveArcDB(root)
		if err != nil {
			return err
		}
		defer db.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return dbTransaction(ctx, db, func(tx *sql.Tx) error {
			result, err := tx.ExecContext(ctx,
				"UPDATE arc_outbox SET status='delivered',attempts=attempts+1 WHERE status='pending' AND rowid<=? AND event_id IN (SELECT id FROM arc_events WHERE aggregate IN ('file','change'))",
				batch.Watermark)
			if err != nil { return err }
			processed, err = result.RowsAffected()
			if err != nil { return err }
			if _, err = tx.ExecContext(ctx,
				"INSERT OR REPLACE INTO arc_meta(key,value) VALUES('index_last_success',?)", nowUTC()); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx,
				"INSERT OR REPLACE INTO arc_meta(key,value) VALUES('index_last_snapshot',?)", before); err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx,
				"INSERT OR REPLACE INTO arc_meta(key,value) VALUES('index_last_error','')")
			return err
		})
	})
	if err != nil { return err }
	return emitJSON(map[string]any{"indexed_events": processed, "index": "healthy", "backend": "arc-idx-full-rebuild", "worktree_snapshot": before})
}
