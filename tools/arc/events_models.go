package main

import "encoding/json"

// policyCheckItem 描述单条策略或原生完成检查项的强类型结构
type policyCheckItem struct {
	// ID 检查项类别标识
	ID string `json:"id"`
	// Verdict 检查项判定结果
	Verdict string `json:"verdict"`
	// Graph 关联验证图ID
	Graph string `json:"graph,omitempty"`
	// ObligationID 关联验证义务ID
	ObligationID string `json:"obligation_id,omitempty"`
	// Task 验证任务完整ID
	Task string `json:"task,omitempty"`
	// Run 运行批次ID
	Run string `json:"run,omitempty"`
	// Round 起始存档标识
	Round string `json:"round,omitempty"`
	// ProofSHA256 规范化证明SHA256摘要
	ProofSHA256 string `json:"proof_sha256,omitempty"`
	// Proof 原始证明JSON字节
	Proof json.RawMessage `json:"proof,omitempty"`
}

// taskCreatedEventPayload 描述 task.created 事件的强类型载荷
type taskCreatedEventPayload struct {
	// Title 任务标题
	Title string `json:"title"`
}

// taskImportedEventPayload 描述 task.imported 事件的强类型载荷
type taskImportedEventPayload struct {
	// Title 任务标题
	Title string `json:"title"`
	// Source 导入来源标识
	Source string `json:"source"`
}

// taskTransitionedEventPayload 描述 task.transitioned 事件的强类型载荷
type taskTransitionedEventPayload struct {
	// From 变更前状态
	From string `json:"from"`
	// To 变更后状态
	To string `json:"to"`
	// FromVersion 变更前版本号
	FromVersion int `json:"from_version"`
}

// taskObligationLinkedEventPayload 描述 task.obligation_linked 事件的强类型载荷
type taskObligationLinkedEventPayload struct {
	// Graph 验证图或故事线ID
	Graph string `json:"graph"`
	// Obligation 验证义务或路由ID
	Obligation string `json:"obligation"`
	// VerifyTask 验证任务完整ID
	VerifyTask string `json:"verify_task"`
}

// taskCompletedEventPayload 描述 task.completed 不可变收据事件的强类型载荷
type taskCompletedEventPayload struct {
	// TaskID 任务唯一ID
	TaskID string `json:"task_id"`
	// FromVersion 完成前版本号
	FromVersion int `json:"from_version"`
	// ReceiptSHA256 完成收据SHA256摘要
	ReceiptSHA256 string `json:"receipt_sha256"`
	// Head 完成绑定的Git提交哈希
	Head string `json:"head"`
	// Tree 完成绑定的Git树哈希
	Tree string `json:"tree"`
	// Digest 完成绑定的工作树摘要
	Digest string `json:"digest"`
	// VerifiedAt 验证完成UTC时间
	VerifiedAt string `json:"verified_at"`
	// SourceSnapshot 源码快照SHA256摘要
	SourceSnapshot string `json:"source_snapshot"`
	// Checks 检查项明细列表
	Checks []policyCheckItem `json:"checks"`
	// ProofKind 证明类型标识
	ProofKind string `json:"proof_kind"`
	// LinksSHA256 关联验证义务列表哈希
	LinksSHA256 string `json:"links_sha256"`
	// Links 关联验证义务快照列表
	Links []nativeTaskLink `json:"links"`
}

// nativeCompletionDigestPayload 描述计算原生完成收据指纹的强类型输入载荷
type nativeCompletionDigestPayload struct {
	// TaskID 任务唯一ID
	TaskID string `json:"task_id"`
	// SourceSnapshot 源码快照摘要
	SourceSnapshot string `json:"source_snapshot"`
	// LinksSHA256 关联验证义务摘要
	LinksSHA256 string `json:"links_sha256"`
	// Checks 原生检查项列表
	Checks []policyCheckItem `json:"checks"`
}

// fileObservedEventPayload 描述 file.<op> 事件的强类型载荷
type fileObservedEventPayload struct {
	// SHA256 文件内容摘要
	SHA256 string `json:"sha256"`
	// Version 文件新版本号
	Version int `json:"version"`
}

// observedFileEntry 描述纳入变更集指纹计算的单个文件状态
type observedFileEntry struct {
	// Path 仓库相对路径
	Path string
	// Operation 变更操作类型
	Operation string
	// SHA 文件内容摘要
	SHA string
}

// changeFingerprintInput 描述计算幂等 ChangeSet 指纹的强类型输入结构
type changeFingerprintInput struct {
	// Task 关联任务ID
	Task string `json:"task"`
	// Root 仓库根目录路径
	Root string `json:"root"`
	// Revision 当前Git提交哈希
	Revision string `json:"revision"`
	// Files 观测到的变更文件列表
	Files []observedFileEntry `json:"files"`
}

// changeCollectedEventPayload 描述 change.collected 事件的强类型载荷
type changeCollectedEventPayload struct {
	// Task 关联任务ID
	Task string `json:"task"`
	// Revision 绑定的Git提交哈希
	Revision string `json:"revision"`
	// Files 变更文件数量
	Files int `json:"files"`
	// Reconciled 是否来自 sync reconcile 补录
	Reconciled bool `json:"reconciled,omitempty"`
}

// policyUpdatedEventPayload 描述 policy.state.updated 事件的强类型载荷
type policyUpdatedEventPayload struct {
	// Version 更新后的策略状态版本号
	Version int64 `json:"version"`
	// SHA256 策略状态规范化JSON摘要
	SHA256 string `json:"sha256"`
}

// policyMigratedEventPayload 描述 policy.state.migrated 事件的强类型载荷
type policyMigratedEventPayload struct {
	// StateSHA256 迁移前原始JSON状态摘要
	StateSHA256 string `json:"state_sha256"`
	// Receipts 导入收据条数
	Receipts int `json:"receipts"`
	// Version 初始状态版本号
	Version int `json:"version"`
}

// storylineObligationSummary 描述从 story_* 六维账本读取的强类型路由状态摘要
type storylineObligationSummary struct {
	// Kind 义务类型标识
	Kind string
	// Expected 预期解锁结局
	Expected string
	// LatestRun 最近一次运行批次ID
	LatestRun string
	// LatestRunStoredStatus 最近一次运行存储状态
	LatestRunStoredStatus string
	// Environment 隔离运行命名空间
	Environment string
	// Fact 最近或首个异常步骤摘要指针
	Fact *verifyFactSummary
}
