package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// profileRepository 扫描目标仓库语言分布、清单配置、已有夹具、代码风险信号与环境工具链
// 请求体
// repoRoot 目标代码仓库路径
// probeTools 是否执行本地工具链可用性探测
// 响应体
// profile 完整仓库代码级验证画像指针
// err 路径解析或目录遍历错误
func profileRepository(repoRoot string, probeTools bool) (*RepoCodeProfile, error) {
	// 1. 解析归一化仓库路径并初始化剖析画像结构
	absRoot := canonicalRepoRoot(repoRoot)
	profile := &RepoCodeProfile{
		RepoRoot:          absRoot,
		SourceFileCounts:  map[string]int{},
		TestFileCounts:    map[string]int{},
		DetectedHarnesses: map[string][]string{},
		ToolsProbed:       probeTools,
	}
	signalHits := map[string][]RiskSignalHit{}
	manifestSeen := map[string]bool{}
	featureSeen := map[string]bool{}
	memberSeen := map[string]bool{}
	harnessSeen := map[string]map[string]bool{}
	hasRustLib := false
	hasRustBin := false

	addHarness := func(key, path string) {
		if harnessSeen[key] == nil {
			harnessSeen[key] = map[string]bool{}
		}
		if !harnessSeen[key][path] {
			harnessSeen[key][path] = true
			profile.DetectedHarnesses[key] = append(profile.DetectedHarnesses[key], path)
		}
	}
	addSignal := func(sig, evidence, file string, line int) {
		if len(signalHits[sig]) >= 3 {
			return
		}
		for _, prev := range signalHits[sig] {
			if prev.File == file && prev.Line == line {
				return
			}
		}
		signalHits[sig] = append(signalHits[sig], RiskSignalHit{Signal: sig, Evidence: evidence, File: file, Line: line})
	}

	// 2. 遍历仓库源码与清单文件，采集真实带行号的结构与风险信号
	walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, werr error) error {
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
		base := filepath.Base(rel)
		lowBase := strings.ToLower(base)
		ext := strings.ToLower(filepath.Ext(rel))

		if isCodeManifestFile(lowBase) && !manifestSeen[rel] {
			manifestSeen[rel] = true
			profile.Manifests = append(profile.Manifests, rel)
			scanManifestSignals(rel, lowBase, path, featureSeen, memberSeen, addSignal)
		}
		if strings.Contains(lowRel, "fuzz/fuzz_targets/") && ext == ".rs" {
			target := strings.TrimSuffix(base, ".rs")
			addHarness("fuzz_targets", target)
			addSignal("has_fuzz_targets", "fuzz target "+rel, rel, 1)
		}
		if strings.HasPrefix(lowRel, "benches/") && ext == ".rs" {
			addHarness("bench_targets", rel)
			addSignal("has_bench_harness", "benchmark file "+rel, rel, 1)
		}
		if (strings.HasPrefix(lowRel, "tests/") || strings.Contains(lowRel, "/tests/")) && ext == ".rs" {
			addHarness("rust_integration_tests", rel)
			profile.TestFileCounts["rust"]++
		}

		lang := detectSourceLanguage(ext)
		if lang == "" {
			return nil
		}
		testFile := isLanguageTestFile(lang, lowRel, lowBase)
		if testFile {
			if !(lang == "rust" && (strings.HasPrefix(lowRel, "tests/") || strings.Contains(lowRel, "/tests/"))) {
				profile.TestFileCounts[lang]++
			}
		} else {
			profile.SourceFileCounts[lang]++
		}
		if lang == "rust" {
			if lowBase == "lib.rs" {
				hasRustLib = true
			}
			if lowBase == "main.rs" || strings.Contains(lowRel, "src/bin/") {
				hasRustBin = true
			}
		}
		raw, rErr := os.ReadFile(path)
		if rErr != nil || len(raw) > 2<<20 {
			return nil
		}
		scanSourceCodeFile(lang, rel, string(raw), testFile, addHarness, addSignal)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	// 3. 整理清单、特性、多模块统计与语言排序，并附加基线信号
	sort.Strings(profile.Manifests)
	for f := range featureSeen {
		profile.CargoFeatures = append(profile.CargoFeatures, f)
	}
	sort.Strings(profile.CargoFeatures)
	for m := range memberSeen {
		profile.WorkspaceMembers = append(profile.WorkspaceMembers, m)
	}
	sort.Strings(profile.WorkspaceMembers)
	for k := range profile.DetectedHarnesses {
		sort.Strings(profile.DetectedHarnesses[k])
	}
	if hasRustLib {
		addHarness("rust_lib", "src/lib.rs")
	}
	if hasRustBin {
		addHarness("rust_bin", "src/main.rs")
	}

	profile.Languages, profile.PrimaryLanguage = rankDetectedLanguages(profile.SourceFileCounts, profile.TestFileCounts, profile.Manifests)
	if len(profile.Manifests) > 0 {
		addSignal("always_baseline", "project manifest "+profile.Manifests[0], profile.Manifests[0], 1)
	} else {
		addSignal("always_baseline", "repository source tree", ".", 1)
	}
	totalTests := 0
	for _, cnt := range profile.TestFileCounts {
		totalTests += cnt
	}
	if totalTests > 0 || len(profile.DetectedHarnesses["inline_tests"]) > 0 {
		src := "."
		if len(profile.Manifests) > 0 {
			src = profile.Manifests[0]
		}
		addSignal("has_tests", fmt.Sprintf("detected %d test file(s)/inline test block(s)", totalTests+len(profile.DetectedHarnesses["inline_tests"])), src, 1)
	}
	if len(profile.WorkspaceMembers) > 1 || len(profile.SourceFileCounts) > 1 || countTotalValues(profile.SourceFileCounts) >= 3 {
		src := "."
		if len(profile.Manifests) > 0 {
			src = profile.Manifests[0]
		}
		addSignal("has_multi_module", "multi-file or multi-crate workspace structure", src, 1)
	}

	var sigKeys []string
	for k := range signalHits {
		sigKeys = append(sigKeys, k)
	}
	sort.Strings(sigKeys)
	for _, k := range sigKeys {
		profile.RiskSignals = append(profile.RiskSignals, signalHits[k]...)
	}

	// 4. 按需批量探测本机已安装测试工具链
	if probeTools {
		cache := buildToolProbeCache(profile.Languages)
		profile.ToolProbes = collectToolProbeStatuses(profile.Languages, cache)
	}
	return profile, nil
}

