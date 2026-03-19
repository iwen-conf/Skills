package scaffold

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultContextBudget = 1200

func isoNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func normalizeMarkdown(content string) []byte {
	return []byte(strings.TrimRight(content, "\n") + "\n")
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeIfMissing(path string, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, normalizeMarkdown(content), 0o644)
}

func writeJSONIfMissing(path string, value any) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0o644)
}

func writeFile(path string, content string) error {
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, normalizeMarkdown(content), 0o644)
}

func expandUser(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if len(path) > 1 && (path[1] == '/' || path[1] == '\\') {
		return filepath.Join(home, path[2:])
	}
	return path
}

func toRelative(root string, value string) string {
	if value == "" {
		return ""
	}
	candidate := expandUser(value)
	if !filepath.IsAbs(candidate) {
		return filepath.ToSlash(filepath.Clean(candidate))
	}
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		resolved = candidate
	}
	rel, err := filepath.Rel(root, resolved)
	if err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return filepath.Clean(resolved)
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
