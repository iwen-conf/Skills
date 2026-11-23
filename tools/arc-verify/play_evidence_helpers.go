package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseStepEvidence 从命令输出行中提取全部 [ARC_STEP_EVIDENCE] 结构化载荷
// 请求体
// output 命令标准输出与错误合并文本
// 响应体
// out 按出现顺序解析出的步骤证据切片
func parseStepEvidence(output string) []StepEvidencePayload {
	// 1. 逐行扫描前缀标记并按强类型 StepEvidencePayload 解码JSON载荷
	var out []StepEvidencePayload
	for _, line := range strings.Split(output, "\n") {
		idx := strings.Index(line, stepEvidencePrefix)
		if idx < 0 {
			continue
		}
		rawJSON := strings.TrimSpace(line[idx+len(stepEvidencePrefix):])
		var payload StepEvidencePayload
		if err := json.Unmarshal([]byte(rawJSON), &payload); err == nil {
			out = append(out, payload)
		}
	}
	return out
}

// mergeAllStepEvidence 按顺序合并同一执行阶段产出的多条步骤证据载荷
// 请求体
// evs 步骤证据载荷切片
// 响应体
// merged 合并后的步骤证据指针（无证据时返回nil）
func mergeAllStepEvidence(evs []StepEvidencePayload) *StepEvidencePayload {
	// 1. 依次叠加各条证据中的状态、探针、副作用与提取变量
	var merged *StepEvidencePayload
	for i := range evs {
		merged = mergeStepEvidence(merged, &evs[i])
	}
	return merged
}

// hasSoftlockSignal 区分真实软锁标记与正常通过日志中的说明文本（如 without softlock）
// 请求体
// output 命令输出文本
// cmdFailed 命令是否以非零状态码失败退出
// ev 解析出的结构化步骤证据指针
// 响应体
// detected 是否确认为真实业务软锁死
func hasSoftlockSignal(output string, cmdFailed bool, ev *StepEvidencePayload) bool {
	// 1. 优先检查结构化证据中的布尔标记与观测副作用枚举
	if ev != nil {
		if ev.Softlock {
			return true
		}
		for _, eff := range ev.ObservedEffects {
			upEff := strings.ToUpper(strings.TrimSpace(eff))
			if upEff == "SOFTLOCK" || upEff == "SOFTLOCK_DETECTED" {
				return true
			}
		}
	}
	// 2. 检查显式 [ARC_SOFTLOCK] 协议标记或失败退出时的自由文本告警
	upOut := strings.ToUpper(output)
	if strings.Contains(upOut, softlockMarkerPrefix) {
		return true
	}
	if cmdFailed && strings.Contains(upOut, "SOFTLOCK") {
		return true
	}
	return false
}

// mergeStepEvidence 将增量步骤证据字段合并至基准证据对象
// 请求体
// base 已有步骤证据指针
// next 新增步骤证据指针
// 响应体
// merged 合并后的步骤证据对象指针
func mergeStepEvidence(base, next *StepEvidencePayload) *StepEvidencePayload {
	// 1. 处理任一侧为空的快速返回分支
	if base == nil {
		if next == nil {
			return nil
		}
		cp := *next
		return &cp
	}
	if next == nil {
		return base
	}
	// 2. 合并各维强类型 FactMap、副作用列表与软锁标志
	if next.Step != 0 {
		base.Step = next.Step
	}
	if len(next.PreconditionsActual) > 0 {
		base.PreconditionsActual = mergeFactMaps(base.PreconditionsActual, next.PreconditionsActual)
	}
	if len(next.ActualInput) > 0 {
		base.ActualInput = mergeFactMaps(base.ActualInput, next.ActualInput)
	}
	if len(next.BeforeState) > 0 {
		base.BeforeState = mergeFactMaps(base.BeforeState, next.BeforeState)
	}
	if len(next.AfterState) > 0 {
		base.AfterState = mergeFactMaps(base.AfterState, next.AfterState)
	}
	if next.ActualBehavior != "" {
		base.ActualBehavior = next.ActualBehavior
	}
	if next.ActualStatus != "" {
		base.ActualStatus = next.ActualStatus
	}
	if len(next.ActualResult) > 0 {
		base.ActualResult = mergeFactMaps(base.ActualResult, next.ActualResult)
	}
	if len(next.ActualProbes) > 0 {
		base.ActualProbes = mergeFactMaps(base.ActualProbes, next.ActualProbes)
	}
	if len(next.ObservedEffects) > 0 {
		base.ObservedEffects = append(base.ObservedEffects, next.ObservedEffects...)
	}
	if len(next.ExtractedVars) > 0 {
		base.ExtractedVars = mergeFactMaps(base.ExtractedVars, next.ExtractedVars)
	}
	if next.Softlock {
		base.Softlock = true
	}
	return base
}

