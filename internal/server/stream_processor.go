// Package server 提供 HTTP 服务器和请求处理功能。
// stream_processor.go 包含流式响应处理器，将 OpenAI SSE 格式转换为 Claude SSE 格式。
package server

import (
	"bufio"
	"fmt"
	"time"

	"github.com/CyrilPeng/claude-code-proxy-golang/internal/config"
	"github.com/CyrilPeng/claude-code-proxy-golang/internal/converter"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/constants"
	"github.com/CyrilPeng/claude-code-proxy-golang/pkg/json"
)

// StreamState 跟踪流式传输过程中的状态
type StreamState struct {
	// 消息标识
	MessageID string

	// 索引管理
	NextIndex          int
	TextBlockIndex     int
	ThinkingBlockIndex int

	// 块状态标志
	TextBlockStarted       bool
	ThinkingBlockStarted   bool
	ThinkingBlockHasContent bool

	// 工具调用跟踪
	CurrentToolCalls  map[int]*ToolCallState
	ProcessedToolIDs  map[string]bool

	// 最终状态
	FinalStopReason string

	// 使用量数据
	UsageData map[string]interface{}
}

// NewStreamState 创建新的流状态
func NewStreamState() *StreamState {
	return &StreamState{
		MessageID:          fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		NextIndex:          0,
		TextBlockIndex:     -1,
		ThinkingBlockIndex: -1,
		TextBlockStarted:   false,
		ThinkingBlockStarted:    false,
		ThinkingBlockHasContent: false,
		CurrentToolCalls:   make(map[int]*ToolCallState),
		ProcessedToolIDs:   make(map[string]bool),
		FinalStopReason:    constants.StopReasonEndTurn,
		UsageData: map[string]interface{}{
			"input_tokens":                0,
			"output_tokens":               0,
			"cache_creation_input_tokens": 0,
			"cache_read_input_tokens":     0,
			"cache_creation": map[string]interface{}{
				"ephemeral_5m_input_tokens": 0,
				"ephemeral_1h_input_tokens": 0,
			},
		},
	}
}

// StreamProcessor 处理 OpenAI 到 Claude 的流式转换
type StreamProcessor struct {
	writer        *bufio.Writer
	cfg           *config.Config
	providerModel string
	startTime     time.Time
	state         *StreamState
}

// NewStreamProcessor 创建新的流处理器
func NewStreamProcessor(w *bufio.Writer, providerModel string, cfg *config.Config, startTime time.Time) *StreamProcessor {
	return &StreamProcessor{
		writer:        w,
		cfg:           cfg,
		providerModel: providerModel,
		startTime:     startTime,
		state:         NewStreamState(),
	}
}

// SendMessageStart 发送初始 message_start 事件
func (p *StreamProcessor) SendMessageStart() {
	writeSSEEvent(p.writer, constants.EventMessageStart, map[string]interface{}{
		"type": constants.EventMessageStart,
		"message": map[string]interface{}{
			"id":            p.state.MessageID,
			"type":          constants.MessageTypeMessage,
			"role":          constants.RoleAssistant,
			"model":         p.providerModel,
			"content":       []interface{}{},
			"stop_reason":   nil,
			"stop_sequence": nil,
			"usage": map[string]interface{}{
				"input_tokens":                0,
				"output_tokens":               0,
				"cache_creation_input_tokens": 0,
				"cache_read_input_tokens":     0,
				"cache_creation": map[string]interface{}{
					"ephemeral_5m_input_tokens": 0,
					"ephemeral_1h_input_tokens": 0,
				},
			},
		},
	})

	writeSSEEvent(p.writer, constants.EventPing, map[string]interface{}{
		"type": constants.EventPing,
	})

	_ = p.writer.Flush()
}

// HandleThinkingDelta 处理思考块增量
// 支持三种格式：reasoning_content (OpenAI)、reasoning_details (OpenRouter)、reasoning (简化格式)
func (p *StreamProcessor) HandleThinkingDelta(delta map[string]interface{}) {
	// 1. 检查 OpenAI 的 reasoning_content 格式（o1/o3 模型）
	if reasoningContent, ok := delta["reasoning_content"].(string); ok && reasoningContent != "" {
		p.sendThinkingContent(reasoningContent)
	}

	// 2. 检查 OpenRouter 的 reasoning_details 格式
	// 仅在尚未处理 reasoning 字段时处理 reasoning_details
	if reasoningDetailsRaw, ok := delta["reasoning_details"]; ok && delta["reasoning"] == nil {
		if reasoningDetails, ok := reasoningDetailsRaw.([]interface{}); ok && len(reasoningDetails) > 0 {
			for _, detailRaw := range reasoningDetails {
				if detail, ok := detailRaw.(map[string]interface{}); ok {
					thinkingText := converter.ExtractReasoningText(detail)
					if thinkingText != "" {
						p.sendThinkingContent(thinkingText)
					}
				}
			}
		}
	}

	// 3. 直接处理 reasoning 字段（某些模型的简化格式）
	if reasoning, ok := delta["reasoning"].(string); ok && reasoning != "" {
		p.sendThinkingContent(reasoning)
	}
}

