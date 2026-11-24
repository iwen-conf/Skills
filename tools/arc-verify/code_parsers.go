package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

const codeMetricPrefix = "[ARC_CODE_METRIC]"

// llvmCovStat 描述 cargo-llvm-cov JSON 中单个覆盖率子项的统计结构
type llvmCovStat struct {
	// Count 插桩元素总数指针
	Count *int64 `json:"count"`
	// Covered 已覆盖数量指针
	Covered *int64 `json:"covered"`
	// Percent 覆盖百分比指针
	Percent *float64 `json:"percent"`
}

// llvmCovDocument 描述 cargo-llvm-cov --summary-only --json 的强类型输出文档
type llvmCovDocument struct {
	// Data 汇总数据集列表
	Data []struct {
		// Totals 全局聚合覆盖率汇总对象
		Totals struct {
			// Lines 行覆盖统计
			Lines *llvmCovStat `json:"lines"`
			// Functions 函数覆盖统计
			Functions *llvmCovStat `json:"functions"`
			// Regions 区域（语句近似）覆盖统计
			Regions *llvmCovStat `json:"regions"`
			// Branches 分支覆盖统计
			Branches *llvmCovStat `json:"branches"`
			// MCDC 修正条件判定覆盖统计
			MCDC *llvmCovStat `json:"mcdc"`
		} `json:"totals"`
	} `json:"data"`
}

// pytestCovDocument 描述 pytest-cov 导出的 coverage.json 结构
type pytestCovDocument struct {
	// Totals 全局统计信息对象
	Totals struct {
		// PercentCovered 行覆盖率百分比
		PercentCovered *float64 `json:"percent_covered"`
		// CoveredLines 已覆盖行数
		CoveredLines int `json:"covered_lines"`
		// NumStatements 总语句数
		NumStatements int `json:"num_statements"`
		// CoveredBranches 已覆盖分支数
		CoveredBranches int `json:"covered_branches"`
		// NumBranches 总分支数
		NumBranches int `json:"num_branches"`
	} `json:"totals"`
}

// istanbulSummaryDocument 描述 vitest/istanbul coverage-summary.json 结构
type istanbulSummaryDocument struct {
	// Total 全量汇总覆盖率映射
	Total struct {
		// Lines 行覆盖统计
		Lines *struct {
			// Pct 覆盖百分比
			Pct *float64 `json:"pct"`
		} `json:"lines"`
		// Statements 语句覆盖统计
		Statements *struct {
			// Pct 覆盖百分比
			Pct *float64 `json:"pct"`
		} `json:"statements"`
		// Functions 函数覆盖统计
		Functions *struct {
			// Pct 覆盖百分比
			Pct *float64 `json:"pct"`
		} `json:"functions"`
		// Branches 分支覆盖统计
		Branches *struct {
			// Pct 覆盖百分比
			Pct *float64 `json:"pct"`
		} `json:"branches"`
	} `json:"total"`
}

// cargoAuditDocument 描述 cargo audit --json 的强类型漏洞报告结构
type cargoAuditDocument struct {
	// Vulnerabilities 漏洞列表包装结构
	Vulnerabilities struct {
		// Found 是否发现漏洞
		Found bool `json:"found"`
		// Count 漏洞数量
		Count int `json:"count"`
		// List 漏洞明细数组
		List []struct {
			// Advisory 安全公告描述
			Advisory struct {
				// ID 漏洞公告ID
				ID string `json:"id"`
			} `json:"advisory"`
		} `json:"list"`
	} `json:"vulnerabilities"`
}

// govulncheckEventDocument 描述 govulncheck -json 输出的流式 NDJSON 发现条目
type govulncheckEventDocument struct {
	// Finding 实测命中的漏洞调用链对象
	Finding *struct {
		// OSV Go 漏洞公告编号
		OSV string `json:"osv"`
	} `json:"finding"`
}

// npmAuditDocument 描述 npm audit --json 的统计汇总结构
type npmAuditDocument struct {
	// Metadata 审计元数据容器
	Metadata *struct {
		// Vulnerabilities 按严重程度分桶的漏洞计数
		Vulnerabilities struct {
			// Low 低危漏洞数
			Low int `json:"low"`
			// Moderate 中危漏洞数
			Moderate int `json:"moderate"`
			// High 高危漏洞数
			High int `json:"high"`
			// Critical 严重漏洞数
			Critical int `json:"critical"`
		} `json:"vulnerabilities"`
	} `json:"metadata"`
}

// pipAuditDependency 描述 pip-audit -f json 单项依赖及关联漏洞列表
type pipAuditDependency struct {
	// Name 依赖包名
	Name string `json:"name"`
	// Vulns 命中漏洞明细数组
	Vulns []struct {
		// ID 漏洞编号
		ID string `json:"id"`
	} `json:"vulns"`
}

