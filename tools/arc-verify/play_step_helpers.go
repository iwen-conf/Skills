package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// executeDeclaredSteps 逐步执行前置检查、业务动作命令与蝴蝶探针命令并核验六维证据
// 请求体
// s 故事线定义指针
// r 当前路由定义
// runID 运行批次ID
// repoRoot 仓库根目录路径
// varsFile 跨步骤共享变量文件路径
// baseEnv 路由基础环境变量切片
// routeVars 跨步骤动态变量映射
// probesTotal 路由探针总数
// logBuf 路由聚合日志缓冲器
// traces 步骤轨迹收集切片指针
// 响应体
// haltedOutcome 提前终止时的路由执行结果
// probesPassed 累计通过的探针数量
// softlock 是否检测到软锁信号
// halted 是否因BLOCKED或FAIL提前中止后续步骤
func executeDeclaredSteps(
	s *Storyline,
	r Route,
	runID, repoRoot, varsFile string,
	baseEnv []string,
	routeVars FactMap,
	probesTotal int,
	logBuf *strings.Builder,
	traces *[]StepTraceRecord,
) (haltedOutcome routeExecutionOutcome, probesPassed int, softlock bool, halted bool) {
	// 1. 按步骤序号依次执行 precheck_cmd、cmd 与 probe_cmd
	for idx, st := range r.Sequence {
		stepNum := st.Step
		if stepNum == 0 {
			stepNum = idx + 1
		}
		reloadVarsFromFile(varsFile, routeVars)
		stepEnv := buildStepEnv(baseEnv, st, stepNum, routeVars)
		var stepOut strings.Builder
		var cmdsRun []string
		t0 := time.Now()

		// 2. 前置条件探测失败归类为环境/夹具 BLOCKED，绝不误判为业务 FAIL
		if strings.TrimSpace(st.PrecheckCmd) != "" {
			cmdsRun = append(cmdsRun, "precheck: "+st.PrecheckCmd)
			out, pErr := runCommandCapture(repoRoot, st.PrecheckCmd, stepEnv)
			fmt.Fprintf(logBuf, "[precheck %d:%s] %s\n%s\n", stepNum, st.Choice, st.PrecheckCmd, out)
			stepOut.WriteString(out + "\n")
			if pErr != nil {
				dur := time.Since(t0).Milliseconds()
				reason := fmt.Sprintf("precheck_cmd failed: %v", pErr)
				*traces = append(*traces, finalizeStepTrace(s, r, st, runID, strings.Join(cmdsRun, " && "), dur, nil, string(VerdictBlocked), reason, strings.TrimSpace(stepOut.String())))
				return routeExecutionOutcome{
					route:  finalizeRouteRecord(s, r, runID, string(VerdictBlocked), false, probesTotal, probesPassed, "", logBuf.String()),
					traces: *traces,
				}, probesPassed, false, true
			}
		}

		// 3. 执行步骤主命令并在非零退出时记录失败轨迹与软锁检测结果
		if strings.TrimSpace(st.Cmd) != "" {
			cmdsRun = append(cmdsRun, st.Cmd)
			out, runErr := runCommandCapture(repoRoot, st.Cmd, stepEnv)
			fmt.Fprintf(logBuf, "[step %d:%s] %s\n%s\n", stepNum, st.Choice, st.Cmd, out)
			stepOut.WriteString(out + "\n")
			if runErr != nil {
				dur := time.Since(t0).Milliseconds()
				ev := mergeAllStepEvidence(parseStepEvidence(stepOut.String()))
				if hasSoftlockSignal(out, true, ev) {
					softlock = true
				}
				*traces = append(*traces, finalizeStepTrace(s, r, st, runID, strings.Join(cmdsRun, " && "), dur, ev, string(VerdictFail), fmt.Sprintf("step command exited non-zero: %v", runErr), strings.TrimSpace(stepOut.String())))
				return routeExecutionOutcome{
					route:  finalizeRouteRecord(s, r, runID, string(VerdictFail), softlock, probesTotal, probesPassed, "", logBuf.String()),
					traces: *traces,
				}, probesPassed, softlock, true
			}
		}

		// 4. 执行独立旁路探针命令核验跨表与异步副作用
		if strings.TrimSpace(st.ProbeCmd) != "" {
			cmdsRun = append(cmdsRun, "probe: "+st.ProbeCmd)
			reloadVarsFromFile(varsFile, routeVars)
			stepEnv = buildStepEnv(baseEnv, st, stepNum, routeVars)
			out, pErr := runCommandCapture(repoRoot, st.ProbeCmd, stepEnv)
			fmt.Fprintf(logBuf, "[probe %d] %s\n%s\n", stepNum, st.ProbeCmd, out)
			stepOut.WriteString(out + "\n")
			if pErr != nil {
				dur := time.Since(t0).Milliseconds()
				ev := mergeAllStepEvidence(parseStepEvidence(stepOut.String()))
				if hasSoftlockSignal(out, true, ev) {
					softlock = true
				}
				*traces = append(*traces, finalizeStepTrace(s, r, st, runID, strings.Join(cmdsRun, " && "), dur, ev, string(VerdictFail), fmt.Sprintf("probe_cmd exited non-zero: %v", pErr), strings.TrimSpace(stepOut.String())))
				return routeExecutionOutcome{
					route:  finalizeRouteRecord(s, r, runID, string(VerdictFail), softlock, probesTotal, probesPassed, "", logBuf.String()),
					traces: *traces,
				}, probesPassed, softlock, true
			}
		}

		// 5. 合并单步结构化证据、回写提取变量并执行内核级 Expected-vs-Actual 判等
		dur := time.Since(t0).Milliseconds()
		rawStepOut := strings.TrimSpace(stepOut.String())
		mergedEv := mergeAllStepEvidence(parseStepEvidence(rawStepOut))
		if hasSoftlockSignal(rawStepOut, false, mergedEv) {
			softlock = true
		}
		if mergedEv != nil && len(mergedEv.ExtractedVars) > 0 {
			for k, v := range mergedEv.ExtractedVars {
				routeVars[k] = v
			}
			saveVarsToFile(varsFile, routeVars)
		}
		stepVerdict, diffReason := evaluateStepEvidence(st, mergedEv)
		if softlock && Verdict(stepVerdict) == VerdictPass {
			stepVerdict = string(VerdictFail)
			diffReason = "explicit softlock marker reported in step evidence/output"
		}
		*traces = append(*traces, finalizeStepTrace(s, r, st, runID, strings.Join(cmdsRun, " && "), dur, mergedEv, stepVerdict, diffReason, rawStepOut))
		if Verdict(stepVerdict) != VerdictPass {
			return routeExecutionOutcome{
				route:  finalizeRouteRecord(s, r, runID, stepVerdict, softlock, probesTotal, probesPassed, "", logBuf.String()),
				traces: *traces,
			}, probesPassed, softlock, true
		}
		probesPassed += len(st.ButterflyProbes)
	}
	return routeExecutionOutcome{}, probesPassed, softlock, false
}

