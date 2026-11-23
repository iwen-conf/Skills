package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// gateProof 记录单个验证图门禁检查的原始输出与退出码
type gateProof struct {
	// Body 门禁命令标准输出文本
	Body string
	// Exit 门禁命令退出状态码
	Exit int
}

// gateContractPartition 描述强契约门禁中的有限域划分证明结构
type gateContractPartition struct {
	// Status 划分证明状态
	Status string `json:"status"`
	// CompleteInScope 是否在声明范围内完备
	CompleteInScope *bool `json:"complete_in_scope"`
}

// gateContractExecution 描述强契约门禁中的分支执行证明结构
type gateContractExecution struct {
	// Status 执行证明状态
	Status string `json:"status"`
}

// gateContextPayload 描述门禁证据中的图上下文标识
type gateContextPayload struct {
	// Graph 验证图或故事线ID
	Graph string `json:"graph"`
}

// gateProofEnvelope 描述验证引擎 gate 命令输出的强类型证据信封
type gateProofEnvelope struct {
	// Verdict 门禁顶层判定字符串
	Verdict *string `json:"verdict"`
	// ProofScope 证明作用范围标识
	ProofScope *string `json:"proof_scope"`
	// InputsUnchanged 输入是否未发生变动
	InputsUnchanged *bool `json:"inputs_unchanged"`
	// ContractProof 强契约划分证明指针
	ContractProof *gateContractPartition `json:"contract_proof"`
	// ContractExecution 强契约分支执行证明指针
	ContractExecution *gateContractExecution `json:"contract_execution"`
	// Context 门禁关联图上下文指针
	Context *gateContextPayload `json:"context"`
}

// readManifest 安全读取并校验统一状态库切换清单
// 请求体
// root Git仓库根目录路径
// 响应体
// manifest 激活态清单对象指针（不存在时返回nil）
// err 符号链接越界、JSON损坏或Schema不兼容错误
func readManifest(root string) (*arcManifest, error) {
	// 1. 拒绝符号链接形式的 .arc 目录防止目录劫持
	arcDir := filepath.Join(root, ".arc")
	if st, e := os.Lstat(arcDir); e == nil {
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("unsafe symlinked .arc directory")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	// 2. 检查切换清单文件是否存在且为普通文件
	st, e := os.Lstat(manifestPath(root))
	if errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, errors.New("arc cutover manifest is not a regular file")
	}
	// 3. 解析清单JSON并严格校验版本号、激活状态与数据库实体文件
	raw, err := os.ReadFile(manifestPath(root))
	if err != nil {
		return nil, err
	}
	var m arcManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid arc manifest: %w", err)
	}
	if m.Version != arcSchemaVersion || m.DB != arcDBName || !m.Active {
		return nil, errors.New("arc manifest has unsupported schema or inactive state (fail closed)")
	}
	if err := checkRegular(statePath(root)); err != nil {
		return nil, fmt.Errorf("active arc db: %w", err)
	}
	return &m, nil
}

// writeManifest 以原子临时文件重命名方式持久化状态库切换清单
// 请求体
// root Git仓库根目录路径
// m 待写入的切换清单结构
// 响应体
// err 文件创建、刷盘或原子替换错误
func writeManifest(root string, m arcManifest) error {
	// 1. 序列化清单为带缩进的JSON字节
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	// 2. 独占创建临时文件并强制刷盘
	path := manifestPath(root)
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_EXCL|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	_, e := f.Write(append(body, '\n'))
	if e == nil {
		e = f.Sync()
	}
	if closeErr := f.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		_ = os.Remove(tmp)
		return e
	}
	// 3. 原子重命名为正式清单路径
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// checkRegular 校验路径存在且为非符号链接的普通文件
// 请求体
// path 待校验文件路径
// 响应体
// err 文件不存在或非常规文件错误
func checkRegular(path string) error {
	// 1. 使用Lstat读取元数据并拒绝非常规文件
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("not an ordinary file: %s", path)
	}
	return nil
}

