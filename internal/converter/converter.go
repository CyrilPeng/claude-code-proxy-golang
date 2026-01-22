// Package converter 处理 Claude 和 OpenAI API 格式之间的双向转换。
//
// 提供将 Claude API 请求转换为 OpenAI 兼容格式的函数，以及将 OpenAI 响应转换回 Claude 格式的函数。
// 包括模型映射、消息结构转换、工具调用处理，以及从推理响应中提取思考块。
package converter

import (
	"fmt"
	"strings"
	"time"

	"github.com/CyrilPeng/claude-code-proxy-golang/internal/config"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/constants"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/json"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/models"
)

// 默认模型映射（当环境变量未设置时使用）
// 可通过以下环境变量覆盖：
//   - ANTHROPIC_DEFAULT_OPUS_MODEL
//   - ANTHROPIC_DEFAULT_SONNET_MODEL
//   - ANTHROPIC_DEFAULT_HAIKU_MODEL
const (
	DefaultOpusModel   = "google/gemini-3-pro-preview"
	DefaultSonnetModel = "google/gemini-3-flash-preview"
	DefaultHaikuModel  = "google/gemini-2.5-pro"
)

// GenerateToolID 生成唯一的工具调用 ID
// 可选的 index 参数用于在同一时间戳内区分多个工具调用
func GenerateToolID(index ...int) string {
	if len(index) > 0 {
		return fmt.Sprintf("%s%d_%d", constants.ToolIDPrefix, time.Now().UnixNano(), index[0])
	}
	return fmt.Sprintf("%s%d", constants.ToolIDPrefix, time.Now().UnixNano())
}

// extractSystemText 从 Claude 的灵活系统参数中提取系统文本。
// Claude 支持字符串格式 ("system": "text") 和包含内容块的数组格式。
// 此函数将两种格式标准化为单个字符串，以兼容 OpenAI。
func extractSystemText(system interface{}) string {
	if system == nil {
		return ""
	}

	// 处理字符串格式
	if systemStr, ok := system.(string); ok {
		return systemStr
	}

	// 处理数组格式
	if systemArr, ok := system.([]interface{}); ok {
		var textParts []string
		for _, block := range systemArr {
			if blockMap, ok := block.(map[string]interface{}); ok {
				if blockMap["type"] == "text" {
					if text, ok := blockMap["text"].(string); ok {
						textParts = append(textParts, text)
					}
				}
			}
		}
		return strings.Join(textParts, "\n")
	}

	return ""
}

// ExtractReasoningText 从 OpenRouter reasoning_details 中提取文本
// 处理不同的推理详情类型：reasoning.text、reasoning.summary、reasoning.encrypted
// 导出此函数以便在 stream_processor.go 中使用
func ExtractReasoningText(detail map[string]interface{}) string {
	detailType, _ := detail["type"].(string)

	switch detailType {
	case constants.ReasoningTypeText:
		// 提取文本字段
		if text, ok := detail["text"].(string); ok {
			return text
		}
	case constants.ReasoningTypeSummary:
		// 提取摘要字段
		if summary, ok := detail["summary"].(string); ok {
			return summary
		}
	case constants.ReasoningTypeEncrypted:
		// 跳过加密的推理内容 - 这是 base64 编码的加密数据，不应显示
		// 像 Grok 这样的模型会在 reasoning.summary 旁边发送这个
		return ""
	}

	return ""
}

