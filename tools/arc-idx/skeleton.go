package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type skeletonSymbol struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Signature string `json:"signature"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Exported  bool   `json:"exported"`
}

type skeletonFile struct {
	Path    string           `json:"path"`
	Layer   string           `json:"layer"`
	Symbols []skeletonSymbol `json:"symbols"`
}

func cmdSkeleton(args []string) error {
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

	var targets []string
	seen := map[string]bool{}
	ignore := repo.ignoreSet()
	for _, p := range positional {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(repo.Root, p)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("stat %s: %w", p, err)
		}
		if !info.IsDir() {
			rel, err := filepath.Rel(repo.Root, abs)
			if err != nil {
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

	var out []skeletonFile
	totalSymbols := 0
	for _, rel := range targets {
		syms, err := extractSkeleton(filepath.Join(repo.Root, rel), *includeAll)
		if err != nil || len(syms) == 0 {
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
		return emitJSON(map[string]any{
			"targets":      positional,
			"file_count":   len(out),
			"symbol_count": totalSymbols,
			"files":        out,
		})
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

func isSkeletonExt(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".java", ".kt", ".rs", ".sql", ".proto":
		return true
	default:
		return false
	}
}

func extractSkeleton(absPath string, includeAll bool) ([]skeletonSymbol, error) {
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

func extractGoSkeleton(absPath string, includeAll bool) ([]skeletonSymbol, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absPath, nil, 0)
	if err != nil {
		return nil, err
	}
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

func formatNode(fset *token.FileSet, node any) string {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return ""
	}
	return buf.String()
}

var (
	reSQLTable = regexp.MustCompile(`(?i)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z0-9_."` + "`" + `]+)`)
	rePyDecl   = regexp.MustCompile(`^(\s*)(async\s+def|def|class)\s+([a-zA-Z_][a-zA-Z0-9_]*)([^:#]*)`)
	reProto    = regexp.MustCompile(`^\s*(service|message|enum|rpc)\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
	reBrace    = regexp.MustCompile(`^\s*(export\s+)?(public\s+|private\s+|protected\s+)?(default\s+)?(abstract\s+)?(async\s+)?(class|interface|type|enum|struct|trait|fn|function)\s+([a-zA-Z_][a-zA-Z0-9_]*)`)
)

func extractSQLSkeleton(absPath string) ([]skeletonSymbol, error) {
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

func extractPythonSkeleton(absPath string, includeAll bool) ([]skeletonSymbol, error) {
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

func extractProtoSkeleton(absPath string) ([]skeletonSymbol, error) {
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

func extractBraceLangSkeleton(absPath string, includeAll bool) ([]skeletonSymbol, error) {
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
			(strings.HasPrefix(strings.TrimSpace(line), "pub "))
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

func findBraceEnd(lines []string, start int) int {
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
