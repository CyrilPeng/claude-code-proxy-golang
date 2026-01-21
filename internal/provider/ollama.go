package provider

import (
	"net/http"

	"github.com/CyrilPeng/claude-code-proxy-golang/internal/config"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/errors"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/json"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/models"
)

// OllamaProvider 实现 Ollama 本地提供商
type OllamaProvider struct {
	*BaseProvider
}

// NewOllamaProvider 创建 Ollama 提供商
func NewOllamaProvider(cfg *config.Config) *OllamaProvider {
	return &OllamaProvider{
		BaseProvider: NewBaseProvider(cfg),
	}
}

// Name 返回提供商名称
func (p *OllamaProvider) Name() string {
	return "Ollama"
}

// Type 返回提供商类型
func (p *OllamaProvider) Type() config.ProviderType {
	return config.ProviderOllama
}

// PrepareRequest 准备 Ollama 格式的请求
func (p *OllamaProvider) PrepareRequest(req *models.OpenAIRequest) error {
	// Ollama 特定：存在工具时设置 tool_choice 为 required
	// 这强制模型使用工具而不是忽略它们
	if len(req.Tools) > 0 && req.ToolChoice == nil {
		req.ToolChoice = "required"
	}

	return nil
}

// AddHeaders 添加 Ollama 特定的 HTTP 头
func (p *OllamaProvider) AddHeaders(httpReq *http.Request) {
	// Ollama 是本地服务，只需要 Content-Type
	httpReq.Header.Set("Content-Type", "application/json")
	// 不设置 Authorization 头
}

// RequiresAuth 返回是否需要认证
func (p *OllamaProvider) RequiresAuth() bool {
	// Ollama 本地服务不需要认证
	return false
}

// HandleError 处理 Ollama 返回的错误
func (p *OllamaProvider) HandleError(statusCode int, body []byte) *errors.ProxyError {
	var errorBody map[string]interface{}
	if err := json.Unmarshal(body, &errorBody); err == nil {
		// Ollama 可能有自己的错误格式
		if errMsg, ok := errorBody["error"].(string); ok {
			return errors.NewAPIError(errMsg).WithProvider(p.Name())
		}
		return errors.FromOpenAIError(statusCode, errorBody).WithProvider(p.Name())
	}
	return errors.FromHTTPStatus(statusCode, string(body)).WithProvider(p.Name())
}

// SupportsReasoning 返回是否支持推理
func (p *OllamaProvider) SupportsReasoning() bool {
	// Ollama 本地模型通常不支持推理功能
	return false
}

// GetTimeout 返回请求超时时间（Ollama 本地模型可能较慢）
func (p *OllamaProvider) GetTimeout() int {
	return 180 // 3 分钟
}

// GetStreamTimeout 返回流式请求超时时间
func (p *OllamaProvider) GetStreamTimeout() int {
	return 600 // 10 分钟
}