// executeRouteVerifyCommand 执行路由级 verify_cmd 并同步更新六维步骤轨迹与重算摘要
// 请求体
// s 故事线定义指针
// r 当前路由定义
// runID 运行批次ID
// repoRoot 仓库根目录路径
// varsFile 跨步骤共享变量文件路径
// baseEnv 路由基础环境变量切片
// routeVars 跨步骤动态变量映射
// hasStepCmds 是否已执行过步骤级命令
// probesTotal 路由探针总数
// probesPassed 已通过探针计数
// softlock 累计软锁状态
// logBuf 路由聚合日志缓冲器
// traces 步骤轨迹收集切片指针
// 响应体
// haltedOutcome 失败或阻断时的路由执行结果
// nextProbesPassed 更新后的通过探针数
// nextSoftlock 更新后的软锁状态
// halted 是否提前终止
func executeRouteVerifyCommand(
	s *Storyline,
	r Route,
	runID, repoRoot, varsFile string,
	baseEnv []string,
	routeVars FactMap,
	hasStepCmds bool,
	probesTotal, probesPassed int,
	softlock bool,
	logBuf *strings.Builder,
	traces *[]StepTraceRecord,
) (haltedOutcome routeExecutionOutcome, nextProbesPassed int, nextSoftlock bool, halted bool) {
	// 1. 加载最新跨步骤变量并执行路由级 verify_cmd
	reloadVarsFromFile(varsFile, routeVars)
	varsJSON, _ := json.Marshal(routeVars)
	vEnv := append(append([]string{}, baseEnv...), "ARC_VARS_JSON="+string(varsJSON))
	t0 := time.Now()
	out, runErr := runCommandCapture(repoRoot, r.VerifyCmd, vEnv)
	dur := time.Since(t0).Milliseconds()
	logBuf.WriteString("[verify_cmd] " + r.VerifyCmd + "\n" + out + "\n")

	// 2. 按步骤序号归并结构化证据并检测软锁信号
	evByStep := map[int]*StepEvidencePayload{}
	evs := parseStepEvidence(out)
	mergedVerifyEv := mergeAllStepEvidence(evs)
	for i := range evs {
		sn := evs[i].Step
		if sn <= 0 {
			sn = i + 1
		}
		evByStep[sn] = mergeStepEvidence(evByStep[sn], &evs[i])
	}
	if hasSoftlockSignal(out, runErr != nil, mergedVerifyEv) {
		softlock = true
	}

	// 3. 若 verify_cmd 非零退出，则补齐或回写最后一步失败轨迹并重算哈希
	if runErr != nil {
		if !hasStepCmds {
			for idx, st := range r.Sequence {
				sn := st.Step
				if sn == 0 {
					sn = idx + 1
				}
				*traces = append(*traces, finalizeStepTrace(s, r, st, runID, r.VerifyCmd, dur, evByStep[sn], string(VerdictFail), fmt.Sprintf("verify_cmd exited non-zero: %v", runErr), out))
			}
		} else if len(*traces) > 0 {
			last := &(*traces)[len(*traces)-1]
			last.Verdict = string(VerdictFail)
			last.DiffReason = fmt.Sprintf("verify_cmd exited non-zero: %v", runErr)
			if out != "" {
				if last.RawOutput != "" {
					last.RawOutput = last.RawOutput + "\n" + out
				} else {
					last.RawOutput = out
				}
			}
			last.TraceSHA256 = computeStepTraceSHA256(*last)
		}
		return routeExecutionOutcome{
			route:  finalizeRouteRecord(s, r, runID, string(VerdictFail), softlock, probesTotal, 0, "", logBuf.String()),
			traces: *traces,
		}, 0, softlock, true
	}

	// 4. 当未配置单步命令时由 verify_cmd 输出驱动逐步骤 Expected-vs-Actual 核验
	if !hasStepCmds {
		for idx, st := range r.Sequence {
			sn := st.Step
			if sn == 0 {
				sn = idx + 1
			}
			ev := evByStep[sn]
			stepVerdict, diffReason := evaluateStepEvidence(st, ev)
			if softlock && Verdict(stepVerdict) == VerdictPass {
				stepVerdict = string(VerdictFail)
				diffReason = "explicit softlock marker reported in verify_cmd output"
			}
			*traces = append(*traces, finalizeStepTrace(s, r, st, runID, r.VerifyCmd, dur, ev, stepVerdict, diffReason, out))
			if Verdict(stepVerdict) != VerdictPass {
				return routeExecutionOutcome{
					route:  finalizeRouteRecord(s, r, runID, stepVerdict, softlock, probesTotal, probesPassed, "", logBuf.String()),
					traces: *traces,
				}, probesPassed, softlock, true
			}
			probesPassed += len(st.ButterflyProbes)
		}
	}
	return routeExecutionOutcome{}, probesPassed, softlock, false
}
