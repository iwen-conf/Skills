package main

import (
	"database/sql"
	"encoding/json"
)

// RouteResultRecord 描述追加写入且永不覆盖历史FAIL的路由级验证收据记录
type RouteResultRecord struct {
	// ID 自增主键
	ID int64 `json:"id"`
	// RunID 所属运行批次ID
	RunID string `json:"run_id"`
	// StoryID 故事线唯一标识
	StoryID string `json:"story_id"`
	// StorySHA256 故事线定义版本摘要
	StorySHA256 string `json:"story_sha256"`
	// RouteID 路由分支唯一标识
	RouteID string `json:"route_id"`
	// LoadSave 加载的检查点存档标识
	LoadSave string `json:"load_save"`
	// RuntimeNS 隔离运行命名空间
	RuntimeNS string `json:"runtime_ns"`
	// Verdict 路由判定结果（PASS/FAIL/BLOCKED）
	Verdict string `json:"verdict"`
	// SoftlockDetected 是否检测到业务软锁死
	SoftlockDetected bool `json:"softlock_detected"`
	// ProbesTotal 声明的蝴蝶探针总数
	ProbesTotal int `json:"probes_total"`
	// ProbesPassed 实际校验通过的蝴蝶探针数
	ProbesPassed int `json:"probes_passed"`
	// UnlockedEnding 成功解锁的结局标识
	UnlockedEnding string `json:"unlocked_ending"`
	// RawOutput 路由执行原始日志文本
	RawOutput string `json:"raw_output"`
	// RepoRoot 执行时规范化仓库根目录
	RepoRoot string `json:"repo_root,omitempty"`
	// SourceCommit 执行时源码Git提交哈希
	SourceCommit string `json:"source_commit,omitempty"`
	// SourceTreeSHA256 执行时源码工作树摘要
	SourceTreeSHA256 string `json:"source_tree_sha256,omitempty"`
	// ReceiptSHA256 路由收据不可变SHA256摘要
	ReceiptSHA256 string `json:"receipt_sha256"`
	// CreatedAt 记录创建UTC时间
	CreatedAt string `json:"created_at"`
}

// StepTraceRecord 描述单步六维 Expected-vs-Actual 预期与实际对比轨迹记录
type StepTraceRecord struct {
	// ID 自增主键
	ID int64 `json:"id"`
	// RunID 所属运行批次ID
	RunID string `json:"run_id"`
	// StoryID 故事线唯一标识
	StoryID string `json:"story_id"`
	// StorySHA256 故事线版本摘要
	StorySHA256 string `json:"story_sha256"`
	// RouteID 路由分支唯一标识
	RouteID string `json:"route_id"`
	// StepNum 步骤序号
	StepNum int `json:"step_num"`
	// Choice 业务分支动作标识
	Choice string `json:"choice"`
	// ExpectedBehavior 预期行为分类枚举
	ExpectedBehavior string `json:"expected_behavior,omitempty"`
	// Preconditions 声明的前置条件强类型映射
	Preconditions FactMap `json:"preconditions,omitempty"`
	// PreconditionsActual 实际观测到的前置条件强类型映射
	PreconditionsActual FactMap `json:"preconditions_actual,omitempty"`
	// ExpectedInput 声明的预期输入参数强类型映射
	ExpectedInput FactMap `json:"expected_input,omitempty"`
	// ActualInput 实际执行时注入的输入参数强类型映射
	ActualInput FactMap `json:"actual_input,omitempty"`
	// BeforeState 步骤执行前状态快照强类型映射
	BeforeState FactMap `json:"before_state,omitempty"`
	// AfterState 步骤执行后状态快照强类型映射
	AfterState FactMap `json:"after_state,omitempty"`
	// ExecutedCmd 实际执行的命令串
	ExecutedCmd string `json:"executed_cmd,omitempty"`
	// DurationMS 步骤执行耗时毫秒数
	DurationMS int64 `json:"duration_ms"`
	// ExpectStatus 预期业务状态值
	ExpectStatus string `json:"expect_status,omitempty"`
	// ActualStatus 实际观测业务状态值
	ActualStatus string `json:"actual_status,omitempty"`
	// ExpectResult 预期返回字段强类型映射
	ExpectResult FactMap `json:"expect_result,omitempty"`
	// ActualResult 实际返回字段强类型映射
	ActualResult FactMap `json:"actual_result,omitempty"`
	// ExpectedProbes 预期耦合蝴蝶探针强类型映射
	ExpectedProbes FactMap `json:"expected_probes,omitempty"`
	// ActualProbes 实际观测蝴蝶探针强类型映射
	ActualProbes FactMap `json:"actual_probes,omitempty"`
	// ForbiddenEffects 严禁触发的副作用标签列表
	ForbiddenEffects []string `json:"forbidden_effects,omitempty"`
	// ObservedEffects 实际观测到的副作用标签列表
	ObservedEffects []string `json:"observed_effects,omitempty"`
	// AntiSoftlock 反软锁逃生不变量声明
	AntiSoftlock string `json:"anti_softlock,omitempty"`
	// Verdict 步骤判定结果（PASS/FAIL/BLOCKED）
	Verdict string `json:"verdict"`
	// DiffReason 预期与实际不符的具体差异原因
	DiffReason string `json:"diff_reason,omitempty"`
	// RawOutput 步骤原始输出文本
	RawOutput string `json:"raw_output,omitempty"`
	// TraceSHA256 步骤轨迹不可变SHA256摘要
	TraceSHA256 string `json:"trace_sha256"`
	// CreatedAt 记录创建UTC时间
	CreatedAt string `json:"created_at"`
}

