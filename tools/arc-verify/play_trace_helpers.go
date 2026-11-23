package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// buildStepEnv 组装注入单步执行子进程的六维预期与动态变量环境列表
// 请求体
// baseEnv 路由基础环境变量切片
// st 当前步骤声明结构
// stepNum 步骤序号
// routeVars 跨步骤提取强类型变量映射
// 响应体
// env 完整子进程环境变量切片
func buildStepEnv(baseEnv []string, st RouteStep, stepNum int, routeVars FactMap) []string {
	// 1. 序列化前置条件、预期输入、探针与跨步骤变量为JSON字符串
	preJSON, _ := json.Marshal(st.Preconditions)
	inJSON, _ := json.Marshal(st.effectiveExpectedInput())
	probesJSON, _ := json.Marshal(st.ButterflyProbes)
	varsJSON, _ := json.Marshal(routeVars)
	env := append([]string{}, baseEnv...)
	env = append(env,
		"ARC_STEP_NUM="+strconv.Itoa(stepNum),
		"ARC_STEP_CHOICE="+st.Choice,
		"ARC_EXPECTED_BEHAVIOR="+st.ExpectedBehavior,
		"ARC_PRECONDITIONS_JSON="+string(preJSON),
		"ARC_EXPECTED_INPUT_JSON="+string(inJSON),
		"ARC_EXPECTED_STATUS="+st.ExpectStatus,
		"ARC_EXPECTED_PROBES_JSON="+string(probesJSON),
		"ARC_VARS_JSON="+string(varsJSON),
	)
	// 2. 将每个动态强类型变量同步导出为 ARC_VAR_<KEY> 环境变量
	for _, k := range sortedFactKeys(routeVars) {
		envKey := "ARC_VAR_" + strings.ToUpper(sanitizeNS(k))
		env = append(env, fmt.Sprintf("%s=%s", envKey, routeVars[k].FormatText()))
	}
	return env
}

// reloadVarsFromFile 从分片共享JSON文件增量加载跨步骤动态变量
// 请求体
// path 变量文件路径
// dst 目标强类型变量映射
// 响应体
// 无
func reloadVarsFromFile(path string, dst FactMap) {
	// 1. 读取文件内容并按强类型 FactMap 合并键值到目标映射
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var m FactMap
	if err := json.Unmarshal(raw, &m); err == nil {
		for k, v := range m {
			dst[k] = v
		}
	}
}

// saveVarsToFile 将更新后的跨步骤动态变量持久化写入分片变量文件
// 请求体
// path 变量文件路径
// vars 强类型变量键值映射
// 响应体
// 无
func saveVarsToFile(path string, vars FactMap) {
	// 1. 序列化为缩进JSON并写入目标文件
	b, err := json.MarshalIndent(vars, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, append(b, '\n'), 0o644)
	}
}

