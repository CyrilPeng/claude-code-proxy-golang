// Package json 提供高性能 JSON 序列化/反序列化功能。
// 使用 goccy/go-json 替代标准库 encoding/json，提升约 2-3 倍性能。
// 该包装器保持与标准库相同的 API，便于全项目统一替换。
package json

import (
	gojson "github.com/goccy/go-json"
)

// Marshal 将值序列化为 JSON 字节切片
// 性能比标准库快约 2-3 倍
func Marshal(v interface{}) ([]byte, error) {
	return gojson.Marshal(v)
}

// MarshalIndent 将值序列化为带缩进的 JSON 字节切片
// 用于调试日志等需要可读格式的场景
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return gojson.MarshalIndent(v, prefix, indent)
}

// Unmarshal 将 JSON 字节切片反序列化为值
// 性能比标准库快约 2-3 倍
func Unmarshal(data []byte, v interface{}) error {
	return gojson.Unmarshal(data, v)
}

// Valid 检查字节切片是否为有效的 JSON
func Valid(data []byte) bool {
	return gojson.Valid(data)
}

// RawMessage 是原始编码的 JSON 值
// 实现 Marshaler 和 Unmarshaler 接口
type RawMessage = gojson.RawMessage

// Number 表示 JSON 数字字面量
type Number = gojson.Number

// Encoder 是 JSON 编码器
type Encoder = gojson.Encoder

// Decoder 是 JSON 解码器
type Decoder = gojson.Decoder

// NewEncoder 创建新的 JSON 编码器
func NewEncoder(w interface{ Write([]byte) (int, error) }) *Encoder {
	return gojson.NewEncoder(w)
}

// NewDecoder 创建新的 JSON 解码器
func NewDecoder(r interface{ Read([]byte) (int, error) }) *Decoder {
	return gojson.NewDecoder(r)
}