// marshalMapJSON 将强类型 FactMap 安全编码为非空JSON列文本
// 请求体
// m 待编码的强类型键值映射
// 响应体
// raw JSON对象字符串（空映射返回"{}"）
func marshalMapJSON(m FactMap) string {
	// 1. 空映射直接返回默认空JSON对象字面量
	if len(m) == 0 {
		return "{}"
	}
	// 2. 序列化非空映射并在异常时兜底返回空JSON对象
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// unmarshalMapJSON 以保留64位整数精度的强类型解码还原 FactMap
// 请求体
// raw 数据库存储的JSON文本
// 响应体
// m 反序列化后的强类型 FactMap（空对象返回nil）
func unmarshalMapJSON(raw string) FactMap {
	// 1. 过滤空字符串与空对象字面量
	if raw == "" || raw == "{}" {
		return nil
	}
	// 2. 解码为强类型 FactMap
	var m FactMap
	if err := json.Unmarshal([]byte(raw), &m); err != nil || len(m) == 0 {
		return nil
	}
	return m
}

// marshalSliceJSON 将字符串切片安全编码为非空JSON数组列文本
// 请求体
// s 待编码的字符串切片
// 响应体
// raw JSON数组字符串（空切片返回"[]"）
func marshalSliceJSON(s []string) string {
	// 1. 空切片返回默认空数组字面量
	if len(s) == 0 {
		return "[]"
	}
	// 2. 序列化字符串切片返回
	b, err := json.Marshal(s)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// unmarshalSliceJSON 反序列化数据库JSON数组列为字符串切片
// 请求体
// raw 数据库存储的JSON数组文本
// 响应体
// s 字符串切片（空数组返回nil）
func unmarshalSliceJSON(raw string) []string {
	// 1. 过滤空字符串与空数组字面量
	if raw == "" || raw == "[]" {
		return nil
	}
	// 2. 反序列化非空数组后返回
	var s []string
	if err := json.Unmarshal([]byte(raw), &s); err != nil || len(s) == 0 {
		return nil
	}
	return s
}

// AppendRouteResult 向验证账本追加不可变路由执行收据（绝不覆盖历史FAIL）
// 请求体
// rec 路由执行结果记录
// 响应体
// err SQL插入错误
func (l *StoryLedger) AppendRouteResult(rec RouteResultRecord) error {
	// 1. 加锁补全创建时间与软锁整型标记并追加写入 story_route_results
	l.mu.Lock()
	defer l.mu.Unlock()
	if rec.CreatedAt == "" {
		rec.CreatedAt = nowUTC()
	}
	softInt := 0
	if rec.SoftlockDetected {
		softInt = 1
	}
	_, err := l.db.Exec(
		`INSERT INTO story_route_results(
			run_id, story_id, story_sha256, route_id, load_save, runtime_ns,
			verdict, softlock_detected, probes_total, probes_passed, unlocked_ending,
			raw_output, repo_root, source_commit, source_tree_sha256, receipt_sha256, created_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		rec.RunID, rec.StoryID, rec.StorySHA256, rec.RouteID, rec.LoadSave, rec.RuntimeNS,
		rec.Verdict, softInt, rec.ProbesTotal, rec.ProbesPassed, rec.UnlockedEnding,
		rec.RawOutput, rec.RepoRoot, rec.SourceCommit, rec.SourceTreeSHA256, rec.ReceiptSHA256, rec.CreatedAt,
	)
	return err
}

// ListRouteResults 按追加顺序读取指定故事线的全部历史路由执行收据
// 请求体
// storyID 故事线唯一标识
// 响应体
// out 路由执行结果切片
// err SQL查询错误
func (l *StoryLedger) ListRouteResults(storyID string) ([]RouteResultRecord, error) {
	// 1. 加锁按自增ID升序读取全部历史路由结果
	l.mu.Lock()
	defer l.mu.Unlock()
	rows, err := l.db.Query(
		`SELECT id, run_id, story_id, story_sha256, route_id, load_save, runtime_ns,
		        verdict, softlock_detected, probes_total, probes_passed, unlocked_ending,
		        raw_output, repo_root, source_commit, source_tree_sha256, receipt_sha256, created_at
		 FROM story_route_results WHERE story_id=? ORDER BY id ASC`, storyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RouteResultRecord
	for rows.Next() {
		var r RouteResultRecord
		var soft int
		if err := rows.Scan(
			&r.ID, &r.RunID, &r.StoryID, &r.StorySHA256, &r.RouteID, &r.LoadSave, &r.RuntimeNS,
			&r.Verdict, &soft, &r.ProbesTotal, &r.ProbesPassed, &r.UnlockedEnding,
			&r.RawOutput, &r.RepoRoot, &r.SourceCommit, &r.SourceTreeSHA256, &r.ReceiptSHA256, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		r.SoftlockDetected = soft != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

// AppendStepTrace 向验证账本追加单步六维执行轨迹记录
// 请求体
// tr 六维步骤轨迹记录
// 响应体
// err SQL插入错误
func (l *StoryLedger) AppendStepTrace(tr StepTraceRecord) error {
	// 1. 加锁序列化各维JSON字段并追加写入 story_step_traces
	l.mu.Lock()
	defer l.mu.Unlock()
	if tr.CreatedAt == "" {
		tr.CreatedAt = nowUTC()
	}
	_, err := l.db.Exec(
		`INSERT INTO story_step_traces(
			run_id, story_id, story_sha256, route_id, step_num, choice, expected_behavior,
			preconditions_json, preconditions_actual_json, expected_input_json, actual_input_json,
			before_state_json, after_state_json, executed_cmd, duration_ms,
			expect_status, actual_status, expect_result_json, actual_result_json,
			expected_probes_json, actual_probes_json, forbidden_effects_json, observed_effects_json,
			anti_softlock, verdict, diff_reason, raw_output, trace_sha256, created_at
		) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		tr.RunID, tr.StoryID, tr.StorySHA256, tr.RouteID, tr.StepNum, tr.Choice, tr.ExpectedBehavior,
		marshalMapJSON(tr.Preconditions), marshalMapJSON(tr.PreconditionsActual),
		marshalMapJSON(tr.ExpectedInput), marshalMapJSON(tr.ActualInput),
		marshalMapJSON(tr.BeforeState), marshalMapJSON(tr.AfterState),
		tr.ExecutedCmd, tr.DurationMS,
		tr.ExpectStatus, tr.ActualStatus,
		marshalMapJSON(tr.ExpectResult), marshalMapJSON(tr.ActualResult),
		marshalMapJSON(tr.ExpectedProbes), marshalMapJSON(tr.ActualProbes),
		marshalSliceJSON(tr.ForbiddenEffects), marshalSliceJSON(tr.ObservedEffects),
		tr.AntiSoftlock, tr.Verdict, tr.DiffReason, tr.RawOutput, tr.TraceSHA256, tr.CreatedAt,
	)
	return err
}

// ListStepTraces 按可选批次与路由过滤条件读取六维步骤轨迹列表
// 请求体
// storyID 故事线唯一标识
// runID 可选运行批次过滤ID
// routeID 可选路由分支过滤ID
// 响应体
// out 反序列化后的六维步骤轨迹列表
// err SQL查询错误
func (l *StoryLedger) ListStepTraces(storyID, runID, routeID string) ([]StepTraceRecord, error) {
	// 1. 加锁按显式强类型参数分支执行条件查询，杜绝动态变参切片
	l.mu.Lock()
	defer l.mu.Unlock()
	baseSelect := `SELECT id, run_id, story_id, story_sha256, route_id, step_num, choice, expected_behavior,
		preconditions_json, preconditions_actual_json, expected_input_json, actual_input_json,
		before_state_json, after_state_json, executed_cmd, duration_ms,
		expect_status, actual_status, expect_result_json, actual_result_json,
		expected_probes_json, actual_probes_json, forbidden_effects_json, observed_effects_json,
		anti_softlock, verdict, diff_reason, raw_output, trace_sha256, created_at
	 FROM story_step_traces WHERE story_id=?`
	var (
		rows *sql.Rows
		err  error
	)
	switch {
	case runID != "" && routeID != "":
		rows, err = l.db.Query(baseSelect+` AND run_id=? AND route_id=? ORDER BY id ASC`, storyID, runID, routeID)
	case runID != "":
		rows, err = l.db.Query(baseSelect+` AND run_id=? ORDER BY id ASC`, storyID, runID)
	case routeID != "":
		rows, err = l.db.Query(baseSelect+` AND route_id=? ORDER BY id ASC`, storyID, routeID)
	default:
		rows, err = l.db.Query(baseSelect+` ORDER BY id ASC`, storyID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// 2. 逐行扫描并还原强类型六维 FactMap 与数组字段
	var out []StepTraceRecord
	for rows.Next() {
		var tr StepTraceRecord
		var preJ, preActJ, expInJ, actInJ, befJ, aftJ, expResJ, actResJ, expPrJ, actPrJ, forbJ, obsJ string
		if err := rows.Scan(
			&tr.ID, &tr.RunID, &tr.StoryID, &tr.StorySHA256, &tr.RouteID, &tr.StepNum, &tr.Choice, &tr.ExpectedBehavior,
			&preJ, &preActJ, &expInJ, &actInJ, &befJ, &aftJ, &tr.ExecutedCmd, &tr.DurationMS,
			&tr.ExpectStatus, &tr.ActualStatus, &expResJ, &actResJ, &expPrJ, &actPrJ, &forbJ, &obsJ,
			&tr.AntiSoftlock, &tr.Verdict, &tr.DiffReason, &tr.RawOutput, &tr.TraceSHA256, &tr.CreatedAt,
		); err != nil {
			return nil, err
		}
		tr.Preconditions = unmarshalMapJSON(preJ)
		tr.PreconditionsActual = unmarshalMapJSON(preActJ)
		tr.ExpectedInput = unmarshalMapJSON(expInJ)
		tr.ActualInput = unmarshalMapJSON(actInJ)
		tr.BeforeState = unmarshalMapJSON(befJ)
		tr.AfterState = unmarshalMapJSON(aftJ)
		tr.ExpectResult = unmarshalMapJSON(expResJ)
		tr.ActualResult = unmarshalMapJSON(actResJ)
		tr.ExpectedProbes = unmarshalMapJSON(expPrJ)
		tr.ActualProbes = unmarshalMapJSON(actPrJ)
		tr.ForbiddenEffects = unmarshalSliceJSON(forbJ)
		tr.ObservedEffects = unmarshalSliceJSON(obsJ)
		out = append(out, tr)
	}
	return out, rows.Err()
}
