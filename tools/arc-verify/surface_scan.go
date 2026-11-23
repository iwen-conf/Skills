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

type SurfaceEntry struct {
	Category string `json:"category" yaml:"category"` // http_api | grpc_rpc | cli_cmd | ui_route | data_entity | async_worker
	Method   string `json:"method,omitempty" yaml:"method,omitempty"`
	Name     string `json:"name" yaml:"name"`
	File     string `json:"file" yaml:"file"`
	Line     int    `json:"line" yaml:"line"`
}

type SurfaceScanSummary struct {
	HTTPAPIs      int `json:"http_apis" yaml:"http_apis"`
	GRPCRPCs      int `json:"grpc_rpcs" yaml:"grpc_rpcs"`
	CLICommands   int `json:"cli_commands" yaml:"cli_commands"`
	UIRoutes      int `json:"ui_routes" yaml:"ui_routes"`
	DataEntities  int `json:"data_entities" yaml:"data_entities"`
	AsyncWorkers  int `json:"async_workers" yaml:"async_workers"`
	ExistingTests int `json:"existing_tests" yaml:"existing_tests"`
}

type SurfaceScanResult struct {
	RepoRoot     string             `json:"repo_root" yaml:"repo_root"`
	Summary      SurfaceScanSummary `json:"summary" yaml:"summary"`
	Entries      []SurfaceEntry     `json:"entries" yaml:"entries"`
	StoryFiles   []string           `json:"story_files,omitempty" yaml:"story_files,omitempty"`
	TestFileSample []string         `json:"test_file_sample,omitempty" yaml:"test_file_sample,omitempty"`
}

var (
	// HTTP routes across Go (gin/echo/chi/net-http), TS/JS (express/fastify), Python (FastAPI/Flask), Java/Kotlin (Spring), Rust (axum/actix)
	reSurfRouterMethod = regexp.MustCompile(`(?i)\.\s*(GET|POST|PUT|PATCH|DELETE|HandleFunc|Handle)\s*\(\s*["` + "`" + `'](/[^"` + "`" + `']*)["` + "`" + `']`)
	reSurfGoMuxPattern = regexp.MustCompile(`\b(?:HandleFunc|Handle)\s*\(\s*["` + "`" + `']((?:GET|POST|PUT|PATCH|DELETE)\s+/[^"` + "`" + `']+|/[^"` + "`" + `']+)["` + "`" + `']`)
	reSurfPyDecorator  = regexp.MustCompile(`@\w+\.(get|post|put|patch|delete|route)\s*\(\s*["'](/[^"']*)["']`)
	reSurfSpringMap    = regexp.MustCompile(`@(Get|Post|Put|Patch|Delete|Request)Mapping\s*\(\s*(?:value\s*=\s*)?["'](/[^"']*)["']`)
	reSurfProtoRPC     = regexp.MustCompile(`^\s*rpc\s+([A-Za-z0-9_]+)\s*\(`)
	reSurfFlagSet      = regexp.MustCompile(`(?:flag\.NewFlagSet|add_parser|Command\{\s*Use:)\s*\(?\s*["']([a-zA-Z0-9_:-]+)["']`)
	reSurfCronOrWorker = regexp.MustCompile(`(?i)(AddFunc|Cron|Schedule|RegisterWorker|Consume|Subscribe|Queue)\w*\s*\(\s*["` + "`" + `']([^"` + "`" + `']+)["` + "`" + `']`)
)