// ConvertRequest 将 Claude API 请求转换为 OpenAI 格式
func ConvertRequest(claudeReq models.ClaudeRequest, cfg *config.Config) (*models.OpenAIRequest, error) {
	// 使用基于模式的路由映射模型
	openaiModel := mapModel(claudeReq.Model, cfg)

	// 提取系统消息（可以是字符串或内容块数组）
	systemText := extractSystemText(claudeReq.System)

	// 转换消息
	openaiMessages := convertMessages(claudeReq.Messages, systemText)

	// 构建 OpenAI 请求
	openaiReq := &models.OpenAIRequest{
		Model:       openaiModel,
		Messages:    openaiMessages,
		Temperature: claudeReq.Temperature,
		TopP:        claudeReq.TopP,
		Stream:      claudeReq.Stream,
	}

	// 启用使用量跟踪和推理 - 针对不同提供商的特定设置
	if claudeReq.Stream != nil && *claudeReq.Stream {
		provider := cfg.DetectProvider()

		switch provider {
		case config.ProviderOpenRouter:
			// OpenRouter 需要启用推理块和使用量跟踪
			// - reasoning.enabled: 在响应中启用思考块
			// - usage.include: 即使在流式模式下也跟踪令牌使用量
			openaiReq.StreamOptions = map[string]interface{}{
				"include_usage": true,
			}
			openaiReq.Usage = map[string]interface{}{
				"include": true,
			}
			openaiReq.Reasoning = map[string]interface{}{
				"enabled": true,
			}

		case config.ProviderOpenAI:
			// OpenAI GPT-5 模型支持 reasoning_effort 参数
			// 此参数控制模型在响应前花多少时间思考
			openaiReq.StreamOptions = map[string]interface{}{
				"include_usage": true,
			}
			openaiReq.ReasoningEffort = "medium" // minimal | low | medium | high

		case config.ProviderOllama:
			// Ollama 需要在有工具时显式设置 tool_choice
			// 否则 Ollama 模型可能不会自然选择使用工具
			if len(claudeReq.Tools) > 0 {
				openaiReq.ToolChoice = "required"
			}
		}
	}

	// 使用自适应的单模型检测设置令牌限制
	if claudeReq.MaxTokens > 0 {
		// 使用基于能力的检测 - 无硬编码模型模式！
		// ShouldUseMaxCompletionTokens 检查缓存的单模型能力：
		// - 缓存命中：使用已学习的值（max_completion_tokens 或 max_tokens）
		// - 缓存未命中：首先尝试 max_completion_tokens（将通过重试自动检测）
		// 这适用于任何模型/提供商，无需代码更改
		if cfg.ShouldUseMaxCompletionTokens(openaiModel) {
			openaiReq.MaxCompletionTokens = claudeReq.MaxTokens
		} else {
			openaiReq.MaxTokens = claudeReq.MaxTokens
		}
	}

	// 转换停止序列
	if len(claudeReq.StopSequences) > 0 {
		openaiReq.Stop = claudeReq.StopSequences
	}

	// 转换工具（如果存在）
	if len(claudeReq.Tools) > 0 {
		openaiReq.Tools = convertTools(claudeReq.Tools)
	}

	return openaiReq, nil
}

// mapModel 使用模式匹配将 Claude 模型名称映射到特定提供商的模型。
// 将 haiku/sonnet/opus 层级路由到适当的 Gemini 模型，并允许
// 通过环境变量覆盖以路由到 Grok、Gemini 或 DeepSeek 等替代提供商。
// 非 Claude 模型名称将原样传递。
func mapModel(claudeModel string, cfg *config.Config) string {
	modelLower := strings.ToLower(claudeModel)

	// Haiku 层级
	if strings.Contains(modelLower, "haiku") {
		if cfg.HaikuModel != "" {
			return cfg.HaikuModel
		}
		return DefaultHaikuModel
	}

	// Sonnet 层级
	if strings.Contains(modelLower, "sonnet") {
		if cfg.SonnetModel != "" {
			return cfg.SonnetModel
		}
		return DefaultSonnetModel
	}

	// Opus 层级
	if strings.Contains(modelLower, "opus") {
		if cfg.OpusModel != "" {
			return cfg.OpusModel
		}
		return DefaultOpusModel
	}

	// 非 Claude 模型直接传递（OpenAI、OpenRouter 等）
	return claudeModel
}

