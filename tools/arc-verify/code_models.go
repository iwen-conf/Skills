package main

import "strings"

// CodePriorityTier 表示代码级测试技术按风险与阶段划分的四级优先级枚举
type CodePriorityTier string

const (
	TierUnspecified CodePriorityTier = ""
	TierP0          CodePriorityTier = "P0"
	TierP1          CodePriorityTier = "P1"
	TierP2          CodePriorityTier = "P2"
	TierP3          CodePriorityTier = "P3"
)

// tierRank 返回优先级枚举对应的数值序位以支持比较与门禁筛选
// 请求体
// t 优先级枚举值
// 响应体
// rank 数值序位（P0=0..P3=3，未知返回99）
func tierRank(t CodePriorityTier) int {
	// 1. 按闭合优先级枚举映射为可排序序位
	switch t {
	case TierP0:
		return 0
	case TierP1:
		return 1
	case TierP2:
		return 2
	case TierP3:
		return 3
	default:
		return 99
	}
}

// parsePriorityTier 将文本输入解析为闭合的 CodePriorityTier 枚举值
// 请求体
// raw 原始优先级文本
// 响应体
// tier 解析得到的优先级枚举
// ok 输入是否为合法优先级标识
func parsePriorityTier(raw string) (CodePriorityTier, bool) {
	// 1. 统一规范化大小写并匹配 P0-P3 闭合枚举
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "P0":
		return TierP0, true
	case "P1":
		return TierP1, true
	case "P2":
		return TierP2, true
	case "P3":
		return TierP3, true
	default:
		return TierUnspecified, false
	}
}

// CodeCategory 表示代码级测试技术的工程分类枚举
type CodeCategory string

const (
	CategoryFunctional      CodeCategory = "functional"
	CategoryCoverage        CodeCategory = "coverage"
	CategoryDefectDiscovery CodeCategory = "defect_discovery"
	CategoryCorrectness     CodeCategory = "correctness"
	CategoryPerformance     CodeCategory = "performance"
	CategoryStaticSecurity  CodeCategory = "static_security"
	CategoryFormalModel     CodeCategory = "formal_model"
	CategoryDocsCompat      CodeCategory = "docs_compat"
)

// CodeSelectionStatus 表示计划阶段对单项测试技术的自动调度决策状态
type CodeSelectionStatus string

const (
	SelectRequired      CodeSelectionStatus = "REQUIRED"
	SelectRecommended   CodeSelectionStatus = "RECOMMENDED"
	SelectDeferred      CodeSelectionStatus = "DEFERRED"
	SelectNotApplicable CodeSelectionStatus = "NOT_APPLICABLE"
)

// CoverageMetricKind 表示代码覆盖率细分的八项闭合指标枚举
type CoverageMetricKind string

const (
	CovLine      CoverageMetricKind = "line"
	CovStatement CoverageMetricKind = "statement"
	CovFunction  CoverageMetricKind = "function"
	CovBranch    CoverageMetricKind = "branch"
	CovCondition CoverageMetricKind = "condition"
	CovDecision  CoverageMetricKind = "decision"
	CovMCDC      CoverageMetricKind = "mcdc"
	CovPath      CoverageMetricKind = "path"
)

// MetricDirection 表示度量指标的判优方向枚举
type MetricDirection string

const (
	DirHigherIsBetter MetricDirection = "higher_is_better"
	DirLowerIsBetter  MetricDirection = "lower_is_better"
	DirExactZero      MetricDirection = "exact_zero"
	DirInformational  MetricDirection = "informational"
)

// CoverageMetricSpec 描述八项代码覆盖率细分指标的语义定义与多语言工具支持矩阵
type CoverageMetricSpec struct {
	// ID 覆盖率指标闭合枚举标识
	ID CoverageMetricKind `yaml:"id" json:"id"`
	// EnglishName 指标英文标准名称
	EnglishName string `yaml:"english_name" json:"english_name"`
	// ChineseName 指标中文标准名称
	ChineseName string `yaml:"chinese_name" json:"chinese_name"`
	// Definition 指标度量口径与工程含义
	Definition string `yaml:"definition" json:"definition"`
	// RustSupport Rust工具链支持说明
	RustSupport string `yaml:"rust_support" json:"rust_support"`
	// MultiLangTools 按语言索引的支持工具说明映射
	MultiLangTools map[string]string `yaml:"multi_lang_tools" json:"multi_lang_tools"`
}

