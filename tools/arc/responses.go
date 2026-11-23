package main

import "encoding/json"

// engineStatusResponse 描述单个子引擎二进制的可用状态与解析路径
type engineStatusResponse struct {
	// Available 子引擎可执行文件是否可用
	Available bool `json:"available"`
	// Path 解析到的子引擎二进制路径
	Path string `json:"path"`
}

// capabilitiesResponse 描述 arc capabilities --json 的完整能力声明结构
type capabilitiesResponse struct {
	// Name 统一CLI入口名称
	Name string `json:"name"`
	// ProtocolVersion 协议版本号
	ProtocolVersion int `json:"protocol_version"`
	// Version CLI当前语义版本
	Version string `json:"version"`
	// Mode 当前运行模式标识
	Mode string `json:"mode"`
	// CodeIndex 代码索引引擎是否可用
	CodeIndex bool `json:"code_index"`
	// BusinessVerify 业务验证引擎是否可用
	BusinessVerify bool `json:"business_verify"`
	// BusinessContractVersion 业务契约版本号
	BusinessContractVersion int `json:"business_contract_version"`
	// BusinessContractProofScope 业务契约证明范围
	BusinessContractProofScope string `json:"business_contract_proof_scope"`
	// BusinessCampaignVersion 业务战役版本号
	BusinessCampaignVersion int `json:"business_campaign_version"`
	// BusinessCampaignScope 业务战役覆盖范围
	BusinessCampaignScope string `json:"business_campaign_scope"`
	// VerificationViews 支持的验证视图列表
	VerificationViews []string `json:"verification_views"`
	// ArcDBResources 本地状态资源接口是否已实现
	ArcDBResources bool `json:"arc_db_resources"`
	// UnifiedDatabase 统一数据库是否已激活
	UnifiedDatabase bool `json:"unified_database"`
	// ArcDBActive 当前仓库arc.db是否处于激活态
	ArcDBActive bool `json:"arc_db_active"`
	// ArcDBSchemaVersion 本地状态库Schema版本
	ArcDBSchemaVersion int `json:"arc_db_schema_version"`
	// ResourceCommands 已支持的资源子命令列表
	ResourceCommands []string `json:"resource_commands"`
	// OutboxConsumer 索引事件队列消费者标识
	OutboxConsumer string `json:"outbox_consumer"`
	// CompletionProof 任务完成证明机制标识
	CompletionProof string `json:"completion_proof"`
	// PolicyStorage 策略存储模式说明
	PolicyStorage string `json:"policy_storage"`
	// Engines 下属子引擎可用性映射
	Engines map[string]engineStatusResponse `json:"engines"`
}

// dbStatusResponse 描述 arc db status 的状态查询响应
type dbStatusResponse struct {
	// Path 数据库文件路径
	Path string `json:"path"`
	// Active 数据库是否已激活
	Active bool `json:"active"`
	// SchemaVersion 数据库Schema版本
	SchemaVersion int `json:"schema_version"`
	// LegacyDB 迁移来源旧库路径
	LegacyDB string `json:"legacy_db,omitempty"`
	// ParityGraphs 迁移一致性校验通过的图数量
	ParityGraphs *int `json:"parity_graphs,omitempty"`
	// PendingEvents 待处理索引队列事件数
	PendingEvents *int `json:"pending_events,omitempty"`
	// GraphCount 当前存储验证图数量
	GraphCount *int `json:"graph_count,omitempty"`
	// PolicyStorage 策略状态存储模式
	PolicyStorage string `json:"policy_storage,omitempty"`
	// PolicyStateVersion 策略状态版本号
	PolicyStateVersion *int64 `json:"policy_state_version,omitempty"`
}

// dbActivateResponse 描述 arc db init/migrate 激活成功后的响应
type dbActivateResponse struct {
	// Database 激活后的数据库文件路径
	Database string `json:"database"`
	// Active 激活状态标志
	Active bool `json:"active"`
	// MigratedGraphs 迁移并校验一致的验证图数量
	MigratedGraphs int `json:"migrated_graphs"`
}

