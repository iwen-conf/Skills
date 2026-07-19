package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// cmdWatch runs the foreground incremental indexer required by arc:idx:
// fsnotify-driven rebuilds with debounce, an idle TTL so RAM is released
// when nobody is working, a hard memory limit, and orphan self-termination.
func cmdWatch(args []string) error {
	fl := flag.NewFlagSet("watch", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	ttl := fl.Duration("ttl", 30*time.Minute, "exit after this long without file events")
	memMB := fl.Int64("mem", 512, "soft memory limit in MiB (Go GC target)")
	debounce := fl.Duration("debounce", 800*time.Millisecond, "quiet period before reindexing")
	pidFile := fl.Bool("pidfile", false, "write .ai-code-index/daemon.pid (used by daemon start)")
	if err := fl.Parse(args); err != nil {
		return err
	}
	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}
	debug.SetMemoryLimit(*memMB << 20)

	if *pidFile {
		if pid, alive := daemonState(repo); alive {
			return fmt.Errorf("daemon already running with pid %d", pid)
		}
		if err := os.MkdirAll(repo.MetaDir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(repo.PidFile, []byte(fmt.Sprint(os.Getpid())), 0o644); err != nil {
			return err
		}
		defer os.Remove(repo.PidFile)
	}

	// Initial full build so the watcher always serves a fresh index.
	if err := reindexQuiet(repo); err != nil {
		return err
	}
	fmt.Printf("watching %s (ttl %s, mem limit %d MiB)\n", repo.Root, *ttl, *memMB)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	ignore := repo.ignoreSet()
	addDirs := func() {
		filepath.WalkDir(repo.Root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return nil
			}
			name := d.Name()
			if path != repo.Root && (ignore[name] || strings.HasPrefix(name, ".")) {
				return fs.SkipDir
			}
			watcher.Add(path)
			return nil
		})
	}
	addDirs()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ppid := os.Getppid()
	orphanTick := time.NewTicker(30 * time.Second)
	defer orphanTick.Stop()
	idle := time.NewTimer(*ttl)
	defer idle.Stop()
	var pending *time.Timer
	pendingC := make(<-chan time.Time) // nil-like channel until a change arrives

	for {
		select {
		case ev := <-watcher.Events:
			base := filepath.Base(ev.Name)
			if ignore[base] || strings.HasPrefix(base, ".") {
				continue
			}
			if ev.Op&fsnotify.Create != 0 {
				if st, err := os.Stat(ev.Name); err == nil && st.IsDir() {
					addDirs()
				}
			}
			if pending == nil {
				pending = time.NewTimer(*debounce)
			} else {
				pending.Reset(*debounce)
			}
			pendingC = pending.C
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(*ttl)
		case <-pendingC:
			pendingC = make(<-chan time.Time)
			start := time.Now()
			if err := reindexQuiet(repo); err != nil {
				fmt.Fprintf(os.Stderr, "reindex failed: %v\n", err)
			} else {
				fmt.Printf("reindexed in %s\n", time.Since(start).Round(time.Millisecond))
			}
		case <-orphanTick.C:
			// If the parent terminal/IDE died we get re-parented; exit to
			// avoid zombie daemons.
			if os.Getppid() != ppid && os.Getppid() == 1 {
				fmt.Println("parent process gone; exiting")
				return nil
			}
		case <-idle.C:
			fmt.Printf("idle for %s; exiting to release memory\n", *ttl)
			return nil
		case sig := <-sigCh:
			fmt.Printf("received %s; exiting\n", sig)
			return nil
		case err := <-watcher.Errors:
			fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
		}
	}
}

func reindexQuiet(repo *Repo) error {
	files, err := walkRepo(repo)
	if err != nil {
		return err
	}
	if err := buildZoekt(repo, files); err != nil {
		return err
	}
	if err := buildTags(repo, files); err != nil {
		fmt.Fprintf(os.Stderr, "warning: ctags rebuild failed: %v\n", err)
	}
	return nil
}

// cmdDaemon manages a detached background watcher with a pid file.
func cmdDaemon(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: arc-idx daemon start|stop|status|restart")
	}
	sub, rest := args[0], args[1:]
	fl := flag.NewFlagSet("daemon", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	if err := fl.Parse(rest); err != nil {
		return err
	}
	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}

	switch sub {
	case "start":
		return daemonStart(repo)
	case "stop":
		return daemonStop(repo)
	case "status":
		pid, alive := daemonState(repo)
		return emitJSON(map[string]any{
			"pid":         pid,
			"alive":       alive,
			"index_dir":   repo.IndexDir,
			"index_age":   ageOrNone(indexMTime(repo)),
			"log_file":    filepath.Join(repo.MetaDir, "daemon.log"),
			"stale_index": !indexMTime(repo).IsZero() && newestSourceMTime(repo).After(indexMTime(repo)),
		})
	case "restart":
		if err := daemonStop(repo); err != nil {
			fmt.Fprintf(os.Stderr, "stop: %v\n", err)
		}
		return daemonStart(repo)
	default:
		return fmt.Errorf("unknown daemon subcommand %q", sub)
	}
}

func daemonStart(repo *Repo) error {
	if pid, alive := daemonState(repo); alive {
		return fmt.Errorf("already running with pid %d", pid)
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(repo.MetaDir, 0o755); err != nil {
		return err
	}
	logPath := filepath.Join(repo.MetaDir, "daemon.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	cmd := exec.Command(self, "watch", "--repo", repo.Root, "--pidfile")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Printf("daemon started with pid %d (log: %s)\n", cmd.Process.Pid, logPath)
	return cmd.Process.Release()
}

func daemonStop(repo *Repo) error {
	pid, alive := daemonState(repo)
	if pid == 0 {
		return fmt.Errorf("no pid file at %s", repo.PidFile)
	}
	if !alive {
		os.Remove(repo.PidFile)
		return fmt.Errorf("pid %d not running; removed stale pid file", pid)
	}
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return err
	}
	for i := 0; i < 50; i++ {
		if _, alive := daemonState(repo); !alive {
			os.Remove(repo.PidFile)
			fmt.Printf("daemon %d stopped\n", pid)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("daemon %d did not exit after SIGTERM", pid)
}

func ageOrNone(t time.Time) string {
	if t.IsZero() {
		return "none"
	}
	return time.Since(t).Round(time.Second).String()
}