// runCommandCapture 在指定目录与环境变量下执行Shell命令并捕获合并输出
// 请求体
// dir 命令工作目录
// command Shell命令字符串
// env 环境变量切片
// 响应体
// output 去除首尾空白的标准输出与错误合并文本
// err 命令非零退出错误
func runCommandCapture(dir, command string, env []string) (string, error) {
	// 1. 启动 sh -c 执行命令并捕获合并输出流
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// finalizeStepTrace 组装单步六维轨迹记录并计算不可变 TraceSHA256 摘要
// 请求体
// s 故事线定义指针
// r 当前路由定义
// st 当前步骤声明
// runID 运行批次ID
// executedCmd 实际执行命令串
// durationMS 耗时毫秒数
// ev 解析合并后的结构化证据指针
// verdict 步骤判定结果
// diffReason 差异原因说明
// rawOutput 原始命令输出
// 响应体
// tr 含TraceSHA256签名的完整步骤轨迹记录
func finalizeStepTrace(
	s *Storyline,
	r Route,
	st RouteStep,
	runID, executedCmd string,
	durationMS int64,
	ev *StepEvidencePayload,
	verdict, diffReason, rawOutput string,
) StepTraceRecord {
	// 1. 填充步骤预期声明基准字段
	stepNum := st.Step
	if stepNum == 0 {
		stepNum = 1
	}
	tr := StepTraceRecord{
		RunID:            runID,
		StoryID:          s.StoryID,
		StorySHA256:      s.SHA256,
		RouteID:          r.RouteID,
		StepNum:          stepNum,
		Choice:           st.Choice,
		ExpectedBehavior: st.ExpectedBehavior,
		Preconditions:    st.Preconditions,
		ExpectedInput:    st.effectiveExpectedInput(),
		BeforeState:      st.BeforeState,
		ExecutedCmd:      executedCmd,
		DurationMS:       durationMS,
		ExpectStatus:     st.ExpectStatus,
		ExpectResult:     st.ExpectResult,
		ExpectedProbes:   st.ButterflyProbes,
		ForbiddenEffects: st.ForbiddenEffects,
		AntiSoftlock:     st.AntiSoftlock,
		Verdict:          verdict,
		DiffReason:       diffReason,
		RawOutput:        rawOutput,
		CreatedAt:        nowUTC(),
	}
	// 2. 注入实际观测证据并计算内容SHA256签名
	if ev != nil {
		tr.PreconditionsActual = ev.PreconditionsActual
		tr.ActualInput = ev.ActualInput
		if len(ev.BeforeState) > 0 {
			tr.BeforeState = ev.BeforeState
		}
		tr.AfterState = ev.AfterState
		tr.ActualStatus = ev.ActualStatus
		tr.ActualResult = ev.ActualResult
		tr.ActualProbes = ev.ActualProbes
		tr.ObservedEffects = ev.ObservedEffects
	}
	if tr.ActualStatus == "" && Verdict(verdict) == VerdictPass && st.ExpectStatus != "" {
		tr.ActualStatus = st.ExpectStatus
	}
	tr.TraceSHA256 = computeStepTraceSHA256(tr)
	return tr
}

// computeStepTraceSHA256 清空自身哈希字段后计算步骤轨迹的确定性SHA256摘要
// 请求体
// tr 步骤轨迹结构体副本
// 响应体
// digest 64位十六进制SHA256摘要
func computeStepTraceSHA256(tr StepTraceRecord) string {
	// 1. 置空 TraceSHA256 字段后序列化计算摘要
	tr.TraceSHA256 = ""
	payload, _ := json.Marshal(tr)
	return sha256Hex(payload)
}

// finalizeRouteRecord 组装路由执行收据并计算不可变 ReceiptSHA256 摘要
// 请求体
// s 故事线定义指针
// r 路由定义
// runID 运行批次ID
// verdict 路由判定结果
// softlock 是否检测到软锁
// probesTotal 探针总数
// probesPassed 通过探针数
// unlocked 解锁结局标识
// rawOutput 原始日志文本
// 响应体
// rec 含ReceiptSHA256签名的路由执行收据
func finalizeRouteRecord(
	s *Storyline,
	r Route,
	runID, verdict string,
	softlock bool,
	probesTotal, probesPassed int,
	unlocked, rawOutput string,
) RouteResultRecord {
	// 1. 构造路由结果记录并计算整体内容SHA256收据指纹
	rec := RouteResultRecord{
		RunID:            runID,
		StoryID:          s.StoryID,
		StorySHA256:      s.SHA256,
		RouteID:          r.RouteID,
		LoadSave:         r.LoadSave,
		RuntimeNS:        r.RuntimeNS,
		Verdict:          verdict,
		SoftlockDetected: softlock,
		ProbesTotal:      probesTotal,
		ProbesPassed:     probesPassed,
		UnlockedEnding:   unlocked,
		RawOutput:        rawOutput,
		CreatedAt:        nowUTC(),
	}
	payload, _ := json.Marshal(rec)
	rec.ReceiptSHA256 = sha256Hex(payload)
	return rec
}
