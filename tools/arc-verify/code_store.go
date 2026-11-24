package main

import (
	"database/sql"
	"encoding/json"
	"errors"
)

// SaveCodePlan 幂等持久化或更新代码级测试计划定义至账本
// 请求体
// plan 代码测试计划结构体指针
// 响应体
// err JSON序列化或SQL写入错误
func (l *StoryLedger) SaveCodePlan(plan *CodeVerifyPlan) error {
	// 1. 加锁序列化完整计划为JSON文本
	l.mu.Lock()
	defer l.mu.Unlock()
	raw, err := json.Marshal(plan)
	if err != nil {
		return err
	}
	// 2. 写入或更新 code_verify_plans 主表记录并同步兼容 graphs 表
	now := nowUTC()
	_, err = l.db.Exec(
		`INSERT INTO code_verify_plans(plan_id, repo_root, primary_language, max_tier, plan_sha256, raw_json, updated_at)
		 VALUES(?,?,?,?,?,?,?)
		 ON CONFLICT(plan_id) DO UPDATE SET
		   repo_root=excluded.repo_root,
		   primary_language=excluded.primary_language,
		   max_tier=excluded.max_tier,
		   plan_sha256=excluded.plan_sha256,
		   raw_json=excluded.raw_json,
		   updated_at=excluded.updated_at`,
		plan.PlanID, plan.RepoRoot, plan.PrimaryLanguage, string(plan.MaxTier), plan.PlanSHA256, string(raw), now,
	)
	if err != nil {
		return err
	}
	_, _ = l.db.Exec(
		`INSERT INTO graphs(id, raw_json, created_at) VALUES(?,?,?)
		 ON CONFLICT(id) DO UPDATE SET raw_json=excluded.raw_json`,
		plan.PlanID, string(raw), now,
	)
	return nil
}

// LoadCodePlan 按计划标识从账本加载代码级测试计划定义
// 请求体
// planID 代码测试计划ID
// 响应体
// plan 反序列化后的代码测试计划指针
// err 未找到记录或JSON解析失败错误
func (l *StoryLedger) LoadCodePlan(planID string) (*CodeVerifyPlan, error) {
	// 1. 加锁查询计划JSON与指纹摘要
	l.mu.Lock()
	defer l.mu.Unlock()
	var raw, sha string
	err := l.db.QueryRow(
		`SELECT raw_json, plan_sha256 FROM code_verify_plans WHERE plan_id=?`,
		planID,
	).Scan(&raw, &sha)
	if err != nil {
		return nil, err
	}
	// 2. 反序列化强类型结构体并恢复摘要字段
	var plan CodeVerifyPlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return nil, err
	}
	plan.PlanSHA256 = sha
	return &plan, nil
}

// LatestCodePlan 按更新时间倒序加载最近保存的代码级测试计划定义
// 请求体
// 无
// 响应体
// plan 最近保存的代码测试计划指针
// err 无已存计划或数据库查询错误
func (l *StoryLedger) LatestCodePlan() (*CodeVerifyPlan, error) {
	// 1. 加锁按更新时间与rowid倒序查询最新计划ID并反序列化
	l.mu.Lock()
	defer l.mu.Unlock()
	var raw, sha string
	err := l.db.QueryRow(
		`SELECT raw_json, plan_sha256 FROM code_verify_plans ORDER BY updated_at DESC, rowid DESC LIMIT 1`,
	).Scan(&raw, &sha)
	if err != nil {
		return nil, err
	}
	var plan CodeVerifyPlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return nil, err
	}
	plan.PlanSHA256 = sha
	return &plan, nil
}

