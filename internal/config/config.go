// Package config 处理从环境变量和 .env 文件加载配置。
//
// 它支持多个配置文件位置（./.env、~/.claude/proxy.env、~/.claude-code-proxy），
// 并根据 OPENAI_BASE_URL 检测提供商类型（OpenRouter、OpenAI、Ollama）。
// 该包还处理模型覆盖，用于将 Claude 模型名称路由到替代提供商。
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

// ProviderType 表示后端提供商类型
type ProviderType string

const (
	ProviderOpenRouter ProviderType = "openrouter"
	ProviderOpenAI     ProviderType = "openai"
	ProviderOllama     ProviderType = "ollama"
	ProviderUnknown    ProviderType = "unknown"
)

// CacheKey 唯一标识用于能力缓存的（提供商，模型）组合
// 使用结构体作为 map 键提供类型安全性和零冲突风险
type CacheKey struct {
	BaseURL string // 提供商基础 URL（例如 "https://openrouter.ai/api/v1"）
	Model   string // 模型名称（例如 "gpt-5"、"openai/gpt-5"）
}

// ModelCapabilities 跟踪特定模型支持的参数
// 这是通过自适应重试机制动态学习的
type ModelCapabilities struct {
	UsesMaxCompletionTokens bool      // 此模型是否使用 max_completion_tokens？
	LastChecked             time.Time // 上次验证时间
}

// 全局能力缓存（(baseURL, model) -> capabilities）
// 由互斥锁保护，用于跨并发请求的线程安全访问
var (
	modelCapabilityCache = make(map[CacheKey]*ModelCapabilities)
	capabilityCacheMutex sync.RWMutex
)

// Config 保存所有代理配置
type Config struct {
	// 必需
	OpenAIAPIKey string

	// 可选
	OpenAIBaseURL   string
	AnthropicAPIKey string

	// 模型路由（如果未设置则基于模式）
	OpusModel   string
	SonnetModel string
	HaikuModel  string

	// 服务器设置
	Host string
	Port string

	// 调试日志
	Debug bool

	// 简单日志 - 每个请求一行摘要
	SimpleLog bool

	// 直通模式 - 直接代理到 Anthropic 而不进行转换
	PassthroughMode bool

	// OpenRouter 特定（可选，改善速率限制）
	OpenRouterAppName string
	OpenRouterAppURL  string
}

// Load 从环境变量读取配置
// 尝试多个位置：./.env、~/.claude/proxy.env、~/.claude-code-proxy
func Load() (*Config, error) {
	// 按优先级顺序尝试加载 .env 文件
	locations := []string{
		".env",
		filepath.Join(os.Getenv("HOME"), ".claude", "proxy.env"),
		filepath.Join(os.Getenv("HOME"), ".claude-code-proxy"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			// 文件存在，加载它（overload 以覆盖现有环境变量）
			if err := godotenv.Overload(loc); err == nil {
				fmt.Printf("📁 已从以下位置加载配置: %s\n", loc)
				break
			}
		}
	}

	// 从环境构建配置
	cfg := &Config{
		OpenAIAPIKey:    os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:   getEnvOrDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),

		// 基于模式的路由（可选覆盖）
		OpusModel:   os.Getenv("ANTHROPIC_DEFAULT_OPUS_MODEL"),
		SonnetModel: os.Getenv("ANTHROPIC_DEFAULT_SONNET_MODEL"),
		HaikuModel:  os.Getenv("ANTHROPIC_DEFAULT_HAIKU_MODEL"),

		// 服务器设置
		Host: getEnvOrDefault("HOST", "0.0.0.0"),
		Port: getEnvOrDefault("PORT", "8082"),

		// 直通模式
		PassthroughMode: getEnvAsBoolOrDefault("PASSTHROUGH_MODE", false),

		// OpenRouter 特定（可选）
		OpenRouterAppName: os.Getenv("OPENROUTER_APP_NAME"),
		OpenRouterAppURL:  os.Getenv("OPENROUTER_APP_URL"),
	}

	// 验证必需字段
	// 允许 Ollama（localhost 端点）缺少 API 密钥
	if cfg.OpenAIAPIKey == "" {
		if !strings.Contains(cfg.OpenAIBaseURL, "localhost") &&
			!strings.Contains(cfg.OpenAIBaseURL, "127.0.0.1") {
			return nil, fmt.Errorf("OPENAI_API_KEY 是必需的（除非使用 localhost/Ollama）")
		}
		// 为 Ollama 设置虚拟密钥
		cfg.OpenAIAPIKey = "ollama"
	}

	return cfg, nil
}