// taskMutationResponse 描述任务创建、状态流转与完成提交后的响应
type taskMutationResponse struct {
	// ID 任务唯一ID
	ID string `json:"id"`
	// Status 任务当前状态
	Status string `json:"status"`
	// Version 任务乐观锁版本号
	Version int `json:"version"`
	// Duplicate 幂等导入时是否为已存在记录
	Duplicate *bool `json:"duplicate,omitempty"`
	// ReceiptSHA256 完成提交绑定的验证收据摘要
	ReceiptSHA256 string `json:"receipt_sha256,omitempty"`
}

// taskCompletionSummary 描述已完成任务关联的收据与源码快照比对摘要
type taskCompletionSummary struct {
	// ReceiptSHA256 完成收据SHA256摘要
	ReceiptSHA256 string `json:"receipt_sha256"`
	// Head 完成时绑定的Git提交哈希
	Head string `json:"head"`
	// SourceRevisionCurrent 当前工作区源码是否仍与完成快照一致
	SourceRevisionCurrent bool `json:"source_revision_current"`
	// IndependentProofRechecked 本次查询是否重新执行了外部独立验证
	IndependentProofRechecked bool `json:"independent_proof_rechecked"`
}

// taskShowResponse 描述 arc task show 的详情响应
type taskShowResponse struct {
	// ID 任务唯一ID
	ID string `json:"id"`
	// Title 任务标题
	Title string `json:"title"`
	// Status 任务当前状态
	Status string `json:"status"`
	// Version 任务当前版本号
	Version int `json:"version"`
	// Updated 最近更新UTC时间戳
	Updated string `json:"updated"`
	// Files 关联文件路径列表
	Files []string `json:"files,omitempty"`
	// Completion 已完成任务的收据校验摘要
	Completion *taskCompletionSummary `json:"completion,omitempty"`
}

// fileUpdateResponse 描述 arc file update 的响应结构
type fileUpdateResponse struct {
	// Path 规范化后的仓库相对路径
	Path string `json:"path"`
	// SHA256 文件内容SHA256摘要
	SHA256 string `json:"sha256"`
	// Task 关联任务ID
	Task string `json:"task"`
	// Index 索引队列同步状态
	Index string `json:"index"`
}

// fileShowResponse 描述 arc file show 的文件观测记录响应
type fileShowResponse struct {
	// Path 规范化后的仓库相对路径
	Path string `json:"path"`
	// SHA256 记录的文件内容SHA256摘要
	SHA256 string `json:"sha256"`
	// Bytes 文件字节大小
	Bytes int64 `json:"bytes"`
	// Version 文件观测版本号
	Version int `json:"version"`
	// ChangedAt 最近观测变更UTC时间
	ChangedAt string `json:"changed_at"`
}

// changeCollectResponse 描述 arc change collect 与 arc update 的采集响应
type changeCollectResponse struct {
	// ChangeID 变更集ID指针（无变更时序列化为null）
	ChangeID *string `json:"change_id"`
	// ChangedFiles 纳入变更集的文件数量
	ChangedFiles int `json:"changed_files"`
	// Duplicate 是否命中已有幂等变更集指纹
	Duplicate *bool `json:"duplicate,omitempty"`
	// Index 索引同步状态标识
	Index string `json:"index"`
	// Status 变更集落库状态
	Status string `json:"status,omitempty"`
}

// changeFileItem 描述变更集内的单条文件变更明细
type changeFileItem struct {
	// Path 仓库相对路径
	Path string `json:"path"`
	// SHA256 文件内容摘要
	SHA256 string `json:"sha256"`
	// Operation 变更操作类型
	Operation string `json:"operation"`
}

// changeShowResponse 描述 arc change show 的变更集详情响应
type changeShowResponse struct {
	// ID 变更集唯一ID
	ID string `json:"id"`
	// Task 关联任务ID
	Task string `json:"task"`
	// Status 变更集状态
	Status string `json:"status"`
	// Revision 采集时绑定的Git提交哈希
	Revision string `json:"revision"`
	// Created 创建UTC时间戳
	Created string `json:"created"`
	// Files 变更文件明细列表
	Files []changeFileItem `json:"files"`
}

