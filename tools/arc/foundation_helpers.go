package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Verdict 表示门禁与验证收据的闭合判定枚举类型
type Verdict string

const (
	VerdictUnspecified Verdict = ""
	VerdictPass        Verdict = "PASS"
	VerdictFail        Verdict = "FAIL"
	VerdictBlocked     Verdict = "BLOCKED"
	VerdictNotRun      Verdict = "NOT_RUN"
	VerdictUnplanned   Verdict = "UNPLANNED"
	VerdictFlaky       Verdict = "FLAKY"
)

// TaskStatus 表示本地任务状态机的闭合状态枚举类型
type TaskStatus string

const (
	TaskStatusUnspecified TaskStatus = ""
	TaskStatusDraft       TaskStatus = "draft"
	TaskStatusReady       TaskStatus = "ready"
	TaskStatusRunning     TaskStatus = "running"
	TaskStatusBlocked     TaskStatus = "blocked"
	TaskStatusVerifying   TaskStatus = "verifying"
	TaskStatusDone        TaskStatus = "done"
	TaskStatusReopened    TaskStatus = "reopened"
)

// nowUTC 获取当前UTC纳秒精度RFC3339时间戳
// 请求体
// 无
// 响应体
// ts RFC3339Nano格式的UTC时间字符串
func nowUTC() string {
	// 1. 获取当前UTC时间并格式化为RFC3339Nano字符串
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// nowRFC3339 获取当前UTC秒级精度RFC3339时间戳
// 请求体
// 无
// 响应体
// ts RFC3339格式的UTC时间字符串
func nowRFC3339() string {
	// 1. 获取当前UTC时间并格式化为标准RFC3339字符串
	return time.Now().UTC().Format(time.RFC3339)
}

// newID 生成带业务前缀的12字节随机十六进制唯一ID
// 请求体
// prefix 标识符业务前缀
// 响应体
// id 带前缀的随机十六进制字符串
// err 随机数读取失败错误
func newID(prefix string) (string, error) {
	// 1. 从加密安全随机源填充12字节缓冲区
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	// 2. 拼接前缀与十六进制编码字符串返回
	return prefix + hex.EncodeToString(buf[:]), nil
}

// shaFile 流式计算普通文件的SHA256十六进制摘要
// 请求体
// path 目标文件路径
// 响应体
// digest 64位十六进制SHA256摘要
// err 文件打开或读取错误
func shaFile(path string) (string, error) {
	// 1. 打开目标文件并在退出时关闭句柄
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	// 2. 流式拷贝至SHA256哈希器并输出十六进制摘要
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// shaBytes 计算字节切片的64位十六进制SHA256摘要
// 请求体
// data 原始字节切片
// 响应体
// digest 64位十六进制SHA256摘要
func shaBytes(data []byte) string {
	// 1. 计算SHA256校验和并编码为十六进制字符串
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// emitRawJSON 将已序列化的强类型JSON字节追加换行写入标准输出
// 请求体
// raw json.Marshal产出的JSON字节切片
// marshalErr 序列化阶段返回的错误
// 响应体
// err 序列化错误或标准输出写入错误
func emitRawJSON(raw []byte, marshalErr error) error {
	// 1. 优先拦截并返回上游序列化错误
	if marshalErr != nil {
		return marshalErr
	}
	// 2. 追加换行符写入标准输出流
	_, err := os.Stdout.Write(append(raw, '\n'))
	return err
}

// canonicalizeJSONBytes 以递归强类型 json.RawMessage 对JSON字节做确定性键序规范化
// 请求体
// raw 原始JSON字节切片
// 响应体
// canonical 去除空白且对象键按字典序排列的规范化JSON字节
// err 非法JSON语法错误
func canonicalizeJSONBytes(raw []byte) ([]byte, error) {
	// 1. 校验非空且语法合法并去除首尾空白
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return nil, errors.New("invalid JSON document")
	}
	// 2. 按对象、数组或标量分支递归规范化子结构
	switch trimmed[0] {
	case '{':
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return nil, err
		}
		norm := make(map[string]json.RawMessage, len(obj))
		for k, v := range obj {
			cv, err := canonicalizeJSONBytes(v)
			if err != nil {
				return nil, err
			}
			norm[k] = cv
		}
		return json.Marshal(norm)
	case '[':
		var arr []json.RawMessage
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return nil, err
		}
		norm := make([]json.RawMessage, len(arr))
		for i, v := range arr {
			cv, err := canonicalizeJSONBytes(v)
			if err != nil {
				return nil, err
			}
			norm[i] = cv
		}
		return json.Marshal(norm)
	default:
		var compact bytes.Buffer
		if err := json.Compact(&compact, trimmed); err != nil {
			return nil, err
		}
		return compact.Bytes(), nil
	}
}

// gitHead 读取指定Git仓库根目录的当前HEAD提交哈希
// 请求体
// root Git仓库根目录路径
// 响应体
// commit 当前HEAD提交哈希字符串
// err Git命令执行失败错误
func gitHead(root string) (string, error) {
	// 1. 执行 git rev-parse HEAD 获取当前提交哈希
	cmd := exec.Command("git", "-C", root, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git HEAD: %w", err)
	}
	// 2. 去除首尾空白字符后返回
	return strings.TrimSpace(string(out)), nil
}

// repositoryRoot 解析目录真实路径并在默认当前目录时向上定位Git根目录
// 请求体
// arg 目录路径参数
// 响应体
// root 解析符号链接后的仓库根目录路径
// err 路径不存在或非目录错误
func repositoryRoot(arg string) (string, error) {
	// 1. 转换为绝对路径并解析符号链接
	abs, err := filepath.Abs(arg)
	if err != nil {
		return "", err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	// 2. 校验目标路径存在且为目录
	st, err := os.Stat(abs)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("not a repository directory: %s", abs)
	}
	// 3. 当传入点号时沿父目录向上查找 .git 标记
	if arg == "." {
		for dir := abs; ; dir = filepath.Dir(dir) {
			if _, statErr := os.Lstat(filepath.Join(dir, ".git")); statErr == nil {
				return dir, nil
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	// 4. 返回最终锁定的目录绝对路径
	return abs, nil
}

// dbTransaction 在显式事务边界内执行回调函数并自动处理回滚或提交
// 请求体
// ctx 请求上下文
// db 数据库连接句柄
// fn 事务内执行的业务回调函数
// 响应体
// err 开启事务、回调执行或提交失败错误
func dbTransaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	// 1. 开启数据库事务并挂载兜底回滚
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// 2. 执行回调并在成功时提交事务
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// arcTableExists 查询SQLite元数据判断指定表是否已存在
// 请求体
// ctx 请求上下文
// db 数据库连接句柄
// name 待检测的数据表名
// 响应体
// exists 数据表是否存在
// err 元数据查询失败错误
func arcTableExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	// 1. 查询 sqlite_master 判断目标表是否存在
	var found string
	err := db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// 2. 比对返回表名与目标名称一致后返回
	return found == name, nil
}
