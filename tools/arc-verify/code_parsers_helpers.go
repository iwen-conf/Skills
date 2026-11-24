package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	reCargoTestSummary   = regexp.MustCompile(`test result:\s*(?:ok|FAILED)\.\s*(\d+)\s+passed;\s*(\d+)\s+failed;\s*(\d+)\s+ignored`)
	reNextestSummary     = regexp.MustCompile(`(\d+)\s+tests?\s+run:\s*(\d+)\s+passed(?:,\s*(\d+)\s+failed)?(?:,\s*(\d+)\s+skipped)?`)
	rePytestPassed       = regexp.MustCompile(`\b(\d+)\s+passed\b`)
	rePytestFailed       = regexp.MustCompile(`\b(\d+)\s+(?:failed|errors?)\b`)
	rePytestSkipped      = regexp.MustCompile(`\b(\d+)\s+skipped\b`)
	reVitestTestsLine    = regexp.MustCompile(`(?m)^\s*Tests\s+(.+)$`)
	reCTestSummary       = regexp.MustCompile(`(\d+)\s+tests?\s+passed\s+out\s+of\s+(\d+)`)
	reGoCoverTotal       = regexp.MustCompile(`total:\s+\(statements\)\s+([0-9]+(?:\.[0-9]+)?)%`)
	reGoCoverFuncLine    = regexp.MustCompile(`^\S+:\d+:\s+\S+\s+([0-9]+(?:\.[0-9]+)?)%$`)
	reMutantsLine        = regexp.MustCompile(`(\d+)\s+mutants?\s+tested(?:[^0-9]+(\d+)\s+caught)?(?:[^0-9]+(\d+)\s+missed)?`)
	reMutantsCaughtText  = regexp.MustCompile(`(\d+)\s+caught`)
	reMutantsMissedText  = regexp.MustCompile(`(\d+)\s+missed`)
	reMutantsTimeoutText = regexp.MustCompile(`(\d+)\s+timeout`)
	reKilledCountText    = regexp.MustCompile(`(?i)(?:killed\s*[:=]\s*(\d+)|(\d+)\s+killed)`)
	reMissedMutText      = regexp.MustCompile(`(?i)(?:(?:lived|survived|missed)\s*[:=]\s*(\d+)|(\d+)\s+(?:lived|survived))`)
	reTimeoutMutText     = regexp.MustCompile(`(?i)(?:timed\s*out|timeout)\s*[:=]\s*(\d+)`)
	reStrykerScoreText   = regexp.MustCompile(`(?i)mutation\s+score:\s*([0-9]+(?:\.[0-9]+)?)%`)
	reFuzzRuns           = regexp.MustCompile(`(?:#|runs:\s*|execs:\s*)(\d+)`)
	reTSCErrorLine       = regexp.MustCompile(`error TS\d+:`)
	reClangTidyWarnLine  = regexp.MustCompile(`:\d+:\d+:\s+(?:warning|error):`)
	reRuffCodeLine       = regexp.MustCompile(`^[^\s:]+:\d+:\d+:\s+[A-Z]+\d+\s+`)
)

