package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Violation struct {
	Rule        string `json:"rule"`
	Severity    string `json:"severity"`
	Actual      any    `json:"actual"`
	Threshold   any    `json:"threshold"`
	File        string `json:"file,omitempty"`
	Line        int    `json:"line,omitempty"`
	Message     string `json:"message,omitempty"`
	Whitelisted bool   `json:"whitelisted"`
	WhitelistID string `json:"whitelist_id,omitempty"`
}

type GateResult struct {
	Status           string      `json:"status"`
	Mode             string      `json:"mode"`
	OverallScore     float64     `json:"overall_score"`
	Violations       []Violation `json:"violations"`
	WhitelistApplied int         `json:"whitelist_applied"`
	ExitCode         int         `json:"exit_code"`
	CheckedAt        string      `json:"checked_at"`
}

type Options struct {
	ProjectPath string
	ScoreDir    string
	Mode        string
	ConfigPath  string
	OutputDir   string
}

var defaultConfig = map[string]any{
	"mode": "strict",
	"thresholds": map[string]any{
		"overall_score":   map[string]any{"warn": 70.0, "fail": 60.0},
		"critical_issues": map[string]any{"warn": 0.0, "fail": 1.0},
		"high_issues":     map[string]any{"warn": 5.0, "fail": 10.0},
		"security_issues": map[string]any{"warn": 0.0, "fail": 1.0},
	},
	"whitelist": []any{},
}

func Execute(opts Options) (GateResult, string, string, error) {
	projectRoot, err := filepath.Abs(opts.ProjectPath)
	if err != nil {
		return GateResult{}, "", "", fmt.Errorf("resolve project: %w", err)
	}
	scoreDir := opts.ScoreDir
	if scoreDir == "" {
		if found := findScoreDirFromContextHub(projectRoot); found != "" {
			scoreDir = found
		} else {
			scoreDir = filepath.Join(projectRoot, ".arc", "score", filepath.Base(projectRoot))
		}
	}
	result, err := RunGateCheck(projectRoot, scoreDir, opts.Mode, opts.ConfigPath)
	if err != nil {
		return GateResult{}, "", "", err
	}
	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = ".arc/gate-reports"
	}
	if !filepath.IsAbs(outputDir) {
		outputDir = filepath.Join(projectRoot, outputDir)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return GateResult{}, "", "", err
	}
	jsonPath := filepath.Join(outputDir, "gate-result.json")
	summaryPath := filepath.Join(outputDir, "summary.md")
	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return GateResult{}, "", "", err
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(jsonPath, payload, 0o644); err != nil {
		return GateResult{}, "", "", err
	}
	summary := GenerateSummary(result)
	if err := os.WriteFile(summaryPath, []byte(summary), 0o644); err != nil {
		return GateResult{}, "", "", err
	}
	return result, jsonPath, summaryPath, nil
}

