package main

import (
	"encoding/json"
	"strings"
)

// isRevisionCurrent 校验已记录的仓库提交与工作树摘要是否仍与当前源码一致
// 请求体
// root 规范化后的目标仓库根目录
// recordedCommit 收据记录的Git提交哈希
// recordedTree 收据记录的工作树摘要
// 响应体
// current 未发生漂移返回true
// curHead 当前仓库HEAD哈希或NO_GIT
// curTree 当前工作树实时摘要
func isRevisionCurrent(root, ledgerPath, recordedCommit, recordedTree string) (current bool, curHead string, curTree string) {
	// 1. 采集当前提交与工作树内容摘要
	curHead = currentGitHead(root)
	curTree = sourceWorktreeDigestExcluding(root, ledgerPath)
	// 2. 强制要求收据具备非空提交与树摘要且双双精确匹配
	if recordedCommit == "" || recordedCommit != curHead {
		return false, curHead, curTree
	}
	if recordedTree == "" || recordedTree != curTree {
		return false, curHead, curTree
	}
	return true, curHead, curTree
}

// emitStoryTaskProof 校验故事线单路由或全量路由收据并输出绑定当前修订的task-proof证明
// 请求体
// ledger 验证账本实例指针
// s 故事线定义指针
// graphID 故事线或图ID
// taskID 任务完整ID
// routeID 已解析的目标路由ID或all通配标识
// 响应体
// exitCode 进程退出状态码
func emitStoryTaskProof(ledger *StoryLedger, s *Storyline, graphID, taskID, routeID string) int {
	// 1. 查询故事线历史路由收据并按目标路由或全量路由归集最新通过记录
	recs, err := ledger.ListRouteResults(graphID)
	if err != nil {
		return emitStoryProofRejection(graphID, taskID, routeID, "NOT_RUN", "", "failed to list storyline receipts: "+err.Error(), nil)
	}
	latestByRoute := map[string]RouteResultRecord{}
	for _, rec := range recs {
		if rec.StorySHA256 == s.SHA256 {
			latestByRoute[rec.RouteID] = rec
		}
	}
	if routeID == "all" {
		return emitStoryAllRoutesProof(ledger, s, graphID, taskID, latestByRoute)
	}
	latest, ok := latestByRoute[routeID]
	if !ok || Verdict(latest.Verdict) != VerdictPass || latest.SoftlockDetected {
		return emitStoryProofRejection(graphID, taskID, routeID, "", "", "no current passing storyline route receipt", nil)
	}
	// 2. 校验收据绑定仓库根目录的当前提交与脏文件树均未漂移
	targetRoot := canonicalRepoRoot(nonEmpty(latest.RepoRoot, "."))
	okRev, curHead, curTree := isRevisionCurrent(targetRoot, ledger.path, latest.SourceCommit, latest.SourceTreeSHA256)
	if !okRev {
		return emitStoryProofRejection(graphID, taskID, routeID, "STALE", latest.RunID,
			"source commit or worktree changed since storyline verification receipt",
			&taskProofContextInfo{
				Graph:        graphID,
				Environment:  latest.RuntimeNS,
				Target:       s.FocalEntity,
				SourceCommit: curHead,
				SourceSHA256: curTree,
				ConfigSHA256: s.SHA256,
				GraphSHA256:  s.SHA256,
				SourceRoot:   targetRoot,
				ConfigPath:   s.SourcePath,
			})
	}
	// 3. 汇总路由收据与六维步骤轨迹哈希并输出有效证明
	rawDigests, seen := []string{latest.ReceiptSHA256}, map[string]bool{latest.ReceiptSHA256: true}
	if traces, tErr := ledger.ListStepTraces(graphID, latest.RunID, routeID); tErr == nil {
		for _, tr := range traces {
			if len(tr.TraceSHA256) == 64 && !seen[tr.TraceSHA256] {
				seen[tr.TraceSHA256] = true
				rawDigests = append(rawDigests, tr.TraceSHA256)
			}
		}
	}
	return emitRawJSON(json.MarshalIndent(taskProofResponse{
		Version:         1,
		Graph:           graphID,
		Task:            taskID,
		ObligationID:    routeID,
		Run:             latest.RunID,
		Round:           latest.LoadSave,
		Verdict:         string(VerdictPass),
		ProofScope:      "linked_task_evidence",
		InputsUnchanged: true,
		EvidenceStatus:  string(VerdictPass),
		Context: &taskProofContextInfo{
			Graph:        graphID,
			Environment:  latest.RuntimeNS,
			Target:       s.FocalEntity,
			SourceCommit: curHead,
			SourceSHA256: curTree,
			ConfigSHA256: s.SHA256,
			GraphSHA256:  s.SHA256,
			SourceRoot:   targetRoot,
			ConfigPath:   s.SourcePath,
		},
		RAWSHA256:     rawDigests,
		ReceiptSHA256: latest.ReceiptSHA256,
	}, "", "  "))
}