// mergeCodeMetricPayloads 将后者显式观测到的非零或非空指标叠加合并到前者上
// 请求体
// base 基础度量指标载荷
// over 覆盖度量指标载荷
// 响应体
// merged 合并后的度量指标载荷
func mergeCodeMetricPayloads(base, over CodeMetricPayload) CodeMetricPayload {
	// 1. 合并测试计数、安全、变异与并发整数指标
	out := base
	if over.TestsPassed > 0 {
		out.TestsPassed = over.TestsPassed
	}
	if over.TestsFailed > 0 {
		out.TestsFailed = over.TestsFailed
	}
	if over.TestsSkipped > 0 {
		out.TestsSkipped = over.TestsSkipped
	}
	if over.Warnings > 0 {
		out.Warnings = over.Warnings
	}
	if over.HighVulns > 0 {
		out.HighVulns = over.HighVulns
	}
	if over.CriticalVulns > 0 {
		out.CriticalVulns = over.CriticalVulns
	}
	if over.Crashes > 0 {
		out.Crashes = over.Crashes
	}
	if over.FuzzExecs > 0 {
		out.FuzzExecs = over.FuzzExecs
	}
	if over.MutantsCaught > 0 {
		out.MutantsCaught = over.MutantsCaught
	}
	if over.MutantsMissed > 0 {
		out.MutantsMissed = over.MutantsMissed
	}
	if over.MutantsTimeout > 0 {
		out.MutantsTimeout = over.MutantsTimeout
	}
	if over.MutantsUnviable > 0 {
		out.MutantsUnviable = over.MutantsUnviable
	}
	if over.MutationScorePct != nil {
		out.MutationScorePct = over.MutationScorePct
	}
	if over.DataRaces > 0 {
		out.DataRaces = over.DataRaces
	}
	if over.UBErrors > 0 {
		out.UBErrors = over.UBErrors
	}
	if over.BenchRegressionPct != nil {
		out.BenchRegressionPct = over.BenchRegressionPct
	}
	if over.RSSGrowthPct != nil {
		out.RSSGrowthPct = over.RSSGrowthPct
	}
	if over.ProofsVerified > 0 {
		out.ProofsVerified = over.ProofsVerified
	}
	if over.ProofsFailed > 0 {
		out.ProofsFailed = over.ProofsFailed
	}
	// 2. 合并八项代码覆盖率明细与扩展映射
	if over.Coverage != nil {
		if out.Coverage == nil {
			cp := *over.Coverage
			out.Coverage = &cp
		} else {
			out.Coverage = mergeCoverageBreakdowns(out.Coverage, over.Coverage)
		}
	}
	if len(over.Extra) > 0 {
		out.Extra = mergeFactMaps(out.Extra, over.Extra)
	}
	return out
}

// mergeCoverageBreakdowns 合并两份八项代码覆盖率细分观测结构体
// 请求体
// a 基础覆盖率指针
// b 覆盖率指针
// 响应体
// merged 合并后的覆盖率指针
func mergeCoverageBreakdowns(a, b *CoverageBreakdown) *CoverageBreakdown {
	// 1. 逐项合并非空浮点指针与来源工具名称
	cp := *a
	if b.LinePct != nil {
		cp.LinePct = b.LinePct
	}
	if b.StatementPct != nil {
		cp.StatementPct = b.StatementPct
	}
	if b.FunctionPct != nil {
		cp.FunctionPct = b.FunctionPct
	}
	if b.BranchPct != nil {
		cp.BranchPct = b.BranchPct
	}
	if b.ConditionPct != nil {
		cp.ConditionPct = b.ConditionPct
	}
	if b.DecisionPct != nil {
		cp.DecisionPct = b.DecisionPct
	}
	if b.MCDCPct != nil {
		cp.MCDCPct = b.MCDCPct
	}
	if b.PathPct != nil {
		cp.PathPct = b.PathPct
	}
	if b.SourceTool != "" {
		cp.SourceTool = b.SourceTool
	}
	return &cp
}

// finalizeCoverageBreakdown 根据非空指针自动填充 MeasuredMetrics 与 UnsupportedMetrics 清单
// 请求体
// cov 覆盖率细分观测指针
// 响应体
// 无
func finalizeCoverageBreakdown(cov *CoverageBreakdown) {
	// 1. 按八项标准覆盖率枚举固定顺序分类已测量与未支持指标
	if cov == nil {
		return
	}
	cov.MeasuredMetrics = nil
	cov.UnsupportedMetrics = nil
	checks := []struct {
		kind CoverageMetricKind
		ptr  *float64
	}{
		{CovLine, cov.LinePct},
		{CovStatement, cov.StatementPct},
		{CovFunction, cov.FunctionPct},
		{CovBranch, cov.BranchPct},
		{CovCondition, cov.ConditionPct},
		{CovDecision, cov.DecisionPct},
		{CovMCDC, cov.MCDCPct},
		{CovPath, cov.PathPct},
	}
	for _, c := range checks {
		if c.ptr != nil {
			cov.MeasuredMetrics = append(cov.MeasuredMetrics, c.kind)
		} else {
			cov.UnsupportedMetrics = append(cov.UnsupportedMetrics, c.kind)
		}
	}
}

