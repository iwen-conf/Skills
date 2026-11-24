package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// computeCodeGate 基于不可变账本收据评估代码测试计划是否满足指定层级与三大易忽略测试门禁
// 请求体
// ledger 验证账本实例指针
// planID 代码测试计划ID
// runID 可选运行批次ID（为空时汇总当前计划版本下各技术最新收据）
// minTier 门禁要求覆盖的最高优先级上限
// requireOverlooked 是否强制要求三大易忽略测试（mutation/property/fuzz）全部通过
// 响应体
// report 结构化门禁判定报告指针
// err 计划不存在或账本查询错误
func computeCodeGate(ledger *StoryLedger, planID, runID string, minTier CodePriorityTier, requireOverlooked bool) (*CodeGateReport, error) {
	// 1. 加载计划定义、确定生效门禁层级并读取历史FAIL计数
	plan, err := ledger.LoadCodePlan(planID)
	if err != nil {
		return nil, fmt.Errorf("code plan %q not found in ledger: %w", planID, err)
	}
	if minTier == TierUnspecified {
		minTier = plan.MaxTier
	}
	if minTier == TierUnspecified {
		minTier = TierP1
	}
	histFails, err := ledger.CountHistoricalCodeFails(planID)
	if err != nil {
		return nil, err
	}
	results, err := ledger.ListCodeResults(planID, runID, "")
	if err != nil {
		return nil, err
	}
	resolvedRunID := runID
	if resolvedRunID == "" {
		if latest, lErr := ledger.LatestCodeRun(planID); lErr == nil && latest != nil {
			resolvedRunID = latest.RunID
		}
	}

	// 2. 提取当前计划版本下各技术的最新执行收据
	latestByTech := map[string]CodeResultRecord{}
	for _, r := range results {
		if r.PlanSHA256 == plan.PlanSHA256 {
			latestByTech[r.TechniqueID] = r
		}
	}

	// 3. 统计必需项与三大易忽略测试的当前达标状态
	rep := &CodeGateReport{
		PlanID:              plan.PlanID,
		PlanSHA256:          plan.PlanSHA256,
		RunID:               resolvedRunID,
		MinTier:             minTier,
		ProofScope:          "code_level_tier_" + string(minTier),
		InputsUnchanged:     true,
		HistoricalFailCount: histFails,
		OverlookedStatus:    map[string]string{},
	}
	overlookedSet := map[string]bool{"mutation": true, "property": true, "fuzz": true}
	for _, t := range plan.Techniques {
		if overlookedSet[t.TechniqueID] {
			if rec, ok := latestByTech[t.TechniqueID]; ok {
				rep.OverlookedStatus[t.TechniqueID] = rec.Verdict
			} else {
				rep.OverlookedStatus[t.TechniqueID] = string(t.Status)
			}
		}
		effTier := t.EffectivePriority
		if effTier == TierUnspecified {
			effTier = t.Priority
		}
		mustPass := (t.Status == SelectRequired && tierRank(effTier) <= tierRank(minTier)) ||
			(requireOverlooked && overlookedSet[t.TechniqueID] && t.Status != SelectNotApplicable)
		if !mustPass {
			continue
		}
		rep.RequiredCount++
		rec, ok := latestByTech[t.TechniqueID]
		if !ok {
			rep.NotRunCount++
			rep.MissingTechniques = append(rep.MissingTechniques, t.TechniqueID)
			continue
		}
		switch Verdict(rec.Verdict) {
		case VerdictPass:
			rep.PassedCount++
		case VerdictFail:
			rep.FailedCount++
			rep.MissingTechniques = append(rep.MissingTechniques, t.TechniqueID)
		default:
			rep.BlockedCount++
			rep.MissingTechniques = append(rep.MissingTechniques, t.TechniqueID)
		}
	}

	// 4. 按零失败、零阻塞且必选项全通过得出总门禁判定
	switch {
	case rep.FailedCount > 0:
		rep.Verdict = string(VerdictFail)
		rep.Reason = fmt.Sprintf("%d required code technique(s) failed: %s", rep.FailedCount, strings.Join(rep.MissingTechniques, ", "))
	case len(latestByTech) == 0 || rep.RequiredCount == 0:
		rep.Verdict = string(VerdictNotRun)
		rep.Reason = "no code-level test execution recorded for current plan revision"
	case rep.BlockedCount > 0 || rep.NotRunCount > 0:
		rep.Verdict = string(VerdictBlocked)
		rep.Reason = fmt.Sprintf("%d blocked and %d not-run required technique(s): %s", rep.BlockedCount, rep.NotRunCount, strings.Join(rep.MissingTechniques, ", "))
	default:
		rep.Verdict = string(VerdictPass)
	}
	return rep, nil
}

