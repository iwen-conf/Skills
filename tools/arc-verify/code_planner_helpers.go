package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	reRustDocTest = regexp.MustCompile("///\\s*```")
	reGoFuzzFunc  = regexp.MustCompile(`\bfunc\s+Fuzz[A-Za-z0-9_]*\s*\(`)
	reGoBenchFunc = regexp.MustCompile(`\bfunc\s+Benchmark[A-Za-z0-9_]*\s*\(`)
	reGoExamFunc  = regexp.MustCompile(`\bfunc\s+Example[A-Za-z0-9_]*\s*\(`)
	rePyDefTest   = regexp.MustCompile(`\bdef\s+test_[A-Za-z0-9_]*`)
)

// toolProbeCache 缓存单次剖析期间对各语言工具链、Python模块与CLI探针命令的探测结果
type toolProbeCache struct {
	// cargoReady 是否存在 cargo 主命令
	cargoReady bool
	// cargoVersion cargo 版本输出行
	cargoVersion string
	// cargoSubcmds cargo --list 解析出的可用子命令映射
	cargoSubcmds map[string]bool
	// nightlyReady 是否已安装 nightly 工具链
	nightlyReady bool
	// miriReady nightly 工具链下是否已安装 miri 组件
	miriReady bool
	// binAvailable 按二进制名缓存的 PATH 可用状态
	binAvailable map[string]bool
	// binVersion 按二进制名缓存的版本或路径信息
	binVersion map[string]string
	// pyProbed 是否已完成 python3 模块探测
	pyProbed bool
	// pyModules 可用 Python 测试与静态模块集合
	pyModules map[string]bool
	// cmdProbed 已探测的 npx/python3 命令可用性缓存
	cmdProbed map[string]bool
	// cmdDetail 已探测命令的版本或失败诊断说明
	cmdDetail map[string]string
}

// isCodeManifestFile 判断文件名是否属于受支持的工程依赖或构建清单文件
// 请求体
// lowBase 小写基础文件名
// 响应体
// matched 是否为工程清单文件
func isCodeManifestFile(lowBase string) bool {
	// 1. 匹配主流语言构建与依赖锁文件
	switch lowBase {
	case "cargo.toml", "cargo.lock", "go.mod", "go.sum", "package.json", "tsconfig.json",
		"pyproject.toml", "requirements.txt", "cmakelists.txt", "package.swift", "pom.xml",
		"build.gradle", "build.gradle.kts":
		return true
	default:
		return false
	}
}

// detectSourceLanguage 根据文件扩展名映射编程语言标识
// 请求体
// ext 小写文件后缀（含点号）
// 响应体
// lang 规范化语言标识（未知扩展名返回空串）
func detectSourceLanguage(ext string) string {
	// 1. 按扩展名分组归类编程语言
	switch ext {
	case ".rs":
		return "rust"
	case ".go":
		return "go"
	case ".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs":
		return "typescript"
	case ".py":
		return "python"
	case ".c", ".cc", ".cpp", ".cxx", ".h", ".hpp":
		return "c_cpp"
	case ".swift":
		return "swift"
	default:
		return ""
	}
}

// isLanguageTestFile 判断给定相对路径是否为特定语言的测试文件
// 请求体
// lang 编程语言标识
// lowRel 小写仓库相对路径
// lowBase 小写基础文件名
// 响应体
// isTest 是否为测试文件
func isLanguageTestFile(lang, lowRel, lowBase string) bool {
	// 1. 识别各语言标准测试文件命名与目录约定
	if strings.Contains(lowRel, "/tests/") || strings.HasPrefix(lowRel, "tests/") ||
		strings.Contains(lowRel, "/__tests__/") || strings.Contains(lowRel, "/benches/") ||
		strings.Contains(lowRel, "fuzz/fuzz_targets/") {
		return true
	}
	switch lang {
	case "go":
		return strings.HasSuffix(lowBase, "_test.go")
	case "rust":
		return strings.HasSuffix(lowBase, "_test.rs")
	case "typescript":
		return strings.Contains(lowBase, ".test.") || strings.Contains(lowBase, ".spec.") || strings.Contains(lowBase, ".bench.")
	case "python":
		return strings.HasPrefix(lowBase, "test_") || strings.HasSuffix(lowBase, "_test.py") || lowBase == "conftest.py"
	default:
		return strings.Contains(lowBase, "_test.") || strings.Contains(lowBase, "test_")
	}
}

