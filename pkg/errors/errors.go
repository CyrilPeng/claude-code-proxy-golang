// Package errors 提供结构化的错误类型和错误处理机制。
// 支持 Claude API 和 OpenAI API 的错误格式转换。
package errors

import (
	"fmt"
	"net/http"
)

// ErrorType 定义错误类型常量
type ErrorType string

const (
	// Claude API 错误类型
	ErrorTypeInvalidRequest   ErrorType = "invalid_request_error"
	ErrorTypeAuthentication   ErrorType = "authentication_error"
	ErrorTypePermission       ErrorType = "permission_error"
	ErrorTypeNotFound         ErrorType = "not_found_error"
	ErrorTypeRateLimit        ErrorType = "rate_limit_error"
	ErrorTypeAPI              ErrorType = "api_error"
	ErrorTypeOverloaded       ErrorType = "overloaded_error"
	ErrorTypeTimeout          ErrorType = "timeout_error"
	ErrorTypeConnection       ErrorType = "connection_error"
	ErrorTypeConversion       ErrorType = "conversion_error"
	ErrorTypeStreamProcessing ErrorType = "stream_processing_error"
)

// ProxyError 是代理的统一错误类型
type ProxyError struct {
	Type       ErrorType `json:"type"`
	Message    string    `json:"message"`
	StatusCode int       `json:"-"` // HTTP 状态码，不序列化到 JSON
	Cause      error     `json:"-"` // 原始错误，不序列化到 JSON
	Provider   string    `json:"-"` // 提供商名称（用于日志）
	Model      string    `json:"-"` // 模型名称（用于日志）
}