// sendThinkingContent 发送思考块内容
func (p *StreamProcessor) sendThinkingContent(content string) {
	// 在第一个思考 delta 时发送思考块的 content_block_start
	if !p.state.ThinkingBlockStarted {
		p.state.ThinkingBlockIndex = p.state.NextIndex
		p.state.NextIndex++

		writeSSEEvent(p.writer, constants.EventContentBlockStart, map[string]interface{}{
			"type":  constants.EventContentBlockStart,
			"index": p.state.ThinkingBlockIndex,
			"content_block": map[string]interface{}{
				"type":      constants.ContentTypeThinking,
				"thinking":  "",
				"signature": "", // 必需，让 Claude Code 正确隐藏/显示思考块
			},
		})
		p.state.ThinkingBlockStarted = true
		_ = p.writer.Flush()
	}

	// 发送思考块 delta
	writeSSEEvent(p.writer, constants.EventContentBlockDelta, map[string]interface{}{
		"type":  constants.EventContentBlockDelta,
		"index": p.state.ThinkingBlockIndex,
		"delta": map[string]interface{}{
			"type":     constants.DeltaTypeThinkingDelta,
			"thinking": content,
		},
	})
	p.state.ThinkingBlockHasContent = true
	_ = p.writer.Flush()
}

// HandleTextDelta 处理文本块增量
func (p *StreamProcessor) HandleTextDelta(content string) {
	if content == "" {
		return
	}

	// 在第一个文本 delta 时发送文本块的 content_block_start
	if !p.state.TextBlockStarted {
		p.state.TextBlockIndex = p.state.NextIndex
		p.state.NextIndex++

		writeSSEEvent(p.writer, constants.EventContentBlockStart, map[string]interface{}{
			"type":  constants.EventContentBlockStart,
			"index": p.state.TextBlockIndex,
			"content_block": map[string]interface{}{
				"type": constants.ContentTypeText,
				"text": "",
			},
		})
		p.state.TextBlockStarted = true
		_ = p.writer.Flush()
	}

	writeSSEEvent(p.writer, constants.EventContentBlockDelta, map[string]interface{}{
		"type":  constants.EventContentBlockDelta,
		"index": p.state.TextBlockIndex,
		"delta": map[string]interface{}{
			"type": constants.DeltaTypeTextDelta,
			"text": content,
		},
	})
	_ = p.writer.Flush()
}

// HandleContentArray 处理 content 数组格式（Claude 原生格式）
func (p *StreamProcessor) HandleContentArray(contentArr []interface{}) {
	for _, block := range contentArr {
		if blockMap, ok := block.(map[string]interface{}); ok {
			blockType, _ := blockMap["type"].(string)

			switch blockType {
			case constants.ContentTypeToolUse:
				p.handleToolUseFromContentArray(blockMap)
			case constants.ContentTypeText:
				if text, ok := blockMap["text"].(string); ok && text != "" {
					p.HandleTextDelta(text)
				}
			case constants.ContentTypeThinking:
				if thinking, ok := blockMap["thinking"].(string); ok && thinking != "" {
					p.sendThinkingContent(thinking)
				}
			}
		}
	}
}

