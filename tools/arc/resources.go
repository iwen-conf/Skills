package main

import (
	"errors"
	"flag"
	"fmt"
)

// resourceCommand 解析资源子命令参数并在仓库排他锁保护下路由至对应服务函数
// 请求体
// kind 资源类别（task/file/change/update）
// args 子命令及参数列表
// 响应体
// err 参数解析错误或底层资源执行错误
func resourceCommand(kind string, args []string) error {
	// 1. 校验子命令动作并解析统一资源标志集合
	if len(args) == 0 {
		return fmt.Errorf("usage: arc %s <action> [--repo DIR] [flags]", kind)
	}
	action := args[0]
	parseArgs := args[1:]
	if kind == "update" {
		action = "collect"
		parseArgs = args
	}
	fs := flag.NewFlagSet("arc "+kind+" "+action, flag.ContinueOnError)
	repoArg := fs.String("repo", ".", "repository root")
	idFlag := fs.String("id", "", "resource id")
	title := fs.String("title", "", "task title")
	to := fs.String("to", "", "target task state")
	expected := fs.Int("expected-version", 0, "expected current version for CAS")
	path := fs.String("path", "", "repo-relative source path")
	task := fs.String("task", "", "optional owning task id")
	worktree := fs.String("worktree", ".", "Git worktree path")
	changedOnly := fs.Bool("changed-only", false, "collect only Git working tree changes")
	graphFlag := fs.String("graph", "", "business graph id for link-verify")
	obligationFlag := fs.String("obligation", "", "verify obligation id for link-verify")
	verifyTaskFlag := fs.String("verify-task", "", "optional verify task id override for link-verify")
	if err := fs.Parse(parseArgs); err != nil {
		return err
	}
	// 2. 归一可选单位置参数作为资源ID或文件路径
	if len(fs.Args()) > 1 {
		return fmt.Errorf("too many positional arguments: %v", fs.Args())
	}
	if len(fs.Args()) == 1 {
		switch {
		case *idFlag == "" && (kind == "task" || kind == "change"):
			*idFlag = fs.Arg(0)
		case *path == "" && kind == "file":
			*path = fs.Arg(0)
		default:
			return errors.New("ambiguous resource identifier")
		}
	}
	// 3. 解析仓库根目录并预检任务完成操作必填参数
	root, err := repositoryRoot(*repoArg)
	if err != nil {
		return err
	}
	completing := kind == "task" && (action == "complete" || (action == "transition" && TaskStatus(*to) == TaskStatusDone))
	if completing && (*idFlag == "" || *expected < 1) {
		return errors.New("completion requires --id and --expected-version")
	}
	// 4. 在仓库排他锁保护下执行完成证明预检与具体资源分发
	return withArcLock(root, func() error {
		var completion *verifiedTaskProof
		if completing {
			proof, pErr := preflightTaskCompletion(root, *idFlag, *expected)
			if pErr != nil {
				return pErr
			}
			completion = &proof
		}
		db, err := openActiveArcDB(root)
		if err != nil {
			return err
		}
		defer db.Close()
		switch kind {
		case "task":
			if completion != nil {
				return commitTaskCompletion(db, root, *idFlag, *expected, *completion)
			}
			switch action {
			case "show":
				return showTaskWithProof(db, root, *idFlag)
			case "link-verify":
				return linkTaskVerify(db, *idFlag, *graphFlag, *obligationFlag, *verifyTaskFlag)
			case "verify-status":
				return showTaskVerifyStatus(db, *idFlag)
			default:
				return executeTask(db, action, *idFlag, *title, *to, *expected)
			}
		case "file":
			switch action {
			case "update":
				return executeFile(db, root, *path, *task)
			case "show":
				return showFile(db, *path)
			default:
				return fmt.Errorf("unsupported arc file action %q (update|show)", action)
			}
		case "change":
			switch action {
			case "collect":
				return collectChanges(db, root, *task, *worktree)
			case "show":
				return showChange(db, *idFlag)
			default:
				return fmt.Errorf("unsupported arc change action %q", action)
			}
		case "update":
			if action != "collect" {
				return fmt.Errorf("unsupported arc update action %q", action)
			}
			if !*changedOnly {
				return errors.New("arc update requires --changed-only in v1; broad full reindexing is forbidden")
			}
			return collectChanges(db, root, *task, *worktree)
		default:
			return fmt.Errorf("unknown arc resource %q", kind)
		}
	})
}

// preflightTaskCompletion 在释放数据库连接后调用独立验证引擎重验任务完成证明
// 请求体
// root 仓库根目录路径
// taskID 待完成任务ID
// expected 期望当前版本号
// 响应体
// proof 重验通过的原生完成证明结构
// err 状态非verifying、版本不匹配或验证证明拒绝错误
func preflightTaskCompletion(root, taskID string, expected int) (verifiedTaskProof, error) {
	// 1. 短暂打开数据库核验当前任务处于期望版本的 verifying 状态后立即关闭句柄
	preflight, err := openActiveArcDB(root)
	if err != nil {
		return verifiedTaskProof{}, err
	}
	var status string
	var version int
	err = preflight.QueryRow("SELECT status,version FROM arc_tasks WHERE id=?", taskID).Scan(&status, &version)
	if closeErr := preflight.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return verifiedTaskProof{}, err
	}
	if version != expected || TaskStatus(status) != TaskStatusVerifying {
		return verifiedTaskProof{}, fmt.Errorf("completion requires current verifying task version %d (actual %s/%d)", expected, status, version)
	}
	// 2. 调用独立验证引擎加载并核验原生绑定收据证明
	return loadCompletionProof(root, taskID)
}
