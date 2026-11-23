package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// policyReceiptCheckFixture 描述测试用门禁收据单项检查结构
type policyReceiptCheckFixture struct {
	// ID 检查项唯一标识
	ID string `json:"id"`
	// Verdict 检查项判定结果
	Verdict string `json:"verdict"`
}

// policyReceiptFixture 描述测试用策略门禁收据完整结构
type policyReceiptFixture struct {
	// TaskID 关联任务ID
	TaskID string `json:"task_id"`
	// Verdict 收据总判定结果
	Verdict string `json:"verdict"`
	// CreatedAt 收据创建时间
	CreatedAt string `json:"created_at"`
	// Head 提交哈希
	Head string `json:"head"`
	// Digest 源码快照摘要
	Digest string `json:"digest"`
	// Tree Git树哈希
	Tree string `json:"tree"`
	// Dirty 工作区脏文件列表
	Dirty []string `json:"dirty"`
	// Checks 检查项列表
	Checks []policyReceiptCheckFixture `json:"checks"`
}

// indexDoctorFixture 描述测试用 arc-idx doctor 响应结构
type indexDoctorFixture struct {
	// RepoRoot 仓库根目录路径
	RepoRoot string `json:"repo_root"`
	// Healthy 索引健康状态
	Healthy bool `json:"healthy"`
	// IndexStale 索引是否过期
	IndexStale bool `json:"index_stale"`
	// TagsPresent 符号标签文件是否存在
	TagsPresent bool `json:"tags_present"`
}

// livePolicyActiveFixture 描述测试用活跃策略状态结构
type livePolicyActiveFixture struct {
	// ID 活跃任务ID
	ID string `json:"id"`
	// Receipt 收据原始JSON载荷
	Receipt json.RawMessage `json:"receipt"`
}

// livePolicyStateFixture 描述测试用策略状态文件顶层结构
type livePolicyStateFixture struct {
	// Active 当前活跃任务对象
	Active livePolicyActiveFixture `json:"active"`
}

// initClosureRepo 初始化用于闭环测试的临时Git仓库与初始提交
// 请求体
// t 测试上下文指针
// root 临时仓库根目录路径
// 响应体
// 无
func initClosureRepo(t *testing.T, root string) {
	// 1. 初始化Git配置并写入提交初始 README.txt
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.name", "Arc Test"},
		{"config", "user.email", "arc-test@example.invalid"},
	} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("initial\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "README.txt"}, {"commit", "-qm", "initial"}} {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
}

