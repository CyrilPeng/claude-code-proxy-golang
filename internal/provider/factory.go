package provider

import (
	"github.com/CyrilPeng/claude-code-proxy-golang/internal/config"
)

// New 根据配置创建适当的提供商实例
// 这是创建提供商的推荐方式
func New(cfg *config.Config) Provider {
	return FromType(cfg.DetectProvider(), cfg)
}

// FromType 根据提供商类型创建提供商实例
func FromType(providerType config.ProviderType, cfg *config.Config) Provider {
	switch providerType {
	case config.ProviderOpenRouter:
		return NewOpenRouterProvider(cfg)
	case config.ProviderOpenAI:
		return NewOpenAIProvider(cfg)
	case config.ProviderOllama:
		return NewOllamaProvider(cfg)
	default:
		return NewGenericProvider(cfg)
	}
}

// Registry 提供商注册表，用于管理多个提供商实例
type Registry struct {
	providers map[config.ProviderType]Provider
	cfg       *config.Config
}

// NewRegistry 创建提供商注册表
func NewRegistry(cfg *config.Config) *Registry {
	return &Registry{
		providers: make(map[config.ProviderType]Provider),
		cfg:       cfg,
	}
}

// Get 获取指定类型的提供商，如果不存在则创建
func (r *Registry) Get(providerType config.ProviderType) Provider {
	if p, exists := r.providers[providerType]; exists {
		return p
	}

	p := FromType(providerType, r.cfg)
	r.providers[providerType] = p
	return p
}

// GetCurrent 获取当前配置的提供商
func (r *Registry) GetCurrent() Provider {
	return r.Get(r.cfg.DetectProvider())
}

// Register 注册自定义提供商
func (r *Registry) Register(providerType config.ProviderType, provider Provider) {
	r.providers[providerType] = provider
}

// All 返回所有已注册的提供商
func (r *Registry) All() map[config.ProviderType]Provider {
	return r.providers
}