// CodeToolAdapter 描述某项测试技术在特定编程语言下的真实执行工具与探针契约
type CodeToolAdapter struct {
	// Language 目标编程语言标识（rust | go | typescript | python | c_cpp | multi）
	Language string `yaml:"language" json:"language"`
	// ToolName 工具名称（如 cargo-llvm-cov、proptest、cargo-mutants）
	ToolName string `yaml:"tool_name" json:"tool_name"`
	// ProbeBin 用于检测可用性的可执行文件名
	ProbeBin string `yaml:"probe_bin" json:"probe_bin"`
	// ProbeArgs 检测版本或子命令可用性的参数列表
	ProbeArgs []string `yaml:"probe_args,omitempty" json:"probe_args,omitempty"`
	// DefaultCmd 默认执行命令模板
	DefaultCmd string `yaml:"default_cmd" json:"default_cmd"`
	// RequiresNightly 是否依赖 Rust Nightly 工具链
	RequiresNightly bool `yaml:"requires_nightly,omitempty" json:"requires_nightly,omitempty"`
	// RequiresHarness 是否要求仓库存在对应测试夹具或目标入口
	RequiresHarness bool `yaml:"requires_harness,omitempty" json:"requires_harness,omitempty"`
	// HarnessKey 在仓库剖析结果中查找夹具列表的键名
	HarnessKey string `yaml:"harness_key,omitempty" json:"harness_key,omitempty"`
	// HarnessHint 缺失测试夹具时向工程师提示的最小构造指引
	HarnessHint string `yaml:"harness_hint,omitempty" json:"harness_hint,omitempty"`
	// ReportFormat 默认产物报告格式标识
	ReportFormat string `yaml:"report_format" json:"report_format"`
	// InstallHint 工具缺失时的安装指引命令
	InstallHint string `yaml:"install_hint" json:"install_hint"`
	// SupportedCoverage 该适配器能真实测量的覆盖率细分指标列表
	SupportedCoverage []CoverageMetricKind `yaml:"supported_coverage,omitempty" json:"supported_coverage,omitempty"`
}

// CodeMetricSpec 描述测试技术产出的单个定量结果指标规格
type CodeMetricSpec struct {
	// Name 指标键名
	Name string `yaml:"name" json:"name"`
	// Unit 计量单位（count | pct | ms | execs_per_sec）
	Unit string `yaml:"unit" json:"unit"`
	// Direction 指标判优方向
	Direction MetricDirection `yaml:"direction" json:"direction"`
	// Description 指标业务与质量含义说明
	Description string `yaml:"description" json:"description"`
}

// CodeFailureCriteria 描述测试技术的机器可判定失败门槛与契约说明
type CodeFailureCriteria struct {
	// FailOnNonZeroExit 进程非零退出是否直接判FAIL
	FailOnNonZeroExit bool `yaml:"fail_on_non_zero_exit" json:"fail_on_non_zero_exit"`
	// MaxFailures 允许的最大失败用例数
	MaxFailures int `yaml:"max_failures" json:"max_failures"`
	// MinLineCoveragePct 最低行覆盖率百分比要求（大于0时生效）
	MinLineCoveragePct float64 `yaml:"min_line_coverage_pct,omitempty" json:"min_line_coverage_pct,omitempty"`
	// MinBranchCoveragePct 最低分支覆盖率百分比要求（大于0时生效）
	MinBranchCoveragePct float64 `yaml:"min_branch_coverage_pct,omitempty" json:"min_branch_coverage_pct,omitempty"`
	// MinMutationScorePct 最低变异杀死率百分比要求（大于0时生效）
	MinMutationScorePct float64 `yaml:"min_mutation_score_pct,omitempty" json:"min_mutation_score_pct,omitempty"`
	// MaxWarnings 允许的最大告警数（配合 EnforceMaxWarnings 启用）
	MaxWarnings int `yaml:"max_warnings,omitempty" json:"max_warnings,omitempty"`
	// EnforceMaxWarnings 是否对告警计数启用零容忍或阈值门禁
	EnforceMaxWarnings bool `yaml:"enforce_max_warnings,omitempty" json:"enforce_max_warnings,omitempty"`
	// MaxHighVulns 允许的高危及以上漏洞或缺陷数
	MaxHighVulns int `yaml:"max_high_vulns,omitempty" json:"max_high_vulns,omitempty"`
	// MaxCrashes 允许的崩溃/Panic数（通常为0）
	MaxCrashes int `yaml:"max_crashes,omitempty" json:"max_crashes,omitempty"`
	// MaxDataRaces 允许的数据竞争数（通常为0）
	MaxDataRaces int `yaml:"max_data_races,omitempty" json:"max_data_races,omitempty"`
	// MaxUBErrors 允许的未定义行为错误数（通常为0）
	MaxUBErrors int `yaml:"max_ub_errors,omitempty" json:"max_ub_errors,omitempty"`
	// MaxRegressionPct 允许的最大性能退化百分比（大于0时生效）
	MaxRegressionPct float64 `yaml:"max_regression_pct,omitempty" json:"max_regression_pct,omitempty"`
	// RequireHarnessForPass 是否要求真实夹具存在才能判PASS
	RequireHarnessForPass bool `yaml:"require_harness_for_pass,omitempty" json:"require_harness_for_pass,omitempty"`
	// Summary 人类与AI可读的失败判定规则摘要
	Summary string `yaml:"summary" json:"summary"`
}

