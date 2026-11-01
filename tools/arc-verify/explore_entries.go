package main

import (
	"os"
	"path/filepath"
	"regexp"
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
	routeLiteral  = regexp.MustCompile(`(?i)(?:HandleFunc|Handle|GET|POST|PUT|PATCH|DELETE)\(\s*"(/[A-Za-z0-9_./-]*)"`)
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
		matches := routeLiteral.FindAllStringSubmatch(text, 16)
		if strings.Contains(name, "openapi") || strings.Contains(name, "swagger") {
			matches = append(matches, openapiPath.FindAllStringSubmatch(text, 16)...)
		}
		for _, match := range matches {
			pathName := match[1]
			class := entryUndeclared
			if approved[pathName] {
				class = entryConfirmed
			}
			if strings.Contains(pathName, "://") || !strings.HasPrefix(pathName, "/") {
				class = entryForbidden
			}
			out = append(out, DiscoveredEntry{
				Class: class, Path: pathName, Method: firstMethod(text), Source: "source", File: rel, Evidence: rel,
			})
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		return out, err
	}
	return out, nil
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
