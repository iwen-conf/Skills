package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	entryConfirmed   = "CONFIRMED_ENTRY"
	entryUndeclared  = "UNDECLARED_ENTRY"
	entryInferred    = "INFERRED_ENTRY"
	entryUnavailable = "UNAVAILABLE_ENTRY"
	entryForbidden   = "FORBIDDEN_ENTRY"
)

type DiscoveredEntry struct {
	Class    string `json:"class"`
	Action   string `json:"action,omitempty"`
	Method   string `json:"method,omitempty"`
	Path     string `json:"path"`
	Role     string `json:"role,omitempty"`
	Source   string `json:"source"`
	File     string `json:"file,omitempty"`
	Evidence string `json:"evidence,omitempty"`
}

var (
	openapiPath   = regexp.MustCompile(`(?m)^[[:space:]]{0,2}(/[A-Za-z0-9_{}/.-]+):[[:space:]]*$`)
	httpMethodLit = regexp.MustCompile(`(?i)"(GET|POST|PUT|PATCH|DELETE)"`)
)

func discoverSourceEntries(root string, adapter ExploreAdapter) ([]DiscoveredEntry, error) {
	if root == "" {
		return nil, nil
	}
	approved := map[string]bool{}
	for _, entry := range adapter.Entries {
		approved[entry.Entry] = true
	}
	out := []DiscoveredEntry{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			if entry != nil && (entry.Name() == ".git" || entry.Name() == "node_modules" || entry.Name() == "vendor") {
				return filepath.SkipDir
			}
			return walkErr
		}
		if len(out) >= 64 {
			return filepath.SkipAll
		}
		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".json") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil || len(body) > 1<<20 {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		text := string(body)
		found := []DiscoveredEntry{}
		if strings.HasSuffix(name, ".go") {
			found = append(found, discoverGoHTTPEntries(rel, body, approved)...)
		}
		if strings.Contains(name, "openapi") || strings.Contains(name, "swagger") {
			found = append(found, discoverOpenAPIEntries(rel, body, approved)...)
			for _, match := range openapiPath.FindAllStringSubmatch(text, 16) {
				found = append(found, classifyDiscoveredPath(match[1], "YAML", rel, approved))
			}
		}
		for _, item := range found {
			if len(out) >= 64 { return filepath.SkipAll }
			out = append(out, item)
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return out, err
	}
	return out, nil
}

func discoverGoHTTPEntries(rel string, src []byte, approved map[string]bool) []DiscoveredEntry {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, src, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}
	out := []DiscoveredEntry{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := callSelector(call)
		method := ""
		switch name {
		case "HandleFunc", "Handle":
			method = "ANY" // net/http HandleFunc does not restrict the method
		case "GET", "POST", "PUT", "PATCH", "DELETE":
			method = name
		default:
			return true
		}
		if len(call.Args) == 0 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		pathName, err := strconv.Unquote(lit.Value)
		if err != nil || !strings.HasPrefix(pathName, "/") {
			return true
		}
		item := classifyDiscoveredPath(pathName, method, rel, approved)
		item.Source = "go-ast"
		item.Evidence = rel + ":" + name
		out = append(out, item)
		return true
	})
	return out
}

func callSelector(call *ast.CallExpr) string {
	switch expr := call.Fun.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.SelectorExpr:
		return expr.Sel.Name
	default:
		return ""
	}
}

func discoverOpenAPIEntries(rel string, src []byte, approved map[string]bool) []DiscoveredEntry {
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if json.Unmarshal(src, &doc) != nil || len(doc.Paths) == 0 {
		return nil
	}
	out := []DiscoveredEntry{}
	for pathName, methods := range doc.Paths {
		method := "POST"
		for name := range methods {
			method = strings.ToUpper(name)
			break
		}
		item := classifyDiscoveredPath(pathName, method, rel, approved)
		item.Source = "openapi"
		out = append(out, item)
	}
	return out
}

func classifyDiscoveredPath(pathName, method, rel string, approved map[string]bool) DiscoveredEntry {
	class := entryUndeclared
	if approved[pathName] {
		class = entryConfirmed
	}
	if strings.Contains(pathName, "://") || !strings.HasPrefix(pathName, "/") {
		class = entryForbidden
	}
	return DiscoveredEntry{Class: class, Path: pathName, Method: method, Source: "source", File: rel, Evidence: rel}
}

func firstMethod(text string) string {
	if match := httpMethodLit.FindStringSubmatch(text); len(match) == 2 {
		return strings.ToUpper(match[1])
	}
	return "POST"
}

func authorizeDiscoveredEntry(item DiscoveredEntry, adapter ExploreAdapter) DiscoveredEntry {
	if item.Class == entryForbidden {
		return item
	}
	for _, entry := range adapter.Entries {
		if entry.Entry == item.Path {
			item.Class = entryConfirmed
			item.Action = entry.Action
			return item
		}
	}
	if item.Class == entryInferred {
		item.Class = entryForbidden
		return item
	}
	item.Class = entryUndeclared
	return item
}
