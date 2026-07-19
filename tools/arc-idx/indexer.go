package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sourcegraph/zoekt"
	zindex "github.com/sourcegraph/zoekt/index"
)

// cmdIndex rebuilds the zoekt shards and the ctags symbol table for the repo.
// Both builds run concurrently and saturate the available cores.
func cmdIndex(args []string) error {
	fl := flag.NewFlagSet("index", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	quiet := fl.Bool("quiet", false, "suppress progress output")
	if err := fl.Parse(args); err != nil {
		return err
	}
	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}
	start := time.Now()

	files, err := walkRepo(repo)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	var zoektErr, tagsErr error
	wg.Add(2)
	go func() {
		defer wg.Done()
		zoektErr = buildZoekt(repo, files)
	}()
	go func() {
		defer wg.Done()
		tagsErr = buildTags(repo, files)
	}()
	wg.Wait()

	if zoektErr != nil {
		return fmt.Errorf("zoekt build: %w", zoektErr)
	}
	if tagsErr != nil && !*quiet {
		fmt.Fprintf(os.Stderr, "warning: ctags build failed: %v\n", tagsErr)
	}
	if !*quiet {
		fmt.Printf("indexed %d files from %s into %s in %s\n",
			len(files), repo.Root, repo.IndexDir, time.Since(start).Round(time.Millisecond))
	}
	return nil
}

// walkRepo returns repo-relative paths of every indexable file.
func walkRepo(repo *Repo) ([]string, error) {
	ignore := repo.ignoreSet()
	var files []string
	err := filepath.WalkDir(repo.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are skipped, not fatal
		}
		name := d.Name()
		if d.IsDir() {
			if path != repo.Root && (ignore[name] || strings.HasPrefix(name, ".") && name != ".") {
				if name == ".github" { // keep CI configs searchable
					return nil
				}
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > int64(repo.Cfg.SizeMax) || info.Size() == 0 {
			return nil
		}
		rel, _ := filepath.Rel(repo.Root, path)
		rel = filepath.ToSlash(rel)
		if repo.ignoredFile(rel) || !repo.indexableExt(rel) {
			return nil
		}
		files = append(files, rel)
		if len(files) >= repo.Cfg.MaxFiles {
			fmt.Fprintf(os.Stderr, "warning: hit max_files=%d; narrow ignore_dirs/profiles or raise max_files in config.json\n", repo.Cfg.MaxFiles)
			return fs.SkipAll
		}
		return nil
	})
	return files, err
}

// buildZoekt writes fresh shards for the given file list.
func buildZoekt(repo *Repo, files []string) error {
	if err := os.MkdirAll(repo.IndexDir, 0o755); err != nil {
		return err
	}
	opts := zindex.Options{
		IndexDir:    repo.IndexDir,
		Parallelism: runtime.NumCPU(),
		SizeMax:     repo.Cfg.SizeMax,
		RepositoryDescription: zoekt.Repository{
			Name: repo.Name,
		},
	}
	if ctags, err := exec.LookPath("ctags"); err == nil {
		opts.CTagsPath = ctags
	}
	opts.SetDefaults()

	builder, err := zindex.NewBuilder(opts)
	if err != nil {
		return err
	}
	for _, rel := range files {
		content, err := os.ReadFile(filepath.Join(repo.Root, rel))
		if err != nil {
			continue
		}
		if bytes.IndexByte(content, 0) >= 0 {
			continue // binary
		}
		if err := builder.AddFile(rel, content); err != nil {
			return err
		}
	}
	return builder.Finish()
}

// buildTags regenerates .ai-code-index/tags.json (one ctags JSON object per
// line). The walked file list is piped via -L so ctags never re-crawls the
// tree itself — scope stays identical to the zoekt shards.
func buildTags(repo *Repo, files []string) error {
	ctagsBin, err := exec.LookPath("ctags")
	if err != nil {
		return fmt.Errorf("ctags not installed")
	}
	if err := os.MkdirAll(repo.MetaDir, 0o755); err != nil {
		return err
	}
	// ctags refuses to overwrite a file it does not recognize as a tag file
	// (the JSON format has no pseudo-tag header), so always start fresh.
	os.Remove(repo.TagsFile)
	cmd := exec.Command(ctagsBin,
		"--output-format=json",
		"--fields=+nKlSz",
		"--tag-relative=never",
		"-L", "-",
		"-f", repo.TagsFile,
	)
	cmd.Dir = repo.Root
	cmd.Stdin = strings.NewReader(strings.Join(files, "\n"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// indexMTime returns the newest shard modification time, or zero if absent.
func indexMTime(repo *Repo) time.Time {
	var newest time.Time
	entries, err := os.ReadDir(repo.IndexDir)
	if err != nil {
		return newest
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".zoekt") {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

// hasShards reports whether any zoekt shard exists for the repo.
func hasShards(repo *Repo) bool {
	return !indexMTime(repo).IsZero()
}
