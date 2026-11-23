package main

import (
	"fmt"
	"sort"
)

// computeCompletion 基于不可变账本历史计算世界标志、路由、蝴蝶探针与结局四项100%完成度
// 请求体
// ledger 验证账本实例指针
// s 故事线定义指针
// worldScan 可选仓库耦合扫描结果指针
// 响应体
// report 四维完成度与判定报告指针
// err 读取历史路由收据失败错误
func computeCompletion(ledger *StoryLedger, s *Storyline, worldScan *WorldScanResult) (*CompletionReport, error) {
	// 1. 读取全量历史路由收据，统计永久保留的历史FAIL数并提取当前版本各路由最新收据
	records, err := ledger.ListRouteResults(s.StoryID)
	if err != nil {
		return nil, err
	}
	latestByRoute := map[string]RouteResultRecord{}
	historicalFails := 0
	for _, rec := range records {
		if Verdict(rec.Verdict) == VerdictFail {
			historicalFails++
		}
		if rec.StorySHA256 == s.SHA256 {
			latestByRoute[rec.RouteID] = rec
		}
	}
	// 2. 遍历声明路由统计通过率、软锁数、蝴蝶探针命中率与已解锁结局集合
	routeStatuses := map[string]string{}
	routesPassed := 0
	unresolvedFails := 0
	softlockCount := 0
	probesTotal := 0
	probesPassed := 0
	unlockedMap := map[string]bool{}
	for _, r := range s.Routes {
		declProbes := 0
		for _, st := range r.Sequence {
			declProbes += len(st.ButterflyProbes)
		}
		probesTotal += declProbes
		rec, ok := latestByRoute[r.RouteID]
		if !ok {
			routeStatuses[r.RouteID] = string(VerdictNotRun)
			continue
		}
		routeStatuses[r.RouteID] = rec.Verdict
		if rec.SoftlockDetected {
			softlockCount++
		}
		switch Verdict(rec.Verdict) {
		case VerdictPass:
			routesPassed++
			probesPassed += min(declProbes, rec.ProbesPassed)
			if rec.UnlockedEnding != "" {
				unlockedMap[rec.UnlockedEnding] = true
			}
		case VerdictFail:
			unresolvedFails++
		}
	}
	// 3. 计算世界状态绑定率、路由覆盖率、探针命中率与必达结局解锁率四项百分比
	worldPct := 100.0
	if len(s.WorldFlags) == 0 {
		worldPct = 0.0
	} else if worldScan != nil && len(worldScan.Candidates) > 0 {
		lint := lintStory(s, worldScan, 100)
		missing := 0
		for _, e := range lint.Errors {
			if e.Code == "UNBOUND_WORLD_FLAG" {
				missing++
			}
		}
		bound := max(0, len(worldScan.Candidates)-missing)
		worldPct = float64(bound) * 100.0 / float64(len(worldScan.Candidates))
	}
	routePct := 0.0
	if len(s.Routes) > 0 {
		routePct = float64(routesPassed) * 100.0 / float64(len(s.Routes))
	}
	probePct := 100.0
	if probesTotal > 0 {
		probePct = float64(probesPassed) * 100.0 / float64(probesTotal)
	}
	var unlockedList, missingList []string
	for _, req := range s.RequiredEndings {
		if unlockedMap[req] {
			unlockedList = append(unlockedList, req)
		} else {
			missingList = append(missingList, req)
		}
	}
	sort.Strings(unlockedList)
	sort.Strings(missingList)
	endingPct := 0.0
	if len(s.RequiredEndings) > 0 {
		endingPct = float64(len(unlockedList)) * 100.0 / float64(len(s.RequiredEndings))
	}
	// 4. 按零软锁、零未决失败与四维全部100%规则得出总判定结果
	verdict := string(VerdictPass)
	reason := ""
	switch {
	case len(latestByRoute) == 0:
		verdict = string(VerdictNotRun)
		reason = "no route executions recorded for current storyline revision"
	case softlockCount > 0:
		verdict = string(VerdictFail)
		reason = fmt.Sprintf("%d softlock violation(s) detected", softlockCount)
	case unresolvedFails > 0:
		verdict = string(VerdictFail)
		reason = fmt.Sprintf("%d route(s) currently failing", unresolvedFails)
	case worldPct < 100 || routePct < 100 || probePct < 100 || endingPct < 100:
		verdict = string(VerdictBlocked)
		reason = fmt.Sprintf("incomplete coverage (world=%.0f%% routes=%.0f%% probes=%.0f%% endings=%.0f%%)",
			worldPct, routePct, probePct, endingPct)
	}
	return &CompletionReport{
		StoryID:              s.StoryID,
		StorySHA256:          s.SHA256,
		Verdict:              verdict,
		WorldFlagBindingPct:  worldPct,
		RouteCoveragePct:     routePct,
		ButterflyProbeHitPct: probePct,
		EndingUnlockPct:      endingPct,
		SoftlockCount:        softlockCount,
		UnresolvedFailCount:  unresolvedFails,
		HistoricalFailCount:  historicalFails,
		UnlockedEndings:      unlockedList,
		MissingEndings:       missingList,
		RouteStatuses:        routeStatuses,
		Reason:               reason,
	}, nil
}
