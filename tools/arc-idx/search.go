package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os/exec"
	"regexp/syntax"
	"strings"
	"time"

	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/query"
	zsearch "github.com/sourcegraph/zoekt/search"
)

// searchResult is the stable JSON shape emitted by `arc-idx search`.
type searchResult struct {
	Query      string       `json:"query"`
	Profile    string       `json:"profile,omitempty"`
	Engine     string       `json:"engine"` // "zoekt" or "rg-fallback"
	FileCount  int          `json:"file_count"`
	MatchCount int          `json:"match_count"`
	Duration   string       `json:"duration"`
	Files      []fileResult `json:"files"`
}

type fileResult struct {
	Path     string      `json:"path"`
	Language string      `json:"language,omitempty"`
	Score    float64     `json:"score,omitempty"`
	Matches  []lineMatch `json:"matches"`
}

type lineMatch struct {
	Line   int      `json:"line"`
	Text   string   `json:"text"`
	Before []string `json:"before,omitempty"`
	After  []string `json:"after,omitempty"`
}

func cmdSearch(args []string) error {
	fl := flag.NewFlagSet("search", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	profile := fl.String("profile", "", "restrict results to a config profile")
	format := fl.String("format", "ai-md", "output format: json | ai-md")
	maxDocs := fl.Int("max", 50, "maximum files to return")
	contextLines := fl.Int("context", 0, "context lines around each match")
	noIndex := fl.Bool("no-index", false, "skip zoekt and use ripgrep directly")
	positional, err := parseInterleaved(fl, args)
	if err != nil {
		return err
	}
	if len(positional) < 1 {
		return fmt.Errorf("usage: arc-idx search <zoekt-query> [--profile P] [--format json|ai-md]")
	}
	qstr := strings.Join(positional, " ")
	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}
	m, err := repo.matcher(*profile)
	if err != nil {
		return err
	}

	start := time.Now()
	var res *searchResult
	if !*noIndex && hasShards(repo) {
		res, err = zoektSearch(repo, qstr, m, *maxDocs, *contextLines)
	} else {
		res, err = rgSearch(repo, qstr, m, *maxDocs, *contextLines)
	}
	if err != nil {
		return err
	}
	res.Query = qstr
	res.Profile = *profile
	res.Duration = time.Since(start).Round(time.Millisecond).String()
	return emitSearch(res, *format)
}