// parseNativeCoverage 解析 cargo-llvm-cov、go tool cover、pytest-cov 与 vitest 覆盖率输出
// 请求体
// lang 目标语言
// toolName 执行工具名
// output 工具输出文本
// artifactDir 产物落盘目录
// 响应体
// payload 含覆盖率细分的度量载荷
func parseNativeCoverage(lang, toolName, output, artifactDir string) CodeMetricPayload {
	// 1. 尝试解析 cargo-llvm-cov JSON 输出
	_ = lang
	cov := &CoverageBreakdown{SourceTool: toolName}
	if strings.Contains(output, `"totals"`) {
		for _, rawObj := range extractAllJSONObjects(output) {
			var llvmDoc llvmCovDocument
			if json.Unmarshal([]byte(rawObj), &llvmDoc) == nil && len(llvmDoc.Data) > 0 {
				t := llvmDoc.Data[0].Totals
				cov.LinePct = validLLVMStatPct(t.Lines)
				cov.StatementPct = validLLVMStatPct(t.Regions)
				cov.FunctionPct = validLLVMStatPct(t.Functions)
				cov.BranchPct = validLLVMStatPct(t.Branches)
				cov.MCDCPct = validLLVMStatPct(t.MCDC)
				finalizeCoverageBreakdown(cov)
				return CodeMetricPayload{Coverage: cov}
			}
		}
	}

	// 2. 解析 Go go tool cover -func 输出
	if m := reGoCoverTotal.FindStringSubmatch(output); m != nil {
		if pct, err := strconv.ParseFloat(m[1], 64); err == nil {
			cov.StatementPct = floatPtr(pct)
			cov.SourceTool = "go tool cover"
		}
		fnTotal, fnCovered := 0, 0
		for _, line := range strings.Split(output, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "total:") {
				continue
			}
			if fm := reGoCoverFuncLine.FindStringSubmatch(trimmed); fm != nil {
				fnTotal++
				if val, err := strconv.ParseFloat(fm[1], 64); err == nil && val > 0 {
					fnCovered++
				}
			}
		}
		if fnTotal > 0 {
			cov.FunctionPct = floatPtr(float64(fnCovered) * 100.0 / float64(fnTotal))
		}
		finalizeCoverageBreakdown(cov)
		return CodeMetricPayload{Coverage: cov}
	}

	// 3. 尝试读取产物或输出中的 pytest-cov coverage.json / vitest coverage-summary.json
	candidates := []string{output}
	if artifactDir != "" {
		for _, rel := range []string{"coverage.json", filepath.Join("coverage", "coverage-summary.json"), "coverage-summary.json"} {
			if raw, err := os.ReadFile(filepath.Join(artifactDir, rel)); err == nil {
				candidates = append([]string{string(raw)}, candidates...)
			}
		}
	}
	for _, src := range candidates {
		rawObj := extractOuterJSON(src)
		if rawObj == "" {
			continue
		}
		var pyDoc pytestCovDocument
		if json.Unmarshal([]byte(rawObj), &pyDoc) == nil && pyDoc.Totals.PercentCovered != nil {
			cov.LinePct = pyDoc.Totals.PercentCovered
			if pyDoc.Totals.NumStatements > 0 {
				cov.StatementPct = floatPtr(float64(pyDoc.Totals.CoveredLines) * 100.0 / float64(pyDoc.Totals.NumStatements))
			}
			if pyDoc.Totals.NumBranches > 0 {
				cov.BranchPct = floatPtr(float64(pyDoc.Totals.CoveredBranches) * 100.0 / float64(pyDoc.Totals.NumBranches))
			}
			cov.SourceTool = "pytest-cov"
			finalizeCoverageBreakdown(cov)
			return CodeMetricPayload{Coverage: cov}
		}
		var istDoc istanbulSummaryDocument
		if json.Unmarshal([]byte(rawObj), &istDoc) == nil && (istDoc.Total.Lines != nil || istDoc.Total.Statements != nil) {
			if istDoc.Total.Lines != nil {
				cov.LinePct = istDoc.Total.Lines.Pct
			}
			if istDoc.Total.Statements != nil {
				cov.StatementPct = istDoc.Total.Statements.Pct
			}
			if istDoc.Total.Functions != nil {
				cov.FunctionPct = istDoc.Total.Functions.Pct
			}
			if istDoc.Total.Branches != nil {
				cov.BranchPct = istDoc.Total.Branches.Pct
			}
			cov.SourceTool = "vitest/istanbul"
			finalizeCoverageBreakdown(cov)
			return CodeMetricPayload{Coverage: cov}
		}
	}

	// 4. 未能解析数值时仍附带空白 Breakdown 明确标示全部未测量指标
	finalizeCoverageBreakdown(cov)
	return CodeMetricPayload{Coverage: cov}
}

