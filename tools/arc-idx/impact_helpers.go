package main

import (
	"regexp"
	"slices"
	"sort"
	"strings"
)

var (
	reSQLColumnLine = regexp.MustCompile(`(?i)^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s+([a-zA-Z0-9_(),\s]+)`)
	reSQLReferences = regexp.MustCompile(`(?i)REFERENCES\s+["` + "`" + `]?([a-zA-Z_][a-zA-Z0-9_]*)["` + "`" + `]?`)
	reStatusName    = regexp.MustCompile(`(?i)(status|state|phase|stage|frozen|locked|pending|active|deleted|cancelled|canceled|dispute|arbitration)`)
	reAmountName    = regexp.MustCompile(`(?i)(balance|amount|cents|pending|in_transit|frozen|escrow|refund|settlement|credit|debit)`)
	reGoStruct      = regexp.MustCompile(`^\s*type\s+([A-Z][a-zA-Z0-9_]*)\s+struct\s*\{`)
	reClassModel    = regexp.MustCompile(`^\s*(?:export\s+)?(?:class|model|interface)\s+([A-Z][a-zA-Z0-9_]*)`)
	reRouteCall     = regexp.MustCompile(`(?i)\.(GET|POST|PUT|PATCH|DELETE|Handle|HandleFunc|Any|Route)\s*\(\s*["` + "`" + `']([^"` + "`" + `']+)["` + "`" + `']`)
	reTopicCall     = regexp.MustCompile(`(?i)(Publish|Subscribe|Consume|Emit|Produce|SendEvent|Topic|Queue)\w*\s*\([^)]*["` + "`" + `']([a-zA-Z0-9_.:/-]+)["` + "`" + `']`)
)

// deriveEntityTokens 从输入实体或表列名推导表名、外键列名与多命名风格搜索词集合
// 请求体
// target 用户输入的目标实体或表列标识（如 users.id）
// 响应体
// tableName 推导出的复数表名
// fkCol 推导出的外键列名
// tokens 去重后的多形态匹配词列表
func deriveEntityTokens(target string) (tableName, fkCol string, tokens []string) {
	// 1. 拆分表名与列名并推导单复数及蛇形/驼峰外键形式
	clean := strings.TrimSpace(target)
	parts := strings.Split(clean, ".")
	base := parts[0]
	tableName = strings.ToLower(base)
	singular := singularize(tableName)
	if len(parts) > 1 {
		fkCol = singular + "_" + strings.ToLower(parts[1])
	} else if strings.HasSuffix(tableName, "_id") {
		fkCol = tableName
		singular = strings.TrimSuffix(tableName, "_id")
		tableName = singular + "s"
	} else {
		fkCol = singular + "_id"
	}
	camel := toPascal(singular) + "ID"
	camelLower := singular + "Id"
	// 2. 去重合并全部派生标识词后返回
	seen := map[string]bool{}
	for _, tok := range []string{clean, tableName, singular, fkCol, camel, camelLower} {
		if tok != "" && !seen[tok] {
			seen[tok] = true
			tokens = append(tokens, tok)
		}
	}
	return tableName, fkCol, tokens
}

// singularize 将常见英文复数表名归一为单数词干
// 请求体
// word 原始英文复数词
// 响应体
// singular 单数词干
func singularize(word string) string {
	// 1. 依次处理 ies/ses/s 常见复数词尾
	switch {
	case strings.HasSuffix(word, "ies") && len(word) > 3:
		return word[:len(word)-3] + "y"
	case strings.HasSuffix(word, "ses") && len(word) > 3:
		return word[:len(word)-2]
	case strings.HasSuffix(word, "s") && len(word) > 2 && !strings.HasSuffix(word, "ss"):
		return word[:len(word)-1]
	default:
		return word
	}
}

// toPascal 将下划线蛇形命名转换为大驼峰命名
// 请求体
// snake 蛇形字符串
// 响应体
// pascal 大驼峰字符串
func toPascal(snake string) string {
	// 1. 按下划线切分并将各段首字母大写拼接
	var b strings.Builder
	for _, p := range strings.Split(snake, "_") {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + p[1:])
	}
	return b.String()
}

