# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在本仓库中工作时提供指导。

## 项目概述

Claude Code Proxy 是一个 HTTP 代理，将 Claude API 请求转换为 OpenAI 兼容格式，使 Claude Code 能够通过 OpenRouter、OpenAI Direct (o1/o3) 和 Ollama（本地）与 200+ 替代模型配合使用。代理作为守护进程运行，执行双向 API 格式转换，并保持完整的 Claude Code 功能兼容性，包括工具调用、扩展思维块和流式传输。

## 构建命令

```bash
# 构建二进制文件
go build -o claude-code-proxy cmd/claude-code-proxy/main.go
# 或使用 make
make build

# 为所有平台构建（创建 dist/ 文件夹）
make build-all

# 运行测试
go test ./...

# 运行特定测试文件
go test -v ./internal/converter

# 运行单个测试
go test -v ./internal/converter -run TestConvertMessagesWithComplexContent

# 带覆盖率运行测试
make test-coverage

# 格式化代码
go fmt ./...

# 编译并以简单日志模式启动代理
go build -o claude-code-proxy cmd/claude-code-proxy/main.go && ./claude-code-proxy -s
```

## 架构

### 核心请求流程

1. **Claude Code** → 发送 Claude API 格式请求到 `localhost:8082`
2. **handlers.go** → 接收 `/v1/messages` POST 请求
3. **converter.go** → 转换 Claude 格式 → OpenAI 格式
   - 通过 `cfg.DetectProvider()` 检测提供商类型（OpenRouter/OpenAI/Ollama）
   - 应用特定于提供商的参数（reasoning format, tool_choice）
   - 使用基于模式的路由将 Claude 模型名称映射到目标提供商模型
4. **handlers.go** → 将 OpenAI 请求转发到配置的提供商
5. **提供商** → 返回 OpenAI 格式响应（流式或非流式）
6. **converter.go** → 转换 OpenAI 格式 → Claude 格式
7. **handlers.go** → 返回 Claude 格式响应给 Claude Code

### 特定于提供商的行为

代理根据 `OPENAI_BASE_URL` 应用不同的请求参数：

**OpenRouter** (`https://openrouter.ai/api/v1`):
- 添加 `reasoning: {enabled: true}` 以支持思维功能
- 使用 `usage: {include: true}` 进行 token 跟踪
- 提取 `reasoning_details` 数组 → 转换为 Claude `thinking` 块

**OpenAI Direct** (`https://api.openai.com/v1`):
- 为 GPT-5 推理模型添加 `reasoning_effort: "medium"`
- 使用标准 `stream_options: {include_usage: true}`

**Ollama** (`http://localhost:*`):
- 存在工具时设置 `tool_choice: "required"`（强制使用工具）
- 无 API 密钥验证（localhost 端点跳过认证）

### 格式转换详情

**工具调用**（converter.go 中的 `convertMessages`）:
- Claude `tool_use` 内容块 → OpenAI `tool_calls` 数组
- OpenAI `tool_calls` → Claude `tool_use` 块
- 维护 `tool_use.id` ↔ `tool_result.tool_use_id` 对应关系
- 转换期间将 JSON 参数保持为字符串

**思维块**（converter.go 中的 `ConvertResponse`）:
- OpenRouter `reasoning_details` → 带有 `signature` 字段的 Claude `thinking` 块
- `signature` 字段是 Claude Code 正确隐藏/显示思维内容的必需字段
- 没有 signature，思维内容会作为普通文本显示在聊天中

**流式传输**（handlers.go 中的 `streamOpenAIToClaude`）:
- 转换 OpenAI SSE 块 (`data: {...}`) → Claude SSE 事件
- 生成正确的事件序列：`message_start`、`content_block_start`、`content_block_delta`、`content_block_stop`、`message_delta`、`message_stop`
- 跟踪内容块索引以维护正确的顺序
- 通过跨块累积函数参数处理工具调用增量

### 基于模式的模型路由

converter.go 中的 `mapModel()` 函数实现智能路由：

