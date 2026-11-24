package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"hash"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type ledgerColumnMigration struct {
	table      string
	column     string
	definition string
}

var ledgerColumnMigrations = []ledgerColumnMigration{
	{table: "story_runs", column: "repo_root", definition: "TEXT NOT NULL DEFAULT ''"},
	{table: "story_runs", column: "source_tree_sha256", definition: "TEXT NOT NULL DEFAULT ''"},
	{table: "story_route_results", column: "repo_root", definition: "TEXT NOT NULL DEFAULT ''"},
	{table: "story_route_results", column: "source_commit", definition: "TEXT NOT NULL DEFAULT ''"},
	{table: "story_route_results", column: "source_tree_sha256", definition: "TEXT NOT NULL DEFAULT ''"},
	{table: "code_verify_runs", column: "source_tree_sha256", definition: "TEXT NOT NULL DEFAULT ''"},
	{table: "code_verify_results", column: "repo_root", definition: "TEXT NOT NULL DEFAULT ''"},
	{table: "code_verify_results", column: "source_commit", definition: "TEXT NOT NULL DEFAULT ''"},
	{table: "code_verify_results", column: "source_tree_sha256", definition: "TEXT NOT NULL DEFAULT ''"},
}

// ensureLedgerColumns 为既有历史账本表幂等补齐版本追踪所需缺失列
// 请求体
// ctx 超时控制上下文
// db 已打开的SQLite账本句柄
// 响应体
// err 查询或ALTER迁移失败错误
func ensureLedgerColumns(ctx context.Context, db *sql.DB) error {
	// 1. 缓存各表已存在列名避免重复扫描PRAGMA
	existing := map[string]map[string]bool{}
	for _, mig := range ledgerColumnMigrations {
		cols, ok := existing[mig.table]
		if !ok {
			loaded, err := readTableColumns(ctx, db, mig.table)
			if err != nil {
				return err
			}
			cols = loaded
			existing[mig.table] = cols
		}
		// 2. 缺失目标列时执行向后兼容的ALTER ADD COLUMN迁移
		if cols[mig.column] {
			continue
		}
		stmt := "ALTER TABLE " + mig.table + " ADD COLUMN " + mig.column + " " + mig.definition
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
		cols[mig.column] = true
	}
	return nil
}

// readTableColumns 读取指定SQLite表的现有列名集合
// 请求体
// ctx 超时控制上下文
// db 数据库句柄
// table 静态表名常量
// 响应体
// cols 列名存在性映射表
// err SQL执行或行扫描错误
func readTableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	// 1. 查询表结构信息并提取第2列字段名
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var (
			cid        int
			name       string
			colType    string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if scanErr := rows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &pk); scanErr != nil {
			return nil, scanErr
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// canonicalRepoRoot 解析并返回经符号链接归一化的仓库绝对路径
// 请求体
// repoRoot 输入仓库目录路径（空字符串默认当前目录）
// 响应体
// canonical 规范化后的绝对路径
func canonicalRepoRoot(repoRoot string) string {
	// 1. 将空路径回退当前目录并计算绝对路径
	trimmed := strings.TrimSpace(repoRoot)
	if trimmed == "" {
		trimmed = "."
	}
	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return filepath.Clean(trimmed)
	}
	// 2. 解析系统符号链接以对齐macOS /var 与 /private/var 等别名路径
	if resolved, rErr := filepath.EvalSymlinks(abs); rErr == nil {
		return resolved
	}
	return abs
}

// sourceWorktreeDigest 计算仓库当前提交与未提交脏工作树的确定性内容摘要
// 请求体
// repoRoot 目标仓库根目录
// 响应体
// digest 64位十六进制工作树版本指纹
func sourceWorktreeDigest(repoRoot string) string {
	// 1. 无额外排除账本路径时委托通用工作树摘要函数
	return sourceWorktreeDigestExcluding(repoRoot, "")
}

// sourceWorktreeDigestExcluding 计算排除指定验证账本及其SQLite伴随文件后的工作树内容摘要
// 请求体
// repoRoot 目标仓库根目录
// ledgerPath 当前验证账本文件路径（可为空）
// 响应体
// digest 64位十六进制工作树版本指纹
func sourceWorktreeDigestExcluding(repoRoot, ledgerPath string) string {
	// 1. 归一化根目录并解析账本文件相对路径，避免写入自身收据导致工作树指纹自漂移
	root := canonicalRepoRoot(repoRoot)
	ledgerRel := resolveLedgerRelPath(root, ledgerPath)
	cmd := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree")
	if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) == "true" {
		if digest, ok := hashGitWorktreeState(root, ledgerRel); ok {
			return digest
		}
	}
	// 2. 非Git目录回退为全量有序目录遍历摘要（排除.git、.arc与当前账本文件）
	return hashWalkWorktreeState(root, ledgerRel)
}

// resolveLedgerRelPath 计算验证账本相对于规范化仓库根目录的正斜杠路径
// 请求体
// canonicalRoot 已归一化的仓库根目录
// ledgerPath 账本数据库路径
// 响应体
// rel 相对路径（不在仓库内时返回空串）
func resolveLedgerRelPath(canonicalRoot, ledgerPath string) string {
	// 1. 对账本父目录执行符号链接归一化，确保 -wal/-shm 等瞬态文件在同根下可匹配
	trimmed := strings.TrimSpace(ledgerPath)
	if trimmed == "" {
		return ""
	}
	canonicalLedger := filepath.Join(canonicalRepoRoot(filepath.Dir(trimmed)), filepath.Base(trimmed))
	rel, err := filepath.Rel(canonicalRoot, canonicalLedger)
	if err != nil {
		return ""
	}
	slashRel := filepath.ToSlash(rel)
	if slashRel == "." || strings.HasPrefix(slashRel, "../") {
		return ""
	}
	return slashRel
}