// validLLVMStatPct 校验 llvm-cov 指标条目的百分比指针并剔除 count==0 的未启用项
// 请求体
// st llvm-cov 单项覆盖率统计指针
// 响应体
// pct 合法覆盖率百分比指针（未启用返回nil）
func validLLVMStatPct(st *llvmCovStat) *float64 {
	// 1. 若明确报告总插桩元素数为0则判定该细分维度未开启
	if st == nil || st.Percent == nil {
		return nil
	}
	if st.Count != nil && *st.Count == 0 {
		return nil
	}
	return floatPtr(*st.Percent)
}

// parseNativeMutation 解析 cargo-mutants、gremlins、mutmut 与 stryker 的变异报告与输出
// 请求体
// output 命令标准输出文本
// artifactDir 产物落盘目录
// 响应体
// payload 含变异体统计与杀死率百分比的度量载荷
func parseNativeMutation(output, artifactDir string) CodeMetricPayload {
	// 1. 优先扫描产物目录及标准输出中的结构化 JSON（outcomes.json / mutants.out / stryker / gremlins）
	var p CodeMetricPayload
	jsonSources := []string{output}
	if artifactDir != "" {
		for _, rel := range []string{
			"outcomes.json",
			filepath.Join("mutants.out", "outcomes.json"),
			filepath.Join("reports", "mutation", "mutation.json"),
			"mutation.json",
		} {
			if raw, err := os.ReadFile(filepath.Join(artifactDir, rel)); err == nil {
				jsonSources = append([]string{string(raw)}, jsonSources...)
			}
		}
	}
	for _, src := range jsonSources {
		if parsed, ok := parseMutationJSON(src); ok {
			return parsed
		}
	}

	// 2. 回退正则解析 cargo-mutants / gremlins / mutmut / stryker 文本总结行
	if m := reMutantsLine.FindStringSubmatch(output); m != nil {
		if len(m) > 2 && m[2] != "" {
			p.MutantsCaught, _ = strconv.Atoi(m[2])
		}
		if len(m) > 3 && m[3] != "" {
			p.MutantsMissed, _ = strconv.Atoi(m[3])
		}
	}
	if p.MutantsCaught == 0 {
		if m := reMutantsCaughtText.FindStringSubmatch(output); m != nil {
			p.MutantsCaught, _ = strconv.Atoi(m[1])
		} else if m := reKilledCountText.FindStringSubmatch(output); m != nil {
			p.MutantsCaught, _ = strconv.Atoi(nonEmpty(m[1], m[2]))
		}
	}
	if p.MutantsMissed == 0 {
		if m := reMutantsMissedText.FindStringSubmatch(output); m != nil {
			p.MutantsMissed, _ = strconv.Atoi(m[1])
		} else if m := reMissedMutText.FindStringSubmatch(output); m != nil {
			p.MutantsMissed, _ = strconv.Atoi(nonEmpty(m[1], m[2]))
		}
	}
	if p.MutantsTimeout == 0 {
		if m := reMutantsTimeoutText.FindStringSubmatch(output); m != nil {
			p.MutantsTimeout, _ = strconv.Atoi(m[1])
		} else if m := reTimeoutMutText.FindStringSubmatch(output); m != nil {
			p.MutantsTimeout, _ = strconv.Atoi(m[1])
		}
	}
	if denom := p.MutantsCaught + p.MutantsMissed; denom > 0 {
		p.MutationScorePct = floatPtr(float64(p.MutantsCaught) * 100.0 / float64(denom))
	} else if m := reStrykerScoreText.FindStringSubmatch(output); m != nil {
		if score, err := strconv.ParseFloat(m[1], 64); err == nil {
			p.MutationScorePct = floatPtr(score)
		}
	}
	return p
}