// Error 实现 error 接口
func (e *ProxyError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap 支持 errors.Unwrap
func (e *ProxyError) Unwrap() error {
	return e.Cause
}

// ToClaudeError 转换为 Claude API 错误响应格式
func (e *ProxyError) ToClaudeError() map[string]interface{} {
	return map[string]interface{}{
		"type": "error",
		"error": map[string]interface{}{
			"type":    string(e.Type),
			"message": e.Message,
		},
	}
}

// WithCause 添加原始错误
func (e *ProxyError) WithCause(cause error) *ProxyError {
	e.Cause = cause
	return e
}

// WithProvider 添加提供商信息
func (e *ProxyError) WithProvider(provider string) *ProxyError {
	e.Provider = provider
	return e
}

// WithModel 添加模型信息
func (e *ProxyError) WithModel(model string) *ProxyError {
	e.Model = model
	return e
}

// 错误构造函数

// NewInvalidRequestError 创建无效请求错误
func NewInvalidRequestError(message string) *ProxyError {
	return &ProxyError{
		Type:       ErrorTypeInvalidRequest,
		Message:    message,
		StatusCode: http.StatusBadRequest,
	}
}

// NewAuthenticationError 创建认证错误
func NewAuthenticationError(message string) *ProxyError {
	return &ProxyError{
		Type:       ErrorTypeAuthentication,
		Message:    message,
		StatusCode: http.StatusUnauthorized,
	}
}

// NewPermissionError 创建权限错误
func NewPermissionError(message string) *ProxyError {
	return &ProxyError{
		Type:       ErrorTypePermission,
		Message:    message,
		StatusCode: http.StatusForbidden,
	}
}

// NewNotFoundError 创建资源未找到错误
func NewNotFoundError(message string) *ProxyError {
	return &ProxyError{
		Type:       ErrorTypeNotFound,
		Message:    message,
		StatusCode: http.StatusNotFound,
	}
}

// NewRateLimitError 创建速率限制错误
func NewRateLimitError(message string) *ProxyError {
	return &ProxyError{
		Type:       ErrorTypeRateLimit,
		Message:    message,
		StatusCode: http.StatusTooManyRequests,
	}
}

// NewAPIError 创建 API 错误
func NewAPIError(message string) *ProxyError {
	return &ProxyError{
		Type:       ErrorTypeAPI,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

// NewOverloadedError 创建过载错误
func NewOverloadedError(message string) *ProxyError {
	return &ProxyError{
		Type:       ErrorTypeOverloaded,
		Message:    message,
		StatusCode: http.StatusServiceUnavailable,
	}
}

// NewTimeoutError 创建超时错误
func NewTimeoutError(message string) *ProxyError {
	return &ProxyError{
		Type:       ErrorTypeTimeout,
		Message:    message,
		StatusCode: http.StatusGatewayTimeout,
	}
}

// NewConnectionError 创建连接错误
func NewConnectionError(message string) *ProxyError {
	return &ProxyError{
		Type:       ErrorTypeConnection,
		Message:    message,
		StatusCode: http.StatusBadGateway,
	}
}

// NewConversionError 创建转换错误
func NewConversionError(message string) *ProxyError {
	return &ProxyError{
		Type:       ErrorTypeConversion,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

// NewStreamProcessingError 创建流处理错误
func NewStreamProcessingError(message string) *ProxyError {
	return &ProxyError{
		Type:       ErrorTypeStreamProcessing,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

// FromHTTPStatus 根据 HTTP 状态码创建适当的错误
func FromHTTPStatus(statusCode int, message string) *ProxyError {
	switch statusCode {
	case http.StatusBadRequest:
		return NewInvalidRequestError(message)
	case http.StatusUnauthorized:
		return NewAuthenticationError(message)
	case http.StatusForbidden:
		return NewPermissionError(message)
	case http.StatusNotFound:
		return NewNotFoundError(message)
	case http.StatusTooManyRequests:
		return NewRateLimitError(message)
	case http.StatusServiceUnavailable:
		return NewOverloadedError(message)
	case http.StatusGatewayTimeout:
		return NewTimeoutError(message)
	case http.StatusBadGateway:
		return NewConnectionError(message)
	default:
		return NewAPIError(message)
	}
}

// FromOpenAIError 从 OpenAI API 错误响应创建 ProxyError
func FromOpenAIError(statusCode int, errorBody map[string]interface{}) *ProxyError {
	message := "Unknown error"
	errorType := ErrorTypeAPI

	// 尝试从 OpenAI 错误格式中提取信息
	if errObj, ok := errorBody["error"].(map[string]interface{}); ok {
		if msg, ok := errObj["message"].(string); ok {
			message = msg
		}
		if typ, ok := errObj["type"].(string); ok {
			// 映射 OpenAI 错误类型到 Claude 错误类型
			switch typ {
			case "invalid_request_error":
				errorType = ErrorTypeInvalidRequest
			case "authentication_error", "invalid_api_key":
				errorType = ErrorTypeAuthentication
			case "permission_denied":
				errorType = ErrorTypePermission
			case "not_found":
				errorType = ErrorTypeNotFound
			case "rate_limit_exceeded":
				errorType = ErrorTypeRateLimit
			case "server_error", "internal_error":
				errorType = ErrorTypeAPI
			case "overloaded":
				errorType = ErrorTypeOverloaded
			default:
				errorType = ErrorTypeAPI
			}
		}
	}

	return &ProxyError{
		Type:       errorType,
		Message:    message,
		StatusCode: statusCode,
	}
}

// IsRetryable 判断错误是否可重试
func (e *ProxyError) IsRetryable() bool {
	switch e.Type {
	case ErrorTypeRateLimit, ErrorTypeOverloaded, ErrorTypeTimeout, ErrorTypeConnection:
		return true
	default:
		return false
	}
}

// IsClientError 判断是否是客户端错误（4xx）
func (e *ProxyError) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// IsServerError 判断是否是服务器错误（5xx）
func (e *ProxyError) IsServerError() bool {
	return e.StatusCode >= 500
}

// Wrap 包装一个错误为 ProxyError
func Wrap(err error, message string) *ProxyError {
	if err == nil {
		return nil
	}

	// 如果已经是 ProxyError，添加额外信息
	if pe, ok := err.(*ProxyError); ok {
		pe.Message = message + ": " + pe.Message
		return pe
	}

	// 创建新的 ProxyError
	return NewAPIError(message).WithCause(err)
}

// WrapWithType 包装一个错误为指定类型的 ProxyError
func WrapWithType(err error, errType ErrorType, message string) *ProxyError {
	if err == nil {
		return nil
	}

	pe := &ProxyError{
		Type:    errType,
		Message: message,
		Cause:   err,
	}

	// 设置适当的状态码
	switch errType {
	case ErrorTypeInvalidRequest:
		pe.StatusCode = http.StatusBadRequest
	case ErrorTypeAuthentication:
		pe.StatusCode = http.StatusUnauthorized
	case ErrorTypePermission:
		pe.StatusCode = http.StatusForbidden
	case ErrorTypeNotFound:
		pe.StatusCode = http.StatusNotFound
	case ErrorTypeRateLimit:
		pe.StatusCode = http.StatusTooManyRequests
	case ErrorTypeOverloaded:
		pe.StatusCode = http.StatusServiceUnavailable
	case ErrorTypeTimeout:
		pe.StatusCode = http.StatusGatewayTimeout
	case ErrorTypeConnection:
		pe.StatusCode = http.StatusBadGateway
	default:
		pe.StatusCode = http.StatusInternalServerError
	}

	return pe
}