// convertMessages 将 Claude 消息转换为 OpenAI 格式。
//
// 处理三种内容类型：
//   - 字符串内容：简单文本消息
//   - 包含块的数组内容：text、tool_use（映射到 tool_calls）和 tool_result（映射到 role=tool）
//   - 工具结果：特殊处理以创建 OpenAI 工具响应消息
//
// 该函数在翻译 Claude 的内容块结构到 OpenAI 的消息格式时保持对话流程，
// 确保工具调用 ID 被保留以用于关联。
func convertMessages(claudeMessages []models.ClaudeMessage, system string) []models.OpenAIMessage {
	openaiMessages := []models.OpenAIMessage{}

	// 如果存在系统消息，添加它
	if system != "" {
		openaiMessages = append(openaiMessages, models.OpenAIMessage{
			Role:    "system",
			Content: system,
		})
	}

	// 转换每条 Claude 消息
	for _, msg := range claudeMessages {
		// 处理内容（可以是字符串或块数组）
		switch content := msg.Content.(type) {
		case string:
			// 简单文本消息
			openaiMessages = append(openaiMessages, models.OpenAIMessage{
				Role:    msg.Role,
				Content: content,
			})

		case []interface{}:
			// 处理复杂内容块
			var textParts []string
			var toolCalls []models.OpenAIToolCall
			var hasToolResult bool

			// 第一遍：检查是否为工具结果消息
			for _, block := range content {
				if blockMap, ok := block.(map[string]interface{}); ok {
					if blockMap["type"] == constants.ContentTypeToolResult {
						hasToolResult = true
						break
					}
				}
			}

			// 根据类型处理块
			for _, block := range content {
				if blockMap, ok := block.(map[string]interface{}); ok {
					blockType := blockMap["type"]

					switch blockType {
					case "text":
						// 提取文本内容
						if text, ok := blockMap["text"].(string); ok {
							textParts = append(textParts, text)
						}

					case constants.ContentTypeToolUse:
						// 将 tool_use 转换为 OpenAI 的 tool_calls 格式
						toolUseID, _ := blockMap["id"].(string)
						toolName, _ := blockMap["name"].(string)
						toolInput := blockMap["input"]

						// 如果 ID 为空，生成一个
						if toolUseID == "" {
							toolUseID = GenerateToolID()
						}

						// 将 input 序列化为 JSON 字符串
						var inputJSON string
						if toolInput != nil {
							if inputBytes, err := json.Marshal(toolInput); err == nil {
								inputJSON = string(inputBytes)
							} else {
								inputJSON = "{}"
							}
						} else {
							inputJSON = "{}"
						}

						toolCall := models.OpenAIToolCall{
							ID:   toolUseID,
							Type: "function",
						}
						toolCall.Function.Name = toolName
						toolCall.Function.Arguments = inputJSON
						toolCalls = append(toolCalls, toolCall)

					case constants.ContentTypeToolResult:
						// 将 tool_result 转换为 OpenAI 的 tool 消息格式
						toolUseID, _ := blockMap["tool_use_id"].(string)
						toolContent := ""

						// 从工具结果中提取内容
						if resultContent, ok := blockMap["content"].(string); ok {
							toolContent = resultContent
						} else if resultContent, ok := blockMap["content"].([]interface{}); ok {
							// 处理工具结果中的复杂内容
							var contentParts []string
							for _, item := range resultContent {
								if itemMap, ok := item.(map[string]interface{}); ok {
									if itemMap["type"] == "text" {
										if text, ok := itemMap["text"].(string); ok {
											contentParts = append(contentParts, text)
										}
									}
								}
							}
							toolContent = strings.Join(contentParts, "\n")
						}

						openaiMessages = append(openaiMessages, models.OpenAIMessage{
							Role:       "tool",
							Content:    toolContent,
							ToolCallID: toolUseID,
						})
					}
				}
			}

			// 添加包含文本和/或工具调用的助手消息
			if len(textParts) > 0 || len(toolCalls) > 0 {
				if !hasToolResult {
					textContent := strings.Join(textParts, "\n")
					openaiMessages = append(openaiMessages, models.OpenAIMessage{
						Role:      msg.Role,
						Content:   textContent,
						ToolCalls: toolCalls,
					})
				}
			}

		default:
			// 未知内容类型，尝试原样添加
			openaiMessages = append(openaiMessages, models.OpenAIMessage{
				Role:    msg.Role,
				Content: content,
			})
		}
	}

	return openaiMessages
}

