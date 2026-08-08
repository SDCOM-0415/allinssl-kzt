package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ConfigParam 描述插件在 AllinSSL 授权配置或 action 参数表单中的字段。
// 1.1.3 后端只保存 type 字符串，但前端会按 type 渲染不同控件。
type ConfigParam struct {
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Description string           `json:"description"`
	Required    bool             `json:"required"`
	Options     []map[string]any `json:"options,omitempty"`
}

// ActionInfo 描述插件对外暴露的一个 action。
type ActionInfo struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Params      []ConfigParam `json:"params,omitempty"`
}

// PluginMetadata 是 get_metadata 返回的核心数据结构，必须能被 AllinSSL 强类型反序列化。
type PluginMetadata struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Version     string        `json:"version"`
	Author      string        `json:"author"`
	Actions     []ActionInfo  `json:"actions"`
	Config      []ConfigParam `json:"config,omitempty"`
}

// Request 是 AllinSSL 通过 stdin 发送给插件的请求。
type Request struct {
	Action string         `json:"action"`
	Params map[string]any `json:"params,omitempty"`
}

// Response 是插件通过 stdout 返回的响应。
type Response struct {
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Result  map[string]any `json:"result,omitempty"`
}

// readRequest 从 stdin 读取完整请求并反序列化。
func readRequest() (*Request, error) {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("读取标准输入失败: %w", err)
	}
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("解析请求失败: %w", err)
	}
	if req.Action == "" {
		return nil, fmt.Errorf("缺少 action 字段")
	}
	return &req, nil
}

// writeResponse 把响应序列化后一次性写入 stdout。
// stdout 是协议通道，不能混入任何其他文本。
func writeResponse(resp Response) {
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}

// success 构造成功响应。
func success(message string, result map[string]any) Response {
	return Response{Status: "success", Message: message, Result: result}
}

// failure 构造失败响应，result 统一为空对象。
func failure(message string) Response {
	return Response{Status: "error", Message: message, Result: map[string]any{}}
}
