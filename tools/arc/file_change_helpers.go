package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// gitChange 描述Git工作区中单个变更文件的相对路径与操作类型
type gitChange struct {
	// Path 仓库相对路径
	Path string
	// Operation 变更动作标签（added/modified/deleted/renamed/untracked）
	Operation string
}

// observedFile 校验仓库边界安全性并计算单个工作区文件的SHA256摘要与变更类型
// 请求体
// root 仓库根目录真实路径
// relative 仓库内相对文件路径
// 响应体
// sha 文件或符号链接目标的SHA256摘要
// size 文件字节数
// op 观测到的文件状态类型
// err 路径穿越、内部目录访问或IO读取错误
func observedFile(root, relative string) (string, int64, string, error) {
	// 1. 校验路径必须为仓库内安全相对路径且禁止访问 .arc 内部目录
	if relative == "" || filepath.IsAbs(relative) {
		return "", 0, "", errors.New("file path must be repository relative")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".arc" || strings.HasPrefix(clean, ".arc"+string(filepath.Separator)) {
		return "", 0, "", errors.New("unsafe or internal file path")
	}
	full := filepath.Join(root, clean)
	// 2. 解析父目录符号链接阻断向工作区外部逃逸的软链穿越
	parent, e := filepath.EvalSymlinks(filepath.Dir(full))
	if e != nil {
		return "", 0, "", e
	}
	rel, e := filepath.Rel(root, parent)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", 0, "", errors.New("file path escapes repository through symlink")
	}
	// 3. 根据文件不存在、符号链接或普通文件分类计算摘要
	st, err := os.Lstat(full)
	if errors.Is(err, os.ErrNotExist) {
		return "", 0, "deleted", nil
	}
	if err != nil {
		return "", 0, "", err
	}
	if st.Mode()&os.ModeSymlink != 0 {
		link, linkErr := os.Readlink(full)
		if linkErr != nil {
			return "", 0, "", linkErr
		}
		return shaBytes([]byte(link)), int64(len(link)), "symlink", nil
	}
	if !st.Mode().IsRegular() {
		return "", 0, "", errors.New("not a regular file")
	}
	sha, err := shaFile(full)
	return sha, st.Size(), "modified", err
}

// storeFile 在事务内对比并更新单文件哈希版本记录及索引事件
// 请求体
// ctx 请求上下文
// tx 当前数据库事务句柄
// root 仓库根目录真实路径
// path 仓库相对文件路径
// 响应体
// sha 当前文件内容SHA256摘要
// err 文件校验或数据库写入错误
func storeFile(ctx context.Context, tx *sql.Tx, root, path string) (string, error) {
	// 1. 观测工作区当前文件摘要与操作类型
	sha, size, op, err := observedFile(root, path)
	if err != nil {
		return "", err
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	// 2. 查询已存储的文件摘要，内容未变时幂等跳过版本递增
	var old string
	var version int
	err = tx.QueryRowContext(ctx, "SELECT sha256,version FROM arc_files WHERE path=?", clean).Scan(&old, &version)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if old == sha && version > 0 {
		return sha, nil
	}
	// 3. 插入或按CAS版本更新文件观测记录并追加索引事件
	if version == 0 {
		_, err = tx.ExecContext(ctx, "INSERT INTO arc_files(path,sha256,bytes,version,changed) VALUES(?,?,?,1,?)", clean, sha, size, nowUTC())
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE arc_files SET sha256=?,bytes=?,version=version+1,changed=? WHERE path=? AND version=?", sha, size, nowUTC(), clean, version)
	}
	if err != nil {
		return "", err
	}
	rawEv, mErr := json.Marshal(fileObservedEventPayload{SHA256: sha, Version: version + 1})
	if err = addEventPayload(ctx, tx, "file", clean, "file."+op, rawEv, mErr); err != nil {
		return "", err
	}
	return sha, nil
}

// changedGitPaths 解析 git status --porcelain=v1 -z 提取工作区全部变更文件
// 请求体
// root Git仓库根目录路径
// 响应体
// result 按路径排序后的变更文件切片
// err Git状态读取异常或变更数量超过1000上限错误
func changedGitPaths(root string) ([]gitChange, error) {
	// 1. 以NUL分隔模式调用 git status 避免特殊字符文件名解析歧义
	cmd := exec.Command("git", "-C", root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	// 2. 逐条解析状态码与重命名源路径并过滤内部生成的元数据文件
	parts := strings.Split(string(out), "\x00")
	ops := map[string]string{}
	for i := 0; i < len(parts); i++ {
		rec := parts[i]
		if rec == "" {
			continue
		}
		if len(rec) < 4 || rec[2] != ' ' {
			return nil, errors.New("invalid git porcelain status")
		}
		code, path := rec[:2], rec[3:]
		if strings.ContainsAny(code, "RC") {
			if i+1 >= len(parts) || parts[i+1] == "" {
				return nil, errors.New("incomplete Git rename/copy record")
			}
			original := parts[i+1]
			i++
			if strings.Contains(code, "R") && original != path && original != ".arc" && !strings.HasPrefix(original, ".arc/") && !isGeneratedIndexFile(original) {
				ops[original] = "deleted"
			}
		}
		if path == ".arc" || strings.HasPrefix(path, ".arc/") || isGeneratedIndexFile(path) {
			continue
		}
		op := "modified"
		switch {
		case strings.Contains(code, "D"):
			op = "deleted"
		case code == "??":
			op = "untracked"
		case strings.Contains(code, "A"):
			op = "added"
		case strings.ContainsAny(code, "RC"):
			op = "renamed"
		}
		ops[path] = op
		if len(ops) > 1000 {
			return nil, errors.New("too many changed files (>1000), narrow scope")
		}
	}
	// 3. 按路径字典序稳定排序输出
	paths := make([]string, 0, len(ops))
	for p := range ops {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	result := make([]gitChange, 0, len(paths))
	for _, p := range paths {
		result = append(result, gitChange{Path: p, Operation: ops[p]})
	}
	return result, nil
}
