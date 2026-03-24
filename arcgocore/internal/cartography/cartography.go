package cartography

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	Version     = "1.0.0"
	StateDir    = ".slim"
	StateFile   = "cartography.json"
	CodemapFile = "codemap.md"
)

type Metadata struct {
	Version         string   `json:"version"`
	LastRun         string   `json:"last_run"`
	Root            string   `json:"root"`
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	Exceptions      []string `json:"exceptions"`
}

type State struct {
	Metadata     Metadata          `json:"metadata"`
	FileHashes   map[string]string `json:"file_hashes"`
	FolderHashes map[string]string `json:"folder_hashes"`
}

type ChangeSet struct {
	Added           []string
	Removed         []string
	Modified        []string
	AffectedFolders []string
}

type PatternMatcher struct {
	regex *regexp.Regexp
}

func NewPatternMatcher(patterns []string) (*PatternMatcher, error) {
	if len(patterns) == 0 {
		return &PatternMatcher{}, nil
	}
	parts := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		normalized := filepath.ToSlash(pattern)
		reg := regexp.QuoteMeta(normalized)
		reg = strings.ReplaceAll(reg, `\*\*/`, `(?:.*/)?`)
		reg = strings.ReplaceAll(reg, `\*\*`, `.*`)
		reg = strings.ReplaceAll(reg, `\*`, `[^/]*`)
		reg = strings.ReplaceAll(reg, `\?`, `.`)
		if strings.HasSuffix(normalized, "/") {
			reg += `.*`
		}
		if strings.HasPrefix(normalized, "/") {
			reg = `^` + reg[1:]
		} else {
			reg = `(?:^|.*/)` + reg
		}
		parts = append(parts, `(?:`+reg+`$)`)
	}
	if len(parts) == 0 {
		return &PatternMatcher{}, nil
	}
	compiled, err := regexp.Compile(strings.Join(parts, `|`))
	if err != nil {
		return nil, err
	}
	return &PatternMatcher{regex: compiled}, nil
}

func (m *PatternMatcher) Matches(target string) bool {
	if m == nil || m.regex == nil {
		return false
	}
	return m.regex.FindStringIndex(filepath.ToSlash(target)) != nil
}

func LoadGitignore(root string) []string {
	payload, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		return nil
	}
	lines := strings.Split(string(payload), "\n")
	patterns := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		patterns = append(patterns, trimmed)
	}
	return patterns
}

func SelectFiles(root string, includePatterns []string, excludePatterns []string, exceptions []string, gitignorePatterns []string) ([]string, error) {
	includeMatcher, err := NewPatternMatcher(includePatterns)
	if err != nil {
		return nil, err
	}
	excludeMatcher, err := NewPatternMatcher(excludePatterns)
	if err != nil {
		return nil, err
	}
	gitignoreMatcher, err := NewPatternMatcher(gitignorePatterns)
	if err != nil {
		return nil, err
	}
	exceptionSet := map[string]struct{}{}
	for _, item := range exceptions {
		exceptionSet[filepath.ToSlash(strings.TrimPrefix(item, "./"))] = struct{}{}
	}
	selected := make([]string, 0)
	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if gitignoreMatcher.Matches(rel) {
			return nil
		}
		if excludeMatcher.Matches(rel) {
			if _, ok := exceptionSet[rel]; !ok {
				return nil
			}
		}
		if includeMatcher.Matches(rel) {
			selected = append(selected, rel)
			return nil
		}
		if _, ok := exceptionSet[rel]; ok {
			selected = append(selected, rel)
		}
		return nil
	})
	sort.Strings(selected)
	return selected, err
}

func ComputeFileHash(path string) string {
	payload, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	digest := md5.Sum(payload)
	return hex.EncodeToString(digest[:])
}

func ComputeFolderHash(folder string, fileHashes map[string]string) string {
	rows := make([]string, 0)
	for file, hash := range fileHashes {
		if folder == "." {
			if !strings.Contains(file, "/") {
				rows = append(rows, fmt.Sprintf("%s:%s", file, hash))
			}
			continue
		}
		if strings.HasPrefix(file, folder+"/") {
			rows = append(rows, fmt.Sprintf("%s:%s", file, hash))
		}
	}
	if len(rows) == 0 {
		return ""
	}
	sort.Strings(rows)
	digest := md5.Sum([]byte(strings.Join(rows, "\n") + "\n"))
	return hex.EncodeToString(digest[:])
}

