package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// FactKind 表示六维验证断言与动态变量值的闭合强类型枚举
type FactKind string

const (
	FactKindUnspecified FactKind = ""
	FactKindNull        FactKind = "null"
	FactKindBool        FactKind = "bool"
	FactKindInt         FactKind = "int"
	FactKindFloat       FactKind = "float"
	FactKindString      FactKind = "string"
	FactKindArray       FactKind = "array"
	FactKindObject      FactKind = "object"
)

// FactMap 表示强类型键值断言与观测状态映射（彻底替代弱类型动态字典）
type FactMap map[string]FactValue

// FactValue 表示编译期确定行为的六维断言值判别联合体
type FactValue struct {
	// Kind 当前载荷所属闭合值类型枚举
	Kind FactKind
	// BoolVal 布尔类型值载荷
	BoolVal bool
	// IntVal 64位有符号整数值载荷
	IntVal int64
	// FloatVal 64位浮点数值载荷
	FloatVal float64
	// StrVal 字符串值载荷
	StrVal string
	// ArrayVal 序列数组元素载荷
	ArrayVal []FactValue
	// ObjectVal 嵌套键值对象载荷
	ObjectVal FactMap
}

// FactNull 构造空值类型的 FactValue
// 请求体
// 无
// 响应体
// val 空值类型的 FactValue
func FactNull() FactValue {
	// 1. 返回显式标记为 FactKindNull 的联合体实例
	return FactValue{Kind: FactKindNull}
}

// FactBool 构造布尔类型的 FactValue
// 请求体
// v 布尔原始值
// 响应体
// val 布尔类型的 FactValue
func FactBool(v bool) FactValue {
	// 1. 返回携带布尔值载荷的联合体实例
	return FactValue{Kind: FactKindBool, BoolVal: v}
}

// FactInt 构造64位整数类型的 FactValue
// 请求体
// v 64位整数原始值
// 响应体
// val 整数类型的 FactValue
func FactInt(v int64) FactValue {
	// 1. 返回携带64位整数载荷的联合体实例
	return FactValue{Kind: FactKindInt, IntVal: v}
}

// FactFloat 构造64位浮点数类型的 FactValue
// 请求体
// v 64位浮点数原始值
// 响应体
// val 浮点数类型的 FactValue
func FactFloat(v float64) FactValue {
	// 1. 返回携带64位浮点数载荷的联合体实例
	return FactValue{Kind: FactKindFloat, FloatVal: v}
}

// FactString 构造字符串类型的 FactValue
// 请求体
// v 字符串原始值
// 响应体
// val 字符串类型的 FactValue
func FactString(v string) FactValue {
	// 1. 返回携带字符串载荷的联合体实例
	return FactValue{Kind: FactKindString, StrVal: v}
}

// MarshalJSON 按闭合类型分支序列化 FactValue 为标准JSON字节流
// 请求体
// 无
// 响应体
// raw 标准JSON字节流
// err 序列化错误
func (v FactValue) MarshalJSON() ([]byte, error) {
	// 1. 按 Kind 闭合枚举分派精确序列化，零反射回退
	switch v.Kind {
	case FactKindUnspecified, FactKindNull:
		return []byte("null"), nil
	case FactKindBool:
		return strconv.AppendBool(nil, v.BoolVal), nil
	case FactKindInt:
		return strconv.AppendInt(nil, v.IntVal, 10), nil
	case FactKindFloat:
		return json.Marshal(v.FloatVal)
	case FactKindString:
		return json.Marshal(v.StrVal)
	case FactKindArray:
		if v.ArrayVal == nil {
			return []byte("[]"), nil
		}
		return json.Marshal(v.ArrayVal)
	case FactKindObject:
		if v.ObjectVal == nil {
			return []byte("{}"), nil
		}
		return json.Marshal(v.ObjectVal)
	default:
		return nil, fmt.Errorf("unsupported FactKind %q", v.Kind)
	}
}

// UnmarshalJSON 将JSON字节流精确解码为保留64位整数精度的 FactValue
// 请求体
// data 原始JSON字节流
// 响应体
// err JSON语法或数值解析错误
func (v *FactValue) UnmarshalJSON(data []byte) error {
	// 1. 裁剪空白并处理 null 与布尔字面量
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*v = FactNull()
		return nil
	}
	if bytes.Equal(trimmed, []byte("true")) {
		*v = FactBool(true)
		return nil
	}
	if bytes.Equal(trimmed, []byte("false")) {
		*v = FactBool(false)
		return nil
	}
	// 2. 按首字节精确解析字符串、数组与嵌套键值对象
	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return err
		}
		*v = FactString(s)
		return nil
	case '[':
		var rawItems []json.RawMessage
		if err := json.Unmarshal(trimmed, &rawItems); err != nil {
			return err
		}
		items := make([]FactValue, len(rawItems))
		for i, raw := range rawItems {
			if err := items[i].UnmarshalJSON(raw); err != nil {
				return err
			}
		}
		*v = FactValue{Kind: FactKindArray, ArrayVal: items}
		return nil
	case '{':
		var rawObj map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &rawObj); err != nil {
			return err
		}
		obj := make(FactMap, len(rawObj))
		for k, raw := range rawObj {
			var child FactValue
			if err := child.UnmarshalJSON(raw); err != nil {
				return err
			}
			obj[k] = child
		}
		*v = FactValue{Kind: FactKindObject, ObjectVal: obj}
		return nil
	default:
		// 3. 优先按64位有符号整数解析数值，含小数或指数时回退64位浮点数
		numText := string(trimmed)
		if !bytes.ContainsAny(trimmed, ".eE") {
			if n, err := strconv.ParseInt(numText, 10, 64); err == nil {
				*v = FactInt(n)
				return nil
			}
		}
		f, err := strconv.ParseFloat(numText, 64)
		if err != nil {
			return fmt.Errorf("invalid JSON fact value %q: %w", numText, err)
		}
		*v = FactFloat(f)
		return nil
	}
}

