package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func checkpointBaseDir(repoRoot, storyID string) string {
	return filepath.Join(repoRoot, ".arc", "verify", "checkpoints", sanitizeNS(storyID))
}

func cloneBaseDir(repoRoot, storyID string) string {
	return filepath.Join(repoRoot, ".arc", "verify", "clones", sanitizeNS(storyID))
}

// buildCheckpoints builds every checkpoint in topological order, persisting snapshots,
// verifying optional assert_cmd preconditions, and recording SHA-256 digests in StoryLedger.
func buildCheckpoints(ledger *StoryLedger, s *Storyline, repoRoot, engineOverride, sourceFile, pgURL string) ([]CheckpointRecord, error) {
	lint := lintStory(s, nil, 10)
	if lint.Status != "PASS" {
		return nil, fmt.Errorf("storyline lint failed: %s", lint.Errors[0].Message)
	}
	cpMap := map[string]Checkpoint{}
	for _, cp := range s.Checkpoints {
		cpMap[cp.SaveID] = cp
	}

	var records []CheckpointRecord
	for _, saveID := range lint.CheckpointOrder {
		cp := cpMap[saveID]
		engine := strings.TrimSpace(engineOverride)
		if engine == "" {
			engine = strings.TrimSpace(cp.Engine)
		}
		if engine == "" {
			engine = "dir"
		}
		rec, err := materializeCheckpoint(s, cp, repoRoot, engine, sourceFile, pgURL)
		if err != nil {
			return nil, fmt.Errorf("checkpoint %s: %w", saveID, err)
		}
		if ledger != nil {
			if err := ledger.RecordCheckpoint(rec); err != nil {
				return nil, err
			}
		}
		records = append(records, rec)
	}
	return records, nil
}

func materializeCheckpoint(s *Storyline, cp Checkpoint, repoRoot, engine, sourceFile, pgURL string) (CheckpointRecord, error) {
	baseDir := checkpointBaseDir(repoRoot, s.StoryID)
	saveDir := filepath.Join(baseDir, sanitizeNS(cp.SaveID))
	if err := os.MkdirAll(saveDir, 0o755); err != nil {
		return CheckpointRecord{}, err
	}

	// Copy parent checkpoint state if chaining from an earlier checkpoint.
	if cp.Parent != "" && cp.Parent != "root" {
		parentDir := filepath.Join(baseDir, sanitizeNS(cp.Parent))
		if _, err := os.Stat(parentDir); err == nil {
			if err := copyTree(parentDir, saveDir); err != nil {
				return CheckpointRecord{}, fmt.Errorf("inherit parent %s: %w", cp.Parent, err)
			}
		}
	}

	snapshotPath := saveDir
	switch engine {
	case "sqlite":
		dst := filepath.Join(saveDir, "checkpoint.db")
		if sourceFile != "" {
			if err := copyFileBytes(sourceFile, dst); err != nil {
				return CheckpointRecord{}, fmt.Errorf("copy sqlite source: %w", err)
			}
		} else if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(dst, nil, 0o644); err != nil {
				return CheckpointRecord{}, err
			}
		}
		snapshotPath = dst
	case "postgres":
		dbName := fmt.Sprintf("arc_save_%s_%s", sanitizeNS(s.StoryID), sanitizeNS(cp.SaveID))
		if pgURL != "" {
			psqlBin, err := exec.LookPath("psql")
			if err != nil {
				return CheckpointRecord{}, fmt.Errorf("postgres engine requested with --pg-url, but psql not found on PATH")
			}
			sqlStmt := fmt.Sprintf("DROP DATABASE IF EXISTS %s; CREATE DATABASE %s;", dbName, dbName)
			cmd := exec.Command(psqlBin, pgURL, "-v", "ON_ERROR_STOP=1", "-c", sqlStmt)
			if out, err := cmd.CombinedOutput(); err != nil {
				return CheckpointRecord{}, fmt.Errorf("psql create checkpoint db %s: %v (%s)", dbName, err, strings.TrimSpace(string(out)))
			}
		}
		snapshotPath = "postgres://" + dbName
	}

	assertFlagsJSON, _ := json.Marshal(cp.AssertFlags)
	env := append(os.Environ(),
		"ARC_STORY_ID="+s.StoryID,
		"ARC_SAVE_ID="+cp.SaveID,
		"ARC_CHECKPOINT_DIR="+saveDir,
		"ARC_CHECKPOINT_PATH="+snapshotPath,
		"ARC_ASSERT_FLAGS_JSON="+string(assertFlagsJSON),
	)
	if strings.TrimSpace(cp.SeedCmd) != "" {
		if err := runShellInDir(repoRoot, cp.SeedCmd, env); err != nil {
			return CheckpointRecord{}, fmt.Errorf("seed_cmd failed: %w", err)
		}
	}
	for _, st := range cp.SetupSteps {
		if strings.TrimSpace(st.Cmd) != "" {
			stepInputJSON, _ := json.Marshal(st.With)
			stepEnv := append(append([]string{}, env...), "ARC_SETUP_WITH_JSON="+string(stepInputJSON))
			if err := runShellInDir(repoRoot, st.Cmd, stepEnv); err != nil {
				return CheckpointRecord{}, fmt.Errorf("setup step %s failed: %w", st.Call, err)
			}
		}
	}
	if strings.TrimSpace(cp.AssertCmd) != "" {
		if err := runShellInDir(repoRoot, cp.AssertCmd, env); err != nil {
			return CheckpointRecord{}, fmt.Errorf("checkpoint precondition assert_cmd failed: %w", err)
		}
	}

	meta := checkpointFileMetadata{
		storyRefBase: storyRefBase{StoryID: s.StoryID},
		SaveID:       cp.SaveID,
		Parent:       cp.Parent,
		Engine:       engine,
		AssertFlags:  cp.AssertFlags,
		AssertCmd:    cp.AssertCmd,
		SetupSteps:   cp.SetupSteps,
	}
	metaBytes, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(filepath.Join(saveDir, "checkpoint.json"), metaBytes, 0o644); err != nil {
		return CheckpointRecord{}, err
	}
	digest := hashPath(saveDir)
	return CheckpointRecord{
		StoryID:      s.StoryID,
		SaveID:       cp.SaveID,
		ParentID:     cp.Parent,
		Engine:       engine,
		SnapshotPath: snapshotPath,
		DigestSHA256: digest,
		CreatedAt:    nowUTC(),
	}, nil
}

