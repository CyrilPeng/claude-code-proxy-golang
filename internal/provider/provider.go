// Package provider 定义统一的提供商接口和实现。
// 支持 OpenRouter、OpenAI Direct 和 Ollama 等多种后端提供商。
package provider

import (
	"net/http"

	"github.com/CyrilPeng/claude-code-proxy-golang/internal/config"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/errors"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/models"
)

// Provider 定义后端提供商的统一接口
type Provider interface {
	// Name 返回提供商名称
	Name() string

	// Type 返回提供商类型
	Type() config.ProviderType

	// PrepareRequest 准备 OpenAI 格式的请求
	// 根据提供商特性添加特定参数（如 reasoning、tool_choice 等）
	PrepareRequest(req *models.OpenAIRequest) error

	// AddHeaders 添加提供商特定的 HTTP 头
	AddHeaders(httpReq *http.Request)

	// RequiresAuth 返回是否需要认证
	RequiresAuth() bool

	// GetAPIKey 返回 API 密钥
	GetAPIKey() string

	// GetBaseURL 返回基础 URL
	GetBaseURL() string

	// GetEndpoint 返回完整的 API 端点 URL
	GetEndpoint() string

	// HandleError 处理提供商返回的错误
	HandleError(statusCode int, body []byte) *errors.ProxyError

	// SupportsStreaming 返回是否支持流式传输
	SupportsStreaming() bool

	// SupportsToolCalls 返回是否支持工具调用
	SupportsToolCalls() bool

	// SupportsReasoning 返回是否支持推理/思考功能
	SupportsReasoning() bool

	// GetTimeout 返回请求超时时间（秒）
	GetTimeout() int

	// GetStreamTimeout 返回流式请求超时时间（秒）
	GetStreamTimeout() int
}

// BaseProvider 提供通用的基础实现
type BaseProvider struct {
	cfg *config.Config
}

// NewBaseProvider 创建基础提供商
func NewBaseProvider(cfg *config.Config) *BaseProvider {
	return &BaseProvider{cfg: cfg}
}

// GetAPIKey 返回 API 密钥
func (p *BaseProvider) GetAPIKey() string {
	return p.cfg.OpenAIAPIKey
}

// GetBaseURL 返回基础 URL
func (p *BaseProvider) GetBaseURL() string {
	return p.cfg.OpenAIBaseURL
}

// GetEndpoint 返回完整的 API 端点 URL
func (p *BaseProvider) GetEndpoint() string {
	return p.cfg.OpenAIBaseURL + "/chat/completions"
}

// SupportsStreaming 默认支持流式传输
func (p *BaseProvider) SupportsStreaming() bool {
	return true
}

// SupportsToolCalls 默认支持工具调用
func (p *BaseProvider) SupportsToolCalls() bool {
	return true
}

// SupportsReasoning 默认不支持推理
func (p *BaseProvider) SupportsReasoning() bool {
	return false
}

// GetTimeout 返回默认请求超时时间（90秒）
func (p *BaseProvider) GetTimeout() int {
	return 90
}

// GetStreamTimeout 返回默认流式请求超时时间（300秒）
func (p *BaseProvider) GetStreamTimeout() int {
	return 300
}

// Config 返回配置（供子类使用）
func (p *BaseProvider) Config() *config.Config {
	return p.cfg
}