func RunGateCheck(projectPath string, scoreDir string, mode string, configPath string) (GateResult, error) {
	config, err := loadConfig(configPath, projectPath)
	if err != nil {
		return GateResult{}, err
	}
	if mode != "" {
		config["mode"] = mode
	}
	effectiveMode := asString(config["mode"])
	scoreData := loadScoreData(scoreDir)

	violations := make([]Violation, 0)
	whitelistApplied := 0
	overall, hasOverall := scoreData["overall"].(map[string]any)
	if !hasOverall {
		violations = append(violations, Violation{Rule: "missing_overall_score", Severity: "fail", Actual: "missing", Threshold: "required", Message: "未找到 score/overall-score.json，无法执行门禁检查"})
	}
	overallScore := 0.0
	bySeverity := map[string]any{}
	if hasOverall {
		overallScore = asFloat(overall["score"])
		if value, ok := overall["by_severity"].(map[string]any); ok {
			bySeverity = value
		}
	}
	thresholds := toMap(config["thresholds"])
	overallThresholds := toMap(thresholds["overall_score"])
	if hasOverall {
		failThreshold := asFloat(overallThresholds["fail"])
		warnThreshold := asFloat(overallThresholds["warn"])
		if overallScore < failThreshold {
			violations = append(violations, Violation{Rule: "overall_score_threshold", Severity: "fail", Actual: overallScore, Threshold: failThreshold, Message: fmt.Sprintf("综合评分 %.2f 低于失败阈值 %.2f", overallScore, failThreshold)})
		} else if overallScore < warnThreshold {
			violations = append(violations, Violation{Rule: "overall_score_threshold", Severity: "warn", Actual: overallScore, Threshold: warnThreshold, Message: fmt.Sprintf("综合评分 %.2f 低于告警阈值 %.2f", overallScore, warnThreshold)})
		}
	}
	if hasOverall {
		criticalThresholds := toMap(thresholds["critical_issues"])
		criticalCount := int(asFloat(bySeverity["critical"]))
		if float64(criticalCount) > asFloat(criticalThresholds["warn"]) {
			severity := "warn"
			threshold := asFloat(criticalThresholds["warn"])
			if float64(criticalCount) >= asFloat(criticalThresholds["fail"]) {
				severity = "fail"
				threshold = asFloat(criticalThresholds["fail"])
			}
			violations = append(violations, Violation{Rule: "critical_issues_threshold", Severity: severity, Actual: criticalCount, Threshold: threshold, Message: fmt.Sprintf("存在 %d 个严重问题", criticalCount)})
		}
		highThresholds := toMap(thresholds["high_issues"])
		highCount := int(asFloat(bySeverity["high"]))
		if float64(highCount) > asFloat(highThresholds["warn"]) {
			severity := "warn"
			threshold := asFloat(highThresholds["warn"])
			if float64(highCount) >= asFloat(highThresholds["fail"]) {
				severity = "fail"
				threshold = asFloat(highThresholds["fail"])
			}
			violations = append(violations, Violation{Rule: "high_issues_threshold", Severity: severity, Actual: highCount, Threshold: threshold, Message: fmt.Sprintf("存在 %d 个高优先级问题", highCount)})
		}
	}

	smell, hasSmell := scoreData["smell"].(map[string]any)
	if !hasSmell {
		violations = append(violations, Violation{Rule: "missing_smell_report", Severity: "fail", Actual: "missing", Threshold: "required", Message: "未找到 analysis/smell-report.json，无法检查安全问题"})
	}
	securityCount := 0
	if hasSmell {
		if entries, ok := smell["violations"].([]any); ok {
			for _, entry := range entries {
				value, ok := entry.(map[string]any)
				if ok && asString(value["category"]) == "security" {
					securityCount++
				}
			}
		}
	}
	securityThresholds := toMap(thresholds["security_issues"])
	if float64(securityCount) > asFloat(securityThresholds["warn"]) {
		severity := "warn"
		threshold := asFloat(securityThresholds["warn"])
		if float64(securityCount) >= asFloat(securityThresholds["fail"]) {
			severity = "fail"
			threshold = asFloat(securityThresholds["fail"])
		}
		violations = append(violations, Violation{Rule: "security_issues_threshold", Severity: severity, Actual: securityCount, Threshold: threshold, Message: fmt.Sprintf("存在 %d 个安全问题", securityCount)})
	}

	whitelist, _ := config["whitelist"].([]any)
	for index := range violations {
		if ok, whitelistID := checkWhitelist(violations[index], whitelist); ok {
			violations[index].Whitelisted = true
			violations[index].WhitelistID = whitelistID
			whitelistApplied++
		}
	}

	unwhitelistedFail := false
	dangerousBlocker := false
	for _, violation := range violations {
		if violation.Severity == "fail" && !violation.Whitelisted {
			unwhitelistedFail = true
		}
		if !violation.Whitelisted && (violation.Rule == "security_issues_threshold" || violation.Rule == "critical_issues_threshold") {
			dangerousBlocker = true
		}
	}

	status := "pass"
	exitCode := 0
	switch effectiveMode {
	case "warn":
	case "strict":
		if unwhitelistedFail {
			status = "fail"
			exitCode = 1
		}
	default:
		if unwhitelistedFail || dangerousBlocker {
			status = "fail"
			exitCode = 1
		}
	}

	return GateResult{Status: status, Mode: effectiveMode, OverallScore: overallScore, Violations: violations, WhitelistApplied: whitelistApplied, ExitCode: exitCode, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func GenerateSummary(result GateResult) string {
	statusEmoji := "✅"
	if result.Status != "pass" {
		statusEmoji = "❌"
	}
	lines := []string{
		"# CI 门禁报告",
		"",
		"## 执行结果",
		"",
		"| 指标 | 值 |",
		"|------|-----|",
		fmt.Sprintf("| **状态** | %s %s |", statusEmoji, strings.ToUpper(result.Status)),
		fmt.Sprintf("| **模式** | %s |", result.Mode),
		fmt.Sprintf("| **综合评分** | %.2f / 100 |", result.OverallScore),
		fmt.Sprintf("| **豁免应用** | %d |", result.WhitelistApplied),
		fmt.Sprintf("| **检查时间** | %s |", result.CheckedAt),
		"",
		"## 违规详情",
		"",
		"| 规则 | 严重程度 | 实际值 | 阈值 | 豁免 |",
		"|------|---------|--------|------|------|",
	}
	if len(result.Violations) == 0 {
		lines = append(lines, "| (无违规) | - | - | - | - |")
	} else {
		for _, violation := range result.Violations {
			whitelisted := "-"
			if violation.Whitelisted {
				whitelisted = "✓"
			}
			lines = append(lines, fmt.Sprintf("| %s | %s | %v | %v | %s |", violation.Rule, violation.Severity, violation.Actual, violation.Threshold, whitelisted))
		}
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}

func loadConfig(configPath string, projectPath string) (map[string]any, error) {
	config := deepCopy(defaultConfig)
	if configPath == "" {
		candidate := filepath.Join(projectPath, ".arc", "gate-config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			configPath = candidate
		}
	}
	if configPath != "" {
		payload, err := os.ReadFile(configPath)
		if err != nil {
			return nil, err
		}
		userConfig := map[string]any{}
		if err := yaml.Unmarshal(payload, &userConfig); err != nil {
			return nil, err
		}
		mergeConfig(config, userConfig)
	}
	if value := os.Getenv("GATE_MODE"); value != "" {
		config["mode"] = value
	}
	return config, nil
}

func mergeConfig(target map[string]any, source map[string]any) {
	if value, ok := source["mode"]; ok {
		target["mode"] = value
	}
	if thresholds, ok := source["thresholds"].(map[string]any); ok {
		current := toMap(target["thresholds"])
		for key, raw := range thresholds {
			normalizedKey := key
			if key == "security_violations" {
				normalizedKey = "security_issues"
			}
			if nextMap, ok := raw.(map[string]any); ok {
				existing := toMap(current[normalizedKey])
				for childKey, childValue := range nextMap {
					existing[childKey] = childValue
				}
				current[normalizedKey] = existing
			} else {
				current[normalizedKey] = raw
			}
		}
		target["thresholds"] = current
	}
	if whitelist, ok := source["whitelist"]; ok {
		target["whitelist"] = whitelist
	}
}

func loadScoreData(scoreDir string) map[string]any {
	result := map[string]any{}
	loadJSONInto(filepath.Join(scoreDir, "score", "overall-score.json"), result, "overall")
	loadJSONInto(filepath.Join(scoreDir, "analysis", "smell-report.json"), result, "smell")
	return result
}

func loadJSONInto(path string, target map[string]any, key string) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var value map[string]any
	if err := json.Unmarshal(payload, &value); err != nil {
		return
	}
	target[key] = value
}

func findScoreDirFromContextHub(projectRoot string) string {
	indexPath := filepath.Join(projectRoot, ".arc", "context-hub", "index.json")
	payload, err := os.ReadFile(indexPath)
	if err != nil {
		return ""
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return ""
	}
	items, ok := raw["artifacts"].([]any)
	if !ok {
		return ""
	}
	bestScore := 999
	bestPath := ""
	for _, item := range items {
		candidate, ok := item.(map[string]any)
		if !ok || asString(candidate["producer_skill"]) != "arc:score" {
			continue
		}
		if artifactExpired(candidate) {
			continue
		}
		pathValue := asString(candidate["path"])
		if pathValue == "" {
			continue
		}
		artifactPath := pathValue
		if !filepath.IsAbs(artifactPath) {
			artifactPath = filepath.Join(projectRoot, artifactPath)
		}
		info, err := os.Stat(artifactPath)
		if err != nil {
			continue
		}
		expectedHash := normalizeSHA256(candidate["content_hash"])
		if expectedHash != "" {
			if info.IsDir() {
				continue
			}
			actualHash, err := sha256File(artifactPath)
			if err != nil || actualHash != expectedHash {
				continue
			}
		}
		preference := preferenceForArtifact(pathValue)
		scoreDir := filepath.Clean(filepath.Join(filepath.Dir(artifactPath), ".."))
		if _, err := os.Stat(filepath.Join(scoreDir, "score", "overall-score.json")); err != nil {
			continue
		}
		if preference < bestScore {
			bestScore = preference
			bestPath = scoreDir
		}
	}
	return bestPath
}

func preferenceForArtifact(path string) int {
	switch {
	case strings.HasSuffix(path, "handoff/review-input.json"):
		return 0
	case strings.HasSuffix(path, "score/overall-score.json"):
		return 1
	case strings.HasSuffix(path, "analysis/smell-report.json"):
		return 2
	case strings.HasSuffix(path, "analysis/bugfix-grades.json"):
		return 3
	default:
		return 10
	}
}

func artifactExpired(item map[string]any) bool {
	value := asString(item["expires_at"])
	if value == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return false
	}
	return time.Now().UTC().After(parsed)
}

func sha256File(path string) (string, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

func normalizeSHA256(value any) string {
	text := strings.TrimSpace(asString(value))
	if text == "" {
		return ""
	}
	if strings.Contains(text, ":") {
		parts := strings.SplitN(text, ":", 2)
		text = strings.TrimSpace(parts[1])
	}
	if len(text) != 64 {
		return ""
	}
	return strings.ToLower(text)
}

func checkWhitelist(violation Violation, whitelist []any) (bool, string) {
	now := time.Now().UTC()
	for _, item := range whitelist {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if asString(entry["rule"]) != violation.Rule {
			continue
		}
		fileValue := asString(entry["file"])
		if fileValue != "" && fileValue != violation.File {
			continue
		}
		expiresAt := asString(entry["expires_at"])
		if expiresAt != "" {
			parsed, err := time.Parse(time.RFC3339, expiresAt)
			if err == nil && now.After(parsed) {
				continue
			}
		}
		return true, asString(entry["id"])
	}
	return false, ""
}

func deepCopy(source map[string]any) map[string]any {
	payload, _ := json.Marshal(source)
	target := map[string]any{}
	_ = json.Unmarshal(payload, &target)
	return target
}

func toMap(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func asFloat(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case json.Number:
		result, _ := typed.Float64()
		return result
	case string:
		parsed, _ := json.Number(typed).Float64()
		return parsed
	default:
		return 0
	}
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}
