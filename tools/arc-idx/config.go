package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Config is loaded from <repo>/.ai-code-index/config.json. Every field has a
// working default so a repo without a config file still indexes correctly.
type Config struct {
	// IndexDir overrides where zoekt shards live. Default:
	// ~/.cache/ai-code-index/zoekt/<repo-basename>
	IndexDir string `json:"index_dir,omitempty"`
	// IgnoreDirs are directory basenames skipped at any depth.
	IgnoreDirs []string `json:"ignore_dirs,omitempty"`
	// IgnoreFiles are file glob patterns (basename or path) never indexed.
	IgnoreFiles []string `json:"ignore_files,omitempty"`
	// IndexExts overrides the default extension allowlist ("*" disables
	// extension filtering entirely). Without a filter, repos that carry
	// bulk text/data (novel chapters, exports, dumps) poison the index.
	IndexExts []string `json:"index_exts,omitempty"`
	// MaxFiles caps how many files a single index build accepts. Default 50000.
	MaxFiles int `json:"max_files,omitempty"`
	// SizeMax is the largest file (bytes) that gets indexed. Default 1 MiB.
	SizeMax int `json:"size_max,omitempty"`
	// Profiles map a scope name to include/exclude glob lists (** supported).
	// An empty include list means "everything not excluded".
	Profiles map[string]Profile `json:"profiles,omitempty"`
}