```go
// Opus 层级（默认：gemini-3-pro-preview）
"*opus*" → google/gemini-3-pro-preview (或 ANTHROPIC_DEFAULT_OPUS_MODEL)

// Sonnet 层级（默认：gemini-3-flash-preview）
"*sonnet*" → google/gemini-3-flash-preview (或 ANTHROPIC_DEFAULT_SONNET_MODEL)

// Haiku 层级（默认：gemini-2.5-pro）
"*haiku*" → google/gemini-2.5-pro (或 ANTHROPIC_DEFAULT_HAIKU_MODEL)
```

通过环境变量覆盖以路由到替代模型（Grok、Gemini、DeepSeek-R1 等）。

### 自适应单模型能力检测

**核心理念**：自动支持所有提供商的特殊情况 - 永远不要让用户承担预先配置的负担。

代理使用完全自适应的系统，通过基于错误的重试和缓存自动学习每个模型支持的参数。这消除了所有硬编码模型模式（v1.3.0 中移除约 100 行）。

**工作原理：**

1. **首次请求（缓存未命中）**：
   - `ShouldUseMaxCompletionTokens()` 检查 `CacheKey{BaseURL, Model}` 的缓存
   - 缓存未命中 → 默认尝试 `max_completion_tokens`（对推理模型正确）
   - 如果提供商返回"不支持的参数"错误，调用 `retryWithoutMaxCompletionTokens()`
   - 重试成功 → 缓存 `{UsesMaxCompletionTokens: false}`
   - 原始请求成功 → 缓存 `{UsesMaxCompletionTokens: true}`

2. **后续请求（缓存命中）**：
   - `ShouldUseMaxCompletionTokens()` 立即返回缓存值
   - 无需试错
   - 首次请求约 1-2 秒延迟，之后即时响应

**缓存结构**（`internal/config/config.go:29-48`）：

```go
type CacheKey struct {
    BaseURL string  // 提供商基础 URL（例如 "https://gpt.erst.dk/api"）
    Model   string  // 模型名称（例如 "gpt-5"）
}

type ModelCapabilities struct {
    UsesMaxCompletionTokens bool      // 通过自适应重试学习
    LastChecked             time.Time // 时间戳
}

// 全局缓存：map[CacheKey]*ModelCapabilities
// 由 sync.RWMutex 保护的线程安全
```

**错误检测**（`internal/server/handlers.go:895-913`）：

```go
func isMaxTokensParameterError(errorMessage string) bool {
    errorLower := strings.ToLower(errorMessage)

    // 广泛的关键字匹配（无状态码限制）
    hasParamIndicator := strings.Contains(errorLower, "parameter") ||
                        strings.Contains(errorLower, "unsupported") ||
                        strings.Contains(errorLower, "invalid")

    hasOurParam := strings.Contains(errorLower, "max_tokens") ||
                   strings.Contains(errorLower, "max_completion_tokens")

    return hasParamIndicator && hasOurParam
}
```

**调试日志**：

使用 `-d` 标志启动代理以查看缓存活动：

```bash
./claude-code-proxy -d -s

# 控制台输出显示：
[DEBUG] Cache MISS: gpt-5 → will auto-detect (try max_completion_tokens)
[DEBUG] Cached: model gpt-5 supports max_completion_tokens (streaming)
[DEBUG] Cache HIT: gpt-5 → max_completion_tokens=true
```

**主要优势**：

- **面向未来**：无需代码更改即可适用于任何新模型/提供商
- **零用户配置**：无需知道每个提供商支持哪些参数
- **单模型粒度**：不同提供商上的相同模型名称分别缓存
- **线程安全**：由 `sync.RWMutex` 保护并发请求
- **内存中**：重启时清除（首次请求重新检测）

**已移除内容**（v1.3.0）：

- `IsReasoningModel()` 函数（30 行）- 检查 gpt-5/o1/o2/o3/o4 模式
- `FetchReasoningModels()` 函数（56 行）- OpenRouter API 调用
- `ReasoningModelCache` 结构（11 行）- 每个提供商的推理模型列表
- Unknown 提供商类型的特定于提供商的硬编码
- 总共移除约 100 行，替换为约 30 行自适应检测

## 配置系统

