package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// ensureArcTaskObligations 幂等确保任务验证义务关联表与索引已创建
// 请求体
// ctx 请求上下文
// db 数据库连接句柄
// 响应体
// err 建表或建索引失败错误
func ensureArcTaskObligations(ctx context.Context, db *sql.DB) error {
	// 1. 依次执行关联表与联合索引DDL语句
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS arc_task_obligations (arc_task_id TEXT NOT NULL, graph TEXT NOT NULL, obligation_id TEXT NOT NULL, verify_task_id TEXT NOT NULL DEFAULT '', updated TEXT NOT NULL, PRIMARY KEY(arc_task_id,graph,obligation_id))`,
		`CREATE INDEX IF NOT EXISTS arc_task_obligations_verify ON arc_task_obligations(graph,obligation_id,verify_task_id)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// linkTaskVerify 将本地任务幂等绑定到业务验证图或Storyline路由义务
// 请求体
// db 数据库连接句柄
// taskID 本地任务ID
// graph 验证图或故事线ID
// obligation 验证义务或路由ID
// verifyTask 可选验证任务ID覆盖值
// 响应体
// err 任务不存在或绑定落库失败错误
func linkTaskVerify(db *sql.DB, taskID, graph, obligation, verifyTask string) error {
	// 1. 校验必填参数并确保义务关联表存在
	if taskID == "" || graph == "" || obligation == "" {
		return errors.New("task link-verify requires --id, --graph, and --obligation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := ensureArcTaskObligations(ctx, db); err != nil {
		return err
	}
	// 2. 读取本地任务状态并探测旧版 tasks 表或新版 story_* 账本中的当前状态标签
	var currentStatus string
	if err := db.QueryRowContext(ctx, "SELECT status FROM arc_tasks WHERE id=?", taskID).Scan(&currentStatus); err != nil {
		return err
	}
	if verifyTask == "" {
		verifyTask = graph + "/" + obligation
	}
	hasVerifyTasks, err := arcTableExists(ctx, db, "tasks")
	if err != nil {
		return err
	}
	verifyTaskStatus := string(VerdictUnplanned)
	if hasVerifyTasks {
		var st string
		qErr := db.QueryRowContext(ctx, "SELECT status FROM tasks WHERE id=?", verifyTask).Scan(&st)
		if qErr == nil {
			verifyTaskStatus = st
		} else if !errors.Is(qErr, sql.ErrNoRows) {
			return qErr
		}
	}
	if verifyTaskStatus == string(VerdictUnplanned) {
		stStatus, _, sErr := lookupStorylineObligation(ctx, db, graph, obligation)
		if sErr != nil {
			return sErr
		}
		if stStatus != "" {
			verifyTaskStatus = stStatus
		}
	}
	if verifyTaskStatus == string(VerdictUnplanned) {
		codeStatus, _, cErr := lookupCodeVerifyObligation(ctx, db, graph, obligation)
		if cErr != nil {
			return cErr
		}
		if codeStatus != "" {
			verifyTaskStatus = codeStatus
		}
	}
	// 3. 在事务内幂等写入关联记录并追加强类型领域事件
	err = dbTransaction(ctx, db, func(tx *sql.Tx) error {
		if _, execErr := tx.ExecContext(ctx,
			"INSERT OR REPLACE INTO arc_task_obligations(arc_task_id,graph,obligation_id,verify_task_id,updated) VALUES(?,?,?,?,?)",
			taskID, graph, obligation, verifyTask, nowUTC()); execErr != nil {
			return execErr
		}
		rawEv, mErr := json.Marshal(taskObligationLinkedEventPayload{
			Graph:      graph,
			Obligation: obligation,
			VerifyTask: verifyTask,
		})
		return addEventPayload(ctx, tx, "task", taskID, "task.obligation_linked", rawEv, mErr)
	})
	if err != nil {
		return err
	}
	// 4. 输出具名关联绑定响应
	return emitRawJSON(json.Marshal(taskLinkVerifyResponse{
		ArcTaskID:        taskID,
		ArcTaskStatus:    currentStatus,
		Graph:            graph,
		ObligationID:     obligation,
		VerifyTaskID:     verifyTask,
		VerifyTaskStatus: verifyTaskStatus,
	}))
}

// showTaskVerifyStatus 汇总展示任务关联验证义务的存储层未重验摘要
// 请求体
// db 数据库连接句柄
// taskID 本地任务ID
// 响应体
// err 任务不存在或联合查询失败错误
func showTaskVerifyStatus(db *sql.DB, taskID string) error {
	// 1. 校验任务ID并读取任务基础信息与全部已绑定义务列表
	if taskID == "" {
		return errors.New("task verify-status requires --id")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := ensureArcTaskObligations(ctx, db); err != nil {
		return err
	}
	var title, taskStatus, updated string
	var version int
	if err := db.QueryRowContext(ctx, "SELECT title,status,version,updated FROM arc_tasks WHERE id=?", taskID).Scan(&title, &taskStatus, &version, &updated); err != nil {
		return err
	}
	type linkedRow struct{ graph, obligation, verifyTask, updated string }
	rows, err := db.QueryContext(ctx, "SELECT graph,obligation_id,verify_task_id,updated FROM arc_task_obligations WHERE arc_task_id=? ORDER BY graph,obligation_id", taskID)
	if err != nil {
		return err
	}
	links := []linkedRow{}
	for rows.Next() {
		var r linkedRow
		if scanErr := rows.Scan(&r.graph, &r.obligation, &r.verifyTask, &r.updated); scanErr != nil {
			_ = rows.Close()
			return scanErr
		}
		links = append(links, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	_ = rows.Close()
	// 2. 检测旧版验证图三张表是否存在以按需联合读取
	hasTasks, err := arcTableExists(ctx, db, "tasks")
	if err != nil {
		return err
	}
	hasRuns, err := arcTableExists(ctx, db, "runs")
	if err != nil {
		return err
	}
	hasStepFacts, err := arcTableExists(ctx, db, "step_facts")
	if err != nil {
		return err
	}
	// 3. 逐条组装强类型验证义务摘要
	obligations := make([]obligationStatusItem, 0, len(links))
	counts := map[string]int{
		string(VerdictPass):      0,
		string(VerdictFail):      0,
		string(VerdictBlocked):   0,
		string(VerdictNotRun):    0,
		string(VerdictUnplanned): 0,
	}
	for _, link := range links {
		vTask := link.verifyTask
		if vTask == "" {
			vTask = link.graph + "/" + link.obligation
		}
		item := obligationStatusItem{
			Graph:          link.graph,
			ObligationID:   link.obligation,
			VerifyTaskID:   vTask,
			LinkedAt:       link.updated,
			EvidenceStatus: "NOT_REVALIDATED",
		}
		status := string(VerdictUnplanned)
		if hasTasks {
			var tStatus, tKind, tExpected string
			qErr := db.QueryRowContext(ctx, "SELECT status,kind,expected FROM tasks WHERE id=?", vTask).Scan(&tStatus, &tKind, &tExpected)
			if qErr == nil {
				status = tStatus
				item.Kind = tKind
				item.Expected = tExpected
			} else if !errors.Is(qErr, sql.ErrNoRows) {
				return qErr
			}
		}
		if hasRuns && status != string(VerdictUnplanned) {
			var runID, runStatus, env, commitSHA string
			qErr := db.QueryRowContext(ctx, "SELECT id,status,environment,commit_sha FROM runs WHERE task=? ORDER BY rowid DESC LIMIT 1", vTask).Scan(&runID, &runStatus, &env, &commitSHA)
			if qErr == nil {
				item.LatestRun = runID
				item.LatestRunStoredStatus = runStatus
				item.Environment = env
				item.CommitSHA = commitSHA
				if hasStepFacts {
					var node, obsState, verdict, reason, sha string
					var seq, firstDiv int
					fErr := db.QueryRowContext(ctx, "SELECT seq,node,observed_state,verdict,reason,first_divergence,evidence_sha256 FROM step_facts WHERE run=? ORDER BY first_divergence DESC, seq DESC LIMIT 1", runID).Scan(&seq, &node, &obsState, &verdict, &reason, &firstDiv, &sha)
					if fErr == nil {
						item.Fact = &verifyFactSummary{
							Seq:             seq,
							Node:            node,
							ObservedState:   obsState,
							StoredVerdict:   verdict,
							Reason:          reason,
							EvidenceStatus:  "NOT_REVALIDATED",
							FirstDivergence: firstDiv == 1,
							EvidenceSHA256:  sha,
						}
					} else if !errors.Is(fErr, sql.ErrNoRows) {
						return fErr
					}
				}
			} else if !errors.Is(qErr, sql.ErrNoRows) {
				return qErr
			}
		}
		if status == string(VerdictUnplanned) {
			stStatus, stSummary, sErr := lookupStorylineObligation(ctx, db, link.graph, link.obligation)
			if sErr != nil {
				return sErr
			}
			if stStatus != "" && stSummary != nil {
				status = stStatus
				item.Kind = stSummary.Kind
				item.Expected = stSummary.Expected
				item.LatestRun = stSummary.LatestRun
				item.LatestRunStoredStatus = stSummary.LatestRunStoredStatus
				item.Environment = stSummary.Environment
				item.Fact = stSummary.Fact
			}
		}
		if status == string(VerdictUnplanned) {
			cStatus, cSummary, cErr := lookupCodeVerifyObligation(ctx, db, link.graph, link.obligation)
			if cErr != nil {
				return cErr
			}
			if cStatus != "" && cSummary != nil {
				status = cStatus
				item.Kind = cSummary.Kind
				item.Expected = cSummary.Expected
				item.LatestRun = cSummary.LatestRun
				item.LatestRunStoredStatus = cSummary.LatestRunStoredStatus
				item.Environment = cSummary.Environment
				item.Fact = cSummary.Fact
			}
		}
		item.StoredStatus = status
		switch Verdict(status) {
		case VerdictPass, VerdictFail, VerdictBlocked, VerdictUnplanned:
			counts[status]++
		default:
			counts[string(VerdictNotRun)]++
		}
		obligations = append(obligations, item)
	}
	// 4. 输出具名未重验状态汇总响应
	return emitRawJSON(json.Marshal(taskVerifyStatusResponse{
		ArcTask:            taskShowResponse{ID: taskID, Title: title, Status: taskStatus, Version: version, Updated: updated},
		Linked:             len(links),
		StoredStatusCounts: counts,
		EvidenceStatus:     "NOT_REVALIDATED",
		ProofScope:         "none",
		Provenance: verifyProvenanceInfo{
			Kind:                "stored_ledger_summary",
			LatestRunOrder:      "rowid_desc",
			EvidenceRevalidated: false,
			Reason:              "Stored task, run and step labels only; RAW, frozen expectations and current inputs have not been revalidated. This summary cannot authorize task completion.",
		},
		Obligations: obligations,
	}))
}

// lookupStorylineObligation 查询Storyline账本中的强类型路由验证状态摘要
// 请求体
// ctx 请求上下文
// db 数据库连接
// storyID 故事线唯一ID
// routeID 路由分支ID
// 响应体
// status 存储状态标签
// summary 强类型路由运行与步骤摘要指针
// err 数据库查询错误
func lookupStorylineObligation(ctx context.Context, db *sql.DB, storyID, routeID string) (status string, summary *storylineObligationSummary, err error) {
	// 1. 检查 story_definitions 表是否存在
	hasDefs, err := arcTableExists(ctx, db, "story_definitions")
	if err != nil || !hasDefs {
		return "", nil, err
	}
	// 2. 读取当前故事线定义JSON与版本哈希
	var rawJSON, storySHA string
	err = db.QueryRowContext(ctx, "SELECT raw_json, story_sha256 FROM story_definitions WHERE story_id=?", storyID).Scan(&rawJSON, &storySHA)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	// 3. 解析故事线路由声明确认目标 routeID（或 all 全量路由）是否已定义
	var parsed struct {
		Routes []struct {
			RouteID       string `json:"route_id"`
			UnlocksEnding string `json:"unlocks_ending"`
		} `json:"routes"`
	}
	if json.Unmarshal([]byte(rawJSON), &parsed) != nil {
		return "", nil, nil
	}
	if routeID == "all" {
		if len(parsed.Routes) == 0 {
			return "", nil, nil
		}
		summary = &storylineObligationSummary{Kind: "STORYLINE_ALL", Expected: "all routes"}
		status = string(VerdictNotRun)
		hasRouteRes, rErr := arcTableExists(ctx, db, "story_route_results")
		if rErr != nil || !hasRouteRes {
			return status, summary, rErr
		}
		allPass, anyFail, anyBlocked := true, false, false
		var latestRun, latestNS string
		for _, r := range parsed.Routes {
			var rID, rVerdict, rNS string
			qErr := db.QueryRowContext(ctx,
				"SELECT run_id, verdict, runtime_ns FROM story_route_results WHERE story_id=? AND route_id=? AND story_sha256=? ORDER BY id DESC LIMIT 1",
				storyID, r.RouteID, storySHA,
			).Scan(&rID, &rVerdict, &rNS)
			if errors.Is(qErr, sql.ErrNoRows) {
				allPass = false
				continue
			}
			if qErr != nil {
				return "", nil, qErr
			}
			latestRun, latestNS = rID, rNS
			switch Verdict(rVerdict) {
			case VerdictPass:
			case VerdictFail:
				anyFail, allPass = true, false
			default:
				anyBlocked, allPass = true, false
			}
		}
		switch {
		case anyFail:
			status = string(VerdictFail)
		case anyBlocked:
			status = string(VerdictBlocked)
		case allPass:
			status = string(VerdictPass)
		default:
			status = string(VerdictNotRun)
		}
		summary.LatestRun = latestRun
		summary.LatestRunStoredStatus = status
		summary.Environment = latestNS
		return status, summary, nil
	}
	foundRoute := false
	expectedEnding := ""
	for _, r := range parsed.Routes {
		if r.RouteID == routeID {
			foundRoute = true
			expectedEnding = r.UnlocksEnding
			break
		}
	}
	if !foundRoute {
		return "", nil, nil
	}
	summary = &storylineObligationSummary{Kind: "STORYLINE_ROUTE", Expected: expectedEnding}
	status = string(VerdictNotRun)
	// 4. 查询 story_route_results 最新存储执行结果
	hasRouteRes, err := arcTableExists(ctx, db, "story_route_results")
	if err != nil || !hasRouteRes {
		return status, summary, err
	}
	var runID, verdict, runtimeNS string
	err = db.QueryRowContext(ctx,
		"SELECT run_id, verdict, runtime_ns FROM story_route_results WHERE story_id=? AND route_id=? AND story_sha256=? ORDER BY id DESC LIMIT 1",
		storyID, routeID, storySHA,
	).Scan(&runID, &verdict, &runtimeNS)
	if errors.Is(err, sql.ErrNoRows) {
		return status, summary, nil
	}
	if err != nil {
		return "", nil, err
	}
	status = verdict
	summary.LatestRun = runID
	summary.LatestRunStoredStatus = verdict
	summary.Environment = runtimeNS
	// 5. 若存在 story_step_traces 则附带强类型步骤事实摘要
	hasTraces, err := arcTableExists(ctx, db, "story_step_traces")
	if err == nil && hasTraces {
		var stepNum int
		var choice, actStatus, stepVerdict, diffReason, traceSHA string
		tErr := db.QueryRowContext(ctx,
			`SELECT step_num, choice, actual_status, verdict, diff_reason, trace_sha256
			 FROM story_step_traces WHERE story_id=? AND run_id=? AND route_id=?
			 ORDER BY CASE WHEN verdict != 'PASS' THEN 0 ELSE 1 END, id DESC LIMIT 1`,
			storyID, runID, routeID,
		).Scan(&stepNum, &choice, &actStatus, &stepVerdict, &diffReason, &traceSHA)
		if tErr == nil {
			summary.Fact = &verifyFactSummary{
				Seq:             stepNum,
				Node:            choice,
				ObservedState:   actStatus,
				StoredVerdict:   stepVerdict,
				Reason:          diffReason,
				EvidenceStatus:  "NOT_REVALIDATED",
				FirstDivergence: stepVerdict != string(VerdictPass),
				EvidenceSHA256:  traceSHA,
			}
		}
	}
	// 6. 返回未重验的强类型存储层状态摘要
	return status, summary, nil
}

// lookupCodeVerifyObligation 查询代码级测试账本中的计划门禁或单项技术验证状态摘要
// 请求体
// ctx 请求上下文
// db 数据库连接句柄
// planID 代码测试计划ID
// obligationID 测试技术ID或all
// 响应体
// status 存储判定状态字符串
// summary 强类型义务摘要指针
// err 数据库查询错误
func lookupCodeVerifyObligation(ctx context.Context, db *sql.DB, planID, obligationID string) (status string, summary *storylineObligationSummary, err error) {
	// 1. 检查 code_verify_plans 表是否存在并读取计划版本信息
	hasPlans, err := arcTableExists(ctx, db, "code_verify_plans")
	if err != nil || !hasPlans {
		return "", nil, err
	}
	var rawJSON, planSHA, primaryLang, maxTier string
	err = db.QueryRowContext(ctx, "SELECT raw_json, plan_sha256, primary_language, max_tier FROM code_verify_plans WHERE plan_id=?", planID).Scan(&rawJSON, &planSHA, &primaryLang, &maxTier)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	// 2. 解析计划技术列表核验目标义务是否合法
	var parsed struct {
		Techniques []struct {
			TechniqueID       string `json:"technique_id"`
			ToolName          string `json:"tool_name"`
			Status            string `json:"status"`
			Priority          string `json:"priority"`
			EffectivePriority string `json:"effective_priority"`
		} `json:"techniques"`
	}
	if json.Unmarshal([]byte(rawJSON), &parsed) != nil {
		return "", nil, nil
	}
	if obligationID == "all" {
		summary = &storylineObligationSummary{Kind: "CODE_VERIFY_PLAN", Expected: maxTier, Environment: primaryLang}
		status = string(VerdictNotRun)
		hasRuns, rErr := arcTableExists(ctx, db, "code_verify_runs")
		if rErr != nil || !hasRuns {
			return status, summary, rErr
		}
		var runID, verdict string
		qErr := db.QueryRowContext(ctx, "SELECT run_id, verdict FROM code_verify_runs WHERE plan_id=? AND plan_sha256=? ORDER BY rowid DESC LIMIT 1", planID, planSHA).Scan(&runID, &verdict)
		if errors.Is(qErr, sql.ErrNoRows) {
			return status, summary, nil
		}
		if qErr != nil {
			return "", nil, qErr
		}
		summary.LatestRun = runID
		summary.LatestRunStoredStatus = verdict
		hasRes, resErr := arcTableExists(ctx, db, "code_verify_results")
		if resErr != nil || !hasRes {
			return verdict, summary, resErr
		}
		hasExplicitStatus := false
		for _, t := range parsed.Techniques {
			if t.Status != "" {
				hasExplicitStatus = true
				break
			}
		}
		allPass, anyFail, anyBlocked := true, false, false
		for _, t := range parsed.Techniques {
			if hasExplicitStatus && t.Status != "REQUIRED" {
				continue
			}
			var resVerdict string
			tErr := db.QueryRowContext(ctx,
				"SELECT verdict FROM code_verify_results WHERE plan_id=? AND technique_id=? AND plan_sha256=? ORDER BY id DESC LIMIT 1",
				planID, t.TechniqueID, planSHA,
			).Scan(&resVerdict)
			if errors.Is(tErr, sql.ErrNoRows) {
				allPass = false
				continue
			}
			if tErr != nil {
				return "", nil, tErr
			}
			switch Verdict(resVerdict) {
			case VerdictPass:
			case VerdictFail:
				anyFail, allPass = true, false
			default:
				anyBlocked, allPass = true, false
			}
		}
		switch {
		case anyFail:
			status = string(VerdictFail)
		case anyBlocked:
			status = string(VerdictBlocked)
		case !allPass:
			status = string(VerdictBlocked)
		default:
			status = string(VerdictPass)
		}
		summary.LatestRunStoredStatus = status
		return status, summary, nil
	}
	foundTech := false
	toolName := ""
	for _, t := range parsed.Techniques {
		if t.TechniqueID == obligationID {
			foundTech = true
			toolName = t.ToolName
			break
		}
	}
	if !foundTech {
		return "", nil, nil
	}
	summary = &storylineObligationSummary{Kind: "CODE_VERIFY_TECHNIQUE", Expected: toolName, Environment: primaryLang}
	status = string(VerdictNotRun)
	// 3. 查询 code_verify_results 中当前计划版本最新一条单项收据
	hasRes, rErr := arcTableExists(ctx, db, "code_verify_results")
	if rErr != nil || !hasRes {
		return status, summary, rErr
	}
	var runID, verdict, failReason, missingPrereq, receiptSHA string
	qErr := db.QueryRowContext(ctx,
		"SELECT run_id, verdict, failure_reason, missing_prerequisite, receipt_sha256 FROM code_verify_results WHERE plan_id=? AND technique_id=? AND plan_sha256=? ORDER BY id DESC LIMIT 1",
		planID, obligationID, planSHA,
	).Scan(&runID, &verdict, &failReason, &missingPrereq, &receiptSHA)
	if errors.Is(qErr, sql.ErrNoRows) {
		return status, summary, nil
	}
	if qErr != nil {
		return "", nil, qErr
	}
	status = verdict
	summary.LatestRun = runID
	summary.LatestRunStoredStatus = verdict
	reason := failReason
	if reason == "" {
		reason = missingPrereq
	}
	summary.Fact = &verifyFactSummary{
		Seq:             1,
		Node:            obligationID,
		ObservedState:   verdict,
		StoredVerdict:   verdict,
		Reason:          reason,
		EvidenceStatus:  "NOT_REVALIDATED",
		FirstDivergence: verdict != string(VerdictPass),
		EvidenceSHA256:  receiptSHA,
	}
	return status, summary, nil
}
