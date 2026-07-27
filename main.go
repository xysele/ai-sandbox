package main

import (
	"log"
	"os"

	"ai-sandbox-go/internal/config"
	"ai-sandbox-go/internal/server"
)

func main() {
	cfg := config.Load()

	log.Printf("[main] AI Sandbox Gateway starting...")
	log.Printf("[main] Port: %s", cfg.Port)
	log.Printf("[main] Display: %s", cfg.Display)
	// 只报告 token 来源，绝不打印内容或前缀：创空间的运行日志对
	// Studio 访问者可见，前缀足以显著缩小暴力猜测空间。
	if cfg.TokenFromEnv {
		log.Printf("[main] Auth: GATEWAY_TOKEN loaded from environment")
	} else {
		log.Printf("[main] Auth: GATEWAY_TOKEN not set, generated an ephemeral one " +
			"(local dev only; set the secret before exposing this service)")
	}

	srv := server.New(cfg)

	if err := srv.Start(); err != nil {
		log.Printf("[main] Server failed: %v", err)
		os.Exit(1)
	}
}
