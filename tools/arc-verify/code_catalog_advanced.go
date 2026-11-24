package main

import (
	"slices"
	"strings"
)

// advancedCodeTechniques 返回目录序号#14至#26的并发、内存安全、性能、静态安全、形式化与文档兼容性测试技术
// 请求体
// 无
// 响应体
// specs 序号#14..#26的测试技术规格切片
func advancedCodeTechniques() []CodeTechniqueSpec {
	// 1. 定义#14-#26测试技术的七维完整规格
	return []CodeTechniqueSpec{
		{
			Index:              14,
			ID:                 "concurrency",
			ChineseName:        "并发测试",
			EnglishName:        "Concurrency Testing",
			Purpose:            "检测并发执行时的时序与同步错误",
			Category:           CategoryCorrectness,
			Priority:           TierP2,
			Languages:          []string{"rust", "go", "typescript", "python"},
			TriggerSignals:     []string{"has_concurrency"},
			RiskPromoteSignals: []string{"has_concurrency"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "loom", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "RUSTFLAGS=\"--cfg loom\" cargo test --release -- loom", RequiresHarness: true, HarnessKey: "loom", HarnessHint: "添加 loom 依赖并编写 loom::model(|| ...) 并发调度交错测试", ReportFormat: "stdout", InstallHint: "cargo add --dev loom"},
				{Language: "go", ToolName: "go test -race", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -race -count=5 ./...", ReportFormat: "stdout", InstallHint: "启用 CGO 与 Go -race 检测器"},
				{Language: "python", ToolName: "pytest concurrent", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest -k concurrent", RequiresHarness: true, HarnessKey: "concurrent", HarnessHint: "编写多线程/asyncio 栅栏交错测试", ReportFormat: "junit", InstallHint: "pip install pytest"},
				{Language: "typescript", ToolName: "vitest concurrent", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run -t concurrent", RequiresHarness: true, HarnessKey: "concurrent", HarnessHint: "编写 Promise.all 竞争与并发约束测试", ReportFormat: "json", InstallHint: "npm i -D vitest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "并发调度交错通过组数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "出现死锁、活锁或顺序违例的并发测试数"},
				{Name: "data_races", Unit: "count", Direction: DirExactZero, Description: "并发执行触发的数据竞争数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, MaxDataRaces: 0, RequireHarnessForPass: true, Summary: "存在任意时序违例、死锁或 data_races > 0 即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md", "stdout"},
			EvidenceStorage: codeEvidenceDefaults("concurrency", []string{"output.log", "metrics.json"}),
		},
		{
			Index:              15,
			ID:                 "data_race",
			ChineseName:        "数据竞争检测",
			EnglishName:        "Data Race Detection",
			Purpose:            "检测非同步内存访问",
			Category:           CategoryCorrectness,
			Priority:           TierP2,
			Languages:          []string{"rust", "go", "c_cpp", "swift"},
			TriggerSignals:     []string{"has_concurrency", "has_unsafe_or_ffi"},
			RiskPromoteSignals: []string{"has_concurrency"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "ThreadSanitizer", ProbeBin: "cargo", ProbeArgs: []string{"+nightly", "--version"}, DefaultCmd: "RUSTFLAGS=\"-Z sanitizer=thread\" cargo +nightly test", RequiresNightly: true, ReportFormat: "stdout", InstallHint: "rustup toolchain install nightly && rustup component add rust-src --toolchain nightly"},
				{Language: "go", ToolName: "go test -race", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -race ./...", ReportFormat: "stdout", InstallHint: "Go 原生支持（需开启 CGO_ENABLED=1）"},
				{Language: "c_cpp", ToolName: "TSan", ProbeBin: "clang++", ProbeArgs: []string{"--version"}, DefaultCmd: "ctest --output-on-failure", ReportFormat: "stdout", InstallHint: "以 -fsanitize=thread 编译并运行 CTest"},
				{Language: "swift", ToolName: "swift test --sanitize=thread", ProbeBin: "swift", ProbeArgs: []string{"--version"}, DefaultCmd: "swift test --sanitize=thread", ReportFormat: "stdout", InstallHint: "安装 Swift 工具链"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "data_races", Unit: "count", Direction: DirExactZero, Description: "ThreadSanitizer 或 Go race 检测器捕获的竞争次数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "竞争检测期间失败的用例数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, MaxDataRaces: 0, Summary: "任意 WARNING: DATA RACE / ThreadSanitizer: data race 零容忍直接判 FAIL"},
			ReportFormats:   []string{"json", "ai-md", "stdout"},
			EvidenceStorage: codeEvidenceDefaults("data_race", []string{"output.log", "metrics.json"}),
		},
		{
			Index:              16,
			ID:                 "memory_safety",
			ChineseName:        "内存安全测试",
			EnglishName:        "Memory Safety Testing",
			Purpose:            "检测越界、悬垂引用、未定义行为等",
			Category:           CategoryCorrectness,
			Priority:           TierP2,
			Languages:          []string{"rust", "c_cpp", "go", "swift"},
			TriggerSignals:     []string{"has_unsafe_or_ffi"},
			RiskPromoteSignals: []string{"has_unsafe_or_ffi"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "Miri + ASan", ProbeBin: "cargo", ProbeArgs: []string{"+nightly", "miri", "--version"}, DefaultCmd: "cargo +nightly miri test", RequiresNightly: true, ReportFormat: "stdout", InstallHint: "rustup +nightly component add miri"},
				{Language: "c_cpp", ToolName: "AddressSanitizer + UBSan", ProbeBin: "clang++", ProbeArgs: []string{"--version"}, DefaultCmd: "ctest --output-on-failure", ReportFormat: "stdout", InstallHint: "以 -fsanitize=address,undefined 编译测试目标"},
				{Language: "go", ToolName: "go checkptr + race", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -race -gcflags=all=-d=checkptr=2 ./...", ReportFormat: "stdout", InstallHint: "Go 原生支持 checkptr 指针合法性检查"},
				{Language: "swift", ToolName: "swift test --sanitize=address", ProbeBin: "swift", ProbeArgs: []string{"--version"}, DefaultCmd: "swift test --sanitize=address", ReportFormat: "stdout", InstallHint: "安装 Swift 工具链"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "ub_errors", Unit: "count", Direction: DirExactZero, Description: "Miri 或 ASan/UBSan 捕获的未定义行为与越界次数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "内存安全校验失败数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, MaxUBErrors: 0, Summary: "存在任意 Undefined Behavior / Stacked Borrows / ASan 报错零容忍判 FAIL"},
			ReportFormats:   []string{"json", "ai-md", "stdout"},
			EvidenceStorage: codeEvidenceDefaults("memory_safety", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          17,
			ID:             "benchmark",
			ChineseName:    "性能基准测试",
			EnglishName:    "Benchmark Testing",
			Purpose:        "测量代码性能与性能退化",
			Category:       CategoryPerformance,
			Priority:       TierP3,
			Languages:      []string{"rust", "go", "typescript", "python", "c_cpp"},
			TriggerSignals: []string{"has_hot_path_or_perf_budget", "has_bench_harness"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "criterion / divan", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo bench --workspace", RequiresHarness: true, HarnessKey: "bench_targets", HarnessHint: "添加 criterion/divan 并在 benches/*.rs 编写基准测试", ReportFormat: "json", InstallHint: "cargo add --dev criterion"},
				{Language: "go", ToolName: "go test -bench", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -bench=. -benchmem -run=^$ ./...", RequiresHarness: true, HarnessKey: "go_bench", HarnessHint: "编写 func BenchmarkXxx(b *testing.B)", ReportFormat: "stdout", InstallHint: "Go 原生 testing.B"},
				{Language: "typescript", ToolName: "vitest bench", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest bench --run", RequiresHarness: true, HarnessKey: "ts_bench", HarnessHint: "创建 *.bench.ts 性能基准文件", ReportFormat: "json", InstallHint: "npm i -D vitest"},
				{Language: "python", ToolName: "pytest-benchmark", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest --benchmark-only", RequiresHarness: true, HarnessKey: "py_bench", HarnessHint: "使用 pytest-benchmark fixture 编写基准测试", ReportFormat: "json", InstallHint: "pip install pytest-benchmark"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "bench_regression_pct", Unit: "pct", Direction: DirLowerIsBetter, Description: "相对于基线的延迟或吞吐退化百分比"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxRegressionPct: 10.0, RequireHarnessForPass: true, Summary: "基准运行失败或性能退化超过 10% 判 FAIL"},
			ReportFormats:   []string{"json", "html", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("benchmark", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          18,
			ID:             "stress",
			ChineseName:    "压力测试",
			EnglishName:    "Stress Testing",
			Purpose:        "检验极端负载下的代码行为",
			Category:       CategoryPerformance,
			Priority:       TierP3,
			Languages:      []string{"rust", "go", "typescript", "python"},
			TriggerSignals: []string{"has_network_or_service_entrypoint", "has_hot_path_or_perf_budget"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "custom stress harness", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --release --workspace -- stress", RequiresHarness: true, HarnessKey: "stress", HarnessHint: "编写高并发/极限负载测试驱动 (test name 含 stress)", ReportFormat: "json", InstallHint: "仓库自建压力测试 Harness"},
				{Language: "go", ToolName: "go stress harness", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -race -run Stress ./...", RequiresHarness: true, HarnessKey: "stress", HarnessHint: "编写 TestStress* 极限并发与队列背压测试", ReportFormat: "json", InstallHint: "Go testing"},
				{Language: "python", ToolName: "pytest stress", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest -k stress", RequiresHarness: true, HarnessKey: "stress", HarnessHint: "编写 test_stress_* 压力验证用例", ReportFormat: "json", InstallHint: "pip install pytest"},
				{Language: "typescript", ToolName: "vitest stress", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run -t stress", RequiresHarness: true, HarnessKey: "stress", HarnessHint: "编写 stress 极限负载用例", ReportFormat: "json", InstallHint: "npm i -D vitest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "crashes", Unit: "count", Direction: DirExactZero, Description: "高负载下发生的 panic/OOM/崩溃次数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "极限负载断言失败数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxCrashes: 0, MaxFailures: 0, RequireHarnessForPass: true, Summary: "极端负载下发生崩溃、OOM 或雪崩不可恢复即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("stress", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          19,
			ID:             "soak",
			ChineseName:    "稳定性测试",
			EnglishName:    "Soak Testing",
			Purpose:        "检测长时间执行后的内存及资源问题",
			Category:       CategoryPerformance,
			Priority:       TierP3,
			Languages:      []string{"rust", "go", "typescript", "python"},
			TriggerSignals: []string{"has_network_or_service_entrypoint"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "custom soak harness", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --release --workspace -- soak", RequiresHarness: true, HarnessKey: "soak", HarnessHint: "编写长稳循环压测并上报 rss_growth_pct (test name 含 soak)", ReportFormat: "json", InstallHint: "仓库自建稳定性测试 Harness"},
				{Language: "go", ToolName: "go soak harness", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -run Soak ./...", RequiresHarness: true, HarnessKey: "soak", HarnessHint: "编写 TestSoak* 内存/Goroutine/FD 泄漏检测用例", ReportFormat: "json", InstallHint: "Go testing + runtime.MemStats"},
				{Language: "python", ToolName: "pytest soak", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest -k soak", RequiresHarness: true, HarnessKey: "soak", HarnessHint: "编写 test_soak_* 资源泄漏长稳测试", ReportFormat: "json", InstallHint: "pip install pytest"},
				{Language: "typescript", ToolName: "vitest soak", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run -t soak", RequiresHarness: true, HarnessKey: "soak", HarnessHint: "编写 soak 资源增长检查测试", ReportFormat: "json", InstallHint: "npm i -D vitest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "rss_growth_pct", Unit: "pct", Direction: DirLowerIsBetter, Description: "长稳执行窗口期 RSS 内存增长率"},
				{Name: "crashes", Unit: "count", Direction: DirExactZero, Description: "长稳期间崩溃次数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxCrashes: 0, RequireHarnessForPass: true, Summary: "长稳运行发生泄露告警、句柄耗尽或进程崩溃即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("soak", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          20,
			ID:             "static_analysis",
			ChineseName:    "静态分析",
			EnglishName:    "Static Analysis",
			Purpose:        "不运行代码检查潜在问题",
			Category:       CategoryStaticSecurity,
			Priority:       TierP0,
			Languages:      []string{"rust", "go", "typescript", "python", "c_cpp"},
			TriggerSignals: []string{"always_baseline"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "clippy", ProbeBin: "cargo", ProbeArgs: []string{"clippy", "--version"}, DefaultCmd: "cargo clippy --workspace --all-targets -- -D warnings", ReportFormat: "sarif", InstallHint: "rustup component add clippy"},
				{Language: "go", ToolName: "golangci-lint", ProbeBin: "golangci-lint", ProbeArgs: []string{"version"}, DefaultCmd: "golangci-lint run", ReportFormat: "json", InstallHint: "go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest (回退 go vet ./...)"},
				{Language: "typescript", ToolName: "tsc", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "tsc", "--version"}, DefaultCmd: "npx tsc --noEmit", ReportFormat: "stdout", InstallHint: "npm i -D typescript"},
				{Language: "python", ToolName: "ruff", ProbeBin: "ruff", ProbeArgs: []string{"--version"}, DefaultCmd: "ruff check .", ReportFormat: "json", InstallHint: "pip install ruff"},
				{Language: "c_cpp", ToolName: "clang-tidy", ProbeBin: "clang-tidy", ProbeArgs: []string{"--version"}, DefaultCmd: `files=$(find . -maxdepth 4 -not -path '*/.*' -not -path './build*' \( -name '*.c' -o -name '*.cc' -o -name '*.cpp' \) | head -n 50); [ -n "$files" ] || { echo "no C/C++ files found"; exit 1; }; clang-tidy --quiet $files`, ReportFormat: "stdout", InstallHint: "安装 LLVM clang-tidy"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "warnings", Unit: "count", Direction: DirExactZero, Description: "静态检查与类型告警总数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxWarnings: 0, EnforceMaxWarnings: true, Summary: "存在任意 clippy -D warnings / vet / lint 告警或非零退出即判 FAIL"},
			ReportFormats:   []string{"json", "sarif", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("static_analysis", []string{"output.log", "metrics.json"}),
		},
		{
			Index:              21,
			ID:                 "sast",
			ChineseName:        "安全代码扫描",
			EnglishName:        "SAST",
			Purpose:            "发现代码安全缺陷",
			Category:           CategoryStaticSecurity,
			Priority:           TierP1,
			Languages:          []string{"rust", "go", "typescript", "python", "c_cpp"},
			TriggerSignals:     []string{"has_security_surface", "has_network_or_service_entrypoint"},
			RiskPromoteSignals: []string{"has_security_surface"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "semgrep", ProbeBin: "semgrep", ProbeArgs: []string{"--version"}, DefaultCmd: "semgrep --config p/default --metrics=off --error --json .", ReportFormat: "sarif", InstallHint: "pip install semgrep (或 brew install semgrep / CodeQL)"},
				{Language: "go", ToolName: "semgrep / gosec", ProbeBin: "semgrep", ProbeArgs: []string{"--version"}, DefaultCmd: "semgrep --config p/default --metrics=off --error --json .", ReportFormat: "sarif", InstallHint: "pip install semgrep 或 go install github.com/securego/gosec/v2/cmd/gosec@latest"},
				{Language: "typescript", ToolName: "semgrep", ProbeBin: "semgrep", ProbeArgs: []string{"--version"}, DefaultCmd: "semgrep --config p/default --metrics=off --error --json .", ReportFormat: "sarif", InstallHint: "pip install semgrep"},
				{Language: "python", ToolName: "semgrep / bandit", ProbeBin: "semgrep", ProbeArgs: []string{"--version"}, DefaultCmd: "semgrep --config p/default --metrics=off --error --json .", ReportFormat: "sarif", InstallHint: "pip install semgrep bandit"},
				{Language: "c_cpp", ToolName: "semgrep / codeql", ProbeBin: "semgrep", ProbeArgs: []string{"--version"}, DefaultCmd: "semgrep --config p/default --metrics=off --error --json .", ReportFormat: "sarif", InstallHint: "pip install semgrep"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "high_vulns", Unit: "count", Direction: DirExactZero, Description: "高危与严重级静态安全缺陷数"},
				{Name: "warnings", Unit: "count", Direction: DirLowerIsBetter, Description: "全部安全规则命中数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxHighVulns: 0, Summary: "存在任意 High/Critical/ERROR 级 SAST 漏洞即判 FAIL"},
			ReportFormats:   []string{"sarif", "json", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("sast", []string{"output.log", "metrics.json", "sast.sarif.json"}),
		},
		{
			Index:          22,
			ID:             "dependency_audit",
			ChineseName:    "依赖安全审计",
			EnglishName:    "Dependency Auditing",
			Purpose:        "检查依赖漏洞与供应链风险",
			Category:       CategoryStaticSecurity,
			Priority:       TierP0,
			Languages:      []string{"rust", "go", "typescript", "python"},
			TriggerSignals: []string{"has_dependencies", "always_baseline"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "cargo-audit / cargo-deny", ProbeBin: "cargo", ProbeArgs: []string{"audit", "--version"}, DefaultCmd: "cargo audit --json", ReportFormat: "json", InstallHint: "cargo install cargo-audit cargo-deny"},
				{Language: "go", ToolName: "govulncheck", ProbeBin: "govulncheck", ProbeArgs: []string{"-version"}, DefaultCmd: "govulncheck -json ./...", ReportFormat: "json", InstallHint: "go install golang.org/x/vuln/cmd/govulncheck@latest"},
				{Language: "typescript", ToolName: "npm audit", ProbeBin: "npm", ProbeArgs: []string{"--version"}, DefaultCmd: "npm audit --audit-level=high --json", ReportFormat: "json", InstallHint: "随 Node.js/npm 内置"},
				{Language: "python", ToolName: "pip-audit", ProbeBin: "pip-audit", ProbeArgs: []string{"--version"}, DefaultCmd: "pip-audit -f json", ReportFormat: "json", InstallHint: "pip install pip-audit"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "high_vulns", Unit: "count", Direction: DirExactZero, Description: "高危及供应链漏洞公告数"},
				{Name: "critical_vulns", Unit: "count", Direction: DirExactZero, Description: "严重漏洞数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxHighVulns: 0, Summary: "检出已知 CVE 漏洞、许可证违规或供应链投毒风险即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("dependency_audit", []string{"output.log", "metrics.json"}),
		},
		{
			Index:              23,
			ID:                 "formal_verification",
			ChineseName:        "形式化验证",
			EnglishName:        "Formal Verification",
			Purpose:            "数学化验证程序性质",
			Category:           CategoryFormalModel,
			Priority:           TierP3,
			Languages:          []string{"rust", "c_cpp"},
			TriggerSignals:     []string{"has_formal_harness", "has_unsafe_or_ffi"},
			RiskPromoteSignals: []string{"has_formal_harness"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "Kani / Prusti", ProbeBin: "cargo", ProbeArgs: []string{"kani", "--version"}, DefaultCmd: "cargo kani --workspace", RequiresHarness: true, HarnessKey: "kani", HarnessHint: "编写 #[kani::proof] 验证证明入口", ReportFormat: "stdout", InstallHint: "cargo install --locked kani-verifier && cargo kani setup"},
				{Language: "c_cpp", ToolName: "CBMC", ProbeBin: "cbmc", ProbeArgs: []string{"--version"}, DefaultCmd: `files=$(grep -rlE '__CPROVER_' . --include='*.c' --include='*.cpp' 2>/dev/null); [ -n "$files" ] || { echo "no CBMC __CPROVER_ harness found"; exit 1; }; cbmc --bounds-check --pointer-check $files`, RequiresHarness: true, HarnessKey: "cbmc", HarnessHint: "编写 __CPROVER_assert 验证函数入口", ReportFormat: "stdout", InstallHint: "安装 CBMC 形式化模型检查器"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "proofs_verified", Unit: "count", Direction: DirHigherIsBetter, Description: "数学证明成立的 proof harness 数"},
				{Name: "proofs_failed", Unit: "count", Direction: DirExactZero, Description: "反例证伪或不可判定的 proof 数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, RequireHarnessForPass: true, Summary: "任意形式化性质出现反例或证明失败即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md", "stdout"},
			EvidenceStorage: codeEvidenceDefaults("formal_verification", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          24,
			ID:             "model_based",
			ChineseName:    "模型测试",
			EnglishName:    "Model-based Testing",
			Purpose:        "根据程序行为模型生成测试",
			Category:       CategoryFormalModel,
			Priority:       TierP3,
			Languages:      []string{"rust", "go", "typescript", "python"},
			TriggerSignals: []string{"has_state_machine_or_protocol"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "proptest-state-machine", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --workspace -- model_based", RequiresHarness: true, HarnessKey: "model_based", HarnessHint: "编写状态转移参考模型并按命令序列对比执行 (含 model_based)", ReportFormat: "stdout", InstallHint: "cargo add --dev proptest-state-machine"},
				{Language: "go", ToolName: "rapid StateMachine", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -run ModelBased ./...", RequiresHarness: true, HarnessKey: "model_based", HarnessHint: "使用 rapid.StateMachineRules 编写状态模型测试", ReportFormat: "stdout", InstallHint: "go get pgregory.net/rapid"},
				{Language: "python", ToolName: "hypothesis RuleBasedStateMachine", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest -k model_based", RequiresHarness: true, HarnessKey: "model_based", HarnessHint: "继承 RuleBasedStateMachine 编写状态模型校验", ReportFormat: "junit", InstallHint: "pip install hypothesis"},
				{Language: "typescript", ToolName: "fast-check modelRun", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "vitest", "--version"}, DefaultCmd: "npx vitest run -t model_based", RequiresHarness: true, HarnessKey: "model_based", HarnessHint: "使用 fc.modelRun 编写命令模型对比测试", ReportFormat: "json", InstallHint: "npm i -D fast-check"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "状态转移序列与模型一致的验证组数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "与状态模型分歧的迁移序列数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, RequireHarnessForPass: true, Summary: "真实实现状态与抽象行为模型发生分歧即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md"},
			EvidenceStorage: codeEvidenceDefaults("model_based", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          25,
			ID:             "doctest",
			ChineseName:    "文档测试",
			EnglishName:    "Documentation Testing",
			Purpose:        "验证文档中的代码示例",
			Category:       CategoryDocsCompat,
			Priority:       TierP1,
			Languages:      []string{"rust", "go", "python"},
			TriggerSignals: []string{"has_public_api_or_docs"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "cargo test --doc", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo test --workspace --doc", ReportFormat: "stdout", InstallHint: "Rust 原生支持 rustdoc 代码块测试"},
				{Language: "go", ToolName: "go test Example", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go test -run Example ./...", ReportFormat: "stdout", InstallHint: "Go 原生 testing Example* 函数"},
				{Language: "python", ToolName: "pytest --doctest-modules", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest --doctest-modules", ReportFormat: "junit", InstallHint: "pip install pytest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "编译并运行通过的文档示例数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "失效的文档示例数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, Summary: "公开文档示例代码无法编译或断言失败即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md", "stdout"},
			EvidenceStorage: codeEvidenceDefaults("doctest", []string{"output.log", "metrics.json"}),
		},
		{
			Index:          26,
			ID:             "compatibility",
			ChineseName:    "兼容性测试",
			EnglishName:    "Compatibility Testing",
			Purpose:        "验证编译目标、特性组合或版本兼容",
			Category:       CategoryDocsCompat,
			Priority:       TierP2,
			Languages:      []string{"rust", "go", "typescript", "python"},
			TriggerSignals: []string{"has_feature_or_option_matrix", "has_cross_target_or_msrv"},
			Tools: []CodeToolAdapter{
				{Language: "rust", ToolName: "cargo check feature matrix", ProbeBin: "cargo", ProbeArgs: []string{"--version"}, DefaultCmd: "cargo check --workspace --all-features && cargo check --workspace --no-default-features", ReportFormat: "stdout", InstallHint: "可选安装 cargo install cargo-hack 以执行全幂集检查"},
				{Language: "go", ToolName: "go build cross-check", ProbeBin: "go", ProbeArgs: []string{"version"}, DefaultCmd: "go build ./... && GOOS=linux go vet ./...", ReportFormat: "stdout", InstallHint: "Go 原生交叉编译校验"},
				{Language: "typescript", ToolName: "tsc matrix", ProbeBin: "npx", ProbeArgs: []string{"--no-install", "tsc", "--version"}, DefaultCmd: "npx tsc --noEmit", ReportFormat: "stdout", InstallHint: "npm i -D typescript"},
				{Language: "python", ToolName: "tox / pytest", ProbeBin: "pytest", ProbeArgs: []string{"--version"}, DefaultCmd: "pytest", ReportFormat: "junit", InstallHint: "pip install tox pytest"},
			},
			Metrics: []CodeMetricSpec{
				{Name: "tests_passed", Unit: "count", Direction: DirHigherIsBetter, Description: "通过的特性组合或目标平台检查项数"},
				{Name: "tests_failed", Unit: "count", Direction: DirExactZero, Description: "失败的特性组合或目标构建数"},
			},
			FailureCriteria: CodeFailureCriteria{FailOnNonZeroExit: true, MaxFailures: 0, Summary: "任意 feature 组合、MSRV 或目标平台编译失败即判 FAIL"},
			ReportFormats:   []string{"json", "ai-md", "stdout"},
			EvidenceStorage: codeEvidenceDefaults("compatibility", []string{"output.log", "metrics.json"}),
		},
	}
}

// findCodeTechnique 按唯一标识精确查找单条代码级测试技术规格
// 请求体
// id 测试技术唯一标识
// 响应体
// spec 匹配的测试技术规格
// found 是否找到匹配条目
func findCodeTechnique(id string) (CodeTechniqueSpec, bool) {
	// 1. 规范化标识并遍历26项标准目录返回匹配项
	needle := strings.ToLower(strings.TrimSpace(id))
	for _, spec := range allCodeTechniques() {
		if spec.ID == needle {
			return spec, true
		}
	}
	return CodeTechniqueSpec{}, false
}

// filterCodeTechniques 按语言、最高优先级及单项ID过滤代码级测试技术目录
// 请求体
// lang 目标语言过滤串（为空不限）
// maxTier 最高优先级上限（为空不限）
// techniqueID 单项技术ID过滤（为空不限）
// 响应体
// out 满足全部筛选条件的有序技术列表
func filterCodeTechniques(lang string, maxTier CodePriorityTier, techniqueID string) []CodeTechniqueSpec {
	// 1. 顺序遍历全部26项技术并依次校验单项ID、优先级上限与语言适用性
	normLang := strings.ToLower(strings.TrimSpace(lang))
	normID := strings.ToLower(strings.TrimSpace(techniqueID))
	var out []CodeTechniqueSpec
	for _, spec := range allCodeTechniques() {
		if normID != "" && spec.ID != normID {
			continue
		}
		if maxTier != TierUnspecified && tierRank(spec.Priority) > tierRank(maxTier) {
			continue
		}
		if normLang != "" && !slices.Contains(spec.Languages, normLang) && !slices.Contains(spec.Languages, "multi") {
			continue
		}
		out = append(out, spec)
	}
	return out
}

// selectToolAdapter 为给定测试技术与目标编程语言匹配最合适的工具适配器
// 请求体
// spec 测试技术规格
// lang 目标编程语言
// 响应体
// adapter 选中的工具适配器
// ok 是否存在可用适配器
func selectToolAdapter(spec CodeTechniqueSpec, lang string) (CodeToolAdapter, bool) {
	// 1. 优先精确匹配目标语言适配器，其次回退 multi 通用适配器
	norm := strings.ToLower(strings.TrimSpace(lang))
	for _, tool := range spec.Tools {
		if tool.Language == norm {
			return tool, true
		}
	}
	for _, tool := range spec.Tools {
		if tool.Language == "multi" {
			return tool, true
		}
	}
	return CodeToolAdapter{}, false
}