// CodeEvidenceSpec 描述测试结果与原始产物的账本持久化及哈希取证契约
type CodeEvidenceSpec struct {
	// LedgerTable 写入的主账本数据表名
	LedgerTable string `yaml:"ledger_table" json:"ledger_table"`
	// ArtifactDirTemplate 原始产物落盘目录模板
	ArtifactDirTemplate string `yaml:"artifact_dir_template" json:"artifact_dir_template"`
	// RawArtifacts 归档的原始文件列表
	RawArtifacts []string `yaml:"raw_artifacts" json:"raw_artifacts"`
	// HashFields 参与不可变收据哈希的核心字段列表
	HashFields []string `yaml:"hash_fields" json:"hash_fields"`
	// ImmutableFailHistory 是否永久保留历史FAIL记录不予覆盖
	ImmutableFailHistory bool `yaml:"immutable_fail_history" json:"immutable_fail_history"`
}

// CodeTechniqueSpec 描述26项代码级测试技术目录中的完整七维规格定义
type CodeTechniqueSpec struct {
	// Index 目录序号（1..26）
	Index int `yaml:"index" json:"index"`
	// ID 技术闭合唯一标识
	ID string `yaml:"id" json:"id"`
	// ChineseName 中文名称
	ChineseName string `yaml:"chinese_name" json:"chinese_name"`
	// EnglishName 英文标准名称
	EnglishName string `yaml:"english_name" json:"english_name"`
	// Purpose 核心测试目的
	Purpose string `yaml:"purpose" json:"purpose"`
	// Category 工程分类枚举
	Category CodeCategory `yaml:"category" json:"category"`
	// Priority 推荐建设阶段（P0..P3）
	Priority CodePriorityTier `yaml:"priority" json:"priority"`
	// TopOverlooked 是否属于最容易忽略的三类深度测试之一
	TopOverlooked bool `yaml:"top_overlooked,omitempty" json:"top_overlooked,omitempty"`
	// OverlookedReason 容易被忽略的工程原因与盲区警示
	OverlookedReason string `yaml:"overlooked_reason,omitempty" json:"overlooked_reason,omitempty"`
	// Languages 适用语言列表
	Languages []string `yaml:"languages" json:"languages"`
	// TriggerSignals 触发纳入计划的仓库结构/风险信号列表
	TriggerSignals []string `yaml:"trigger_signals" json:"trigger_signals"`
	// RiskPromoteSignals 命中后强制提级为REQUIRED的高风险信号列表
	RiskPromoteSignals []string `yaml:"risk_promote_signals,omitempty" json:"risk_promote_signals,omitempty"`
	// Tools 按语言提供的执行工具适配器列表
	Tools []CodeToolAdapter `yaml:"tools" json:"tools"`
	// Metrics 该技术产出的核心度量指标清单
	Metrics []CodeMetricSpec `yaml:"metrics" json:"metrics"`
	// FailureCriteria 机器与人工双重失败判定标准
	FailureCriteria CodeFailureCriteria `yaml:"failure_criteria" json:"failure_criteria"`
	// ReportFormats 支持的输出报告格式列表
	ReportFormats []string `yaml:"report_formats" json:"report_formats"`
	// EvidenceStorage 账本与原始产物证据存储契约
	EvidenceStorage CodeEvidenceSpec `yaml:"evidence_storage" json:"evidence_storage"`
}

// RiskSignalHit 记录从真实源码或配置中扫描命中的单条结构风险信号及出处
type RiskSignalHit struct {
	// Signal 命中的风险或结构信号标识
	Signal string `yaml:"signal" json:"signal"`
	// Evidence 触发该信号的代码或配置摘要
	Evidence string `yaml:"evidence" json:"evidence"`
	// File 仓库相对文件路径
	File string `yaml:"file" json:"file"`
	// Line 源码行号（从1开始）
	Line int `yaml:"line" json:"line"`
}

