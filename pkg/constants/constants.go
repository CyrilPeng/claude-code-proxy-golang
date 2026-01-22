// Package constants 定义项目中使用的所有常量。
// 将魔法字符串集中管理，提高代码可维护性和一致性。
package constants

// 提供商 URL 标识符
const (
	// ProviderURLOpenRouter OpenRouter API 的 URL 标识符
	ProviderURLOpenRouter = "openrouter.ai"
	// ProviderURLOpenAI OpenAI API 的 URL 标识符
	ProviderURLOpenAI = "api.openai.com"
	// ProviderURLLocalhost 本地主机标识符
	ProviderURLLocalhost = "localhost"
	// ProviderURLLoopback 回环地址标识符
	ProviderURLLoopback = "127.0.0.1"
)

// 内容块类型
const (
	// ContentTypeText 文本内容块
	ContentTypeText = "text"
	// ContentTypeThinking 思考内容块
	ContentTypeThinking = "thinking"
	// ContentTypeToolUse 工具使用内容块
	ContentTypeToolUse = "tool_use"
	// ContentTypeToolResult 工具结果内容块
	ContentTypeToolResult = "tool_result"
)

// 消息角色
const (
	// RoleSystem 系统消息
	RoleSystem = "system"
	// RoleUser 用户消息
	RoleUser = "user"
	// RoleAssistant 助手消息
	RoleAssistant = "assistant"
	// RoleTool 工具消息（OpenAI 格式）
	RoleTool = "tool"
)

// 停止原因（Claude 格式）
const (
	// StopReasonEndTurn 正常结束
	StopReasonEndTurn = "end_turn"
	// StopReasonToolUse 工具调用
	StopReasonToolUse = "tool_use"
	// StopReasonMaxTokens 达到最大令牌数
	StopReasonMaxTokens = "max_tokens"
)

// 完成原因（OpenAI 格式）
const (
	// FinishReasonStop 正常停止
	FinishReasonStop = "stop"
	// FinishReasonLength 达到长度限制
	FinishReasonLength = "length"
	// FinishReasonToolCalls 工具调用
	FinishReasonToolCalls = "tool_calls"
	// FinishReasonFunctionCall 函数调用（旧版）
	FinishReasonFunctionCall = "function_call"
	// FinishReasonContentFilter 内容过滤
	FinishReasonContentFilter = "content_filter"
)

// SSE 事件类型
const (
	// EventMessageStart 消息开始事件
	EventMessageStart = "message_start"
	// EventMessageDelta 消息增量事件
	EventMessageDelta = "message_delta"
	// EventMessageStop 消息停止事件
	EventMessageStop = "message_stop"
	// EventContentBlockStart 内容块开始事件
	EventContentBlockStart = "content_block_start"
	// EventContentBlockDelta 内容块增量事件
	EventContentBlockDelta = "content_block_delta"
	// EventContentBlockStop 内容块停止事件
	EventContentBlockStop = "content_block_stop"
	// EventPing ping 事件
	EventPing = "ping"
	// EventError 错误事件
	EventError = "error"
)

// Delta 类型
const (
	// DeltaTypeTextDelta 文本增量
	DeltaTypeTextDelta = "text_delta"
	// DeltaTypeThinkingDelta 思考增量
	DeltaTypeThinkingDelta = "thinking_delta"
	// DeltaTypeInputJSONDelta 工具输入 JSON 增量
	DeltaTypeInputJSONDelta = "input_json_delta"
)

// 推理详情类型（OpenRouter 格式）
const (
	// ReasoningTypeText 推理文本
	ReasoningTypeText = "reasoning.text"
	// ReasoningTypeSummary 推理摘要
	ReasoningTypeSummary = "reasoning.summary"
	// ReasoningTypeEncrypted 加密推理
	ReasoningTypeEncrypted = "reasoning.encrypted"
)

// API 端点路径
const (
	// EndpointChatCompletions OpenAI 聊天完成端点
	EndpointChatCompletions = "/chat/completions"
	// EndpointMessages Claude 消息端点
	EndpointMessages = "/v1/messages"
	// EndpointCountTokens 令牌计数端点
	EndpointCountTokens = "/v1/messages/count_tokens"
	// EndpointHealth 健康检查端点
	EndpointHealth = "/health"
)

// HTTP 头名称
const (
	// HeaderContentType Content-Type 头
	HeaderContentType = "Content-Type"
	// HeaderAuthorization Authorization 头
	HeaderAuthorization = "Authorization"
	// HeaderXAPIKey X-API-Key 头（Claude 格式）
	HeaderXAPIKey = "x-api-key"
	// HeaderHTTPReferer HTTP-Referer 头（OpenRouter）
	HeaderHTTPReferer = "HTTP-Referer"
	// HeaderXTitle X-Title 头（OpenRouter）
	HeaderXTitle = "X-Title"
	// HeaderCacheControl Cache-Control 头
	HeaderCacheControl = "Cache-Control"
	// HeaderConnection Connection 头
	HeaderConnection = "Connection"
	// HeaderXAccelBuffering X-Accel-Buffering 头
	HeaderXAccelBuffering = "X-Accel-Buffering"
)

// MIME 类型值
const (
	// MIMETypeJSON JSON 内容类型
	MIMETypeJSON = "application/json"
	// MIMETypeSSE SSE 内容类型
	MIMETypeSSE = "text/event-stream"
)

// 工具类型
const (
	// ToolTypeFunction 函数类型工具
	ToolTypeFunction = "function"
	// ToolIDPrefix 工具调用 ID 前缀
	ToolIDPrefix = "toolu_"
)

// 消息类型
const (
	// MessageTypeMessage 消息类型
	MessageTypeMessage = "message"
	// MessageTypeError 错误类型
	MessageTypeError = "error"
)

// 错误类型
const (
	// ErrorTypeInvalidRequest 无效请求错误
	ErrorTypeInvalidRequest = "invalid_request_error"
	// ErrorTypeAuthentication 认证错误
	ErrorTypeAuthentication = "authentication_error"
	// ErrorTypeAPIError API 错误
	ErrorTypeAPIError = "api_error"
)

// 工具选择模式
const (
	// ToolChoiceAuto 自动选择
	ToolChoiceAuto = "auto"
	// ToolChoiceRequired 强制使用
	ToolChoiceRequired = "required"
	// ToolChoiceNone 不使用
	ToolChoiceNone = "none"
)

// 推理努力级别（OpenAI GPT-5）
const (
	// ReasoningEffortMinimal 最小努力
	ReasoningEffortMinimal = "minimal"
	// ReasoningEffortLow 低努力
	ReasoningEffortLow = "low"
	// ReasoningEffortMedium 中等努力
	ReasoningEffortMedium = "medium"
	// ReasoningEffortHigh 高努力
	ReasoningEffortHigh = "high"
)