// cloneCheckpoint creates an isolated millisecond clone for a route's runtime_ns,
// returning both the logical snapshot path and the local working directory for shard state.
func cloneCheckpointWithDir(repoRoot, storyID, saveID, runtimeNS, engine, pgURL string) (clonePath, workDir string, err error) {
	srcDir := filepath.Join(checkpointBaseDir(repoRoot, storyID), sanitizeNS(saveID))
	dstDir := filepath.Join(cloneBaseDir(repoRoot, storyID), sanitizeNS(saveID)+"_"+sanitizeNS(runtimeNS))
	_ = os.RemoveAll(dstDir)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return "", "", err
	}

	switch engine {
	case "postgres":
		tplDB := fmt.Sprintf("arc_save_%s_%s", sanitizeNS(storyID), sanitizeNS(saveID))
		cloneDB := fmt.Sprintf("arc_run_%s_%s", sanitizeNS(saveID), sanitizeNS(runtimeNS))
		if pgURL != "" {
			psqlBin, err := exec.LookPath("psql")
			if err != nil {
				return "", "", fmt.Errorf("psql not found on PATH")
			}
			sqlStmt := fmt.Sprintf("DROP DATABASE IF EXISTS %s; CREATE DATABASE %s TEMPLATE %s;", cloneDB, cloneDB, tplDB)
			cmd := exec.Command(psqlBin, pgURL, "-v", "ON_ERROR_STOP=1", "-c", sqlStmt)
			if out, err := cmd.CombinedOutput(); err != nil {
				return "", "", fmt.Errorf("psql clone %s from %s: %v (%s)", cloneDB, tplDB, err, strings.TrimSpace(string(out)))
			}
		}
		return "postgres://" + cloneDB, dstDir, nil
	case "sqlite":
		srcDB := filepath.Join(srcDir, "checkpoint.db")
		dstDB := filepath.Join(dstDir, "shard.db")
		if _, err := os.Stat(srcDB); err == nil {
			if err := copyFileBytes(srcDB, dstDB); err != nil {
				return "", "", err
			}
		}
		return dstDB, dstDir, nil
	default:
		if _, err := os.Stat(srcDir); err == nil {
			if err := copyTree(srcDir, dstDir); err != nil {
				return "", "", err
			}
		}
		return dstDir, dstDir, nil
	}
}

func cloneCheckpoint(repoRoot, storyID, saveID, runtimeNS, engine, pgURL string) (string, error) {
	clonePath, _, err := cloneCheckpointWithDir(repoRoot, storyID, saveID, runtimeNS, engine, pgURL)
	return clonePath, err
}

func runShellInDir(dir, command string, env []string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}