// parseMutationJSON 尝试将单段文本解码为 cargo-mutants、stryker 或 gremlins 结构化 JSON 报告
// 请求体
// src 原始 JSON 或混排日志文本
// 响应体
// payload 解析出的变异度量载荷
// matched 是否成功匹配到结构化变异报告
func parseMutationJSON(src string) (CodeMetricPayload, bool) {
	// 1. 提取外层 JSON 并按字段结构分发解析
	rawObj := extractOuterJSON(src)
	if rawObj == "" {
		return CodeMetricPayload{}, false
	}
	var p CodeMetricPayload
	var doc cargoMutantsOutcomesDocument
	if json.Unmarshal([]byte(rawObj), &doc) == nil && len(doc.Outcomes) > 0 {
		for _, o := range doc.Outcomes {
			low := strings.ToLower(o.Summary)
			switch {
			case strings.Contains(low, "caught"):
				p.MutantsCaught++
			case strings.Contains(low, "missed"):
				p.MutantsMissed++
			case strings.Contains(low, "timeout"):
				p.MutantsTimeout++
			case strings.Contains(low, "unviable"):
				p.MutantsUnviable++
			}
		}
		if denom := p.MutantsCaught + p.MutantsMissed; denom > 0 {
			p.MutationScorePct = floatPtr(float64(p.MutantsCaught) * 100.0 / float64(denom))
		}
		return p, true
	}
	var strykerDoc strykerReportDocument
	if json.Unmarshal([]byte(rawObj), &strykerDoc) == nil && len(strykerDoc.Files) > 0 {
		for _, f := range strykerDoc.Files {
			for _, m := range f.Mutants {
				switch strings.ToLower(strings.TrimSpace(m.Status)) {
				case "killed":
					p.MutantsCaught++
				case "survived", "nocoverage":
					p.MutantsMissed++
				case "timeout":
					p.MutantsTimeout++
				case "compileerror", "runtimeerror", "ignored":
					p.MutantsUnviable++
				}
			}
		}
		if denom := p.MutantsCaught + p.MutantsMissed; denom > 0 {
			p.MutationScorePct = floatPtr(float64(p.MutantsCaught) * 100.0 / float64(denom))
		}
		return p, true
	}
	var gremDoc gremlinsSummaryDocument
	if json.Unmarshal([]byte(rawObj), &gremDoc) == nil && (gremDoc.Killed > 0 || gremDoc.Lived > 0 || gremDoc.TimedOut > 0 || gremDoc.NotViable > 0) {
		p.MutantsCaught = gremDoc.Killed
		p.MutantsMissed = gremDoc.Lived
		p.MutantsTimeout = gremDoc.TimedOut
		p.MutantsUnviable = gremDoc.NotViable
		if denom := p.MutantsCaught + p.MutantsMissed; denom > 0 {
			p.MutationScorePct = floatPtr(float64(p.MutantsCaught) * 100.0 / float64(denom))
		}
		return p, true
	}
	return CodeMetricPayload{}, false
}