// handleToolUseFromContentArray 处理来自 content 数组的工具调用
func (p *StreamProcessor) handleToolUseFromContentArray(blockMap map[string]interface{}) {
	toolID, _ := blockMap["id"].(string)
	toolName, _ := blockMap["name"].(string)
	toolInput := blockMap["input"]

	if p.cfg.Debug {
		fmt.Printf("[调试] 从 content 数组中提取 tool_use: ID=%s, Name=%s, Input=%v\n", toolID, toolName, toolInput)
	}

	// 如果没有 ID，生成一个
	if toolID == "" {
		toolID = converter.GenerateToolID()
	}

	// 检查是否已经处理过此工具调用
	if p.state.ProcessedToolIDs[toolID] {
		if p.cfg.Debug {
			fmt.Printf("[调试] 跳过已处理的工具调用: ID=%s\n", toolID)
		}
		return
	}
	p.state.ProcessedToolIDs[toolID] = true

	// 创建新的工具调用状态
	tcIndex := len(p.state.CurrentToolCalls)
	p.state.CurrentToolCalls[tcIndex] = &ToolCallState{
		ID:          toolID,
		Name:        toolName,
		ArgsBuffer:  "",
		ClaudeIndex: p.state.NextIndex,
		Started:     true,
	}
	p.state.NextIndex++

	// 序列化 input
	var inputJSON string
	if toolInput != nil {
		if inputBytes, err := json.Marshal(toolInput); err == nil {
			inputJSON = string(inputBytes)
		}
	}
	if inputJSON == "" {
		inputJSON = "{}"
	}
	p.state.CurrentToolCalls[tcIndex].ArgsBuffer = inputJSON

	// 确保文本块已启动
	p.ensureTextBlockStarted()

	// 更新工具调用的索引
	p.state.CurrentToolCalls[tcIndex].ClaudeIndex = p.state.NextIndex
	p.state.NextIndex++

	// 发送 content_block_start
	writeSSEEvent(p.writer, constants.EventContentBlockStart, map[string]interface{}{
		"type":  constants.EventContentBlockStart,
		"index": p.state.CurrentToolCalls[tcIndex].ClaudeIndex,
		"content_block": map[string]interface{}{
			"type":  constants.ContentTypeToolUse,
			"id":    toolID,
			"name":  toolName,
			"input": map[string]interface{}{},
		},
	})
	_ = p.writer.Flush()
}

// ensureTextBlockStarted 确保文本块已启动（用于工具调用前的占位符）
func (p *StreamProcessor) ensureTextBlockStarted() {
	if p.state.TextBlockStarted {
		return
	}

	p.state.TextBlockIndex = p.state.NextIndex
	p.state.NextIndex++

	writeSSEEvent(p.writer, constants.EventContentBlockStart, map[string]interface{}{
		"type":  constants.EventContentBlockStart,
		"index": p.state.TextBlockIndex,
		"content_block": map[string]interface{}{
			"type": constants.ContentTypeText,
			"text": "",
		},
	})
	writeSSEEvent(p.writer, constants.EventContentBlockDelta, map[string]interface{}{
		"type":  constants.EventContentBlockDelta,
		"index": p.state.TextBlockIndex,
		"delta": map[string]interface{}{
			"type": constants.DeltaTypeTextDelta,
			"text": "正在调用工具：",
		},
	})
	p.state.TextBlockStarted = true
	_ = p.writer.Flush()
}

// HandleToolCallsDelta 处理 tool_calls 数组格式的工具调用
func (p *StreamProcessor) HandleToolCallsDelta(toolCallsRaw interface{}) {
	if p.cfg.Debug {
		toolCallsJSON, _ := json.Marshal(toolCallsRaw)
		fmt.Printf("[调试] 原始 tool_calls delta: %s\n", string(toolCallsJSON))
	}

	toolCalls, ok := toolCallsRaw.([]interface{})
	if !ok || len(toolCalls) == 0 {
		return
	}

	for i, tcRaw := range toolCalls {
		tcDelta, ok := tcRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// 获取工具调用索引
		tcIndex := i
		if idx, ok := tcDelta["index"].(float64); ok {
			tcIndex = int(idx)
		}

		// 如果不存在则初始化工具调用跟踪
		if _, exists := p.state.CurrentToolCalls[tcIndex]; !exists {
			p.state.CurrentToolCalls[tcIndex] = &ToolCallState{
				ID:          "",
				Name:        "",
				ArgsBuffer:  "",
				ClaudeIndex: 0,
				Started:     false,
			}
		}

		toolCall := p.state.CurrentToolCalls[tcIndex]

		// 如果提供了工具调用 ID 则更新
		if id, ok := tcDelta["id"].(string); ok {
			if p.state.ProcessedToolIDs[id] {
				if p.cfg.Debug {
					fmt.Printf("[调试] 跳过已处理的工具调用 (tool_calls): ID=%s\n", id)
				}
				continue
			}
			toolCall.ID = id
		}

		// 确保文本块已启动
		p.ensureTextBlockStarted()

		// 处理函数数据
		if functionData, ok := tcDelta["function"].(map[string]interface{}); ok {
			p.processToolCallFunction(tcIndex, toolCall, functionData)
		}
	}
}

