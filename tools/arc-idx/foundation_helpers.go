package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"os"
	"strings"
)

var htmlUnescaper = strings.NewReplacer(
	`\u003c`, `<`,
	`\u003e`, `>`,
	`\u0026`, `&`,
)

// emitRawJSON 将已序列化的强类型缩进JSON字节按不转义HTML规则写入标准输出
// 请求体
// raw json.MarshalIndent产出的JSON字节切片
// marshalErr 序列化阶段返回的错误
// 响应体
// err 序列化错误或标准输出写入错误
func emitRawJSON(raw []byte, marshalErr error) error {
	// 1. 优先拦截并返回上游强类型结构体的序列化错误
	if marshalErr != nil {
		return marshalErr
	}
	// 2. 还原HTML尖括号与和号转义并追加换行写入标准输出
	clean := []byte(htmlUnescaper.Replace(string(append(raw, '\n'))))
	_, err := os.Stdout.Write(clean)
	return err
}

// parseInterleaved 解析与位置参数穿插排列的命令行标志
// 请求体
// fl 标准库FlagSet实例
// args 原始命令行参数切片
// 响应体
// positional 提取出的位置参数列表
// err 标志解析错误
func parseInterleaved(fl *flag.FlagSet, args []string) ([]string, error) {
	// 1. 循环解析当前前缀标志并收集首个未消费的位置参数
	var positional []string
	for len(args) > 0 {
		if err := fl.Parse(args); err != nil {
			return nil, err
		}
		args = fl.Args()
		if len(args) == 0 {
			break
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
	// 2. 返回按顺序提取的全部位置参数
	return positional, nil
}

// fileContentHash 计算文件内容的前16字节截断SHA256增量索引指纹
// 请求体
// absPath 目标文件绝对路径
// 响应体
// digest 带sha256前缀的十六进制内容指纹（读取失败返回空字符串）
func fileContentHash(absPath string) string {
	// 1. 读取文件字节并计算SHA256前16字节十六进制指纹
	data, err := os.ReadFile(absPath)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:16])
}

// orDefault 当输入字符串为空时返回指定默认值
// 请求体
// s 待检查字符串
// d 空值回退默认字符串
// 响应体
// out 非空字符串结果
func orDefault(s, d string) string {
	// 1. 判断字符串是否为空并返回原值或默认值
	if s == "" {
		return d
	}
	return s
}