// LoadWithDebug 加载配置并设置调试模式
func LoadWithDebug(debug bool) (*Config, error) {
	cfg, err := Load()
	if err != nil {
		return nil, err
	}
	cfg.Debug = debug
	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1" || value == "yes"
	}
	return defaultValue
}

// DetectProvider 根据基础 URL 识别提供商类型
func (c *Config) DetectProvider() ProviderType {
	baseURL := strings.ToLower(c.OpenAIBaseURL)

	if strings.Contains(baseURL, "openrouter.ai") {
		return ProviderOpenRouter
	}
	if strings.Contains(baseURL, "api.openai.com") {
		return ProviderOpenAI
	}
	if strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1") {
		return ProviderOllama
	}
	return ProviderUnknown
}

// IsLocalhost 如果基础 URL 指向 localhost 则返回 true
func (c *Config) IsLocalhost() bool {
	baseURL := strings.ToLower(c.OpenAIBaseURL)
	return strings.Contains(baseURL, "localhost") || strings.Contains(baseURL, "127.0.0.1")
}


// GetModelCapabilities 检索（提供商，模型）组合的缓存能力。
// 如果尚未缓存任何能力（此模型的首次请求），则返回 nil。
// 使用读锁保证线程安全。
func GetModelCapabilities(key CacheKey) *ModelCapabilities {
	capabilityCacheMutex.RLock()
	defer capabilityCacheMutex.RUnlock()
	return modelCapabilityCache[key]
}

// SetModelCapabilities 缓存（提供商，模型）组合的能力。
// 在通过自适应重试检测到特定模型支持哪些参数后调用。
// 使用写锁保证线程安全。
func SetModelCapabilities(key CacheKey, capabilities *ModelCapabilities) {
	capabilityCacheMutex.Lock()
	defer capabilityCacheMutex.Unlock()
	capabilities.LastChecked = time.Now()
	modelCapabilityCache[key] = capabilities
}

// ShouldUseMaxCompletionTokens 根据通过自适应检测学习到的缓存模型能力，
// 确定是否应发送 max_completion_tokens。
// 没有硬编码的模型模式 - 首次请求时对所有模型都尝试 max_completion_tokens。
func (c *Config) ShouldUseMaxCompletionTokens(modelName string) bool {
	// 为此（提供商，模型）组合构建缓存键
	key := CacheKey{
		BaseURL: c.OpenAIBaseURL,
		Model:   modelName,
	}

	// 检查是否有关于此特定模型的缓存知识
	caps := GetModelCapabilities(key)
	if caps != nil {
		// 缓存命中 - 使用已学习的能力
		if c.Debug {
			fmt.Printf("[调试] 缓存命中: %s → max_completion_tokens=%v\n",
				modelName, caps.UsesMaxCompletionTokens)
		}
		return caps.UsesMaxCompletionTokens
	}

	// 缓存未命中 - 默认首先尝试 max_completion_tokens
	// handlers.go 中的重试机制将检测是否不支持
	// 并自动回退到 max_tokens，然后缓存结果
	if c.Debug {
		fmt.Printf("[调试] 缓存未命中: %s → 将自动检测（尝试 max_completion_tokens）\n", modelName)
	}
	return true
}

// ValidationError 表示配置验证错误
type ValidationError struct {
	Field   string // 出错的字段名
	Message string // 错误描述
}

// Error 实现 error 接口
func (e *ValidationError) Error() string {
	return fmt.Sprintf("配置验证错误 [%s]: %s", e.Field, e.Message)
}

// ValidationErrors 表示多个验证错误
type ValidationErrors []ValidationError