// processToolCallFunction 处理工具调用的函数数据
func (p *StreamProcessor) processToolCallFunction(tcIndex int, toolCall *ToolCallState, functionData map[string]interface{}) {
	// 更新函数名称
	if name, ok := functionData["name"].(string); ok {
		toolCall.Name = name
	}

	// 当有函数名称时启动内容块
	if toolCall.Name != "" && !toolCall.Started {
		if toolCall.ID == "" {
			toolCall.ID = converter.GenerateToolID(tcIndex)
			if p.cfg.Debug {
				fmt.Printf("[调试] 为工具 %s 生成 ID: %s\n", toolCall.Name, toolCall.ID)
			}
		}

		// 检查是否已处理
		if p.state.ProcessedToolIDs[toolCall.ID] {
			if p.cfg.Debug {
				fmt.Printf("[调试] 跳过已处理的工具调用 (启动时): ID=%s\n", toolCall.ID)
			}
			return
		}
		p.state.ProcessedToolIDs[toolCall.ID] = true

		toolCall.ClaudeIndex = p.state.NextIndex
		p.state.NextIndex++
		toolCall.Started = true

		writeSSEEvent(p.writer, constants.EventContentBlockStart, map[string]interface{}{
			"type":  constants.EventContentBlockStart,
			"index": toolCall.ClaudeIndex,
			"content_block": map[string]interface{}{
				"type":  constants.ContentTypeToolUse,
				"id":    toolCall.ID,
				"name":  toolCall.Name,
				"input": map[string]interface{}{},
			},
		})
		_ = p.writer.Flush()

		// 注意：不再在这里发送累积的参数
		// 参数将在 finalizeToolCall 中统一清理和发送
		// 这样可以确保 SanitizeToolArgs 能够处理完整的参数
		if p.cfg.Debug && toolCall.ArgsBuffer != "" {
			fmt.Printf("[调试] 工具 %s 有启动前累积的参数: '%s' (将在完成时发送)\n", toolCall.Name, toolCall.ArgsBuffer)
		}
	}

	// 处理函数参数 - 只累积，不发送
	// 参数将在 finalizeToolCall 中统一清理和发送
	_ = p.extractToolArgs(toolCall, functionData)
}

// extractToolArgs 提取工具参数
func (p *StreamProcessor) extractToolArgs(toolCall *ToolCallState, functionData map[string]interface{}) string {
	var argsChunk string

	if args, ok := functionData["arguments"].(string); ok && args != "" {
		toolCall.ArgsBuffer += args
		argsChunk = args
		if p.cfg.Debug {
			fmt.Printf("[调试] 工具 %s 累积参数(字符串): '%s', 当前缓冲区: '%s'\n", toolCall.Name, args, toolCall.ArgsBuffer)
		}
	} else if argsMap, ok := functionData["arguments"].(map[string]interface{}); ok {
		if argsJSON, err := json.Marshal(argsMap); err == nil {
			toolCall.ArgsBuffer = string(argsJSON)
			argsChunk = string(argsJSON)
			if p.cfg.Debug {
				fmt.Printf("[调试] 工具 %s 的参数是对象格式: %s\n", toolCall.Name, toolCall.ArgsBuffer)
			}
		}
	} else if functionData["arguments"] != nil {
		if argsJSON, err := json.Marshal(functionData["arguments"]); err == nil {
			toolCall.ArgsBuffer = string(argsJSON)
			argsChunk = string(argsJSON)
			if p.cfg.Debug {
				fmt.Printf("[调试] 工具 %s 的参数是其他格式: %T -> %s\n", toolCall.Name, functionData["arguments"], toolCall.ArgsBuffer)
			}
		}
	}

	return argsChunk
}

// HandleUsageData 处理使用量数据
func (p *StreamProcessor) HandleUsageData(usage map[string]interface{}) {
	if p.cfg.Debug {
		usageJSON, _ := json.Marshal(usage)
		fmt.Printf("[调试] 收到来自 OpenAI 的使用量: %s\n", string(usageJSON))
	}

	inputTokens := 0
	outputTokens := 0
	if val, ok := usage["prompt_tokens"].(float64); ok {
		inputTokens = int(val)
	}
	if val, ok := usage["completion_tokens"].(float64); ok {
		outputTokens = int(val)
	}

	p.state.UsageData = map[string]interface{}{
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
	}

	// 如果存在缓存指标则添加
	if promptTokensDetails, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
		if cachedTokens, ok := promptTokensDetails["cached_tokens"].(float64); ok && cachedTokens > 0 {
			p.state.UsageData["cache_read_input_tokens"] = int(cachedTokens)
		}
	}

	if p.cfg.Debug {
		usageDataJSON, _ := json.Marshal(p.state.UsageData)
		fmt.Printf("[调试] 累积的使用量数据: %s\n", string(usageDataJSON))
	}
}

