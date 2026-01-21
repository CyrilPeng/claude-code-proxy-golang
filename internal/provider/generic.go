package provider

import (
	"net/http"

	"github.com/CyrilPeng/claude-code-proxy-golang/internal/config"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/errors"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/json"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/models"
)

// GenericProvider 实现通用/未知提供商
// 用于处理未明确识别的 OpenAI 兼容 API
type GenericProvider struct {
	*BaseProvider
}

// NewGenericProvider 创建通用提供商
func NewGenericProvider(cfg *config.Config) *GenericProvider {
	return &GenericProvider{
		BaseProvider: NewBaseProvider(cfg),
	}
}

// Name 返回提供商名称
func (p *GenericProvider) Name() string {
	return "Generic"
}

// Type 返回提供商类型
func (p *GenericProvider) Type() config.ProviderType {
	return config.ProviderUnknown
}

// PrepareRequest 准备通用格式的请求
func (p *GenericProvider) PrepareRequest(req *models.OpenAIRequest) error {
	// 通用提供商不添加任何特定参数
	// 依赖自适应能力检测机制
	return nil
}

// AddHeaders 添加通用 HTTP 头
func (p *GenericProvider) AddHeaders(httpReq *http.Request) {
	cfg := p.Config()

	httpReq.Header.Set("Content-Type", "application/json")

	// 如果配置了 API 密钥且不是本地服务，添加认证头
	if cfg.OpenAIAPIKey != "" && !cfg.IsLocalhost() {
		httpReq.Header.Set("Authorization", "Bearer "+cfg.OpenAIAPIKey)
	}
}

// RequiresAuth 返回是否需要认证
func (p *GenericProvider) RequiresAuth() bool {
	// 根据是否是本地服务决定
	return !p.Config().IsLocalhost()
}

// HandleError 处理通用错误
func (p *GenericProvider) HandleError(statusCode int, body []byte) *errors.ProxyError {
	var errorBody map[string]interface{}
	if err := json.Unmarshal(body, &errorBody); err == nil {
		return errors.FromOpenAIError(statusCode, errorBody).WithProvider(p.Name())
	}
	return errors.FromHTTPStatus(statusCode, string(body)).WithProvider(p.Name())
}

// SupportsReasoning 返回是否支持推理
func (p *GenericProvider) SupportsReasoning() bool {
	// 通用提供商默认不假设支持推理
	return false
}
