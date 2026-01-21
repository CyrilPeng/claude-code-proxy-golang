package main

import (
	"fmt"
	"os"

	"github.com/CyrilPeng/claude-code-proxy-golang/internal/config"
	"github.com/CyrilPeng/claude-code-proxy-golang/internal/daemon"
	"github.com/CyrilPeng/claude-code-proxy-golang/internal/server"
)

func main() {
	// 解析命令和标志
	debug := false
	simpleLog := false
	enableLog := false
	command := ""

	if len(os.Args) > 1 {
		for i := 1; i < len(os.Args); i++ {
			arg := os.Args[i]
			switch arg {
			case "-d", "--debug":
				debug = true
			case "-s", "--simple":
				simpleLog = true
			case "-l", "--log":
				enableLog = true
			case "stop", "status", "version", "help", "-h", "--help":
				command = arg
			}
		}

		// 处理命令
		switch command {
		case "stop":
			daemon.Stop()
			return
		case "status":
			daemon.Status()
			return
		case "version":
			fmt.Println("claude-code-proxy v1.0.0")
			return
		case "help", "-h", "--help":
			printHelp()
			return
		}
	}

	// 加载配置（带调试模式）
	var cfg *config.Config
	var err error
	if debug {
		cfg, err = config.LoadWithDebug(true)
		fmt.Println("🐛 调试模式已启用 - 完整请求/响应日志记录已激活")
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 如果请求，启用简单日志记录
	if simpleLog {
		cfg.SimpleLog = true
		fmt.Println("📊 简单日志模式已启用 - 每个请求一行摘要")
	}

	// 检查是否已在运行
	if daemon.IsRunning() {
		fmt.Println("代理已在运行中")
		os.Exit(0)
	}

	// 守护进程化（后台运行）
	if err := daemon.Start(enableLog); err != nil {
		fmt.Fprintf(os.Stderr, "启动守护进程失败: %v\n", err)
		os.Exit(1)
	}

	// 启动 HTTP 服务器（阻塞）
	// 注意：无需预取推理模型 - 自适应按模型检测通过重试机制自动处理所有模型
	if err := server.Start(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "启动服务器失败: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`Claude Code Proxy - Claude Code 的 OpenAI API 代理

用法:
  claude-code-proxy [-d|--debug] [-s|--simple] [-l|--log]  启动代理守护进程
  claude-code-proxy stop                                   停止代理守护进程
  claude-code-proxy status                                 检查代理是否正在运行
  claude-code-proxy version                                显示版本
  claude-code-proxy help                                   显示此帮助

标志:
  -d, --debug     启用调试模式（记录完整的请求/响应）
  -s, --simple    启用简单日志模式（每个请求一行摘要）
  -l, --log       启用日志文件记录（默认不记录日志文件）

配置:
  配置文件位置（按顺序检查）:
    1. ./.env
    2. ~/.claude/proxy.env
    3. ~/.claude-code-proxy

  必需:
    OPENAI_API_KEY         您的 OpenAI API 密钥

  可选:
    ANTHROPIC_DEFAULT_OPUS_MODEL    覆盖 Opus 路由
    ANTHROPIC_DEFAULT_SONNET_MODEL  覆盖 Sonnet 路由
    ANTHROPIC_DEFAULT_HAIKU_MODEL   覆盖 Haiku 路由
    OPENAI_BASE_URL                 OpenAI API 基础 URL
    HOST                            服务器主机（默认: 0.0.0.0）
    PORT                            服务器端口（默认: 8082）

示例:
  # 启动代理
  claude-code-proxy

  # 配合 Claude Code 使用（通过 ccp 包装脚本）
  ccp chat

  # 或手动配置
  ANTHROPIC_BASE_URL=http://localhost:8082 claude chat`)
}
