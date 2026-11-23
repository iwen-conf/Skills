package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ImpactReport 描述目标业务实体跨SQL表、ORM模型、路由与异步消息主题的反向耦合报告
type ImpactReport struct {
	// Target 输入目标实体标识
	Target string `json:"target"`
	// DerivedTokens 派生出的多形态搜索标识词
	DerivedTokens []string `json:"derived_tokens"`
	// SQLTables 命中的SQL耦合数据表列表
	SQLTables []SQLCoupling `json:"sql_tables"`
	// ORMModels 命中的ORM模型列表
	ORMModels []ORMCoupling `json:"orm_models"`
	// Routes 关联HTTP/RPC路由命中列表
	Routes []RouteHit `json:"routes"`
	// AsyncTopics 关联异步消息主题命中列表
	AsyncTopics []TopicHit `json:"async_topics"`
	// ProdReferences 生产代码引用命中列表
	ProdReferences []CodeRefHit `json:"prod_references"`
	// WorldFlagHints 推导出的候选 world_flags 列表
	WorldFlagHints []WorldFlagHint `json:"world_flag_hints"`
}

// SQLCoupling 描述单张耦合SQL数据表的外键列、状态列与金额列信息
type SQLCoupling struct {
	// Table 数据表名称
	Table string `json:"table"`
	// File 所在SQL文件相对路径
	File string `json:"file"`
	// Line 建表语句行号
	Line int `json:"line"`
	// FKColumns 外键关联列名列表
	FKColumns []string `json:"fk_columns"`
	// StatusFields 状态阶段列名列表
	StatusFields []string `json:"status_fields,omitempty"`
	// AmountFields 金额余额列名列表
	AmountFields []string `json:"amount_fields,omitempty"`
}

// ORMCoupling 描述单个耦合ORM结构体或模型类的外键与状态字段信息
type ORMCoupling struct {
	// Model 模型结构体或类名称
	Model string `json:"model"`
	// File 所在源码文件相对路径
	File string `json:"file"`
	// Line 声明起始行号
	Line int `json:"line"`
	// RefFields 外键关联字段名列表
	RefFields []string `json:"ref_fields"`
	// StatusFields 状态与金额字段名列表
	StatusFields []string `json:"status_fields,omitempty"`
}

// RouteHit 描述单处关联路由声明命中位置
type RouteHit struct {
	// File 路由文件相对路径
	File string `json:"file"`
	// Line 命中行号
	Line int `json:"line"`
	// Text 截断后的路由代码行
	Text string `json:"text"`
}

// TopicHit 描述单处关联异步消息主题发布或订阅命中位置
type TopicHit struct {
	// Topic 消息主题或队列名称
	Topic string `json:"topic"`
	// File 源码文件相对路径
	File string `json:"file"`
	// Line 命中行号
	Line int `json:"line"`
	// Text 截断后的调用代码行
	Text string `json:"text"`
}

// CodeRefHit 描述生产源码中对目标实体的引用位置
type CodeRefHit struct {
	// File 源码文件相对路径
	File string `json:"file"`
	// Line 命中行号
	Line int `json:"line"`
	// Layer 文件权威分层标签
	Layer string `json:"layer"`
	// Text 截断后的代码行文本
	Text string `json:"text"`
}

// WorldFlagHint 描述推荐纳入 Storyline world_flags 的耦合状态提示
type WorldFlagHint struct {
	// Flag 推荐的 world_flag 键名
	Flag string `json:"flag"`
	// Source 对应的表列、模型字段或消息主题来源
	Source string `json:"source"`
	// File 来源文件相对路径
	File string `json:"file"`
	// Line 来源行号
	Line int `json:"line"`
	// Reason 推导原因分类
	Reason string `json:"reason"`
}

// cmdImpact 执行跨层SQL/ORM/路由/异步队列反向耦合扫描并输出JSON或Markdown
// 请求体
// args 命令行参数切片
// 响应体
// err 参数缺失、仓库读取或编码错误
func cmdImpact(args []string) error {
	// 1. 解析目标实体参数、生产过滤开关与输出格式标志
	fl := flag.NewFlagSet("impact", flag.ExitOnError)
	repoDir := fl.String("repo", ".", "repository root or any directory inside it")
	prodOnly := fl.Bool("prod-only", true, "exclude test and doc files from code references")
	formatMode := fl.String("format", "ai-md", "output format: json | ai-md")
	positional, err := parseInterleaved(fl, args)
	if err != nil {
		return err
	}
	if len(positional) < 1 {
		return fmt.Errorf("usage: arc idx impact <entity-or-table.col> [--prod-only] [--format json|ai-md]")
	}
	// 2. 定位仓库并执行跨层影响面扫描
	repo, err := findRepo(*repoDir)
	if err != nil {
		return err
	}
	report, err := scanImpact(repo, positional[0], *prodOnly)
	if err != nil {
		return err
	}
	// 3. 按请求格式输出结构化报告
	if *formatMode == "json" {
		return emitRawJSON(json.MarshalIndent(report, "", "  "))
	}
	fmt.Print(renderImpactMarkdown(report))
	return nil
}

