package main

// allCoverageMetrics 返回八项代码覆盖率细分指标的完整标准定义与工具链支持矩阵
// 请求体
// 无
// 响应体
// metrics 八项覆盖率指标规格切片
func allCoverageMetrics() []CoverageMetricSpec {
	// 1. 返回严格对应软件工程标准的八项覆盖率细分定义
	return []CoverageMetricSpec{
		{
			ID:          CovLine,
			EnglishName: "Line Coverage",
			ChineseName: "行覆盖率",
			Definition:  "可执行源码行中被测试至少执行一次的比例",
			RustSupport: "Stable 支持：`cargo llvm-cov --workspace --summary-only --json` 直接产出 lines.percent",
			MultiLangTools: map[string]string{
				"rust": "cargo-llvm-cov", "go": "go test -coverprofile (statement 近似)", "typescript": "vitest/istanbul", "python": "pytest-cov/coverage.py", "c_cpp": "llvm-cov/gcov",
			},
		},
		{
			ID:          CovStatement,
			EnglishName: "Statement Coverage",
			ChineseName: "语句覆盖率",
			Definition:  "程序语法语句或基本执行块中被测试执行的比例",
			RustSupport: "由 `cargo llvm-cov` region/instantiation 粒度近似反映",
			MultiLangTools: map[string]string{
				"rust": "cargo-llvm-cov (region)", "go": "go tool cover -func", "typescript": "vitest/v8-istanbul", "python": "coverage.py", "c_cpp": "gcov/llvm-cov",
			},
		},
		{
			ID:          CovFunction,
			EnglishName: "Function Coverage",
			ChineseName: "函数覆盖率",
			Definition:  "已定义函数或方法中在测试期间至少被调用一次的比例",
			RustSupport: "Stable 支持：`cargo llvm-cov --json` 直接产出 functions.percent",
			MultiLangTools: map[string]string{
				"rust": "cargo-llvm-cov", "go": "go tool cover -func (非0%函数占比)", "typescript": "vitest/istanbul", "python": "coverage.py/inspect", "c_cpp": "llvm-cov",
			},
		},
		{
			ID:          CovBranch,
			EnglishName: "Branch Coverage",
			ChineseName: "分支覆盖率",
			Definition:  "控制流中各条件跳转分支（true/false 与 match 分支臂）的执行比例",
			RustSupport: "Nightly 实验支持：`cargo +nightly llvm-cov --branch --json`",
			MultiLangTools: map[string]string{
				"rust": "cargo +nightly llvm-cov --branch", "go": "需 LLVM/第三方插桩（标准 go cover 不测分支）", "typescript": "istanbul/c8 branches", "python": "pytest-cov --cov-branch", "c_cpp": "llvm-cov/gcovr --branches",
			},
		},
		{
			ID:          CovCondition,
			EnglishName: "Condition Coverage",
			ChineseName: "条件覆盖率",
			Definition:  "复合布尔表达式中每个原子子条件真假取值均被独立触发的比例",
			RustSupport: "借助 Nightly branch/MC-DC 插桩与属性边界用例联合验证",
			MultiLangTools: map[string]string{
				"rust": "cargo +nightly llvm-cov (--branch/--mcdc)", "go": "需外部布尔子条件分析器", "typescript": "有限（多合并为 branch）", "python": "有限（branch 为主）", "c_cpp": "gcovr/GCC -fcondition-coverage",
			},
		},
		{
			ID:          CovDecision,
			EnglishName: "Decision Coverage",
			ChineseName: "决策覆盖率",
			Definition:  "每个判定点（if/while/match 守卫整体）取真与取假结果全覆盖的比例",
			RustSupport: "与 `--branch` 及 `proptest` 判定路径测试结合度量",
			MultiLangTools: map[string]string{
				"rust": "cargo +nightly llvm-cov --branch", "go": "静态控制流/第三方插桩", "typescript": "istanbul branch/decision", "python": "coverage.py --branch", "c_cpp": "llvm-cov/gcovr",
			},
		},
		{
			ID:          CovMCDC,
			EnglishName: "MC/DC Coverage",
			ChineseName: "修正条件/判定覆盖率",
			Definition:  "每个原子条件均能在其余条件不变时独立翻转整个判定结果的航空级覆盖率",
			RustSupport: "Nightly 支持：`RUSTFLAGS='-C instrument-coverage=mcdc' cargo +nightly llvm-cov --mcdc`",
			MultiLangTools: map[string]string{
				"rust": "cargo +nightly llvm-cov --mcdc", "go": "原生工具链不支持", "typescript": "原生工具链不支持", "python": "原生工具链不支持", "c_cpp": "clang -fcoverage-mcdc / gcc 14+",
			},
		},
		{
			ID:          CovPath,
			EnglishName: "Path Coverage",
			ChineseName: "路径覆盖率",
			Definition:  "函数或状态机内部独立控制流路径组合的执行比例（通常需符号或属性采样）",
			RustSupport: "由 `proptest` 状态序列、`kani` 符号路径探索与核心逻辑枚举共同逼近",
			MultiLangTools: map[string]string{
				"rust": "proptest + kani (路径采样/符号覆盖)", "go": "rapid/fuzz 路径探索", "typescript": "fast-check 状态机路径", "python": "hypothesis stateful", "c_cpp": "klee/cbmc/libFuzzer",
			},
		},
	}
}