// scanManifestSignals 解析工程清单文件提取依赖、特性、工作区成员与安全配置信号
// 请求体
// rel 清单相对路径
// lowBase 小写文件名
// fullPath 清单绝对路径
// features 特性集合收集映射
// members 工作区成员收集映射
// addSignal 风险信号记录回调
// 响应体
// 无
func scanManifestSignals(
	rel, lowBase, fullPath string,
	features, members map[string]bool,
	addSignal func(sig, evidence, file string, line int),
) {
	// 1. 读取清单元数据并在超限或不可读时安全退出
	raw, err := os.ReadFile(fullPath)
	if err != nil || len(raw) > 2<<20 {
		return
	}
	lines := strings.Split(string(raw), "\n")

	// 2. 针对 Cargo.toml 解析段落型与键型依赖、特性及精确 [workspace].members
	if lowBase == "cargo.toml" {
		section := ""
		inWorkspaceMembers := false
		for idx, line := range lines {
			ln := idx + 1
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.HasPrefix(trimmed, "[") {
				inWorkspaceMembers = false
				section = strings.ToLower(trimmed)
				if section == "[workspace]" {
					addSignal("has_multi_module", "[workspace] section in Cargo.toml", rel, ln)
				}
				if section == "[features]" {
					addSignal("has_feature_or_option_matrix", "[features] matrix in Cargo.toml", rel, ln)
				}
				if crateKey := cargoSectionCrateName(section); crateKey != "" {
					classifyRustDependencyKey(crateKey, rel, ln, addSignal)
					addSignal("has_dependencies", "crate dependency "+crateKey, rel, ln)
				}
				continue
			}
			lowLine := strings.ToLower(trimmed)
			if strings.HasPrefix(lowLine, "rust-version") {
				addSignal("has_cross_target_or_msrv", trimmed, rel, ln)
			}
			if strings.Contains(section, "dependencies") || section == "" {
				key := strings.TrimSpace(strings.SplitN(trimmed, "=", 2)[0])
				key = strings.Trim(key, "\"'")
				classifyRustDependencyKey(key, rel, ln, addSignal)
				if key != "" && key != "edition" && key != "version" && key != "name" && strings.Contains(section, "dependencies") {
					addSignal("has_dependencies", "crate dependency "+key, rel, ln)
				}
			}
			if section == "[features]" {
				if eq := strings.IndexByte(trimmed, '='); eq > 0 {
					if feat := strings.TrimSpace(trimmed[:eq]); feat != "" {
						features[feat] = true
					}
				}
			}
			if section == "[workspace]" {
				inWorkspaceMembers = collectCargoWorkspaceMembers(trimmed, inWorkspaceMembers, members)
			}
		}
		return
	}

	// 3. 解析其它语言工程清单的依赖与并发/网络/测试信号
	for idx, line := range lines {
		ln := idx + 1
		low := strings.ToLower(strings.TrimSpace(line))
		if low == "" || strings.HasPrefix(low, "#") || strings.HasPrefix(low, "//") {
			continue
		}
		if strings.Contains(low, "require") || strings.Contains(low, "dependencies") || strings.Contains(low, "devdependencies") {
			addSignal("has_dependencies", "manifest dependencies", rel, ln)
		}
		if strings.Contains(low, "vitest") || strings.Contains(low, "pytest") || strings.Contains(low, "jest") ||
			strings.Contains(low, "rapid") || strings.Contains(low, "hypothesis") || strings.Contains(low, "fast-check") {
			addSignal("has_tests", "test framework declared in manifest", rel, ln)
		}
		if strings.Contains(low, "gin") || strings.Contains(low, "fastapi") || strings.Contains(low, "express") ||
			strings.Contains(low, "next") || strings.Contains(low, "pgx") {
			addSignal("has_network_or_service_entrypoint", "web/db dependency in manifest", rel, ln)
			addSignal("has_security_surface", "service/auth dependency in manifest", rel, ln)
		}
	}
}

// cargoSectionCrateName 提取 Cargo [dependencies.<crate>] 类节头中的依赖包名
// 请求体
// section 小写段落头文本
// 响应体
// crateName 解析出的依赖包名（非包级依赖段落返回空串）
func cargoSectionCrateName(section string) string {
	// 1. 剥离方括号并匹配 dependencies.<crate> 前缀
	inner := strings.TrimSuffix(strings.TrimPrefix(section, "["), "]")
	for _, prefix := range []string{"dependencies.", "dev-dependencies.", "build-dependencies.", "workspace.dependencies."} {
		if strings.HasPrefix(inner, prefix) {
			rest := strings.TrimPrefix(inner, prefix)
			if dot := strings.IndexByte(rest, '.'); dot >= 0 {
				rest = rest[:dot]
			}
			return strings.Trim(strings.TrimSpace(rest), "\"'")
		}
	}
	return ""
}

// collectCargoWorkspaceMembers 仅从 [workspace] 节的 members 键值中收集工作区成员路径
// 请求体
// trimmed 已去空白的当前行文本
// inMembers 当前是否处于多行 members 数组内部
// members 工作区成员收集映射
// 响应体
// nextInMembers 更新后的多行 members 状态
func collectCargoWorkspaceMembers(trimmed string, inMembers bool, members map[string]bool) bool {
	// 1. 识别 members = [...] 起始行，避免误采集 resolver/exclude 等无关字符串
	active := inMembers
	rhs := trimmed
	if eq := strings.IndexByte(trimmed, '='); eq >= 0 {
		key := strings.TrimSpace(trimmed[:eq])
		if key != "members" {
			return false
		}
		active = true
		rhs = strings.TrimSpace(trimmed[eq+1:])
	}
	if !active {
		return false
	}
	for i, part := range strings.Split(rhs, "\"") {
		if i%2 == 1 && part != "" {
			members[part] = true
		}
	}
	if strings.Contains(rhs, "]") {
		return false
	}
	return true
}