// TestOutboxFailureRaceAndIdempotentRecovery 验证Outbox失败重试、并发修改竞态拦截与幂等恢复
func TestOutboxFailureRaceAndIdempotentRecovery(t *testing.T) {
	root, db := testArcState(t)
	initClosureRepo(t, root)
	if err := executeTask(db, "create", "", "a pending task", "", 0); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "feature.go"), []byte("package feature\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := executeFile(db, root, "feature.go", ""); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	countCalls := 0
	failBuild := func(context.Context, string) error {
		countCalls++
		return errors.New("index backend unavailable")
	}
	if err := flushIndexOutboxWith(root, failBuild); err == nil {
		t.Fatal("failure incorrectly acknowledged events")
	}
	check := func(wantStatus string, wantAttempts int) {
		t.Helper()
		d, err := openActiveArcDB(root)
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		var status string
		var attempts int
		if err := d.QueryRow("SELECT status, attempts FROM arc_outbox LIMIT 1").Scan(&status, &attempts); err != nil {
			t.Fatal(err)
		}
		if status != wantStatus || attempts != wantAttempts {
			t.Fatalf("outbox status=%s attempts=%d want=%s/%d", status, attempts, wantStatus, wantAttempts)
		}
	}
	check("pending", 1)
	changeDuringIndex := func(context.Context, string) error {
		countCalls++
		return os.WriteFile(filepath.Join(root, "README.txt"), []byte("changed during indexing\n"), 0600)
	}
	if err := flushIndexOutboxWith(root, changeDuringIndex); err == nil {
		t.Fatal("source race incorrectly acknowledged events")
	}
	check("pending", 2)
	db2, err := openActiveArcDB(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := reconcileGit(db2, root, ""); err != nil {
		db2.Close()
		t.Fatal(err)
	}
	if err := db2.Close(); err != nil {
		t.Fatal(err)
	}
	okBuild := func(context.Context, string) error {
		countCalls++
		return nil
	}
	if err := flushIndexOutboxWith(root, okBuild); err != nil {
		t.Fatal(err)
	}
	check("delivered", 3)
	if err := flushIndexOutboxWith(root, okBuild); err != nil {
		t.Fatal(err)
	}
	if countCalls != 3 {
		t.Fatalf("idempotent no-pending flush ran backend: %d", countCalls)
	}
}

// testPolicyReceipt 构造强类型策略门禁测试收据状态对象
// 请求体
// taskID 关联任务ID
// verdict 收据总判定结果
// 响应体
// status 组装完成的策略任务状态
func testPolicyReceipt(taskID, verdict string) policyTaskStatus {
	// 1. 构造强类型收据载荷并序列化注入任务状态
	status := policyTaskStatus{ReceiptOK: true}
	status.Task.ID = taskID
	status.Tree.Head = "abc"
	status.Tree.Digest = "source-digest"
	status.Tree.Dirty = []string{}
	receipt := policyReceiptFixture{
		TaskID:    taskID,
		Verdict:   verdict,
		CreatedAt: "2026-11-09T00:00:00Z",
		Head:      "abc",
		Digest:    "source-digest",
		Tree:      "git-tree",
		Dirty:     []string{},
		Checks: []policyReceiptCheckFixture{
			{ID: "snapshot", Verdict: "PASS"},
			{ID: "cli_receipt", Verdict: "PASS"},
			{ID: "tests", Verdict: "PASS"},
		},
	}
	status.Task.Receipt, _ = json.Marshal(receipt)
	return status
}

// TestIndexBackendMustReturnRealHealthyDoctor 验证索引重建要求 doctor 返回健康且匹配仓库根的诊断响应
func TestIndexBackendMustReturnRealHealthyDoctor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scripted backend is POSIX-only")
	}
	bin := t.TempDir()
	script := "#!/bin/sh\ncase \"$1\" in\n  index) exit 0 ;;\n  doctor) printf '%s\\n' \"$ARC_INDEX_DOCTOR_JSON\"; exit 0 ;;\nesac\nexit 2\n"
	if err := os.WriteFile(filepath.Join(bin, "arc-idx"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	root := testRepoRoot(t)
	for _, tc := range []struct {
		name                                    string
		healthy, stale, tags, correctRoot, want bool
	}{
		{"healthy", true, false, true, true, true},
		{"unhealthy", false, false, true, true, false},
		{"stale", true, true, true, true, false},
		{"missing-tags", true, false, false, true, false},
		{"wrong-repository", true, false, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := root
			if !tc.correctRoot {
				repo = t.TempDir()
			}
			body, _ := json.Marshal(indexDoctorFixture{
				RepoRoot:    repo,
				Healthy:     tc.healthy,
				IndexStale:  tc.stale,
				TagsPresent: tc.tags,
			})
			t.Setenv("ARC_INDEX_DOCTOR_JSON", string(body))
			err := rebuildIndex(context.Background(), root)
			if (err == nil) != tc.want {
				t.Fatalf("index health acceptance=%v error=%v", tc.want, err)
			}
		})
	}
}

// TestInitialIndexWithEmptyOutboxAndGeneratedMetadata 验证空Outbox首次建索引以及忽略生成索引元数据目录
func TestInitialIndexWithEmptyOutboxAndGeneratedMetadata(t *testing.T) {
	root, db := testArcState(t)
	initClosureRepo(t, root)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := sourceSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".ai-code-index"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".ai-code-index", "tags.json"), []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := sourceSnapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("generated index metadata changed the source fingerprint")
	}
	calls := 0
	indexer := func(context.Context, string) error {
		calls++
		return nil
	}
	if err := flushIndexOutboxWith(root, indexer); err != nil {
		t.Fatal(err)
	}
	if err := flushIndexOutboxWith(root, indexer); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected one initial index build, got %d", calls)
	}
	if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("unrecorded edit\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := flushIndexOutboxWith(root, indexer); err == nil {
		t.Fatal("unobserved Git source drift was silently considered indexed")
	}
	if calls != 1 {
		t.Fatal("no-op flush unexpectedly rebuilt without recording source changes")
	}
}