// ToolProbeStatus 记录本机环境针对特定测试工具的可用性探测结果
type ToolProbeStatus struct {
	// Language 所属编程语言
	Language string `yaml:"language" json:"language"`
	// ToolName 工具标识
	ToolName string `yaml:"tool_name" json:"tool_name"`
	// ProbeCmd 实际探测命令
	ProbeCmd string `yaml:"probe_cmd" json:"probe_cmd"`
	// Available 工具当前是否已就绪可执行
	Available bool `yaml:"available" json:"available"`
	// VersionLine 探测到的版本或状态行
	VersionLine string `yaml:"version_line,omitempty" json:"version_line,omitempty"`
	// InstallHint 缺失时的安装指令
	InstallHint string `yaml:"install_hint,omitempty" json:"install_hint,omitempty"`
}

// RepoCodeProfile 描述对目标代码库执行语言、结构、夹具与工具探针剖析后的完整画像
type RepoCodeProfile struct {
	// RepoRoot 仓库根目录绝对路径
	RepoRoot string `yaml:"repo_root" json:"repo_root"`
	// PrimaryLanguage 识别出的主开发语言
	PrimaryLanguage string `yaml:"primary_language" json:"primary_language"`
	// Languages 检测到的全部编程语言列表
	Languages []string `yaml:"languages" json:"languages"`
	// Manifests 检测到的工程清单与锁文件相对路径
	Manifests []string `yaml:"manifests" json:"manifests"`
	// SourceFileCounts 按语言统计的生产源码文件数量
	SourceFileCounts map[string]int `yaml:"source_file_counts" json:"source_file_counts"`
	// TestFileCounts 按语言统计的测试文件数量
	TestFileCounts map[string]int `yaml:"test_file_counts" json:"test_file_counts"`
	// WorkspaceMembers 检测到的工作区成员目录或Crate名称
	WorkspaceMembers []string `yaml:"workspace_members,omitempty" json:"workspace_members,omitempty"`
	// CargoFeatures 检测到的Rust Cargo特性列表
	CargoFeatures []string `yaml:"cargo_features,omitempty" json:"cargo_features,omitempty"`
	// DetectedHarnesses 按夹具键名索引的已检测测试目标/夹具路径列表
	DetectedHarnesses map[string][]string `yaml:"detected_harnesses,omitempty" json:"detected_harnesses,omitempty"`
	// RiskSignals 命中并带真实行号出处的结构风险信号列表
	RiskSignals []RiskSignalHit `yaml:"risk_signals" json:"risk_signals"`
	// ToolProbes 环境工具链就绪状态列表
	ToolProbes []ToolProbeStatus `yaml:"tool_probes,omitempty" json:"tool_probes,omitempty"`
	// ToolsProbed 本次剖析是否执行了真实工具探测
	ToolsProbed bool `yaml:"tools_probed" json:"tools_probed"`
}

// PlannedTechnique 描述面向特定代码库自动装配或用户定制后的单项目标测试计划项
type PlannedTechnique struct {
	// Index 目录序号（1..26）
	Index int `yaml:"index" json:"index"`
	// TechniqueID 测试技术标识
	TechniqueID string `yaml:"technique_id" json:"technique_id"`
	// ChineseName 中文名称
	ChineseName string `yaml:"chinese_name" json:"chinese_name"`
	// EnglishName 英文名称
	EnglishName string `yaml:"english_name" json:"english_name"`
	// Priority 目录原始优先级（P0..P3）
	Priority CodePriorityTier `yaml:"priority" json:"priority"`
	// EffectivePriority 经风险提级后的实际生效优先级
	EffectivePriority CodePriorityTier `yaml:"effective_priority" json:"effective_priority"`
	// Status 调度决策状态（REQUIRED | RECOMMENDED | DEFERRED | NOT_APPLICABLE）
	Status CodeSelectionStatus `yaml:"status" json:"status"`
	// PromotedByRisk 是否因命中高风险代码信号而强制提前
	PromotedByRisk bool `yaml:"promoted_by_risk,omitempty" json:"promoted_by_risk,omitempty"`
	// TopOverlooked 是否属于最容易忽略的三项深度测试
	TopOverlooked bool `yaml:"top_overlooked,omitempty" json:"top_overlooked,omitempty"`
	// Language 选定的执行语言
	Language string `yaml:"language" json:"language"`
	// ToolName 选定的具体执行工具名
	ToolName string `yaml:"tool_name" json:"tool_name"`
	// Command 最终装配的可执行命令
	Command string `yaml:"command" json:"command"`
	// ToolAvailable 探测确认执行工具是否可用
	ToolAvailable bool `yaml:"tool_available" json:"tool_available"`
	// HarnessRequired 该技术是否依赖特定测试夹具
	HarnessRequired bool `yaml:"harness_required" json:"harness_required"`
	// HarnessReady 仓库内是否已检测到所需测试夹具
	HarnessReady bool `yaml:"harness_ready" json:"harness_ready"`
	// MissingPrerequisite 导致暂时阻塞的具体缺失前提说明
	MissingPrerequisite string `yaml:"missing_prerequisite,omitempty" json:"missing_prerequisite,omitempty"`
	// InstallHint 缺失工具时的安装指引
	InstallHint string `yaml:"install_hint,omitempty" json:"install_hint,omitempty"`
	// TriggeredBy 命中并触发本项测试的仓库信号列表
	TriggeredBy []string `yaml:"triggered_by,omitempty" json:"triggered_by,omitempty"`
	// Reason 调度状态与优先级决定的简要说明
	Reason string `yaml:"reason" json:"reason"`
	// FailureCriteria 生效的失败判定门槛
	FailureCriteria CodeFailureCriteria `yaml:"failure_criteria" json:"failure_criteria"`
	// ReportFormats 支持的输出报告格式
	ReportFormats []string `yaml:"report_formats" json:"report_formats"`
	// EvidencePath 原始产物存储相对路径模板
	EvidencePath string `yaml:"evidence_path" json:"evidence_path"`
}