// codeEvidenceDefaults 构造代码级测试技术的标准账本与产物存储契约
// 请求体
// techID 测试技术标识
// rawArtifacts 需随批次归档的原始产物文件名列表
// 响应体
// spec 标准证据存储契约结构体
func codeEvidenceDefaults(techID string, rawArtifacts []string) CodeEvidenceSpec {
	// 1. 组装统一的不可变账本表名、产物目录模板与哈希字段清单
	return CodeEvidenceSpec{
		LedgerTable:          "code_verify_results",
		ArtifactDirTemplate:  ".arc/verify/code/<run_id>/" + techID,
		RawArtifacts:         rawArtifacts,
		HashFields:           []string{"plan_sha256", "executed_cmd", "exit_code", "verdict", "metrics_json", "artifact_sha256"},
		ImmutableFailHistory: true,
	}
}

// allCodeTechniques 返回完整有序的26项代码级测试技术目录（#1..#26）
// 请求体
// 无
// 响应体
// specs 按序号1..26排列的测试技术规格切片
func allCodeTechniques() []CodeTechniqueSpec {
	// 1. 合并基础与进阶两段目录确保完整覆盖全部26项代码级测试技术
	out := make([]CodeTechniqueSpec, 0, 26)
	out = append(out, coreCodeTechniques()...)
	out = append(out, advancedCodeTechniques()...)
	return out
}

