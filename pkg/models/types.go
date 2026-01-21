package models

// ClaudeMessage 表示 Claude API 格式的消息
type ClaudeMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // 可以是字符串或 []ContentBlock
}

// ContentBlock 表示 Claude 格式的内容块
type ContentBlock struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	Thinking  string      `json:"thinking,omitempty"`  // 用于思考块
	Signature *string     `json:"signature,omitempty"` // 思考块隐藏所必需（使用指针以包含空字符串）
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Input     interface{} `json:"input,omitempty"`
}

// ClaudeRequest 表示完整的 Claude API 请求
type ClaudeRequest struct {
	Model         string          `json:"model"`
	Messages      []ClaudeMessage `json:"messages"`
	MaxTokens     int             `json:"max_tokens"`
	Temperature   *float64        `json:"temperature,omitempty"`
	TopP          *float64        `json:"top_p,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Stream        *bool           `json:"stream,omitempty"`
	System        interface{}     `json:"system,omitempty"` // 可以是字符串或内容块数组
	Tools         []Tool          `json:"tools,omitempty"`
}

// Tool 表示函数/工具定义
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

// OpenAIMessage 表示 OpenAI 格式的消息
type OpenAIMessage struct {
	Role             string           `json:"role"`
	Content          interface{}      `json:"content,omitempty"` // 字符串或 null
	ToolCalls        []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
	ReasoningDetails []interface{}    `json:"reasoning_details,omitempty"` // OpenRouter 推理
}

// OpenAIToolCall 表示 OpenAI 格式的工具调用
type OpenAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// OpenAIRequest 表示完整的 OpenAI API 请求
type OpenAIRequest struct {
	Model               string                 `json:"model"`
	Messages            []OpenAIMessage        `json:"messages"`
	MaxTokens           int                    `json:"max_tokens,omitempty"`
	MaxCompletionTokens int                    `json:"max_completion_tokens,omitempty"`
	Temperature         *float64               `json:"temperature,omitempty"`
	TopP                *float64               `json:"top_p,omitempty"`
	Stop                []string               `json:"stop,omitempty"`
	Stream              *bool                  `json:"stream,omitempty"`
	StreamOptions       map[string]interface{} `json:"stream_options,omitempty"`   // OpenAI 标准
	Usage               map[string]interface{} `json:"usage,omitempty"`            // OpenRouter
	Reasoning           map[string]interface{} `json:"reasoning,omitempty"`        // OpenRouter 推理令牌
	ReasoningEffort     string                 `json:"reasoning_effort,omitempty"` // OpenAI 聊天完成推理（GPT-5 模型）
	Tools               []OpenAITool           `json:"tools,omitempty"`
	ToolChoice          interface{}            `json:"tool_choice,omitempty"` // 强制使用工具："auto"、"required" 或特定工具
}

// OpenAITool 表示 OpenAI 格式的工具
type OpenAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Parameters  interface{} `json:"parameters"`
	} `json:"function"`
}

// ClaudeResponse 表示 Claude API 响应
type ClaudeResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   *string        `json:"stop_reason"`
	StopSequence *string        `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

// Usage 表示令牌使用信息
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// OpenAIResponse 表示 OpenAI API 响应
type OpenAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   OpenAIUsage    `json:"usage"`
}

// OpenAIChoice 表示 OpenAI 响应中的选择
type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason *string       `json:"finish_reason"`
}

// OpenAIUsage 表示 OpenAI 格式的令牌使用量
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
