package main

import (
	"fmt"
	"html"
	"strings"
)

// renderCatalogMarkdown 将26项代码级测试技术目录与8项覆盖率细分矩阵渲染为AI可读Markdown
// 请求体
// items 筛选后的测试技术规格列表
// covMetrics 覆盖率细分指标规格列表
// showCov 是否渲染8项覆盖率细分附表
// 响应体
// md 渲染完成的Markdown文本
func renderCatalogMarkdown(items []CodeTechniqueSpec, covMetrics []CoverageMetricSpec, showCov bool) string {
	// 1. 渲染目录总表包含序号、名称、优先级、适用语言、默认工具与失败标准
	var b strings.Builder
	fmt.Fprintf(&b, "## Code-Level Testing Catalog (%d techniques)\n\n", len(items))
	b.WriteString("| # | ID | 技术名称 | 优先级 | 首选工具 | 触发信号 | 失败标准 |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, spec := range items {
		toolDesc := "-"
		if len(spec.Tools) > 0 {
			toolDesc = spec.Tools[0].ToolName
		}
		mark := ""
		if spec.TopOverlooked {
			mark = " ★"
		}
		fmt.Fprintf(&b, "| %d | `%s` | %s (%s)%s | `%s` | %s | `%s` | %s |\n",
			spec.Index, spec.ID, spec.ChineseName, spec.EnglishName, mark,
			spec.Priority, toolDesc, strings.Join(spec.TriggerSignals, ", "), spec.FailureCriteria.Summary)
	}
	// 2. 按需渲染八项代码覆盖率细分指标矩阵
	if showCov && len(covMetrics) > 0 {
		b.WriteString("\n### 8 Code Coverage Metrics\n\n")
		b.WriteString("| ID | 英文名称 | 中文名称 | 核心定义 | Rust 支持说明 |\n")
		b.WriteString("|---|---|---|---|---|\n")
		for _, m := range covMetrics {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n",
				m.ID, m.EnglishName, m.ChineseName, m.Definition, m.RustSupport)
		}
	}
	return b.String()
}

// renderCodePlanMarkdown 将自动装配的26项代码级测试计划与真实风险信号渲染为Markdown
// 请求体
// plan 代码级测试计划结构体指针
// 响应体
// md 渲染完成的Markdown文本
func renderCodePlanMarkdown(plan *CodeVerifyPlan) string {
	// 1. 输出计划头部概览、统计计数与仓库扫描命中的真实代码风险出处
	var b strings.Builder
	fmt.Fprintf(&b, "## Code Verification Plan `%s` (`%s`, max-tier `%s`)\n\n", plan.PlanID, plan.PrimaryLanguage, plan.MaxTier)
	fmt.Fprintf(&b, "- Repo: `%s` · Plan SHA256: `%s`\n", plan.RepoRoot, plan.PlanSHA256)
	fmt.Fprintf(&b, "- Selection: **%d** REQUIRED · **%d** RECOMMENDED · **%d** DEFERRED · **%d** NOT_APPLICABLE\n\n",
		plan.RequiredCount, plan.RecommendedCount, plan.DeferredCount, plan.NotApplicableCount)

	if len(plan.Profile.RiskSignals) > 0 {
		b.WriteString("### Detected Repository Signals (Provenance)\n")
		for _, hit := range plan.Profile.RiskSignals {
			fmt.Fprintf(&b, "- `%s` at `%s:%d` — `%s`\n", hit.Signal, hit.File, hit.Line, hit.Evidence)
		}
		b.WriteString("\n")
	}

	// 2. 渲染按调度状态分组的技术计划明细表
	b.WriteString("### Planned Techniques Matrix\n\n")
	b.WriteString("| # | ID | 状态 | 优先级 | 工具 | 可用/夹具 | 命令与决策原因 |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, t := range plan.Techniques {
		tierLabel := string(t.EffectivePriority)
		if t.PromotedByRisk {
			tierLabel = fmt.Sprintf("%s (promoted from %s)", t.EffectivePriority, t.Priority)
		}
		readyLabel := fmt.Sprintf("tool=%v", t.ToolAvailable)
		if t.HarnessRequired {
			readyLabel += fmt.Sprintf(" harness=%v", t.HarnessReady)
		}
		detail := t.Reason
		if t.Command != "" {
			detail = fmt.Sprintf("`%s` — %s", t.Command, t.Reason)
		}
		fmt.Fprintf(&b, "| %d | `%s` (%s) | **%s** | `%s` | %s | %s | %s |\n",
			t.Index, t.TechniqueID, t.ChineseName, t.Status, tierLabel, nonEmpty(t.ToolName, "-"), readyLabel, detail)
	}
	return b.String()
}