// emitStoryAllRoutesProof 校验故事线全量必需路由及完成门禁并返回联合证明
// 请求体
// ledger 验证账本实例指针
// s 故事线定义指针
// graphID 故事线或图ID
// taskID 任务完整ID
// latestByRoute 按路由ID索引的最新收据映射
// 响应体
// exitCode 进程退出状态码
func emitStoryAllRoutesProof(ledger *StoryLedger, s *Storyline, graphID, taskID string, latestByRoute map[string]RouteResultRecord) int {
	// 1. 先核验故事线四维100%完成度门禁是否整体通过
	comp, err := computeCompletion(ledger, s, nil)
	if err != nil || Verdict(comp.Verdict) != VerdictPass {
		return emitStoryProofRejection(graphID, taskID, "all", "", "", "storyline completion gate has not reached 100% PASS", nil)
	}
	var (
		rawDigests []string
		seen       = map[string]bool{}
		anchorRec  RouteResultRecord
		hasAnchor  bool
		targetRoot string
		curHead    string
		curTree    string
	)
	for _, r := range s.Routes {
		rec, ok := latestByRoute[r.RouteID]
		if !ok || Verdict(rec.Verdict) != VerdictPass || rec.SoftlockDetected {
			return emitStoryProofRejection(graphID, taskID, "all", "", "", "missing current passing receipt for route "+r.RouteID, nil)
		}
		recRoot := canonicalRepoRoot(nonEmpty(rec.RepoRoot, "."))
		okRev, h, tree := isRevisionCurrent(recRoot, ledger.path, rec.SourceCommit, rec.SourceTreeSHA256)
		if !okRev {
			return emitStoryProofRejection(graphID, taskID, "all", "STALE", rec.RunID,
				"source commit or worktree changed since route "+r.RouteID+" verification receipt",
				&taskProofContextInfo{
					Graph:        graphID,
					Environment:  rec.RuntimeNS,
					Target:       s.FocalEntity,
					SourceCommit: h,
					SourceSHA256: tree,
					ConfigSHA256: s.SHA256,
					GraphSHA256:  s.SHA256,
					SourceRoot:   recRoot,
					ConfigPath:   s.SourcePath,
				})
		}
		if !hasAnchor {
			anchorRec = rec
			targetRoot, curHead, curTree = recRoot, h, tree
			hasAnchor = true
		}
		if len(rec.ReceiptSHA256) == 64 && !seen[rec.ReceiptSHA256] {
			seen[rec.ReceiptSHA256] = true
			rawDigests = append(rawDigests, rec.ReceiptSHA256)
		}
		if traces, tErr := ledger.ListStepTraces(graphID, rec.RunID, r.RouteID); tErr == nil {
			for _, tr := range traces {
				if len(tr.TraceSHA256) == 64 && !seen[tr.TraceSHA256] {
					seen[tr.TraceSHA256] = true
					rawDigests = append(rawDigests, tr.TraceSHA256)
				}
			}
		}
	}
	if !hasAnchor || len(rawDigests) == 0 {
		return emitStoryProofRejection(graphID, taskID, "all", "", "", "no verified storyline route receipts available", nil)
	}
	// 2. 生成全路由汇总收据哈希与源码上下文
	receiptSHA := sha256Hex([]byte(anchorRec.RunID + ":" + s.SHA256 + ":all"))
	return emitRawJSON(json.MarshalIndent(taskProofResponse{
		Version:         1,
		Graph:           graphID,
		Task:            taskID,
		ObligationID:    "all",
		Run:             anchorRec.RunID,
		Round:           nonEmpty(anchorRec.LoadSave, "all"),
		Verdict:         string(VerdictPass),
		ProofScope:      "linked_task_evidence",
		InputsUnchanged: true,
		EvidenceStatus:  string(VerdictPass),
		Context: &taskProofContextInfo{
			Graph:        graphID,
			Environment:  nonEmpty(anchorRec.RuntimeNS, "all"),
			Target:       s.FocalEntity,
			SourceCommit: curHead,
			SourceSHA256: curTree,
			ConfigSHA256: s.SHA256,
			GraphSHA256:  s.SHA256,
			SourceRoot:   targetRoot,
			ConfigPath:   s.SourcePath,
		},
		RAWSHA256:     rawDigests,
		ReceiptSHA256: receiptSHA,
	}, "", "  "))
}