// HandleFinishReason 处理完成原因
func (p *StreamProcessor) HandleFinishReason(finishReason string) {
	switch finishReason {
	case constants.FinishReasonLength:
		p.state.FinalStopReason = constants.StopReasonMaxTokens
	case constants.FinishReasonToolCalls, constants.FinishReasonFunctionCall:
		p.state.FinalStopReason = constants.StopReasonToolUse
	case constants.FinishReasonStop:
		p.state.FinalStopReason = constants.StopReasonEndTurn
	default:
		p.state.FinalStopReason = constants.StopReasonEndTurn
	}
}

// FinalizeBlocks 完成所有内容块并发送最终事件
func (p *StreamProcessor) FinalizeBlocks() {
	// 如果文本块已启动，发送 content_block_stop
	if p.state.TextBlockStarted && p.state.TextBlockIndex != -1 {
		writeSSEEvent(p.writer, constants.EventContentBlockStop, map[string]interface{}{
			"type":  constants.EventContentBlockStop,
			"index": p.state.TextBlockIndex,
		})
		_ = p.writer.Flush()
	}

	// 为每个工具调用发送最终 JSON 和 content_block_stop
	for tcIndex, toolData := range p.state.CurrentToolCalls {
		p.finalizeToolCall(tcIndex, toolData)
	}

	// 如果思考块有内容，发送 content_block_stop
	if p.state.ThinkingBlockStarted && p.state.ThinkingBlockHasContent && p.state.ThinkingBlockIndex != -1 {
		writeSSEEvent(p.writer, constants.EventContentBlockStop, map[string]interface{}{
			"type":  constants.EventContentBlockStop,
			"index": p.state.ThinkingBlockIndex,
		})
		_ = p.writer.Flush()
	}

	// 调试：检查是否收到使用量数据
	if p.cfg.Debug {
		inputTokens, _ := p.state.UsageData["input_tokens"].(int)
		outputTokens, _ := p.state.UsageData["output_tokens"].(int)
		if inputTokens == 0 && outputTokens == 0 {
			fmt.Printf("[调试] OpenRouter 流式传输：使用量数据不可用（流式 API 的预期限制）\n")
		}
	}

	// 发送 message_delta
	if p.cfg.Debug {
		usageDataJSON, _ := json.Marshal(p.state.UsageData)
		fmt.Printf("[调试] 发送带有使用量数据的 message_delta: %s\n", string(usageDataJSON))
	}
	writeSSEEvent(p.writer, constants.EventMessageDelta, map[string]interface{}{
		"type": constants.EventMessageDelta,
		"delta": map[string]interface{}{
			"stop_reason":   p.state.FinalStopReason,
			"stop_sequence": nil,
		},
		"usage": p.state.UsageData,
	})
	_ = p.writer.Flush()

	// 发送 message_stop
	writeSSEEvent(p.writer, constants.EventMessageStop, map[string]interface{}{
		"type": constants.EventMessageStop,
	})
	_ = p.writer.Flush()

	// 简单日志
	p.logSimpleSummary()
}

