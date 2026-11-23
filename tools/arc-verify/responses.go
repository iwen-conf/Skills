package main

// storyRefBase 供故事线相关CLI响应扁平嵌入复用的标识基座（践行Go组合优于继承）
type storyRefBase struct {
	// StoryID 故事线唯一标识
	StoryID string `json:"story_id"`
}

// initResponse 描述 arc verify init 的账本初始化响应
type initResponse struct {
	// Initialized 账本是否初始化成功
	Initialized bool `json:"initialized"`
	// DB 账本数据库文件路径
	DB string `json:"db"`
	// Engine 验证引擎版本标识
	Engine string `json:"engine"`
}

// importResponse 描述 arc verify import 的故事线导入响应
type importResponse struct {
	storyRefBase
	// Imported 是否成功写入故事线定义
	Imported bool `json:"imported"`
	// SHA256 故事线原文SHA256摘要
	SHA256 string `json:"sha256"`
}

// planShardsFileResponse 描述 arc verify plan-shards --out 落盘后的响应
type planShardsFileResponse struct {
	// Written 输出文件路径
	Written string `json:"written"`
	// Shards 规划出的并发盲测分片数量
	Shards int `json:"shards"`
}

// checkpointListResponse 描述检查点构建与列表查询的响应结构
type checkpointListResponse struct {
	storyRefBase
	// Checkpoints 检查点元数据列表
	Checkpoints []CheckpointRecord `json:"checkpoints"`
}

// checkpointCloneResponse 描述 arc verify checkpoint clone 的分片克隆响应
type checkpointCloneResponse struct {
	storyRefBase
	// SaveID 源检查点存档ID
	SaveID string `json:"save_id"`
	// RuntimeNS 隔离运行命名空间
	RuntimeNS string `json:"runtime_ns"`
	// ClonePath 克隆后的隔离状态路径
	ClonePath string `json:"clone_path"`
}

// checkpointFileMetadata 描述写入检查点目录 checkpoint.json 的结构化元数据
type checkpointFileMetadata struct {
	storyRefBase
	// SaveID 检查点存档ID
	SaveID string `json:"save_id"`
	// Parent 父级检查点ID
	Parent string `json:"parent"`
	// Engine 检查点存储引擎类型
	Engine string `json:"engine"`
	// AssertFlags 检查点前置状态断言映射
	AssertFlags FactMap `json:"assert_flags,omitempty"`
	// AssertCmd 检查点前置校验命令
	AssertCmd string `json:"assert_cmd,omitempty"`
	// SetupSteps 检查点初始化步骤列表
	SetupSteps []SetupStep `json:"setup_steps,omitempty"`
}

// gateContextInfo 描述 arc verify gate 输出中的故事线上下文标识
type gateContextInfo struct {
	storyRefBase
	// Graph 兼容旧版验证图字段的故事线ID
	Graph string `json:"graph"`
	// StorySHA256 故事线定义SHA256摘要
	StorySHA256 string `json:"story_sha256"`
}

// gateResponse 描述 arc verify gate 的统一门禁判定响应
type gateResponse struct {
	// Verdict 门禁总判定结果
	Verdict string `json:"verdict"`
	// ProofScope 证明覆盖范围标识
	ProofScope string `json:"proof_scope"`
	// InputsUnchanged 输入快照是否未发生漂移
	InputsUnchanged bool `json:"inputs_unchanged"`
	// Context 门禁关联的故事线上下文
	Context gateContextInfo `json:"context"`
	// Completion 四维100%完成度报告
	Completion *CompletionReport `json:"completion"`
}

// taskProofContextInfo 描述原生任务完成证明绑定的源码与配置指纹上下文
type taskProofContextInfo struct {
	// Graph 故事线或验证图ID
	Graph string `json:"graph"`
	// Environment 隔离运行命名空间
	Environment string `json:"environment"`
	// Target 核心受控业务实体标识
	Target string `json:"target"`
	// SourceCommit 执行时的Git提交哈希
	SourceCommit string `json:"source_commit"`
	// SourceSHA256 源码关联指纹摘要
	SourceSHA256 string `json:"source_sha256"`
	// ConfigSHA256 故事线配置指纹摘要
	ConfigSHA256 string `json:"config_sha256"`
	// GraphSHA256 验证图定义指纹摘要
	GraphSHA256 string `json:"graph_sha256"`
	// SourceRoot 仓库根目录真实路径
	SourceRoot string `json:"source_root"`
	// ConfigPath 故事线声明文件路径
	ConfigPath string `json:"config_path"`
}

// taskProofResponse 描述 arc verify task-proof 的结构化证明响应
type taskProofResponse struct {
	// Version 证明协议版本号
	Version int `json:"version"`
	// Graph 故事线或验证图ID
	Graph string `json:"graph"`
	// Task 验证任务完整标识
	Task string `json:"task"`
	// ObligationID 路由或验证义务ID
	ObligationID string `json:"obligation_id"`
	// Run 验证运行批次ID
	Run string `json:"run,omitempty"`
	// Round 起始检查点存档标识
	Round string `json:"round,omitempty"`
	// Verdict 证明判定结果
	Verdict string `json:"verdict"`
	// Reason 未通过或未运行原因说明
	Reason string `json:"reason,omitempty"`
	// ProofScope 证明作用范围标识
	ProofScope string `json:"proof_scope,omitempty"`
	// InputsUnchanged 输入配置是否未变更
	InputsUnchanged bool `json:"inputs_unchanged,omitempty"`
	// EvidenceStatus 证据核验状态
	EvidenceStatus string `json:"evidence_status,omitempty"`
	// Context 绑定的环境与源码指纹上下文
	Context *taskProofContextInfo `json:"context,omitempty"`
	// RAWSHA256 原始路由收据与六维步骤轨迹哈希列表
	RAWSHA256 []string `json:"raw_sha256,omitempty"`
	// ReceiptSHA256 路由级收据SHA256摘要
	ReceiptSHA256 string `json:"receipt_sha256,omitempty"`
}

// snapshotExportResponse 描述 arc verify snapshot 导出完成响应
type snapshotExportResponse struct {
	storyRefBase
	// Exported 静态工作台是否导出成功
	Exported bool `json:"exported"`
	// Out 导出目录路径
	Out string `json:"out"`
}

// snapshotDocument 描述导出到 snapshot.json 的完整只读工作台载荷
type snapshotDocument struct {
	// SchemaVersion 快照Schema版本号
	SchemaVersion int `json:"schema_version"`
	// Kind 快照文档类型标识
	Kind string `json:"kind"`
	// Storyline 故事线完整定义
	Storyline *Storyline `json:"storyline"`
	// LatestRunID 最近一次运行批次ID
	LatestRunID string `json:"latest_run_id"`
	// Checkpoints 已记录检查点列表
	Checkpoints []CheckpointRecord `json:"checkpoints"`
	// RouteResults 历史路由执行结果列表
	RouteResults []RouteResultRecord `json:"route_results"`
	// StepTraces 最新批次六维步骤轨迹列表
	StepTraces []StepTraceRecord `json:"step_traces"`
	// Completion 完成度评估报告
	Completion *CompletionReport `json:"completion"`
	// ExportedAt 快照导出UTC时间
	ExportedAt string `json:"exported_at"`
}
