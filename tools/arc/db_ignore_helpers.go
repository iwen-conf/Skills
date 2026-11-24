package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var arcSchema = []string{
	`CREATE TABLE IF NOT EXISTS arc_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_tasks (id TEXT PRIMARY KEY, title TEXT NOT NULL, status TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1, created TEXT NOT NULL, updated TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_files (path TEXT PRIMARY KEY, sha256 TEXT NOT NULL, bytes INTEGER NOT NULL, version INTEGER NOT NULL, changed TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_task_files (task_id TEXT NOT NULL, path TEXT NOT NULL, updated TEXT NOT NULL, PRIMARY KEY(task_id,path))`,
	`CREATE TABLE IF NOT EXISTS arc_changes (id TEXT PRIMARY KEY, task TEXT NOT NULL, fingerprint TEXT NOT NULL UNIQUE, status TEXT NOT NULL, worktree TEXT NOT NULL, revision TEXT NOT NULL, created TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_change_files (change_id TEXT NOT NULL, path TEXT NOT NULL, sha256 TEXT NOT NULL, operation TEXT NOT NULL, PRIMARY KEY (change_id,path))`,
	`CREATE TABLE IF NOT EXISTS arc_events (id TEXT PRIMARY KEY, aggregate TEXT NOT NULL, aggregate_id TEXT NOT NULL, kind TEXT NOT NULL, payload TEXT NOT NULL, created TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS arc_outbox (event_id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0)`,
	`CREATE TABLE IF NOT EXISTS arc_task_obligations (arc_task_id TEXT NOT NULL, graph TEXT NOT NULL, obligation_id TEXT NOT NULL, verify_task_id TEXT NOT NULL DEFAULT '', updated TEXT NOT NULL, PRIMARY KEY(arc_task_id,graph,obligation_id))`,
	`CREATE INDEX IF NOT EXISTS arc_events_aggregate ON arc_events(aggregate,aggregate_id,created)`,
	`CREATE INDEX IF NOT EXISTS arc_changes_task ON arc_changes(task,created)`,
	`CREATE INDEX IF NOT EXISTS arc_task_obligations_verify ON arc_task_obligations(graph,obligation_id,verify_task_id)`,
}

// ensureLocalArcExclude 将生成的本地状态文件写入Git仓库私有info/exclude忽略列表
// 请求体
// root Git仓库根目录路径
// 响应体
// err 解析Git目录或写入忽略规则错误
func ensureLocalArcExclude(root string) error {
	// 1. 非Git目录无需写入忽略配置
	if _, err := os.Lstat(filepath.Join(root, ".git")); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	// 2. 解析 info/exclude 绝对路径并检查幂等标记
	cmd := exec.Command("git", "-C", root, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude")
	raw, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("resolve Git info/exclude: %w", err)
	}
	target := strings.TrimSpace(string(raw))
	if target == "" || !filepath.IsAbs(target) {
		return errors.New("Git exclude path is not absolute")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	old, err := os.ReadFile(target)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	marker := "# arc.db generated local state (managed by arc)\n"
	wanted := []string{
		"/.arc/arc.db",
		"/.arc/arc.db-*",
		"/.arc/arc.db.*",
		"/.arc/arc.json",
		"/.arc/arc.json.tmp",
		"/.arc/arc.lock",
		"/.arc/arc.lock.exclusive",
		"/.arc/backups/",
		"/.arc/artifacts/",
		"/.arc/handoff/",
		"/.arc/verify/code/",
	}
	var missing []string
	existingText := string(old)
	hasMarker := strings.Contains(existingText, marker)
	for _, entry := range wanted {
		if !strings.Contains(existingText, entry+"\n") && !strings.HasSuffix(existingText, entry) {
			missing = append(missing, entry)
		}
	}
	if hasMarker && len(missing) == 0 {
		return nil
	}
	// 3. 追加受管忽略规则块或缺失条目并刷盘保存
	var b strings.Builder
	if !hasMarker {
		b.WriteString(marker)
	}
	for _, entry := range missing {
		b.WriteString(entry + "\n")
	}
	if !hasMarker {
		b.WriteString("# end arc.db generated local state\n")
	}
	ignore := b.String()
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(old) > 0 && old[len(old)-1] != '\n' {
		if _, err := f.Write([]byte("\n")); err != nil {
			return err
		}
	}
	if _, err := f.Write([]byte(ignore)); err != nil {
		return err
	}
	return f.Sync()
}

// ensureArcPrivateFiles 初始化安全的 .arc 目录及内部 .gitignore 保护规则
// 请求体
// root Git仓库根目录路径
// 响应体
// err 目录越界或写入忽略规则失败错误
func ensureArcPrivateFiles(root string) error {
	// 1. 校验或创建非符号链接的 .arc 私有目录
	dir := filepath.Join(root, ".arc")
	if st, err := os.Lstat(dir); err == nil {
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe .arc directory: %s", dir)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// 2. 读取已有 .arc/.gitignore 并判断是否已包含完整受管规则
	ignore := filepath.Join(dir, ".gitignore")
	raw, err := os.ReadFile(ignore)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if st, e := os.Lstat(ignore); e == nil && !st.Mode().IsRegular() {
		return errors.New("unsafe .arc/.gitignore")
	}
	text := string(raw)
	hasHeader := strings.Contains(text, "# arc local state (generated)")
	wanted := []string{
		"arc.db",
		"arc.db-*",
		"arc.json",
		"arc.lock",
		"backups/",
		"policy/",
		"artifacts/",
		"handoff/",
		"verify/*.db",
		"verify/*.db-*",
		"verify/code/",
	}
	var missing []string
	for _, entry := range wanted {
		if !strings.Contains(text, entry+"\n") && !strings.HasSuffix(text, entry) {
			missing = append(missing, entry)
		}
	}
	if hasHeader && len(missing) == 0 {
		return nil
	}
	// 3. 通过临时文件原子更新 .arc/.gitignore
	var b strings.Builder
	if !hasHeader {
		b.WriteString("# arc local state (generated)\n")
	}
	for _, entry := range missing {
		b.WriteString(entry + "\n")
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += b.String()
	tmp := ignore + ".tmp"
	file, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.WriteString(text)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	if closeErr := file.Close(); writeErr == nil {
		writeErr = closeErr
	}
	if writeErr != nil {
		_ = os.Remove(tmp)
		return writeErr
	}
	if err := os.Rename(tmp, ignore); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