// explainCodeVerify 组装指定代码测试计划及运行批次的完整台账分析报告
// 请求体
// ledger 验证账本实例指针
// planID 代码测试计划ID（为空时取最新计划）
// runID 可选运行批次ID（为空时取最新运行批次）
// 响应体
// report 完整代码测试审计台账报告指针
// err 计划未找到或数据库查询错误
func explainCodeVerify(ledger *StoryLedger, planID, runID string) (*CodeExplainReport, error) {
	// 1. 加载目标或最新代码验证计划
	var (
		plan *CodeVerifyPlan
		err  error
	)
	if strings.TrimSpace(planID) != "" {
		plan, err = ledger.LoadCodePlan(planID)
	} else {
		plan, err = ledger.LatestCodePlan()
	}
	if err != nil {
		return nil, fmt.Errorf("code plan not found: %w", err)
	}

	// 2. 定位运行批次、明细收据、历史FAIL计数与门禁判定
	var runRec *CodeRunRecord
	if strings.TrimSpace(runID) != "" {
		runRec, err = ledger.GetCodeRun(runID)
		if err != nil {
			return nil, err
		}
	} else {
		runRec, err = ledger.LatestCodeRun(plan.PlanID)
		if err != nil {
			return nil, err
		}
		if runRec != nil {
			runID = runRec.RunID
		}
	}
	results, err := ledger.ListCodeResults(plan.PlanID, runID, "")
	if err != nil {
		return nil, err
	}
	histFails, err := ledger.CountHistoricalCodeFails(plan.PlanID)
	if err != nil {
		return nil, err
	}
	gateRep, err := computeCodeGate(ledger, plan.PlanID, runID, plan.MaxTier, false)
	if err != nil {
		return nil, err
	}
	return &CodeExplainReport{
		PlanID:              plan.PlanID,
		PlanSHA256:          plan.PlanSHA256,
		RepoRoot:            plan.RepoRoot,
		PrimaryLanguage:     plan.PrimaryLanguage,
		LatestRun:           runRec,
		Plan:                plan,
		Results:             results,
		HistoricalFailCount: histFails,
		Gate:                gateRep,
	}, nil
}

// exportCodeSnapshot 导出包含8项覆盖率面板、三大易忽略测试卡片与26项矩阵的自包含只读HTML+JSON工作台
// 请求体
// ledger 验证账本实例指针
// planID 目标计划ID
// runID 可选运行批次ID
// outDir 必须为空或不存在的输出目录路径
// 响应体
// err 目录校验失败、计划不存在或文件写入错误
func exportCodeSnapshot(ledger *StoryLedger, planID, runID, outDir string) error {
	// 1. 校验输出目录不存在或为空后创建目录
	if outDir == "" {
		return fmt.Errorf("--out directory is required")
	}
	if entries, err := os.ReadDir(outDir); err == nil && len(entries) > 0 {
		return fmt.Errorf("snapshot output directory %s must be absent or empty", outDir)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	// 2. 加载完整审计数据组装 codeSnapshotDocument 并落盘 snapshot.json 与 index.html
	rep, err := explainCodeVerify(ledger, planID, runID)
	if err != nil {
		return err
	}
	doc := codeSnapshotDocument{
		SchemaVersion:   1,
		Kind:            "arc_code_verify_snapshot",
		Plan:            rep.Plan,
		LatestRun:       rep.LatestRun,
		Results:         rep.Results,
		Gate:            rep.Gate,
		CoverageMetrics: allCoverageMetrics(),
		ExportedAt:      nowUTC(),
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "snapshot.json"), append(body, '\n'), 0o644); err != nil {
		return err
	}
	page := renderCodeSnapshotHTML(doc, body)
	return os.WriteFile(filepath.Join(outDir, "index.html"), []byte(page), 0o644)
}