func zoektSearch(repo *Repo, qstr string, m *profileMatcher, maxDocs, contextLines int) (*searchResult, error) {
	q, err := query.Parse(qstr)
	if err != nil {
		return nil, fmt.Errorf("query parse: %w", err)
	}
	q = query.Simplify(profileFileQuery(m, q))

	searcher, err := zsearch.NewDirectorySearcher(repo.IndexDir)
	if err != nil {
		return nil, fmt.Errorf("open index %s: %w (run `arc-idx index`)", repo.IndexDir, err)
	}
	defer searcher.Close()

	opts := &zoekt.SearchOptions{
		MaxDocDisplayCount: maxDocs * 4, // headroom for post-hoc profile filtering
		NumContextLines:    contextLines,
		ShardMaxMatchCount: 100_000,
		TotalMaxMatchCount: 200_000,
		MaxWallTime:        10 * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	sr, err := searcher.Search(ctx, q, opts)
	if err != nil {
		return nil, err
	}

	res := &searchResult{Engine: "zoekt"}
	for _, fm := range sr.Files {
		if !m.match(fm.FileName) {
			continue
		}
		fr := fileResult{Path: fm.FileName, Language: fm.Language, Score: fm.Score}
		for _, lm := range fm.LineMatches {
			if lm.FileName {
				continue
			}
			fr.Matches = append(fr.Matches, lineMatch{
				Line:   lm.LineNumber,
				Text:   truncateLine(strings.TrimRight(string(lm.Line), "\n")),
				Before: splitContext(lm.Before),
				After:  splitContext(lm.After),
			})
		}
		if len(fr.Matches) == 0 && len(fm.LineMatches) > 0 {
			// filename-only match: still report the file
			fr.Matches = []lineMatch{}
		}
		res.MatchCount += len(fr.Matches)
		res.Files = append(res.Files, fr)
		if len(res.Files) >= maxDocs {
			break
		}
	}
	res.FileCount = len(res.Files)
	return res, nil
}

func splitContext(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	for i, l := range lines {
		lines[i] = truncateLine(l)
	}
	return lines
}

// truncateLine keeps AI context clean: minified or generated single-line
// blobs are cut at 300 runes with an ellipsis marker.
func truncateLine(s string) string {
	const max = 300
	if len(s) <= max {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + " …(truncated)"
}

// rgSearch is the index-free fallback: ripgrep with JSON events.
// Only the pattern portion of the query is used; zoekt atoms like f:/lang:
// are translated where possible.
func rgSearch(repo *Repo, qstr string, m *profileMatcher, maxDocs, contextLines int) (*searchResult, error) {
	rgBin, err := exec.LookPath("rg")
	if err != nil {
		return nil, fmt.Errorf("no zoekt index and rg not installed; run `arc-idx index`")
	}
	pattern, rgArgs := translateQueryToRg(qstr)
	rgArgs = append(rgArgs, "--json", "--max-count", "20")
	if contextLines > 0 {
		rgArgs = append(rgArgs, "-C", fmt.Sprint(contextLines))
	}
	for _, d := range repo.Cfg.IgnoreDirs {
		rgArgs = append(rgArgs, "--glob", "!"+d)
	}
	rgArgs = append(rgArgs, "--", pattern, ".")
	cmd := exec.Command(rgBin, rgArgs...)
	cmd.Dir = repo.Root
	out, _ := cmd.Output() // rg exits 1 on zero matches; that is not an error

	res := &searchResult{Engine: "rg-fallback"}
	byFile := map[string]*fileResult{}
	var order []string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		var ev struct {
			Type string `json:"type"`
			Data struct {
				Path struct {
					Text string `json:"text"`
				} `json:"path"`
				Lines struct {
					Text string `json:"text"`
				} `json:"lines"`
				LineNumber int `json:"line_number"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(line), &ev) != nil || ev.Type != "match" {
			continue
		}
		rel := strings.TrimPrefix(ev.Data.Path.Text, "./")
		if !m.match(rel) {
			continue
		}
		fr, ok := byFile[rel]
		if !ok {
			if len(order) >= maxDocs {
				continue
			}
			fr = &fileResult{Path: rel}
			byFile[rel] = fr
			order = append(order, rel)
		}
		fr.Matches = append(fr.Matches, lineMatch{
			Line: ev.Data.LineNumber,
			Text: truncateLine(strings.TrimRight(ev.Data.Lines.Text, "\n")),
		})
		res.MatchCount++
	}
	for _, p := range order {
		res.Files = append(res.Files, *byFile[p])
	}
	res.FileCount = len(res.Files)
	return res, nil
}

// translateQueryToRg strips zoekt atoms into rg flags where a direct
// equivalent exists and returns the residual pattern.
func translateQueryToRg(qstr string) (string, []string) {
	var terms, rgArgs []string
	caseSensitive := false
	for _, tok := range strings.Fields(qstr) {
		switch {
		case strings.HasPrefix(tok, "f:"):
			rgArgs = append(rgArgs, "--glob", "*"+strings.Trim(strings.TrimPrefix(tok, "f:"), `\^$`)+"*")
		case strings.HasPrefix(tok, "lang:"):
			rgArgs = append(rgArgs, "--type", strings.TrimPrefix(tok, "lang:"))
		case tok == "case:yes":
			caseSensitive = true
		case strings.HasPrefix(tok, "sym:"):
			terms = append(terms, strings.TrimPrefix(tok, "sym:"))
		default:
			terms = append(terms, tok)
		}
	}
	if !caseSensitive {
		rgArgs = append(rgArgs, "-i")
	}
	return strings.Join(terms, ".*"), rgArgs
}

// profileFileQuery builds a zoekt query restriction from profile globs so
// filtering happens inside the engine as well as post-hoc.
func profileFileQuery(m *profileMatcher, base query.Q) query.Q {
	qs := []query.Q{base}
	for _, re := range m.include {
		if sre, err := syntax.Parse(re.String(), syntax.Perl); err == nil {
			qs = append(qs, &query.Regexp{Regexp: sre, FileName: true})
		}
	}
	for _, re := range m.exclude {
		if sre, err := syntax.Parse(re.String(), syntax.Perl); err == nil {
			qs = append(qs, &query.Not{Child: &query.Regexp{Regexp: sre, FileName: true}})
		}
	}
	if len(qs) == 1 {
		return base
	}
	return query.NewAnd(qs...)
}