// taskLinkVerifyResponse 描述 arc task link-verify 的绑定结果响应
type taskLinkVerifyResponse struct {
	// ArcTaskID 本地任务ID
	ArcTaskID string `json:"arc_task_id"`
	// ArcTaskStatus 本地任务当前状态
	ArcTaskStatus string `json:"arc_task_status"`
	// Graph 验证图或故事线ID
	Graph string `json:"graph"`
	// ObligationID 验证义务或路由ID
	ObligationID string `json:"obligation_id"`
	// VerifyTaskID 验证侧完整任务标识
	VerifyTaskID string `json:"verify_task_id"`
	// VerifyTaskStatus 验证侧当前存储状态
	VerifyTaskStatus string `json:"verify_task_status"`
}

// verifyFactSummary 描述存储层最近一次步骤执行事实摘要
type verifyFactSummary struct {
	// Seq 步骤序号
	Seq int `json:"seq"`
	// Node 步骤节点或分支标识
	Node string `json:"node"`
	// ObservedState 观测到的实际状态
	ObservedState string `json:"observed_state"`
	// StoredVerdict 存储的步骤判定结果
	StoredVerdict string `json:"stored_verdict"`
	// Reason 差异或失败原因
	Reason string `json:"reason"`
	// EvidenceStatus 证据重验状态标识
	EvidenceStatus string `json:"evidence_status"`
	// FirstDivergence 是否为首个分叉失败步骤
	FirstDivergence bool `json:"first_divergence"`
	// EvidenceSHA256 步骤证据哈希摘要
	EvidenceSHA256 string `json:"evidence_sha256"`
}

// obligationStatusItem 描述单条关联验证义务的未重验存储摘要
type obligationStatusItem struct {
	// Graph 验证图或故事线ID
	Graph string `json:"graph"`
	// ObligationID 验证义务或路由ID
	ObligationID string `json:"obligation_id"`
	// VerifyTaskID 验证任务完整ID
	VerifyTaskID string `json:"verify_task_id"`
	// LinkedAt 绑定建立时间
	LinkedAt string `json:"linked_at"`
	// EvidenceStatus 证据重验状态（固定为NOT_REVALIDATED）
	EvidenceStatus string `json:"evidence_status"`
	// Kind 义务类型标识
	Kind string `json:"kind,omitempty"`
	// Expected 期望结局或目标状态
	Expected string `json:"expected,omitempty"`
	// LatestRun 最近一次运行ID
	LatestRun string `json:"latest_run,omitempty"`
	// LatestRunStoredStatus 最近一次运行的存储判定
	LatestRunStoredStatus string `json:"latest_run_stored_status,omitempty"`
	// Environment 运行命名空间或环境名
	Environment string `json:"environment,omitempty"`
	// CommitSHA 运行时记录的提交哈希
	CommitSHA string `json:"commit_sha,omitempty"`
	// Fact 最近或首个异常步骤摘要
	Fact *verifyFactSummary `json:"fact,omitempty"`
	// StoredStatus 义务当前存储状态
	StoredStatus string `json:"stored_status"`
}

// verifyProvenanceInfo 描述 verify-status 的来源与不可直接授权完成声明
type verifyProvenanceInfo struct {
	// Kind 摘要类别标识
	Kind string `json:"kind"`
	// LatestRunOrder 最近运行排序规则
	LatestRunOrder string `json:"latest_run_order"`
	// EvidenceRevalidated 证据是否经过重验
	EvidenceRevalidated bool `json:"evidence_revalidated"`
	// Reason 未重验说明文本
	Reason string `json:"reason"`
}