// copyFileExact 以独占创建与刷盘语义精确复制文件
// 请求体
// src 源文件路径
// dest 目标文件路径
// 响应体
// err 打开、拷贝或同步磁盘错误
func copyFileExact(src, dest string) error {
	// 1. 打开源文件并以独占模式创建0600权限目标文件
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	// 2. 流式复制内容并执行Sync落盘与异常清理
	_, e := io.Copy(out, in)
	if e == nil {
		e = out.Sync()
	}
	if closeErr := out.Close(); e == nil {
		e = closeErr
	}
	if e != nil {
		_ = os.Remove(dest)
	}
	return e
}

// noLiveSidecars 校验旧数据库不存在未静默合并的活跃WAL或锁边车文件
// 请求体
// src 源SQLite数据库文件路径
// 响应体
// err 检测到非空或非常规边车文件错误
func noLiveSidecars(src string) error {
	// 1. 遍历所有已知SQLite/Turso边车后缀检查是否存在残留写入数据
	for _, ext := range []string{"-wal", "-shm", "-journal", ".wal", ".shm", ".mvcc", "-mvcc", ".lock"} {
		file := src + ext
		st, err := os.Lstat(file)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !st.Mode().IsRegular() || st.Size() > 0 {
			return fmt.Errorf("legacy database has a live or unsafe sidecar %s; quiesce and checkpoint first", file)
		}
	}
	// 2. 全部后缀安全通过
	return nil
}