// convertTools 将 Claude 工具定义转换为 OpenAI 函数调用格式。
// 将工具名称、描述和 input_schema 映射到 OpenAI 的函数结构。
// 同时在描述中添加参数提示，帮助模型正确使用参数。
func convertTools(claudeTools []models.Tool) []models.OpenAITool {
	openaiTools := make([]models.OpenAITool, len(claudeTools))

	for i, tool := range claudeTools {
		openaiTools[i] = models.OpenAITool{
			Type: "function",
		}
		openaiTools[i].Function.Name = tool.Name
		// 增强工具描述，添加参数使用提示
		openaiTools[i].Function.Description = enhanceToolDescription(tool.Name, tool.Description)
		openaiTools[i].Function.Parameters = tool.InputSchema
	}

	return openaiTools
}

// enhanceToolDescription 增强工具描述，添加参数使用提示
// 这有助于防止模型使用错误的参数名称（如 "query"）
func enhanceToolDescription(toolName, description string) string {
	toolNameLower := strings.ToLower(toolName)

	// 根据工具类型添加参数提示（双语强调）
	var paramHint string
	switch {
	case strings.Contains(toolNameLower, "edit"):
		paramHint = "\n\n[REQUIRED PARAMS] file_path, old_string, new_string - ALL THREE are required with DIFFERENT values. DO NOT use 'query'.\n【必需参数】file_path, old_string, new_string（三个都必需，值必须不同）。禁止使用 query。"
	case strings.Contains(toolNameLower, "bash"):
		paramHint = "\n\n[REQUIRED PARAM] command - The shell command to execute. DO NOT use 'query'.\n【必需参数】command（要执行的命令）。禁止使用 query。"
	case strings.Contains(toolNameLower, "read"):
		paramHint = "\n\n[REQUIRED PARAM] file_path - The absolute path to read. DO NOT use 'query'.\n【必需参数】file_path（绝对路径）。禁止使用 query。"
	case strings.Contains(toolNameLower, "write"):
		paramHint = "\n\n[REQUIRED PARAMS] file_path, content - BOTH required. DO NOT use 'query'.\n【必需参数】file_path, content（两个都必需）。禁止使用 query。"
	case strings.Contains(toolNameLower, "grep"):
		paramHint = "\n\n[REQUIRED PARAM] pattern - The regex pattern to search. DO NOT use 'query'.\n【必需参数】pattern（正则表达式）。禁止使用 query。"
	case strings.Contains(toolNameLower, "glob"):
		paramHint = "\n\n[REQUIRED PARAM] pattern - The glob pattern to match. DO NOT use 'query'.\n【必需参数】pattern（glob 模式）。禁止使用 query。"
	case strings.Contains(toolNameLower, "lsp"):
		paramHint = "\n\n[REQUIRED PARAMS] operation, filePath, line, character - ALL required. DO NOT use 'query'.\n【必需参数】operation, filePath, line, character（全部必需）。禁止使用 query。"
	case strings.Contains(toolNameLower, "task") && !strings.Contains(toolNameLower, "todo"):
		paramHint = "\n\n[REQUIRED PARAMS] description, prompt, subagent_type - ALL required. DO NOT use 'query'.\n【必需参数】description, prompt, subagent_type（全部必需）。禁止使用 query。"
	case strings.Contains(toolNameLower, "webfetch") || strings.Contains(toolNameLower, "fetch"):
		paramHint = "\n\n[REQUIRED PARAMS] url, prompt - BOTH required. DO NOT use 'query'.\n【必需参数】url, prompt（两个都必需）。禁止使用 query。"
	case strings.Contains(toolNameLower, "websearch") || strings.Contains(toolNameLower, "search"):
		// WebSearch 确实使用 query 参数
		paramHint = "\n\n[REQUIRED PARAM] query - The search query string.\n【必需参数】query（搜索查询字符串）。"
	}

	return description + paramHint
}