// classifyRustDependencyKey 根据 Cargo 依赖包名记录并发、编解码、解析器、网络与测试信号
// 请求体
// key 依赖包名
// rel 清单相对路径
// ln 所在行号
// addSignal 风险信号回调
// 响应体
// 无
func classifyRustDependencyKey(key, rel string, ln int, addSignal func(sig, evidence, file string, line int)) {
	// 1. 按代表性生态包名归类仓库技术与风险特征信号
	switch key {
	case "tokio", "async-std", "rayon", "crossbeam", "parking_lot", "dashmap", "loom":
		addSignal("has_concurrency", "dependency "+key, rel, ln)
	case "serde", "serde_json", "bincode", "prost", "rmp-serde", "toml", "serde_yaml":
		addSignal("has_codec_or_algebraic_logic", "serialization codec "+key, rel, ln)
		addSignal("has_serialized_outputs", "serialization crate "+key, rel, ln)
	case "nom", "winnow", "pest", "lalrpop", "bytes":
		addSignal("has_parser_or_untrusted_io", "parser crate "+key, rel, ln)
	case "axum", "actix-web", "warp", "rocket", "tonic", "hyper", "reqwest", "sqlx":
		addSignal("has_network_or_service_entrypoint", "network/db crate "+key, rel, ln)
		addSignal("has_security_surface", "service/db surface "+key, rel, ln)
	case "proptest", "quickcheck", "rstest", "insta", "criterion", "divan", "kani":
		addSignal("has_tests", "testing crate "+key, rel, ln)
		if key == "criterion" || key == "divan" {
			addSignal("has_hot_path_or_perf_budget", "benchmark crate "+key, rel, ln)
			addSignal("has_bench_harness", "benchmark crate "+key, rel, ln)
		}
		if key == "rstest" {
			addSignal("has_table_or_matrix_inputs", "parameterized crate "+key, rel, ln)
		}
		if key == "insta" {
			addSignal("has_serialized_outputs", "snapshot crate "+key, rel, ln)
		}
	}
}

// isPureCommentLine 判断源码行是否属于对应语言的纯注释行
// 请求体
// lang 编程语言标识
// trimmed 去首尾空白的源码行
// 响应体
// comment 若为纯注释行返回true
func isPureCommentLine(lang, trimmed string) bool {
	// 1. 排除 C/Rust/JS/Go 斜杠星号注释与 Python 井号注释（保留 Rust #[attr] 与 C #include）
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
		return true
	}
	if lang == "python" && strings.HasPrefix(trimmed, "#") {
		return true
	}
	return false
}

