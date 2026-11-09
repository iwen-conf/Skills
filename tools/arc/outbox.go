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
	"os"
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

func pendingIndexBatch(db *sql.DB) (outboxBatch, error) {
	var batch outboxBatch
	err := db.QueryRow("SELECT COALESCE(MAX(rowid),0), COUNT(*) FROM arc_outbox WHERE status='pending'").
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
		sha, size, op, err := observedFile(root, item.Path)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(h, "%d:%s:%s:%s:%d:%s\n", len(item.Path), item.Path, item.Operation, sha, size, op)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
	run := exec.CommandContext(ctx, bin, "index", "--repo", root, "--quiet")
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
					"UPDATE arc_outbox SET status='delivered',attempts=attempts+1 WHERE status='pending' AND rowid<=?",
					batch.Watermark)
			} else {
				result, err = tx.ExecContext(ctx,
					"UPDATE arc_outbox SET attempts=attempts+1 WHERE status='pending' AND rowid<=?",
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
	var batch outboxBatch
	err := withArcLock(root, func() error {
		db, err := openActiveArcDB(root)
		if err != nil {
			return err
		}
		defer db.Close()
		batch, err = pendingIndexBatch(db)
		return err
	})
	if err != nil {
		return err
	}
	if batch.Count == 0 {
		return emitJSON(map[string]any{"indexed_events": 0, "pending": 0, "index": "no_pending_work"})
	}

	before, err := sourceSnapshot(root)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	indexErr := rebuildIndex(ctx, root)
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
		db, err := openActiveArcDB(root)
		if err != nil {
			return err
		}
		defer db.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		return dbTransaction(ctx, db, func(tx *sql.Tx) error {
			result, err := tx.ExecContext(ctx,
				"UPDATE arc_outbox SET status='delivered',attempts=attempts+1 WHERE status='pending' AND rowid<=?",
				batch.Watermark)
			if err != nil { return err }
			processed, err = result.RowsAffected()
			if err != nil { return err }
			if _, err = tx.ExecContext(ctx,
				"INSERT OR REPLACE INTO arc_meta(key,value) VALUES('index_last_success',?)", nowUTC()); err != nil {
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