配置加载优先级（见 `internal/config/config.go`）：
1. `./.env`（本地项目覆盖）
2. `~/.claude/proxy.env`（推荐位置）
3. `~/.claude-code-proxy`（旧位置）

使用 `godotenv.Overload()` 允许后面的文件覆盖前面的。

通过 `DetectProvider()` 中的 URL 模式匹配检测提供商：
- 包含 `openrouter.ai` → ProviderOpenRouter
- 包含 `api.openai.com` → ProviderOpenAI
- 包含 `localhost` 或 `127.0.0.1` → ProviderOllama
- 否则 → ProviderUnknown

## ���试策略

测试套件有两个主要类别：

**提供商测试**（`internal/converter/provider_test.go`）：
- 验证特定于提供商的请求参数是否正确
- 确保 OpenRouter 获得 `reasoning: {enabled: true}` 而非 `reasoning_effort`
- 确保 OpenAI Direct 获得 `reasoning_effort` 而非 `reasoning` 对象
- 确保 Ollama 在存在工具时获得 `tool_choice: "required"`
- 测试提供商隔离（无参数交叉污染）

**转换测试**（`internal/converter/converter_test.go`）：
- 测试 Claude → OpenAI 消息转换
- 测试工具调用格式转换
- 测试从 reasoning_details 提取思维块
- 测试流式块聚合

添加新提供商支持时，按照现有模式在 `provider_test.go` 中创建测试。

## 手动测试

使用 Claude Code CLI 手动测试代理：

### 1. 后台启动代理

```bash
# 先构建
go build -o claude-code-proxy cmd/claude-code-proxy/main.go

# 以简单日志模式启动（推荐用于测试）
./claude-code-proxy -s &

# 或带调试日志
./claude-code-proxy -d &

# 检查是否运行
./claude-code-proxy status
```

### 2. 使用不同的 Claude 模型层级测试

代理将 Claude 模型名称路由到配置的后端模型：

```bash
# 测试 Opus 层级（路由到 ANTHROPIC_DEFAULT_OPUS_MODEL 或 google/gemini-3-pro-preview）
ANTHROPIC_BASE_URL=http://localhost:8082 claude --model opus -p "hi"

# 测试 Sonnet 层级（路由到 ANTHROPIC_DEFAULT_SONNET_MODEL 或 google/gemini-3-flash-preview）
ANTHROPIC_BASE_URL=http://localhost:8082 claude --model sonnet -p "hi"

# 测试 Haiku 层级（路由到 ANTHROPIC_DEFAULT_HAIKU_MODEL 或 google/gemini-2.5-pro）
ANTHROPIC_BASE_URL=http://localhost:8082 claude --model haiku -p "hi"
```

### 3. 验证路由

检查代理日志以查看使用了哪个后端模型：

```bash
# 简单日志模式显示：
# [REQ] https://openrouter.ai/api/v1 model=openai/gpt-5 in=20 out=5 tok/s=25.3

# 调试模式显示完整的请求/响应 JSON
# 日志文件存储在操作系统临时目录：
# - Windows: %TEMP%\claude-code-proxy-golang\claude-code-proxy.log
# - Linux/Mac: /tmp/claude-code-proxy-golang/claude-code-proxy.log
```

### 4. 测试工具调用

```bash
# 使用触发工具使用的提示进行测试
ANTHROPIC_BASE_URL=http://localhost:8082 claude --model sonnet -p "列出当前目录中的文件"

# 应该在调试日志中看到 tool_calls
# 验证正确的 Claude tool_use → OpenAI tool_calls → Claude tool_result 转换
```

### 5. 测试流式传输和思维块

```bash
# 使用推理模型测试（应显示思维块）
# 在 .env 中配置：ANTHROPIC_DEFAULT_SONNET_MODEL=openai/gpt-5
ANTHROPIC_BASE_URL=http://localhost:8082 claude --model sonnet -p "解方程: 2x + 5 = 15"

# 应该在 Claude Code UI 中显示思维过程
# 验证日志中 reasoning_details → thinking 块的转换
```

### 6. 停止代理

```bash
./claude-code-proxy stop
```

### 测试不同提供商