// CodePlanOptions 描述生成代码级测试计划时的筛选与严格程度参数
type CodePlanOptions struct {
	// PlanID 可选自定义计划ID（为空时按仓库目录名自动生成）
	PlanID string
	// Language 显式指定目标语言（为空时自动推断主语言）
	Language string
	// MaxTier 目标建设优先级上限（P0..P3）
	MaxTier CodePriorityTier
	// Strict 是否将目标层级内依赖夹具的适用技术全部提升为 REQUIRED
	Strict bool
	// IncludeAllApplicable 是否把全部适用技术设为 REQUIRED（忽略层级上限）
	IncludeAllApplicable bool
	// ProbeTools 是否探测本机工具可用性（默认 true）
	ProbeTools bool
}

// CodeVerifyPlan 描述针对目标仓库编译产出的可执行代码级测试计划契约
type CodeVerifyPlan struct {
	// PlanID 计划唯一标识
	PlanID string `yaml:"plan_id" json:"plan_id"`
	// RepoRoot 目标仓库根目录路径
	RepoRoot string `yaml:"repo_root" json:"repo_root"`
	// PrimaryLanguage 计划锁定的目标语言
	PrimaryLanguage string `yaml:"primary_language" json:"primary_language"`
	// MaxTier 计划使用的最高优先级阈值
	MaxTier CodePriorityTier `yaml:"max_tier" json:"max_tier"`
	// Profile 仓库剖析摘要与真实风险信号证据
	Profile RepoCodeProfile `yaml:"profile" json:"profile"`
	// Techniques 覆盖全部26项技术的计划评估条目列表
	Techniques []PlannedTechnique `yaml:"techniques" json:"techniques"`
	// RequiredCount 纳入必选执行的技术数量
	RequiredCount int `yaml:"required_count" json:"required_count"`
	// RecommendedCount 建议补充夹具或优先引入的技术数量
	RecommendedCount int `yaml:"recommended_count" json:"recommended_count"`
	// DeferredCount 推迟至更高阶段的技术数量
	DeferredCount int `yaml:"deferred_count" json:"deferred_count"`
	// NotApplicableCount 当前项目不适用的技术数量
	NotApplicableCount int `yaml:"not_applicable_count" json:"not_applicable_count"`
	// PlanSHA256 计划定义的确定性SHA256内容摘要
	PlanSHA256 string `yaml:"-" json:"plan_sha256"`
	// SourcePath 计划YAML或JSON文件来源路径（如有）
	SourcePath string `yaml:"-" json:"source_path,omitempty"`
	// CreatedAt 计划生成UTC时间
	CreatedAt string `yaml:"created_at" json:"created_at"`
}

// CoverageBreakdown 描述八项代码覆盖率细分指标的真实测量结果与未支持清单
type CoverageBreakdown struct {
	// LinePct 行覆盖率百分比（nil表示未测量）
	LinePct *float64 `json:"line_pct,omitempty"`
	// StatementPct 语句覆盖率百分比（nil表示未测量）
	StatementPct *float64 `json:"statement_pct,omitempty"`
	// FunctionPct 函数覆盖率百分比（nil表示未测量）
	FunctionPct *float64 `json:"function_pct,omitempty"`
	// BranchPct 分支覆盖率百分比（nil表示未测量）
	BranchPct *float64 `json:"branch_pct,omitempty"`
	// ConditionPct 条件覆盖率百分比（nil表示未测量）
	ConditionPct *float64 `json:"condition_pct,omitempty"`
	// DecisionPct 决策覆盖率百分比（nil表示未测量）
	DecisionPct *float64 `json:"decision_pct,omitempty"`
	// MCDCPct 修正条件判定覆盖率百分比（nil表示未测量）
	MCDCPct *float64 `json:"mcdc_pct,omitempty"`
	// PathPct 路径覆盖率百分比（nil表示未测量）
	PathPct *float64 `json:"path_pct,omitempty"`
	// MeasuredMetrics 本次真实采集到数值的覆盖率指标集合
	MeasuredMetrics []CoverageMetricKind `json:"measured_metrics,omitempty"`
	// UnsupportedMetrics 当前工具链未提供或无法采集的指标集合
	UnsupportedMetrics []CoverageMetricKind `json:"unsupported_metrics,omitempty"`
	// SourceTool 产出本次覆盖率数据的真实工具名
	SourceTool string `json:"source_tool,omitempty"`
}

