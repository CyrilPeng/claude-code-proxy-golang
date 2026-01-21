package provider

import (
	"net/http"

	"github.com/CyrilPeng/claude-code-proxy-golang/internal/config"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/errors"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/json"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/models"
)

// OpenAIProvider 实现 OpenAI Direct 提供商
type OpenAIProvider struct {
	*BaseProvider
}

// NewOpenAIProvider 创建 OpenAI Direct 提供商
func NewOpenAIProvider(cfg *config.Config) *OpenAIProvider {
	return &OpenAIProvider{
		BaseProvider: NewBaseProvider(cfg),
	}
}

// Name 返回提供商名称
func (p *OpenAIProvider) Name() string {
	return "OpenAI"
}

// Type 返回提供商类型
func (p *OpenAIProvider) Type() config.ProviderType {
	return config.ProviderOpenAI
}

// PrepareRequest 准备 OpenAI 格式的请求
func (p *OpenAIProvider) PrepareRequest(req *models.OpenAIRequest) error {
	// OpenAI 特定：为推理模型添加 reasoning_effort 参数
	// GPT-5、o1、o3 等模型支持此参数
	if req.ReasoningEffort == "" {
		req.ReasoningEffort = "medium"
	}

	return nil
}

// AddHeaders 添加 OpenAI 特定的 HTTP 头
func (p *OpenAIProvider) AddHeaders(httpReq *http.Request) {
	cfg := p.Config()

	httpReq.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")
}

// RequiresAuth 返回是否需要认证
func (p *OpenAIProvider) RequiresAuth() bool {
	return true
}

// HandleError 处理 OpenAI 返回的错误
func (p *OpenAIProvider) HandleError(statusCode int, body []byte) *errors.ProxyError {
	var errorBody map[string]interface{}
	if err := json.Unmarshal(body, &errorBody); err == nil {
		return errors.FromOpenAIError(statusCode, errorBody).WithProvider(p.Name())
	}
	return errors.FromHTTPStatus(statusCode, string(body)).WithProvider(p.Name())
}

// SupportsReasoning 返回是否支持推理
func (p *OpenAIProvider) SupportsReasoning() bool {
	return true
}