// parseNativeSecurityAudit 解析 cargo-audit、govulncheck、npm audit、pip-audit 与 semgrep 漏洞报告
// 请求体
// techID 测试技术标识
// output 工具输出文本
// 响应体
// payload 含高危漏洞数与告警数的度量载荷
func parseNativeSecurityAudit(techID, output string) CodeMetricPayload {
	// 1. 优先处理 dependency_audit 的多工具 JSON/NDJSON 报告格式
	var p CodeMetricPayload
	if techID == "dependency_audit" {
		if arrRaw := extractOuterJSONArray(output); arrRaw != "" {
			var pyList []pipAuditDependency
			if json.Unmarshal([]byte(arrRaw), &pyList) == nil {
				for _, dep := range pyList {
					p.HighVulns += len(dep.Vulns)
				}
				return p
			}
		}
		objs := extractAllJSONObjects(output)
		foundGoFinding := false
		seenOSV := map[string]bool{}
		for _, obj := range objs {
			var goDoc govulncheckEventDocument
			if json.Unmarshal([]byte(obj), &goDoc) == nil && goDoc.Finding != nil && goDoc.Finding.OSV != "" {
				if !seenOSV[goDoc.Finding.OSV] {
					seenOSV[goDoc.Finding.OSV] = true
					p.HighVulns++
				}
				foundGoFinding = true
			}
		}
		if foundGoFinding {
			return p
		}
		for _, rawObj := range objs {
			var auditDoc cargoAuditDocument
			if json.Unmarshal([]byte(rawObj), &auditDoc) == nil && (auditDoc.Vulnerabilities.Found || auditDoc.Vulnerabilities.Count > 0 || len(auditDoc.Vulnerabilities.List) > 0) {
				cnt := auditDoc.Vulnerabilities.Count
				if cnt == 0 {
					cnt = len(auditDoc.Vulnerabilities.List)
				}
				p.HighVulns = cnt
				return p
			}
			var npmDoc npmAuditDocument
			if json.Unmarshal([]byte(rawObj), &npmDoc) == nil && npmDoc.Metadata != nil {
				v := npmDoc.Metadata.Vulnerabilities
				p.CriticalVulns = v.Critical
				p.HighVulns = v.High
				p.Warnings = v.Moderate + v.Low
				return p
			}
			var pipWrap pipAuditWrapperDocument
			if json.Unmarshal([]byte(rawObj), &pipWrap) == nil && len(pipWrap.Dependencies) > 0 {
				for _, dep := range pipWrap.Dependencies {
					p.HighVulns += len(dep.Vulns)
				}
				return p
			}
		}
	}

	// 2. 解析 SAST 工具（semgrep）的 JSON 命中统计
	for _, rawObj := range extractAllJSONObjects(output) {
		var sgDoc semgrepDocument
		if json.Unmarshal([]byte(rawObj), &sgDoc) == nil && len(sgDoc.Results) > 0 {
			for _, r := range sgDoc.Results {
				sev := strings.ToUpper(r.Extra.Severity)
				if sev == "ERROR" || sev == "CRITICAL" || sev == "HIGH" {
					p.HighVulns++
				} else {
					p.Warnings++
				}
			}
			return p
		}
	}
	return p
}

// parseNativeStaticWarnings 统计 clippy、go vet、ruff、tsc 与 clang-tidy 输出中的告警行数
// 请求体
// lang 目标语言
// output 静态检查命令输出文本
// 响应体
// payload 含告警计数的度量载荷
func parseNativeStaticWarnings(lang, output string) CodeMetricPayload {
	// 1. 按常见编译器与检查器告警格式逐行计数（含 Python ruff JSON 与文本诊断）
	var p CodeMetricPayload
	if lang == "python" {
		if arrRaw := extractOuterJSONArray(output); arrRaw != "" {
			var ruffItems []ruffDiagnosticItem
			if json.Unmarshal([]byte(arrRaw), &ruffItems) == nil {
				p.Warnings = len(ruffItems)
				return p
			}
		}
	}
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "warning:") && !strings.Contains(trimmed, "warnings emitted") && !strings.Contains(trimmed, "warning emitted"):
			p.Warnings++
		case lang == "go" && strings.Contains(trimmed, ".go:") && !strings.HasPrefix(trimmed, "#"):
			p.Warnings++
		case lang == "typescript" && reTSCErrorLine.MatchString(trimmed):
			p.Warnings++
		case lang == "python" && reRuffCodeLine.MatchString(trimmed):
			p.Warnings++
		case lang == "c_cpp" && reClangTidyWarnLine.MatchString(trimmed):
			p.Warnings++
		}
	}
	return p
}

// parseNativeConcurrencyAndMemory 识别 ThreadSanitizer、Go race、Miri 与 AddressSanitizer 缺陷输出
// 请求体
// output 运行时分析器输出文本
// 响应体
// payload 含数据竞争与未定义行为计数的度量载荷
func parseNativeConcurrencyAndMemory(output string) CodeMetricPayload {
	// 1. 扫描数据竞争与内存未定义行为特征串并计数
	var p CodeMetricPayload
	p.DataRaces += strings.Count(output, "WARNING: DATA RACE")
	p.DataRaces += strings.Count(output, "WARNING: ThreadSanitizer: data race")
	p.UBErrors += strings.Count(output, "Undefined Behavior")
	p.UBErrors += strings.Count(output, "ERROR: AddressSanitizer")
	p.UBErrors += strings.Count(output, "ERROR: UndefinedBehaviorSanitizer")
	return p
}