// buildCodeVerifyPlan 基于仓库画像、优先级上限与风险提级策略自动装配26项代码级测试计划
// 请求体
// repoRoot 目标代码仓库路径
// opts 计划生成控制参数
// 响应体
// plan 编译完成且含内容指纹的代码测试计划指针
// err 目录剖析或参数错误
func buildCodeVerifyPlan(repoRoot string, opts CodePlanOptions) (*CodeVerifyPlan, error) {
	// 1. 剖析仓库语言、夹具、风险信号与工具状态
	maxTier := opts.MaxTier
	if maxTier == TierUnspecified {
		maxTier = TierP1
	}
	profile, err := profileRepository(repoRoot, opts.ProbeTools)
	if err != nil {
		return nil, err
	}
	var cache *toolProbeCache
	if opts.ProbeTools {
		cache = buildToolProbeCache(profile.Languages)
	}
	targetLang := strings.ToLower(strings.TrimSpace(opts.Language))
	if targetLang == "" {
		targetLang = profile.PrimaryLanguage
	}
	planID := strings.TrimSpace(opts.PlanID)
	if planID == "" {
		base := sanitizeNS(filepath.Base(profile.RepoRoot))
		if base == "" || base == "default" {
			base = "repo"
		}
		planID = base + "_code_verify"
	}

	// 2. 构建命中信号映射并逐项评估26个测试技术
	sigMap := map[string]bool{}
	for _, hit := range profile.RiskSignals {
		sigMap[hit.Signal] = true
	}
	plan := &CodeVerifyPlan{
		PlanID:          planID,
		RepoRoot:        profile.RepoRoot,
		PrimaryLanguage: targetLang,
		MaxTier:         maxTier,
		Profile:         *profile,
		CreatedAt:       nowUTC(),
	}
	for _, spec := range allCodeTechniques() {
		planned := evaluateTechniqueForPlan(spec, targetLang, maxTier, opts, profile, sigMap, cache)
		plan.Techniques = append(plan.Techniques, planned)
		switch planned.Status {
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
	plan.PlanSHA256 = computeCodePlanSHA256(plan)
	return plan, nil
}
