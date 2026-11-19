package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ImpactReport captures cross-layer reverse couplings for a target entity/column/symbol.
type ImpactReport struct {
	Target         string          `json:"target"`
	DerivedTokens  []string        `json:"derived_tokens"`
	SQLTables      []SQLCoupling   `json:"sql_tables"`
	ORMModels      []ORMCoupling   `json:"orm_models"`
	Routes         []RouteHit      `json:"routes"`
	AsyncTopics    []TopicHit      `json:"async_topics"`
	ProdReferences []CodeRefHit    `json:"prod_references"`
	WorldFlagHints []WorldFlagHint `json:"world_flag_hints"`
}

type SQLCoupling struct {
	Table        string   `json:"table"`
	File         string   `json:"file"`
	Line         int      `json:"line"`
	FKColumns    []string `json:"fk_columns"`
	StatusFields []string `json:"status_fields,omitempty"`
	AmountFields []string `json:"amount_fields,omitempty"`
}

type ORMCoupling struct {
	Model        string   `json:"model"`
	File         string   `json:"file"`
	Line         int      `json:"line"`
	RefFields    []string `json:"ref_fields"`
	StatusFields []string `json:"status_fields,omitempty"`
}

type RouteHit struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

type TopicHit struct {
	Topic string `json:"topic"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Text  string `json:"text"`
}

type CodeRefHit struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Layer string `json:"layer"`
	Text  string `json:"text"`
}

type WorldFlagHint struct {
	Flag   string `json:"flag"`
	Source string `json:"source"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Reason string `json:"reason"`
}

func cmdImpact(args []string) error {
	fl := flag.NewFlagSet("impact", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	prodOnly := fl.Bool("prod-only", true, "exclude test and doc files from code references")
	formatMode := fl.String("format", "ai-md", "output format: json | ai-md")
	positional, err := parseInterleaved(fl, args)
	if err != nil {
		return err
	}
	if len(positional) < 1 {
		return fmt.Errorf("usage: arc idx impact <entity-or-table.col> [--prod-only] [--format json|ai-md]")
	}
	target := positional[0]
	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}
	report, err := scanImpact(repo, target, *prodOnly)
	if err != nil {
		return err
	}
	if *formatMode == "json" {
		return emitJSON(report)
	}
	fmt.Print(renderImpactMarkdown(report))
	return nil
}

func deriveEntityTokens(target string) (tableName, fkCol string, tokens []string) {
	clean := strings.TrimSpace(target)
	parts := strings.Split(clean, ".")
	base := parts[0]
	tableName = strings.ToLower(base)
	singular := singularize(tableName)
	if len(parts) > 1 {
		fkCol = singular + "_" + strings.ToLower(parts[1])
	} else if strings.HasSuffix(tableName, "_id") {
		fkCol = tableName
		singular = strings.TrimSuffix(tableName, "_id")
		tableName = singular + "s"
	} else {
		fkCol = singular + "_id"
	}
	camel := toPascal(singular) + "ID"
	camelLower := singular + "Id"
	seen := map[string]bool{}
	for _, tok := range []string{clean, tableName, singular, fkCol, camel, camelLower} {
		if tok != "" && !seen[tok] {
			seen[tok] = true
			tokens = append(tokens, tok)
		}
	}
	return tableName, fkCol, tokens
}