// parseNativeFuzz 提取模糊测试输出中的崩溃计数与累计执行样本数
// 请求体
// output 模糊测试日志文本
// 响应体
// payload 含崩溃次数与执行样本数的度量载荷
func parseNativeFuzz(output string) CodeMetricPayload {
	// 1. 检测崩溃标记与 libFuzzer/Go fuzz 迭代次数
	var p CodeMetricPayload
	for _, line := range strings.Split(output, "\n") {
		up := strings.ToUpper(line)
		if strings.Contains(up, "SUMMARY: ADDRESS") || strings.Contains(up, "PANIC:") || strings.Contains(up, "TEST UNIT WRITTEN TO") {
			p.Crashes++
		}
		if m := reFuzzRuns.FindStringSubmatch(line); m != nil {
			if n, err := strconv.ParseInt(m[1], 10, 64); err == nil && n > p.FuzzExecs {
				p.FuzzExecs = n
			}
		}
	}
	return p
}

// parseNativeTestCounts 从 cargo test、nextest、go test、pytest、vitest、ctest 与 cargo kani 输出提取计数
// 请求体
// output 测试运行器输出文本
// 响应体
// payload 汇总的测试计数与形式化证明载荷
func parseNativeTestCounts(output string) CodeMetricPayload {
	// 1. 解析 cargo kani 形式化验证结果（与其余测试计数互不排斥）
	var p CodeMetricPayload
	p.ProofsVerified = strings.Count(output, "VERIFICATION:- SUCCESSFUL")
	p.ProofsFailed = strings.Count(output, "VERIFICATION:- FAILED")

	// 2. 累加 Rust cargo test 与 nextest 摘要行
	for _, m := range reCargoTestSummary.FindAllStringSubmatch(output, -1) {
		passN, _ := strconv.Atoi(m[1])
		failN, _ := strconv.Atoi(m[2])
		skipN, _ := strconv.Atoi(m[3])
		p.TestsPassed += passN
		p.TestsFailed += failN
		p.TestsSkipped += skipN
	}
	if p.TestsPassed == 0 && p.TestsFailed == 0 {
		for _, m := range reNextestSummary.FindAllStringSubmatch(output, -1) {
			passN, _ := strconv.Atoi(m[2])
			failN, _ := strconv.Atoi(m[3])
			skipN, _ := strconv.Atoi(m[4])
			p.TestsPassed += passN
			p.TestsFailed += failN
			p.TestsSkipped += skipN
		}
	}

	// 3. 解析 Go 逐行测试结果
	if p.TestsPassed == 0 && p.TestsFailed == 0 {
		for _, line := range strings.Split(output, "\n") {
			trimmed := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(trimmed, "--- PASS:"):
				p.TestsPassed++
			case strings.HasPrefix(trimmed, "--- FAIL:"):
				p.TestsFailed++
			case strings.HasPrefix(trimmed, "--- SKIP:"):
				p.TestsSkipped++
			}
		}
	}

	// 4. 解析 Vitest JSON/文本、CTest 与 Pytest 总结行
	if p.TestsPassed == 0 && p.TestsFailed == 0 {
		if m := reVitestTestsLine.FindStringSubmatch(output); m != nil {
			summary := m[1]
			if pm := rePytestPassed.FindStringSubmatch(summary); pm != nil {
				p.TestsPassed, _ = strconv.Atoi(pm[1])
			}
			if fm := rePytestFailed.FindStringSubmatch(summary); fm != nil {
				p.TestsFailed, _ = strconv.Atoi(fm[1])
			}
			if sm := rePytestSkipped.FindStringSubmatch(summary); sm != nil {
				p.TestsSkipped, _ = strconv.Atoi(sm[1])
			}
		} else if m := reCTestSummary.FindStringSubmatch(output); m != nil {
			passN, _ := strconv.Atoi(m[1])
			totalN, _ := strconv.Atoi(m[2])
			p.TestsPassed = passN
			if totalN > passN {
				p.TestsFailed = totalN - passN
			}
		} else {
			for _, line := range strings.Split(output, "\n") {
				trimmed := strings.TrimSpace(line)
				if !strings.HasPrefix(trimmed, "=") && !strings.Contains(trimmed, " in ") {
					continue
				}
				if pm := rePytestPassed.FindStringSubmatch(trimmed); pm != nil {
					p.TestsPassed, _ = strconv.Atoi(pm[1])
				}
				for _, fm := range rePytestFailed.FindAllStringSubmatch(trimmed, -1) {
					n, _ := strconv.Atoi(fm[1])
					p.TestsFailed += n
				}
				if sm := rePytestSkipped.FindStringSubmatch(trimmed); sm != nil {
					p.TestsSkipped, _ = strconv.Atoi(sm[1])
				}
			}
		}
	}
	return p
}