// scanImpact 遍历仓库候选文件扫描目标实体的全链路耦合关系
// 请求体
// repo 已解析的仓库上下文指针
// target 目标实体标识（如 users.id）
// prodOnly 是否排除测试与文档文件
// 响应体
// report 完整跨层影响面报告指针
// err 枚举仓库文件失败错误
func scanImpact(repo *Repo, target string, prodOnly bool) (*ImpactReport, error) {
	// 1. 枚举仓库可索引文件并推导目标实体的多命名风格词元
	files, err := walkRepo(repo)
	if err != nil {
		return nil, err
	}
	tableName, fkCol, tokens := deriveEntityTokens(target)
	report := &ImpactReport{
		Target:        target,
		DerivedTokens: tokens,
	}
	fkColLower := strings.ToLower(fkCol)
	tableLower := strings.ToLower(tableName)
	singular := singularize(tableLower)
	camelLower := strings.ToLower(toPascal(singular) + "id")
	// 2. 逐文件扫描SQL建表、ORM结构体、路由注册、异步消息与生产代码引用
	for _, rel := range files {
		layer := pathLayer(rel)
		if prodOnly && (layer == "doc" || layer == "test") {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(repo.Root, rel))
		if readErr != nil {
			continue
		}
		text := string(raw)
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".sql" {
			report.SQLTables = append(report.SQLTables, scanSQLFile(rel, text, tableLower, fkColLower)...)
		}
		if ext == ".go" || ext == ".ts" || ext == ".tsx" || ext == ".py" || ext == ".prisma" || ext == ".java" || ext == ".kt" {
			report.ORMModels = append(report.ORMModels, scanORMFile(rel, text, fkColLower, camelLower, tableLower)...)
		}
		for idx, line := range strings.Split(text, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			low := strings.ToLower(trimmed)
			if m := reRouteCall.FindStringSubmatch(trimmed); m != nil {
				if strings.Contains(low, singular) || strings.Contains(low, tableLower) || strings.Contains(low, strings.ToLower(target)) {
					report.Routes = append(report.Routes, RouteHit{File: rel, Line: idx + 1, Text: truncateLine(trimmed)})
				}
			}
			if m := reTopicCall.FindStringSubmatch(trimmed); m != nil {
				topic := m[2]
				topicLow := strings.ToLower(topic)
				if strings.Contains(topicLow, singular) || strings.Contains(topicLow, tableLower) || strings.Contains(low, fkColLower) || strings.Contains(low, camelLower) {
					report.AsyncTopics = append(report.AsyncTopics, TopicHit{Topic: topic, File: rel, Line: idx + 1, Text: truncateLine(trimmed)})
				}
			}
			if layer == "prod" && len(report.ProdReferences) < 80 &&
				(strings.Contains(trimmed, target) || strings.Contains(low, fkColLower) || strings.Contains(low, camelLower)) {
				report.ProdReferences = append(report.ProdReferences, CodeRefHit{File: rel, Line: idx + 1, Layer: layer, Text: truncateLine(trimmed)})
			}
		}
	}
	// 3. 合成候选 world_flags 提示并返回
	report.WorldFlagHints = buildWorldFlagHints(report)
	return report, nil
}

// renderImpactMarkdown 将跨层影响面扫描报告渲染为紧凑Markdown文本
// 请求体
// r 跨层影响面报告指针
// 响应体
// md 格式化后的Markdown字符串
func renderImpactMarkdown(r *ImpactReport) string {
	// 1. 按SQL表、ORM模型、路由/异步Topic与候选world_flags四节格式化输出
	var b strings.Builder
	fmt.Fprintf(&b, "## impact `%s`\n\n", r.Target)
	fmt.Fprintf(&b, "Tokens: `%s`\n\n", strings.Join(r.DerivedTokens, "`, `"))
	fmt.Fprintf(&b, "### SQL Tables (%d)\n", len(r.SQLTables))
	for _, t := range r.SQLTables {
		fmt.Fprintf(&b, "- `%s` (%s:%d) fk=[%s] status=[%s] amount=[%s]\n",
			t.Table, t.File, t.Line,
			strings.Join(t.FKColumns, ","),
			strings.Join(t.StatusFields, ","),
			strings.Join(t.AmountFields, ","))
	}
	fmt.Fprintf(&b, "\n### ORM Models (%d)\n", len(r.ORMModels))
	for _, m := range r.ORMModels {
		fmt.Fprintf(&b, "- `%s` (%s:%d) refs=[%s] state=[%s]\n",
			m.Model, m.File, m.Line,
			strings.Join(m.RefFields, ","),
			strings.Join(m.StatusFields, ","))
	}
	fmt.Fprintf(&b, "\n### Routes (%d) & Async Topics (%d)\n", len(r.Routes), len(r.AsyncTopics))
	for _, rt := range r.Routes {
		fmt.Fprintf(&b, "- route %s:%d `%s`\n", rt.File, rt.Line, rt.Text)
	}
	for _, tp := range r.AsyncTopics {
		fmt.Fprintf(&b, "- topic `%s` %s:%d `%s`\n", tp.Topic, tp.File, tp.Line, tp.Text)
	}
	fmt.Fprintf(&b, "\n### Candidate `world_flags` (%d)\n", len(r.WorldFlagHints))
	for _, h := range r.WorldFlagHints {
		fmt.Fprintf(&b, "- `%s`: `%s` (%s:%d — %s)\n", h.Flag, h.Source, h.File, h.Line, h.Reason)
	}
	return b.String()
}