// Error 实现 error 接口
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	var msgs []string
	for _, err := range e {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("配置验证失败，共 %d 个错误:\n  - %s", len(e), strings.Join(msgs, "\n  - "))
}

// Validate 验证配置的有效性
// 返回 nil 表示配置有效，否则返回 ValidationErrors
func (c *Config) Validate() error {
	var errs ValidationErrors

	// 验证 OpenAI Base URL
	if c.OpenAIBaseURL == "" {
		errs = append(errs, ValidationError{
			Field:   "OPENAI_BASE_URL",
			Message: "不能为空",
		})
	} else {
		// 验证 URL 格式
		parsedURL, err := url.Parse(c.OpenAIBaseURL)
		if err != nil {
			errs = append(errs, ValidationError{
				Field:   "OPENAI_BASE_URL",
				Message: fmt.Sprintf("URL 格式无效: %v", err),
			})
		} else {
			// 验证 scheme
			if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
				errs = append(errs, ValidationError{
					Field:   "OPENAI_BASE_URL",
					Message: fmt.Sprintf("URL scheme 必须是 http 或 https，当前为: %s", parsedURL.Scheme),
				})
			}
			// 验证 host
			if parsedURL.Host == "" {
				errs = append(errs, ValidationError{
					Field:   "OPENAI_BASE_URL",
					Message: "URL 缺少主机名",
				})
			}
		}
	}

	// 验证 API Key（非本地环境必需）
	if c.OpenAIAPIKey == "" {
		if !c.IsLocalhost() {
			errs = append(errs, ValidationError{
				Field:   "OPENAI_API_KEY",
				Message: "非本地环境必须设置 API 密钥",
			})
		}
	}

	// 验证端口号
	if c.Port != "" {
		var port int
		if _, err := fmt.Sscanf(c.Port, "%d", &port); err != nil {
			errs = append(errs, ValidationError{
				Field:   "PORT",
				Message: fmt.Sprintf("端口号格式无效: %s", c.Port),
			})
		} else if port < 1 || port > 65535 {
			errs = append(errs, ValidationError{
				Field:   "PORT",
				Message: fmt.Sprintf("端口号必须在 1-65535 之间，当前为: %d", port),
			})
		}
	}

	// 验证 OpenRouter 特定配置
	if c.DetectProvider() == ProviderOpenRouter {
		// OpenRouter App URL 应该是有效的 URL（如果设置了的话）
		if c.OpenRouterAppURL != "" {
			if _, err := url.Parse(c.OpenRouterAppURL); err != nil {
				errs = append(errs, ValidationError{
					Field:   "OPENROUTER_APP_URL",
					Message: fmt.Sprintf("URL 格式无效: %v", err),
				})
			}
		}
	}

	// 验证模型配置（警告级别，不阻止启动）
	// 这里只做格式检查，不验证模型是否存在

	if len(errs) > 0 {
		return errs
	}
	return nil
}

// MustValidate 验证配置，如果无效则 panic
// 用于程序启动时的配置检查
func (c *Config) MustValidate() {
	if err := c.Validate(); err != nil {
		panic(fmt.Sprintf("配置验证失败: %v", err))
	}
}

// ValidateWithWarnings 验证配置并返回警告信息
// 返回 (errors, warnings)
func (c *Config) ValidateWithWarnings() (error, []string) {
	err := c.Validate()
	var warnings []string

	// 添加警告（不阻止启动但建议修复的问题）
	if c.OpenRouterAppName == "" && c.DetectProvider() == ProviderOpenRouter {
		warnings = append(warnings, "建议设置 OPENROUTER_APP_NAME 以获得更好的速率限制")
	}
	if c.OpenRouterAppURL == "" && c.DetectProvider() == ProviderOpenRouter {
		warnings = append(warnings, "建议设置 OPENROUTER_APP_URL 以获得更好的速率限制")
	}
	if c.AnthropicAPIKey == "" {
		warnings = append(warnings, "未设置 ANTHROPIC_API_KEY，将不验证入站请求的 API 密钥")
	}

	return err, warnings
}