// renderCodeExplainMarkdown 将代码测试审计台账与八项覆盖率明细渲染为Markdown
// 请求体
// rep 代码测试审计台账报告指针
// 响应体
// md 渲染完成的Markdown文本
func renderCodeExplainMarkdown(rep *CodeExplainReport) string {
	// 1. 输出计划头、门禁判定与三大易忽略测试状态
	var b strings.Builder
	runLabel := "NONE"
	if rep.LatestRun != nil {
		runLabel = rep.LatestRun.RunID
	}
	fmt.Fprintf(&b, "## Code Verification Ledger `%s` (run `%s`, verdict **%s**)\n\n", rep.PlanID, runLabel, rep.Gate.Verdict)
	fmt.Fprintf(&b, "- Language: `%s` · Historical FAILs Preserved: **%d** · Passed/Required: **%d/%d**\n",
		rep.PrimaryLanguage, rep.HistoricalFailCount, rep.Gate.PassedCount, rep.Gate.RequiredCount)
	fmt.Fprintf(&b, "- Top-3 Overlooked Status: mutation=`%s` · property=`%s` · fuzz=`%s`\n\n",
		rep.Gate.OverlookedStatus["mutation"], rep.Gate.OverlookedStatus["property"], rep.Gate.OverlookedStatus["fuzz"])

	// 2. 逐条渲染测试执行结果、关键度量与八项覆盖率细分详情
	for _, r := range rep.Results {
		fmt.Fprintf(&b, "### `%s` (%s / %s) — **%s** (%dms, exit=%d)\n",
			r.TechniqueID, r.Priority, r.ToolName, r.Verdict, r.DurationMS, r.ExitCode)
		fmt.Fprintf(&b, "- Cmd: `%s`\n", r.ExecutedCmd)
		if r.Metrics.TestsPassed > 0 || r.Metrics.TestsFailed > 0 || r.Metrics.TestsSkipped > 0 {
			fmt.Fprintf(&b, "- Tests: passed=%d · failed=%d · skipped=%d\n", r.Metrics.TestsPassed, r.Metrics.TestsFailed, r.Metrics.TestsSkipped)
		}
		if r.Metrics.MutationScorePct != nil {
			fmt.Fprintf(&b, "- Mutation: score=**%.1f%%** (caught=%d, missed=%d, timeout=%d, unviable=%d)\n",
				*r.Metrics.MutationScorePct, r.Metrics.MutantsCaught, r.Metrics.MutantsMissed, r.Metrics.MutantsTimeout, r.Metrics.MutantsUnviable)
		}
		if r.Metrics.Warnings > 0 || r.Metrics.HighVulns > 0 || r.Metrics.Crashes > 0 || r.Metrics.DataRaces > 0 || r.Metrics.UBErrors > 0 {
			fmt.Fprintf(&b, "- Diagnostics: warnings=%d · high_vulns=%d · crashes=%d · data_races=%d · ub_errors=%d\n",
				r.Metrics.Warnings, r.Metrics.HighVulns, r.Metrics.Crashes, r.Metrics.DataRaces, r.Metrics.UBErrors)
		}
		if r.Metrics.Coverage != nil {
			b.WriteString(renderCoverageBreakdownMarkdown(r.Metrics.Coverage))
		}
		if r.MissingPrerequisite != "" {
			fmt.Fprintf(&b, "- Missing Prerequisite: **%s**\n", r.MissingPrerequisite)
		}
		if r.FailureReason != "" {
			fmt.Fprintf(&b, "- Failure Reason: **%s**\n", r.FailureReason)
		}
		fmt.Fprintf(&b, "- Receipt SHA256: `%s` · Artifact: `%s` (`%s`)\n\n", r.ReceiptSHA256, r.ArtifactPath, r.ArtifactSHA256)
	}
	return b.String()
}

// renderCoverageBreakdownMarkdown 格式化八项覆盖率细分观测与未支持清单
// 请求体
// cov 八项代码覆盖率明细指针
// 响应体
// md 格式化的覆盖率清单文本
func renderCoverageBreakdownMarkdown(cov *CoverageBreakdown) string {
	// 1. 按固定八项顺序逐一输出真实数值或 NOT_MEASURED 声明
	var b strings.Builder
	fmt.Fprintf(&b, "- Coverage Breakdown (`%s`):\n", nonEmpty(cov.SourceTool, "coverage"))
	rows := []struct {
		label string
		ptr   *float64
	}{
		{"Line", cov.LinePct},
		{"Statement", cov.StatementPct},
		{"Function", cov.FunctionPct},
		{"Branch", cov.BranchPct},
		{"Condition", cov.ConditionPct},
		{"Decision", cov.DecisionPct},
		{"MC/DC", cov.MCDCPct},
		{"Path", cov.PathPct},
	}
	for _, row := range rows {
		if row.ptr != nil {
			fmt.Fprintf(&b, "  - %s: **%.1f%%**\n", row.label, *row.ptr)
		} else {
			fmt.Fprintf(&b, "  - %s: `NOT_MEASURED` (unsupported by current tool/flags)\n", row.label)
		}
	}
	return b.String()
}