func GetFoldersWithFiles(files []string) []string {
	folders := map[string]struct{}{".": {}}
	for _, file := range files {
		parts := strings.Split(file, "/")
		for index := 1; index < len(parts); index++ {
			folders[strings.Join(parts[:index], "/")] = struct{}{}
		}
	}
	result := make([]string, 0, len(folders))
	for folder := range folders {
		result = append(result, folder)
	}
	sort.Strings(result)
	return result
}

func LoadState(root string) (*State, error) {
	path := filepath.Join(root, StateDir, StateFile)
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state State
	if err := json.Unmarshal(payload, &state); err != nil {
		return nil, nil
	}
	if state.FileHashes == nil {
		state.FileHashes = map[string]string{}
	}
	if state.FolderHashes == nil {
		state.FolderHashes = map[string]string{}
	}
	return &state, nil
}

func SaveState(root string, state State) error {
	targetDir := filepath.Join(root, StateDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	temp, err := os.CreateTemp(targetDir, "cartography-*.json")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if _, err := temp.Write(payload); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, filepath.Join(targetDir, StateFile))
}

func CreateEmptyCodemap(folderPath string, folderName string) error {
	codemapPath := filepath.Join(folderPath, CodemapFile)
	if _, err := os.Stat(codemapPath); err == nil {
		return nil
	}
	content := strings.Join([]string{
		fmt.Sprintf("# %s/", folderName),
		"",
		"## Responsibility",
		"",
		"## Design",
		"",
		"## Flow",
		"",
		"## Integration",
		"",
	}, "\n")
	return os.WriteFile(codemapPath, []byte(content), 0o644)
}

func Init(root string, includePatterns []string, excludePatterns []string, exceptions []string) (State, []string, error) {
	if len(includePatterns) == 0 {
		includePatterns = []string{"**/*"}
	}
	files, err := SelectFiles(root, includePatterns, excludePatterns, exceptions, LoadGitignore(root))
	if err != nil {
		return State{}, nil, err
	}
	fileHashes := map[string]string{}
	for _, rel := range files {
		fileHashes[rel] = ComputeFileHash(filepath.Join(root, rel))
	}
	folders := GetFoldersWithFiles(files)
	folderHashes := map[string]string{}
	for _, folder := range folders {
		folderHashes[folder] = ComputeFolderHash(folder, fileHashes)
	}
	state := State{
		Metadata:     Metadata{Version: Version, LastRun: time.Now().UTC().Format(time.RFC3339Nano), Root: root, IncludePatterns: includePatterns, ExcludePatterns: excludePatterns, Exceptions: exceptions},
		FileHashes:   fileHashes,
		FolderHashes: folderHashes,
	}
	if err := SaveState(root, state); err != nil {
		return State{}, nil, err
	}
	for _, folder := range folders {
		folderPath := root
		folderName := filepath.Base(root)
		if folder != "." {
			folderPath = filepath.Join(root, filepath.FromSlash(folder))
			folderName = folder
		}
		if err := CreateEmptyCodemap(folderPath, folderName); err != nil {
			return State{}, nil, err
		}
	}
	return state, folders, nil
}

func DetectChanges(root string, state *State) (ChangeSet, map[string]string, error) {
	if state == nil {
		return ChangeSet{}, nil, fmt.Errorf("no cartography state found")
	}
	includePatterns := state.Metadata.IncludePatterns
	if len(includePatterns) == 0 {
		includePatterns = []string{"**/*"}
	}
	files, err := SelectFiles(root, includePatterns, state.Metadata.ExcludePatterns, state.Metadata.Exceptions, LoadGitignore(root))
	if err != nil {
		return ChangeSet{}, nil, err
	}
	currentHashes := map[string]string{}
	for _, rel := range files {
		currentHashes[rel] = ComputeFileHash(filepath.Join(root, rel))
	}
	change := ChangeSet{}
	affected := map[string]struct{}{}
	for key := range currentHashes {
		if _, ok := state.FileHashes[key]; !ok {
			change.Added = append(change.Added, key)
			collectAffectedFolders(key, affected)
		}
	}
	for key := range state.FileHashes {
		if _, ok := currentHashes[key]; !ok {
			change.Removed = append(change.Removed, key)
			collectAffectedFolders(key, affected)
		}
	}
	for key, hash := range currentHashes {
		if oldHash, ok := state.FileHashes[key]; ok && oldHash != hash {
			change.Modified = append(change.Modified, key)
			collectAffectedFolders(key, affected)
		}
	}
	sort.Strings(change.Added)
	sort.Strings(change.Removed)
	sort.Strings(change.Modified)
	change.AffectedFolders = foldersFromSet(affected)
	return change, currentHashes, nil
}

