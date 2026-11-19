package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type WorldFlagCandidate struct {
	Flag   string `json:"flag" yaml:"flag"`
	Source string `json:"source" yaml:"source"`
	File   string `json:"file" yaml:"file"`
	Line   int    `json:"line" yaml:"line"`
	Kind   string `json:"kind" yaml:"kind"` // sql_status | sql_amount | sql_fk | orm_state | async_topic
}

type WorldScanResult struct {
	FocalEntity    string               `json:"focal_entity" yaml:"focal_entity"`
	RepoRoot       string               `json:"repo_root" yaml:"repo_root"`
	DerivedTokens  []string             `json:"derived_tokens" yaml:"derived_tokens"`
	Candidates     []WorldFlagCandidate `json:"candidates" yaml:"candidates"`
	SuggestedFlags map[string]string    `json:"world_flags" yaml:"world_flags"`
}

var (
	wsIgnoreDirs = map[string]bool{
		".git": true, "node_modules": true, "dist": true, "build": true,
		"vendor": true, ".arc": true, ".venv": true, ".magi": true,
		".ai-code-index": true, "__pycache__": true, "target": true,
	}
	reWSCreateTable = regexp.MustCompile(`(?i)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?["` + "`" + `]?([a-zA-Z_][a-zA-Z0-9_.]*)["` + "`" + `]?`)
	reWSReferences  = regexp.MustCompile(`(?i)REFERENCES\s+["` + "`" + `]?([a-zA-Z_][a-zA-Z0-9_]*)["` + "`" + `]?`)
	reWSColLine     = regexp.MustCompile(`(?i)^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s+([a-zA-Z0-9_(),\s]+)`)
	reWSStatus      = regexp.MustCompile(`(?i)(status|state|phase|stage|frozen|locked|pending|active|deleted|cancelled|canceled|dispute|arbitration)`)
	reWSAmount      = regexp.MustCompile(`(?i)(balance|amount|cents|pending|in_transit|frozen|escrow|refund|settlement|credit|debit)`)
	reWSStruct      = regexp.MustCompile(`^\s*(?:export\s+)?(?:type\s+([A-Z][a-zA-Z0-9_]*)\s+struct|(?:class|model)\s+([A-Z][a-zA-Z0-9_]*))`)
	reWSTopic       = regexp.MustCompile(`(?i)(Publish|Subscribe|Consume|Emit|Produce|SendEvent|Topic|Queue)\w*\s*\([^)]*["` + "`" + `']([a-zA-Z0-9_.:/-]+)["` + "`" + `']`)
)