// renderCodeGateMarkdown 将分层门禁报告渲染为简洁AI可读Markdown
// 请求体
// rep 代码测试门禁报告指针
// 响应体
// md 渲染完成的Markdown文本
func renderCodeGateMarkdown(rep *CodeGateReport) string {
	// 1. 组装门禁判定、计数统计与三大易忽略测试状态
	var b strings.Builder
	fmt.Fprintf(&b, "## Code Verify Gate `%s` (`%s`): **%s**\n", rep.PlanID, rep.MinTier, rep.Verdict)
	fmt.Fprintf(&b, "- Passed: **%d/%d** · Failed: **%d** · Blocked: **%d** · NotRun: **%d** · Historical FAILs: **%d**\n",
		rep.PassedCount, rep.RequiredCount, rep.FailedCount, rep.BlockedCount, rep.NotRunCount, rep.HistoricalFailCount)
	fmt.Fprintf(&b, "- Top-3 Overlooked: mutation=`%s`, property=`%s`, fuzz=`%s`\n",
		rep.OverlookedStatus["mutation"], rep.OverlookedStatus["property"], rep.OverlookedStatus["fuzz"])
	if len(rep.MissingTechniques) > 0 {
		fmt.Fprintf(&b, "- Pending / Failing: `%s`\n", strings.Join(rep.MissingTechniques, "`, `"))
	}
	if rep.Reason != "" {
		fmt.Fprintf(&b, "- Reason: %s\n", rep.Reason)
	}
	return b.String()
}