// scanSourceCodeFile 扫描单份源码文件的并发、unsafe、解析、边界、状态机、文档与全部测试夹具信号
// 请求体
// lang 编程语言标识
// rel 仓库相对路径
// text 源码文本内容
// isTest 是否为测试文件
// addHarness 夹具发现记录回调
// addSignal 风险信号记录回调
// 响应体
// 无
func scanSourceCodeFile(
	lang, rel, text string,
	isTest bool,
	addHarness func(key, path string),
	addSignal func(sig, evidence, file string, line int),
) {
	// 1. 登记各语言测试文件夹具与独立 .bench. 文件标记
	lines := strings.Split(text, "\n")
	inInlineTest := false
	lowBase := strings.ToLower(filepath.Base(rel))
	if isTest {
		addHarness(lang+"_tests", rel)
		switch lang {
		case "python":
			addHarness("py_tests", rel)
		case "typescript":
			addHarness("ts_tests", rel)
		}
	}
	if lang == "typescript" && strings.Contains(lowBase, ".bench.") {
		addHarness("ts_bench", rel)
		addSignal("has_bench_harness", "benchmark file "+rel, rel, 1)
	}

	// 2. 逐行扫描多语言测试夹具、注释隔离的专项测试用例与生产风险特征
	for idx, line := range lines {
		ln := idx + 1
		trimmed := strings.TrimSpace(line)
		low := strings.ToLower(trimmed)

		if lang == "rust" && strings.Contains(trimmed, "#[cfg(test)]") {
			inInlineTest = true
		}
		if lang == "rust" && (strings.Contains(trimmed, "#[test]") || strings.Contains(trimmed, "#[tokio::test]")) {
			addHarness("inline_tests", rel)
		}
		if strings.Contains(trimmed, "proptest!") {
			addHarness("proptest", rel)
		}
		if strings.Contains(trimmed, "insta::") || strings.Contains(trimmed, "assert_snapshot!") {
			addHarness("insta", rel)
		}
		if strings.Contains(trimmed, "loom::model") {
			addHarness("loom", rel)
		}
		if strings.Contains(trimmed, "#[rstest]") || strings.Contains(trimmed, "rstest::") {
			addHarness("rstest", rel)
		}
		if strings.Contains(trimmed, "#[kani::proof]") {
			addHarness("kani", rel)
			addSignal("has_formal_harness", "#[kani::proof] detected", rel, ln)
		}
		if lang == "go" {
			if reGoFuzzFunc.MatchString(trimmed) {
				addHarness("go_fuzz", rel)
			}
			if reGoBenchFunc.MatchString(trimmed) {
				addHarness("go_bench", rel)
			}
			if reGoExamFunc.MatchString(trimmed) {
				addHarness("go_examples", rel)
				addSignal("has_public_api_or_docs", trimmed, rel, ln)
			}
			if strings.Contains(trimmed, "rapid.Check") {
				addHarness("go_property", rel)
			}
			if isTest && !isPureCommentLine(lang, trimmed) && (strings.Contains(trimmed, "testdata") || strings.Contains(trimmed, "Snapshot")) {
				addHarness("go_snapshot", rel)
			}
		}
		if lang == "python" {
			if strings.HasPrefix(trimmed, "@given") || strings.Contains(low, "from hypothesis") || strings.Contains(low, "import hypothesis") {
				addHarness("py_hypothesis", rel)
			}
			if strings.Contains(low, "atheris.setup") || strings.Contains(low, "import atheris") {
				addHarness("py_fuzz", rel)
				addSignal("has_fuzz_targets", trimmed, rel, ln)
			}
			if strings.Contains(low, "pytest.mark.benchmark") || (isTest && (strings.Contains(trimmed, "benchmark(") || strings.Contains(low, "def test_bench"))) {
				addHarness("py_bench", rel)
				addSignal("has_bench_harness", trimmed, rel, ln)
			}
			if isTest && !isPureCommentLine(lang, trimmed) && (strings.Contains(low, "syrupy") || (rePyDefTest.MatchString(low) && strings.Contains(low, "snapshot"))) {
				addHarness("py_snapshot", rel)
			}
		}
		if lang == "typescript" {
			if strings.Contains(trimmed, "fc.property") || strings.Contains(trimmed, "fc.assert") || strings.Contains(low, "fast-check") {
				addHarness("ts_fastcheck", rel)
			}
			if strings.Contains(trimmed, "export function fuzz(") || strings.Contains(trimmed, "export async function fuzz(") || strings.Contains(low, "@jazzer.js") {
				addHarness("ts_fuzz", rel)
				addSignal("has_fuzz_targets", trimmed, rel, ln)
			}
			if isTest && !isPureCommentLine(lang, trimmed) && (strings.HasPrefix(trimmed, "bench(") || strings.Contains(trimmed, "bench.")) {
				addHarness("ts_bench", rel)
				addSignal("has_bench_harness", trimmed, rel, ln)
			}
			if strings.Contains(trimmed, "toMatchSnapshot(") || strings.Contains(trimmed, "toMatchInlineSnapshot(") {
				addHarness("ts_snapshot", rel)
			}
		}
		if lang == "c_cpp" {
			if strings.Contains(trimmed, "LLVMFuzzerTestOneInput") {
				addHarness("c_fuzz", rel)
				addSignal("has_fuzz_targets", trimmed, rel, ln)
			}
			if strings.Contains(trimmed, "__CPROVER_") {
				addHarness("cbmc", rel)
				addSignal("has_formal_harness", trimmed, rel, ln)
			}
		}

		if isTest || inInlineTest {
			if isPureCommentLine(lang, trimmed) {
				continue
			}
			if strings.Contains(low, "concurrent") || strings.Contains(trimmed, "Promise.all") ||
				strings.Contains(trimmed, "asyncio.gather") || strings.Contains(trimmed, "threading.Barrier") ||
				(lang == "go" && (strings.Contains(trimmed, "go func") || strings.Contains(trimmed, "sync.WaitGroup"))) {
				addHarness("concurrent", rel)
			}
			switch {
			case strings.Contains(low, "boundary"):
				addHarness("boundary", rel)
				addSignal("has_boundary_constraints", trimmed, rel, ln)
			case strings.Contains(low, "differential"):
				addHarness("differential", rel)
				addSignal("has_reference_or_dual_impl", trimmed, rel, ln)
			case strings.Contains(low, "metamorphic"):
				addHarness("metamorphic", rel)
				addSignal("has_numeric_or_search_transform", trimmed, rel, ln)
			case strings.Contains(low, "combinatorial"):
				addHarness("combinatorial", rel)
				addSignal("has_feature_or_option_matrix", trimmed, rel, ln)
			case strings.Contains(low, "stress"):
				addHarness("stress", rel)
				addSignal("has_hot_path_or_perf_budget", trimmed, rel, ln)
			case strings.Contains(low, "soak"):
				addHarness("soak", rel)
				addSignal("has_network_or_service_entrypoint", trimmed, rel, ln)
			case strings.Contains(low, "model_based") || strings.Contains(low, "modelbased"):
				addHarness("model_based", rel)
				addSignal("has_state_machine_or_protocol", trimmed, rel, ln)
			}
			continue
		}

		// 3. 扫描生产代码里的真实高风险特征
		if (lang == "rust" && (strings.Contains(trimmed, "unsafe {") || strings.Contains(trimmed, "unsafe fn ") || strings.Contains(trimmed, "extern \"C\""))) ||
			(lang == "go" && strings.Contains(trimmed, "import \"unsafe\"")) ||
			lang == "c_cpp" {
			addSignal("has_unsafe_or_ffi", trimmed, rel, ln)
		}
		if strings.Contains(trimmed, "Arc<") || strings.Contains(trimmed, "Mutex") || strings.Contains(trimmed, "RwLock") ||
			strings.Contains(trimmed, "Atomic") || strings.Contains(trimmed, "tokio::spawn") || strings.Contains(trimmed, "thread::spawn") ||
			strings.Contains(trimmed, "go func") || strings.Contains(trimmed, "sync.") || strings.Contains(trimmed, "chan ") ||
			strings.Contains(trimmed, "Promise.all") || strings.Contains(trimmed, "asyncio.gather") || strings.Contains(trimmed, "threading.") {
			addSignal("has_concurrency", trimmed, rel, ln)
		}
		if strings.Contains(low, "from_slice") || strings.Contains(low, "from_bytes") || strings.Contains(low, "json.unmarshal") ||
			strings.Contains(low, "yaml.unmarshal") || strings.Contains(low, "decode(") || strings.Contains(low, "parse") {
			addSignal("has_parser_or_untrusted_io", trimmed, rel, ln)
		}
		if strings.Contains(low, "encode") || strings.Contains(low, "decode") || strings.Contains(low, "serialize") ||
			strings.Contains(low, "deserialize") || strings.Contains(low, "compress") || strings.Contains(low, "encrypt") {
			addSignal("has_codec_or_algebraic_logic", trimmed, rel, ln)
			addSignal("has_serialized_outputs", trimmed, rel, ln)
		}
		if strings.Contains(low, "password") || strings.Contains(low, "secret") || strings.Contains(low, "jwt") ||
			strings.Contains(low, "hmac") || strings.Contains(low, "sql") || strings.Contains(low, "exec.command") ||
			strings.Contains(low, "process::command") {
			addSignal("has_security_surface", trimmed, rel, ln)
		}
		if strings.Contains(low, "statemachine") || strings.Contains(low, "state_machine") || strings.Contains(low, "transition") ||
			strings.Contains(low, "protocol") {
			addSignal("has_state_machine_or_protocol", trimmed, rel, ln)
		}
		if strings.Contains(low, "min_") || strings.Contains(low, "max_") || strings.Contains(low, "saturating_") ||
			strings.Contains(low, "checked_") || strings.Contains(low, "overflow") || strings.Contains(low, "limit") {
			addSignal("has_boundary_constraints", trimmed, rel, ln)
		}
		if lang == "rust" && reRustDocTest.MatchString(trimmed) {
			addSignal("has_public_api_or_docs", "rustdoc executable example", rel, ln)
		}
		if lang == "rust" && strings.Contains(trimmed, "pub fn ") {
			addSignal("has_public_api_or_docs", trimmed, rel, ln)
		}
		if strings.Contains(low, "balance") || strings.Contains(low, "ledger") || strings.Contains(low, "order") ||
			strings.Contains(low, "refund") || strings.Contains(low, "consensus") {
			addSignal("has_critical_core_logic", trimmed, rel, ln)
		}
	}
}

