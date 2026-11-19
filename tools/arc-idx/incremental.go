package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// pathLayer classifies a repo-relative path for authority-weighted ranking.
// Ordering: prod > contract > test > doc.
func pathLayer(rel string) string {
	lower := strings.ToLower(filepath.ToSlash(rel))
	base := filepath.Base(lower)
	ext := strings.ToLower(filepath.Ext(base))

	switch ext {
	case ".md", ".mdx", ".rst", ".adoc", ".txt":
		return "doc"
	case ".proto", ".sql", ".graphql", ".gql", ".thrift", ".avdl":
		return "contract"
	}
	if strings.Contains(lower, "openapi") || strings.Contains(lower, "swagger") ||
		strings.Contains(lower, "/migrations/") || strings.HasPrefix(lower, "migrations/") ||
		strings.Contains(lower, "/schema") || strings.HasPrefix(lower, "schemas/") {
		return "contract"
	}
	if strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".test.ts") || strings.HasSuffix(base, ".spec.ts") ||
		strings.HasSuffix(base, ".test.js") || strings.HasSuffix(base, ".spec.js") ||
		strings.HasSuffix(base, "_test.py") || strings.HasPrefix(base, "test_") ||
		strings.Contains(lower, "/tests/") || strings.HasPrefix(lower, "tests/") ||
		strings.Contains(lower, "/__tests__/") || strings.Contains(lower, "/fixtures/") {
		return "test"
	}
	return "prod"
}

func layerWeight(layer string) float64 {
	switch layer {
	case "prod":
		return 4.0
	case "contract":
		return 3.0
	case "test":
		return 1.2
	case "doc":
		return 0.4
	default:
		return 1.0
	}
}

type indexManifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

func manifestFilePath(repo *Repo) string {
	return filepath.Join(repo.IndexDir, "manifest.json")
}

func loadManifest(repo *Repo) *indexManifest {
	data, err := os.ReadFile(manifestFilePath(repo))
	if err != nil {
		return nil
	}
	var m indexManifest
	if json.Unmarshal(data, &m) != nil || m.Version != 1 || m.Files == nil {
		return nil
	}
	return &m
}

func saveManifest(repo *Repo, files map[string]string) error {
	if err := os.MkdirAll(repo.IndexDir, 0o755); err != nil {
		return err
	}
	payload, err := json.Marshal(indexManifest{Version: 1, Files: files})
	if err != nil {
		return err
	}
	tmp := manifestFilePath(repo) + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, manifestFilePath(repo))
}

// collectRepoHashes uses `git ls-files -s` + `git status --porcelain` for
// content-addressed incremental indexing when inside a Git repository, falling
// back to WalkDir + sha256 otherwise.
func collectRepoHashes(repo *Repo, files []string) map[string]string {
	hashes := make(map[string]string, len(files))
	gitBlobs, dirty, ok := gitBlobHashes(repo.Root)
	if ok {
		for _, rel := range files {
			if !dirty[rel] {
				if sha, exists := gitBlobs[rel]; exists {
					hashes[rel] = "git:" + sha
					continue
				}
			}
			hashes[rel] = fileContentHash(filepath.Join(repo.Root, rel))
		}
		return hashes
	}
	for _, rel := range files {
		hashes[rel] = fileContentHash(filepath.Join(repo.Root, rel))
	}
	return hashes
}

func gitBlobHashes(root string) (map[string]string, map[string]bool, bool) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil, nil, false
	}
	lsCmd := exec.Command("git", "ls-files", "-s")
	lsCmd.Dir = root
	lsOut, err := lsCmd.Output()
	if err != nil {
		return nil, nil, false
	}
	blobs := make(map[string]string)
	for _, line := range strings.Split(string(lsOut), "\n") {
		if line == "" {
			continue
		}
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}
		meta := strings.Fields(line[:tab])
		if len(meta) < 2 {
			continue
		}
		rel := filepath.ToSlash(strings.Trim(line[tab+1:], `"`))
		blobs[rel] = meta[1]
	}

	dirty := make(map[string]bool)
	stCmd := exec.Command("git", "status", "--porcelain", "-uall")
	stCmd.Dir = root
	if stOut, err := stCmd.Output(); err == nil {
		for _, line := range strings.Split(string(stOut), "\n") {
			if len(line) < 4 {
				continue
			}
			p := strings.TrimSpace(line[3:])
			if idx := strings.Index(p, " -> "); idx >= 0 {
				dirty[filepath.ToSlash(p[:idx])] = true
				p = p[idx+4:]
			}
			dirty[filepath.ToSlash(strings.Trim(p, `"`))] = true
		}
	}
	return blobs, dirty, true
}

func fileContentHash(absPath string) string {
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:16])
}

// diffManifest returns sorted paths added, modified, or removed since prev.
func diffManifest(prev, curr map[string]string) (changed []string, removed []string) {
	for path, hash := range curr {
		if prevHash, ok := prev[path]; !ok || prevHash != hash {
			changed = append(changed, path)
		}
	}
	for path := range prev {
		if _, ok := curr[path]; !ok {
			removed = append(removed, path)
		}
	}
	sort.Strings(changed)
	sort.Strings(removed)
	return changed, removed
}

// walkRepoGitFast lists indexable files using `git ls-files` (tracked + untracked
// non-ignored) when possible, avoiding full directory traversal over ignored trees.
func walkRepoGitFast(repo *Repo, strict bool) ([]string, bool, error) {
	if _, err := os.Stat(filepath.Join(repo.Root, ".git")); err != nil {
		return nil, false, nil
	}
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = repo.Root
	out, err := cmd.Output()
	if err != nil {
		return nil, false, nil
	}
	ignore := repo.ignoreSet()
	var files []string
	seen := make(map[string]bool)
	for _, raw := range bytes.Split(out, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		rel := filepath.ToSlash(string(raw))
		if seen[rel] {
			continue
		}
		seen[rel] = true
		if hasIgnoredSegment(rel, ignore) {
			continue
		}
		if repo.ignoredFile(rel) || !repo.indexableExt(rel) {
			continue
		}
		info, statErr := os.Stat(filepath.Join(repo.Root, rel))
		if statErr != nil || !info.Mode().IsRegular() {
			if strict && statErr != nil && !os.IsNotExist(statErr) {
				return nil, true, statErr
			}
			continue
		}
		if info.Size() == 0 || info.Size() > int64(repo.Cfg.SizeMax) {
			continue
		}
		files = append(files, rel)
		if len(files) >= repo.Cfg.MaxFiles {
			break
		}
	}
	sort.Strings(files)
	return files, true, nil
}

func hasIgnoredSegment(rel string, ignore map[string]bool) bool {
	parts := strings.Split(rel, "/")
	for i := 0; i < len(parts)-1; i++ {
		seg := parts[i]
		if seg == ".github" {
			continue
		}
		if ignore[seg] || strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}
