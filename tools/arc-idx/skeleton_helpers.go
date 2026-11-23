package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reSQLTable = regexp.MustCompile(`(?i)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z0-9_."` + "`" + `]+)`)
	rePyDecl   = regexp.MustCompile(`^(\s*)(async\s+def|def|class)\s+([a-zA-Z_][a-zA-Z0-9_]*)([^:#]*)`)
	reProto    = regexp.MustCompile(`^\s*(service|message|enum|rpc)\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	reBrace    = regexp.MustCompile(`^\s*(export\s+)?(public\s+|private\s+|protected\s+)?(default\s+)?(abstract\s+)?(async\s+)?(class|interface|type|enum|struct|trait|fn|function)\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
)

// isSkeletonExt 判断文件扩展名是否属于支持提取符号骨架的源码类型
// 请求体
// rel 仓库相对路径
// 响应体
// ok 是否支持提取符号骨架
func isSkeletonExt(rel string) bool {
	// 1. 匹配支持AST或声明级骨架提取的扩展名白名单
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".java", ".kt", ".rs", ".sql", ".proto":
		return true
	default:
		return false
	}
}

// formatNode 将Go AST语法节点格式化为单行签名字符串
// 请求体
// fset 源码位置FileSet
// node 待格式化的强类型AST节点
// 响应体
// sig 格式化后的Go代码字符串
func formatNode(fset *token.FileSet, node ast.Node) string {
	// 1. 调用 go/format 渲染AST节点文本
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return ""
	}
	return buf.String()
}

// extractSQLSkeleton 从SQL迁移文件中提取全部 CREATE TABLE 建表签名与起止行号
// 请求体
// absPath SQL文件绝对路径
// 响应体
// syms 建表符号骨架切片
// err 文件读取错误
func extractSQLSkeleton(absPath string) ([]skeletonSymbol, error) {
	// 1. 读取SQL文件并按行扫描 CREATE TABLE 语句块
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var syms []skeletonSymbol
	for i := 0; i < len(lines); i++ {
		m := reSQLTable.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		name := strings.Trim(m[1], "\"`")
		end := i
		for j := i; j < len(lines); j++ {
			end = j
			if strings.Contains(lines[j], ";") {
				break
			}
		}
		syms = append(syms, skeletonSymbol{
			Kind:      "table",
			Name:      name,
			Signature: "CREATE TABLE " + name,
			StartLine: i + 1,
			EndLine:   end + 1,
			Exported:  true,
		})
		i = end
	}
	return syms, nil
}

// extractPythonSkeleton 基于缩进层级提取Python类与函数签名及行号区间
// 请求体
// absPath Python文件绝对路径
// includeAll 是否包含下划线开头的私有符号
// 响应体
// syms Python符号骨架切片
// err 文件读取错误
func extractPythonSkeleton(absPath string, includeAll bool) ([]skeletonSymbol, error) {
	// 1. 读取Python文件逐行匹配 def/class 声明并按缩进回退定位结束行
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var syms []skeletonSymbol
	for i, line := range lines {
		m := rePyDecl.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		indent := len(m[1])
		kind := strings.TrimSpace(m[2])
		name := m[3]
		exported := !strings.HasPrefix(name, "_")
		if !includeAll && !exported {
			continue
		}
		end := i
		for j := i + 1; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			curIndent := len(lines[j]) - len(strings.TrimLeft(lines[j], " \t"))
			if curIndent <= indent {
				break
			}
			end = j
		}
		syms = append(syms, skeletonSymbol{
			Kind:      kind,
			Name:      name,
			Signature: strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ":")),
			StartLine: i + 1,
			EndLine:   end + 1,
			Exported:  exported,
		})
	}
	return syms, nil
}

// extractProtoSkeleton 从Protobuf契约文件中提取service/message/enum/rpc骨架
// 请求体
// absPath Proto文件绝对路径
// 响应体
// syms Proto符号骨架切片
// err 文件读取错误
func extractProtoSkeleton(absPath string) ([]skeletonSymbol, error) {
	// 1. 扫描Proto顶层与RPC声明并匹配花括号闭合区间
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var syms []skeletonSymbol
	for i, line := range lines {
		m := reProto.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		end := findBraceEnd(lines, i)
		syms = append(syms, skeletonSymbol{
			Kind:      m[1],
			Name:      m[2],
			Signature: strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "{")),
			StartLine: i + 1,
			EndLine:   end + 1,
			Exported:  true,
		})
	}
	return syms, nil
}

// extractBraceLangSkeleton 从TS/JS/Java/Kotlin/Rust花括号语言文件中提取声明骨架
// 请求体
// absPath 源码文件绝对路径
// includeAll 是否包含非导出符号
// 响应体
// syms 符号骨架切片
// err 文件读取错误
func extractBraceLangSkeleton(absPath string, includeAll bool) ([]skeletonSymbol, error) {
	// 1. 扫描声明行判断导出修饰符并计算花括号块结束行号
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	var syms []skeletonSymbol
	for i, line := range lines {
		m := reBrace.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		kind := m[6]
		name := m[7]
		exported := strings.TrimSpace(m[1]) != "" || strings.TrimSpace(m[2]) == "public" ||
			strings.HasPrefix(strings.TrimSpace(line), "pub ")
		if !includeAll && !exported {
			continue
		}
		end := findBraceEnd(lines, i)
		sig := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "{"))
		syms = append(syms, skeletonSymbol{
			Kind:      kind,
			Name:      name,
			Signature: sig,
			StartLine: i + 1,
			EndLine:   end + 1,
			Exported:  exported,
		})
	}
	return syms, nil
}

// findBraceEnd 从起始行向后追踪花括号深度找到声明块闭合行下标
// 请求体
// lines 文件按行切分切片
// start 声明起始行下标（0-based）
// 响应体
// end 闭合行下标（0-based）
func findBraceEnd(lines []string, start int) int {
	// 1. 累计左花括号与右花括号深度，归零或遇到无块分号时返回行下标
	depth := 0
	opened := false
	for i := start; i < len(lines); i++ {
		for _, ch := range lines[i] {
			if ch == '{' {
				depth++
				opened = true
			} else if ch == '}' {
				depth--
			}
		}
		if opened && depth <= 0 {
			return i
		}
		if !opened && strings.Contains(lines[i], ";") {
			return i
		}
	}
	return start
}
