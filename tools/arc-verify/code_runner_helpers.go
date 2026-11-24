package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// newCodeRunID 生成带 CV- 前缀的加密随机代码测试运行批次ID
// 请求体
// 无
// 响应体
// id CV-前缀的16位十六进制运行ID
func newCodeRunID() string {
	// 1. 生成8字节安全随机数并编码为十六进制运行批次ID
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "CV-" + hex.EncodeToString(b[:])
}

// selectRunnableTechniques 根据过滤清单与工具探测缓存筛选并补全本次运行的目标测试技术
// 请求体
// plan 代码测试计划指针
// opts 运行控制选项
// cache 工具探针缓存指针
// 响应体
// selected 待执行计划条目切片
// err 无可运行技术或包含未知技术标识错误
func selectRunnableTechniques(plan *CodeVerifyPlan, opts CodeRunOptions, cache *toolProbeCache) ([]PlannedTechnique, error) {
	// 1. 构建按技术ID索引的计划映射
	byID := make(map[string]PlannedTechnique, len(plan.Techniques))
	for _, t := range plan.Techniques {
		byID[t.TechniqueID] = t
	}
	// 2. 若用户显式指定技术子集，优先读取计划条目否则从标准目录与工具缓存动态补齐适配器
	var selected []PlannedTechnique
	if len(opts.Techniques) > 0 {
		seen := map[string]bool{}
		for _, rawID := range opts.Techniques {
			id := strings.ToLower(strings.TrimSpace(rawID))
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			if item, ok := byID[id]; ok {
				selected = append(selected, item)
				continue
			}
			if spec, ok := findCodeTechnique(id); ok {
				selected = append(selected, hydrateAdHocTechnique(spec, plan, cache))
				continue
			}
			return nil, fmt.Errorf("unknown code testing technique %q", rawID)
		}
		return selected, nil
	}
	for _, t := range plan.Techniques {
		if t.Status == SelectRequired || (opts.IncludeRecommended && t.Status == SelectRecommended) {
			selected = append(selected, t)
		}
	}
	if len(selected) == 0 {
		return nil, errors.New("no runnable techniques selected in code verify plan")
	}
	return selected, nil
}

// hydrateAdHocTechnique 为计划外通过 --technique 临时指定的测试技术装配适配器、命令与夹具状态
// 请求体
// spec 标准目录中的技术规格
// plan 当前代码验证计划指针
// cache 工具探针缓存指针
// 响应体
// item 装配完成的可执行技术条目
func hydrateAdHocTechnique(spec CodeTechniqueSpec, plan *CodeVerifyPlan, cache *toolProbeCache) PlannedTechnique {
	// 1. 基于主语言匹配适配器并解析动态回退命令与工具可用状态
	item := PlannedTechnique{
		Index:             spec.Index,
		TechniqueID:       spec.ID,
		ChineseName:       spec.ChineseName,
		EnglishName:       spec.EnglishName,
		Priority:          spec.Priority,
		EffectivePriority: spec.Priority,
		TopOverlooked:     spec.TopOverlooked,
		Status:            SelectRequired,
		Language:          plan.PrimaryLanguage,
		FailureCriteria:   spec.FailureCriteria,
		ReportFormats:     spec.ReportFormats,
		EvidencePath:      spec.EvidenceStorage.ArtifactDirTemplate,
		Reason:            "explicitly requested by --technique",
	}
	adapter, ok := selectToolAdapter(spec, plan.PrimaryLanguage)
	if !ok {
		item.ToolAvailable = false
		item.MissingPrerequisite = fmt.Sprintf("technique %s does not target %s", spec.ID, plan.PrimaryLanguage)
		return item
	}
	item.ToolName = adapter.ToolName
	item.Command = adapter.DefaultCmd
	item.InstallHint = adapter.InstallHint
	item.HarnessRequired = adapter.RequiresHarness
	avail, _ := probeAdapterAvailability(adapter, cache)
	item.Command, item.ToolName, item.ToolAvailable = resolveDynamicToolCommand(spec.ID, plan.PrimaryLanguage, adapter, &plan.Profile, cache, avail)
	if !adapter.RequiresHarness {
		item.HarnessReady = true
	} else {
		item.HarnessReady = len(plan.Profile.DetectedHarnesses[adapter.HarnessKey]) > 0
		if !item.HarnessReady && spec.ID == "property" && len(plan.Profile.DetectedHarnesses["proptest"]) > 0 {
			item.HarnessReady = true
		}
		if !item.HarnessReady {
			item.MissingPrerequisite = adapter.HarnessHint
		}
	}
	return item
}