// pipAuditWrapperDocument 描述新版 pip-audit 包装结构
type pipAuditWrapperDocument struct {
	// Dependencies 依赖审计条目数组
	Dependencies []pipAuditDependency `json:"dependencies"`
}

// ruffDiagnosticItem 描述 ruff check --output-format json 单条诊断记录
type ruffDiagnosticItem struct {
	// Code Ruff 规则编号
	Code string `json:"code"`
}

// semgrepDocument 描述 semgrep --json 的强类型扫描报告结构
type semgrepDocument struct {
	// Results 命中的安全规则条目列表
	Results []struct {
		// Extra 扩展元数据
		Extra struct {
			// Severity 规则严重级别
			Severity string `json:"severity"`
		} `json:"extra"`
	} `json:"results"`
}

// cargoMutantsOutcomesDocument 描述 cargo-mutants outcomes.json 结构
type cargoMutantsOutcomesDocument struct {
	// Outcomes 单个变异体执行结果列表
	Outcomes []struct {
		// Summary 变异体摘要结果（CaughtMutant | MissedMutant | Timeout | Unviable）
		Summary string `json:"summary"`
	} `json:"outcomes"`
}

// strykerReportDocument 描述 Stryker mutation.json 的单文件变异结果映射
type strykerReportDocument struct {
	// Files 按源码文件分组的变异体检测结果
	Files map[string]struct {
		// Mutants 单个文件内的变异体列表
		Mutants []struct {
			// Status 变异体状态（Killed | Survived | NoCoverage | Timeout | CompileError）
			Status string `json:"status"`
		} `json:"mutants"`
	} `json:"files"`
}

// gremlinsSummaryDocument 描述 gremlins JSON 汇总结构
type gremlinsSummaryDocument struct {
	// Killed 已杀死的变异体数
	Killed int `json:"killed"`
	// Lived 逃逸存活的变异体数
	Lived int `json:"lived"`
	// TimedOut 超时变异体数
	TimedOut int `json:"timed_out"`
	// NotViable 不可编译变异体数
	NotViable int `json:"not_viable"`
}

// parseTechniqueOutput 从原生工具日志、产物文件及 [ARC_CODE_METRIC] 协议行解析强类型指标载荷
// 请求体
// techID 测试技术唯一标识
// lang 目标编程语言
// toolName 执行工具名
// output 命令标准输出与错误合并文本
// artifactDir 当前批次产物目录路径
// 响应体
// payload 解析并合并完成的强类型测试度量指标
func parseTechniqueOutput(techID, lang, toolName, output, artifactDir string) CodeMetricPayload {
	// 1. 优先扫描 [ARC_CODE_METRIC] 协议行获取结构化指标
	var payload CodeMetricPayload
	for _, line := range strings.Split(output, "\n") {
		idx := strings.Index(line, codeMetricPrefix)
		if idx < 0 {
			continue
		}
		rawJSON := strings.TrimSpace(line[idx+len(codeMetricPrefix):])
		var item CodeMetricPayload
		if json.Unmarshal([]byte(rawJSON), &item) == nil {
			payload = mergeCodeMetricPayloads(payload, item)
		}
	}

	// 2. 按技术类型与工具输出继续提取原生报告数字（仅补齐未显式声明字段）
	var native CodeMetricPayload
	switch techID {
	case "coverage":
		native = parseNativeCoverage(lang, toolName, output, artifactDir)
	case "mutation":
		native = parseNativeMutation(output, artifactDir)
	case "dependency_audit", "sast":
		native = parseNativeSecurityAudit(techID, output)
	case "static_analysis":
		native = parseNativeStaticWarnings(lang, output)
	case "data_race", "memory_safety", "concurrency":
		native = parseNativeConcurrencyAndMemory(output)
		testStats := parseNativeTestCounts(output)
		native.TestsPassed = testStats.TestsPassed
		native.TestsFailed = testStats.TestsFailed
		native.TestsSkipped = testStats.TestsSkipped
	case "fuzz":
		native = parseNativeFuzz(output)
	default:
		native = parseNativeTestCounts(output)
	}
	payload = mergeCodeMetricPayloads(native, payload)
	if payload.Coverage != nil {
		finalizeCoverageBreakdown(payload.Coverage)
	}
	return payload
}

