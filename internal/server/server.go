// Package server 实现 HTTP 代理服务器，在 Claude API 格式和
// OpenAI 兼容提供商（OpenRouter、OpenAI Direct、Ollama）之间进行转换。
//
// 服务器在 /v1/messages 上接收 Claude API 请求，将其转换为 OpenAI 格式，
// 转发到配置的提供商，并将响应转换回 Claude 格式。
// 它处理流式（SSE）和非流式响应，包括来自推理模型的工具调用和思维块。
package server

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/CyrilPeng/claude-code-proxy-golang/internal/config"
	"github.com/CyrilPeng/claude-code-proxy-golang/internal/converter"
	"github.com/CyrilPeng/claude-code-proxy-golang/internal/daemon"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

const (
	// ProxyVersion 是 Claude Code Proxy 的当前版本
	ProxyVersion = "1.0.0"
)

// Start 初始化并启动 HTTP 服务器
func Start(cfg *config.Config) error {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ServerHeader:          "Claude-Code-Proxy",
		AppName:               "Claude Code Proxy v" + ProxyVersion,
	})

	// 中间件
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "*",
	}))

	// 仅在启用简单日志模式时启用 HTTP 日志记录
	if cfg.SimpleLog {
		app.Use(logger.New(logger.Config{
			Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
		}))
	}

	// 健康检查端点
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": ProxyVersion,
		})
	})

	// 根端点 - 代理信息
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Claude Code Proxy",
			"version": ProxyVersion,
			"status":  "running",
			"config": fiber.Map{
				"openai_base_url": cfg.OpenAIBaseURL,
				"routing_mode":    getRoutingMode(cfg),
				"opus_model":      getOpusModel(cfg),
				"sonnet_model":    getSonnetModel(cfg),
				"haiku_model":     getHaikuModel(cfg),
			},
			"endpoints": fiber.Map{
				"health":       "/health",
				"messages":     "/v1/messages",
				"count_tokens": "/v1/messages/count_tokens",
			},
		})
	})

	// Claude API 端点
	setupClaudeEndpoints(app, cfg)

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\n🛑 正在关闭...")
		daemon.Cleanup()
		_ = app.Shutdown()
	}()

	// 启动服务器
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	fmt.Printf("✅ 代理运行于 http://localhost:%s\n", cfg.Port)

	if cfg.PassthroughMode {
		fmt.Printf("   模式: 直通（直接到 Anthropic API）\n")
	} else {
		fmt.Printf("   模式: 转换（通过 %s）\n", cfg.OpenAIBaseURL)
		fmt.Printf("   模型路由: %s\n", getRoutingMode(cfg))

		// 显示实际的模型映射
		if cfg.OpusModel != "" || cfg.SonnetModel != "" || cfg.HaikuModel != "" {
			fmt.Printf("   模型:\n")
			if cfg.OpusModel != "" {
				fmt.Printf("     - Opus   → %s\n", cfg.OpusModel)
			}
			if cfg.SonnetModel != "" {
				fmt.Printf("     - Sonnet → %s\n", cfg.SonnetModel)
			}
			if cfg.HaikuModel != "" {
				fmt.Printf("     - Haiku  → %s\n", cfg.HaikuModel)
			}
		}
	}

	return app.Listen(addr)
}

func getRoutingMode(cfg *config.Config) string {
	if cfg.OpusModel != "" || cfg.SonnetModel != "" || cfg.HaikuModel != "" {
		return "自定义（环境变量覆盖）"
	}
	return "基于模式"
}

func getOpusModel(cfg *config.Config) string {
	if cfg.OpusModel != "" {
		return cfg.OpusModel
	}
	return converter.DefaultOpusModel + "（基于模式）"
}

func getSonnetModel(cfg *config.Config) string {
	if cfg.SonnetModel != "" {
		return cfg.SonnetModel
	}
	return "版本感知（基于模式）"
}

func getHaikuModel(cfg *config.Config) string {
	if cfg.HaikuModel != "" {
		return cfg.HaikuModel
	}
	return converter.DefaultHaikuModel + "（基于模式）"
}

func setupClaudeEndpoints(app *fiber.App, cfg *config.Config) {
	// 消息端点 - 主 Claude API
	app.Post("/v1/messages", func(c *fiber.Ctx) error {
		return handleMessages(c, cfg)
	})

	// 令牌计数端点
	app.Post("/v1/messages/count_tokens", func(c *fiber.Ctx) error {
		return handleCountTokens(c, cfg)
	})
}