type Profile struct {
	Include []string `json:"include,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

var defaultIgnoreDirs = []string{
	".git", "node_modules", "dist", "build", "target", ".next", ".nuxt",
	"coverage", "tmp", "vendor", ".arc", ".magi", ".ace-tool", ".venv",
	".cocoindex_code", ".pytest_cache", ".mypy_cache", ".ruff_cache",
	"__pycache__", ".idea", ".gradle", ".dart_tool", "Pods", "DerivedData",
	".ai-code-index", "oh_modules", "_release", "release", "out", "outputs",
	"uploads", "logs",
}

// defaultIndexExts is the code/config/docs allowlist. Deliberately excludes
// .txt/.csv/.log and media: an AI *code* index must not swallow bulk data.
var defaultIndexExts = []string{
	"go", "rs", "py", "rb", "php", "lua", "pl", "r",
	"js", "jsx", "ts", "tsx", "mjs", "cjs", "svelte", "vue", "astro",
	"kt", "kts", "java", "groovy", "scala", "swift", "m", "mm",
	"c", "h", "cc", "cpp", "cxx", "hpp", "hh", "cs", "zig", "ets",
	"sh", "bash", "zsh", "fish", "ps1", "bat", "cmd",
	"sql", "proto", "graphql", "gql", "thrift", "avdl",
	"yaml", "yml", "toml", "json", "json5", "jsonc", "xml", "plist",
	"html", "htm", "css", "scss", "sass", "less", "styl",
	"md", "mdx", "rst", "adoc", "tex",
	"gradle", "properties", "conf", "ini", "env", "tf", "tfvars",
	"cue", "nix", "dockerfile", "mod", "sum", "work", "editorconfig",
	"gitignore", "gitattributes", "tmpl", "gotmpl", "hbs", "ejs", "vto",
}

// specialBasenames are extension-less files that always belong in the index.
var specialBasenames = map[string]bool{
	"Makefile": true, "makefile": true, "Dockerfile": true, "Justfile": true,
	"Rakefile": true, "Gemfile": true, "Procfile": true, "BUILD": true,
	"WORKSPACE": true, "CMakeLists.txt": true, "go.mod": true, "go.sum": true,
	"LICENSE": true, "CODEOWNERS": true, "AGENTS.md": true, "CLAUDE.md": true,
}

var defaultIgnoreFiles = []string{
	"*.lock", "*-lock.json", "*.lockb", "yarn.lock", "go.sum",
	"*.min.js", "*.min.css", "*.map", "*.snap",
	"skills.index.json",
}

// Repo bundles the resolved repository root, its config, and derived paths.
type Repo struct {
	Root     string
	Name     string
	Cfg      Config
	IndexDir string // zoekt shard directory
	MetaDir  string // <root>/.ai-code-index
	TagsFile string // <MetaDir>/tags.json
	PidFile  string // <MetaDir>/daemon.pid
}

// findRepo walks upward from dir looking for .ai-code-index/ or .git/.
func findRepo(start string) (*Repo, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	probe := dir
	root := ""
	for {
		if st, err := os.Stat(filepath.Join(probe, ".ai-code-index")); err == nil && st.IsDir() {
			root = probe
			break
		}
		if st, err := os.Stat(filepath.Join(probe, ".git")); err == nil && st.IsDir() {
			root = probe
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			break
		}
		probe = parent
	}
	if root == "" {
		// No marker found: treat the starting directory itself as the repo.
		root = dir
	}
	r := &Repo{Root: root, Name: filepath.Base(root)}
	r.MetaDir = filepath.Join(root, ".ai-code-index")
	r.TagsFile = filepath.Join(r.MetaDir, "tags.json")
	r.PidFile = filepath.Join(r.MetaDir, "daemon.pid")

	cfgPath := filepath.Join(r.MetaDir, "config.json")
	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := json.Unmarshal(data, &r.Cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", cfgPath, err)
		}
	}
	if r.Cfg.SizeMax == 0 {
		r.Cfg.SizeMax = 1 << 20
	}
	if len(r.Cfg.IgnoreDirs) == 0 {
		r.Cfg.IgnoreDirs = defaultIgnoreDirs
	}
	if len(r.Cfg.IgnoreFiles) == 0 {
		r.Cfg.IgnoreFiles = defaultIgnoreFiles
	}
	if len(r.Cfg.IndexExts) == 0 {
		r.Cfg.IndexExts = defaultIndexExts
	}
	if r.Cfg.MaxFiles == 0 {
		r.Cfg.MaxFiles = 50_000
	}
	if r.Cfg.IndexDir != "" {
		r.IndexDir = expandHome(r.Cfg.IndexDir)
	} else if env := os.Getenv("AI_CODE_INDEX_DIR"); env != "" {
		r.IndexDir = filepath.Join(expandHome(env), r.Name)
	} else {
		home, _ := os.UserHomeDir()
		r.IndexDir = filepath.Join(home, ".cache", "ai-code-index", "zoekt", r.Name)
	}
	return r, nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func (r *Repo) ignoreSet() map[string]bool {
	set := make(map[string]bool, len(r.Cfg.IgnoreDirs))
	for _, d := range r.Cfg.IgnoreDirs {
		set[d] = true
	}
	return set
}

// ignoredFile reports whether a repo-relative path matches ignore_files.
func (r *Repo) ignoredFile(relPath string) bool {
	base := filepath.Base(relPath)
	for _, g := range r.Cfg.IgnoreFiles {
		if ok, _ := filepath.Match(g, base); ok {
			return true
		}
		if ok, _ := filepath.Match(g, relPath); ok {
			return true
		}
	}
	return false
}

// indexableExt reports whether the file's extension (or special basename)
// belongs in the index per the allowlist.
func (r *Repo) indexableExt(relPath string) bool {
	base := filepath.Base(relPath)
	if specialBasenames[base] {
		return true
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(base)), ".")
	if ext == "" {
		// Dot-files like .gitignore: try the name after the dot.
		if strings.HasPrefix(base, ".") {
			ext = strings.ToLower(base[1:])
		}
	}
	for _, e := range r.Cfg.IndexExts {
		if e == "*" || e == ext {
			return true
		}
	}
	return false
}

// parseInterleaved parses flags that may appear before, between, or after
// positional arguments (Go's flag package stops at the first positional).
func parseInterleaved(fl *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for len(args) > 0 {
		if err := fl.Parse(args); err != nil {
			return nil, err
		}
		args = fl.Args()
		if len(args) == 0 {
			break
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
	return positional, nil
}

// profileMatcher compiles a profile's globs; a nil matcher accepts all paths.
type profileMatcher struct {
	include []*regexp.Regexp
	exclude []*regexp.Regexp
}

func (r *Repo) matcher(profile string) (*profileMatcher, error) {
	if profile == "" || profile == "all" {
		return &profileMatcher{}, nil
	}
	p, ok := r.Cfg.Profiles[profile]
	if !ok {
		known := make([]string, 0, len(r.Cfg.Profiles)+1)
		known = append(known, "all")
		for k := range r.Cfg.Profiles {
			known = append(known, k)
		}
		return nil, fmt.Errorf("unknown profile %q (available: %s)", profile, strings.Join(known, ", "))
	}
	m := &profileMatcher{}
	for _, g := range p.Include {
		re, err := globToRegexp(g)
		if err != nil {
			return nil, fmt.Errorf("profile %s include %q: %w", profile, g, err)
		}
		m.include = append(m.include, re)
	}
	for _, g := range p.Exclude {
		re, err := globToRegexp(g)
		if err != nil {
			return nil, fmt.Errorf("profile %s exclude %q: %w", profile, g, err)
		}
		m.exclude = append(m.exclude, re)
	}
	return m, nil
}

func (m *profileMatcher) match(relPath string) bool {
	for _, re := range m.exclude {
		if re.MatchString(relPath) {
			return false
		}
	}
	if len(m.include) == 0 {
		return true
	}
	for _, re := range m.include {
		if re.MatchString(relPath) {
			return true
		}
	}
	return false
}

// globToRegexp converts a **-style glob into an anchored regexp over
// slash-separated relative paths. `**` crosses directories, `*` and `?`
// stay within one segment. A trailing `/` or bare directory name matches
// the whole subtree.
func globToRegexp(glob string) (*regexp.Regexp, error) {
	g := strings.TrimPrefix(glob, "./")
	var b strings.Builder
	b.WriteString(`^`)
	i := 0
	for i < len(g) {
		c := g[i]
		switch c {
		case '*':
			if strings.HasPrefix(g[i:], "**/") {
				b.WriteString(`(?:[^/]+/)*`)
				i += 3
				continue
			}
			if strings.HasPrefix(g[i:], "**") {
				b.WriteString(`.*`)
				i += 2
				continue
			}
			b.WriteString(`[^/]*`)
		case '?':
			b.WriteString(`[^/]`)
		case '.', '+', '(', ')', '|', '[', ']', '{', '}', '^', '$', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
		i++
	}
	// Bare directory globs like "docs" or "docs/" cover the subtree.
	if strings.HasSuffix(g, "/") {
		b.WriteString(`.*$`)
	} else if !strings.ContainsAny(g, "*?.") {
		b.WriteString(`(?:/.*)?$`)
	} else {
		b.WriteString(`$`)
	}
	return regexp.Compile(b.String())
}
