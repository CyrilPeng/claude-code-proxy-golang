// Package daemon 处理代理服务器的后台进程管理。
//
// 它管理 PID 文件的创建/删除、进程健康检查，并提供启动、停止和检查代理守护进程状态的函数。
// 守护进程在后台运行，可以通过 CLI（start、stop、status 命令）进行控制。
package daemon

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

const (
	healthURL = "http://localhost:8082/health"
)

var (
	// pidFile 动态初始化以使用操作系统特定的临时目录
	pidFile string
	// logFile 是日志文件的路径
	logFile string
)

// init 确保临时目录存在并初始化 pidFile
func init() {
	// 获取操作系统特定的临时目录
	tempDir := os.TempDir()
	proxyTempDir := filepath.Join(tempDir, "claude-code-proxy-golang")

	// 检查目录是否存在，如果不存在则创建
	if _, err := os.Stat(proxyTempDir); os.IsNotExist(err) {
		// 使用适当的权限创建目录
		if err := os.MkdirAll(proxyTempDir, 0755); err != nil {
			// 如果创建失败，回退到系统临时目录
			fmt.Fprintf(os.Stderr, "警告: 创建目录 %s 失败: %v\n", proxyTempDir, err)
			pidFile = filepath.Join(tempDir, "claude-code-proxy.pid")
			return
		}
	}

	// 设置 pidFile 和 logFile 路径
	pidFile = filepath.Join(proxyTempDir, "claude-code-proxy.pid")
	logFile = filepath.Join(proxyTempDir, "claude-code-proxy.log")
}

// IsRunning 检查代理守护进程是否正在运行
func IsRunning() bool {
	// 首先尝试健康检查
	resp, err := http.Get(healthURL)
	if err == nil {
		_ = resp.Body.Close()
		return resp.StatusCode == 200
	}

	// 回退：检查 PID 文件
	return isProcessRunning()
}

// Start 将当前进程守护进程化
// enableLog 参数控制是否将输出重定向到日志文件
func Start(enableLog bool) error {
	// 检查是否已在运行
	if IsRunning() {
		return fmt.Errorf("代理已在运行中")
	}

	// 清理过期的 PID 文件
	cleanupPID()

	// 写入 PID 文件
	if err := writePID(); err != nil {
		return fmt.Errorf("写入 PID 文件失败: %w", err)
	}

	// 在重定向输出之前打印启动消息（以便用户在控制台中看到）
	fmt.Println("🚀 正在启动 Claude Code Proxy 守护进程...")

	// 只有在启用日志时才重定向到日志文件
	if enableLog {
		fmt.Printf("📝 日志文件: %s\n", logFile)
		// 将 stdout 和 stderr 重定向到日志文件
		if err := redirectOutputToLogFile(); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 重定向输出到日志文件失败: %v\n", err)
			// 继续执行 - 控制台日志记录仍然有效
		}
	}

	return nil
}

// Stop 停止正在运行的守护进程
func Stop() {
	if !IsRunning() {
		fmt.Println("代理未在运行")
		return
	}

	pid, err := readPID()
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取 PID 失败: %v\n", err)
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "查找进程失败: %v\n", err)
		return
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "停止进程失败: %v\n", err)
		return
	}

	cleanupPID()
	fmt.Println("✅ 代理已停止")
}

// Status 打印当前守护进程状态
func Status() {
	if IsRunning() {
		pid, _ := readPID()
		fmt.Printf("✅ 代理正在运行（PID: %d）\n", pid)
		fmt.Printf("   健康检查端点: %s\n", healthURL)
		fmt.Printf("   日志文件: %s\n", logFile)
	} else {
		fmt.Println("❌ 代理未在运行")
	}
}

// 辅助函数

// redirectOutputToLogFile 将 stdout 和 stderr 重定向到日志文件
func redirectOutputToLogFile() error {
	// 以追加模式打开日志文件，如果不存在则创建
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件 %s 失败: %w", logFile, err)
	}

	// 将 stdout 和 stderr 重定向到日志文件
	os.Stdout = f
	os.Stderr = f

	return nil
}

func writePID() error {
	pid := os.Getpid()
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)), 0644)
}

func readPID() (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(string(data))
}

func cleanupPID() {
	_ = os.Remove(pidFile) // 忽略错误
}

func isProcessRunning() bool {
	pid, err := readPID()
	if err != nil {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// 发送信号 0 检查进程是否存在
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Cleanup 应在关闭时调用
func Cleanup() {
	cleanupPID()
}

// GetTempDir 返回代理使用的临时目录
func GetTempDir() string {
	return filepath.Dir(pidFile)
}

// GetLogFile 返回日志文件路径
func GetLogFile() string {
	return logFile
}