// scanSQLFile 扫描单个SQL文件提取引用目标实体的表、外键列、状态列与金额列
// 请求体
// rel SQL文件相对路径
// text SQL源码文本
// tableLower 目标表名小写
// fkColLower 目标外键名小写
// 响应体
// hits 命中的SQL表耦合列表
func scanSQLFile(rel, text, tableLower, fkColLower string) []SQLCoupling {
	// 1. 遍历各 CREATE TABLE 语句块识别 REFERENCES 外键与同名关联列
	lines := strings.Split(text, "\n")
	var hits []SQLCoupling
	for i := 0; i < len(lines); i++ {
		m := reSQLTable.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		tbl := strings.Trim(m[1], "\"`")
		end := i
		for j := i; j < len(lines); j++ {
			end = j
			if strings.Contains(lines[j], ";") {
				break
			}
		}
		var fkCols, statusCols, amountCols []string
		matched := strings.EqualFold(tbl, tableLower)
		for j := i + 1; j <= end; j++ {
			l := strings.TrimSpace(lines[j])
			if l == "" || strings.HasPrefix(l, "--") || strings.HasPrefix(l, ")") {
				continue
			}
			low := strings.ToLower(l)
			if ref := reSQLReferences.FindStringSubmatch(l); ref != nil && strings.EqualFold(ref[1], tableLower) {
				matched = true
				if cm := reSQLColumnLine.FindStringSubmatch(l); cm != nil {
					fkCols = appendUnique(fkCols, cm[1])
				} else {
					fkCols = appendUnique(fkCols, "FOREIGN_KEY("+tableLower+")")
				}
			}
			if cm := reSQLColumnLine.FindStringSubmatch(l); cm != nil {
				col := cm[1]
				colLow := strings.ToLower(col)
				if isSQLKeyword(colLow) {
					continue
				}
				if colLow == fkColLower || strings.HasSuffix(colLow, "_"+fkColLower) {
					matched = true
					fkCols = appendUnique(fkCols, col)
				}
				if reStatusName.MatchString(colLow) {
					statusCols = appendUnique(statusCols, col)
				}
				if reAmountName.MatchString(colLow) {
					amountCols = appendUnique(amountCols, col)
				}
			} else if strings.Contains(low, fkColLower) {
				matched = true
			}
		}
		if matched {
			hits = append(hits, SQLCoupling{
				Table:        tbl,
				File:         rel,
				Line:         i + 1,
				FKColumns:    fkCols,
				StatusFields: statusCols,
				AmountFields: amountCols,
			})
		}
		i = end
	}
	return hits
}

// isSQLKeyword 判断标识符是否为SQL约束或DDL保留关键字
// 请求体
// w 小写单词字符串
// 响应体
// ok 是否为SQL保留关键字
func isSQLKeyword(w string) bool {
	// 1. 匹配建表块常见约束关键字
	switch w {
	case "primary", "foreign", "constraint", "unique", "index", "key", "check", "create", "alter", "drop":
		return true
	default:
		return false
	}
}

// scanORMFile 扫描源码文件中的结构体或ORM模型提取外键关联字段与状态金额字段
// 请求体
// rel 源码文件相对路径
// text 文件文本内容
// fkColLower 蛇形外键名小写
// camelLower 驼峰外键名小写
// tableLower 目标表名小写
// 响应体
// hits 命中的ORM模型耦合列表
func scanORMFile(rel, text, fkColLower, camelLower, tableLower string) []ORMCoupling {
	// 1. 定位结构体或类定义块并扫描内部关联字段与状态/金额字段
	lines := strings.Split(text, "\n")
	var hits []ORMCoupling
	for i := 0; i < len(lines); i++ {
		var model string
		if m := reGoStruct.FindStringSubmatch(lines[i]); m != nil {
			model = m[1]
		} else if m := reClassModel.FindStringSubmatch(lines[i]); m != nil {
			model = m[1]
		} else {
			continue
		}
		end := findBraceEnd(lines, i)
		if end == i {
			end = min(len(lines)-1, i+40)
		}
		var refs, statuses []string
		for j := i + 1; j <= end; j++ {
			trimmed := strings.TrimSpace(lines[j])
			if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") {
				continue
			}
			low := strings.ToLower(trimmed)
			if strings.Contains(low, fkColLower) || strings.Contains(low, camelLower) ||
				(strings.Contains(low, "references") && strings.Contains(low, tableLower)) {
				if fieldName := firstIdentifier(trimmed); fieldName != "" {
					refs = appendUnique(refs, fieldName)
				}
			}
			if reStatusName.MatchString(low) || reAmountName.MatchString(low) {
				if fieldName := firstIdentifier(trimmed); fieldName != "" {
					statuses = appendUnique(statuses, fieldName)
				}
			}
		}
		if len(refs) > 0 {
			hits = append(hits, ORMCoupling{
				Model:        model,
				File:         rel,
				Line:         i + 1,
				RefFields:    refs,
				StatusFields: statuses,
			})
		}
		i = end
	}
	return hits
}