// ConvertResponse 将 OpenAI 响应转换为 Claude 格式
func ConvertResponse(openaiResp *models.OpenAIResponse, requestedModel string) (*models.ClaudeResponse, error) {
	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenAI response")
	}

	choice := openaiResp.Choices[0]

	// 将内容转换为 Claude 格式
	var contentBlocks []models.ContentBlock

	// 处理 reasoning_details（转换为思考块）
	// 这必须在其他内容块之前
	if len(choice.Message.ReasoningDetails) > 0 {
		emptySignature := "" // 用于创建空字符串指针
		for _, reasoningDetail := range choice.Message.ReasoningDetails {
			if detailMap, ok := reasoningDetail.(map[string]interface{}); ok {
				thinkingText := ExtractReasoningText(detailMap)
				if thinkingText != "" {
					contentBlocks = append(contentBlocks, models.ContentBlock{
						Type:      "thinking",
						Thinking:  thinkingText,
						Signature: &emptySignature, // Claude Code 正确隐藏/显示思考块所必需
					})
				}
			}
		}
	}

	// 用于跟踪已处理的工具调用 ID，防止双重处理
	// 当后端同时返回 Claude 原生格式（content 数组中的 tool_use）和 OpenAI 格式（tool_calls 数组）时
	// 需要去重以避免同一个工具调用被处理两次
	processedToolIDs := make(map[string]bool)

	// 处理文本内容
	if choice.Message.Content != nil {
		// 处理字符串内容
		if contentStr, ok := choice.Message.Content.(string); ok && contentStr != "" {
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type: "text",
				Text: contentStr,
			})
		}

		// 处理数组内容（某些提供商的 Claude 原生格式）
		// 某些 OpenAI 兼容 API 可能直接返回 Claude 风格的内容块
		if contentArr, ok := choice.Message.Content.([]interface{}); ok {
			for _, block := range contentArr {
				if blockMap, ok := block.(map[string]interface{}); ok {
					blockType, _ := blockMap["type"].(string)
					switch blockType {
					case "thinking":
						// Claude 原生思考块
						if thinking, ok := blockMap["thinking"].(string); ok {
							emptySignature := "" // 用于创建空字符串指针
							contentBlocks = append(contentBlocks, models.ContentBlock{
								Type:      "thinking",
								Thinking:  thinking,
								Signature: &emptySignature, // Claude Code 所必需
							})
						}
					case "text":
						// Claude 原生文本块
						if text, ok := blockMap["text"].(string); ok {
							contentBlocks = append(contentBlocks, models.ContentBlock{
								Type: "text",
								Text: text,
							})
						}
					case constants.ContentTypeToolUse:
						// Claude 原生 tool_use 块
						toolID, _ := blockMap["id"].(string)
						toolName, _ := blockMap["name"].(string)
						toolInput := blockMap["input"]

						// 如果没有 ID，生成一个
						if toolID == "" {
							toolID = GenerateToolID()
						}

						// 标记此工具调用已处理
						processedToolIDs[toolID] = true

						contentBlocks = append(contentBlocks, models.ContentBlock{
							Type:  constants.ContentTypeToolUse,
							ID:    toolID,
							Name:  toolName,
							Input: sanitizeToolInputFromInterface(toolName, toolInput),
						})
					}
				}
			}
		}
	}

	// 处理工具调用（转换为 tool_use 块）
	// 跳过已经从 content 数组中处理过的工具调用
	for _, toolCall := range choice.Message.ToolCalls {
		// 检查是否已经处理过此工具调用
		if processedToolIDs[toolCall.ID] {
			continue
		}
		contentBlocks = append(contentBlocks, models.ContentBlock{
			Type:  "tool_use",
			ID:    toolCall.ID,
			Name:  toolCall.Function.Name,
			Input: sanitizeToolInput(toolCall.Function.Name, toolCall.Function.Arguments),
		})
	}

	// 转换完成原因
	var stopReason *string
	if choice.FinishReason != nil {
		reason := convertFinishReason(*choice.FinishReason)
		stopReason = &reason
	}

	// 构建 Claude 响应
	claudeResp := &models.ClaudeResponse{
		ID:         openaiResp.ID,
		Type:       "message",
		Role:       "assistant",
		Content:    contentBlocks,
		Model:      requestedModel, // 使用原始请求的模型
		StopReason: stopReason,
		Usage: models.Usage{
			InputTokens:  openaiResp.Usage.PromptTokens,
			OutputTokens: openaiResp.Usage.CompletionTokens,
		},
	}

	return claudeResp, nil
}