// emitStoryProofRejection 输出结构化的 NOT_RUN/STALE 证明拒绝响应并返回退出码 1
// 请求体
// graphID 图或故事线标识
// taskID 完整任务标识
// obligationID 路由或技术义务标识
// evidenceStatus 证据核验状态标识（如STALE）
// runID 关联历史运行批次ID
// reason 拒绝授权的明细原因
// ctx 可选源码指纹上下文
// 响应体
// exitCode 固定返回退出码1
func emitStoryProofRejection(graphID, taskID, obligationID, evidenceStatus, runID, reason string, ctx *taskProofContextInfo) int {
	// 1. 构造非PASS的 taskProofResponse 并序列化到标准输出
	_ = emitRawJSON(json.MarshalIndent(taskProofResponse{
		Version:         1,
		Graph:           graphID,
		Task:            taskID,
		ObligationID:    obligationID,
		Run:             runID,
		Verdict:         string(VerdictNotRun),
		Reason:          reason,
		InputsUnchanged: false,
		EvidenceStatus:  evidenceStatus,
		Context:         ctx,
	}, "", "  "))
	return 1
}

// tryEmitCodeTaskProofWithObligation 校验代码测试计划指定义务或全计划门禁并导出当前修订证明
// 请求体
// ledger 验证账本实例指针
// graphID 代码测试计划ID
// taskID 任务完整标识
// obligationOverride 显式传入的 --obligation 义务标识（为空时回退taskID后缀）
// 响应体
// exitCode 进程退出状态码
// handled 是否已按代码测试计划接管处理
func tryEmitCodeTaskProofWithObligation(ledger *StoryLedger, graphID, taskID, obligationOverride string) (exitCode int, handled bool) {
	// 1. 检查账本内是否存在目标代码验证计划
	plan, err := ledger.LoadCodePlan(graphID)
	if err != nil || plan == nil {
		return 0, false
	}
	obligationID := strings.TrimSpace(obligationOverride)
	if obligationID == "" {
		obligationID = strings.TrimPrefix(taskID, graphID+"/")
	}
	targetRoot := canonicalRepoRoot(nonEmpty(plan.RepoRoot, "."))
	curHead := currentGitHead(targetRoot)
	curTree := sourceWorktreeDigestExcluding(targetRoot, ledger.path)

	// 2. 按计划版本归集各测试技术最新收据并校验修订一致性与严禁跨提交拼装
	results, rErr := ledger.ListCodeResults(plan.PlanID, "", "")
	if rErr != nil {
		return emitStoryProofRejection(graphID, taskID, obligationID, "", "", "failed to load code verification results: "+rErr.Error(), nil), true
	}
	latestByTech := map[string]CodeResultRecord{}
	for _, rec := range results {
		if rec.PlanSHA256 == plan.PlanSHA256 {
			latestByTech[rec.TechniqueID] = rec
		}
	}
	var (
		runID         string
		receiptSHA256 string
		rawDigests    []string
		seenDigest    = map[string]bool{}
		staleReason   string
		staleRunID    string
	)
	addDigest := func(d string) {
		if len(d) == 64 && !seenDigest[d] {
			seenDigest[d] = true
			rawDigests = append(rawDigests, d)
		}
	}
	if obligationID == "all" {
		gateRep, gErr := computeCodeGate(ledger, plan.PlanID, "", plan.MaxTier, false)
		if gErr == nil && Verdict(gateRep.Verdict) == VerdictPass {
			runID = gateRep.RunID
			for _, t := range plan.Techniques {
				rec, ok := latestByTech[t.TechniqueID]
				if t.Status == SelectRequired && (!ok || Verdict(rec.Verdict) != VerdictPass) {
					runID = ""
					break
				}
				if !ok || Verdict(rec.Verdict) != VerdictPass {
					continue
				}
				recRoot := canonicalRepoRoot(nonEmpty(rec.RepoRoot, targetRoot))
				okRev, _, _ := isRevisionCurrent(recRoot, ledger.path, rec.SourceCommit, rec.SourceTreeSHA256)
				if !okRev {
					staleReason = "source commit or worktree changed since " + rec.TechniqueID + " verification receipt"
					staleRunID = rec.RunID
					break
				}
				addDigest(rec.ReceiptSHA256)
				addDigest(rec.ArtifactSHA256)
			}
			if staleReason == "" && runID != "" && len(rawDigests) > 0 {
				receiptSHA256 = sha256Hex([]byte(runID + ":" + plan.PlanSHA256))
			}
		}
	} else if rec, ok := latestByTech[obligationID]; ok && Verdict(rec.Verdict) == VerdictPass {
		recRoot := canonicalRepoRoot(nonEmpty(rec.RepoRoot, targetRoot))
		okRev, _, _ := isRevisionCurrent(recRoot, ledger.path, rec.SourceCommit, rec.SourceTreeSHA256)
		if !okRev {
			staleReason = "source commit or worktree changed since code verification receipt"
			staleRunID = rec.RunID
		} else {
			runID = rec.RunID
			receiptSHA256 = rec.ReceiptSHA256
			addDigest(rec.ReceiptSHA256)
			addDigest(rec.ArtifactSHA256)
		}
	}

	// 3. 若收据已因源码修改或跨提交局部重跑而过期，则显式返回 STALE 与 NOT_RUN
	ctxInfo := &taskProofContextInfo{
		Graph:        graphID,
		Environment:  nonEmpty(plan.PrimaryLanguage, "code"),
		Target:       nonEmpty(plan.RepoRoot, plan.PrimaryLanguage),
		SourceCommit: curHead,
		SourceSHA256: curTree,
		ConfigSHA256: plan.PlanSHA256,
		GraphSHA256:  plan.PlanSHA256,
		SourceRoot:   targetRoot,
		ConfigPath:   nonEmpty(plan.SourcePath, plan.RepoRoot),
	}
	if staleReason != "" {
		return emitStoryProofRejection(graphID, taskID, obligationID, "STALE", staleRunID, staleReason, ctxInfo), true
	}
	if runID == "" || len(receiptSHA256) != 64 || len(rawDigests) == 0 {
		return emitStoryProofRejection(graphID, taskID, obligationID, "", "", "no current passing code verification receipt", nil), true
	}
	return emitRawJSON(json.MarshalIndent(taskProofResponse{
		Version:         1,
		Graph:           graphID,
		Task:            taskID,
		ObligationID:    obligationID,
		Run:             runID,
		Round:           nonEmpty(string(plan.MaxTier), "P1"),
		Verdict:         string(VerdictPass),
		ProofScope:      "linked_task_evidence",
		InputsUnchanged: true,
		EvidenceStatus:  string(VerdictPass),
		Context:         ctxInfo,
		RAWSHA256:       rawDigests,
		ReceiptSHA256:   receiptSHA256,
	}, "", "  ")), true
}