// CodeMetricPayload 描述单次代码级测试执行后解析得到的强类型度量指标载荷
type CodeMetricPayload struct {
	// TestsPassed 通过的测试数量
	TestsPassed int `json:"tests_passed,omitempty"`
	// TestsFailed 失败的测试数量
	TestsFailed int `json:"tests_failed,omitempty"`
	// TestsSkipped 被忽略或跳过的测试数量
	TestsSkipped int `json:"tests_skipped,omitempty"`
	// Warnings 静态分析或编译器告警数量
	Warnings int `json:"warnings,omitempty"`
	// HighVulns 高危安全缺陷或依赖漏洞数
	HighVulns int `json:"high_vulns,omitempty"`
	// CriticalVulns 严重安全漏洞数
	CriticalVulns int `json:"critical_vulns,omitempty"`
	// Crashes 模糊测试或压力测试发现的崩溃数
	Crashes int `json:"crashes,omitempty"`
	// FuzzExecs 模糊测试累计或单轮执行次数
	FuzzExecs int64 `json:"fuzz_execs,omitempty"`
	// MutantsCaught 被测试套件成功杀死的变异体数量
	MutantsCaught int `json:"mutants_caught,omitempty"`
	// MutantsMissed 未被测试捕捉到的存活变异体数量
	MutantsMissed int `json:"mutants_missed,omitempty"`
	// MutantsTimeout 执行超时的变异体数量
	MutantsTimeout int `json:"mutants_timeout,omitempty"`
	// MutantsUnviable 编译失败或不可行变异体数量
	MutantsUnviable int `json:"mutants_unviable,omitempty"`
	// MutationScorePct 变异杀死率百分比（nil表示未运行变异测试）
	MutationScorePct *float64 `json:"mutation_score_pct,omitempty"`
	// DataRaces 检测到的并发数据竞争数量
	DataRaces int `json:"data_races,omitempty"`
	// UBErrors Miri/ASan检测到的未定义行为与内存错误数量
	UBErrors int `json:"ub_errors,omitempty"`
	// BenchRegressionPct 性能基准相较基线的退化百分比
	BenchRegressionPct *float64 `json:"bench_regression_pct,omitempty"`
	// RSSGrowthPct 稳定性测试期间常驻内存增长百分比
	RSSGrowthPct *float64 `json:"rss_growth_pct,omitempty"`
	// ProofsVerified 形式化验证通过的证明目标数
	ProofsVerified int `json:"proofs_verified,omitempty"`
	// ProofsFailed 形式化验证失败的证明目标数
	ProofsFailed int `json:"proofs_failed,omitempty"`
	// Coverage 八项代码覆盖率细分观测详情
	Coverage *CoverageBreakdown `json:"coverage,omitempty"`
	// Extra 扩展强类型观测字段映射
	Extra FactMap `json:"extra,omitempty"`
}

// CodeRunOptions 描述执行代码级测试计划时的运行时控制选项
type CodeRunOptions struct {
	// Techniques 显式过滤执行的测试技术ID列表（为空时执行全部 REQUIRED）
	Techniques []string
	// IncludeRecommended 是否一并执行 RECOMMENDED 项
	IncludeRecommended bool
	// DryRun 是否只输出阻塞提示而不执行真实命令
	DryRun bool
	// AllowUncheckedHarness 是否允许忽略夹具缺失检测直接执行命令
	AllowUncheckedHarness bool
	// TimeoutSec 单项测试命令执行超时秒数（默认300秒）
	TimeoutSec int
	// RepoRoot 覆盖仓库根目录路径
	RepoRoot string
}