// convertFinishReason 将 OpenAI 的完成原因映射到 Claude 格式
func convertFinishReason(openaiReason string) string {
	switch openaiReason {
	case constants.FinishReasonStop:
		return constants.StopReasonEndTurn
	case constants.FinishReasonLength:
		return constants.StopReasonMaxTokens
	case constants.FinishReasonToolCalls:
		return constants.StopReasonToolUse
	case constants.FinishReasonContentFilter:
		return constants.StopReasonEndTurn // Claude 没有完全对应的值
	default:
		return constants.StopReasonEndTurn
	}
}

// sanitizeToolInput 修复参数格式错误的常见模型错误。
// 特别处理模型幻觉出 "query" 参数而不是正确的必需参数（file_path、command、pattern 等）的问题。
// 此函数始终移除 "query" 参数，因为它对任何工具都无效。
func sanitizeToolInput(toolName string, argsJSON string) interface{} {
	// 处理空字符串或空白字符串的情况
	argsJSON = strings.TrimSpace(argsJSON)
	if argsJSON == "" || argsJSON == "{}" {
		// 返回空对象而不是空字符串，避免工具调用失败
		return map[string]interface{}{}
	}

	var input map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &input); err != nil {
		// 如果不是 JSON 对象，原样返回（可能是字符串或原始类型）
		return argsJSON
	}

	// 无论是否存在 query，始终清理输入
	return SanitizeToolArgs(toolName, input)
}

