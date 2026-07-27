package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
)

// Config 保存运行期配置，全部来自环境变量。
type Config struct {
	Port          string
	Display       string
	GatewayToken  string
	ScreenshotDir string

	// TokenFromEnv 记录 token 是注入的还是本次启动随机生成的。
	// 生产部署必须是 true —— 随机 token 只有本进程知道，容器一重启
	// 就变了，调用方会突然全部 401，这是很难排查的故障。
	TokenFromEnv bool
}

// Load 读取环境变量并准备运行所需的目录。
func Load() *Config {
	token := os.Getenv("GATEWAY_TOKEN")
	fromEnv := token != ""
	if !fromEnv {
		token = "cs_" + randomHex(24)
		log.Printf("[config] WARNING: GATEWAY_TOKEN not set; generated an ephemeral token. " +
			"Callers cannot know it and it changes on every restart. " +
			"Set GATEWAY_TOKEN as a secret for any real deployment.")
	}

	screenshotDir := getEnv("SCREENSHOT_DIR", "/tmp/sandbox_screenshots")
	if err := os.MkdirAll(screenshotDir, 0o755); err != nil {
		log.Printf("[config] warning: cannot create screenshot dir %s: %v", screenshotDir, err)
	}

	return &Config{
		Port:          getEnv("PORT", "7860"),
		Display:       getEnv("DISPLAY", ":99"),
		GatewayToken:  token,
		ScreenshotDir: screenshotDir,
		TokenFromEnv:  fromEnv,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败意味着系统熵源不可用，继续运行会得到可预测的
		// token，比直接退出更危险。
		log.Fatalf("[config] cannot read random bytes: %v", err)
	}
	return hex.EncodeToString(b)
}