// CodeRunRecord 描述持久化在 code_verify_runs 表中的单次代码测试运行批次元数据
type CodeRunRecord struct {
	// RunID 运行批次唯一ID（CV-前缀）
	RunID string `json:"run_id"`
	// PlanID 关联的代码测试计划ID
	PlanID string `yaml:"plan_id" json:"plan_id"`
	// PlanSHA256 计划定义版本SHA256摘要
	PlanSHA256 string `json:"plan_sha256"`
	// RepoRoot 仓库根目录路径
	RepoRoot string `json:"repo_root"`
	// SourceCommit 执行时的Git提交哈希
	SourceCommit string `json:"source_commit"`
	// SourceTreeSHA256 执行时的源码工作树内容摘要
	SourceTreeSHA256 string `json:"source_tree_sha256,omitempty"`
	// MaxTier 本批次计划目标优先级上限
	MaxTier string `json:"max_tier"`
	// Verdict 批次总判定结果（PASS | FAIL | BLOCKED | NOT_RUN）
	Verdict string `json:"verdict"`
	// RequiredTotal 本批次计划执行的技术总数
	RequiredTotal int `json:"required_total"`
	// PassedCount 通过项数
	PassedCount int `json:"passed_count"`
	// FailedCount 失败项数
	FailedCount int `json:"failed_count"`
	// BlockedCount 环境或工具缺失阻塞项数
	BlockedCount int `json:"blocked_count"`
	// StartedAt 批次启动UTC时间戳
	StartedAt string `json:"started_at"`
	// FinishedAt 批次完成UTC时间戳
	FinishedAt string `json:"finished_at"`
}

// CodeResultRecord 描述追加写入且永不覆盖历史FAIL的单项代码测试执行收据
type CodeResultRecord struct {
	// ID 自增主键
	ID int64 `json:"id"`
	// RunID 所属运行批次ID
	RunID string `json:"run_id"`
	// PlanID 所属代码测试计划ID
	PlanID string `json:"plan_id"`
	// PlanSHA256 计划定义版本SHA256摘要
	PlanSHA256 string `json:"plan_sha256"`
	// TechniqueID 测试技术标识
	TechniqueID string `json:"technique_id"`
	// Priority 生效优先级（P0..P3）
	Priority string `json:"priority"`
	// Language 执行语言
	Language string `json:"language"`
	// ToolName 执行工具名称
	ToolName string `json:"tool_name"`
	// ExecutedCmd 实际执行命令
	ExecutedCmd string `json:"executed_cmd"`
	// Verdict 判定结果（PASS | FAIL | BLOCKED | NOT_RUN）
	Verdict string `json:"verdict"`
	// ExitCode 进程退出码
	ExitCode int `json:"exit_code"`
	// DurationMS 执行耗时毫秒数
	DurationMS int64 `json:"duration_ms"`
	// Metrics 强类型测试度量指标载荷
	Metrics CodeMetricPayload `json:"metrics"`
	// FailureReason 失败原因明细
	FailureReason string `json:"failure_reason,omitempty"`
	// MissingPrerequisite 阻断原因（工具或夹具缺失）
	MissingPrerequisite string `json:"missing_prerequisite,omitempty"`
	// ArtifactPath 原始产物落盘目录相对或绝对路径
	ArtifactPath string `json:"artifact_path,omitempty"`
	// ArtifactSHA256 产物目录内容的确定性SHA256摘要
	ArtifactSHA256 string `json:"artifact_sha256,omitempty"`
	// RawOutput 截断或完整原始命令输出文本
	RawOutput string `json:"raw_output,omitempty"`
	// RepoRoot 执行时规范化仓库根目录
	RepoRoot string `json:"repo_root,omitempty"`
	// SourceCommit 执行时源码Git提交哈希
	SourceCommit string `json:"source_commit,omitempty"`
	// SourceTreeSHA256 执行时源码工作树摘要
	SourceTreeSHA256 string `json:"source_tree_sha256,omitempty"`
	// ReceiptSHA256 本条不可变收据的内容SHA256摘要
	ReceiptSHA256 string `json:"receipt_sha256"`
	// CreatedAt 收据生成UTC时间
	CreatedAt string `json:"created_at"`
}

// CodeRunSummary 描述 arc verify code run 返回的完整执行结果汇总
type CodeRunSummary struct {
	// Run 本次运行批次元数据
	Run CodeRunRecord `json:"run"`
	// Results 本批次执行出的各技术收据列表
	Results []CodeResultRecord `json:"results"`
	// HistoricalFailCount 账本中该计划永久保留的历史FAIL累计次数
	HistoricalFailCount int `json:"historical_fail_count"`
	// Gate 同步计算出的门禁完成度评估报告
	Gate *CodeGateReport `json:"gate"`
}

