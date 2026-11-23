package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Storyline 描述底特律式六维分支验证契约（前置条件、预期输入、状态变迁、执行轨迹、预期行为、结局探针）
type Storyline struct {
	// StoryID 故事线唯一标识
	StoryID string `yaml:"story_id" json:"story_id"`
	// FocalEntity 核心受控业务实体（如 users.id）
	FocalEntity string `yaml:"focal_entity" json:"focal_entity"`
	// FrozenReadonly 严禁被并发路由修改的冻结只读路径模式
	FrozenReadonly []string `yaml:"frozen_readonly,omitempty" json:"frozen_readonly,omitempty"`
	// BlindMask 盲测模式下向子Agent隐藏的敏感路径掩码
	BlindMask []string `yaml:"blind_mask,omitempty" json:"blind_mask,omitempty"`
	// WorldFlags 跨表与异步耦合世界状态标志映射
	WorldFlags map[string]string `yaml:"world_flags" json:"world_flags"`
	// Checkpoints 拓扑有序的可复用检查点存档列表
	Checkpoints []Checkpoint `yaml:"checkpoints" json:"checkpoints"`
	// Routes 并发或串行执行的分支路由集合
	Routes []Route `yaml:"routes" json:"routes"`
	// RequiredEndings 必须100%解锁的业务结局集合
	RequiredEndings []string `yaml:"required_endings" json:"required_endings"`
	// SourcePath 加载器注入的源文件路径元数据
	SourcePath string `yaml:"-" json:"source_path,omitempty"`
	// SHA256 加载器计算的原文SHA256摘要
	SHA256 string `yaml:"-" json:"sha256,omitempty"`
}

// Checkpoint 描述可供多条路由毫秒级克隆复用的状态检查点存档
type Checkpoint struct {
	// SaveID 检查点存档唯一ID
	SaveID string `yaml:"save_id" json:"save_id"`
	// Parent 父级检查点ID（根节点为 root）
	Parent string `yaml:"parent" json:"parent"`
	// Engine 存储引擎类型（dir | sqlite | postgres）
	Engine string `yaml:"engine,omitempty" json:"engine,omitempty"`
	// SetupSteps 构建检查点的初始化步骤序列
	SetupSteps []SetupStep `yaml:"setup_steps" json:"setup_steps"`
	// AssertFlags 检查点构建完成后的状态断言映射
	AssertFlags FactMap `yaml:"assert_flags,omitempty" json:"assert_flags,omitempty"`
	// SeedCmd 数据灌入Shell命令
	SeedCmd string `yaml:"seed_cmd,omitempty" json:"seed_cmd,omitempty"`
	// AssertCmd 检查点前置断言校验命令
	AssertCmd string `yaml:"assert_cmd,omitempty" json:"assert_cmd,omitempty"`
}

// SetupStep 描述检查点构建过程中的单步初始化调用
type SetupStep struct {
	// Call 初始化动作名称
	Call string `yaml:"call" json:"call"`
	// With 初始化动作强类型入参映射
	With FactMap `yaml:"with,omitempty" json:"with,omitempty"`
	// Cmd 实际执行的初始化命令
	Cmd string `yaml:"cmd,omitempty" json:"cmd,omitempty"`
}

// Route 描述从指定检查点分叉出的独立业务路径与结局目标
type Route struct {
	// RouteID 路由唯一标识
	RouteID string `yaml:"route_id" json:"route_id"`
	// LoadSave 起始加载的检查点存档ID
	LoadSave string `yaml:"load_save" json:"load_save"`
	// RunnerType 执行器层级类型
	RunnerType string `yaml:"runner_type" json:"runner_type"`
	// AgentHint 并发调度Agent倾向提示
	AgentHint string `yaml:"agent_hint,omitempty" json:"agent_hint,omitempty"`
	// WriteScope 允许写入的互不重叠文件范围列表
	WriteScope []string `yaml:"write_scope,omitempty" json:"write_scope,omitempty"`
	// RuntimeNS 隔离运行命名空间
	RuntimeNS string `yaml:"runtime_ns,omitempty" json:"runtime_ns,omitempty"`
	// VerifyCmd 路由级总体验证命令
	VerifyCmd string `yaml:"verify_cmd,omitempty" json:"verify_cmd,omitempty"`
	// Sequence 按顺序推进的六维验证步骤列表
	Sequence []RouteStep `yaml:"sequence" json:"sequence"`
	// UnlocksEnding 该路由通过后解锁的业务结局标识
	UnlocksEnding string `yaml:"unlocks_ending" json:"unlocks_ending"`
}