// SanitizeToolArgs 通过移除无效的 "query" 参数并根据工具类型将其映射到正确的必需参数来修复工具参数。
// 导出此函数以便在流式和非流式处理器中使用。
//
// 此函数处理以下几种情况：
// 1. 模型发送 {"query": "..."} 而不是正确的参数
// 2. 模型发送 {"query": "{...}"} 内部包含 JSON 编码的参数
// 3. 模型发送正确的参数但附带额外的 "query" 字段
func SanitizeToolArgs(toolName string, input map[string]interface{}) map[string]interface{} {
	if input == nil {
		// 返回空对象而不是 nil，避免工具调用失败
		return map[string]interface{}{}
	}

	// 将工具名称标准化为小写以进行匹配
	toolNameLower := strings.ToLower(toolName)

	// 如果存在 query 参数则提取并移除（不区分大小写）
	var queryContent string
	var queryMap map[string]interface{}
	for key, val := range input {
		if strings.ToLower(key) == "query" {
			switch v := val.(type) {
			case string:
				queryContent = v
			case map[string]interface{}:
				// query 值是对象，直接合并到 input 中
				queryMap = v
			}
			delete(input, key)
		}
	}

	// 如果 query 是对象，直接合并其内容
	if queryMap != nil {
		for k, v := range queryMap {
			if _, exists := input[k]; !exists {
				input[k] = v
			}
		}
		// 如果合并后有所有必需参数，则返回
		if hasRequiredParams(toolNameLower, input) {
			return input
		}
	}

	// 如果没有找到 query 字符串，直接返回原始输入
	if queryContent == "" {
		return input
	}

	// 首先，尝试将 query 解析为 JSON 对象并合并到输入中
	// 这处理模型发送 {"query": "{\"file_path\":\"...\", \"old_string\":\"...\"}"} 的情况
	if strings.HasPrefix(strings.TrimSpace(queryContent), "{") {
		var parsedQuery map[string]interface{}
		if err := json.Unmarshal([]byte(queryContent), &parsedQuery); err == nil {
			// 将解析的 query 合并到输入中（不覆盖现有键）
			for k, v := range parsedQuery {
				if _, exists := input[k]; !exists {
					input[k] = v
				}
			}
			// 合并后，如果有所有必需参数，则返回
			if hasRequiredParams(toolNameLower, input) {
				return input
			}
		}
	}

	// 根据工具类型将 query 内容映射到正确的必需参数
	// 使用 contains 进行模糊匹配以处理 "mcp__xxx__Edit" 等变体
	switch {
	// Edit 工具：需要 file_path、old_string、new_string
	case strings.Contains(toolNameLower, "edit"):
		if _, ok := input["file_path"]; !ok {
			input["file_path"] = queryContent
		}
		if _, ok := input["old_string"]; !ok {
			input["old_string"] = queryContent
		}
		if _, ok := input["new_string"]; !ok {
			input["new_string"] = queryContent
		}

	// Grep 工具：需要 pattern
	case strings.Contains(toolNameLower, "grep"):
		if _, ok := input["pattern"]; !ok {
			input["pattern"] = queryContent
		}
		// 如果缺少 path 则设置默认值
		if _, ok := input["path"]; !ok {
			input["path"] = "."
		}

	// Bash 工具：需要 command
	case strings.Contains(toolNameLower, "bash"):
		if _, ok := input["command"]; !ok {
			input["command"] = queryContent
		}

	// Read/ReadFile 工具：需要 file_path
	case strings.Contains(toolNameLower, "read"):
		if _, ok := input["file_path"]; !ok {
			input["file_path"] = queryContent
		}

	// Write/WriteFile 工具：需要 file_path、content
	case strings.Contains(toolNameLower, "write"):
		if _, ok := input["file_path"]; !ok {
			input["file_path"] = queryContent
		}
		if _, ok := input["content"]; !ok {
			input["content"] = queryContent
		}

	// Glob 工具：需要 pattern
	case strings.Contains(toolNameLower, "glob"):
		if _, ok := input["pattern"]; !ok {
			input["pattern"] = queryContent
		}

	// LSP 工具：需要 filePath
	case strings.Contains(toolNameLower, "lsp"):
		if _, ok := input["filePath"]; !ok {
			input["filePath"] = queryContent
		}

	// Task 工具：使用 prompt（但不处理 TodoWrite，因为它需要 todos 数组）
	case strings.Contains(toolNameLower, "task") && !strings.Contains(toolNameLower, "todo"):
		if _, ok := input["prompt"]; !ok {
			input["prompt"] = queryContent
		}

	// TodoWrite 工具：需要 todos 数组
	// 尝试将 query 解析为 JSON 数组
	case strings.Contains(toolNameLower, "todo"):
		if _, ok := input["todos"]; !ok {
			// 尝试将 queryContent 解析为 JSON 数组
			if strings.HasPrefix(strings.TrimSpace(queryContent), "[") {
				var todosArray []interface{}
				if err := json.Unmarshal([]byte(queryContent), &todosArray); err == nil {
					input["todos"] = todosArray
				}
			}
		}

	// WebFetch/WebSearch：根据情况使用 url 或 query
	case strings.Contains(toolNameLower, "webfetch") || strings.Contains(toolNameLower, "fetch"):
		if _, ok := input["url"]; !ok {
			input["url"] = queryContent
		}
	case strings.Contains(toolNameLower, "websearch") || strings.Contains(toolNameLower, "search"):
		// WebSearch 实际使用 "query" - 但我们已经移除了它，所以恢复它
		input["query"] = queryContent

	// Skill 工具：需要 skill 参数
	case strings.Contains(toolNameLower, "skill"):
		if _, ok := input["skill"]; !ok {
			input["skill"] = queryContent
		}

	// AskUserQuestion 工具：需要 questions 参数
	case strings.Contains(toolNameLower, "askuserquestion") || strings.Contains(toolNameLower, "ask"):
		if _, ok := input["questions"]; !ok {
			// 尝试将 queryContent 解析为 JSON 数组
			if strings.HasPrefix(strings.TrimSpace(queryContent), "[") {
				var questionsArray []interface{}
				if err := json.Unmarshal([]byte(queryContent), &questionsArray); err == nil {
					input["questions"] = questionsArray
				}
			}
		}

	// NotebookEdit 工具：需要 notebook_path 和 new_source
	case strings.Contains(toolNameLower, "notebook"):
		if _, ok := input["notebook_path"]; !ok {
			input["notebook_path"] = queryContent
		}
	}

	return input
}

