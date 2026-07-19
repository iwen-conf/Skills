package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// doctorReport is the JSON shape of `arc-idx doctor`.
type doctorReport struct {
	RepoRoot     string            `json:"repo_root"`
	IndexDir     string            `json:"index_dir"`
	Healthy      bool              `json:"healthy"`
	Tools        map[string]string `json:"tools"`
	IndexAge     string            `json:"index_age,omitempty"`
	IndexStale   bool              `json:"index_stale"`
	TagsPresent  bool              `json:"tags_present"`
	DaemonPid    int               `json:"daemon_pid,omitempty"`
	DaemonAlive  bool              `json:"daemon_alive"`
	Profiles     []string          `json:"profiles"`
	Advice       []string          `json:"advice,omitempty"`
	ConfigSource string            `json:"config_source"`
}

func cmdDoctor(args []string) error {
	fl := flag.NewFlagSet("doctor", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	format := fl.String("format", "ai-md", "output format: json | ai-md")
	if err := fl.Parse(args); err != nil {
		return err
	}
	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}

	rep := doctorReport{
		RepoRoot: repo.Root,
		IndexDir: repo.IndexDir,
		Tools:    map[string]string{},
		Healthy:  true,
	}
	for _, tool := range []string{"ctags", "ast-grep", "rg", "fd"} {
		if p, err := exec.LookPath(tool); err == nil {
			rep.Tools[tool] = p
		} else {
			rep.Tools[tool] = "MISSING"
			if tool == "ctags" || tool == "ast-grep" {
				rep.Healthy = false
				rep.Advice = append(rep.Advice, "brew install "+map[string]string{"ctags": "universal-ctags", "ast-grep": "ast-grep"}[tool])
			}
		}
	}

	cfgPath := filepath.Join(repo.MetaDir, "config.json")
	if _, err := os.Stat(cfgPath); err == nil {
		rep.ConfigSource = cfgPath
	} else {
		rep.ConfigSource = "built-in defaults"
	}
	for name := range repo.Cfg.Profiles {
		rep.Profiles = append(rep.Profiles, name)
	}

	idxTime := indexMTime(repo)
	if idxTime.IsZero() {
		rep.Healthy = false
		rep.IndexStale = true
		rep.Advice = append(rep.Advice, "no zoekt shards: run `arc-idx index`")
	} else {
		rep.IndexAge = time.Since(idxTime).Round(time.Second).String()
		if newest := newestSourceMTime(repo); newest.After(idxTime) {
			rep.IndexStale = true
			rep.Advice = append(rep.Advice, "index older than sources: run `arc-idx index` or `arc-idx daemon start`")
		}
	}
	if _, err := os.Stat(repo.TagsFile); err == nil {
		rep.TagsPresent = true
	} else {
		rep.Advice = append(rep.Advice, "no symbol table: run `arc-idx index`")
	}
	if pid, alive := daemonState(repo); pid > 0 {
		rep.DaemonPid = pid
		rep.DaemonAlive = alive
		if !alive {
			rep.Advice = append(rep.Advice, "stale daemon.pid: run `arc-idx daemon restart` or delete the pid file")
		}
	}

	if *format == "json" {
		return emitJSON(rep)
	}
	var b strings.Builder
	status := "healthy"
	if !rep.Healthy || rep.IndexStale {
		status = "needs attention"
	}
	fmt.Fprintf(&b, "## arc-idx doctor — %s\n\n", status)
	fmt.Fprintf(&b, "- repo: %s\n- index: %s (age: %s, stale: %v)\n- symbols: %v\n- daemon: pid=%d alive=%v\n- config: %s\n- profiles: %s\n",
		rep.RepoRoot, rep.IndexDir, orDefault(rep.IndexAge, "none"), rep.IndexStale,
		rep.TagsPresent, rep.DaemonPid, rep.DaemonAlive, rep.ConfigSource,
		orDefault(strings.Join(rep.Profiles, ", "), "(none defined)"))
	for tool, p := range rep.Tools {
		fmt.Fprintf(&b, "- tool %s: %s\n", tool, p)
	}
	if len(rep.Advice) > 0 {
		b.WriteString("\n### advice\n")
		for _, a := range rep.Advice {
			fmt.Fprintf(&b, "- %s\n", a)
		}
	}
	fmt.Print(b.String())
	return nil
}

// newestSourceMTime scans for the most recently modified indexable file.
func newestSourceMTime(repo *Repo) time.Time {
	var newest time.Time
	files, err := walkRepo(repo)
	if err != nil {
		return newest
	}
	for _, rel := range files {
		if info, err := os.Stat(filepath.Join(repo.Root, rel)); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest
}

// daemonState reads the pid file and probes the process.
func daemonState(repo *Repo) (pid int, alive bool) {
	data, err := os.ReadFile(repo.PidFile)
	if err != nil {
		return 0, false
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	// Signal 0 checks existence without touching the process.
	if err := syscall.Kill(pid, 0); err != nil {
		return pid, false
	}
	return pid, true
}