func singularize(word string) string {
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

func toPascal(snake string) string {
	var b strings.Builder
	for _, p := range strings.Split(snake, "_") {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

var (
	reSQLColumnLine = regexp.MustCompile(`(?i)^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s+([a-zA-Z0-9_(),\s]+)`)
	reSQLReferences = regexp.MustCompile(`(?i)REFERENCES\s+["` + "`" + `]?([a-zA-Z_][a-zA-Z0-9_]*)["` + "`" + `]?`)
	reStatusName    = regexp.MustCompile(`(?i)(status|state|phase|stage|frozen|locked|pending|active|deleted|cancelled|canceled|dispute|arbitration)`)
	reAmountName    = regexp.MustCompile(`(?i)(balance|amount|cents|pending|in_transit|frozen|escrow|refund|settlement|credit|debit)`)
	reGoStruct      = regexp.MustCompile(`^\s*type\s+([A-Z][a-zA-Z0-9_]*)\s+struct\s*\{`)
	reClassModel    = regexp.MustCompile(`^\s*(?:export\s+)?(?:class|model|interface)\s+([A-Z][a-zA-Z0-9_]*)`)
	reRouteCall     = regexp.MustCompile(`(?i)\.(GET|POST|PUT|PATCH|DELETE|Handle|HandleFunc|Any|Route)\s*\(\s*["` + "`" + `']([^"` + "`" + `']+)["` + "`" + `']`)
	reTopicCall     = regexp.MustCompile(`(?i)(Publish|Subscribe|Consume|Emit|Produce|SendEvent|Topic|Queue)\w*\s*\([^)]*["` + "`" + `']([a-zA-Z0-9_.:/-]+)["` + "`" + `']`)
)

func scanImpact(repo *Repo, target string, prodOnly bool) (*ImpactReport, error) {
	files, err := walkRepo(repo)
	if err != nil {
		return nil, err
	}
	tableName, fkCol, tokens := deriveEntityTokens(target)
	report := &ImpactReport{
		Target:        target,
		DerivedTokens: tokens,
	}

	fkColLower := strings.ToLower(fkCol)
	tableLower := strings.ToLower(tableName)
	singular := singularize(tableLower)
	camelLower := strings.ToLower(toPascal(singular) + "id")

	for _, rel := range files {
		layer := pathLayer(rel)
		if prodOnly && (layer == "doc" || layer == "test") {
			continue
		}
		abs := filepath.Join(repo.Root, rel)
		raw, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		text := string(raw)
		ext := strings.ToLower(filepath.Ext(rel))

		if ext == ".sql" {
			report.SQLTables = append(report.SQLTables, scanSQLFile(rel, text, tableLower, fkColLower)...)
		}
		if ext == ".go" || ext == ".ts" || ext == ".tsx" || ext == ".py" || ext == ".prisma" || ext == ".java" || ext == ".kt" {
			report.ORMModels = append(report.ORMModels, scanORMFile(rel, text, fkColLower, camelLower, tableLower)...)
		}

		lines := strings.Split(text, "\n")
		for idx, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			low := strings.ToLower(trimmed)
			if m := reRouteCall.FindStringSubmatch(trimmed); m != nil {
				if strings.Contains(low, singular) || strings.Contains(low, tableLower) || strings.Contains(low, strings.ToLower(target)) {
					report.Routes = append(report.Routes, RouteHit{
						File: rel, Line: idx + 1, Text: truncateLine(trimmed),
					})
				}
			}
			if m := reTopicCall.FindStringSubmatch(trimmed); m != nil {
				topic := m[2]
				topicLow := strings.ToLower(topic)
				if strings.Contains(topicLow, singular) || strings.Contains(topicLow, tableLower) || strings.Contains(low, fkColLower) || strings.Contains(low, camelLower) {
					report.AsyncTopics = append(report.AsyncTopics, TopicHit{
						Topic: topic, File: rel, Line: idx + 1, Text: truncateLine(trimmed),
					})
				}
			}
			if layer == "prod" && (strings.Contains(trimmed, target) || strings.Contains(low, fkColLower) || strings.Contains(low, camelLower)) {
				if len(report.ProdReferences) < 80 {
					report.ProdReferences = append(report.ProdReferences, CodeRefHit{
						File: rel, Line: idx + 1, Layer: layer, Text: truncateLine(trimmed),
					})
				}
			}
		}
	}

	report.WorldFlagHints = buildWorldFlagHints(report)
	return report, nil
}

func scanSQLFile(rel, text, tableLower, fkColLower string) []SQLCoupling {
	lines := strings.Split(text, "\n")
	var hits []SQLCoupling
	for i := 0; i < len(lines); i++ {
		m := reSQLTable.FindStringSubmatch(lines[i])
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
		var fkCols, statusCols, amountCols []string
		matched := strings.EqualFold(tbl, tableLower)
		for j := i + 1; j <= end; j++ {
			l := strings.TrimSpace(lines[j])
			if l == "" || strings.HasPrefix(l, "--") || strings.HasPrefix(l, ")") {
				continue
			}
			low := strings.ToLower(l)
			if ref := reSQLReferences.FindStringSubmatch(l); ref != nil && strings.EqualFold(ref[1], tableLower) {
				matched = true
				if cm := reSQLColumnLine.FindStringSubmatch(l); cm != nil {
					fkCols = appendUnique(fkCols, cm[1])
				} else {
					fkCols = appendUnique(fkCols, "FOREIGN_KEY("+tableLower+")")
				}
			}
			if cm := reSQLColumnLine.FindStringSubmatch(l); cm != nil {
				col := cm[1]
				colLow := strings.ToLower(col)
				if isSQLKeyword(colLow) {
					continue
				}
				if colLow == fkColLower || strings.HasSuffix(colLow, "_"+fkColLower) {
					matched = true
					fkCols = appendUnique(fkCols, col)
				}
				if reStatusName.MatchString(colLow) {
					statusCols = appendUnique(statusCols, col)
				}
				if reAmountName.MatchString(colLow) {
					amountCols = appendUnique(amountCols, col)
				}
			} else if strings.Contains(low, fkColLower) {
				matched = true
			}
		}
		if matched {
			hits = append(hits, SQLCoupling{
				Table:        tbl,
				File:         rel,
				Line:         i + 1,
				FKColumns:    fkCols,
				StatusFields: statusCols,
				AmountFields: amountCols,
			})
		}
		i = end
	}
	return hits
}

func isSQLKeyword(w string) bool {
	switch w {
	case "primary", "foreign", "constraint", "unique", "index", "key", "check", "create", "alter", "drop":
		return true
	default:
		return false
	}
}

func scanORMFile(rel, text, fkColLower, camelLower, tableLower string) []ORMCoupling {
	lines := strings.Split(text, "\n")
	var hits []ORMCoupling
	for i := 0; i < len(lines); i++ {
		var model string
		if m := reGoStruct.FindStringSubmatch(lines[i]); m != nil {
			model = m[1]
		} else if m := reClassModel.FindStringSubmatch(lines[i]); m != nil {
			model = m[1]
		} else {
			continue
		}
		end := findBraceEnd(lines, i)
		if end == i {
			end = min(len(lines)-1, i+40)
		}
		var refs, statuses []string
		for j := i + 1; j <= end; j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
				continue
			}
			low := strings.ToLower(trimmed)
			if strings.Contains(low, fkColLower) || strings.Contains(low, camelLower) ||
				(strings.Contains(low, "references") && strings.Contains(low, tableLower)) {
				fieldName := firstIdentifier(trimmed)
				if fieldName != "" {
					refs = appendUnique(refs, fieldName)
				}
			}
			if reStatusName.MatchString(low) || reAmountName.MatchString(low) {
				fieldName := firstIdentifier(trimmed)
				if fieldName != "" {
					statuses = appendUnique(statuses, fieldName)
				}
			}
		}
		if len(refs) > 0 {
			hits = append(hits, ORMCoupling{
				Model:        model,
				File:         rel,
				Line:         i + 1,
				RefFields:    refs,
				StatusFields: statuses,
			})
		}
		i = end
	}
	return hits
}