// TestTaskProofRejectsAuthorizationAndStaleness 验证策略证明拒绝豁免授权、过期工作树与无独立测试通过记录
func TestTaskProofRejectsAuthorizationAndStaleness(t *testing.T) {
	const id = "policy-task"
	staleStatus := func() policyTaskStatus {
		s := testPolicyReceipt(id, "PASS")
		s.ReceiptOK = false
		return s
	}()
	inconsistentTree := func() policyTaskStatus {
		s := testPolicyReceipt(id, "PASS")
		s.Tree.Digest = "different"
		return s
	}()
	noIndependentProof := func() policyTaskStatus {
		s := testPolicyReceipt(id, "PASS")
		var receipt policyReceiptFixture
		_ = json.Unmarshal(s.Task.Receipt, &receipt)
		receipt.Checks = []policyReceiptCheckFixture{
			{ID: "snapshot", Verdict: "PASS"},
			{ID: "cli_receipt", Verdict: "PASS"},
			{ID: "tests", Verdict: "NOT_RUN"},
		}
		s.Task.Receipt, _ = json.Marshal(receipt)
		return s
	}()
	for _, tc := range []struct {
		name   string
		status policyTaskStatus
		valid  bool
	}{
		{"PASS", testPolicyReceipt(id, "PASS"), true},
		{"waiver-not-test", testPolicyReceipt(id, "AUTHORIZED"), false},
		{"failed-tests", testPolicyReceipt(id, "FAIL"), false},
		{"wrong-task", testPolicyReceipt("another-task", "PASS"), false},
		{"stale-tree", staleStatus, false},
		{"inconsistent-tree", inconsistentTree, false},
		{"no-independent-proof", noIndependentProof, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proof, err := requirePolicyProof(id, tc.status)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
			if tc.valid && proof.ReceiptSHA256 == "" {
				t.Fatal("PASS not fingerprinted")
			}
		})
	}
}