// CodeExplainReport 描述 arc verify code explain 输出的详细代码级测试台账
type CodeExplainReport struct {
	// PlanID 计划ID
	PlanID string `json:"plan_id"`
	// PlanSHA256 计划SHA256摘要
	PlanSHA256 string `json:"plan_sha256"`
	// RepoRoot 仓库根目录
	RepoRoot string `json:"repo_root"`
	// PrimaryLanguage 主语言
	PrimaryLanguage string `json:"primary_language"`
	// LatestRun 最近一次或指定的运行批次记录
	LatestRun *CodeRunRecord `json:"latest_run,omitempty"`
	// Plan 完整代码测试计划定义
	Plan *CodeVerifyPlan `json:"plan"`
	// Results 该批次下各技术执行结果收据列表
	Results []CodeResultRecord `json:"results"`
	// HistoricalFailCount 历史FAIL累计保留数
	HistoricalFailCount int `json:"historical_fail_count"`
	// Gate 门禁评估结果
	Gate *CodeGateReport `json:"gate"`
}

// CodeGateReport 描述 arc verify code gate 的分层门禁与完成度判定报告
type CodeGateReport struct {
	// PlanID 计划ID
	PlanID string `json:"plan_id"`
	// PlanSHA256 计划SHA256摘要
	PlanSHA256 string `json:"plan_sha256"`
	// RunID 评估对应的运行批次ID
	RunID string `json:"run_id,omitempty"`
	// MinTier 本次门禁要求的最高必须达标层级
	MinTier CodePriorityTier `json:"min_tier"`
	// Verdict 门禁总判定结果（PASS | FAIL | BLOCKED | NOT_RUN）
	Verdict string `json:"verdict"`
	// ProofScope 证明范围标识
	ProofScope string `json:"proof_scope"`
	// InputsUnchanged 计划定义未偏离账本版本标志
	InputsUnchanged bool `json:"inputs_unchanged"`
	// RequiredCount 需强制通过的技术总数
	RequiredCount int `json:"required_count"`
	// PassedCount 通过项数
	PassedCount int `json:"passed_count"`
	// FailedCount 失败项数
	FailedCount int `json:"failed_count"`
	// BlockedCount 阻塞项数
	BlockedCount int `json:"blocked_count"`
	// NotRunCount 尚未执行项数
	NotRunCount int `json:"not_run_count"`
	// HistoricalFailCount 账本永久保留的历史FAIL计数
	HistoricalFailCount int `json:"historical_fail_count"`
	// OverlookedStatus 最容易忽略的三项测试（mutation/property/fuzz）当前状态映射
	OverlookedStatus map[string]string `json:"overlooked_status"`
	// MissingTechniques 尚未通过的必选技术ID列表
	MissingTechniques []string `json:"missing_techniques,omitempty"`
	// Reason 未达成PASS的具体说明
	Reason string `json:"reason,omitempty"`
}

// codeCatalogResponse 描述 arc verify code catalog 输出的具名目录响应
type codeCatalogResponse struct {
	// Version 目录协议版本
	Version int `json:"version"`
	// TotalTechniques 收录的代码级测试技术总数（固定26）
	TotalTechniques int `json:"total_techniques"`
	// TotalCoverageMetrics 收录的代码覆盖率细分指标总数（固定8）
	TotalCoverageMetrics int `json:"total_coverage_metrics"`
	// Techniques 筛选后的测试技术规格列表
	Techniques []CodeTechniqueSpec `json:"techniques"`
	// CoverageMetrics 八项代码覆盖率细分指标规格列表
	CoverageMetrics []CoverageMetricSpec `json:"coverage_metrics,omitempty"`
}

// codeSnapshotDocument 描述导出到代码测试工作台 snapshot.json 的完整只读文档
type codeSnapshotDocument struct {
	// SchemaVersion 快照Schema版本号
	SchemaVersion int `json:"schema_version"`
	// Kind 快照类型标识
	Kind string `json:"kind"`
	// Plan 代码级测试计划定义
	Plan *CodeVerifyPlan `json:"plan"`
	// LatestRun 最新运行批次元数据
	LatestRun *CodeRunRecord `json:"latest_run,omitempty"`
	// Results 最新运行批次的单项测试收据列表
	Results []CodeResultRecord `json:"results"`
	// Gate 门禁判定汇总报告
	Gate *CodeGateReport `json:"gate"`
	// CoverageMetrics 八项覆盖率指标说明矩阵
	CoverageMetrics []CoverageMetricSpec `json:"coverage_metrics"`
	// ExportedAt 导出UTC时间
	ExportedAt string `json:"exported_at"`
}

// codeSnapshotExportResponse 描述 arc verify code snapshot 导出成功响应
type codeSnapshotExportResponse struct {
	// PlanID 导出的计划ID
	PlanID string `json:"plan_id"`
	// RunID 导出的运行批次ID
	RunID string `json:"run_id,omitempty"`
	// Exported 是否导出成功
	Exported bool `json:"exported"`
	// Out 导出目录路径
	Out string `json:"out"`
}
