package main

// symbolQueryResponse 描述 arc idx symbol --format json 的符号检索响应
type symbolQueryResponse struct {
	// Query 检索关键字
	Query string `json:"query"`
	// Count 命中符号数量
	Count int `json:"count"`
	// Symbols 符号条目列表
	Symbols []symbolEntry `json:"symbols"`
}

// astMatchItem 描述单条 ast-grep 结构化匹配命中记录
type astMatchItem struct {
	// File 命中文件相对路径
	File string `json:"file"`
	// Line 起始行号（1-based）
	Line int `json:"line"`
	// Lines 匹配源码文本片段
	Lines string `json:"text"`
}

// astQueryResponse 描述 arc idx ast --format json 的结构化检索响应
type astQueryResponse struct {
	// Pattern AST匹配模式串
	Pattern string `json:"pattern"`
	// Lang 目标编程语言标识
	Lang string `json:"lang"`
	// Count 命中总条数
	Count int `json:"count"`
	// Matches 命中结果列表
	Matches []astMatchItem `json:"matches"`
}

// filesListResponse 描述 arc idx files --format json 的候选文件枚举响应
type filesListResponse struct {
	// Profile 过滤配置Profile名称
	Profile string `json:"profile"`
	// Count 命中文件总数
	Count int `json:"count"`
	// Files 排序后的相对路径列表
	Files []string `json:"files"`
}

// langStatItem 描述单个文件扩展名的文件数与代码行数统计
type langStatItem struct {
	// Files 文件数量
	Files int `json:"files"`
	// Lines 代码总行数
	Lines int `json:"lines"`
}

// statsQueryResponse 描述 arc idx stats --format json 的代码规模统计响应
type statsQueryResponse struct {
	// Profile 过滤配置Profile名称
	Profile string `json:"profile"`
	// Total 汇总文件数与总行数
	Total langStatItem `json:"total"`
	// ByExtension 按扩展名分组的统计映射
	ByExtension map[string]*langStatItem `json:"by_extension"`
}

// skeletonQueryResponse 描述 arc idx skeleton --format json 的骨架提取响应
type skeletonQueryResponse struct {
	// Targets 输入目标路径列表
	Targets []string `json:"targets"`
	// FileCount 提取出符号的文件数量
	FileCount int `json:"file_count"`
	// SymbolCount 提取出的符号总数
	SymbolCount int `json:"symbol_count"`
	// Files 各文件符号骨架列表
	Files []skeletonFile `json:"files"`
}

// daemonStatusResponse 描述 arc idx daemon status 的守护进程状态响应
type daemonStatusResponse struct {
	// PID 守护进程PID
	PID int `json:"pid"`
	// Alive 守护进程是否存活
	Alive bool `json:"alive"`
	// IndexDir Zoekt索引分片目录路径
	IndexDir string `json:"index_dir"`
	// IndexAge 索引距今时长描述
	IndexAge string `json:"index_age"`
	// LogFile 守护进程日志文件路径
	LogFile string `json:"log_file"`
	// StaleIndex 源码修改时间是否晚于索引时间
	StaleIndex bool `json:"stale_index"`
}