// extractOuterJSON 从混有日志前缀的文本中提取首个平衡且合法的 JSON 对象文本
// 请求体
// text 原始输出文本
// 响应体
// jsonStr 合法 JSON 对象文本（未找到返回空串）
func extractOuterJSON(text string) string {
	// 1. 使用引号与转义感知的括号平衡扫描器返回首个合法 JSON 对象
	objs := extractBalancedJSON(text, '{', '}')
	if len(objs) == 0 {
		return ""
	}
	return objs[0]
}

// extractOuterJSONArray 从混有日志前缀的文本中提取首个平衡且合法的 JSON 数组文本
// 请求体
// text 原始输出文本
// 响应体
// jsonArr 合法 JSON 数组文本（未找到返回空串）
func extractOuterJSONArray(text string) string {
	// 1. 使用引号与转义感知的括号平衡扫描器返回首个合法 JSON 数组
	arrs := extractBalancedJSON(text, '[', ']')
	if len(arrs) == 0 {
		return ""
	}
	return arrs[0]
}

// extractAllJSONObjects 提取输出文本中全部独立顶层 JSON 对象切片（兼容 NDJSON 与带日志前缀流）
// 请求体
// text 原始输出或 NDJSON 文本
// 响应体
// objs 合法顶层 JSON 对象文本切片
func extractAllJSONObjects(text string) []string {
	// 1. 扫描文本中的顶层平衡 JSON 对象
	return extractBalancedJSON(text, '{', '}')
}

// extractBalancedJSON 在含噪声文本中提取所有配对且通过 json.Valid 校验的顶层 JSON 片段
// 请求体
// text 待扫描原始文本
// openCh 起始界定符（'{' 或 '['）
// closeCh 结束界定符（'}' 或 ']'）
// 响应体
// spans 合法 JSON 片段切片
func extractBalancedJSON(text string, openCh, closeCh byte) []string {
	// 1. 按字符串转义边界追踪括号深度并收集所有有效顶级段落
	var spans []string
	data := []byte(text)
	for i := 0; i < len(data); i++ {
		if data[i] != openCh {
			continue
		}
		depth := 0
		inString := false
		escaped := false
		endIdx := -1
		for j := i; j < len(data); j++ {
			b := data[j]
			if inString {
				if escaped {
					escaped = false
					continue
				}
				if b == '\\' {
					escaped = true
					continue
				}
				if b == '"' {
					inString = false
				}
				continue
			}
			switch b {
			case '"':
				inString = true
			case openCh:
				depth++
			case closeCh:
				depth--
				if depth == 0 {
					endIdx = j
				}
			}
			if endIdx >= 0 {
				break
			}
		}
		if endIdx < 0 {
			continue
		}
		candidate := bytes.TrimSpace(data[i : endIdx+1])
		if json.Valid(candidate) {
			spans = append(spans, string(candidate))
			i = endIdx
		}
	}
	return spans
}

// floatPtr 构造64位浮点数指针以区分0值与未测量状态
// 请求体
// v 浮点数值
// 响应体
// ptr 指向副本的指针
func floatPtr(v float64) *float64 {
	// 1. 分配并返回局部变量副本指针
	val := v
	return &val
}
