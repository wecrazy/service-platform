package fun

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"service-platform/internal/config"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

// EnsureRedisRunning checks if Redis is running and attempts to start it if not.
func EnsureRedisRunning(host string, port int) error {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err == nil {
		conn.Close()
		logrus.Infof("✅ Redis is already running at %s", addr)
		return nil
	}

	logrus.Warnf("⚠️ Redis is NOT running at %s. Attempting to start...", addr)

	switch runtime.GOOS {
	case "windows":
		return startRedisWindows()
	case "linux":
		return startRedisLinux()
	default:
		return fmt.Errorf("unsupported OS for auto-starting Redis: %s", runtime.GOOS)
	}
}

func startRedisWindows() error {
	// 1. Try Native Windows Redis
	if _, err := exec.LookPath("redis-server"); err == nil {
		// Context with timeout for the command execution
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Start Redis in a new PowerShell window
		// Using Start-Process to ensure it runs independently
		cmd := exec.CommandContext(ctx, "powershell", "-Command", "Start-Process", "redis-server", "-WindowStyle", "Minimized")

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start Redis on Windows: %w", err)
		}

		logrus.Info("🚀 Attempted to start Redis in a new window. Waiting for it to be ready...")
		return waitForRedis(config.GetConfig().Redis.Host, config.GetConfig().Redis.Port, 20)
	}

	// 2. Try WSL Redis
	if _, err := exec.LookPath("wsl"); err == nil {
		logrus.Info("⚠️ Redis not found natively, checking WSL...")

		// Check if redis-server exists in WSL
		if err := exec.Command("wsl", "which", "redis-server").Run(); err == nil {
			logrus.Info("✅ Found redis-server in WSL, attempting to start...")

			// Try starting it in WSL
			cmd := exec.Command("wsl", "redis-server", "--daemonize", "yes")
			if err := cmd.Run(); err != nil {
				// If daemonize fails, try backgrounding with nohup
				logrus.Warnf("Failed to start daemonized Redis in WSL: %v. Trying alternative...", err)
				cmd = exec.Command("wsl", "nohup", "redis-server", "&")
				if err := cmd.Start(); err != nil {
					return fmt.Errorf("failed to start Redis in WSL: %w", err)
				}
			}

			logrus.Info("🚀 Attempted to start Redis in WSL. Waiting for it to be ready...")
			return waitForRedis(config.GetConfig().Redis.Host, config.GetConfig().Redis.Port, 20)
		}
	}

	return fmt.Errorf("redis-server not found in PATH or WSL. Please install Redis for Windows or in WSL")
}

func startRedisLinux() error {
	// Check if redis-server is in PATH
	_, err := exec.LookPath("redis-server")
	if err != nil {
		return fmt.Errorf("redis-server not found in PATH")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start Redis in background
	cmd := exec.CommandContext(ctx, "redis-server", "--daemonize", "yes")
	if err := cmd.Start(); err != nil {
		// Try without daemonize if it fails
		cmd = exec.CommandContext(ctx, "redis-server", "&")
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start Redis on Linux: %w", err)
		}
	}

	logrus.Info("🚀 Attempted to start Redis on Linux. Waiting for it to be ready...")
	return waitForRedis(config.GetConfig().Redis.Host, config.GetConfig().Redis.Port, 20)
}

func waitForRedis(host string, port int, maxAttempts int) error {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	for i := 0; i < maxAttempts; i++ {
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err == nil {
			conn.Close()
			logrus.Infof("✅ Redis started successfully at %s", addr)
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timed out waiting for Redis to start at %s after %d seconds", addr, maxAttempts)
}

func GetRedis(key string, redisDB *redis.Client) string {
	val, _ := redisDB.Get(context.Background(), key).Result()
	return val
}