// RouteStep 描述路由序列中单步六维预期契约声明
type RouteStep struct {
	// Step 步骤序号（从1递增）
	Step int `yaml:"step" json:"step"`
	// Choice 业务分支动作标识
	Choice string `yaml:"choice" json:"choice"`
	// ExpectedBehavior 预期行为分类枚举
	ExpectedBehavior string `yaml:"expected_behavior,omitempty" json:"expected_behavior,omitempty"`
	// Preconditions 前置环境与业务条件强类型映射
	Preconditions FactMap `yaml:"preconditions,omitempty" json:"preconditions,omitempty"`
	// PrecheckCmd 前置条件探测命令（失败判BLOCKED）
	PrecheckCmd string `yaml:"precheck_cmd,omitempty" json:"precheck_cmd,omitempty"`
	// ExpectedInput 显式声明的预期输入映射
	ExpectedInput FactMap `yaml:"expected_input,omitempty" json:"expected_input,omitempty"`
	// With 兼容简写形式的预期输入映射
	With FactMap `yaml:"with,omitempty" json:"with,omitempty"`
	// BeforeState 步骤执行前状态基准映射
	BeforeState FactMap `yaml:"before_state,omitempty" json:"before_state,omitempty"`
	// Cmd 步骤主执行命令
	Cmd string `yaml:"cmd,omitempty" json:"cmd,omitempty"`
	// ExpectStatus 预期业务目标状态字符串
	ExpectStatus string `yaml:"expect_status" json:"expect_status"`
	// ExpectResult 预期返回载荷强类型映射
	ExpectResult FactMap `yaml:"expect_result,omitempty" json:"expect_result,omitempty"`
	// ButterflyProbes 跨表与异步耦合蝴蝶探针强类型映射
	ButterflyProbes FactMap `yaml:"butterfly_probes,omitempty" json:"butterfly_probes,omitempty"`
	// ForbiddenEffects 零容忍禁止触发的副作用标签列表
	ForbiddenEffects []string `yaml:"forbidden_effects,omitempty" json:"forbidden_effects,omitempty"`
	// ProbeCmd 旁路蝴蝶探针核验命令
	ProbeCmd string `yaml:"probe_cmd,omitempty" json:"probe_cmd,omitempty"`
	// AntiSoftlock 中间悬挂态反软锁逃生不变量声明
	AntiSoftlock string `yaml:"anti_softlock,omitempty" json:"anti_softlock,omitempty"`
}

// effectiveExpectedInput 返回当前步骤生效的强类型预期输入映射
// 请求体
// 无
// 响应体
// input 优先返回 ExpectedInput，未配置时回退 With
func (st RouteStep) effectiveExpectedInput() FactMap {
	// 1. 优先选用显式 ExpectedInput 字段，否则回退 With 字段
	if len(st.ExpectedInput) > 0 {
		return st.ExpectedInput
	}
	return st.With
}

// StepEvidencePayload 描述测试执行器通过 [ARC_STEP_EVIDENCE] 输出的结构化实际观测载荷
type StepEvidencePayload struct {
	// Step 对应的步骤序号
	Step int `json:"step,omitempty"`
	// PreconditionsActual 实际观测到的前置条件映射
	PreconditionsActual FactMap `json:"preconditions_actual,omitempty"`
	// ActualInput 实际传入步骤的入参映射
	ActualInput FactMap `json:"actual_input,omitempty"`
	// BeforeState 实际记录的执行前状态映射
	BeforeState FactMap `json:"before_state,omitempty"`
	// AfterState 实际记录的执行后状态映射
	AfterState FactMap `json:"after_state,omitempty"`
	// ActualBehavior 实际观测行为分类
	ActualBehavior string `json:"actual_behavior,omitempty"`
	// ActualStatus 实际观测业务状态
	ActualStatus string `json:"actual_status,omitempty"`
	// ActualResult 实际返回结果映射
	ActualResult FactMap `json:"actual_result,omitempty"`
	// ActualProbes 实际探测到的蝴蝶探针键值映射
	ActualProbes FactMap `json:"actual_probes,omitempty"`
	// ObservedEffects 实际观测到的副作用标签列表
	ObservedEffects []string `json:"observed_effects,omitempty"`
	// ExtractedVars 提取供后续步骤消费的跨步骤强类型变量映射
	ExtractedVars FactMap `json:"extracted_vars,omitempty"`
	// Softlock 是否显式观测到业务软锁死
	Softlock bool `json:"softlock,omitempty"`
}

// ShardPlan 描述从Storyline编译出的多Agent并发分片调度清单
type ShardPlan struct {
	// StoryID 故事线唯一标识
	StoryID string `yaml:"story_id" json:"story_id"`
	// FocalEntity 核心受控业务实体
	FocalEntity string `yaml:"focal_entity" json:"focal_entity"`
	// StorySHA256 故事线定义原文SHA256摘要
	StorySHA256 string `yaml:"story_sha256" json:"story_sha256"`
	// FrozenReadonly 冻结只读路径列表
	FrozenReadonly []string `yaml:"frozen_readonly" json:"frozen_readonly"`
	// BlindMask 盲测隐藏路径掩码列表
	BlindMask []string `yaml:"blind_mask,omitempty" json:"blind_mask,omitempty"`
	// CheckpointOrder 拓扑排序后的检查点构建顺序
	CheckpointOrder []string `yaml:"checkpoint_order" json:"checkpoint_order"`
	// ConcurrentShards 互不干扰的并发验证分片列表
	ConcurrentShards []VerifyShard `yaml:"concurrent_shards" json:"concurrent_shards"`
}

