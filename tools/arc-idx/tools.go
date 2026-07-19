package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ---- arc-idx symbol -------------------------------------------------------

// symbolEntry mirrors the ctags JSON line format we care about.
type symbolEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Line      int    `json:"line"`
	Kind      string `json:"kind"`
	Language  string `json:"language,omitempty"`
	Scope     string `json:"scope,omitempty"`
	ScopeKind string `json:"scopeKind,omitempty"`
	Signature string `json:"signature,omitempty"`
}

func cmdSymbol(args []string) error {
	fl := flag.NewFlagSet("symbol", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	kind := fl.String("kind", "", "filter by ctags kind (func, struct, class, method, ...)")
	lang := fl.String("lang", "", "filter by language")
	exact := fl.Bool("exact", false, "exact name match instead of substring")
	format := fl.String("format", "ai-md", "output format: json | ai-md")
	limit := fl.Int("max", 100, "maximum symbols to return")
	positional, err := parseInterleaved(fl, args)
	if err != nil {
		return err
	}
	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(repo.TagsFile); err != nil {
		walked, werr := walkRepo(repo)
		if werr != nil {
			return werr
		}
		if err := buildTags(repo, walked); err != nil {
			return fmt.Errorf("no tags file and rebuild failed: %w", err)
		}
	}
	needle := ""
	if len(positional) > 0 {
		needle = positional[0]
	}

	f, err := os.Open(repo.TagsFile)
	if err != nil {
		return err
	}
	defer f.Close()

	var out []symbolEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var e symbolEntry
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Name == "" {
			continue
		}
		e.Path = normalizeTagPath(e.Path, repo.Root)
		if needle != "" {
			if *exact && e.Name != needle {
				continue
			}
			if !*exact && !strings.Contains(strings.ToLower(e.Name), strings.ToLower(needle)) {
				continue
			}
		}
		if *kind != "" && !strings.EqualFold(e.Kind, *kind) {
			continue
		}
		if *lang != "" && !strings.EqualFold(e.Language, *lang) {
			continue
		}
		out = append(out, e)
		if len(out) >= *limit {
			break
		}
	}
	if *format == "json" {
		return emitJSON(map[string]any{"query": needle, "count": len(out), "symbols": out})
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## symbols %q — %d hits\n\n", needle, len(out))
	for _, e := range out {
		fmt.Fprintf(&b, "- `%s` %s", e.Kind, e.Name)
		if e.Signature != "" {
			fmt.Fprintf(&b, "`%s`", e.Signature)
		}
		if e.Scope != "" {
			fmt.Fprintf(&b, " (in %s %s)", e.ScopeKind, e.Scope)
		}
		fmt.Fprintf(&b, " — %s:%d\n", e.Path, e.Line)
	}
	fmt.Print(b.String())
	return nil
}

// normalizeTagPath rewrites ctags paths to repo-relative form.
func normalizeTagPath(p, root string) string {
	if filepath.IsAbs(p) {
		if rel, err := filepath.Rel(root, p); err == nil {
			return filepath.ToSlash(rel)
		}
	}
	return strings.TrimPrefix(filepath.ToSlash(p), "./")
}

// ---- arc-idx ast ----------------------------------------------------------

func cmdAst(args []string) error {
	fl := flag.NewFlagSet("ast", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	lang := fl.String("lang", "", "ast-grep language (required)")
	format := fl.String("format", "ai-md", "output format: json | ai-md")
	positional, err := parseInterleaved(fl, args)
	if err != nil {
		return err
	}
	if len(positional) < 1 || *lang == "" {
		return fmt.Errorf("usage: arc-idx ast --lang go '<pattern>' [paths...]")
	}
	pattern := positional[0]
	paths := positional[1:]

	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}
	sgBin, err := exec.LookPath("ast-grep")
	if err != nil {
		if sgBin, err = exec.LookPath("sg"); err != nil {
			return fmt.Errorf("ast-grep is not installed (brew install ast-grep)")
		}
	}
	sgArgs := []string{"run", "--lang", *lang, "--pattern", pattern, "--json=stream"}
	if len(paths) > 0 {
		sgArgs = append(sgArgs, paths...)
	} else {
		sgArgs = append(sgArgs, ".")
	}
	cmd := exec.Command(sgBin, sgArgs...)
	cmd.Dir = repo.Root
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		if ee, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("ast-grep: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return err
	}

	type astMatch struct {
		File  string `json:"file"`
		Line  int    `json:"line"`
		Lines string `json:"text"`
	}
	var matches []astMatch
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		var ev struct {
			File  string `json:"file"`
			Lines string `json:"lines"`
			Range struct {
				Start struct {
					Line int `json:"line"`
				} `json:"start"`
			} `json:"range"`
		}
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		matches = append(matches, astMatch{
			File:  ev.File,
			Line:  ev.Range.Start.Line + 1, // ast-grep lines are 0-based
			Lines: ev.Lines,
		})
	}
	if *format == "json" {
		return emitJSON(map[string]any{"pattern": pattern, "lang": *lang, "count": len(matches), "matches": matches})
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## ast %s %q — %d matches\n\n", *lang, pattern, len(matches))
	for _, mt := range matches {
		fmt.Fprintf(&b, "### %s:%d\n```%s\n%s\n```\n\n", mt.File, mt.Line, *lang, strings.TrimRight(mt.Lines, "\n"))
	}
	fmt.Print(b.String())
	return nil
}

// ---- arc-idx files --------------------------------------------------------

func cmdFiles(args []string) error {
	fl := flag.NewFlagSet("files", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	profile := fl.String("profile", "", "restrict to a config profile")
	format := fl.String("format", "plain", "output format: plain | json")
	positional, err := parseInterleaved(fl, args)
	if err != nil {
		return err
	}
	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}
	m, err := repo.matcher(*profile)
	if err != nil {
		return err
	}
	var patRe = func(string) bool { return true }
	if len(positional) > 0 {
		re, err := globToRegexp("**/*" + positional[0] + "*")
		if err != nil {
			return err
		}
		lower := strings.ToLower(positional[0])
		patRe = func(p string) bool {
			return re.MatchString(p) || strings.Contains(strings.ToLower(p), lower)
		}
	}
	files, err := walkRepo(repo)
	if err != nil {
		return err
	}
	var out []string
	for _, f := range files {
		if m.match(f) && patRe(f) {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	if *format == "json" {
		return emitJSON(map[string]any{"profile": *profile, "count": len(out), "files": out})
	}
	for _, f := range out {
		fmt.Println(f)
	}
	return nil
}

// ---- arc-idx stats --------------------------------------------------------

func cmdStats(args []string) error {
	fl := flag.NewFlagSet("stats", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	profile := fl.String("profile", "", "restrict to a config profile")
	format := fl.String("format", "ai-md", "output format: json | ai-md")
	if err := fl.Parse(args); err != nil {
		return err
	}
	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}
	m, err := repo.matcher(*profile)
	if err != nil {
		return err
	}
	files, err := walkRepo(repo)
	if err != nil {
		return err
	}
	type langStat struct {
		Files int `json:"files"`
		Lines int `json:"lines"`
	}
	stats := map[string]*langStat{}
	total := langStat{}
	for _, rel := range files {
		if !m.match(rel) {
			continue
		}
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == "" {
			ext = "(none)"
		}
		content, err := os.ReadFile(filepath.Join(repo.Root, rel))
		if err != nil {
			continue
		}
		lines := strings.Count(string(content), "\n") + 1
		s, ok := stats[ext]
		if !ok {
			s = &langStat{}
			stats[ext] = s
		}
		s.Files++
		s.Lines += lines
		total.Files++
		total.Lines += lines
	}
	if *format == "json" {
		return emitJSON(map[string]any{"profile": *profile, "total": total, "by_extension": stats})
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return stats[keys[i]].Lines > stats[keys[j]].Lines })
	var b strings.Builder
	fmt.Fprintf(&b, "## stats (profile: %s)\n\n| ext | files | lines |\n|---|---|---|\n", orDefault(*profile, "all"))
	for _, k := range keys {
		fmt.Fprintf(&b, "| %s | %d | %d |\n", k, stats[k].Files, stats[k].Lines)
	}
	fmt.Fprintf(&b, "| **total** | **%d** | **%d** |\n", total.Files, total.Lines)
	fmt.Print(b.String())
	return nil
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