// coreCodeTechniques 返回目录序号#1至#13的基础、覆盖率与核心缺陷发现测试技术
// 请求体
// 无
// 响应体
// specs 序号#1..#13的测试技术规格切片
func coreCodeTechniques() []CodeTechniqueSpec {
	// 1. 定义#1-#13测试技术的七维完整规格
	return []CodeTechniqueSpec{
		{
			Index:          1,
			ID:             "unit",
			ChineseName:    "单元测试",
			EnglishName:    "Unit Testing",
			Purpose:        "验证函数、方法的正确性",
			Category:       CategoryFunctional,
			Priority:       TierP0,
			Languages:      []string{"rust", "go", "typescript", "python", "c_cpp"},
			TriggerSignals: []string{"always_baseline", "has_tests"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "cargo test", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --workspace --lib", ReportFormat: "stdout", InstallHint: "rustup toolchain install stable"},
				{Language: "go", ToolName: "go test", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -v ./...", ReportFormat: "stdout", InstallHint: "安装 Go 工具链"},
				{Language: "typescript", ToolName: "vitest", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run", ReportFormat: "json", InstallHint: "npm i -D vitest"},
				{Language: "python", ToolName: "pytest", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest -q", ReportFormat: "junit", InstallHint: "pip install pytest"},
				{Language: "c_cpp", ToolName: "ctest", ProbeBin: "ctest", ProbeArgs: []string{"--version"}, DefaultCmd: "ctest --output-on-failure", ReportFormat: "stdout", InstallHint: "安装 CMake/CTest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "通过的单元测试数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "失败的单元测试数"},
				{Name: "tests_skipped", Unit: "count", Direction: DirInformational, Description: "被忽略或跳过的用例数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, Summary: "进程非零退出或 tests_failed > 0 即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md", "html", "stdout"},
			EvidenceStorage: codeEvidenceDefaults("unit", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          2,
			ID:             "integration",
			ChineseName:    "集成测试",
			EnglishName:    "Integration Testing",
			Purpose:        "验证代码模块之间的协作",
			Category:       CategoryFunctional,
			Priority:       TierP0,
			Languages:      []string{"rust", "go", "typescript", "python", "c_cpp"},
			TriggerSignals: []string{"has_multi_module", "has_tests", "always_baseline"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "cargo test", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --workspace --tests", ReportFormat: "stdout", InstallHint: "rustup toolchain install stable"},
				{Language: "go", ToolName: "go test", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -v -count=1 ./...", ReportFormat: "stdout", InstallHint: "安装 Go 工具链"},
				{Language: "typescript", ToolName: "vitest", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run", ReportFormat: "json", InstallHint: "npm i -D vitest"},
				{Language: "python", ToolName: "pytest", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest", ReportFormat: "junit", InstallHint: "pip install pytest"},
				{Language: "c_cpp", ToolName: "ctest", ProbeBin: "ctest", ProbeArgs: []string{"--version"}, DefaultCmd: "ctest --output-on-failure", ReportFormat: "stdout", InstallHint: "安装 CMake/CTest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "通过的模块集成用例数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "失败的集成用例数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, Summary: "跨模块接口协作测试出现任意失败或非零退出即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md", "html", "stdout"},
			EvidenceStorage: codeEvidenceDefaults("integration", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          3,
			ID:             "coverage",
			ChineseName:    "代码覆盖率分析",
			EnglishName:    "Code Coverage",
			Purpose:        "测量测试执行过哪些代码（细分行/语句/函数/分支/条件/决策/MC-DC/路径8项指标）",
			Category:       CategoryCoverage,
			Priority:       TierP0,
			Languages:      []string{"rust", "go", "typescript", "python", "c_cpp"},
			TriggerSignals: []string{"always_baseline", "has_tests"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "cargo-llvm-cov", ProbeBin: "cargo", ProbeArgs: []string{"llvm-cov", "--version"}, DefaultCmd: "cargo llvm-cov --workspace --summary-only --json", ReportFormat: "json", InstallHint: "cargo install cargo-llvm-cov && rustup component add llvm-tools-preview", SupportedCoverage: []CoverageMetricKind{CovLine, CovStatement, CovFunction, CovBranch, CovMCDC}},
				{Language: "go", ToolName: "go tool cover", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -covermode=atomic -coverprofile=\"$ARC_CODE_ARTIFACT_DIR/cover.out\" ./... && go tool cover -func=\"$ARC_CODE_ARTIFACT_DIR/cover.out\"", ReportFormat: "text", InstallHint: "Go 内置覆盖率工具链", SupportedCoverage: []CoverageMetricKind{CovStatement, CovFunction}},
				{Language: "typescript", ToolName: "vitest coverage", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run --coverage --coverage.reporter=json-summary --coverage.reportsDirectory=\"$ARC_CODE_ARTIFACT_DIR\"", ReportFormat: "json", InstallHint: "npm i -D @vitest/coverage-v8", SupportedCoverage: []CoverageMetricKind{CovLine, CovStatement, CovFunction, CovBranch}},
				{Language: "python", ToolName: "pytest-cov", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest --cov --cov-branch --cov-report=json:\"$ARC_CODE_ARTIFACT_DIR/coverage.json\"", ReportFormat: "json", InstallHint: "pip install pytest-cov", SupportedCoverage: []CoverageMetricKind{CovLine, CovStatement, CovFunction, CovBranch}},
				{Language: "c_cpp", ToolName: "llvm-cov", ProbeBin: "llvm-cov", ProbeArgs: []string{"--version"}, DefaultCmd: `[ -n "$LLVM_COV_BIN" ] && [ -f default.profdata ] && llvm-cov export -summary-only -instr-profile=default.profdata "$LLVM_COV_BIN"`, ReportFormat: "json", InstallHint: "安装 LLVM 工具链并设置 LLVM_COV_BIN 与 default.profdata", SupportedCoverage: []CoverageMetricKind{CovLine, CovFunction, CovBranch, CovMCDC}},
			},
			Metrics: []CodeMetricSpec{
				{Name: "line_pct", Unit: "pct", Direction: DirHigherIsBetter, Description: "行覆盖率百分比"},
				{Name: "statement_pct", Unit: "pct", Direction: DirHigherIsBetter, Description: "语句覆盖率百分比"},
				{Name: "function_pct", Unit: "pct", Direction: DirHigherIsBetter, Description: "函数覆盖率百分比"},
				{Name: "branch_pct", Unit: "pct", Direction: DirHigherIsBetter, Description: "分支覆盖率百分比"},
				{Name: "mcdc_pct", Unit: "pct", Direction: DirHigherIsBetter, Description: "MC/DC 覆盖率百分比（支持工具时采集）"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, Summary: "覆盖率采集命令失败判 FAIL；未测量指标以 null 与 unsupported_metrics 诚实披露"},
			ReportFormats:   []string{"json", "html", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("coverage", []string{"output.log", "metrics.json", "cover.out", "coverage.json"}),
		},
		{
			Index:              4,
			ID:                 "fuzz",
			ChineseName:        "模糊测试",
			EnglishName:        "Fuzz Testing",
			Purpose:            "使用大量自动生成的输入寻找崩溃和缺陷",
			Category:           CategoryDefectDiscovery,
			Priority:           TierP1,
			TopOverlooked:      true,
			OverlookedReason:   "人工单测难以覆盖畸形字节流、极端嵌套与序列化状态组合，最易在不可信解析入口漏掉 panic/OOM/越界",
			Languages:          []string{"rust", "go", "python", "typescript", "c_cpp"},
			TriggerSignals:     []string{"has_parser_or_untrusted_io", "has_unsafe_or_ffi", "has_codec_or_algebraic_logic"},
			RiskPromoteSignals: []string{"has_parser_or_untrusted_io", "has_unsafe_or_ffi"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "cargo-fuzz", ProbeBin: "cargo", ProbeArgs: []string{"fuzz", "--help"}, DefaultCmd: "cargo +nightly fuzz run fuzz_target_1 -- -max_total_time=30", RequiresNightly: true, RequiresHarness: true, HarnessKey: "fuzz_targets", HarnessHint: "创建 fuzz/fuzz_targets/<target>.rs（cargo fuzz init）", ReportFormat: "corpus", InstallHint: "cargo install cargo-fuzz && rustup toolchain install nightly"},
				{Language: "go", ToolName: "go test -fuzz", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: `found=0; for p in $(go list -f '{{.ImportPath}}:{{.Dir}}' ./...); do pkg="${p%%:*}"; dir="${p#*:}"; for fn in $(grep -hE '^func Fuzz[A-Za-z0-9_]+\(' "$dir"/*_test.go 2>/dev/null | sed -E 's/^func (Fuzz[A-Za-z0-9_]+)\(.*/\1/'); do found=$((found+1)); go test -fuzz="^${fn}$" -fuzztime=10s "$pkg" || exit 1; done; done; [ "$found" -gt 0 ] || { echo "no Go fuzz targets found"; exit 1; }`, RequiresHarness: true, HarnessKey: "go_fuzz", HarnessHint: "编写 func FuzzXxx(f *testing.F)", ReportFormat: "corpus", InstallHint: "Go 1.18+ 原生支持"},
				{Language: "python", ToolName: "atheris", ProbeBin: "python3", ProbeArgs: []string{"-c", "import atheris"}, DefaultCmd: "python3 fuzz_target.py -atheris_runs=10000", RequiresHarness: true, HarnessKey: "py_fuzz", HarnessHint: "编写 atheris.Setup 模糊目标脚本", ReportFormat: "corpus", InstallHint: "pip install atheris"},
				{Language: "typescript", ToolName: "jazzer.js", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "jazzer", "--help"}, DefaultCmd: "npx jazzer fuzz_target", RequiresHarness: true, HarnessKey: "ts_fuzz", HarnessHint: "编写导出 fuzz(data: Buffer) 的目标文件", ReportFormat: "corpus", InstallHint: "npm i -D @jazzer.js/core"},
				{Language: "c_cpp", ToolName: "libFuzzer", ProbeBin: "clang++", ProbeArgs: []string{"--version"}, DefaultCmd: "ctest -R fuzz", RequiresHarness: true, HarnessKey: "c_fuzz", HarnessHint: "编写 LLVMFuzzerTestOneInput 入口", ReportFormat: "corpus", InstallHint: "安装 Clang libFuzzer"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "crashes", Unit: "count", Direction: DirExactZero, Description: "触发崩溃、panic 或 sanitizer 告警的语料数"},
				{Name: "fuzz_execs", Unit: "count", Direction: DirHigherIsBetter, Description: "累计或本轮执行的随机变异样本数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxCrashes: 0, RequireHarnessForPass: true, Summary: "发现任意崩溃/panic/artifact 或 crashes > 0 即判 FAIL；缺少 fuzz target 判 BLOCKED"},
			ReportFormats:   []string{"json", "ai-md", "corpus"},
			EvidenceStorage: codeEvidenceDefaults("fuzz", []string{"output.log", "metrics.json", "crash_artifacts"}),
		},
		{
			Index:              5,
			ID:                 "property",
			ChineseName:        "属性测试",
			EnglishName:        "Property-based Testing",
			Purpose:            "验证代码始终满足特定数学或逻辑性质",
			Category:           CategoryDefectDiscovery,
			Priority:           TierP1,
			TopOverlooked:      true,
			OverlookedReason:   "常规示例单测只盯住手写样例；往返可逆、守恒律、幂等性、排序不变量必须依靠随机生成+Shrink缩小反例验证",
			Languages:          []string{"rust", "go", "python", "typescript"},
			TriggerSignals:     []string{"has_codec_or_algebraic_logic", "has_state_machine_or_protocol", "has_critical_core_logic"},
			RiskPromoteSignals: []string{"has_codec_or_algebraic_logic", "has_state_machine_or_protocol"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "proptest", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --workspace -- proptest", RequiresHarness: true, HarnessKey: "proptest", HarnessHint: "添加 proptest 依赖并编写 proptest! 不变量测试", ReportFormat: "stdout", InstallHint: "cargo add --dev proptest"},
				{Language: "go", ToolName: "rapid", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -run Property ./...", RequiresHarness: true, HarnessKey: "go_property", HarnessHint: "使用 pgregory.net/rapid 编写 rapid.Check 属性测试", ReportFormat: "stdout", InstallHint: "go get pgregory.net/rapid"},
				{Language: "python", ToolName: "hypothesis", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest -k property", RequiresHarness: true, HarnessKey: "py_hypothesis", HarnessHint: "使用 @given(...) 编写 Hypothesis 属性测试", ReportFormat: "stdout", InstallHint: "pip install hypothesis"},
				{Language: "typescript", ToolName: "fast-check", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run -t property", RequiresHarness: true, HarnessKey: "ts_fastcheck", HarnessHint: "使用 fc.assert(fc.property(...)) 编写属性测试", ReportFormat: "json", InstallHint: "npm i -D fast-check"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "通过的不变量属性组数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "被反例推翻的属性测试数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, RequireHarnessForPass: true, Summary: "任一数学/逻辑不变量被反例破坏即判 FAIL；未编写属性测试夹具判 BLOCKED"},
			ReportFormats:   []string{"json", "ai-md", "stdout"},
			EvidenceStorage: codeEvidenceDefaults("property", []string{"output.log", "metrics.json"}),
		},
		{
			Index:              6,
			ID:                 "mutation",
			ChineseName:        "变异测试",
			EnglishName:        "Mutation Testing",
			Purpose:            "故意修改代码，验证测试能否发现错误",
			Category:           CategoryDefectDiscovery,
			Priority:           TierP1,
			TopOverlooked:      true,
			OverlookedReason:   "即使行覆盖率达到100%，测试也可能没有有效断言；只有变异测试能把“只跑不 assert”的虚假高覆盖打回原形",
			Languages:          []string{"rust", "go", "python", "typescript"},
			TriggerSignals:     []string{"has_tests", "has_critical_core_logic"},
			RiskPromoteSignals: []string{"has_critical_core_logic"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "cargo-mutants", ProbeBin: "cargo", ProbeArgs: []string{"mutants", "--version"}, DefaultCmd: "cargo mutants --in-place=false --output \"$ARC_CODE_ARTIFACT_DIR\"", ReportFormat: "json", InstallHint: "cargo install cargo-mutants"},
				{Language: "go", ToolName: "gremlins", ProbeBin: "gremlins", ProbeArgs: []string{"version"}, DefaultCmd: "gremlins unleash ./...", ReportFormat: "json", InstallHint: "go install github.com/go-gremlins/gremlins/cmd/gremlins@latest"},
				{Language: "python", ToolName: "mutmut", ProbeBin: "mutmut", ProbeArgs: []string{"--version"}, DefaultCmd: "mutmut run", ReportFormat: "text", InstallHint: "pip install mutmut"},
				{Language: "typescript", ToolName: "stryker", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "stryker", "--version"}, DefaultCmd: "npx stryker run", ReportFormat: "json", InstallHint: "npm i -D @stryker-mutator/core"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "mutants_caught", Unit: "count", Direction: DirHigherIsBetter, Description: "被测试套件杀死的变异体数"},
				{Name: "mutants_missed", Unit: "count", Direction: DirLowerIsBetter, Description: "逃逸存活的变异体数（测试盲区）"},
				{Name: "mutation_score_pct", Unit: "pct", Direction: DirHigherIsBetter, Description: "变异杀死率 = caught / (caught + missed) * 100"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MinMutationScorePct: 80.0, Summary: "逃逸变异体导致退出非零或 mutation_score_pct < 80% 判 FAIL（可按风险定制阈值）"},
			ReportFormats:   []string{"json", "ai-md", "html"},
			EvidenceStorage: codeEvidenceDefaults("mutation", []string{"output.log", "metrics.json", "outcomes.json"}),
		},
		{
			Index:          7,
			ID:             "regression",
			ChineseName:    "回归测试",
			EnglishName:    "Regression Testing",
			Purpose:        "确保代码变更未破坏已有功能",
			Category:       CategoryFunctional,
			Priority:       TierP0,
			Languages:      []string{"rust", "go", "typescript", "python", "c_cpp"},
			TriggerSignals: []string{"always_baseline", "has_tests"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "cargo-nextest", ProbeBin: "cargo", ProbeArgs: []string{"nextest", "--version"}, DefaultCmd: "cargo nextest run --workspace", ReportFormat: "junit", InstallHint: "cargo install cargo-nextest (无安装时自动回退 cargo test --workspace)"},
				{Language: "go", ToolName: "go test", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -v -count=1 ./...", ReportFormat: "stdout", InstallHint: "安装 Go 工具链"},
				{Language: "typescript", ToolName: "vitest", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run", ReportFormat: "json", InstallHint: "npm i -D vitest"},
				{Language: "python", ToolName: "pytest", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest", ReportFormat: "junit", InstallHint: "pip install pytest"},
				{Language: "c_cpp", ToolName: "ctest", ProbeBin: "ctest", ProbeArgs: []string{"--version"}, DefaultCmd: "ctest --output-on-failure", ReportFormat: "stdout", InstallHint: "安装 CMake/CTest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "无缓存重放通过的全量回归测试数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "回归破坏测试数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, Summary: "全量无缓存重跑存在失败用例或非零退出即判 FAIL"},
			ReportFormats:   []string{"json", "junit", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("regression", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          8,
			ID:             "snapshot",
			ChineseName:    "快照测试",
			EnglishName:    "Snapshot Testing",
			Purpose:        "比较输出与已批准的快照",
			Category:       CategoryFunctional,
			Priority:       TierP1,
			Languages:      []string{"rust", "go", "typescript", "python"},
			TriggerSignals: []string{"has_serialized_outputs"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "insta", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --workspace -- insta", RequiresHarness: true, HarnessKey: "insta", HarnessHint: "添加 insta 依赖并建立 .snap 批准快照", ReportFormat: "diff", InstallHint: "cargo add --dev insta && cargo install cargo-insta"},
				{Language: "go", ToolName: "go test snapshot", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -run Snapshot ./...", RequiresHarness: true, HarnessKey: "go_snapshot", HarnessHint: "编写 Golden/Snapshot 测试并提交 testdata 快照", ReportFormat: "diff", InstallHint: "Go 原生 golden testdata"},
				{Language: "typescript", ToolName: "vitest snapshot", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run -t snapshot", RequiresHarness: true, HarnessKey: "ts_snapshot", HarnessHint: "编写 toMatchSnapshot() 断言", ReportFormat: "diff", InstallHint: "npm i -D vitest"},
				{Language: "python", ToolName: "syrupy", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest -k snapshot --snapshot-warn-unused", RequiresHarness: true, HarnessKey: "py_snapshot", HarnessHint: "使用 syrupy snapshot fixture", ReportFormat: "diff", InstallHint: "pip install syrupy"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "与已批准基线一致的快照比对数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "快照发生未批准漂移的失败数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, RequireHarnessForPass: true, Summary: "禁止测试阶段自动覆盖快照；存在未批准快照差异即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md", "diff"},
			EvidenceStorage: codeEvidenceDefaults("snapshot", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          9,
			ID:             "differential",
			ChineseName:    "差分测试",
			EnglishName:    "Differential Testing",
			Purpose:        "比较不同实现对相同输入的输出",
			Category:       CategoryCorrectness,
			Priority:       TierP2,
			Languages:      []string{"rust", "go", "typescript", "python", "c_cpp"},
			TriggerSignals: []string{"has_reference_or_dual_impl", "has_codec_or_algebraic_logic"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "custom differential harness", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --workspace -- differential", RequiresHarness: true, HarnessKey: "differential", HarnessHint: "编写参考模型与目标实现的同输入比对测试 (test name 含 differential)", ReportFormat: "json", InstallHint: "仓库自建差分测试驱动"},
				{Language: "go", ToolName: "go differential harness", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -run Differential ./...", RequiresHarness: true, HarnessKey: "differential", HarnessHint: "编写 TestDifferential* 对比基准实现输出", ReportFormat: "json", InstallHint: "Go 原生 testing"},
				{Language: "python", ToolName: "pytest differential", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest -k differential", RequiresHarness: true, HarnessKey: "differential", HarnessHint: "编写 test_differential_* 对比测试", ReportFormat: "json", InstallHint: "pip install pytest"},
				{Language: "typescript", ToolName: "vitest differential", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run -t differential", RequiresHarness: true, HarnessKey: "differential", HarnessHint: "编写 differential 对比测试套件", ReportFormat: "json", InstallHint: "npm i -D vitest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "双实现输出一致的用例数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "同一输入产生分歧输出的对数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, RequireHarnessForPass: true, Summary: "新旧或参考实现输出出现分歧即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md", "diff"},
			EvidenceStorage: codeEvidenceDefaults("differential", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          10,
			ID:             "metamorphic",
			ChineseName:    "变形测试",
			EnglishName:    "Metamorphic Testing",
			Purpose:        "验证输入变化与输出变化之间的关系",
			Category:       CategoryCorrectness,
			Priority:       TierP2,
			Languages:      []string{"rust", "go", "typescript", "python"},
			TriggerSignals: []string{"has_numeric_or_search_transform", "has_codec_or_algebraic_logic"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "proptest (metamorphic)", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --workspace -- metamorphic", RequiresHarness: true, HarnessKey: "metamorphic", HarnessHint: "使用 proptest 编写输入变换-输出关系验证 (含 metamorphic)", ReportFormat: "stdout", InstallHint: "cargo add --dev proptest"},
				{Language: "go", ToolName: "go metamorphic harness", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -run Metamorphic ./...", RequiresHarness: true, HarnessKey: "metamorphic", HarnessHint: "编写 TestMetamorphic* 关系测试", ReportFormat: "stdout", InstallHint: "Go testing / rapid"},
				{Language: "python", ToolName: "hypothesis metamorphic", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest -k metamorphic", RequiresHarness: true, HarnessKey: "metamorphic", HarnessHint: "编写 test_metamorphic_* 关系测试", ReportFormat: "stdout", InstallHint: "pip install hypothesis"},
				{Language: "typescript", ToolName: "fast-check metamorphic", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run -t metamorphic", RequiresHarness: true, HarnessKey: "metamorphic", HarnessHint: "编写 metamorphic 关系测试", ReportFormat: "json", InstallHint: "npm i -D fast-check"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "满足变形关系的验证组数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "违背变形关系的反例数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, RequireHarnessForPass: true, Summary: "输入按约定变换后输出关系不成立即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("metamorphic", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          11,
			ID:             "boundary",
			ChineseName:    "边界值测试",
			EnglishName:    "Boundary Value Testing",
			Purpose:        "检测最大值、最小值和临界值问题",
			Category:       CategoryFunctional,
			Priority:       TierP1,
			Languages:      []string{"rust", "go", "typescript", "python", "c_cpp"},
			TriggerSignals: []string{"has_boundary_constraints", "always_baseline"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "rstest", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --workspace -- boundary", RequiresHarness: true, HarnessKey: "boundary", HarnessHint: "使用 rstest #[values(...)] 覆盖 0/1/MIN/MAX/溢出临界值 (含 boundary)", ReportFormat: "stdout", InstallHint: "cargo add --dev rstest"},
				{Language: "go", ToolName: "go table boundary", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -run Boundary ./...", RequiresHarness: true, HarnessKey: "boundary", HarnessHint: "编写 Test*Boundary* 临界值表驱动用例", ReportFormat: "stdout", InstallHint: "Go testing"},
				{Language: "python", ToolName: "pytest boundary", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest -k boundary", RequiresHarness: true, HarnessKey: "boundary", HarnessHint: "使用 @pytest.mark.parametrize 覆盖临界值", ReportFormat: "junit", InstallHint: "pip install pytest"},
				{Language: "typescript", ToolName: "vitest boundary", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run -t boundary", RequiresHarness: true, HarnessKey: "boundary", HarnessHint: "使用 test.each 覆盖临界值", ReportFormat: "json", InstallHint: "npm i -D vitest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "通过的临界值测试点"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "临界值溢出或错一缺陷数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, RequireHarnessForPass: true, Summary: "存在 off-by-one/溢出/空值临界缺陷即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("boundary", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          12,
			ID:             "parameterized",
			ChineseName:    "参数化测试",
			EnglishName:    "Parameterized Testing",
			Purpose:        "使用多组数据运行同一测试逻辑",
			Category:       CategoryFunctional,
			Priority:       TierP1,
			Languages:      []string{"rust", "go", "typescript", "python"},
			TriggerSignals: []string{"has_table_or_matrix_inputs", "has_tests"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "rstest", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --workspace -- param", RequiresHarness: true, HarnessKey: "rstest", HarnessHint: "使用 rstest #[case(...)] 构造多组输入参数矩阵", ReportFormat: "stdout", InstallHint: "cargo add --dev rstest"},
				{Language: "go", ToolName: "go table-driven", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -v ./...", RequiresHarness: true, HarnessKey: "go_tests", HarnessHint: "编写 []struct 表驱动测试用例", ReportFormat: "stdout", InstallHint: "Go 原生表驱动模式"},
				{Language: "python", ToolName: "pytest parametrize", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest", RequiresHarness: true, HarnessKey: "py_tests", HarnessHint: "使用 @pytest.mark.parametrize", ReportFormat: "junit", InstallHint: "pip install pytest"},
				{Language: "typescript", ToolName: "vitest each", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run", RequiresHarness: true, HarnessKey: "ts_tests", HarnessHint: "使用 it.each([...])", ReportFormat: "json", InstallHint: "npm i -D vitest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "通过的参数化向量行数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "失败的参数向量数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, RequireHarnessForPass: true, Summary: "任意参数矩阵向量断言失败即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("parameterized", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          13,
			ID:             "combinatorial",
			ChineseName:    "组合测试",
			EnglishName:    "Combinatorial Testing",
			Purpose:        "检查不同参数组合产生的缺陷",
			Category:       CategoryFunctional,
			Priority:       TierP2,
			Languages:      []string{"rust", "go", "typescript", "python"},
			TriggerSignals: []string{"has_feature_or_option_matrix", "has_boundary_constraints"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "combinatorial generator", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --workspace -- combinatorial", RequiresHarness: true, HarnessKey: "combinatorial", HarnessHint: "构建成对/多因子正交组合测试 (含 combinatorial)", ReportFormat: "stdout", InstallHint: "使用 itertools/rstest 正交组合生成器"},
				{Language: "go", ToolName: "go pairwise harness", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -run Combinatorial ./...", RequiresHarness: true, HarnessKey: "combinatorial", HarnessHint: "编写 TestCombinatorial* 成对因子组合测试", ReportFormat: "stdout", InstallHint: "Go 原生组合生成器"},
				{Language: "python", ToolName: "allpairspy", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest -k combinatorial", RequiresHarness: true, HarnessKey: "combinatorial", HarnessHint: "使用 allpairspy 生成成对参数组合", ReportFormat: "junit", InstallHint: "pip install allpairspy pytest"},
				{Language: "typescript", ToolName: "pairwise harness", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run -t combinatorial", RequiresHarness: true, HarnessKey: "combinatorial", HarnessHint: "编写成对选项矩阵组合测试", ReportFormat: "json", InstallHint: "npm i -D vitest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "通过的因子组合用例数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "发生冲突或故障的参数组合数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, RequireHarnessForPass: true, Summary: "任意参数或特性组合触发冲突、panic 或错误即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("combinatorial", []string{"output.log", "metrics.json"}),
		},
	}
}