// rankDetectedLanguages 按源码数量、测试数量与工程清单权重对检测到的语言排序
// 请求体
// srcCounts 生产源码计数映射
// testCounts 测试文件计数映射
// manifests 清单文件相对路径切片
// 响应体
// langs 按权重降序排列的语言切片
// primary 主开发语言标识
func rankDetectedLanguages(srcCounts, testCounts map[string]int, manifests []string) ([]string, string) {
	// 1. 聚合各语言权重并按权重降序稳定排序
	weights := map[string]int{}
	for lang, n := range srcCounts {
		weights[lang] += n * 10
	}
	for lang, n := range testCounts {
		weights[lang] += n * 5
	}
	for _, m := range manifests {
		low := strings.ToLower(filepath.Base(m))
		switch low {
		case "cargo.toml":
			weights["rust"] += 50
		case "go.mod":
			weights["go"] += 50
		case "package.json", "tsconfig.json":
			weights["typescript"] += 40
		case "pyproject.toml", "requirements.txt":
			weights["python"] += 40
		case "cmakelists.txt":
			weights["c_cpp"] += 40
		case "package.swift":
			weights["swift"] += 40
		}
	}
	var langs []string
	for lang := range weights {
		langs = append(langs, lang)
	}
	sort.Slice(langs, func(i, j int) bool {
		if weights[langs[i]] != weights[langs[j]] {
			return weights[langs[i]] > weights[langs[j]]
		}
		return langs[i] < langs[j]
	})
	primary := "rust"
	if len(langs) > 0 {
		primary = langs[0]
	}
	return langs, primary
}

// countTotalValues 统计字符串整数字典的总和
// 请求体
// counts 整型计数字典
// 响应体
// total 累计和
func countTotalValues(counts map[string]int) int {
	// 1. 累加映射中的全部分项计数
	total := 0
	for _, v := range counts {
		total += v
	}
	return total
}

// buildToolProbeCache 批量探测本机已安装的工具链、Cargo子命令、Nightly组件及Python插件状态
// 请求体
// languages 仓库检测到的语言列表
// 响应体
// cache 填充完成的工具探针缓存结构指针
func buildToolProbeCache(languages []string) *toolProbeCache {
	// 1. 初始化缓存容器并定位 PATH 中的常用测试工具
	cache := &toolProbeCache{
		cargoSubcmds: map[string]bool{},
		binAvailable: map[string]bool{},
		binVersion:   map[string]string{},
		pyModules:    map[string]bool{},
		cmdProbed:    map[string]bool{},
		cmdDetail:    map[string]string{},
	}
	for _, bin := range []string{"go", "golangci-lint", "govulncheck", "gremlins", "python3", "pytest", "ruff", "pip-audit", "mutmut", "npm", "npx", "semgrep", "ctest", "clang++", "clang-tidy", "llvm-cov", "cbmc", "swift"} {
		if p, err := exec.LookPath(bin); err == nil {
			cache.binAvailable[bin] = true
			cache.binVersion[bin] = p
		}
	}
	// 2. 当仓库涉及 Python 或全量探测时，通过单次 python3 探针检查插件可用性
	needPython := len(languages) == 0 || slices.Contains(languages, "python")
	if needPython && cache.binAvailable["python3"] {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		cmd := exec.CommandContext(ctx, "python3", "-c",
			"import importlib.util,sys; print(','.join(m for m in sys.argv[1:] if importlib.util.find_spec(m)))",
			"pytest", "pytest_cov", "hypothesis", "pytest_benchmark", "syrupy", "atheris", "mutmut", "ruff",
		)
		out, err := cmd.CombinedOutput()
		cancel()
		if err == nil {
			cache.pyProbed = true
			for _, m := range strings.Split(strings.TrimSpace(string(out)), ",") {
				if mod := strings.TrimSpace(m); mod != "" {
					cache.pyModules[mod] = true
				}
			}
		}
	}
	// 3. 针对 Rust 项目仅调用轻量探针获取全量 Cargo 子命令与 Nightly/Miri 状态
	needRust := len(languages) == 0 || slices.Contains(languages, "rust")
	if needRust {
		if out, err := exec.Command("cargo", "--version").CombinedOutput(); err == nil {
			cache.cargoReady = true
			cache.cargoVersion = strings.TrimSpace(string(out))
			cache.binAvailable["cargo"] = true
			cache.binVersion["cargo"] = cache.cargoVersion
		}
		if cache.cargoReady {
			if out, err := exec.Command("cargo", "--list").CombinedOutput(); err == nil {
				for _, line := range strings.Split(string(out), "\n") {
					fields := strings.Fields(line)
					if len(fields) > 0 && !strings.HasSuffix(fields[0], ":") {
						cache.cargoSubcmds[fields[0]] = true
					}
				}
			}
			if out, err := exec.Command("rustup", "toolchain", "list").CombinedOutput(); err == nil {
				if strings.Contains(string(out), "nightly") {
					cache.nightlyReady = true
				}
			}
			if cache.nightlyReady {
				if out, err := exec.Command("rustup", "component", "list", "--installed", "--toolchain", "nightly").CombinedOutput(); err == nil {
					if strings.Contains(string(out), "miri") {
						cache.miriReady = true
					}
				}
			}
		}
	}
	return cache
}