func Update(root string, state *State) (State, error) {
	if state == nil {
		return State{}, fmt.Errorf("no cartography state found")
	}
	includePatterns := state.Metadata.IncludePatterns
	if len(includePatterns) == 0 {
		includePatterns = []string{"**/*"}
	}
	files, err := SelectFiles(root, includePatterns, state.Metadata.ExcludePatterns, state.Metadata.Exceptions, LoadGitignore(root))
	if err != nil {
		return State{}, err
	}
	fileHashes := map[string]string{}
	for _, rel := range files {
		fileHashes[rel] = ComputeFileHash(filepath.Join(root, rel))
	}
	folders := GetFoldersWithFiles(files)
	folderHashes := map[string]string{}
	for _, folder := range folders {
		folderHashes[folder] = ComputeFolderHash(folder, fileHashes)
	}
	next := State{Metadata: state.Metadata, FileHashes: fileHashes, FolderHashes: folderHashes}
	next.Metadata.LastRun = time.Now().UTC().Format(time.RFC3339Nano)
	if err := SaveState(root, next); err != nil {
		return State{}, err
	}
	return next, nil
}

func Export(root string, state *State, tier int) (map[string]any, error) {
	if state == nil {
		return nil, fmt.Errorf("no cartography state found")
	}
	includePatterns := state.Metadata.IncludePatterns
	if len(includePatterns) == 0 {
		includePatterns = []string{"**/*"}
	}
	files, err := SelectFiles(root, includePatterns, state.Metadata.ExcludePatterns, state.Metadata.Exceptions, LoadGitignore(root))
	if err != nil {
		return nil, err
	}
	entries := make([]map[string]any, 0, len(files))
	totalChars := 0
	for _, rel := range files {
		entry := extractFileInfo(filepath.Join(root, rel), root, tier)
		totalChars += len(fmt.Sprint(entry))
		entries = append(entries, entry)
	}
	return map[string]any{
		"version":               Version,
		"tier":                  tier,
		"generated_at":          time.Now().UTC().Format(time.RFC3339Nano),
		"producer_skill":        "arc-cartography",
		"path":                  root,
		"total_tokens_estimate": totalChars / 4,
		"entries":               entries,
	}, nil
}

func PrintChanges(change ChangeSet) string {
	if len(change.Added)+len(change.Removed)+len(change.Modified) == 0 {
		return "No changes detected.\n"
	}
	lines := make([]string, 0)
	if len(change.Added) > 0 {
		lines = append(lines, fmt.Sprintf("\n%d added:", len(change.Added)))
		for _, item := range change.Added {
			lines = append(lines, fmt.Sprintf("  + %s", item))
		}
	}
	if len(change.Removed) > 0 {
		lines = append(lines, fmt.Sprintf("\n%d removed:", len(change.Removed)))
		for _, item := range change.Removed {
			lines = append(lines, fmt.Sprintf("  - %s", item))
		}
	}
	if len(change.Modified) > 0 {
		lines = append(lines, fmt.Sprintf("\n%d modified:", len(change.Modified)))
		for _, item := range change.Modified {
			lines = append(lines, fmt.Sprintf("  ~ %s", item))
		}
	}
	lines = append(lines, fmt.Sprintf("\n%d folders affected:", len(change.AffectedFolders)))
	for _, folder := range change.AffectedFolders {
		lines = append(lines, fmt.Sprintf("  %s/", folder))
	}
	return strings.Join(lines, "\n") + "\n"
}

func WriteExport(pathValue string, payload map[string]any) error {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if pathValue == "-" {
		_, err = os.Stdout.Write(encoded)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(pathValue), 0o755); err != nil {
		return err
	}
	return os.WriteFile(pathValue, encoded, 0o644)
}

func collectAffectedFolders(path string, affected map[string]struct{}) {
	affected["."] = struct{}{}
	parts := strings.Split(path, "/")
	for index := 1; index < len(parts); index++ {
		affected[strings.Join(parts[:index], "/")] = struct{}{}
	}
}

func foldersFromSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

func extractFileInfo(filePath string, root string, tier int) map[string]any {
	rel, _ := filepath.Rel(root, filePath)
	rel = filepath.ToSlash(rel)
	entry := map[string]any{"name": filepath.Base(filePath), "type": "file", "path": rel, "line_start": 1}
	payload, err := os.ReadFile(filePath)
	if err != nil {
		return entry
	}
	content := string(payload)
	lines := strings.Split(content, "\n")
	entry["line_end"] = len(lines)
	language := map[string]string{".py": "python", ".js": "javascript", ".ts": "typescript", ".tsx": "tsx", ".jsx": "jsx", ".java": "java", ".go": "go", ".rs": "rust", ".rb": "ruby", ".php": "php", ".c": "c", ".cpp": "cpp", ".h": "c", ".hpp": "cpp", ".cs": "csharp", ".swift": "swift", ".kt": "kotlin", ".scala": "scala", ".md": "markdown", ".json": "json", ".yaml": "yaml", ".yml": "yaml", ".sh": "shell", ".bash": "shell", ".zsh": "shell"}[strings.ToLower(filepath.Ext(filePath))]
	if language == "" {
		language = "unknown"
	}
	entry["language"] = language
	if tier >= 1 {
		if signature := extractSignature(content, language); signature != "" {
			entry["signature"] = signature
		}
	}
	if tier >= 2 {
		if dependencies := extractDependencies(content, language); len(dependencies) > 0 {
			entry["dependencies"] = dependencies
		}
		if summary := extractSummary(lines); summary != "" {
			entry["summary"] = summary
		}
	}
	if tier >= 3 {
		if doc := extractDocstring(content, language); doc != "" {
			entry["docstring"] = doc
		}
		if len(content) <= 10000 {
			entry["code"] = content
		} else {
			entry["code"] = content[:10000] + "\n... (truncated)"
		}
	}
	return entry
}

func extractSignature(content string, language string) string {
	patterns := map[string][]string{"python": {"def ", "class ", "async def "}, "javascript": {"function ", "const ", "export ", "class "}, "typescript": {"function ", "const ", "export ", "class ", "interface "}, "go": {"func ", "type "}, "rust": {"fn ", "pub fn ", "struct ", "enum "}, "java": {"public ", "class ", "interface "}, "ruby": {"def ", "class ", "module "}}
	lines := strings.Split(content, "\n")
	if len(lines) > 50 {
		lines = lines[:50]
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, pattern := range patterns[language] {
			if strings.HasPrefix(trimmed, pattern) {
				if len(trimmed) > 100 {
					return trimmed[:100]
				}
				return trimmed
			}
		}
	}
	return ""
}

func extractDependencies(content string, language string) []string {
	patterns := map[string][]string{"python": {"import ", "from "}, "javascript": {"import ", "require("}, "typescript": {"import ", "require("}, "go": {"import "}, "rust": {"use "}, "java": {"import "}, "ruby": {"require ", "include "}}
	lines := strings.Split(content, "\n")
	if len(lines) > 100 {
		lines = lines[:100]
	}
	result := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, pattern := range patterns[language] {
			if strings.HasPrefix(trimmed, pattern) {
				if len(trimmed) > 100 {
					trimmed = trimmed[:100]
				}
				result = append(result, trimmed)
				break
			}
		}
		if len(result) >= 20 {
			break
		}
	}
	return result
}

func extractSummary(lines []string) string {
	if len(lines) > 20 {
		lines = lines[:20]
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") && len(trimmed) > 3 {
			return trimmed[2:]
		}
		if strings.HasPrefix(trimmed, "// ") && len(trimmed) > 4 {
			return trimmed[3:]
		}
	}
	return ""
}

func extractDocstring(content string, language string) string {
	if language == "python" {
		if strings.HasPrefix(content, `"""`) {
			if end := strings.Index(content[3:], `"""`); end >= 0 {
				return content[3 : end+3]
			}
		}
		if strings.HasPrefix(content, "'''") {
			if end := strings.Index(content[3:], "'''"); end >= 0 {
				return content[3 : end+3]
			}
		}
	}
	if strings.HasPrefix(content, "/*") {
		if end := strings.Index(content[2:], "*/"); end >= 0 {
			return content[2 : end+2]
		}
	}
	return ""
}