**OpenRouter：**
```bash
# .env 或 ~/.claude/proxy.env
OPENAI_BASE_URL=https://openrouter.ai/api/v1
OPENAI_API_KEY=sk-or-v1-...
ANTHROPIC_DEFAULT_SONNET_MODEL=openai/gpt-5

# 测试
./claude-code-proxy -s &
ANTHROPIC_BASE_URL=http://localhost:8082 claude --model sonnet -p "hi"
```

**OpenAI Direct：**
```bash
# .env
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_API_KEY=sk-proj-...

# 使用推理模型测试
ANTHROPIC_BASE_URL=http://localhost:8082 claude --model opus -p "仔细思考这个问题"
```

**Ollama（本地）：**
```bash
# .env
OPENAI_BASE_URL=http://localhost:11434/v1
ANTHROPIC_DEFAULT_SONNET_MODEL=qwen2.5:14b

# 先启动 Ollama
ollama serve &

# 测试代理
./claude-code-proxy -s &
ANTHROPIC_BASE_URL=http://localhost:8082 claude --model sonnet -p "hi"
```

## 简单日志模式

`-s` 或 `--simple` 标志启用单行请求摘要：

```
[REQ] <base_url> model=<provider_model> in=<tokens> out=<tokens> tok/s=<rate>
```

实现：
- 在请求开始时跟踪 `startTime := time.Now()`
- 从响应使用数据中提取 token 计数
- 计算吞吐量：`tokensPerSec = float64(outputTokens) / duration`
- 在流式（`streamOpenAIToClaude`）和非流式处理程序中输出

Token 提取需要 `float64 → int` 转换，因为 JSON 将数字解组为 float64。

## 常见陷阱

1. **工具参数必须是字符串**：OpenAI 期望 `arguments: "{\"key\":\"value\"}"` 而非 `arguments: {key: "value"}`

2. **思维块需要 signature 字段**：没有 `signature: "..."` 字段，Claude Code 会将思维显示为纯文本而非隐藏它

3. **提供商参数隔离**：永远不要混合 OpenRouter 的 `reasoning` 对象和 OpenAI 的 `reasoning_effort` 参数 - `ConvertRequest()` 中的检测逻辑确保这一点

4. **流式索引跟踪**：内容块必须在 SSE 事件中保持一致的索引 - 使用状态结构跟踪当前索引

5. **Token 计数类型转换**：从 map 中提取时始终将 JSON 数字类型转换为 int：`int(val.(float64))`

## 守护进程

代理作为后台守护进程运行（见 `internal/daemon/daemon.go`）：
- 在操作系统特定的临时目录下的 `claude-code-proxy-golang/` 中创建 PID 文件
  - Windows：`%TEMP%\claude-code-proxy-golang\claude-code-proxy.pid`
  - Linux/Mac：`/tmp/claude-code-proxy-golang/claude-code-proxy.pid`
- 将 stdout/stderr 重定向到同一目录中的日志文件
- `./claude-code-proxy status` 检查进程是否运行
- `./claude-code-proxy stop` 通过 PID 文件杀死守护进程

本地测试时，使用 `-d` 标志进行调试日志以查看完整的请求/响应。

## 包结构

- `cmd/claude-code-proxy/main.go` - 入口点，CLI 参数解析
- `internal/config/` - 环境变量加载，提供商检测
- `internal/converter/` - Claude ↔ OpenAI 格式转换逻辑
- `internal/server/` - HTTP 服务器 (Fiber)，请求处理程序，流式传输
- `internal/daemon/` - 进程管理，PID 文件处理
- `pkg/models/` - Claude 和 OpenAI 格式的共享类型定义
- `scripts/ccp` - 启动守护进程并执行 Claude Code 的包装脚本

## 关键文件

- `internal/converter/converter.go:ConvertRequest()` - Claude → OpenAI 请求转换，带特定于提供商的参数
- `internal/converter/converter.go:ConvertResponse()` - OpenAI → Claude 响应转换，思维块提取
- `internal/server/handlers.go:streamOpenAIToClaude()` - SSE 块转换，事件生成
- `internal/config/config.go:DetectProvider()` - 基于 URL 的提供商检测
- `pkg/models/types.go` - 所有请求/响应类型定义