// runLegacyVerify 调用 arc-verify 子引擎执行数据库初始化或门禁比对命令
// 请求体
// root 仓库根目录路径
// dbfile 目标验证数据库路径
// command 子引擎命令名称
// extra 附加命令行参数
// 响应体
// out 命令标准输出字节
// code 子进程退出码
// err 子引擎不可用或启动失败错误
func runLegacyVerify(root, dbfile, command string, extra ...string) ([]byte, int, error) {
	// 1. 解析私有的 arc-verify 后端二进制路径
	bin, err := resolveBackend("arc-verify")
	if err != nil {
		return nil, 127, fmt.Errorf("arc-verify backend unavailable: %w", err)
	}
	// 2. 注入 --db 参数执行命令并提取真实退出码
	args := append([]string{command, "--db", dbfile}, extra...)
	cmd := exec.Command(bin, args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err == nil {
		return out, 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out, exit.ExitCode(), nil
	}
	return nil, 1, err
}

// validateGateProof 使用强类型 gateProofEnvelope 校验门禁JSON证据与退出码一致性
// 请求体
// body 门禁输出原始JSON字节
// exitCode 门禁进程退出码
// graph 目标验证图ID
// 响应体
// err 证据格式非法或判定与退出码矛盾错误
func validateGateProof(body []byte, exitCode int, graph string) error {
	// 1. 反序列化为强类型门禁证据结构并校验顶层verdict非空
	var proof gateProofEnvelope
	if err := json.Unmarshal(body, &proof); err != nil {
		return fmt.Errorf("graph %q gate did not return JSON evidence: %w", graph, err)
	}
	if proof.Verdict == nil {
		return fmt.Errorf("graph %q gate missing verdict", graph)
	}
	verdict := *proof.Verdict
	// 2. 按闭合Verdict枚举核验退出码、证明范围与输入未变更声明
	switch Verdict(verdict) {
	case VerdictPass:
		if exitCode != 0 {
			return fmt.Errorf("graph %q returned PASS with exit %d", graph, exitCode)
		}
		if proof.ProofScope == nil {
			return fmt.Errorf("graph %q PASS lacks proof scope", graph)
		}
		scope := *proof.ProofScope
		if scope != "storyline_completion_100pct" &&
			scope != "planned_rule_tasks" &&
			scope != "complete_business_model_obligations" &&
			scope != "legacy_business_model_obligations" &&
			scope != "declared_finite_contract_scope_and_verified_branch_execution" {
			return fmt.Errorf("graph %q PASS lacks proof scope", graph)
		}
		if scope == "declared_finite_contract_scope_and_verified_branch_execution" {
			if proof.ContractProof == nil || proof.ContractProof.Status != "PROVEN_IN_SCOPE" ||
				proof.ContractProof.CompleteInScope == nil || !*proof.ContractProof.CompleteInScope {
				return fmt.Errorf("graph %q strong PASS lacks finite contract partition proof", graph)
			}
			if proof.ContractExecution == nil || proof.ContractExecution.Status != string(VerdictPass) {
				return fmt.Errorf("graph %q strong PASS lacks per-branch execution proof", graph)
			}
		}
		if proof.InputsUnchanged == nil || !*proof.InputsUnchanged {
			return fmt.Errorf("graph %q PASS lacks unchanged-input attestation", graph)
		}
		if proof.Context == nil || proof.Context.Graph != graph {
			return fmt.Errorf("graph %q PASS has no matching graph context", graph)
		}
	case VerdictFail, VerdictBlocked, VerdictNotRun, VerdictFlaky:
		if exitCode == 0 {
			return fmt.Errorf("graph %q non-PASS verdict %q returned exit 0", graph, verdict)
		}
	default:
		return fmt.Errorf("graph %q unsupported verdict %q", graph, verdict)
	}
	return nil
}

// snapshotGateProofs 采集指定数据库内所有验证图的基线门禁证据快照
// 请求体
// root 仓库根目录路径
// dbfile 目标验证数据库路径
// 响应体
// proofs 按图ID索引的门禁输出快照映射
// err 查询图列表或门禁证据校验错误
func snapshotGateProofs(root, dbfile string) (map[string]gateProof, error) {
	// 1. 打开数据库读取全部已注册验证图ID
	db, err := openArcDB(dbfile)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query("SELECT id FROM graphs ORDER BY id")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("legacy graph schema missing: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			_ = rows.Close()
			_ = db.Close()
			return nil, scanErr
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	_ = rows.Close()
	_ = db.Close()
	if err != nil {
		return nil, err
	}
	// 2. 按排序后的图ID依次运行门禁并校验结构化证据
	sort.Strings(ids)
	out := make(map[string]gateProof, len(ids))
	for _, graph := range ids {
		body, exit, runErr := runLegacyVerify(root, dbfile, "gate", "--graph", graph)
		if runErr != nil {
			return nil, runErr
		}
		if valErr := validateGateProof(body, exit, graph); valErr != nil {
			return nil, fmt.Errorf("baseline gate: %w", valErr)
		}
		out[graph] = gateProof{Body: string(body), Exit: exit}
	}
	return out, nil
}

// verifyParity 比对迁移前基线、当前旧库与新库的全部验证图门禁输出一致性
// 请求体
// root 仓库根目录路径
// oldDB 旧验证数据库路径
// newDB 迁移后新统一数据库路径
// baseline 迁移前采集的门禁快照映射
// 响应体
// count 通过等价性校验的验证图数量
// err 任一验证图输出或退出码不一致错误
func verifyParity(root, oldDB, newDB string, baseline map[string]gateProof) (int, error) {
	// 1. 重新采集旧库快照确认迁移期间未被并发改写
	after, err := snapshotGateProofs(root, oldDB)
	if err != nil {
		return 0, err
	}
	if len(after) != len(baseline) {
		return 0, errors.New("graph set changed during migration")
	}
	// 2. 逐图核验新库门禁输出与基线字节级完全一致
	ids := make([]string, 0, len(baseline))
	for id := range baseline {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, graph := range ids {
		prior := baseline[graph]
		current, ok := after[graph]
		if !ok || current.Exit != prior.Exit || current.Body != prior.Body {
			return 0, fmt.Errorf("legacy graph %q proof changed during migration; arc.db NOT activated", graph)
		}
		newBody, newExit, runErr := runLegacyVerify(root, newDB, "gate", "--graph", graph)
		if runErr != nil {
			return 0, runErr
		}
		if valErr := validateGateProof(newBody, newExit, graph); valErr != nil {
			return 0, fmt.Errorf("migrated gate not trustworthy: %w", valErr)
		}
		if newExit != prior.Exit || string(newBody) != prior.Body {
			return 0, fmt.Errorf("graph %q parity differs (legacy exit %d, new exit %d); arc.db NOT activated", graph, prior.Exit, newExit)
		}
	}
	return len(ids), nil
}