// renderCodeSnapshotHTML 渲染内嵌UTF-8声明与CSP安全策略的26项代码测试工作台HTML页面
// 请求体
// doc 快照完整数据文档
// snapshotJSON 序列化后的snapshot.json字节流
// 响应体
// htmlPage 完整静态HTML字符串
func renderCodeSnapshotHTML(doc codeSnapshotDocument, snapshotJSON []byte) string {
	// 1. 写入首1024字节内的 <meta charset="UTF-8"> 与 CSP 头
	safeJSON := strings.ReplaceAll(string(snapshotJSON), "</script>", `<\/script>`)
	resByTech := map[string]CodeResultRecord{}
	var latestCov *CoverageBreakdown
	for _, r := range doc.Results {
		resByTech[r.TechniqueID] = r
		if r.Metrics.Coverage != nil {
			latestCov = r.Metrics.Coverage
		}
	}

	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"zh-CN\">\n<head>\n")
	b.WriteString("<meta charset=\"UTF-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n")
	b.WriteString(snapshotCSP + "\n")
	fmt.Fprintf(&b, "<title>Arc Code Verify Workbench — %s</title>\n", html.EscapeString(doc.Plan.PlanID))
	b.WriteString(`<style>
body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,sans-serif;margin:0;background:#0b1120;color:#e2e8f0}
header{padding:16px 24px;border-bottom:1px solid #334155;display:flex;justify-content:space-between;align-items:center}
.badge{padding:4px 10px;border-radius:999px;font-weight:700;font-size:12px}
.PASS,.REQUIRED{background:#14532d;color:#86efac}.FAIL{background:#7f1d1d;color:#fca5a5}.BLOCKED,.NOT_RUN,.RECOMMENDED{background:#78350f;color:#fde68a}.DEFERRED,.NOT_APPLICABLE{background:#334155;color:#cbd5e1}
main{display:grid;grid-template-columns:300px 1fr 320px;gap:16px;padding:16px}
section{background:#1e293b;border:1px solid #334155;border-radius:10px;padding:14px}
h2{margin:0 0 10px;font-size:13px;color:#93c5fd;text-transform:uppercase;letter-spacing:.04em}
.card{background:#0f172a;border:1px solid #334155;border-radius:8px;padding:10px;margin-bottom:8px;font-size:13px}
.grid4{display:grid;grid-template-columns:repeat(4,1fr);gap:8px;margin-bottom:12px}
.metric{background:#0f172a;border:1px solid #334155;border-radius:8px;padding:10px;text-align:center}
.metric b{font-size:18px;display:block;color:#38bdf8}
code{color:#f8fafc;background:#334155;padding:1px 5px;border-radius:4px}
@media(max-width:960px){main{grid-template-columns:1fr}}
</style>`)
	b.WriteString("\n</head>\n<body>\n")
	fmt.Fprintf(&b, "<header><div><strong>%s</strong> · lang <code>%s</code> · tier <code>%s</code></div><span class=\"badge %s\">%s</span></header>\n",
		html.EscapeString(doc.Plan.PlanID), html.EscapeString(doc.Plan.PrimaryLanguage), html.EscapeString(string(doc.Gate.MinTier)),
		html.EscapeString(doc.Gate.Verdict), html.EscapeString(doc.Gate.Verdict))
	b.WriteString("<main>\n")

	// 2. 左栏渲染仓库风险信号出处与八项覆盖率明细
	b.WriteString("<section><h2>8 Coverage Metrics</h2>\n")
	for _, cm := range doc.CoverageMetrics {
		valText := "NOT_MEASURED"
		if latestCov != nil {
			if ptr := coverageFieldPtr(latestCov, cm.ID); ptr != nil {
				valText = fmt.Sprintf("%.1f%%", *ptr)
			}
		}
		fmt.Fprintf(&b, "<div class=\"card\"><strong>%s</strong> (%s)<br><code>%s</code></div>\n",
			html.EscapeString(cm.ChineseName), html.EscapeString(cm.EnglishName), html.EscapeString(valText))
	}
	b.WriteString("<h2>Repository Risk Signals</h2>\n")
	for _, sig := range doc.Plan.Profile.RiskSignals {
		fmt.Fprintf(&b, "<div class=\"card\"><code>%s</code><br><small>%s:%d</small></div>\n",
			html.EscapeString(sig.Signal), html.EscapeString(sig.File), sig.Line)
	}
	b.WriteString("</section>\n")

	// 3. 中栏渲染 KPI 与26项测试技术矩阵
	b.WriteString("<section><h2>26 Code-Level Techniques Matrix</h2>\n")
	fmt.Fprintf(&b, `<div class="grid4">
<div class="metric"><b>%d/%d</b>Passed</div>
<div class="metric"><b>%d</b>Failed</div>
<div class="metric"><b>%d</b>Blocked</div>
<div class="metric"><b>%d</b>Hist FAILs</div>
</div>`, doc.Gate.PassedCount, doc.Gate.RequiredCount, doc.Gate.FailedCount, doc.Gate.BlockedCount, doc.Gate.HistoricalFailCount)
	for _, t := range doc.Plan.Techniques {
		badge := string(t.Status)
		if rec, ok := resByTech[t.TechniqueID]; ok {
			badge = rec.Verdict
		}
		fmt.Fprintf(&b, "<div class=\"card\"><div style=\"display:flex;justify-content:space-between\"><strong>#%d %s (%s)</strong><span class=\"badge %s\">%s</span></div>",
			t.Index, html.EscapeString(t.ChineseName), html.EscapeString(t.TechniqueID), html.EscapeString(badge), html.EscapeString(badge))
		fmt.Fprintf(&b, "<div><small>tier: <code>%s</code> · tool: <code>%s</code></small></div>",
			html.EscapeString(string(t.EffectivePriority)), html.EscapeString(nonEmpty(t.ToolName, "-")))
		if t.Command != "" {
			fmt.Fprintf(&b, "<div><small><code>%s</code></small></div>", html.EscapeString(t.Command))
		}
		b.WriteString("</div>\n")
	}
	b.WriteString("</section>\n")

	// 4. 右栏渲染三大易忽略测试聚焦面板
	b.WriteString("<section><h2>Top 3 Overlooked Tests</h2>\n")
	for _, key := range []string{"mutation", "property", "fuzz"} {
		st := doc.Gate.OverlookedStatus[key]
		if st == "" {
			st = "NOT_RUN"
		}
		fmt.Fprintf(&b, "<div class=\"card\" style=\"display:flex;justify-content:space-between\"><code>%s</code><span class=\"badge %s\">%s</span></div>\n",
			key, html.EscapeString(st), html.EscapeString(st))
	}
	b.WriteString("</section>\n</main>\n")
	b.WriteString(`<script type="application/json" id="arc-code-snapshot">` + safeJSON + "</script>\n")
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// coverageFieldPtr 按 CoverageMetricKind 枚举返回对应覆盖率百分比字段指针
// 请求体
// cov 覆盖率细分结构指针
// kind 目标指标枚举
// 响应体
// ptr 对应的浮点百分比指针
func coverageFieldPtr(cov *CoverageBreakdown, kind CoverageMetricKind) *float64 {
	// 1. 按闭合枚举映射结构体字段指针
	switch kind {
	case CovLine:
		return cov.LinePct
	case CovStatement:
		return cov.StatementPct
	case CovFunction:
		return cov.FunctionPct
	case CovBranch:
		return cov.BranchPct
	case CovCondition:
		return cov.ConditionPct
	case CovDecision:
		return cov.DecisionPct
	case CovMCDC:
		return cov.MCDCPct
	case CovPath:
		return cov.PathPct
	default:
		return nil
	}
}