// TestPolicyReceiptRevocationBlocksCompletion 验证撤销或替换策略收据会阻断任务完成提交
func TestPolicyReceiptRevocationBlocksCompletion(t *testing.T) {
	root, db := testArcState(t)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".arc", "policy")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	const id = "policy-lock-test"
	status := testPolicyReceipt(id, "PASS")
	proof, err := requirePolicyProof(id, status)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.json")
	write := func(task string, receipt json.RawMessage) {
		t.Helper()
		body, err := json.Marshal(livePolicyStateFixture{
			Active: livePolicyActiveFixture{ID: task, Receipt: receipt},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write(id, status.Task.Receipt)
	if err := validateLivePolicyReceipt(root, proof); err != nil {
		t.Fatal(err)
	}
	write(id, json.RawMessage("null"))
	if err := validateLivePolicyReceipt(root, proof); err == nil {
		t.Fatal("revoked PASS accepted")
	}
	write("other-task", status.Task.Receipt)
	if err := validateLivePolicyReceipt(root, proof); err == nil {
		t.Fatal("changed task identity accepted")
	}
	changed := testPolicyReceipt(id, "PASS")
	var receipt policyReceiptFixture
	if err := json.Unmarshal(changed.Task.Receipt, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.CreatedAt = "2026-11-09T01:00:00Z"
	changed.Task.Receipt, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	write(id, changed.Task.Receipt)
	if err := validateLivePolicyReceipt(root, proof); err == nil {
		t.Fatal("replaced PASS accepted")
	}
}

// TestPolicyCompletionLockExcludesConcurrentTaskOperation 验证策略任务文件锁互斥拒绝重入并发操作
func TestPolicyCompletionLockExcludesConcurrentTaskOperation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows policy lock is fail-closed until compatible adapter")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".arc", "policy"), 0700); err != nil {
		t.Fatal(err)
	}
	err := withPolicyTaskLock(root, func() error {
		return withPolicyTaskLock(root, func() error {
			return nil
		})
	})
	if err == nil {
		t.Fatal("concurrent policy task lock accepted")
	}
	if err := withPolicyTaskLock(root, func() error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestTaskCompletionCLIUsesNativeLinkedProofWithoutPolicy 验证绑定原生证明的任务直接通过 arc-verify 完成而不调用策略插件
func TestTaskCompletionCLIUsesNativeLinkedProofWithoutPolicy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("scripted native verifier is POSIX-only")
	}
	root, db := testArcState(t)
	initClosureRepo(t, root)
	const id = "native-cli-final"
	if err := executeTask(db, "ensure", id, "native verified task", "", 0); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		to      string
		version int
	}{{"ready", 1}, {"running", 2}, {"verifying", 3}} {
		if err := executeTask(db, "transition", id, "", step.to, step.version); err != nil {
			t.Fatal(err)
		}
	}
	link := insertNativeCompletionLink(t, db, id, "O1")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := flushIndexOutboxWith(root, func(context.Context, string) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(testNativeTaskProof(t, root, link))
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\n[ \"$1\" = task-proof ] || exit 2\nprintf '%s\\n' \"$ARC_TEST_NATIVE_PROOF\"\n"
	if err := os.WriteFile(filepath.Join(bin, "arc-verify"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	policy := "#!/bin/sh\nprintf called > \"$ARC_TEST_POLICY_MARKER\"\nexit 7\n"
	if err := os.WriteFile(filepath.Join(bin, "arc-policy"), []byte(policy), 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "policy-called")
	t.Setenv("ARC_TEST_POLICY_MARKER", marker)
	t.Setenv("ARC_TEST_NATIVE_PROOF", string(body))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := resourceCommand("task", []string{"complete", "--repo", root, "--id", id, "--expected-version", "4"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("native completion invoked a policy plugin")
	}
	if err := resourceCommand("task", []string{"show", "--repo", root, "--id", id}); err != nil {
		t.Fatal(err)
	}
	if err := resourceCommand("task", []string{"complete", "--repo", root, "--id", id, "--expected-version", "4"}); err == nil {
		t.Fatal("same proof completed task twice")
	}
}

// TestCompletionRequiresConfirmedOutboxAndBoundRevision 验证任务完成提交要求已投递Outbox且工作树修订与证明严格一致
func TestCompletionRequiresConfirmedOutboxAndBoundRevision(t *testing.T) {
	root, db := testArcState(t)
	initClosureRepo(t, root)
	const id = "policy-task-final"
	if err := executeTask(db, "ensure", id, "final task (arc-code)", "", 0); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		to      string
		version int
	}{{"ready", 1}, {"running", 2}, {"verifying", 3}} {
		if err := executeTask(db, "transition", id, "", step.to, step.version); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "feature.go"), []byte("package feature\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := executeFile(db, root, "feature.go", id); err != nil {
		t.Fatal(err)
	}
	insertNativeCompletionLink(t, db, id, "O1")
	proof, err := loadNativeCompletionProof(root, id, func(_ context.Context, _ string, link nativeTaskLink) ([]byte, error) {
		return json.Marshal(testNativeTaskProof(t, root, link))
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := commitTaskCompletion(db, root, id, 4, proof); err == nil {
		t.Fatal("completion with pending events succeeded")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := flushIndexOutboxWith(root, func(context.Context, string) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	db, err = openActiveArcDB(root)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	changedProof := proof
	if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("new changes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := commitTaskCompletion(db, root, id, 4, changedProof); err == nil {
		t.Fatal("stale revision was accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("initial\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := commitTaskCompletion(db, root, id, 4, proof); err != nil {
		t.Fatal(err)
	}
	var state string
	var version int
	if err := db.QueryRow("SELECT status,version FROM arc_tasks WHERE id=?", id).Scan(&state, &version); err != nil {
		t.Fatal(err)
	}
	if state != "done" || version != 5 {
		t.Fatalf("completion state=%s version=%d", state, version)
	}
	var completed int
	if err := db.QueryRow("SELECT COUNT(*) FROM arc_events WHERE aggregate_id=? AND kind='task.completed'", id).Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if completed != 1 {
		t.Fatalf("completion events=%d want one", completed)
	}
	if err := commitTaskCompletion(db, root, id, 4, proof); err == nil {
		t.Fatal("CAS allowed duplicate completion")
	}
}