// hasRequiredParams 检查输入是否具有工具所需的最少必需参数
func hasRequiredParams(toolNameLower string, input map[string]interface{}) bool {
	switch {
	case strings.Contains(toolNameLower, "edit"):
		_, hasFilePath := input["file_path"]
		_, hasOldString := input["old_string"]
		_, hasNewString := input["new_string"]
		return hasFilePath && hasOldString && hasNewString
	case strings.Contains(toolNameLower, "bash"):
		_, hasCommand := input["command"]
		return hasCommand
	case strings.Contains(toolNameLower, "read"):
		_, hasFilePath := input["file_path"]
		return hasFilePath
	case strings.Contains(toolNameLower, "grep"):
		_, hasPattern := input["pattern"]
		return hasPattern
	case strings.Contains(toolNameLower, "glob"):
		_, hasPattern := input["pattern"]
		return hasPattern
	case strings.Contains(toolNameLower, "write"):
		_, hasFilePath := input["file_path"]
		_, hasContent := input["content"]
		return hasFilePath && hasContent
	case strings.Contains(toolNameLower, "todo"):
		_, hasTodos := input["todos"]
		return hasTodos
	case strings.Contains(toolNameLower, "skill"):
		_, hasSkill := input["skill"]
		return hasSkill
	case strings.Contains(toolNameLower, "notebook"):
		_, hasPath := input["notebook_path"]
		return hasPath
	default:
		return true
	}
}

// sanitizeToolInputFromInterface 处理 interface{} 类型的工具输入
// 用于处理从 Claude 原生格式的 content 数组中提取的 tool_use 块
func sanitizeToolInputFromInterface(toolName string, input interface{}) interface{} {
	if input == nil {
		return map[string]interface{}{}
	}

	// 如果已经是 map，直接清理
	if inputMap, ok := input.(map[string]interface{}); ok {
		return SanitizeToolArgs(toolName, inputMap)
	}

	// 如果是字符串，尝试解析为 JSON
	if inputStr, ok := input.(string); ok {
		inputStr = strings.TrimSpace(inputStr)
		if inputStr == "" || inputStr == "{}" {
			return map[string]interface{}{}
		}

		var inputMap map[string]interface{}
		if err := json.Unmarshal([]byte(inputStr), &inputMap); err == nil {
			return SanitizeToolArgs(toolName, inputMap)
		}
		// 解析失败，返回原始字符串
		return inputStr
	}

	// 其他类型，尝试序列化后再解析
	if inputBytes, err := json.Marshal(input); err == nil {
		var inputMap map[string]interface{}
		if err := json.Unmarshal(inputBytes, &inputMap); err == nil {
			return SanitizeToolArgs(toolName, inputMap)
		}
	}

	// 无法处理，返回原始值
	return input
}
