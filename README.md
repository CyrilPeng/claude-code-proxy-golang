# Claude Code Proxy (Go)

<div align="center">

[![最新版本](https://img.shields.io/github/v/release/CyrilPeng/claude-code-proxy-golang?label=version)](https://github.com/CyrilPeng/claude-code-proxy-golang/releases/latest)
[![Go 版本](https://img.shields.io/github/go-mod/go-version/CyrilPeng/claude-code-proxy-golang)](https://go.dev/)
[![许可证](https://img.shields.io/github/license/CyrilPeng/claude-code-proxy-golang)](LICENSE)
[![GitHub issues](https://img.shields.io/github/issues/CyrilPeng/claude-code-proxy-golang)](https://github.com/CyrilPeng/claude-code-proxy-golang/issues)

🔀 **本项目基于 [nielspeter/claude-code-proxy](https://github.com/nielspeter/claude-code-proxy) 修改**

</div>

一个轻量级 HTTP 代理，**将 OpenAI 兼容 API 转换为 Anthropic API 格式**，使 Claude Code 能够与任何 OpenAI 兼容的模型提供商配合使用。

```
┌─────────────┐     Anthropic API     ┌─────────────┐     OpenAI API      ┌─────────────┐
│ Claude Code │ ──────────────────►   │    Proxy    │ ──────────────────► │  Provider   │
│   (CLI)     │ ◄──────────────────   │ (localhost) │ ◄────────────────── │ (API/本地)  │
└─────────────┘     Claude 格式       └─────────────┘     OpenAI 格式     └─────────────┘
```

## 为什么需要这个代理？

**Claude Code** 只支持 Anthropic 的 API。本代理让你可以：

- 🌐 **使用任意模型**：通过 OpenRouter 访问 200+ 模型（GPT、Gemini、Grok、DeepSeek 等）
- 💰 **节省成本**：使用更便宜的模型或免费的本地模型
- 🏠 **本地运行**：通过 Ollama 使用完全离线的本地模型
- 🔄 **无缝切换**：保持 Claude Code 的所有功能，只是后端模型不同

## 支持的提供商

| 提供商 | 说明 | 适用场景 |
|--------|------|----------|
| **[OpenRouter](https://openrouter.ai)** | 统一 API 访问 200+ 模型 | 访问多种云端模型 |
| **OpenAI Direct** | 直接使用 OpenAI API | 使用 GPT 系列模型 |
| **[Ollama](https://ollama.ai)** | 本地模型推理 | 离线使用、隐私保护 |
| **其他 OpenAI 兼容 API** | 任何兼容端点 | 自建服务、其他提供商 |

## 快速开始

### 1. 安装

**方式一：下载预编译版本**

从 [Releases](https://github.com/CyrilPeng/claude-code-proxy-golang/releases) 下载对应平台的二进制文件。

**方式二：从源码构建**

```bash
git clone https://github.com/CyrilPeng/claude-code-proxy-golang.git
cd claude-code-proxy-golang

# 构建
go build -o claude-code-proxy cmd/claude-code-proxy/main.go   # Linux/macOS
go build -o claude-code-proxy.exe cmd/claude-code-proxy/main.go  # Windows

# 或使用 make
make build
```

### 2. 配置

创建配置文件 `~/.claude/proxy.env`：

```bash
mkdir -p ~/.claude
cat > ~/.claude/proxy.env << 'EOF'
# === 必需配置 ===
# API 端点（选择一个提供商）
OPENAI_BASE_URL=https://openrouter.ai/api/v1
OPENAI_API_KEY=sk-or-v1-your-openrouter-key

# === 模型路由 ===
# 当 Claude Code 请求 opus/sonnet/haiku 时，使用这些模型
ANTHROPIC_DEFAULT_OPUS_MODEL=google/gemini-3-pro-preview
ANTHROPIC_DEFAULT_SONNET_MODEL=google/gemini-3-flash-preview
ANTHROPIC_DEFAULT_HAIKU_MODEL=google/gemini-2.5-pro
EOF
```

<details>
<summary>📋 其他提供商配置示例</summary>

**OpenAI Direct：**
```bash
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_API_KEY=sk-proj-your-openai-key
ANTHROPIC_DEFAULT_SONNET_MODEL=gpt-4o
ANTHROPIC_DEFAULT_HAIKU_MODEL=gpt-4o-mini
```

**Ollama（本地）：**
```bash
OPENAI_BASE_URL=http://localhost:11434/v1
# Ollama 不需要 API Key
ANTHROPIC_DEFAULT_SONNET_MODEL=qwen2.5:14b
ANTHROPIC_DEFAULT_HAIKU_MODEL=qwen2.5:7b
```

**自定义 OpenAI 兼容端点：**
```bash
OPENAI_BASE_URL=https://your-custom-endpoint.com/v1
OPENAI_API_KEY=your-api-key
```
</details>

### 3. 启动

```bash
# 启动代理（后台守护进程模式）
./claude-code-proxy

# 查看状态
./claude-code-proxy status

# 停止代理
./claude-code-proxy stop
```

### 4. 使用 Claude Code

```bash
# 设置环境变量指向代理
export ANTHROPIC_BASE_URL=http://localhost:8082

# 正常使用 Claude Code
claude chat
claude code /path/to/project
```

**使用 ccp 包装器（推荐）：**

```bash
# 安装包装器
make install

# 直接使用 ccp 代替 claude
ccp chat                    # 自动启动代理并设置环境变量
ccp code /path/to/project
```

## 功能特性

### ✅ 完整 Claude Code 兼容性

代理完全支持 Claude Code 的所有功能：

| 功能 | 说明 |
|------|------|
| **工具调用** | 所有内置工具：`Read`、`Write`、`Edit`、`Bash`、`Glob`、`Grep`、`LSP`、`Task`、`TodoWrite` 等 |
| **扩展思维** | 正确处理 thinking 块，在 UI 中显示"思考了 Xs"指示器 |
| **流式响应** | 实时流式传输，准确的 SSE 事件格式 |
| **Token 跟踪** | 准确的输入/输出 token 计数 |

### ✅ 智能模型路由

代理自动将 Claude 模型名称映射到配置的后端模型：

| Claude 模型 | 默认映射 | 配置变量 |
|-------------|----------|----------|
| `*opus*` | `google/gemini-3-pro-preview` | `ANTHROPIC_DEFAULT_OPUS_MODEL` |
| `*sonnet*` | `google/gemini-3-flash-preview` | `ANTHROPIC_DEFAULT_SONNET_MODEL` |
| `*haiku*` | `google/gemini-2.5-pro` | `ANTHROPIC_DEFAULT_HAIKU_MODEL` |

### ✅ 自适应参数检测

代理自动学习每个模型支持的 API 参数，无需手动配置：

1. **首次请求**：尝试发送完整参数，如果失败则自动重试
2. **后续请求**：使用缓存的知识，即时响应

```
[DEBUG] Cache MISS: gemini-3-pro-preview → will auto-detect (try max_completion_tokens)
[DEBUG] Cached: model gemini-3-pro-preview supports max_completion_tokens
[DEBUG] Cache HIT: gemini-3-pro-preview → max_completion_tokens=true
```

### ✅ 工具参数自动修复

代理自动修复某些模型常见的工具调用错误：

- 模型错误使用 `query` 参数时，自动映射到正确参数（如 `command`、`file_path`、`pattern`）
- 处理 thinking 模型的特殊响应格式
- 支持对象格式和字符串格式的工具参数

## 命令参考

```bash
./claude-code-proxy              # 启动守护进程
./claude-code-proxy status       # 检查运行状态
./claude-code-proxy stop         # 停止守护进程
./claude-code-proxy version      # 显示版本
./claude-code-proxy help         # 显示帮助
```

**启动选项：**

```bash
-d, --debug     # 调试模式：记录完整请求/响应
-s, --simple    # 简单日志：单行请求摘要
-l, --log       # 启用日志文件记录（默认不记录日志文件）
```

**示例：**

```bash
./claude-code-proxy -d      # 调试模式
./claude-code-proxy -s      # 简单日志
./claude-code-proxy -l      # 启用日志文件记录
./claude-code-proxy -s -l   # 简单日志 + 日志文件
./claude-code-proxy -d -l   # 调试模式 + 日志文件
```

## 配置参考

### 必需配置

| 变量 | 说明 |
|------|------|
| `OPENAI_API_KEY` | API 密钥（Ollama 本地模式不需要） |

### 可选配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `OPENAI_BASE_URL` | - | API 基础 URL |
| `ANTHROPIC_DEFAULT_OPUS_MODEL` | `google/gemini-3-pro-preview` | opus 层级映射模型 |
| `ANTHROPIC_DEFAULT_SONNET_MODEL` | `google/gemini-3-flash-preview` | sonnet 层级映射模型 |
| `ANTHROPIC_DEFAULT_HAIKU_MODEL` | `google/gemini-2.5-pro` | haiku 层级映射模型 |
| `HOST` | `0.0.0.0` | 代理监听地址 |
| `PORT` | `8082` | 代理监听端口 |
| `ANTHROPIC_API_KEY` | - | 客户端验证密钥（可选） |

### OpenRouter 专用配置

| 变量 | 说明 |
|------|------|
| `OPENROUTER_APP_NAME` | 应用名称（用于仪表板追踪） |
| `OPENROUTER_APP_URL` | 应用 URL（可获得更高速率限制） |

### 配置文件位置

代理按以下顺序加载配置（后面的覆盖前面的）：

1. `./.env` - 当前目录
2. `~/.claude/proxy.env` - 推荐位置
3. `~/.claude-code-proxy` - 旧位置（兼容）

## 工作原理

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              请求流程                                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. Claude Code 发送 Anthropic 格式请求                                      │
│     POST /v1/messages                                                       │
│     { model: "claude-sonnet-4", messages: [...], tools: [...] }             │
│                              ↓                                              │
│  2. 代理转换为 OpenAI 格式                                                   │
│     - 模型路由: claude-sonnet-4 → google/gemini-3-flash-preview              │
│     - 消息转换: Claude content blocks → OpenAI messages                     │
│     - 工具转换: tool_use/tool_result → tool_calls/tool messages             │
│                              ↓                                              │
│  3. 发送到 OpenAI 兼容提供商                                                 │
│     POST https://openrouter.ai/api/v1/chat/completions                      │
│                              ↓                                              │
│  4. 接收 OpenAI 格式响应                                                     │
│                              ↓                                              │
│  5. 代理转换回 Claude 格式                                                   │
│     - 响应转换: OpenAI choices → Claude content blocks                       │
│     - 思维块: reasoning_details → thinking blocks                            │
│     - 工具调用: tool_calls → tool_use blocks                                 │
│                              ↓                                              │
│  6. Claude Code 接收正确格式的响应                                           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 格式转换示例

**Claude 工具调用 → OpenAI 格式：**
```json
// Claude tool_use
{ "type": "tool_use", "id": "toolu_123", "name": "Bash", "input": { "command": "ls -la" } }

// 转换为 OpenAI tool_calls
{ "tool_calls": [{ "id": "toolu_123", "type": "function", "function": { "name": "Bash", "arguments": "{\"command\":\"ls -la\"}" } }] }
```

**OpenAI 思维 → Claude 格式：**
```json
// OpenAI reasoning_details
{ "reasoning_details": [{ "type": "reasoning.text", "text": "让我思考..." }] }

// 转换为 Claude thinking block
{ "type": "thinking", "thinking": "让我思考...", "signature": "" }
```

## 项目结构

```
claude-code-proxy-golang/
├── cmd/
│   └── claude-code-proxy/
│       └── main.go              # 程序入口
├── internal/
│   ├── config/                  # 配置管理、提供商检测
│   ├── converter/               # Claude ↔ OpenAI 格式转换
│   ├── server/                  # HTTP 服务器、请求处理、流式传输
│   └── daemon/                  # 守护进程管理
├── pkg/
│   └── models/                  # 类型定义
├── scripts/
│   └── ccp                      # Shell 包装脚本
├── CLAUDE.md                    # 开发者文档
└── README.md                    # 本文件
```

## 开发

```bash
# 开发模式运行
go run cmd/claude-code-proxy/main.go -d

# 运行测试
go test ./...

# 带覆盖率测试
go test -cover ./...

# 格式化代码
go fmt ./...

# 构建所有平台
make build-all
```

### 日志位置

- **Windows**: `%TEMP%\claude-code-proxy-golang\claude-code-proxy.log`
- **Linux/macOS**: `/tmp/claude-code-proxy-golang/claude-code-proxy.log`

## 常见问题

<details>
<summary><strong>Q: 代理启动后 Claude Code 无法连接？</strong></summary>

检查以下几点：
1. 确认代理正在运行：`./claude-code-proxy status`
2. 确认环境变量已设置：`echo $ANTHROPIC_BASE_URL`
3. 检查端口是否被占用：`lsof -i :8082`
</details>

<details>
<summary><strong>Q: 工具调用失败，提示参数缺失？</strong></summary>

某些模型可能不完全遵循工具调用规范。代理已内置自动修复逻辑，但如果问题持续：
1. 使用调试模式查看详细日志：`./claude-code-proxy -d`
2. 尝试切换到更强的模型
3. 检查模型是否支持 function calling
</details>

<details>
<summary><strong>Q: 如何切换不同的模型？</strong></summary>

修改 `~/.claude/proxy.env` 中的模型配置，然后重启代理：
```bash
./claude-code-proxy stop
./claude-code-proxy
```
</details>

<details>
<summary><strong>Q: 本地 Ollama 模型可以使用吗？</strong></summary>

可以！举个例子：
```bash
OPENAI_BASE_URL=http://localhost:11434/v1
ANTHROPIC_DEFAULT_SONNET_MODEL=qwen2.5:14b
```
确保 Ollama 正在运行：`ollama serve`
</details>

## 贡献

欢迎提交 Issue 和 Pull Request！

- 🐛 报告问题：[GitHub Issues](https://github.com/CyrilPeng/claude-code-proxy-golang/issues)
- 💡 功能建议：[GitHub Discussions](https://github.com/CyrilPeng/claude-code-proxy-golang/discussions)

## 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件

## 致谢

- [nielspeter/claude-code-proxy](https://github.com/nielspeter/claude-code-proxy) - 原始仓库
- [Anthropic](https://anthropic.com) - Claude Code CLI
