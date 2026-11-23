package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Verdict 表示Storyline验证引擎的闭合判定枚举类型
type Verdict string

const (
	VerdictUnspecified Verdict = ""
	VerdictPass        Verdict = "PASS"
	VerdictFail        Verdict = "FAIL"
	VerdictBlocked     Verdict = "BLOCKED"
	VerdictNotRun      Verdict = "NOT_RUN"
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

// newRunID 生成带 SR- 前缀的加密随机运行批次ID
// 请求体
// 无
// 响应体
// id SR-前缀的随机十六进制字符串
func newRunID() string {
	// 1. 生成8字节安全随机数并编码为十六进制运行ID
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "SR-" + hex.EncodeToString(b[:])
}

// currentGitHead 读取目标仓库当前HEAD哈希并在非Git环境回退 NO_GIT
// 请求体
// repoRoot 仓库根目录路径
// 响应体
// head 提交哈希或NO_GIT回退标识
func currentGitHead(repoRoot string) string {
	// 1. 执行 git rev-parse HEAD 读取提交哈希
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "NO_GIT"
	}
	// 2. 去除首尾空白并返回
	return strings.TrimSpace(string(out))
}

// sha256Hex 计算字节切片的64位十六进制SHA256摘要
// 请求体
// data 原始字节切片
// 响应体
// digest 64位十六进制SHA256摘要
func sha256Hex(data []byte) string {
	// 1. 计算SHA256校验和并编码为十六进制字符串
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// emitRawJSON 将预序列化的JSON字节流输出至标准输出并返回进程退出码
// 请求体
// raw 已按两空格缩进序列化的JSON字节流
// marshalErr 序列化阶段错误
// 响应体
// code 输出成功返回0，编码或写入失败返回1
func emitRawJSON(raw []byte, marshalErr error) int {
	// 1. 校验具名结构体JSON序列化是否成功
	if marshalErr != nil {
		fmt.Fprintf(os.Stderr, "encode json: %v\n", marshalErr)
		return 1
	}
	// 2. 写入标准输出并补齐换行符
	if _, err := os.Stdout.Write(append(raw, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "write json: %v\n", err)
		return 1
	}
	return 0
}

// parseInterleavedFlags 解析与位置参数穿插排列的命令行标志
// 请求体
// fl 标准库FlagSet实例
// args 原始参数切片
// 响应体
// pos 提取出的位置参数切片
// err 标志解析错误
func parseInterleavedFlags(fl *flag.FlagSet, args []string) ([]string, error) {
	// 1. 循环解析当前前缀标志并收集未消费的位置参数
	var pos []string
	for len(args) > 0 {
		if err := fl.Parse(args); err != nil {
			return nil, err
		}
		args = fl.Args()
		if len(args) == 0 {
			break
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
	// 2. 返回全部位置参数
	return pos, nil
}

// copyFileBytes 流式复制单个文件并同步刷盘
// 请求体
// src 源文件路径
// dst 目标文件路径
// 响应体
// err 文件打开、目录创建或刷盘错误
func copyFileBytes(src, dst string) error {
	// 1. 打开源文件并确保目标父目录存在
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// 2. 创建目标文件流式拷贝并执行Sync刷盘
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// copyTree 递归复制整个目录树结构与文件内容
// 请求体
// src 源目录路径
// dst 目标目录路径
// 响应体
// err 目录遍历或文件复制错误
func copyTree(src, dst string) error {
	// 1. 遍历源目录按相对路径重建子目录并复制文件
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFileBytes(path, target)
	})
}

// hashPath 计算目录树的确定性相对路径与内容SHA256摘要
// 请求体
// root 目标目录根路径
// 响应体
// digest 64位十六进制目录摘要
func hashPath(root string) string {
	// 1. 收集目录内所有普通文件相对路径并排序保证跨平台稳定
	var rels []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if rel, rErr := filepath.Rel(root, path); rErr == nil {
			rels = append(rels, rel)
		}
		return nil
	})
	sort.Strings(rels)
	// 2. 顺序写入规范化相对路径与文件内容输出SHA256摘要
	h := sha256.New()
	for _, rel := range rels {
		_, _ = h.Write([]byte(filepath.ToSlash(rel)))
		if data, rerr := os.ReadFile(filepath.Join(root, rel)); rerr == nil {
			_, _ = h.Write(data)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