// UnmarshalYAML 从 yaml.v3 语法树节点强类型解码 FactValue
// 请求体
// node YAML AST节点指针
// 响应体
// err YAML节点类型或标量解码错误
func (v *FactValue) UnmarshalYAML(node *yaml.Node) error {
	// 1. 处理空节点与锚点别名引用解包
	if node == nil {
		*v = FactNull()
		return nil
	}
	if node.Kind == yaml.AliasNode && node.Alias != nil {
		return v.UnmarshalYAML(node.Alias)
	}
	// 2. 按标量标签、序列节点与映射节点闭合解码
	switch node.Kind {
	case yaml.ScalarNode:
		switch node.ShortTag() {
		case "!!null":
			*v = FactNull()
			return nil
		case "!!bool":
			var b bool
			if err := node.Decode(&b); err != nil {
				return err
			}
			*v = FactBool(b)
			return nil
		case "!!int":
			var n int64
			if err := node.Decode(&n); err != nil {
				return err
			}
			*v = FactInt(n)
			return nil
		case "!!float":
			var f float64
			if err := node.Decode(&f); err != nil {
				return err
			}
			*v = FactFloat(f)
			return nil
		default:
			*v = FactString(node.Value)
			return nil
		}
	case yaml.SequenceNode:
		items := make([]FactValue, len(node.Content))
		for i, child := range node.Content {
			if err := items[i].UnmarshalYAML(child); err != nil {
				return err
			}
		}
		*v = FactValue{Kind: FactKindArray, ArrayVal: items}
		return nil
	case yaml.MappingNode:
		obj := make(FactMap, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			var child FactValue
			if err := child.UnmarshalYAML(node.Content[i+1]); err != nil {
				return err
			}
			obj[key] = child
		}
		*v = FactValue{Kind: FactKindObject, ObjectVal: obj}
		return nil
	default:
		return fmt.Errorf("unsupported YAML node kind %v", node.Kind)
	}
}

// FormatText 输出用于环境变量导出与可读差异对比的确定性文本表示
// 请求体
// 无
// 响应体
// text 标量原值或复合JSON文本
func (v FactValue) FormatText() string {
	// 1. 标量直接格式化为无引号字面量，数组与对象输出紧凑JSON
	switch v.Kind {
	case FactKindUnspecified, FactKindNull:
		return "null"
	case FactKindBool:
		return strconv.FormatBool(v.BoolVal)
	case FactKindInt:
		return strconv.FormatInt(v.IntVal, 10)
	case FactKindFloat:
		return strconv.FormatFloat(v.FloatVal, 'f', -1, 64)
	case FactKindString:
		return v.StrVal
	default:
		raw, err := v.MarshalJSON()
		if err != nil {
			return ""
		}
		return string(raw)
	}
}

// Equal 执行跨YAML整数与JSON浮点无损兼容的深度相等判断
// 请求体
// other 待比对的另一个 FactValue
// 响应体
// same 两者语义值是否严格一致
func (v FactValue) Equal(other FactValue) bool {
	// 1. 空值与未指定状态视为语义空值相等
	vNull := v.Kind == FactKindUnspecified || v.Kind == FactKindNull
	oNull := other.Kind == FactKindUnspecified || other.Kind == FactKindNull
	if vNull || oNull {
		return vNull && oNull
	}
	// 2. 处理整型与无小数浮点型之间的精确数值等价
	if v.Kind == FactKindInt && other.Kind == FactKindFloat {
		return float64(v.IntVal) == other.FloatVal
	}
	if v.Kind == FactKindFloat && other.Kind == FactKindInt {
		return v.FloatVal == float64(other.IntVal)
	}
	if v.Kind != other.Kind {
		return false
	}
	// 3. 按相同闭合类型分支递归比对标量、数组或嵌套对象
	switch v.Kind {
	case FactKindBool:
		return v.BoolVal == other.BoolVal
	case FactKindInt:
		return v.IntVal == other.IntVal
	case FactKindFloat:
		return v.FloatVal == other.FloatVal
	case FactKindString:
		return v.StrVal == other.StrVal
	case FactKindArray:
		if len(v.ArrayVal) != len(other.ArrayVal) {
			return false
		}
		for i := range v.ArrayVal {
			if !v.ArrayVal[i].Equal(other.ArrayVal[i]) {
				return false
			}
		}
		return true
	case FactKindObject:
		if len(v.ObjectVal) != len(other.ObjectVal) {
			return false
		}
		for k, sub := range v.ObjectVal {
			otherSub, ok := other.ObjectVal[k]
			if !ok || !sub.Equal(otherSub) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