type gitDirtyEntry struct {
	status string
	path   string
	origin string
}

// hashGitWorktreeState 基于Git HEAD与porcelain脏文件列表生成精确工作树摘要
// 请求体
// root 已归一化的Git仓库根目录
// ledgerRel 验证账本相对路径
// 响应体
// digest 64位十六进制工作树摘要
// ok Git状态命令执行是否成功
func hashGitWorktreeState(root, ledgerRel string) (string, bool) {
	// 1. 以NUL分隔采集全部未提交修改与未跟踪文件并过滤账本运行产物目录
	cmd := exec.Command("git", "-C", root, "status", "--porcelain=v1", "-z", "-uall")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	tokens := strings.Split(string(out), "\x00")
	var entries []gitDirtyEntry
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if len(tok) < 4 {
			continue
		}
		status := tok[:2]
		rel := filepath.ToSlash(tok[3:])
		origin := ""
		if (status[0] == 'R' || status[0] == 'C') && i+1 < len(tokens) {
			origin = filepath.ToSlash(tokens[i+1])
			i++
		}
		if shouldIgnoreWorktreePath(rel, ledgerRel) {
			continue
		}
		entries = append(entries, gitDirtyEntry{status: status, path: rel, origin: origin})
	}
	// 2. 排序脏文件路径并连同HEAD与脏文件内容字节流一并哈希
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].path != entries[j].path {
			return entries[i].path < entries[j].path
		}
		return entries[i].status < entries[j].status
	})
	h := sha256.New()
	_, _ = h.Write([]byte("GIT\x00HEAD:" + currentGitHead(root) + "\x00"))
	for _, entry := range entries {
		_, _ = h.Write([]byte(entry.status + ":" + entry.origin + "->" + entry.path + "\x00"))
		appendPathDigest(h, filepath.Join(root, filepath.FromSlash(entry.path)))
	}
	return hex.EncodeToString(h.Sum(nil)), true
}

// hashWalkWorktreeState 对非Git工作目录按确定顺序哈希全部源码与配置条目
// 请求体
// root 已归一化的目标目录根路径
// ledgerRel 验证账本相对路径
// 响应体
// digest 64位十六进制目录内容摘要
func hashWalkWorktreeState(root, ledgerRel string) string {
	// 1. 递归收集目录中除.git/.arc/账本库/索引运行时文件外的全部相对路径
	var rels []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, rErr := filepath.Rel(root, path)
		if rErr != nil || rel == "." {
			return nil
		}
		slashRel := filepath.ToSlash(rel)
		if shouldIgnoreWorktreePath(slashRel, ledgerRel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			rels = append(rels, slashRel)
		}
		return nil
	})
	sort.Strings(rels)
	// 2. 顺序写入相对路径与实时文件内容或符号链接目标
	h := sha256.New()
	_, _ = h.Write([]byte("WALK\x00"))
	for _, rel := range rels {
		_, _ = h.Write([]byte(rel + "\x00"))
		appendPathDigest(h, filepath.Join(root, filepath.FromSlash(rel)))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// shouldIgnoreWorktreePath 判断路径是否属于验证账本与代码索引瞬态文件
// 请求体
// slashRel 正斜杠分隔的相对路径
// ledgerRel 当前账本相对路径
// 响应体
// ignore 应忽略返回true
func shouldIgnoreWorktreePath(slashRel, ledgerRel string) bool {
	// 1. 排除.git与.arc验证账本目录避免验证写入污染源码指纹
	if slashRel == ".git" || strings.HasPrefix(slashRel, ".git/") {
		return true
	}
	if slashRel == ".arc" || strings.HasPrefix(slashRel, ".arc/") {
		return true
	}
	// 2. 排除当前使用的外部账本文件及SQLite WAL/SHM/Journal伴随文件
	if ledgerRel != "" && (slashRel == ledgerRel ||
		slashRel == ledgerRel+"-wal" ||
		slashRel == ledgerRel+"-shm" ||
		slashRel == ledgerRel+"-journal") {
		return true
	}
	// 3. 排除本地代码索引生成的tags与daemon锁文件
	if strings.HasPrefix(slashRel, ".ai-code-index/tags") || strings.HasPrefix(slashRel, ".ai-code-index/daemon.") {
		return true
	}
	return false
}

// appendPathDigest 将单个路径的文件字节或符号链接目标写入哈希摘要器
// 请求体
// h 正在计算的SHA256摘要器
// absPath 目标文件绝对路径
// 响应体
// 无
func appendPathDigest(h hash.Hash, absPath string) {
	// 1. 通过Lstat识别缺失、符号链接或目录条目避免误读
	info, err := os.Lstat(absPath)
	if err != nil {
		_, _ = h.Write([]byte("MISSING\x00"))
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, _ := os.Readlink(absPath)
		_, _ = h.Write([]byte("LINK:" + target + "\x00"))
		return
	}
	if info.IsDir() {
		_, _ = h.Write([]byte("DIR\x00"))
		return
	}
	// 2. 读取常规文件内容并追加至哈希流
	data, err := os.ReadFile(absPath)
	if err != nil {
		_, _ = h.Write([]byte("UNREADABLE\x00"))
		return
	}
	_, _ = h.Write([]byte("FILE:"))
	_, _ = h.Write(data)
	_, _ = h.Write([]byte("\x00"))
}