// scanProjectSurface inventories all real entry points, stateful tables/models,
// async workers, and existing tests across repoRoot for Veteran-QA full-business E2E planning.
func scanProjectSurface(repoRoot string) (*SurfaceScanResult, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}
	res := &SurfaceScanResult{RepoRoot: absRoot}
	seen := map[string]bool{}
	add := func(e SurfaceEntry) {
		key := fmt.Sprintf("%s|%s|%s|%s", e.Category, e.Method, e.Name, e.File)
		if seen[key] {
			return
		}
		seen[key] = true
		res.Entries = append(res.Entries, e)
		switch e.Category {
		case "http_api":
			res.Summary.HTTPAPIs++
		case "grpc_rpc":
			res.Summary.GRPCRPCs++
		case "cli_cmd":
			res.Summary.CLICommands++
		case "ui_route":
			res.Summary.UIRoutes++
		case "data_entity":
			res.Summary.DataEntities++
		case "async_worker":
			res.Summary.AsyncWorkers++
		}
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
		lowRel := strings.ToLower(rel)
		ext := strings.ToLower(filepath.Ext(rel))

		if strings.HasSuffix(lowRel, ".story.yaml") || strings.HasSuffix(lowRel, ".story.yml") || strings.HasSuffix(lowRel, ".story.json") {
			res.StoryFiles = append(res.StoryFiles, rel)
			return nil
		}
		if isTestFile(lowRel) {
			res.Summary.ExistingTests++
			if len(res.TestFileSample) < 25 {
				res.TestFileSample = append(res.TestFileSample, rel)
			}
			return nil
		}

		// Detect Next.js / Nuxt / Remix file-based UI routes.
		base := filepath.Base(lowRel)
		if (base == "page.tsx" || base == "page.jsx" || base == "index.vue") &&
			(strings.Contains(lowRel, "app/") || strings.Contains(lowRel, "pages/")) {
			add(SurfaceEntry{Category: "ui_route", Method: "UI", Name: "/" + filepath.Dir(rel), File: rel, Line: 1})
		}

		switch ext {
		case ".go", ".ts", ".tsx", ".js", ".py", ".java", ".kt", ".rs", ".sql", ".proto", ".prisma":
		default:
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) > 2<<20 {
			return nil
		}
		lines := strings.Split(string(raw), "\n")
		for idx, line := range lines {
			ln := idx + 1
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "#") {
				continue
			}

			switch ext {
			case ".sql":
				if m := reWSCreateTable.FindStringSubmatch(line); m != nil {
					add(SurfaceEntry{Category: "data_entity", Method: "SQL", Name: strings.Trim(m[1], "\"`"), File: rel, Line: ln})
				}
			case ".proto":
				if m := reSurfProtoRPC.FindStringSubmatch(line); m != nil {
					add(SurfaceEntry{Category: "grpc_rpc", Method: "RPC", Name: m[1], File: rel, Line: ln})
				}
			default:
				if m := reSurfRouterMethod.FindStringSubmatch(line); m != nil {
					method := strings.ToUpper(m[1])
					if method == "HANDLEFUNC" || method == "HANDLE" {
						method = "ANY"
					}
					add(SurfaceEntry{Category: "http_api", Method: method, Name: m[2], File: rel, Line: ln})
				} else if m := reSurfGoMuxPattern.FindStringSubmatch(line); m != nil {
					parts := strings.Fields(m[1])
					if len(parts) == 2 {
						add(SurfaceEntry{Category: "http_api", Method: parts[0], Name: parts[1], File: rel, Line: ln})
					} else {
						add(SurfaceEntry{Category: "http_api", Method: "ANY", Name: m[1], File: rel, Line: ln})
					}
				} else if m := reSurfPyDecorator.FindStringSubmatch(line); m != nil {
					add(SurfaceEntry{Category: "http_api", Method: strings.ToUpper(m[1]), Name: m[2], File: rel, Line: ln})
				} else if m := reSurfSpringMap.FindStringSubmatch(line); m != nil {
					add(SurfaceEntry{Category: "http_api", Method: strings.ToUpper(m[1]), Name: m[2], File: rel, Line: ln})
				}
				if m := reSurfFlagSet.FindStringSubmatch(line); m != nil {
					add(SurfaceEntry{Category: "cli_cmd", Method: "CLI", Name: m[1], File: rel, Line: ln})
				}
				if m := reSurfCronOrWorker.FindStringSubmatch(line); m != nil {
					add(SurfaceEntry{Category: "async_worker", Method: strings.ToUpper(m[1]), Name: m[2], File: rel, Line: ln})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(res.Entries, func(i, j int) bool {
		if res.Entries[i].Category != res.Entries[j].Category {
			return res.Entries[i].Category < res.Entries[j].Category
		}
		if res.Entries[i].Name != res.Entries[j].Name {
			return res.Entries[i].Name < res.Entries[j].Name
		}
		return res.Entries[i].File < res.Entries[j].File
	})
	sort.Strings(res.StoryFiles)
	sort.Strings(res.TestFileSample)
	return res, nil
}

func isTestFile(lowRel string) bool {
	base := filepath.Base(lowRel)
	return strings.HasSuffix(base, "_test.go") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".test.tsx") ||
		strings.HasSuffix(base, ".spec.ts") ||
		strings.HasSuffix(base, ".spec.tsx") ||
		strings.HasPrefix(base, "test_") ||
		strings.HasSuffix(base, "_test.py")
}

func renderSurfaceScanMarkdown(r *SurfaceScanResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Full-Business Surface Recon (`%s`)\n\n", r.RepoRoot)
	fmt.Fprintf(&b, "- HTTP APIs: **%d** · gRPC RPCs: **%d** · CLI Commands: **%d** · UI Routes: **%d**\n",
		r.Summary.HTTPAPIs, r.Summary.GRPCRPCs, r.Summary.CLICommands, r.Summary.UIRoutes)
	fmt.Fprintf(&b, "- Data Entities (SQL): **%d** · Async Workers/Topics: **%d** · Existing Test Files: **%d** · Storylines: **%d**\n\n",
		r.Summary.DataEntities, r.Summary.AsyncWorkers, r.Summary.ExistingTests, len(r.StoryFiles))
	for _, e := range r.Entries {
		if e.Method != "" {
			fmt.Fprintf(&b, "- [%s] `%s %s` (%s:%d)\n", e.Category, e.Method, e.Name, e.File, e.Line)
		} else {
			fmt.Fprintf(&b, "- [%s] `%s` (%s:%d)\n", e.Category, e.Name, e.File, e.Line)
		}
	}
	return b.String()
}