// finalizeToolCall 完成单个工具调用
func (p *StreamProcessor) finalizeToolCall(tcIndex int, toolData *ToolCallState) {
	// 如果工具调用有 Name 但还没有启动，在这里启动它
	if !toolData.Started && toolData.Name != "" {
		if toolData.ID == "" {
			toolData.ID = converter.GenerateToolID(tcIndex)
			if p.cfg.Debug {
				fmt.Printf("[调试] 为工具 %s 生成 ID: %s\n", toolData.Name, toolData.ID)
			}
		}

		if p.state.ProcessedToolIDs[toolData.ID] {
			if p.cfg.Debug {
				fmt.Printf("[调试] 跳过已处理的工具调用 (延迟启动): ID=%s\n", toolData.ID)
			}
			return
		}
		p.state.ProcessedToolIDs[toolData.ID] = true

		toolData.ClaudeIndex = p.state.NextIndex
		p.state.NextIndex++
		toolData.Started = true

		writeSSEEvent(p.writer, constants.EventContentBlockStart, map[string]interface{}{
			"type":  constants.EventContentBlockStart,
			"index": toolData.ClaudeIndex,
			"content_block": map[string]interface{}{
				"type":  constants.ContentTypeToolUse,
				"id":    toolData.ID,
				"name":  toolData.Name,
				"input": map[string]interface{}{},
			},
		})
		_ = p.writer.Flush()

		if p.cfg.Debug {
			fmt.Printf("[调试] 延迟启动工具块: ID=%s, Name=%s, Index=%d\n", toolData.ID, toolData.Name, toolData.ClaudeIndex)
		}
	}

	// 检查 Started 和 claude_index 是否有效
	if toolData.Started && toolData.ClaudeIndex != -1 {
		// 始终在完成时发送清理后的参数
		// 这确保了 SanitizeToolArgs 能够处理完整的参数并修复错误的 query 参数
		p.sendFinalToolArgs(toolData)

		writeSSEEvent(p.writer, constants.EventContentBlockStop, map[string]interface{}{
			"type":  constants.EventContentBlockStop,
			"index": toolData.ClaudeIndex,
		})
		_ = p.writer.Flush()
	}
}

// sendFinalToolArgs 发送工具的最终参数（经过清理）
func (p *StreamProcessor) sendFinalToolArgs(toolData *ToolCallState) {
	if p.cfg.Debug {
		fmt.Printf("[调试] 工具 %s 最终参数缓冲区: '%s' (长度: %d)\n", toolData.Name, toolData.ArgsBuffer, len(toolData.ArgsBuffer))
	}

	var sanitizedJSON []byte

	if toolData.ArgsBuffer != "" {
		var jsonArgs map[string]interface{}
		if err := json.Unmarshal([]byte(toolData.ArgsBuffer), &jsonArgs); err == nil {
			sanitizedArgs := converter.SanitizeToolArgs(toolData.Name, jsonArgs)
			sanitizedJSON, _ = json.Marshal(sanitizedArgs)
			if p.cfg.Debug && string(sanitizedJSON) != toolData.ArgsBuffer {
				fmt.Printf("[调试] 工具 %s 参数已清理: 原始='%s' -> 清理后='%s'\n", toolData.Name, toolData.ArgsBuffer, string(sanitizedJSON))
			}
		} else {
			if p.cfg.Debug {
				fmt.Printf("[调试] 工具 %s 的参数 JSON 解析失败: %v, 原始: %s\n", toolData.Name, err, toolData.ArgsBuffer)
			}
			sanitizedJSON = []byte("{}")
		}
	} else {
		if p.cfg.Debug {
			fmt.Printf("[调试] 工具 %s 的参数为空，发送空对象\n", toolData.Name)
		}
		sanitizedJSON = []byte("{}")
	}

	writeSSEEvent(p.writer, constants.EventContentBlockDelta, map[string]interface{}{
		"type":  constants.EventContentBlockDelta,
		"index": toolData.ClaudeIndex,
		"delta": map[string]interface{}{
			"type":         constants.DeltaTypeInputJSONDelta,
			"partial_json": string(sanitizedJSON),
		},
	})
	_ = p.writer.Flush()
}

// logSimpleSummary 输出简单日志摘要
func (p *StreamProcessor) logSimpleSummary() {
	if !p.cfg.SimpleLog {
		return
	}

	inputTokens := 0
	outputTokens := 0

	if val, ok := p.state.UsageData["input_tokens"].(int); ok {
		inputTokens = val
	} else if val, ok := p.state.UsageData["input_tokens"].(float64); ok {
		inputTokens = int(val)
	}

	if val, ok := p.state.UsageData["output_tokens"].(int); ok {
		outputTokens = val
	} else if val, ok := p.state.UsageData["output_tokens"].(float64); ok {
		outputTokens = int(val)
	}

	if p.cfg.Debug {
		fmt.Printf("[调试] 使用量数据: %+v\n", p.state.UsageData)
	}

	duration := time.Since(p.startTime).Seconds()
	tokensPerSec := 0.0
	if duration > 0 && outputTokens > 0 {
		tokensPerSec = float64(outputTokens) / duration
	}

	timestamp := time.Now().Format("15:04:05")
	fmt.Printf("[%s] [请求] %s 模型=%s 输入=%d 输出=%d 令牌/秒=%.1f\n",
		timestamp,
		p.cfg.OpenAIBaseURL,
		p.providerModel,
		inputTokens,
		outputTokens,
		tokensPerSec)
}