func firstIdentifier(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	tok := strings.TrimRight(fields[0], ":;,?")
	if tok == "var" || tok == "const" || tok == "readonly" || tok == "public" || tok == "private" {
		if len(fields) > 1 {
			tok = strings.TrimRight(fields[1], ":;,?")
		}
	}
	return tok
}

func buildWorldFlagHints(r *ImpactReport) []WorldFlagHint {
	var hints []WorldFlagHint
	seen := map[string]bool{}
	add := func(flag, source, file string, line int, reason string) {
		if seen[flag] {
			return
		}
		seen[flag] = true
		hints = append(hints, WorldFlagHint{
			Flag: flag, Source: source, File: file, Line: line, Reason: reason,
		})
	}
	for _, t := range r.SQLTables {
		for _, col := range t.StatusFields {
			add(t.Table+"_"+col, t.Table+"."+col, t.File, t.Line, "sql_status_column")
		}
		for _, col := range t.AmountFields {
			add(t.Table+"_"+col, t.Table+"."+col, t.File, t.Line, "sql_amount_column")
		}
		if len(t.StatusFields) == 0 && len(t.AmountFields) == 0 && len(t.FKColumns) > 0 {
			add(t.Table+"_active_count", t.Table+".*", t.File, t.Line, "sql_fk_reference")
		}
	}
	for _, m := range r.ORMModels {
		prefix := strings.ToLower(m.Model)
		for _, f := range m.StatusFields {
			add(prefix+"_"+strings.ToLower(f), m.Model+"."+f, m.File, m.Line, "orm_state_field")
		}
	}
	for _, topic := range r.AsyncTopics {
		key := "async_" + strings.NewReplacer(".", "_", "/", "_", "-", "_", ":", "_").Replace(strings.ToLower(topic.Topic))
		add(key, "topic:"+topic.Topic, topic.File, topic.Line, "async_topic_coupling")
	}
	sort.Slice(hints, func(i, j int) bool { return hints[i].Flag < hints[j].Flag })
	return hints
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func renderImpactMarkdown(r *ImpactReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## impact `%s`\n\n", r.Target)
	fmt.Fprintf(&b, "Tokens: `%s`\n\n", strings.Join(r.DerivedTokens, "`, `"))

	fmt.Fprintf(&b, "### SQL Tables (%d)\n", len(r.SQLTables))
	for _, t := range r.SQLTables {
		fmt.Fprintf(&b, "- `%s` (%s:%d) fk=[%s] status=[%s] amount=[%s]\n",
			t.Table, t.File, t.Line,
			strings.Join(t.FKColumns, ","),
			strings.Join(t.StatusFields, ","),
			strings.Join(t.AmountFields, ","))
	}
	fmt.Fprintf(&b, "\n### ORM Models (%d)\n", len(r.ORMModels))
	for _, m := range r.ORMModels {
		fmt.Fprintf(&b, "- `%s` (%s:%d) refs=[%s] state=[%s]\n",
			m.Model, m.File, m.Line,
			strings.Join(m.RefFields, ","),
			strings.Join(m.StatusFields, ","))
	}
	fmt.Fprintf(&b, "\n### Routes (%d) & Async Topics (%d)\n", len(r.Routes), len(r.AsyncTopics))
	for _, rt := range r.Routes {
		fmt.Fprintf(&b, "- route %s:%d `%s`\n", rt.File, rt.Line, rt.Text)
	}
	for _, tp := range r.AsyncTopics {
		fmt.Fprintf(&b, "- topic `%s` %s:%d `%s`\n", tp.Topic, tp.File, tp.Line, tp.Text)
	}
	fmt.Fprintf(&b, "\n### Candidate `world_flags` (%d)\n", len(r.WorldFlagHints))
	for _, h := range r.WorldFlagHints {
		fmt.Fprintf(&b, "- `%s`: `%s` (%s:%d — %s)\n", h.Flag, h.Source, h.File, h.Line, h.Reason)
	}
	return b.String()
}