// taskVerifyStatusResponse 描述 arc task verify-status 的完整汇总响应
type taskVerifyStatusResponse struct {
	// ArcTask 任务基础信息
	ArcTask taskShowResponse `json:"arc_task"`
	// Linked 关联验证义务总数
	Linked int `json:"linked"`
	// StoredStatusCounts 各存储状态计数映射
	StoredStatusCounts map[string]int `json:"stored_status_counts"`
	// EvidenceStatus 顶层证据重验状态
	EvidenceStatus string `json:"evidence_status"`
	// ProofScope 证明作用范围
	ProofScope string `json:"proof_scope"`
	// Provenance 摘要来源与安全边界声明
	Provenance verifyProvenanceInfo `json:"provenance"`
	// Obligations 关联验证义务明细列表
	Obligations []obligationStatusItem `json:"obligations"`
}

// eventRecordResponse 描述单条持久化领域事件记录
type eventRecordResponse struct {
	// ID 事件唯一ID
	ID string `json:"id"`
	// Aggregate 聚合根类别
	Aggregate string `json:"resource"`
	// AggregateID 聚合根实例ID
	AggregateID string `json:"resource_id"`
	// Kind 事件类型名
	Kind string `json:"kind"`
	// Payload 原始事件JSON载荷
	Payload json.RawMessage `json:"payload"`
	// Created 事件发生UTC时间
	Created string `json:"created_at"`
}

// eventListResponse 描述 arc event list 的事件列表响应
type eventListResponse struct {
	// Events 事件记录切片
	Events []eventRecordResponse `json:"events"`
	// Count 返回事件条数
	Count int `json:"count"`
}

// reconcileResponse 描述 arc sync reconcile 的Git对账响应
type reconcileResponse struct {
	// Source 变更同步来源标识
	Source string `json:"source"`
	// PreviousHead 上次对账记录的HEAD哈希
	PreviousHead string `json:"previous_head"`
	// CurrentHead 当前仓库HEAD哈希
	CurrentHead string `json:"current_head"`
	// CommitsReconciled 是否存在跨提交增量回放
	CommitsReconciled bool `json:"commits_reconciled"`
	// ChangedFiles 本次纳入对账的变更文件数
	ChangedFiles int `json:"changed_files"`
	// ChangeID 生成或复用的变更集ID
	ChangeID string `json:"change_id"`
	// Duplicate 是否命中已有变更集指纹
	Duplicate bool `json:"duplicate"`
	// Index 索引队列同步状态
	Index string `json:"index"`
}

// indexFlushResponse 描述 arc index flush 的索引刷新响应
type indexFlushResponse struct {
	// IndexedEvents 本次确认投递的事件数量
	IndexedEvents int64 `json:"indexed_events"`
	// Pending 剩余待处理事件数（仅无待办时输出）
	Pending *int `json:"pending,omitempty"`
	// Index 索引健康状态标识
	Index string `json:"index"`
	// Backend 执行刷新的索引后端标识
	Backend string `json:"backend,omitempty"`
	// WorktreeSnapshot 已确认的工作区源码快照哈希
	WorktreeSnapshot string `json:"worktree_snapshot,omitempty"`
}

// policyVersionResponse 描述策略状态版本查询与更新响应
type policyVersionResponse struct {
	// Mode 策略存储模式
	Mode string `json:"mode"`
	// Version 当前乐观锁版本号
	Version int64 `json:"version"`
}

// policyMigrateResponse 描述策略存储迁移完成响应
type policyMigrateResponse struct {
	// Storage 激活的存储引擎名
	Storage string `json:"storage"`
	// Version 初始或当前状态版本
	Version int `json:"version,omitempty"`
	// AlreadyMigrated 是否在本次调用前已完成迁移
	AlreadyMigrated bool `json:"already_migrated,omitempty"`
	// Backup 原始JSON状态备份路径
	Backup string `json:"backup,omitempty"`
	// ImportedReceipts 导入的历史收据条数
	ImportedReceipts int `json:"imported_receipts,omitempty"`
	// LegacyGuard 旧文件防回写墓碑类型
	LegacyGuard string `json:"legacy_guard"`
	// Active 迁移后存储是否处于激活态
	Active bool `json:"active"`
}