// VerifyShard 描述单个并发隔离验证分片的调度契约
type VerifyShard struct {
	// ShardID 分片唯一ID（对应路由ID）
	ShardID string `yaml:"shard_id" json:"shard_id"`
	// LoadSave 加载的检查点存档ID
	LoadSave string `yaml:"load_save" json:"load_save"`
	// Layer 验证器分层标识
	Layer string `yaml:"layer" json:"layer"`
	// AgentHint 推荐执行Agent类型
	AgentHint string `yaml:"agent_hint" json:"agent_hint"`
	// WriteScope 独占写入文件范围列表
	WriteScope []string `yaml:"write_scope" json:"write_scope"`
	// RuntimeNS 隔离运行命名空间
	RuntimeNS string `yaml:"runtime_ns" json:"runtime_ns"`
	// UnlocksEnding 目标解锁结局标识
	UnlocksEnding string `yaml:"unlocks_ending" json:"unlocks_ending"`
	// VerifyCmd 分片执行验证命令
	VerifyCmd string `yaml:"verify_cmd" json:"verify_cmd"`
}

// CompletionReport 描述故事线四维100%完成度与历史失败保留评估报告
type CompletionReport struct {
	// StoryID 故事线唯一标识
	StoryID string `json:"story_id"`
	// StorySHA256 故事线原文SHA256摘要
	StorySHA256 string `json:"story_sha256"`
	// Verdict 完成度总判定（PASS | FAIL | BLOCKED | NOT_RUN）
	Verdict string `json:"verdict"`
	// WorldFlagBindingPct 世界状态标志绑定覆盖率百分比
	WorldFlagBindingPct float64 `json:"world_flag_binding_pct"`
	// RouteCoveragePct 分支路由通过覆盖率百分比
	RouteCoveragePct float64 `json:"route_coverage_pct"`
	// ButterflyProbeHitPct 蝴蝶探针命中率百分比
	ButterflyProbeHitPct float64 `json:"butterfly_probe_hit_pct"`
	// EndingUnlockPct 必需结局解锁率百分比
	EndingUnlockPct float64 `json:"ending_unlock_pct"`
	// SoftlockCount 当前活跃软锁计数
	SoftlockCount int `json:"softlock_count"`
	// UnresolvedFailCount 当前版本未解决失败路由数
	UnresolvedFailCount int `json:"unresolved_fail_count"`
	// HistoricalFailCount 账本永久保留的历史FAIL累计次数
	HistoricalFailCount int `json:"historical_fail_count"`
	// UnlockedEndings 已解锁结局列表
	UnlockedEndings []string `json:"unlocked_endings"`
	// MissingEndings 尚未解锁结局列表
	MissingEndings []string `json:"missing_endings"`
	// RouteStatuses 各路由最新状态映射
	RouteStatuses map[string]string `json:"route_statuses"`
	// Reason 未达成PASS的具体阻断或失败原因
	Reason string `json:"reason,omitempty"`
}

// loadStoryline 从YAML或JSON文件加载故事线契约并补全哈希与默认隔离命名空间
// 请求体
// path 故事线声明文件路径
// 响应体
// story 解析后的故事线结构体指针
// err 文件读取或语法解析错误
func loadStoryline(path string) (*Storyline, error) {
	// 1. 读取故事线原始字节并按扩展名执行JSON或YAML强类型解码
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read storyline %s: %w", path, err)
	}
	var s Storyline
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("parse JSON storyline %s: %w", path, err)
		}
	} else {
		if err := yaml.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("parse YAML storyline %s: %w", path, err)
		}
	}
	// 2. 记录绝对路径、原文SHA256摘要并为缺省路由生成隔离命名空间
	abs, err := filepath.Abs(path)
	if err == nil {
		s.SourcePath = abs
	} else {
		s.SourcePath = path
	}
	sum := sha256.Sum256(raw)
	s.SHA256 = hex.EncodeToString(sum[:])
	for i := range s.Routes {
		if strings.TrimSpace(s.Routes[i].RuntimeNS) == "" && s.Routes[i].RouteID != "" {
			s.Routes[i].RuntimeNS = "ns_" + sanitizeNS(s.Routes[i].RouteID)
		}
	}
	return &s, nil
}

// sanitizeNS 将任意标识符规范化为仅含小写字母、数字与下划线的安全命名空间词元
// 请求体
// id 原始路由或检查点标识
// 响应体
// ns 规范化后的安全命名空间字符串
func sanitizeNS(id string) string {
	// 1. 逐字符过滤非字母数字字符并替换为下划线
	var b strings.Builder
	for _, r := range strings.ToLower(id) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "default"
	}
	return b.String()
}

// sortedStringKeys 返回字符串映射按字典序升序排列的键列表
// 请求体
// m 字符串键值映射
// 响应体
// keys 升序排列的键名切片
func sortedStringKeys(m map[string]string) []string {
	// 1. 提取全部键名并执行字典序排序
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortedFactKeys 返回强类型断言映射按字典序升序排列的键列表
// 请求体
// m 强类型 FactMap 映射
// 响应体
// keys 升序排列的键名切片
func sortedFactKeys(m FactMap) []string {
	// 1. 提取全部键名并执行字典序排序
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
