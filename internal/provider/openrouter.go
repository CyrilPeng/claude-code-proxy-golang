package provider

import (
	"net/http"

	"github.com/CyrilPeng/claude-code-proxy-golang/internal/config"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/errors"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/json"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/models"
)

// OpenRouterProvider 实现 OpenRouter 提供商
type OpenRouterProvider struct {
	*BaseProvider
}

// NewOpenRouterProvider 创建 OpenRouter 提供商
func NewOpenRouterProvider(cfg *config.Config) *OpenRouterProvider {
	return &OpenRouterProvider{
		BaseProvider: NewBaseProvider(cfg),
	}
}

// Name 返回提供商名称
func (p *OpenRouterProvider) Name() string {
	return "OpenRouter"
}

// Type 返回提供商类型
func (p *OpenRouterProvider) Type() config.ProviderType {
	return config.ProviderOpenRouter
}

// PrepareRequest 准备 OpenRouter 格式的请求
func (p *OpenRouterProvider) PrepareRequest(req *models.OpenAIRequest) error {
	// OpenRouter 特定：添加 reasoning 参数以支持思考功能
	if req.Reasoning == nil {
		req.Reasoning = map[string]interface{}{
			"enabled": true,
		}
	}

	// OpenRouter 特定：添加 usage 参数以获取 token 统计
	if req.Usage == nil {
		req.Usage = map[string]interface{}{
			"include": true,
		}
	}

	return nil
}

// AddHeaders 添加 OpenRouter 特定的 HTTP 头
func (p *OpenRouterProvider) AddHeaders(httpReq *http.Request) {
	cfg := p.Config()

	// 设置认证头
	httpReq.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	// OpenRouter 特定头
	if cfg.OpenRouterAppURL != "" {
		httpReq.Header.Set("HTTP-Referer", cfg.OpenRouterAppURL)
	}
	if cfg.OpenRouterAppName != "" {
		httpReq.Header.Set("X-Title", cfg.OpenRouterAppName)
	}
}

// RequiresAuth 返回是否需要认证
func (p *OpenRouterProvider) RequiresAuth() bool {
	return true
}

// HandleError 处理 OpenRouter 返回的错误
func (p *OpenRouterProvider) HandleError(statusCode int, body []byte) *errors.ProxyError {
	var errorBody map[string]interface{}
	if err := json.Unmarshal(body, &errorBody); err == nil {
		return errors.FromOpenAIError(statusCode, errorBody).WithProvider(p.Name())
	}
	return errors.FromHTTPStatus(statusCode, string(body)).WithProvider(p.Name())
}

// SupportsReasoning 返回是否支持推理
func (p *OpenRouterProvider) SupportsReasoning() bool {
	return true
}