// probeAdapterAvailability 利用探针缓存实时判定单个工具适配器及所需子模块是否可执行
// 请求体
// adapter 工具适配器定义
// cache 工具探针缓存指针
// 响应体
// available 工具与所需工具链是否就绪
// detail 工具状态说明行
func probeAdapterAvailability(adapter CodeToolAdapter, cache *toolProbeCache) (bool, string) {
	// 1. 若探针缓存未初始化则回退基础 PATH 检测
	if cache == nil {
		if _, err := exec.LookPath(adapter.ProbeBin); err == nil {
			return true, adapter.ProbeBin
		}
		return false, "not probed"
	}
	// 2. 对 Cargo 子命令与 Nightly 需求执行精确探测
	if adapter.ProbeBin == "cargo" {
		if !cache.cargoReady {
			return false, "cargo not found on PATH"
		}
		if adapter.RequiresNightly && !cache.nightlyReady {
			return false, "nightly toolchain not installed"
		}
		if len(adapter.ProbeArgs) > 0 {
			sub := adapter.ProbeArgs[0]
			if sub == "+nightly" && len(adapter.ProbeArgs) > 1 {
				sub = adapter.ProbeArgs[1]
			}
			switch sub {
			case "llvm-cov", "fuzz", "mutants", "nextest", "audit", "deny", "clippy", "kani", "hack":
				if !cache.cargoSubcmds[sub] {
					return false, "cargo subcommand '" + sub + "' not installed"
				}
			case "miri":
				if !cache.miriReady {
					return false, "rustup nightly miri component not installed"
				}
			}
		}
		return true, cache.cargoVersion
	}
	// 3. 对 npx 与 python3 带参数探针执行带3秒超时及结果缓存的真实子进程探测
	if !cache.binAvailable[adapter.ProbeBin] {
		return false, adapter.ProbeBin + " not found on PATH"
	}
	if (adapter.ProbeBin == "npx" || adapter.ProbeBin == "python3") && len(adapter.ProbeArgs) > 0 {
		cmdKey := adapter.ProbeBin + " " + strings.Join(adapter.ProbeArgs, " ")
		if cachedOK, seen := cache.cmdProbed[cmdKey]; seen {
			return cachedOK, cache.cmdDetail[cmdKey]
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		out, err := exec.CommandContext(ctx, adapter.ProbeBin, adapter.ProbeArgs...).CombinedOutput()
		cancel()
		detail := strings.TrimSpace(string(out))
		ok := err == nil
		if !ok {
			if detail == "" {
				detail = cmdKey + " probe failed"
			}
		} else if detail == "" {
			detail = cmdKey
		}
		cache.cmdProbed[cmdKey] = ok
		cache.cmdDetail[cmdKey] = detail
		return ok, detail
	}
	// 4. 对依赖 Python 插件的 pytest 适配器校验证实插件模块已安装
	if adapter.ProbeBin == "pytest" && cache.pyProbed {
		requiredMod := requiredPythonPluginForTool(adapter.ToolName)
		if requiredMod != "" && !cache.pyModules[requiredMod] {
			return false, "python module '" + requiredMod + "' not installed"
		}
	}
	return true, cache.binVersion[adapter.ProbeBin]
}

// requiredPythonPluginForTool 映射特定 Python 测试适配器依赖的可选导入模块名
// 请求体
// toolName 适配器工具名称
// 响应体
// mod 需要验证的 Python 模块名（无需额外插件返回空串）
func requiredPythonPluginForTool(toolName string) string {
	// 1. 匹配带外部插件依赖的 pytest 适配器
	low := strings.ToLower(toolName)
	switch {
	case strings.Contains(low, "pytest-cov"):
		return "pytest_cov"
	case strings.Contains(low, "hypothesis"):
		return "hypothesis"
	case strings.Contains(low, "pytest-benchmark"):
		return "pytest_benchmark"
	case strings.Contains(low, "syrupy"):
		return "syrupy"
	default:
		return ""
	}
}

// collectToolProbeStatuses 汇总当前仓库语言关联的所有代表性测试工具探测状态
// 请求体
// languages 仓库涉及语言列表
// cache 工具探针缓存指针
// 响应体
// statuses 工具探测状态记录切片
func collectToolProbeStatuses(languages []string, cache *toolProbeCache) []ToolProbeStatus {
	// 1. 去重收集目标语言对应的工具适配器并填充探测结果
	langSet := map[string]bool{}
	for _, l := range languages {
		langSet[l] = true
	}
	seen := map[string]bool{}
	var out []ToolProbeStatus
	for _, spec := range allCodeTechniques() {
		for _, tool := range spec.Tools {
			if len(langSet) > 0 && !langSet[tool.Language] && tool.Language != "multi" {
				continue
			}
			key := tool.Language + "|" + tool.ToolName
			if seen[key] {
				continue
			}
			seen[key] = true
			ok, detail := probeAdapterAvailability(tool, cache)
			out = append(out, ToolProbeStatus{
				Language:    tool.Language,
				ToolName:    tool.ToolName,
				ProbeCmd:    strings.TrimSpace(tool.ProbeBin + " " + strings.Join(tool.ProbeArgs, " ")),
				Available:   ok,
				VersionLine: detail,
				InstallHint: tool.InstallHint,
			})
		}
	}
	return out
}

// evaluateTechniqueForPlan 评估单个测试技术在目标仓库中的生效优先级、命令解析与调度状态
// 请求体
// spec 测试技术规格
// lang 目标编程语言
// maxTier 计划最高层级上限
// opts 计划生成选项
// profile 仓库剖析画像指针
// sigMap 已命中的仓库信号集合
// cache 工具探针缓存指针
// 响应体
// item 评估后的单项计划条目
func evaluateTechniqueForPlan(
	spec CodeTechniqueSpec,
	lang string,
	maxTier CodePriorityTier,
	opts CodePlanOptions,
	profile *RepoCodeProfile,
	sigMap map[string]bool,
	cache *toolProbeCache,
) PlannedTechnique {
	// 1. 匹配语言工具适配器，无适配语言直接判 NOT_APPLICABLE
	item := PlannedTechnique{
		Index:             spec.Index,
		TechniqueID:       spec.ID,
		ChineseName:       spec.ChineseName,
		EnglishName:       spec.EnglishName,
		Priority:          spec.Priority,
		EffectivePriority: spec.Priority,
		TopOverlooked:     spec.TopOverlooked,
		Language:          lang,
		FailureCriteria:   spec.FailureCriteria,
		ReportFormats:     spec.ReportFormats,
		EvidencePath:      spec.EvidenceStorage.ArtifactDirTemplate,
	}
	adapter, ok := selectToolAdapter(spec, lang)
	if !ok {
		item.Status = SelectNotApplicable
		item.Reason = fmt.Sprintf("technique %s does not target %s", spec.ID, lang)
		return item
	}
	item.ToolName = adapter.ToolName
	item.Command = adapter.DefaultCmd
	item.InstallHint = adapter.InstallHint
	item.HarnessRequired = adapter.RequiresHarness
	item.ToolAvailable, _ = probeAdapterAvailability(adapter, cache)
	item.Command, item.ToolName, item.ToolAvailable = resolveDynamicToolCommand(spec.ID, lang, adapter, profile, cache, item.ToolAvailable)

	// 2. 检测仓库触发信号与风险提级信号
	for _, sig := range spec.TriggerSignals {
		if sigMap[sig] {
			item.TriggeredBy = append(item.TriggeredBy, sig)
		}
	}
	for _, rSig := range spec.RiskPromoteSignals {
		if sigMap[rSig] {
			item.PromotedByRisk = true
			item.EffectivePriority = TierP0
			if !slices.Contains(item.TriggeredBy, rSig) {
				item.TriggeredBy = append(item.TriggeredBy, rSig)
			}
		}
	}
	if !adapter.RequiresHarness {
		item.HarnessReady = true
	} else if profile != nil {
		item.HarnessReady = len(profile.DetectedHarnesses[adapter.HarnessKey]) > 0
		if !item.HarnessReady && spec.ID == "property" && len(profile.DetectedHarnesses["proptest"]) > 0 {
			item.HarnessReady = true
		}
	}

	// 3. 按风险提级、P0基线、层级上限、三大易忽略提示与夹具缺失状态确定最终调度决策
	applicable := len(item.TriggeredBy) > 0 || spec.Priority == TierP0
	if !applicable && !opts.IncludeAllApplicable {
		item.Status = SelectNotApplicable
		item.Reason = "no matching repository trigger signals"
		return item
	}
	withinTier := opts.IncludeAllApplicable || tierRank(spec.Priority) <= tierRank(maxTier) || item.PromotedByRisk
	if !withinTier {
		if spec.TopOverlooked && len(item.TriggeredBy) > 0 {
			item.Status = SelectRecommended
			item.Reason = "top overlooked technique matched signals (" + strings.Join(item.TriggeredBy, ", ") + ")"
			if item.HarnessRequired && !item.HarnessReady {
				item.MissingPrerequisite = adapter.HarnessHint
			}
			return item
		}
		item.Status = SelectDeferred
		item.Reason = fmt.Sprintf("tier %s exceeds requested max_tier %s", spec.Priority, maxTier)
		return item
	}
	if item.HarnessRequired && !item.HarnessReady && !opts.Strict && !item.PromotedByRisk {
		item.Status = SelectRecommended
		item.MissingPrerequisite = adapter.HarnessHint
		item.Reason = "harness not yet authored in repository; add harness to enforce in gate"
		return item
	}
	item.Status = SelectRequired
	if item.PromotedByRisk {
		item.Reason = "risk-promoted to P0 due to " + strings.Join(item.TriggeredBy, ", ")
	} else if len(item.TriggeredBy) > 0 {
		item.Reason = "selected by tier " + string(spec.Priority) + " (" + strings.Join(item.TriggeredBy, ", ") + ")"
	} else {
		item.Reason = "P0 baseline requirement"
	}
	if !item.ToolAvailable {
		item.MissingPrerequisite = "missing tool: " + item.ToolName
	} else if item.HarnessRequired && !item.HarnessReady {
		item.MissingPrerequisite = adapter.HarnessHint
	}
	return item
}

// resolveDynamicToolCommand 根据仓库结构与已装工具对默认命令执行务实解析与优雅回退
// 请求体
// techID 测试技术ID
// lang 目标编程语言
// adapter 默认工具适配器
// profile 仓库画像指针
// cache 工具探针缓存指针
// toolAvail 原始工具可用状态
// 响应体
// cmd 解析后的可执行命令
// toolName 解析后的工具名称
// avail 更新后的工具可用状态
func resolveDynamicToolCommand(
	techID, lang string,
	adapter CodeToolAdapter,
	profile *RepoCodeProfile,
	cache *toolProbeCache,
	toolAvail bool,
) (string, string, bool) {
	// 1. 针对 Rust 仓库根据 lib/bin、nextest、fuzz target 与 cargo-hack 动态调整命令
	cmd := adapter.DefaultCmd
	toolName := adapter.ToolName
	avail := toolAvail
	if lang == "rust" && profile != nil {
		hasLib := len(profile.DetectedHarnesses["rust_lib"]) > 0
		hasBin := len(profile.DetectedHarnesses["rust_bin"]) > 0
		if techID == "unit" && !hasLib && hasBin {
			cmd = "cargo test --workspace --bins"
		}
		if techID == "regression" && !avail && cache != nil && cache.cargoReady {
			cmd = "cargo test --workspace"
			toolName = "cargo test (fallback)"
			avail = true
		}
		if techID == "fuzz" && len(profile.DetectedHarnesses["fuzz_targets"]) > 0 {
			cmd = "cargo +nightly fuzz run " + profile.DetectedHarnesses["fuzz_targets"][0] + " -- -max_total_time=30"
		}
		if techID == "compatibility" && cache != nil && cache.cargoSubcmds["hack"] {
			cmd = "cargo hack check --feature-powerset"
			toolName = "cargo-hack"
			avail = true
		}
	}
	// 2. 针对 Go 仓库在缺失 golangci-lint 时回退标准 go vet ./...
	if lang == "go" && techID == "static_analysis" && !avail && cache != nil && cache.binAvailable["go"] {
		cmd = "go vet ./..."
		toolName = "go vet (fallback)"
		avail = true
	}
	return cmd, toolName, avail
}

// computeCodePlanSHA256 清除易变环境与主机路径字段后计算代码验证计划的确定性SHA256摘要
// 请求体
// plan 代码测试计划结构体指针
// 响应体
// digest 64位十六进制SHA256摘要
func computeCodePlanSHA256(plan *CodeVerifyPlan) string {
	// 1. 复制值结构体并归零 RepoRoot、ToolProbes、CreatedAt 与来源路径以保障跨工作树与探测状态确定性
	cp := *plan
	cp.PlanSHA256 = ""
	cp.SourcePath = ""
	cp.CreatedAt = ""
	cp.RepoRoot = ""
	cp.Profile.RepoRoot = ""
	cp.Profile.ToolProbes = nil
	raw, _ := json.Marshal(cp)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// loadCodeVerifyPlan 从YAML或JSON文件加载代码级测试计划并重算内容摘要
// 请求体
// path 计划文件路径
// 响应体
// plan 解析后的代码验证计划指针
// err 文件读取或解码错误
func loadCodeVerifyPlan(path string) (*CodeVerifyPlan, error) {
	// 1. 按扩展名执行 JSON 或 YAML 反序列化
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read code verify plan %s: %w", path, err)
	}
	var plan CodeVerifyPlan
	if strings.EqualFold(filepath.Ext(path), ".json") {
		if err := json.Unmarshal(raw, &plan); err != nil {
			return nil, fmt.Errorf("parse JSON code verify plan %s: %w", path, err)
		}
	} else {
		if err := yaml.Unmarshal(raw, &plan); err != nil {
			return nil, fmt.Errorf("parse YAML code verify plan %s: %w", path, err)
		}
	}
	// 2. 规范化来源路径、统计计数与SHA256内容摘要
	if abs, absErr := filepath.Abs(path); absErr == nil {
		plan.SourcePath = abs
	} else {
		plan.SourcePath = path
	}
	plan.RequiredCount, plan.RecommendedCount, plan.DeferredCount, plan.NotApplicableCount = 0, 0, 0, 0
	for _, t := range plan.Techniques {
		switch t.Status {
		case SelectRequired:
			plan.RequiredCount++
		case SelectRecommended:
			plan.RecommendedCount++
		case SelectDeferred:
			plan.DeferredCount++
		case SelectNotApplicable:
			plan.NotApplicableCount++
		}
	}
	plan.PlanSHA256 = computeCodePlanSHA256(&plan)
	return &plan, nil
}