// mergeFactMaps 浅拷贝合并两个强类型 FactMap（后者覆盖同名键）
// 请求体
// a 基础强类型键值映射
// b 增量覆盖强类型键值映射
// 响应体
// out 合并后的新强类型键值映射
func mergeFactMaps(a, b FactMap) FactMap {
	// 1. 创建新映射依次写入两侧强类型键值对
	out := make(FactMap, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// evaluateStepEvidence 执行内核级六维 Expected-vs-Actual 对比（前置不符判BLOCKED，结果/探针/禁令违例判FAIL）
// 请求体
// st 步骤预期声明
// ev 实际观测到的结构化证据指针
// 响应体
// verdict 闭合步骤判定结果
// diffReason 差异明细拼接描述
func evaluateStepEvidence(st RouteStep, ev *StepEvidencePayload) (verdict string, diffReason string) {
	// 1. 无结构化证据时保持退出码驱动通过，显式软锁直接判FAIL
	if ev == nil {
		return string(VerdictPass), ""
	}
	if ev.Softlock {
		return string(VerdictFail), "softlock reported in step evidence"
	}
	// 2. 核对前置条件，不满足归类为环境夹具 BLOCKED
	if len(st.Preconditions) > 0 && len(ev.PreconditionsActual) > 0 {
		for _, k := range sortedFactKeys(st.Preconditions) {
			expVal := st.Preconditions[k]
			actVal, ok := ev.PreconditionsActual[k]
			if !ok || !expVal.Equal(actVal) {
				return string(VerdictBlocked), fmt.Sprintf("precondition %q mismatch: expected %s, actual %s", k, expVal.FormatText(), actVal.FormatText())
			}
		}
	}
	// 3. 逐项核对业务状态、预期输出、蝴蝶探针与零容忍禁止副作用
	var diffs []string
	if strings.TrimSpace(st.ExpectStatus) != "" && strings.TrimSpace(ev.ActualStatus) != "" {
		if strings.TrimSpace(st.ExpectStatus) != strings.TrimSpace(ev.ActualStatus) {
			diffs = append(diffs, fmt.Sprintf("status mismatch: expected %q, actual %q", st.ExpectStatus, ev.ActualStatus))
		}
	}
	if len(st.ExpectResult) > 0 && ev.ActualResult != nil {
		for _, k := range sortedFactKeys(st.ExpectResult) {
			expVal := st.ExpectResult[k]
			actVal, ok := ev.ActualResult[k]
			if !ok || !expVal.Equal(actVal) {
				diffs = append(diffs, fmt.Sprintf("result[%s] mismatch: expected %s, actual %s", k, expVal.FormatText(), actVal.FormatText()))
			}
		}
	}
	if len(st.ButterflyProbes) > 0 && ev.ActualProbes != nil {
		for _, k := range sortedFactKeys(st.ButterflyProbes) {
			expVal := st.ButterflyProbes[k]
			actVal, ok := ev.ActualProbes[k]
			if !ok || !expVal.Equal(actVal) {
				diffs = append(diffs, fmt.Sprintf("butterfly_probe[%s] mismatch: expected %s, actual %s", k, expVal.FormatText(), actVal.FormatText()))
			}
		}
	}
	if len(st.ForbiddenEffects) > 0 && len(ev.ObservedEffects) > 0 {
		obsSet := map[string]bool{}
		for _, eff := range ev.ObservedEffects {
			obsSet[strings.TrimSpace(eff)] = true
		}
		for _, forb := range st.ForbiddenEffects {
			if obsSet[strings.TrimSpace(forb)] {
				diffs = append(diffs, fmt.Sprintf("forbidden_effect triggered: %q", forb))
			}
		}
	}
	if len(diffs) > 0 {
		return string(VerdictFail), strings.Join(diffs, "; ")
	}
	return string(VerdictPass), ""
}