// RecordCodeRun 记录单次代码级测试运行的批次元数据与统计计数
// 请求体
// rec 运行批次记录结构体
// 响应体
// err SQL写入错误
func (l *StoryLedger) RecordCodeRun(rec CodeRunRecord) error {
	// 1. 加锁补全结束时间并插入 code_verify_runs 记录
	l.mu.Lock()
	defer l.mu.Unlock()
	if rec.FinishedAt == "" {
		rec.FinishedAt = nowUTC()
	}
	_, err := l.db.Exec(
		`INSERT INTO code_verify_runs(
			run_id, plan_id, plan_sha256, repo_root, source_commit, source_tree_sha256, max_tier, verdict,
			required_total, passed_count, failed_count, blocked_count, started_at, finished_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.RunID, rec.PlanID, rec.PlanSHA256, rec.RepoRoot, rec.SourceCommit, rec.SourceTreeSHA256, rec.MaxTier, rec.Verdict,
		rec.RequiredTotal, rec.PassedCount, rec.FailedCount, rec.BlockedCount, rec.StartedAt, rec.FinishedAt,
	)
	return err
}

// GetCodeRun 按运行批次ID精确查询代码测试运行记录
// 请求体
// runID 运行批次ID
// 响应体
// rec 匹配的代码测试运行批次记录指针（无记录时返回nil）
// err SQL查询错误
func (l *StoryLedger) GetCodeRun(runID string) (*CodeRunRecord, error) {
	// 1. 加锁按主键查询运行记录并使用 errors.Is 识别空结果
	l.mu.Lock()
	defer l.mu.Unlock()
	var rec CodeRunRecord
	err := l.db.QueryRow(
		`SELECT run_id, plan_id, plan_sha256, repo_root, source_commit, source_tree_sha256, max_tier, verdict,
		        required_total, passed_count, failed_count, blocked_count, started_at, finished_at
		 FROM code_verify_runs WHERE run_id=?`,
		runID,
	).Scan(
		&rec.RunID, &rec.PlanID, &rec.PlanSHA256, &rec.RepoRoot, &rec.SourceCommit, &rec.SourceTreeSHA256, &rec.MaxTier, &rec.Verdict,
		&rec.RequiredTotal, &rec.PassedCount, &rec.FailedCount, &rec.BlockedCount, &rec.StartedAt, &rec.FinishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// LatestCodeRun 查询指定代码测试计划最近一次运行批次记录
// 请求体
// planID 代码测试计划ID
// 响应体
// rec 最新运行批次记录指针（无记录时返回nil）
// err SQL查询错误
func (l *StoryLedger) LatestCodeRun(planID string) (*CodeRunRecord, error) {
	// 1. 加锁按结束时间与rowid倒序查询最新运行记录
	l.mu.Lock()
	defer l.mu.Unlock()
	var rec CodeRunRecord
	err := l.db.QueryRow(
		`SELECT run_id, plan_id, plan_sha256, repo_root, source_commit, source_tree_sha256, max_tier, verdict,
		        required_total, passed_count, failed_count, blocked_count, started_at, finished_at
		 FROM code_verify_runs WHERE plan_id=? ORDER BY finished_at DESC, rowid DESC LIMIT 1`,
		planID,
	).Scan(
		&rec.RunID, &rec.PlanID, &rec.PlanSHA256, &rec.RepoRoot, &rec.SourceCommit, &rec.SourceTreeSHA256, &rec.MaxTier, &rec.Verdict,
		&rec.RequiredTotal, &rec.PassedCount, &rec.FailedCount, &rec.BlockedCount, &rec.StartedAt, &rec.FinishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// AppendCodeResult 向验证账本追加不可变的单项代码测试收据（绝不覆盖历史FAIL）
// 请求体
// rec 单项测试执行结果收据
// 响应体
// err JSON编码或SQL写入错误
func (l *StoryLedger) AppendCodeResult(rec CodeResultRecord) error {
	// 1. 加锁编码指标载荷与独立覆盖率明细为JSON文本
	l.mu.Lock()
	defer l.mu.Unlock()
	if rec.CreatedAt == "" {
		rec.CreatedAt = nowUTC()
	}
	metricsBytes, err := json.Marshal(rec.Metrics)
	if err != nil {
		metricsBytes = []byte("{}")
	}
	covBytes := []byte("{}")
	if rec.Metrics.Coverage != nil {
		if cb, cErr := json.Marshal(rec.Metrics.Coverage); cErr == nil {
			covBytes = cb
		}
	}
	// 2. 严格追加写入 code_verify_results 主表
	_, err = l.db.Exec(
		`INSERT INTO code_verify_results(
			run_id, plan_id, plan_sha256, technique_id, priority, language, tool_name, executed_cmd,
			verdict, exit_code, duration_ms, metrics_json, coverage_json, failure_reason, missing_prerequisite,
			artifact_path, artifact_sha256, raw_output, repo_root, source_commit, source_tree_sha256, receipt_sha256, created_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.RunID, rec.PlanID, rec.PlanSHA256, rec.TechniqueID, rec.Priority, rec.Language, rec.ToolName, rec.ExecutedCmd,
		rec.Verdict, rec.ExitCode, rec.DurationMS, string(metricsBytes), string(covBytes), rec.FailureReason, rec.MissingPrerequisite,
		rec.ArtifactPath, rec.ArtifactSHA256, rec.RawOutput, rec.RepoRoot, rec.SourceCommit, rec.SourceTreeSHA256, rec.ReceiptSHA256, rec.CreatedAt,
	)
	return err
}

// ListCodeResults 按可选运行批次与技术标识过滤条件读取代码级测试收据列表
// 请求体
// planID 代码测试计划ID
// runID 可选运行批次过滤ID
// techniqueID 可选测试技术过滤ID
// 响应体
// out 强类型代码测试收据切片
// err SQL查询或扫描错误
func (l *StoryLedger) ListCodeResults(planID, runID, techniqueID string) ([]CodeResultRecord, error) {
	// 1. 加锁按显式强类型条件分支选择固定SQL查询（严禁动态拼接参数切片）
	l.mu.Lock()
	defer l.mu.Unlock()
	base := `SELECT id, run_id, plan_id, plan_sha256, technique_id, priority, language, tool_name, executed_cmd,
		verdict, exit_code, duration_ms, metrics_json, coverage_json, failure_reason, missing_prerequisite,
		artifact_path, artifact_sha256, raw_output, repo_root, source_commit, source_tree_sha256, receipt_sha256, created_at
	 FROM code_verify_results WHERE plan_id=?`
	var (
		rows *sql.Rows
		err  error
	)
	switch {
	case runID != "" && techniqueID != "":
		rows, err = l.db.Query(base+` AND run_id=? AND technique_id=? ORDER BY id ASC`, planID, runID, techniqueID)
	case runID != "":
		rows, err = l.db.Query(base+` AND run_id=? ORDER BY id ASC`, planID, runID)
	case techniqueID != "":
		rows, err = l.db.Query(base+` AND technique_id=? ORDER BY id ASC`, planID, techniqueID)
	default:
		rows, err = l.db.Query(base+` ORDER BY id ASC`, planID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// 2. 逐行扫描并反序列化强类型指标与覆盖率载荷
	var out []CodeResultRecord
	for rows.Next() {
		var r CodeResultRecord
		var metricsJSON, covJSON string
		if scanErr := rows.Scan(
			&r.ID, &r.RunID, &r.PlanID, &r.PlanSHA256, &r.TechniqueID, &r.Priority, &r.Language, &r.ToolName, &r.ExecutedCmd,
			&r.Verdict, &r.ExitCode, &r.DurationMS, &metricsJSON, &covJSON, &r.FailureReason, &r.MissingPrerequisite,
			&r.ArtifactPath, &r.ArtifactSHA256, &r.RawOutput, &r.RepoRoot, &r.SourceCommit, &r.SourceTreeSHA256, &r.ReceiptSHA256, &r.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		if metricsJSON != "" && metricsJSON != "{}" {
			_ = json.Unmarshal([]byte(metricsJSON), &r.Metrics)
		}
		if r.Metrics.Coverage == nil && covJSON != "" && covJSON != "{}" && covJSON != "null" {
			var cov CoverageBreakdown
			if json.Unmarshal([]byte(covJSON), &cov) == nil {
				r.Metrics.Coverage = &cov
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountHistoricalCodeFails 统计指定计划在不可变账本中累计保留的历史FAIL次数
// 请求体
// planID 代码测试计划ID
// 响应体
// count 历史FAIL收据总条数
// err SQL统计查询错误
func (l *StoryLedger) CountHistoricalCodeFails(planID string) (int, error) {
	// 1. 加锁聚合查询指定计划下全部历史FAIL记录条数
	l.mu.Lock()
	defer l.mu.Unlock()
	var count int
	err := l.db.QueryRow(
		`SELECT count(*) FROM code_verify_results WHERE plan_id=? AND verdict=?`,
		planID, string(VerdictFail),
	).Scan(&count)
	return count, err
}
