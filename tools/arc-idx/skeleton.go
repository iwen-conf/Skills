package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// skeletonSymbol 描述单个源码符号的类别、脱敏签名与行号区间
type skeletonSymbol struct {
	// Kind 符号类别（func/method/struct/interface/table等）
	Kind string `json:"kind"`
	// Name 符号名称
	Name string `json:"name"`
	// Signature 不含函数体的声明签名文本
	Signature string `json:"signature"`
	// StartLine 起始行号（1-based）
	StartLine int `json:"start_line"`
	// EndLine 结束行号（1-based）
	EndLine int `json:"end_line"`
	// Exported 是否为导出公开符号
	Exported bool `json:"exported"`
}

// skeletonFile 描述单个文件的层级分类与提取出的符号骨架列表
type skeletonFile struct {
	// Path 仓库相对文件路径
	Path string `json:"path"`
	// Layer 权威层级分类（prod/contract/test/doc）
	Layer string `json:"layer"`
	// Symbols 提取出的符号列表
	Symbols []skeletonSymbol `json:"symbols"`
}

// cmdSkeleton 提取目标文件或目录的公开符号签名骨架并按JSON或Markdown输出
// 请求体
// args 命令行参数切片
// 响应体
// err 路径不存在或输出编码错误
func cmdSkeleton(args []string) error {
	// 1. 解析仓库目录、是否包含私有符号与输出格式标志
	fl := flag.NewFlagSet("skeleton", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	includeAll := fl.Bool("all", false, "include unexported/private symbols")
	formatMode := fl.String("format", "ai-md", "output format: json | ai-md")
	positional, err := parseInterleaved(fl, args)
	if err != nil {
		return err
	}
	if len(positional) == 0 {
		return fmt.Errorf("usage: arc idx skeleton <path...> [--all] [--format json|ai-md]")
	}
	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}
	// 2. 展开目标文件与目录树收集去重后的候选相对路径
	var targets []string
	seen := map[string]bool{}
	ignore := repo.ignoreSet()
	for _, p := range positional {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(repo.Root, p)
		}
		info, statErr := os.Stat(abs)
		if statErr != nil {
			return fmt.Errorf("stat %s: %w", p, statErr)
		}
		if !info.IsDir() {
			rel, rErr := filepath.Rel(repo.Root, abs)
			if rErr != nil {
				rel = p
			}
			rel = filepath.ToSlash(rel)
			if !seen[rel] {
				seen[rel] = true
				targets = append(targets, rel)
			}
			continue
		}
		_ = filepath.WalkDir(abs, func(path string, d fs.DirEntry, werr error) error {
			if werr != nil {
				return nil
			}
			if d.IsDir() {
				if path != abs && (ignore[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
					return fs.SkipDir
				}
				return nil
			}
			rel, rerr := filepath.Rel(repo.Root, path)
			if rerr != nil {
				return nil
			}
			rel = filepath.ToSlash(rel)
			if repo.ignoredFile(rel) || !isSkeletonExt(rel) {
				return nil
			}
			if !seen[rel] {
				seen[rel] = true
				targets = append(targets, rel)
			}
			return nil
		})
	}
	sort.Strings(targets)
	// 3. 逐文件提取符号骨架并输出具名结构体JSON或紧凑Markdown
	var out []skeletonFile
	totalSymbols := 0
	for _, rel := range targets {
		syms, exErr := extractSkeleton(filepath.Join(repo.Root, rel), *includeAll)
		if exErr != nil || len(syms) == 0 {
			continue
		}
		out = append(out, skeletonFile{
			Path:    rel,
			Layer:   pathLayer(rel),
			Symbols: syms,
		})
		totalSymbols += len(syms)
	}
	if *formatMode == "json" {
		return emitRawJSON(json.MarshalIndent(skeletonQueryResponse{
			Targets:     positional,
			FileCount:   len(out),
			SymbolCount: totalSymbols,
			Files:       out,
		}, "", "  "))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## skeleton (%d files, %d symbols)\n\n", len(out), totalSymbols)
	for _, f := range out {
		fmt.Fprintf(&b, "### %s [%s]\n", f.Path, f.Layer)
		for _, s := range f.Symbols {
			fmt.Fprintf(&b, "- L%d–L%d `%s` `%s`\n", s.StartLine, s.EndLine, s.Kind, s.Signature)
		}
		b.WriteByte('\n')
	}
	fmt.Print(b.String())
	return nil
}

// extractSkeleton 根据文件扩展名分派至对应语言的符号骨架解析器
// 请求体
// absPath 目标文件绝对路径
// includeAll 是否包含非导出私有符号
// 响应体
// syms 符号骨架切片
// err 文件解析错误
func extractSkeleton(absPath string, includeAll bool) ([]skeletonSymbol, error) {
	// 1. 按扩展名路由到Go原生AST或SQL/Python/Proto/通用花括号解析器
	ext := strings.ToLower(filepath.Ext(absPath))
	switch ext {
	case ".go":
		return extractGoSkeleton(absPath, includeAll)
	case ".sql":
		return extractSQLSkeleton(absPath)
	case ".py":
		return extractPythonSkeleton(absPath, includeAll)
	case ".proto":
		return extractProtoSkeleton(absPath)
	default:
		return extractBraceLangSkeleton(absPath, includeAll)
	}
}

// extractGoSkeleton 使用 go/parser 原生AST提取Go源码的类型与函数签名（剥离函数体）
// 请求体
// absPath Go源码文件绝对路径
// includeAll 是否包含未导出符号
// 响应体
// syms Go符号骨架切片
// err Go语法解析错误
func extractGoSkeleton(absPath string, includeAll bool) ([]skeletonSymbol, error) {
	// 1. 解析Go源码AST语法树
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		return nil, err
	}
	// 2. 遍历顶层函数、方法与类型声明，剥离Body与Doc后生成纯签名
	var syms []skeletonSymbol
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			exported := d.Name.IsExported()
			if !includeAll && !exported {
				continue
			}
			kind := "func"
			name := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				kind = "method"
				name = formatNode(fset, d.Recv.List[0].Type) + "." + d.Name.Name
			}
			sigDecl := *d
			sigDecl.Body = nil
			sigDecl.Doc = nil
			sig := strings.TrimSpace(formatNode(fset, &sigDecl))
			syms = append(syms, skeletonSymbol{
				Kind:      kind,
				Name:      name,
				Signature: sig,
				StartLine: fset.Position(d.Pos()).Line,
				EndLine:   fset.Position(d.End()).Line,
				Exported:  exported,
			})
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				exported := ts.Name.IsExported()
				if !includeAll && !exported {
					continue
				}
				kind := "type"
				sig := "type " + ts.Name.Name
				switch t := ts.Type.(type) {
				case *ast.StructType:
					kind = "struct"
					sig = fmt.Sprintf("type %s struct { /* %d fields */ }", ts.Name.Name, len(t.Fields.List))
				case *ast.InterfaceType:
					kind = "interface"
					var methods []string
					for _, m := range t.Methods.List {
						for _, n := range m.Names {
							methods = append(methods, n.Name)
						}
					}
					sig = fmt.Sprintf("type %s interface { %s }", ts.Name.Name, strings.Join(methods, "; "))
				default:
					sig = "type " + ts.Name.Name + " " + formatNode(fset, ts.Type)
				}
				syms = append(syms, skeletonSymbol{
					Kind:      kind,
					Name:      ts.Name.Name,
					Signature: sig,
					StartLine: fset.Position(ts.Pos()).Line,
					EndLine:   fset.Position(ts.End()).Line,
					Exported:  exported,
				})
			}
		}
	}
	return syms, nil
}
