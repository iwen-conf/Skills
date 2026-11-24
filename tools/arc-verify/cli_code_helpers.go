package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// printCodeUsage 向标准错误输出 arc verify code 子命令完整用法说明
// 请求体
// 无
// 响应体
// 无
func printCodeUsage() {
	// 1. 打印六项代码级测试引擎子命令与标志说明
	fmt.Fprintln(os.Stderr, `Usage:
  arc verify code catalog  [--lang rust|go|typescript|python|c_cpp] [--tier P0|P1|P2|P3] [--technique ID] [--with-coverage] [--format json|yaml|ai-md]
  arc verify code plan     [--repo DIR] [--lang LANG] [--max-tier P0|P1|P2|P3] [--plan-id ID] [--strict] [--all-applicable] [--no-probe] [--save] [--out FILE] [--format json|yaml|ai-md] [--db PATH]
  arc verify code run      [--repo DIR] [--plan FILE|ID] [--plan-id ID] [--lang LANG] [--max-tier P0|P1|P2|P3] [--technique ID1,ID2] [--include-recommended] [--strict] [--dry-run] [--allow-unchecked-harness] [--timeout SEC] [--format json|ai-md] [--db PATH]
  arc verify code explain  [--plan ID] [--run ID] [--format json|ai-md] [--db PATH]
  arc verify code gate     [--plan ID] [--run ID] [--min-tier P0|P1|P2|P3] [--require-overlooked] [--format json|ai-md] [--db PATH]
  arc verify code snapshot [--plan ID] [--run ID] --out <empty_dir> [--db PATH]`)
}

// marshalCodePlanByExt 根据输出文件后缀序列化代码测试计划为YAML或JSON字节流
// 请求体
// plan 代码测试计划指针
// path 输出文件路径
// 响应体
// raw 序列化后的文件字节切片
// err 编码错误
func marshalCodePlanByExt(plan *CodeVerifyPlan, path string) ([]byte, error) {
	// 1. 识别 .json 后缀否则默认使用 YAML 编码
	if strings.EqualFold(filepath.Ext(path), ".json") {
		raw, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(raw, '\n'), nil
	}
	return yaml.Marshal(plan)
}

// resolveCodePlanForRun 解析 --plan 路径或账本内 plan_id，仅在显式指定仓库路径时覆盖计划根目录
// 请求体
// ledger 验证账本实例指针
// planFile --plan 传入的文件路径或计划ID
// planID 可选覆盖的 --plan-id 标识
// repoDir 命令行解析出的仓库路径
// explicitRepo 是否显式通过 --repo 或位置参数指定仓库路径
// lang 可选覆盖的主语言
// maxTier 最高优先级上限
// strict 是否开启严格夹具检查
// 响应体
// plan 解析或装配好的代码测试计划指针
// runRepoRoot 传入 runCodeVerifyPlan 的仓库覆盖路径（未显式指定时为空以保留 plan.RepoRoot）
// err 计划读取、反序列化或仓库剖析失败错误
func resolveCodePlanForRun(
	ledger *StoryLedger,
	planFile, planID, repoDir string,
	explicitRepo bool,
	lang string,
	maxTier CodePriorityTier,
	strict bool,
) (*CodeVerifyPlan, string, error) {
	// 1. 若指定 --plan 则先尝试读取磁盘文件，不存在时回退读取账本内已保存 plan_id
	trimmedPlan := strings.TrimSpace(planFile)
	if trimmedPlan != "" {
		var (
			plan *CodeVerifyPlan
			err  error
		)
		if _, statErr := os.Stat(trimmedPlan); statErr == nil {
			plan, err = loadCodeVerifyPlan(trimmedPlan)
		} else if errors.Is(statErr, os.ErrNotExist) {
			plan, err = ledger.LoadCodePlan(trimmedPlan)
			if err != nil {
				err = fmt.Errorf("load code verify plan %q (not found on disk or in ledger): %w", trimmedPlan, err)
			}
		} else {
			err = statErr
		}
		if err != nil {
			return nil, "", err
		}
		if planID != "" {
			plan.PlanID = planID
			plan.PlanSHA256 = computeCodePlanSHA256(plan)
		}
		runRepoRoot := ""
		if explicitRepo {
			runRepoRoot = repoDir
		}
		return plan, runRepoRoot, nil
	}

	// 2. 未指定 --plan 时按仓库根目录实时构建 26 项代码级测试计划
	plan, err := buildCodeVerifyPlan(repoDir, CodePlanOptions{
		PlanID:     planID,
		Language:   lang,
		MaxTier:    maxTier,
		Strict:     strict,
		ProbeTools: true,
	})
	if err != nil {
		return nil, "", err
	}
	return plan, repoDir, nil
}