// scanWorldFlags scans SQL schemas/migrations, ORM models, and async topics in repoRoot
// to extract every coupled entity referencing focalEntity (e.g. "users.id").
func scanWorldFlags(repoRoot, focalEntity string) (*WorldScanResult, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	tableLower, fkColLower, camelLower, tokens := deriveWorldTokens(focalEntity)
	singular := singularizeWord(tableLower)

	res := &WorldScanResult{
		FocalEntity:    focalEntity,
		RepoRoot:       absRoot,
		DerivedTokens:  tokens,
		SuggestedFlags: map[string]string{},
	}
	seen := map[string]bool{}
	add := func(c WorldFlagCandidate) {
		if seen[c.Flag] {
			return
		}
		seen[c.Flag] = true
		res.Candidates = append(res.Candidates, c)
		res.SuggestedFlags[c.Flag] = c.Source
	}

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return nil
		}
		if d.IsDir() {
			if path != absRoot && (wsIgnoreDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return fs.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(absRoot, path)
		if rerr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		ext := strings.ToLower(filepath.Ext(rel))
		if isTestOrDocPath(rel, ext) {
			return nil
		}
		switch ext {
		case ".sql", ".go", ".ts", ".tsx", ".py", ".java", ".kt", ".prisma":
		default:
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) > 2<<20 {
			return nil
		}
		text := string(raw)
		if ext == ".sql" {
			for _, c := range extractSQLWorldFlags(rel, text, tableLower, fkColLower) {
				add(c)
			}
		} else {
			for _, c := range extractCodeWorldFlags(rel, text, tableLower, singular, fkColLower, camelLower) {
				add(c)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(res.Candidates, func(i, j int) bool {
		return res.Candidates[i].Flag < res.Candidates[j].Flag
	})
	return res, nil
}

func deriveWorldTokens(target string) (tableLower, fkColLower, camelLower string, tokens []string) {
	clean := strings.TrimSpace(target)
	parts := strings.Split(clean, ".")
	base := strings.ToLower(parts[0])
	singular := singularizeWord(base)
	if len(parts) > 1 {
		tableLower = base
		fkColLower = singular + "_" + strings.ToLower(parts[1])
	} else if strings.HasSuffix(base, "_id") {
		fkColLower = base
		singular = strings.TrimSuffix(base, "_id")
		tableLower = singular + "s"
	} else {
		tableLower = base
		fkColLower = singular + "_id"
	}
	camelLower = strings.ToLower(singular + "id")
	seen := map[string]bool{}
	for _, t := range []string{clean, tableLower, singular, fkColLower, camelLower} {
		if t != "" && !seen[t] {
			seen[t] = true
			tokens = append(tokens, t)
		}
	}
	return
}

func singularizeWord(word string) string {
	switch {
	case strings.HasSuffix(word, "ies") && len(word) > 3:
		return word[:len(word)-3] + "y"
	case strings.HasSuffix(word, "ses") && len(word) > 3:
		return word[:len(word)-2]
	case strings.HasSuffix(word, "s") && len(word) > 2 && !strings.HasSuffix(word, "ss"):
		return word[:len(word)-1]
	default:
		return word
	}
}

func isTestOrDocPath(rel, ext string) bool {
	if ext == ".md" || ext == ".txt" {
		return true
	}
	low := strings.ToLower(rel)
	base := filepath.Base(low)
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".spec.ts") ||
		strings.HasPrefix(base, "test_") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.Contains(low, "/tests/") ||
		strings.HasPrefix(low, "tests/")
}

func extractSQLWorldFlags(rel, text, tableLower, fkColLower string) []WorldFlagCandidate {
	lines := strings.Split(text, "\n")
	var out []WorldFlagCandidate
	for i := 0; i < len(lines); i++ {
		m := reWSCreateTable.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		tbl := strings.Trim(m[1], "\"`")
		end := i
		for j := i; j < len(lines); j++ {
			end = j
			if strings.Contains(lines[j], ";") {
				break
			}
		}
		matched := strings.EqualFold(tbl, tableLower)
		var statuses, amounts []string
		for j := i + 1; j <= end; j++ {
			l := strings.TrimSpace(lines[j])
			if l == "" || strings.HasPrefix(l, "--") || strings.HasPrefix(l, ")") {
				continue
			}
			if ref := reWSReferences.FindStringSubmatch(l); ref != nil && strings.EqualFold(ref[1], tableLower) {
				matched = true
			}
			cm := reWSColLine.FindStringSubmatch(l)
			if cm == nil {
				continue
			}
			col := strings.ToLower(cm[1])
			switch col {
			case "primary", "foreign", "constraint", "unique", "index", "key", "check":
				continue
			}
			if col == fkColLower || strings.HasSuffix(col, "_"+fkColLower) {
				matched = true
			}
			if reWSStatus.MatchString(col) {
				statuses = append(statuses, cm[1])
			} else if reWSAmount.MatchString(col) {
				amounts = append(amounts, cm[1])
			}
		}
		if matched {
			for _, col := range statuses {
				out = append(out, WorldFlagCandidate{
					Flag: tbl + "_" + col, Source: tbl + "." + col, File: rel, Line: i + 1, Kind: "sql_status",
				})
			}
			for _, col := range amounts {
				out = append(out, WorldFlagCandidate{
					Flag: tbl + "_" + col, Source: tbl + "." + col, File: rel, Line: i + 1, Kind: "sql_amount",
				})
			}
			if len(statuses) == 0 && len(amounts) == 0 {
				out = append(out, WorldFlagCandidate{
					Flag: tbl + "_active_count", Source: tbl + ".count", File: rel, Line: i + 1, Kind: "sql_fk",
				})
			}
		}
		i = end
	}
	return out
}

func extractCodeWorldFlags(rel, text, tableLower, singular, fkColLower, camelLower string) []WorldFlagCandidate {
	lines := strings.Split(text, "\n")
	var out []WorldFlagCandidate
	for i := 0; i < len(lines); i++ {
		if m := reWSTopic.FindStringSubmatch(lines[i]); m != nil {
			topic := m[2]
			low := strings.ToLower(lines[i])
			tLow := strings.ToLower(topic)
			if strings.Contains(tLow, singular) || strings.Contains(tLow, tableLower) ||
				strings.Contains(low, fkColLower) || strings.Contains(low, camelLower) {
				flagName := "async_" + sanitizeNS(topic)
				out = append(out, WorldFlagCandidate{
					Flag: flagName, Source: "topic:" + topic, File: rel, Line: i + 1, Kind: "async_topic",
				})
			}
		}
		sm := reWSStruct.FindStringSubmatch(lines[i])
		if sm == nil {
			continue
		}
		model := sm[1]
		if model == "" {
			model = sm[2]
		}
		end := findEndBrace(lines, i)
		hasRef := false
		var stateFields []string
		for j := i + 1; j <= end; j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
				continue
			}
			low := strings.ToLower(trimmed)
			if strings.Contains(low, fkColLower) || strings.Contains(low, camelLower) ||
				(strings.Contains(low, "references") && strings.Contains(low, tableLower)) {
				hasRef = true
			}
			if reWSStatus.MatchString(low) || reWSAmount.MatchString(low) {
				f := strings.Fields(trimmed)
				if len(f) > 0 {
					fieldName := strings.TrimRight(f[0], ":;,?")
					if fieldName != "" && !strings.HasPrefix(fieldName, "//") {
						stateFields = append(stateFields, fieldName)
					}
				}
			}
		}
		if hasRef {
			for _, f := range stateFields {
				out = append(out, WorldFlagCandidate{
					Flag:   strings.ToLower(model) + "_" + strings.ToLower(f),
					Source: model + "." + f,
					File:   rel,
					Line:   i + 1,
					Kind:   "orm_state",
				})
			}
		}
		i = end
	}
	return out
}

func findEndBrace(lines []string, start int) int {
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
	}
	if start+30 < len(lines) {
		return start + 30
	}
	return len(lines) - 1
}

func renderWorldScanMarkdown(r *WorldScanResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## world-scan `%s` (%d candidate world_flags)\n\n", r.FocalEntity, len(r.Candidates))
	for _, c := range r.Candidates {
		fmt.Fprintf(&b, "- `%s`: `%s` (%s:%d, %s)\n", c.Flag, c.Source, c.File, c.Line, c.Kind)
	}
	return b.String()
}