// evaluateCodeFailureCriteria 依据失败门槛、退出码与观测指标计算单项测试最终判定（缺失强制阈值指标判BLOCKED）
// 请求体
// criteria 失败门槛契约
// exitCode 进程退出码
// metrics 解析得到的强类型指标载荷
// 响应体
// verdict 判定枚举（PASS | FAIL | BLOCKED）
// reason 失败或阻断详细说明（通过时为空）
func evaluateCodeFailureCriteria(criteria CodeFailureCriteria, exitCode int, metrics CodeMetricPayload) (Verdict, string) {
	// 1. 依次核对退出码、失败计数、覆盖率、变异杀死率、漏洞、竞争、UB 与性能回归阈值
	var reasons []string
	var blockedReasons []string
	if criteria.FailOnNonZeroExit && exitCode != 0 {
		reasons = append(reasons, fmt.Sprintf("command exited with code %d", exitCode))
	}
	if metrics.TestsFailed > criteria.MaxFailures {
		reasons = append(reasons, fmt.Sprintf("tests_failed=%d exceeds max_failures=%d", metrics.TestsFailed, criteria.MaxFailures))
	}
	if criteria.MinLineCoveragePct > 0 {
		var lineCov *float64
		if metrics.Coverage != nil {
			lineCov = metrics.Coverage.LinePct
			if lineCov == nil {
				lineCov = metrics.Coverage.StatementPct
			}
		}
		if lineCov == nil {
			blockedReasons = append(blockedReasons, "line/statement coverage metric not measured")
		} else if *lineCov < criteria.MinLineCoveragePct {
			reasons = append(reasons, fmt.Sprintf("line_pct=%.1f%% below min=%.1f%%", *lineCov, criteria.MinLineCoveragePct))
		}
	}
	if criteria.MinBranchCoveragePct > 0 {
		if metrics.Coverage == nil || metrics.Coverage.BranchPct == nil {
			blockedReasons = append(blockedReasons, "branch coverage metric not measured")
		} else if *metrics.Coverage.BranchPct < criteria.MinBranchCoveragePct {
			reasons = append(reasons, fmt.Sprintf("branch_pct=%.1f%% below min=%.1f%%", *metrics.Coverage.BranchPct, criteria.MinBranchCoveragePct))
		}
	}
	if criteria.MinMutationScorePct > 0 {
		if metrics.MutationScorePct == nil {
			blockedReasons = append(blockedReasons, "mutation_score_pct metric not measured")
		} else if *metrics.MutationScorePct < criteria.MinMutationScorePct {
			reasons = append(reasons, fmt.Sprintf("mutation_score=%.1f%% below min=%.1f%%", *metrics.MutationScorePct, criteria.MinMutationScorePct))
		}
	}
	if criteria.EnforceMaxWarnings && metrics.Warnings > criteria.MaxWarnings {
		reasons = append(reasons, fmt.Sprintf("warnings=%d exceeds max=%d", metrics.Warnings, criteria.MaxWarnings))
	}
	if metrics.HighVulns+metrics.CriticalVulns > criteria.MaxHighVulns {
		reasons = append(reasons, fmt.Sprintf("high/critical vulns=%d exceeds max=%d", metrics.HighVulns+metrics.CriticalVulns, criteria.MaxHighVulns))
	}
	if metrics.Crashes > criteria.MaxCrashes {
		reasons = append(reasons, fmt.Sprintf("crashes=%d exceeds max=%d", metrics.Crashes, criteria.MaxCrashes))
	}
	if metrics.DataRaces > criteria.MaxDataRaces {
		reasons = append(reasons, fmt.Sprintf("data_races=%d exceeds max=%d", metrics.DataRaces, criteria.MaxDataRaces))
	}
	if metrics.UBErrors > criteria.MaxUBErrors {
		reasons = append(reasons, fmt.Sprintf("ub_errors=%d exceeds max=%d", metrics.UBErrors, criteria.MaxUBErrors))
	}
	if metrics.ProofsFailed > 0 {
		reasons = append(reasons, fmt.Sprintf("proofs_failed=%d > 0", metrics.ProofsFailed))
	}
	if criteria.MaxRegressionPct > 0 && metrics.BenchRegressionPct != nil && *metrics.BenchRegressionPct > criteria.MaxRegressionPct {
		reasons = append(reasons, fmt.Sprintf("bench_regression=%.1f%% exceeds max=%.1f%%", *metrics.BenchRegressionPct, criteria.MaxRegressionPct))
	}
	// 2. 确定性失败优先返回 FAIL；仅当强制阈值维度未产出真实度量时判 BLOCKED
	if len(reasons) > 0 {
		return VerdictFail, strings.Join(reasons, "; ")
	}
	if len(blockedReasons) > 0 {
		return VerdictBlocked, strings.Join(blockedReasons, "; ")
	}
	return VerdictPass, ""
}