// firstIdentifier 提取字段声明行的主标识符名称（跳过可见性修饰符）
// 请求体
// line 去除首尾空白的声明行文本
// 响应体
// ident 提取出的字段标识符
func firstIdentifier(line string) string {
	// 1. 切分空白字段并跳过 var/const/public 等前置修饰词
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	tok := strings.TrimRight(fields[0], ":;,?")
	if tok == "var" || tok == "const" || tok == "readonly" || tok == "public" || tok == "private" {
		if len(fields) > 1 {
			tok = strings.TrimRight(fields[1], ":;,?")
		}
	}
	return tok
}

// buildWorldFlagHints 根据扫描出的SQL表、ORM字段与异步Topic生成去重排序的 world_flags 提示
// 请求体
// r 跨层影响面扫描报告指针
// 响应体
// hints 排序后的候选 world_flags 切片
func buildWorldFlagHints(r *ImpactReport) []WorldFlagHint {
	// 1. 汇总SQL状态列、金额列、外键计数、ORM状态字段与异步主题生成候选标志
	var hints []WorldFlagHint
	seen := map[string]bool{}
	add := func(flag, source, file string, line int, reason string) {
		if seen[flag] {
			return
		}
		seen[flag] = true
		hints = append(hints, WorldFlagHint{
			Flag: flag, Source: source, File: file, Line: line, Reason: reason,
		})
	}
	for _, t := range r.SQLTables {
		for _, col := range t.StatusFields {
			add(t.Table+"_"+col, t.Table+"."+col, t.File, t.Line, "sql_status_column")
		}
		for _, col := range t.AmountFields {
			add(t.Table+"_"+col, t.Table+"."+col, t.File, t.Line, "sql_amount_column")
		}
		if len(t.StatusFields) == 0 && len(t.AmountFields) == 0 && len(t.FKColumns) > 0 {
			add(t.Table+"_active_count", t.Table+".*", t.File, t.Line, "sql_fk_reference")
		}
	}
	for _, m := range r.ORMModels {
		prefix := strings.ToLower(m.Model)
		for _, f := range m.StatusFields {
			add(prefix+"_"+strings.ToLower(f), m.Model+"."+f, m.File, m.Line, "orm_state_field")
		}
	}
	for _, topic := range r.AsyncTopics {
		key := "async_" + strings.NewReplacer(".", "_", "/", "_", "-", "_", ":", "_").Replace(strings.ToLower(topic.Topic))
		add(key, "topic:"+topic.Topic, topic.File, topic.Line, "async_topic_coupling")
	}
	// 2. 按标志名称字典序稳定排序返回
	sort.Slice(hints, func(i, j int) bool { return hints[i].Flag < hints[j].Flag })
	return hints
}

// appendUnique 当切片中不存在相同字符串时追加元素
// 请求体
// slice 原始字符串切片
// item 待追加字符串
// 响应体
// out 去重追加后的字符串切片
func appendUnique(slice []string, item string) []string {
	// 1. 使用标准库 slices.Contains 检查是否已存在该元素，不存在则追加
	if slices.Contains(slice, item) {
		return slice
	}
	return append(slice, item)
}