// verifyLiveCommandReady 在执行前二次复核命令行首程序、复合Shell脚本底层适配器及Cargo/Python环境是否真实可用
// 请求体
// cmdStr 待执行Shell命令串
// tech 计划技术条目
// cache 工具探针缓存指针
// 响应体
// ready 命令所需二进制与子命令是否就绪
// reason 未就绪原因说明
func verifyLiveCommandReady(cmdStr string, tech PlannedTechnique, cache *toolProbeCache) (bool, string) {
	// 1. 提取剥离环境变量前缀后的可执行程序名与紧邻参数
	prog, subArgs := extractCommandExecutable(cmdStr)
	if prog == "" {
		return false, "empty command"
	}
	// 2. 若命令以复合 Shell 关键字或 test 方括号起始，则直接探测所属目录适配器的底层二进制
	if prog == "sh" || prog == "bash" || prog == "for" || prog == "if" || prog == "[" {
		if spec, ok := findCodeTechnique(tech.TechniqueID); ok {
			if adapter, okA := selectToolAdapter(spec, tech.Language); okA {
				return probeAdapterAvailability(adapter, cache)
			}
		}
		return true, ""
	}
	if _, err := exec.LookPath(prog); err != nil {
		return false, prog + " not found on PATH"
	}
	if prog == "cargo" && cache != nil {
		if len(subArgs) > 0 && subArgs[0] == "+nightly" {
			if !cache.nightlyReady {
				return false, "rustup nightly toolchain not installed"
			}
			subArgs = subArgs[1:]
		}
		if len(subArgs) > 0 {
			switch subArgs[0] {
			case "llvm-cov", "fuzz", "mutants", "nextest", "audit", "deny", "clippy", "kani", "hack":
				if !cache.cargoSubcmds[subArgs[0]] {
					return false, "cargo subcommand '" + subArgs[0] + "' not installed"
				}
			case "miri":
				if !cache.miriReady {
					return false, "rustup nightly miri component not installed"
				}
			}
		}
	}
	return true, ""
}

// extractCommandExecutable 从可能带 KEY=VAL 前缀的Shell命令中提取首执行程序及子参数
// 请求体
// cmdStr 原始Shell命令文本
// 响应体
// prog 首个命令可执行文件名
// subArgs 紧随其后的前两个参数词元
func extractCommandExecutable(cmdStr string) (prog string, subArgs []string) {
	// 1. 按空白拆分词元并跳过合法环境变量赋值前缀（含带分号赋值语句如 found=0;）
	fields := strings.Fields(cmdStr)
	idx := 0
	for idx < len(fields) && strings.Contains(fields[idx], "=") && !strings.HasPrefix(fields[idx], "-") {
		idx++
	}
	if idx >= len(fields) {
		return "", nil
	}
	prog = strings.TrimRight(fields[idx], ";")
	for j := idx + 1; j < len(fields) && len(subArgs) < 2; j++ {
		subArgs = append(subArgs, fields[j])
	}
	return prog, subArgs
}

// isTestExecutionTechnique 判断技术标识是否属于应受“零用例即阻断”约束的测试执行类目
// 请求体
// techID 测试技术标识
// 响应体
// isTestExec 若属于测试执行类目返回true
func isTestExecutionTechnique(techID string) bool {
	// 1. 覆盖全量断言类、性质类、文档与兼容性测试运行器
	switch techID {
	case "unit", "integration", "regression", "doctest",
		"property", "snapshot", "differential", "metamorphic",
		"boundary", "parameterized", "combinatorial",
		"concurrency", "stress", "soak", "model_based", "compatibility":
		return true
	default:
		return false
	}
}

// isZeroTestsResult 综合退出码（含 pytest exit=5）、解析计数与框架空跑提示判定零用例阻断状态
// 请求体
// techID 测试技术标识
// exitCode 进程退出状态码
// metrics 解析得到的测试度量载荷
// output 原始命令合并输出
// 响应体
// blocked 零用例未真正执行返回true
func isZeroTestsResult(techID string, exitCode int, metrics CodeMetricPayload, output string) bool {
	// 1. 仅约束测试执行类技术；非测试类检查（静态/依赖/SAST等）交由各自门槛处理
	if !isTestExecutionTechnique(techID) {
		return false
	}
	low := strings.ToLower(output)
	if exitCode == 5 && (strings.Contains(low, "collected 0 items") || strings.Contains(low, "no tests ran")) {
		return true
	}
	if exitCode == 0 && metrics.TestsPassed == 0 && metrics.TestsFailed == 0 && hasZeroTestsRanNotice(output) {
		return true
	}
	return false
}

// hasZeroTestsRanNotice 判断测试输出是否明确表示未执行任何用例
// 请求体
// output 测试输出文本
// 响应体
// zero 是否未执行任何测试用例
func hasZeroTestsRanNotice(output string) bool {
	// 1. 匹配 Rust/Go/Pytest/Vitest/CTest 常见零用例空跑提示
	low := strings.ToLower(output)
	return (strings.Contains(low, "running 0 tests") && !strings.Contains(low, "1 passed") && !strings.Contains(low, "tests passed")) ||
		(strings.Contains(low, "[no test files]") && !strings.Contains(low, "--- pass:")) ||
		strings.Contains(low, "no tests to run") ||
		strings.Contains(low, "collected 0 items") ||
		strings.Contains(low, "no test files found") ||
		strings.Contains(low, "no tests were found")
}

// truncateCodeOutput 按字节上限截断过长日志并保留尾部摘要可见
// 请求体
// s 原始日志字符串
// maxBytes 允许保留的最大字节数
// 响应体
// out 截断后的日志字符串
func truncateCodeOutput(s string, maxBytes int) string {
	// 1. 超出上限时保留头尾并插入截断提示
	if len(s) <= maxBytes || maxBytes < 256 {
		return s
	}
	head := maxBytes / 2
	tail := maxBytes - head
	return s[:head] + "\n...[truncated by arc-verify]...\n" + s[len(s)-tail:]
}

// computeCodeResultSHA256 清空自身摘要字段后计算单项测试收据的确定性SHA256指纹
// 请求体
// rec 代码测试收据结构体副本
// 响应体
// digest 64位十六进制SHA256摘要
func computeCodeResultSHA256(rec CodeResultRecord) string {
	// 1. 置空自增ID与自身哈希字段后序列化计算摘要
	rec.ID = 0
	rec.ReceiptSHA256 = ""
	raw, _ := json.Marshal(rec)
	return sha256Hex(raw)
}
